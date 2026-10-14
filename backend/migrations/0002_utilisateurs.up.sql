-- F1 — utilisateurs et sessions.
CREATE TYPE role_utilisateur AS ENUM ('MEMBRE', 'ARBITRE', 'ADMIN');
CREATE TYPE statut_utilisateur AS ENUM ('ACTIF', 'SUSPENDU', 'SUPPRIME');

CREATE TABLE utilisateur (
    id_utilisateur     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pseudo             VARCHAR(50)  NOT NULL UNIQUE,
    email              VARCHAR(255) UNIQUE,                 -- D12 : facultatif
    github_id          BIGINT       NOT NULL UNIQUE,
    github_login       VARCHAR(39)  NOT NULL,
    github_cree_le     TIMESTAMPTZ  NOT NULL,
    bio                TEXT CHECK (length(bio) <= 1000),
    role               role_utilisateur   NOT NULL DEFAULT 'MEMBRE',
    statut_utilisateur statut_utilisateur NOT NULL DEFAULT 'ACTIF',
    score_reputation   NUMERIC(5,2) NOT NULL DEFAULT 0 CHECK (score_reputation >= 0),
    date_inscription   TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- State OAuth à usage unique, stocké haché.
CREATE TABLE oauth_state (
    hash      BYTEA PRIMARY KEY,
    expire_le TIMESTAMPTZ NOT NULL
);

-- Jetons de rafraîchissement, stockés hachés, regroupés par famille de rotation.
CREATE TABLE jeton_rafraichissement (
    id_jeton       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hash           BYTEA NOT NULL UNIQUE,
    famille        UUID  NOT NULL,
    id_utilisateur UUID  NOT NULL REFERENCES utilisateur ON DELETE CASCADE,
    cree_le        TIMESTAMPTZ NOT NULL DEFAULT now(),
    expire_le      TIMESTAMPTZ NOT NULL,
    revoque_le     TIMESTAMPTZ
);
CREATE INDEX ix_jeton_famille ON jeton_rafraichissement (famille) WHERE revoque_le IS NULL;
