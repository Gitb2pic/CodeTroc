-- F9 — litiges et arbitrage (un arbitre au MVP, D9).
CREATE TYPE statut_litige AS ENUM ('OUVERT', 'CLOS');
CREATE TYPE decision_litige AS ENUM ('LIBERER', 'REMBOURSER', 'PARTAGER');

CREATE TABLE litige (
    id_litige        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    id_jalon         UUID NOT NULL REFERENCES jalon,
    id_ouvreur       UUID NOT NULL REFERENCES utilisateur,
    motif            TEXT NOT NULL CHECK (length(motif) BETWEEN 10 AND 4000),
    reponse          TEXT CHECK (length(reponse) <= 4000),
    id_arbitre       UUID REFERENCES utilisateur,
    statut_litige    statut_litige NOT NULL DEFAULT 'OUVERT',
    decision         decision_litige,
    part_prestataire SMALLINT CHECK (part_prestataire BETWEEN 1 AND 99),
    justification    TEXT CHECK (length(justification) <= 4000),
    date_ouverture   TIMESTAMPTZ NOT NULL DEFAULT now(),
    date_affectation TIMESTAMPTZ,
    date_cloture     TIMESTAMPTZ,
    CHECK ((decision = 'PARTAGER') = (part_prestataire IS NOT NULL) OR decision IS NULL),        -- RG17
    CHECK (statut_litige = 'OUVERT' OR (decision IS NOT NULL AND id_arbitre IS NOT NULL AND justification IS NOT NULL))
);
-- RG15 : un seul litige ouvert par jalon.
CREATE UNIQUE INDEX ux_litige_ouvert ON litige (id_jalon) WHERE statut_litige = 'OUVERT';
CREATE INDEX ix_litige_arbitre ON litige (id_arbitre, statut_litige);
