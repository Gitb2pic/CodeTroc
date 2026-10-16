# Déployer CodeTroc sur un VPS

Une seule machine, Docker Compose : Traefik (HTTPS Let's Encrypt), PostgreSQL,
migrations, API, worker et front. Les fichiers utilisés : `compose.yaml` +
`compose.prod.yaml`.

## 0. Prérequis

- Un VPS Linux (Debian 12 ou Ubuntu 24.04), **2 Go de RAM minimum** (la
  compilation Go et Vite en consomment), 20 Go de disque.
- Le domaine `codetroc.labtools.tech`.
- Un compte GitHub avec lequel créer l'OAuth App et la GitHub App.

## 1. DNS

Crée un enregistrement **A** (et **AAAA** si le VPS a de l'IPv6) :
`codetroc.labtools.tech → <IP du VPS>`. Attends qu'il réponde
(`dig +short codetroc.labtools.tech`) avant le premier démarrage, sinon Let's
Encrypt échoue.

## 2. Préparer le serveur

```sh
ssh root@<IP>
apt update && apt upgrade -y
# Docker Engine + plugin Compose (dépôt officiel Docker)
curl -fsSL https://get.docker.com | sh
# Pare-feu : SSH, HTTP, HTTPS uniquement (PostgreSQL n'est jamais exposé)
apt install -y ufw
ufw allow OpenSSH && ufw allow 80/tcp && ufw allow 443/tcp && ufw enable
# Avec 2 Go de RAM, un swap évite les échecs de compilation
fallocate -l 2G /swapfile && chmod 600 /swapfile && mkswap /swapfile && swapon /swapfile
echo '/swapfile none swap sw 0 0' >> /etc/fstab
```

> Docker publie ses ports en contournant ufw : seuls 80 et 443 le sont dans
> `compose.prod.yaml`, la base reste sur un réseau interne.

## 3. Récupérer le code

```sh
git clone https://github.com/Gitb2pic/CodeTroc.git /opt/codetroc
cd /opt/codetroc
```

## 4. Créer les applications GitHub

### 4a. OAuth App (connexion des membres)

GitHub → *Settings* → *Developer settings* → *OAuth Apps* → *New OAuth App* :

| Champ | Valeur |
|---|---|
| Homepage URL | `https://codetroc.labtools.tech` |
| Authorization callback URL | `https://codetroc.labtools.tech/api/v1/auth/github/callback` |

Note le **Client ID**, génère un **Client secret**.

### 4b. GitHub App (vérification des PR et webhooks)

GitHub → *Developer settings* → *GitHub Apps* → *New GitHub App* :

| Champ | Valeur |
|---|---|
| Homepage URL | `https://codetroc.labtools.tech` |
| Webhook URL | `https://codetroc.labtools.tech/api/v1/webhooks/github` |
| Webhook secret | une valeur aléatoire : `openssl rand -hex 32` |
| Permissions (Repository) | **Pull requests : Read-only**, **Checks : Read-only**, **Metadata : Read-only**, **Issues : Read and write** (projet Scrum, F16) |
| Événements | **Pull request**, **Check suite**, **Check run**, **Issues** |

Crée l'App, note son **App ID**, puis génère et télécharge une **clé privée**
(`.pem`). Les membres installent l'App sur les dépôts de leurs échanges : sans
installation, les preuves restent « en attente ». Si l'App existait déjà sans
**Issues : Read and write**, ajoute la permission puis fais-la accepter par
chaque propriétaire de dépôt : en attendant, le Scrum fonctionne sur CodeTroc et
affiche « permission manquante » au lieu de créer les issues.

## 5. Secrets et configuration

```sh
cd /opt/codetroc
./scripts/dev-secrets.sh          # mot de passe PostgreSQL, clé JWT, clé du coffre (VAULT_KEK)

# Secrets GitHub (fichiers lus par les conteneurs, jamais en variables d'environnement)
printf '%s' '<client secret OAuth>' > secrets/github_client_secret
printf '%s' '<webhook secret>'      > secrets/github_webhook_secret
cp /chemin/vers/cle-github-app.pem    secrets/github_app_private_key.pem
chmod 644 secrets/*                  # lisibles par les utilisateurs non root des conteneurs
chmod 700 secrets                    # le dossier reste privé sur l'hôte

cp .env.prod.example .env && chmod 600 .env
nano .env    # compléter ACME_EMAIL, GITHUB_CLIENT_ID, GITHUB_APP_ID
```

**Email (facultatif).** L'inscription par email et mot de passe (F18) reste
fermée tant que `SMTP_HOST` est vide dans `.env` : la page d'accueil ne propose
alors que GitHub. Pour l'ouvrir, renseigner un relais SMTP authentifié
(Brevo, Mailjet, Scaleway TEM, OVH…) : `SMTP_HOST`, `SMTP_PORT` (465 = TLS
direct, sinon STARTTLS obligatoire), `SMTP_USER`, `SMTP_PASSWORD`, `SMTP_FROM`
(adresse d'un domaine autorisé chez le relais, avec SPF et DKIM configurés),
puis `docker compose -f compose.yaml -f compose.prod.yaml up -d api`.

> **Sauvegarde `secrets/vault_kek` hors du serveur** (gestionnaire de mots de
> passe) : sans elle, les secrets déposés dans le coffre sont illisibles, même
> avec une sauvegarde de la base.

## 6. Démarrer

```sh
docker compose -f compose.yaml -f compose.prod.yaml up -d --build
docker compose -f compose.yaml -f compose.prod.yaml ps
```

Ordre automatique : PostgreSQL → migrations → API et worker ; Traefik obtient
le certificat au premier accès HTTPS (quelques secondes).

Vérifications :

```sh
curl -I http://codetroc.labtools.tech            # 301 vers https
curl https://codetroc.labtools.tech/api/v1/ready # {"status":"ready"}
```

Puis ouvre `https://codetroc.labtools.tech`, connecte-toi avec GitHub.

Pour te donner le rôle d'administrateur après ta première connexion :

```sh
docker compose -f compose.yaml -f compose.prod.yaml exec db \
  psql -U codetroc -d codetroc -c "UPDATE utilisateur SET role = 'ADMIN' WHERE github_login = 'Gitb2pic';"
```

## 7. Sauvegardes

```sh
chmod +x scripts/backup.sh
crontab -e
# chaque nuit à 3 h 15
15 3 * * * cd /opt/codetroc && ./scripts/backup.sh >> backups/backup.log 2>&1
```

Copie régulièrement `backups/` hors du VPS. Restauration :

```sh
docker compose -f compose.yaml -f compose.prod.yaml exec -T db \
  pg_restore -U codetroc -d codetroc --clean --if-exists < backups/codetroc-AAAAMMJJ-HHMMSS.dump
```

## 8. Mettre à jour

```sh
cd /opt/codetroc
git pull
docker compose -f compose.yaml -f compose.prod.yaml up -d --build
docker image prune -f
```

Les nouvelles migrations s'appliquent automatiquement avant le redémarrage de
l'API.

## 9. Exploiter

| Besoin | Commande |
|---|---|
| Journaux de l'API | `docker compose -f compose.yaml -f compose.prod.yaml logs -f api` |
| Journaux du worker | `… logs -f worker` |
| Redémarrer un service | `… restart api` |
| Tout arrêter | `… down` (les données restent dans le volume `pgdata`) |

Pour taper moins : `alias dc='docker compose -f compose.yaml -f compose.prod.yaml'`.

## En cas de problème

| Symptôme | Piste |
|---|---|
| Certificat invalide | DNS pas encore propagé, ou ports 80/443 fermés chez l'hébergeur. `dc logs traefik` |
| `/ready` en 503 | Base indisponible : `dc logs db migrate` |
| L'API redémarre en boucle | Configuration refusée au démarrage (le message liste tout ce qui manque) : `dc logs api` |
| Connexion GitHub refusée | URL de retour de l'OAuth App différente de `https://<domaine>/api/v1/auth/github/callback` |
| Preuves « en attente » | GitHub App non installée sur le dépôt, ou webhook en échec (onglet *Advanced* de l'App) |
