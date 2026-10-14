-- F11 — messagerie liée à un échange.
CREATE TABLE message (
    id_message UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    id_echange UUID NOT NULL REFERENCES echange,
    id_auteur  UUID NOT NULL REFERENCES utilisateur,
    contenu    TEXT NOT NULL CHECK (length(contenu) BETWEEN 1 AND 5000),
    date_envoi TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_message_echange ON message (id_echange, date_envoi DESC, id_message DESC);
