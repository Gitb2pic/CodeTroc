-- F18 — comptes par email et mot de passe, à côté de la connexion GitHub.
-- Un compte a au moins une méthode de connexion : GitHub ou un mot de passe.
ALTER TABLE utilisateur ALTER COLUMN github_id DROP NOT NULL;
ALTER TABLE utilisateur ALTER COLUMN github_login DROP NOT NULL;
ALTER TABLE utilisateur ALTER COLUMN github_cree_le DROP NOT NULL;
ALTER TABLE utilisateur ADD COLUMN mot_de_passe_hash TEXT;      -- Argon2id, format PHC
ALTER TABLE utilisateur ADD COLUMN email_verifie_le TIMESTAMPTZ;
ALTER TABLE utilisateur ADD CONSTRAINT ck_utilisateur_connexion
    CHECK (github_id IS NOT NULL OR mot_de_passe_hash IS NOT NULL OR statut_utilisateur = 'SUPPRIME');
ALTER TABLE utilisateur ADD CONSTRAINT ck_utilisateur_mdp_email
    CHECK (mot_de_passe_hash IS NULL OR email IS NOT NULL);

-- Jetons envoyés par email (confirmation, réinitialisation), à usage unique, stockés hachés.
CREATE TABLE jeton_email (
    hash           BYTEA PRIMARY KEY,
    id_utilisateur UUID NOT NULL REFERENCES utilisateur ON DELETE CASCADE,
    type_jeton     VARCHAR(20) NOT NULL CHECK (type_jeton IN ('CONFIRMATION', 'REINITIALISATION')),
    cree_le        TIMESTAMPTZ NOT NULL DEFAULT now(),
    expire_le      TIMESTAMPTZ NOT NULL,
    utilise_le     TIMESTAMPTZ
);
CREATE INDEX ix_jeton_email_utilisateur ON jeton_email (id_utilisateur, type_jeton);

-- Un state OAuth peut servir à lier GitHub à un compte existant (connecté).
ALTER TABLE oauth_state ADD COLUMN id_utilisateur UUID REFERENCES utilisateur ON DELETE CASCADE;
