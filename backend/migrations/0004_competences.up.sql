-- F3 — compétences des membres.
CREATE TYPE niveau_competence AS ENUM ('DEBUTANT', 'INTERMEDIAIRE', 'CONFIRME', 'EXPERT');

CREATE TABLE competence (
    id_competence      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    libelle_competence VARCHAR(60) NOT NULL UNIQUE,
    categorie          VARCHAR(40) NOT NULL
);

CREATE TABLE posseder (
    id_utilisateur UUID NOT NULL REFERENCES utilisateur ON DELETE CASCADE,
    id_competence  UUID NOT NULL REFERENCES competence,
    niveau         niveau_competence NOT NULL,
    PRIMARY KEY (id_utilisateur, id_competence)
);
CREATE INDEX ix_posseder_competence ON posseder (id_competence, niveau);

INSERT INTO competence (libelle_competence, categorie) VALUES
  ('Docker', 'devops'), ('GitHub Actions', 'devops'), ('Kubernetes', 'devops'),
  ('Terraform', 'devops'), ('AWS', 'devops'), ('Prometheus', 'devops'), ('Bash', 'devops'),
  ('Node.js', 'back'), ('Python', 'back'), ('FastAPI', 'back'), ('Go', 'back'),
  ('Rust', 'back'), ('Postgres', 'donnees'), ('Prisma', 'donnees'),
  ('React', 'front'), ('Next.js', 'front'), ('TypeScript', 'front'),
  ('Playwright', 'qualite'), ('Tests unitaires', 'qualite'), ('Sécurité', 'securite');
