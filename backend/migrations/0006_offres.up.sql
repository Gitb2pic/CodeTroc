-- F5 — offres des membres sur les packs.
CREATE TYPE statut_offre AS ENUM ('PUBLIEE', 'RETIREE');

CREATE TABLE offre (
    id_offre         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    titre_offre      VARCHAR(120) NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    prix_credits     INTEGER NOT NULL CHECK (prix_credits > 0),
    delai_jours      INTEGER NOT NULL CHECK (delai_jours BETWEEN 1 AND 90),
    jalons_proposes  JSONB NOT NULL CHECK (jsonb_typeof(jalons_proposes) = 'array'),
    statut_offre     statut_offre NOT NULL DEFAULT 'PUBLIEE',
    date_publication TIMESTAMPTZ NOT NULL DEFAULT now(),
    id_prestataire   UUID NOT NULL REFERENCES utilisateur,
    id_pack          UUID NOT NULL REFERENCES pack
);
CREATE INDEX ix_offre_publiee ON offre (date_publication DESC, id_offre DESC) WHERE statut_offre = 'PUBLIEE';
CREATE INDEX ix_offre_pack ON offre (id_pack) WHERE statut_offre = 'PUBLIEE';
CREATE INDEX ix_offre_prestataire ON offre (id_prestataire);
