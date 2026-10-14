-- F4 — catalogue de packs standardisés.
CREATE TYPE type_preuve AS ENUM ('PR_CI', 'MANUELLE');

CREATE TABLE pack (
    id_pack              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    titre_pack           VARCHAR(120) NOT NULL,
    description_pack     TEXT NOT NULL,
    livrable             TEXT NOT NULL,
    criteres_acceptation JSONB NOT NULL CHECK (jsonb_typeof(criteres_acceptation) = 'array'),
    type_preuve          type_preuve NOT NULL DEFAULT 'PR_CI',
    prix_min             INTEGER NOT NULL CHECK (prix_min > 0),
    prix_max             INTEGER NOT NULL,
    actif                BOOLEAN NOT NULL DEFAULT true,
    date_creation        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (prix_max >= prix_min)
);
CREATE INDEX ix_pack_actif ON pack (date_creation DESC, id_pack DESC) WHERE actif;

CREATE TABLE requerir (
    id_pack       UUID NOT NULL REFERENCES pack ON DELETE CASCADE,
    id_competence UUID NOT NULL REFERENCES competence,
    PRIMARY KEY (id_pack, id_competence)
);
CREATE INDEX ix_requerir_competence ON requerir (id_competence);

-- Catalogue de départ (dossier projet, prix à l'échelle de l'interface).
WITH p AS (
  INSERT INTO pack (titre_pack, description_pack, livrable, criteres_acceptation, type_preuve, prix_min, prix_max) VALUES
  ('Dockeriser une application', 'Conteneuriser une application existante et sa base pour un lancement en une commande.',
   'Dockerfile multi-stage, docker-compose, README',
   '[{"texte":"docker compose up lance l''application sans étape manuelle","verification":"CI"},{"texte":"Image construite en CI","verification":"CI"},{"texte":"README de mise en route","verification":"REVUE"}]', 'PR_CI', 60, 120),
  ('Pipeline CI/CD GitHub Actions', 'Workflow lint, tests, build et déploiement automatique.',
   'Workflows dans .github/workflows',
   '[{"texte":"Lint et tests lancés sur chaque PR","verification":"CI"},{"texte":"Pipeline vert sur la branche principale","verification":"CI"},{"texte":"Déploiement automatique après merge","verification":"CI"},{"texte":"Aucun secret en clair dans le dépôt","verification":"SCAN"}]', 'PR_CI', 80, 160),
  ('Durcissement de base d''un VPS', 'SSH par clé, pare-feu, mises à jour automatiques.',
   'Checklist validée et rapport',
   '[{"texte":"Connexion SSH par mot de passe désactivée","verification":"REVUE"},{"texte":"Pare-feu actif, ports 22/80/443 seulement","verification":"REVUE"},{"texte":"Rapport livré","verification":"REVUE"}]', 'MANUELLE', 50, 100),
  ('Landing page Next.js', 'Page responsive conforme à la maquette, déployée.',
   'Page déployée et code sur le dépôt',
   '[{"texte":"Conforme à la maquette","verification":"REVUE"},{"texte":"Build et lint verts en CI","verification":"CI"},{"texte":"Lisible sur mobile","verification":"REVUE"}]', 'PR_CI', 80, 140),
  ('Intégration de maquette React', 'Composants réutilisables à partir d''une maquette.',
   'Composants documentés',
   '[{"texte":"Rendu conforme à la maquette","verification":"REVUE"},{"texte":"Tests de composants verts","verification":"CI"}]', 'PR_CI', 70, 180),
  ('Tests unitaires d''un module', 'Suite de tests sur un module ciblé.',
   'Suite de tests',
   '[{"texte":"Tests verts en CI","verification":"CI"},{"texte":"Couverture du module visé","verification":"CI"}]', 'PR_CI', 40, 90),
  ('Suite de tests E2E Playwright', 'Parcours critiques couverts de bout en bout.',
   'Tests Playwright exécutés en CI',
   '[{"texte":"Parcours critiques couverts","verification":"REVUE"},{"texte":"Suite verte en CI","verification":"CI"}]', 'PR_CI', 50, 120),
  ('Revue de code d''une PR', 'Revue détaillée avec points bloquants identifiés.',
   'Commentaires et rapport',
   '[{"texte":"Rapport livré","verification":"REVUE"},{"texte":"Points bloquants identifiés","verification":"REVUE"}]', 'MANUELLE', 20, 80)
  RETURNING id_pack, titre_pack
)
INSERT INTO requerir (id_pack, id_competence)
SELECT p.id_pack, c.id_competence FROM p JOIN competence c ON
  (p.titre_pack LIKE 'Dockeriser%'            AND c.libelle_competence IN ('Docker'))
  OR (p.titre_pack LIKE 'Pipeline%'           AND c.libelle_competence IN ('GitHub Actions', 'Docker'))
  OR (p.titre_pack LIKE 'Durcissement%'       AND c.libelle_competence IN ('Sécurité', 'Bash'))
  OR (p.titre_pack LIKE 'Landing%'            AND c.libelle_competence IN ('Next.js', 'React'))
  OR (p.titre_pack LIKE 'Intégration%'        AND c.libelle_competence IN ('React', 'TypeScript'))
  OR (p.titre_pack LIKE 'Tests unitaires%'    AND c.libelle_competence IN ('Tests unitaires'))
  OR (p.titre_pack LIKE 'Suite de tests E2E%' AND c.libelle_competence IN ('Playwright', 'TypeScript'))
  OR (p.titre_pack LIKE 'Revue de code%'      AND c.libelle_competence IN ('Sécurité'));
