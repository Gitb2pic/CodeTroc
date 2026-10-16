-- F16 — cadre Scrum d'un échange : projet, sprints, stories, avenants de
-- planning et file des appels GitHub (outbox).
CREATE TYPE statut_story   AS ENUM ('BACKLOG', 'A_FAIRE', 'EN_COURS', 'EN_REVUE', 'TERMINE');
CREATE TYPE statut_sprint  AS ENUM ('PLANIFIE', 'ACTIF', 'TERMINE', 'ANNULE');
CREATE TYPE statut_synchro AS ENUM ('INACTIVE', 'ACTIVE', 'PERMISSION_MANQUANTE', 'ERREUR');
CREATE TYPE statut_avenant AS ENUM ('PROPOSE', 'ACCEPTE', 'REFUSE', 'RETIRE');

-- D20 : durée de sprint choisie dans l'offre (et l'annonce, F17), figée dans l'échange.
ALTER TABLE offre   ADD COLUMN duree_sprint_jours SMALLINT NOT NULL DEFAULT 7 CHECK (duree_sprint_jours IN (7, 14));
ALTER TABLE echange ADD COLUMN duree_sprint_jours SMALLINT NOT NULL DEFAULT 7 CHECK (duree_sprint_jours IN (7, 14));

-- 1-1 avec l'échange ; créé à l'acceptation (R-S1).
CREATE TABLE projet_scrum (
    id_echange         UUID PRIMARY KEY REFERENCES echange,
    template           VARCHAR(40) NOT NULL DEFAULT 'scrum-standard-v1',
    objectif_produit   TEXT NOT NULL DEFAULT '' CHECK (length(objectif_produit) <= 2000),
    definition_of_done JSONB NOT NULL CHECK (jsonb_typeof(definition_of_done) = 'array'),
    statut_synchro     statut_synchro NOT NULL DEFAULT 'INACTIVE',
    detail_synchro     TEXT,
    date_creation      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Jalon → GitHub : un milestone par jalon.
ALTER TABLE jalon ADD COLUMN github_milestone_no INTEGER;

CREATE TABLE sprint (
    id_sprint       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    id_echange      UUID NOT NULL REFERENCES projet_scrum,
    id_jalon        UUID NOT NULL REFERENCES jalon,
    numero          SMALLINT NOT NULL CHECK (numero BETWEEN 1 AND 50),
    debut_prevu     TIMESTAMPTZ NOT NULL,
    fin_prevue      TIMESTAMPTZ NOT NULL CHECK (fin_prevue > debut_prevu),
    debut_reel      TIMESTAMPTZ,
    fin_reelle      TIMESTAMPTZ,
    statut_sprint   statut_sprint NOT NULL DEFAULT 'PLANIFIE',
    objectif_sprint TEXT NOT NULL DEFAULT '' CHECK (length(objectif_sprint) <= 1000),
    github_issue_no INTEGER,
    retro_bien      TEXT CHECK (length(retro_bien) <= 2000),
    retro_ameliorer TEXT CHECK (length(retro_ameliorer) <= 2000),
    retro_actions   TEXT CHECK (length(retro_actions) <= 2000),
    UNIQUE (id_echange, numero)
);
CREATE INDEX ix_sprint_a_demarrer ON sprint (debut_prevu) WHERE statut_sprint = 'PLANIFIE';
CREATE INDEX ix_sprint_actif      ON sprint (fin_prevue)  WHERE statut_sprint = 'ACTIF';
-- R-S5 : un seul sprint actif par projet.
CREATE UNIQUE INDEX ux_sprint_actif ON sprint (id_echange) WHERE statut_sprint = 'ACTIF';

CREATE TABLE story (
    id_story        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    id_echange      UUID NOT NULL REFERENCES projet_scrum,
    id_jalon        UUID REFERENCES jalon,
    id_sprint       UUID REFERENCES sprint,  -- NULL = backlog produit
    titre           VARCHAR(160) NOT NULL,
    description     TEXT NOT NULL DEFAULT '' CHECK (length(description) <= 4000),
    criteres        JSONB NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(criteres) = 'array'),
    points          SMALLINT CHECK (points IN (0, 1, 2, 3, 5, 8, 13, 21)),
    priorite        INTEGER NOT NULL,
    statut_story    statut_story NOT NULL DEFAULT 'BACKLOG',
    github_issue_no INTEGER,
    date_creation   TIMESTAMPTZ NOT NULL DEFAULT now(),
    date_terminee   TIMESTAMPTZ,
    cree_par        UUID NOT NULL REFERENCES utilisateur,
    CHECK ((id_sprint IS NULL) = (statut_story = 'BACKLOG'))
);
CREATE INDEX ix_story_sprint ON story (id_sprint, priorite);
CREATE INDEX ix_story_projet ON story (id_echange, priorite);
CREATE INDEX ix_story_issue  ON story (github_issue_no) WHERE github_issue_no IS NOT NULL;

-- D21 / R-S10 : avenant de planning, proposé par une partie, accepté par l'autre.
CREATE TABLE avenant (
    id_avenant       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    id_echange       UUID NOT NULL REFERENCES projet_scrum,
    propose_par      UUID NOT NULL REFERENCES utilisateur,
    echeances        JSONB NOT NULL CHECK (jsonb_typeof(echeances) = 'array'),
    motif            TEXT NOT NULL DEFAULT '' CHECK (length(motif) <= 1000),
    statut_avenant   statut_avenant NOT NULL DEFAULT 'PROPOSE',
    date_proposition TIMESTAMPTZ NOT NULL DEFAULT now(),
    date_decision    TIMESTAMPTZ,
    decide_par       UUID REFERENCES utilisateur
);
CREATE UNIQUE INDEX ux_avenant_ouvert ON avenant (id_echange) WHERE statut_avenant = 'PROPOSE';

-- D19 : file des appels GitHub, remplie dans la transaction métier, vidée par le worker.
CREATE TABLE outbox_github (
    id             BIGSERIAL PRIMARY KEY,
    cle            VARCHAR(120) NOT NULL UNIQUE,
    id_echange     UUID NOT NULL REFERENCES echange,
    operation      VARCHAR(30) NOT NULL CHECK (operation IN
                   ('LABELS', 'MILESTONE', 'ISSUE_SPRINT', 'ISSUE_STORY', 'FERMER_ISSUE', 'MAJ_ISSUE')),
    charge         JSONB NOT NULL CHECK (jsonb_typeof(charge) = 'object'),
    tentatives     SMALLINT NOT NULL DEFAULT 0,
    prochain_essai TIMESTAMPTZ NOT NULL DEFAULT now(),
    erreur         TEXT,
    traite_le      TIMESTAMPTZ,
    cree_le        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_outbox_a_traiter ON outbox_github (prochain_essai, id) WHERE traite_le IS NULL;
