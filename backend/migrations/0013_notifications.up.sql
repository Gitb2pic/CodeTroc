-- F13 — notifications, écrites dans la même transaction que l'action qui les cause.
CREATE TABLE notification (
    id_notification UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    id_utilisateur  UUID NOT NULL REFERENCES utilisateur,
    type            VARCHAR(40) NOT NULL,
    id_echange      UUID REFERENCES echange,
    donnees         JSONB NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(donnees) = 'object'),
    lue             BOOLEAN NOT NULL DEFAULT false,
    date_creation   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_notification_utilisateur ON notification (id_utilisateur, date_creation DESC, id_notification DESC);
CREATE INDEX ix_notification_non_lues ON notification (id_utilisateur) WHERE NOT lue;
