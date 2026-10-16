-- F17 — annonces (demandes d'aide) et candidatures des prestataires.
CREATE TYPE statut_annonce AS ENUM ('OUVERTE', 'POURVUE', 'RETIREE', 'EXPIREE');
CREATE TYPE statut_candidature AS ENUM ('EN_ATTENTE', 'ACCEPTEE', 'REFUSEE', 'RETIREE');

CREATE TABLE annonce (
    id_annonce         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    titre_annonce      VARCHAR(120) NOT NULL,
    besoin             TEXT NOT NULL CHECK (length(besoin) BETWEEN 1 AND 4000),
    depot_owner        VARCHAR(39)  NOT NULL,
    depot_nom          VARCHAR(100) NOT NULL,
    budget_credits     INTEGER NOT NULL CHECK (budget_credits > 0),
    delai_jours        INTEGER NOT NULL CHECK (delai_jours BETWEEN 1 AND 90),
    jalons_souhaites   JSONB NOT NULL CHECK (jsonb_typeof(jalons_souhaites) = 'array'),
    duree_sprint_jours SMALLINT NOT NULL DEFAULT 7 CHECK (duree_sprint_jours IN (7, 14)),
    statut_annonce     statut_annonce NOT NULL DEFAULT 'OUVERTE',
    date_publication   TIMESTAMPTZ NOT NULL DEFAULT now(),
    date_expiration    TIMESTAMPTZ NOT NULL,
    id_auteur          UUID NOT NULL REFERENCES utilisateur,
    id_pack            UUID NOT NULL REFERENCES pack,
    CHECK (date_expiration > date_publication)
);
CREATE INDEX ix_annonce_ouverte ON annonce (date_publication DESC, id_annonce DESC) WHERE statut_annonce = 'OUVERTE';
CREATE INDEX ix_annonce_expiration ON annonce (date_expiration) WHERE statut_annonce = 'OUVERTE';
CREATE INDEX ix_annonce_auteur ON annonce (id_auteur, date_publication DESC);

CREATE TABLE candidature (
    id_candidature      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    id_annonce          UUID NOT NULL REFERENCES annonce,
    id_prestataire      UUID NOT NULL REFERENCES utilisateur,
    prix_credits        INTEGER NOT NULL CHECK (prix_credits > 0),
    delai_jours         INTEGER NOT NULL CHECK (delai_jours BETWEEN 1 AND 90),
    jalons_proposes     JSONB NOT NULL CHECK (jsonb_typeof(jalons_proposes) = 'array'),
    message             TEXT NOT NULL DEFAULT '' CHECK (length(message) <= 2000),
    statut_candidature  statut_candidature NOT NULL DEFAULT 'EN_ATTENTE',
    date_candidature    TIMESTAMPTZ NOT NULL DEFAULT now(),
    date_decision       TIMESTAMPTZ
);
-- Une seule candidature en attente par prestataire et par annonce.
CREATE UNIQUE INDEX ux_candidature_active ON candidature (id_annonce, id_prestataire) WHERE statut_candidature = 'EN_ATTENTE';
CREATE INDEX ix_candidature_annonce ON candidature (id_annonce, date_candidature);
CREATE INDEX ix_candidature_prestataire ON candidature (id_prestataire, date_candidature DESC);

-- Un échange naît d'une offre (F6) ou d'une annonce (F17) : exactement l'un des deux.
-- Le pack est recopié dans l'échange : il fixe le type de preuve quelle que soit l'origine.
ALTER TABLE echange ADD COLUMN id_pack UUID REFERENCES pack;
UPDATE echange e SET id_pack = o.id_pack FROM offre o WHERE o.id_offre = e.id_offre;
ALTER TABLE echange ALTER COLUMN id_pack SET NOT NULL;
ALTER TABLE echange ALTER COLUMN id_offre DROP NOT NULL;
ALTER TABLE echange ADD COLUMN id_annonce UUID REFERENCES annonce;
ALTER TABLE echange ADD CONSTRAINT ck_echange_origine CHECK ((id_offre IS NULL) <> (id_annonce IS NULL));
-- Une annonce ne produit qu'un échange.
CREATE UNIQUE INDEX ux_echange_annonce ON echange (id_annonce) WHERE id_annonce IS NOT NULL;
