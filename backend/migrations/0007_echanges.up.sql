-- F6 — échanges, jalons, historique des transitions, idempotence.
CREATE TYPE statut_echange AS ENUM ('PROPOSE', 'REFUSE', 'EN_COURS', 'EN_LITIGE', 'TERMINE', 'ANNULE');
CREATE TYPE statut_jalon AS ENUM ('EN_ATTENTE', 'EN_COURS', 'LIVRE', 'CONTESTE', 'LIBERE', 'REMBOURSE', 'PARTAGE');

CREATE TABLE echange (
    id_echange              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    statut_echange          statut_echange NOT NULL DEFAULT 'PROPOSE',
    montant_total           INTEGER NOT NULL CHECK (montant_total > 0),
    depot_owner             VARCHAR(39)  NOT NULL,
    depot_nom               VARCHAR(100) NOT NULL,
    github_repo_id          BIGINT,                   -- résolu à la première vérification (D6)
    besoin                  TEXT NOT NULL DEFAULT '' CHECK (length(besoin) <= 4000),
    delai_jours             INTEGER NOT NULL CHECK (delai_jours > 0),
    date_creation           TIMESTAMPTZ NOT NULL DEFAULT now(),
    date_acceptation        TIMESTAMPTZ,
    date_cloture            TIMESTAMPTZ,
    annulation_demandee_par UUID REFERENCES utilisateur,
    id_demandeur            UUID NOT NULL REFERENCES utilisateur,
    id_prestataire          UUID NOT NULL REFERENCES utilisateur,  -- copie immuable de l'auteur de l'offre
    id_offre                UUID NOT NULL REFERENCES offre,
    CHECK (id_demandeur <> id_prestataire)                            -- RG09
);
CREATE INDEX ix_echange_demandeur   ON echange (id_demandeur, date_creation DESC);
CREATE INDEX ix_echange_prestataire ON echange (id_prestataire, date_creation DESC);

CREATE TABLE jalon (
    id_jalon       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    id_echange     UUID NOT NULL REFERENCES echange,
    ordre          SMALLINT NOT NULL CHECK (ordre BETWEEN 1 AND 10),
    titre_jalon    VARCHAR(120) NOT NULL,
    montant_jalon  INTEGER NOT NULL CHECK (montant_jalon > 0),
    statut_jalon   statut_jalon NOT NULL DEFAULT 'EN_ATTENTE',
    echeance       TIMESTAMPTZ,
    date_livraison TIMESTAMPTZ,
    date_cloture   TIMESTAMPTZ,
    UNIQUE (id_echange, ordre)
);
-- Planificateur : jalons livrés en attente de validation tacite.
CREATE INDEX ix_jalon_livre ON jalon (date_livraison) WHERE statut_jalon = 'LIVRE';

-- ENF04 : chaque changement d'état est horodaté et attribué (NULL = système).
CREATE TABLE historique_transition (
    id_transition BIGSERIAL PRIMARY KEY,
    entite        VARCHAR(20) NOT NULL,
    id_entite     UUID NOT NULL,
    de            VARCHAR(20),
    vers          VARCHAR(20) NOT NULL,
    par           UUID REFERENCES utilisateur,
    motif         TEXT,
    date          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_historique_entite ON historique_transition (id_entite, date);

-- D4 : clés Idempotency-Key, propres à chaque utilisateur.
CREATE TABLE cle_idempotence (
    id_utilisateur UUID NOT NULL REFERENCES utilisateur ON DELETE CASCADE,
    cle            VARCHAR(100) NOT NULL,
    route          TEXT NOT NULL,
    empreinte      BYTEA NOT NULL,
    statut_http    SMALLINT,
    corps          BYTEA,
    cree_le        TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (id_utilisateur, cle)
);

ALTER TABLE compte    ADD CONSTRAINT fk_compte_echange    FOREIGN KEY (id_echange) REFERENCES echange;
ALTER TABLE mouvement ADD CONSTRAINT fk_mouvement_echange FOREIGN KEY (id_echange) REFERENCES echange;
ALTER TABLE mouvement ADD CONSTRAINT fk_mouvement_jalon   FOREIGN KEY (id_jalon)   REFERENCES jalon;
