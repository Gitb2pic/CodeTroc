-- F7 — preuves de livraison et webhooks GitHub.
CREATE TYPE statut_preuve AS ENUM ('EN_ATTENTE', 'VALIDE', 'REJETEE');

CREATE TABLE preuve (
    id_preuve     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    id_jalon      UUID NOT NULL REFERENCES jalon,
    type_preuve   type_preuve NOT NULL,
    pr_owner      VARCHAR(39),
    pr_repo       VARCHAR(100),
    pr_numero     INTEGER,
    github_pr_id  BIGINT,
    sha_commit    VARCHAR(64),
    note          TEXT CHECK (length(note) <= 2000),
    statut_preuve statut_preuve NOT NULL DEFAULT 'EN_ATTENTE',
    detail_verif  TEXT,
    date_verif    TIMESTAMPTZ,
    date_creation TIMESTAMPTZ NOT NULL DEFAULT now(),
    soumis_par    UUID NOT NULL REFERENCES utilisateur,
    CHECK ((type_preuve = 'PR_CI') = (pr_numero IS NOT NULL)),
    CHECK (type_preuve = 'PR_CI' OR note IS NOT NULL)
);
CREATE INDEX ix_preuve_jalon ON preuve (id_jalon, date_creation);
-- Réconciliation et webhooks : preuves encore à vérifier.
CREATE INDEX ix_preuve_a_verifier ON preuve (lower(pr_owner), lower(pr_repo), pr_numero) WHERE statut_preuve <> 'VALIDE';
-- D6 : une PR ne prouve qu'un seul jalon.
CREATE UNIQUE INDEX ux_preuve_pr_valide ON preuve (github_pr_id) WHERE statut_preuve = 'VALIDE';

-- Livraisons de webhooks déjà traitées (X-GitHub-Delivery).
CREATE TABLE webhook_livraison (
    id_livraison VARCHAR(100) PRIMARY KEY,
    evenement    VARCHAR(50) NOT NULL,
    recu_le      TIMESTAMPTZ NOT NULL DEFAULT now()
);
