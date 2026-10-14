-- F12 — coffre de secrets d'accès, chiffrés par enveloppe (AES-256-GCM).
-- La valeur n'existe en clair qu'en mémoire, le temps d'une requête.
CREATE TABLE secret (
    id_secret        UUID PRIMARY KEY,
    id_echange       UUID NOT NULL REFERENCES echange,
    id_deposant      UUID NOT NULL REFERENCES utilisateur,
    libelle          VARCHAR(100) NOT NULL,
    type_secret      VARCHAR(20) NOT NULL CHECK (type_secret IN ('JETON', 'CLE_API', 'MOT_DE_PASSE', 'AUTRE')),
    portee           VARCHAR(300),
    valeur_chiffree  BYTEA,
    nonce            BYTEA,
    dek_chiffree     BYTEA,
    dek_nonce        BYTEA,
    kek_version      INT,
    date_expiration  TIMESTAMPTZ NOT NULL,
    revoque          BOOLEAN NOT NULL DEFAULT false,
    date_revocation  TIMESTAMPTZ,
    date_creation    TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Un secret actif a sa matière chiffrée ; un secret révoqué n'en a plus.
    CONSTRAINT ck_secret_matiere CHECK (
        (NOT revoque AND valeur_chiffree IS NOT NULL AND nonce IS NOT NULL AND dek_chiffree IS NOT NULL AND dek_nonce IS NOT NULL AND kek_version IS NOT NULL AND date_revocation IS NULL)
     OR (revoque AND valeur_chiffree IS NULL AND nonce IS NULL AND dek_chiffree IS NULL AND dek_nonce IS NULL AND date_revocation IS NOT NULL)),
    CHECK (date_expiration > date_creation)
);
CREATE INDEX ix_secret_echange ON secret (id_echange, date_creation DESC);
CREATE INDEX ix_secret_expiration ON secret (date_expiration) WHERE NOT revoque;

-- Journal d'accès, en ajout seul.
CREATE TABLE secret_journal (
    id_entree      BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_secret      UUID NOT NULL REFERENCES secret,
    id_utilisateur UUID REFERENCES utilisateur,           -- NULL : action du système
    action         VARCHAR(20) NOT NULL CHECK (action IN ('CREATION', 'REVELATION', 'REFUS', 'REVOCATION', 'EXPIRATION')),
    ip             INET,
    date_action    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_secret_journal ON secret_journal (id_secret, id_entree);

CREATE FUNCTION interdire_modification_journal() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'secret_journal est en ajout seul';
END $$ LANGUAGE plpgsql;

CREATE TRIGGER secret_journal_ajout_seul
    BEFORE UPDATE OR DELETE ON secret_journal
    FOR EACH ROW EXECUTE FUNCTION interdire_modification_journal();
