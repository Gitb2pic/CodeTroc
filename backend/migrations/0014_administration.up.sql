-- F14 — suppression de compte par anonymisation (D13).
-- Empreinte SHA-256 de l'identifiant GitHub des comptes supprimés : une
-- réinscription crée un compte neuf, mais sans second bonus de bienvenue.
CREATE TABLE github_supprime (
    empreinte BYTEA PRIMARY KEY,
    date_suppression TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE utilisateur ADD COLUMN date_suppression TIMESTAMPTZ;
