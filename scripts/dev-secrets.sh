#!/usr/bin/env sh
# Génère les secrets de l'environnement local dans ./secrets (jamais commités).
set -eu
cd "$(dirname "$0")/.."
mkdir -p secrets
chmod 700 secrets
if [ -f secrets/db_password ]; then
  echo "secrets/ existe déjà : rien à faire."
  exit 0
fi
umask 077
pw=$(head -c 24 /dev/urandom | od -An -tx1 | tr -d ' \n')
printf '%s' "$pw" > secrets/db_password
printf 'postgres://codetroc:%s@db:5432/codetroc?sslmode=disable' "$pw" > secrets/database_url
# Fichiers lisibles par les utilisateurs non root des conteneurs (montage
# direct en Compose) ; le dossier, lui, reste privé sur l'hôte.
chmod 644 secrets/db_password secrets/database_url
echo "Secrets générés dans ./secrets."
