-- F10 — avis croisés et réputation.
CREATE TABLE avis (
    id_avis         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    id_echange      UUID NOT NULL REFERENCES echange,
    id_auteur       UUID NOT NULL REFERENCES utilisateur,
    id_destinataire UUID NOT NULL REFERENCES utilisateur,
    note            SMALLINT NOT NULL CHECK (note BETWEEN 1 AND 5),
    commentaire     TEXT CHECK (length(commentaire) <= 2000),
    date_avis       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (id_echange, id_auteur),                          -- RG20
    CHECK (id_auteur <> id_destinataire)
);
CREATE INDEX ix_avis_destinataire ON avis (id_destinataire, date_avis DESC);
