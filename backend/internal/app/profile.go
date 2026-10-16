package app

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/gitb2pic/codetroc/backend/internal/domain/catalog"
)

var (
	validPseudo  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{2,49}$`)
	validLevels  = map[string]bool{"DEBUTANT": true, "INTERMEDIAIRE": true, "CONFIRME": true, "EXPERT": true}
	errPseudo    = conflict("pseudo-pris", "Ce pseudo est déjà utilisé.")
	errUnknownSk = unprocessable("competence-inconnue", "Une des compétences n'existe pas.")
)

const maxSkillsPerUser = 30

// fieldInvalid construit une erreur 400 sur un champ.
func fieldInvalid(field, detail string) *Error {
	return invalid("donnees-invalides", field+" : "+detail)
}

// fromValidation traduit une erreur de validation du domaine.
func fromValidation(err error) error {
	var v *catalog.ValidationError
	if errors.As(err, &v) {
		return invalid(v.Code, v.Field+" : "+v.Detail)
	}
	return err
}

// ProfilePatch modifie le profil ; un champ nil reste inchangé, une bio vide est effacée.
type ProfilePatch struct {
	Pseudo *string
	Bio    *string
}

// UpdateProfile modifie le pseudo et la bio du membre (UC02).
func (s *Service) UpdateProfile(ctx context.Context, a Actor, p ProfilePatch) (User, error) {
	if p.Pseudo != nil && !validPseudo.MatchString(*p.Pseudo) {
		return User{}, fieldInvalid("pseudo", "3 à 50 caractères : lettres, chiffres, - et _, sans tiret initial")
	}
	var bio *string
	if p.Bio != nil {
		if utf8.RuneCountInString(*p.Bio) > 1000 {
			return User{}, fieldInvalid("bio", "1000 caractères maximum")
		}
		if b := strings.TrimSpace(*p.Bio); b != "" {
			bio = &b
		}
	}
	u, err := scanUser(s.db.QueryRow(ctx, `
		UPDATE utilisateur SET
		  pseudo = COALESCE($2, pseudo),
		  bio    = CASE WHEN $3 THEN $4 ELSE bio END
		WHERE id_utilisateur = $1
		RETURNING `+userColumns, a.ID, p.Pseudo, p.Bio != nil, bio))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return User{}, errPseudo
	}
	return u, notFoundIfNoRows(err)
}

// Skill est une compétence du référentiel.
type Skill struct {
	ID       uuid.UUID
	Label    string
	Category string
}

// Skills renvoie le référentiel des compétences.
func (s *Service) Skills(ctx context.Context) ([]Skill, error) {
	rows, err := s.db.Query(ctx, `SELECT id_competence, libelle_competence, categorie FROM competence ORDER BY categorie, libelle_competence`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Skill])
}

// SkillLevel associe une compétence à un niveau déclaré.
type SkillLevel struct {
	SkillID uuid.UUID
	Level   string
}

// UserSkill est une compétence déclarée par un membre.
type UserSkill struct {
	ID    uuid.UUID
	Label string
	Level string
}

// SetMySkills remplace d'un bloc les compétences du membre (RG06).
func (s *Service) SetMySkills(ctx context.Context, a Actor, skills []SkillLevel) ([]UserSkill, error) {
	if len(skills) > maxSkillsPerUser {
		return nil, fieldInvalid("competences", "30 compétences maximum")
	}
	seen := map[uuid.UUID]bool{}
	for _, sk := range skills {
		if !validLevels[sk.Level] {
			return nil, fieldInvalid("niveau", "DEBUTANT, INTERMEDIAIRE, CONFIRME ou EXPERT")
		}
		if seen[sk.SkillID] {
			return nil, fieldInvalid("competences", "compétence en double")
		}
		seen[sk.SkillID] = true
	}
	err := s.tx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM posseder WHERE id_utilisateur = $1`, a.ID); err != nil {
			return err
		}
		for _, sk := range skills {
			tag, err := tx.Exec(ctx, `
				INSERT INTO posseder (id_utilisateur, id_competence, niveau)
				SELECT $1, id_competence, $3 FROM competence WHERE id_competence = $2`, a.ID, sk.SkillID, sk.Level)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return errUnknownSk
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.userSkills(ctx, a.ID)
}

func (s *Service) userSkills(ctx context.Context, userID uuid.UUID) ([]UserSkill, error) {
	rows, err := s.db.Query(ctx, `
		SELECT c.id_competence, c.libelle_competence, p.niveau
		FROM posseder p JOIN competence c USING (id_competence)
		WHERE p.id_utilisateur = $1 ORDER BY c.libelle_competence`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[UserSkill])
}

// PublicProfile est la fiche visible de tous (sans e-mail).
type PublicProfile struct {
	ID                 uuid.UUID
	Pseudo             string
	GitHubLogin        string
	Bio                *string
	Reputation         float64
	JoinedAt           time.Time
	Skills             []UserSkill
	VerifiedDeliveries int
	ReviewsCount       int
}

// PublicProfile renvoie le profil public d'un membre (UC05).
func (s *Service) PublicProfile(ctx context.Context, id uuid.UUID) (PublicProfile, error) {
	var p PublicProfile
	err := s.db.QueryRow(ctx, `
		SELECT u.id_utilisateur, u.pseudo, COALESCE(u.github_login, ''), u.bio, u.score_reputation::float8, u.date_inscription,
		       (`+verifiedDeliveriesSQL+`), (`+reviewsCountSQL+`)
		FROM utilisateur u
		WHERE u.id_utilisateur = $1 AND u.statut_utilisateur <> 'SUPPRIME'`, id).
		Scan(&p.ID, &p.Pseudo, &p.GitHubLogin, &p.Bio, &p.Reputation, &p.JoinedAt, &p.VerifiedDeliveries, &p.ReviewsCount)
	if err != nil {
		return PublicProfile{}, notFoundIfNoRows(err)
	}
	p.Skills, err = s.userSkills(ctx, id)
	return p, err
}
