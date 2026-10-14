-- F2 — grand livre des crédits en partie double.
CREATE TYPE type_compte AS ENUM ('UTILISATEUR', 'SEQUESTRE', 'EMISSION');
CREATE TYPE type_mouvement AS ENUM ('BONUS', 'BLOCAGE', 'LIBERATION', 'REMBOURSEMENT');

CREATE TABLE compte (
    id_compte      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type_compte    type_compte NOT NULL,
    solde          BIGINT NOT NULL DEFAULT 0,
    id_utilisateur UUID UNIQUE REFERENCES utilisateur,
    id_echange     UUID UNIQUE,                       -- clé étrangère ajoutée avec F6
    cree_le        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_compte_proprietaire CHECK (
        (type_compte = 'UTILISATEUR' AND id_utilisateur IS NOT NULL AND id_echange IS NULL)
     OR (type_compte = 'SEQUESTRE'   AND id_echange IS NOT NULL AND id_utilisateur IS NULL)
     OR (type_compte = 'EMISSION'    AND id_echange IS NULL AND id_utilisateur IS NULL)),
    CONSTRAINT ck_solde_positif CHECK (type_compte = 'EMISSION' OR solde >= 0)   -- RG03
);
CREATE UNIQUE INDEX ux_compte_emission ON compte (type_compte) WHERE type_compte = 'EMISSION';

CREATE TABLE mouvement (
    id_mouvement     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type_mouvement   type_mouvement NOT NULL,
    montant          BIGINT NOT NULL CHECK (montant > 0),             -- RG04
    cle_idempotence  VARCHAR(120) NOT NULL UNIQUE,                    -- D4
    date_mouvement   TIMESTAMPTZ NOT NULL DEFAULT now(),
    id_compte_debit  UUID NOT NULL REFERENCES compte,
    id_compte_credit UUID NOT NULL REFERENCES compte,
    id_echange       UUID,                                            -- D3 ; clé étrangère avec F6
    id_jalon         UUID,
    CHECK (id_compte_debit <> id_compte_credit)
);
CREATE INDEX ix_mvt_debit  ON mouvement (id_compte_debit, date_mouvement DESC, id_mouvement DESC);
CREATE INDEX ix_mvt_credit ON mouvement (id_compte_credit, date_mouvement DESC, id_mouvement DESC);

-- Le grand livre est en ajout seul : aucune écriture n'est modifiée ni supprimée.
CREATE FUNCTION interdire_modification_mouvement() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'le grand livre est en ajout seul';
END
$$ LANGUAGE plpgsql;
CREATE TRIGGER tr_mouvement_ajout_seul BEFORE UPDATE OR DELETE ON mouvement
  FOR EACH ROW EXECUTE FUNCTION interdire_modification_mouvement();

INSERT INTO compte (type_compte) VALUES ('EMISSION');
