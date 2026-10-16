package app

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gitb2pic/codetroc/backend/internal/platform/token"
)

const oauthStateTTL = 10 * time.Minute

// User est un membre de CodeTroc.
type User struct {
	ID          uuid.UUID
	Pseudo      string
	Email       *string
	GitHubLogin string // vide pour un compte email sans GitHub lié (F18)
	HasGitHub   bool
	// EmailVerified : adresse confirmée par lien (comptes email, F18).
	EmailVerified bool
	Bio           *string
	Role          Role
	Status        string
	Reputation    float64
	JoinedAt      time.Time
}

// Session est le résultat d'une connexion ou d'un rafraîchissement.
type Session struct {
	User           User
	AccessToken    string
	AccessExpires  time.Time
	RefreshToken   string
	RefreshExpires time.Time
	// Durées de validité restantes, utiles pour expires_in et Max-Age.
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

var (
	errOAuthState    = newErr(KindUnauthenticated, "etat-oauth-invalide", "Connexion refusée", "Le paramètre state est absent, expiré ou déjà utilisé. Relance la connexion.")
	errGitHubRefused = newErr(KindUnauthenticated, "github-refuse", "Connexion refusée", "GitHub n'a pas validé l'autorisation.")
	errNoGitHub      = newErr(KindUnavailable, "github-non-configure", "Connexion GitHub indisponible", "La connexion GitHub n'est pas configurée sur ce serveur.")
	errSuspended     = newErr(KindForbidden, "compte-suspendu", "Compte suspendu", "Ce compte est suspendu ou supprimé.")
	errTooRecent     = newErr(KindForbidden, "compte-github-trop-recent", "Compte GitHub trop récent", "Le compte GitHub ne remplit pas l'ancienneté minimale.")
	errRefreshReused = newErr(KindUnauthenticated, "jeton-reutilise", "Session révoquée", "Ce jeton a déjà servi : toutes les sessions liées ont été fermées.")
)

const userColumns = `id_utilisateur, pseudo, email, COALESCE(github_login, ''), github_id IS NOT NULL, email_verifie_le IS NOT NULL, bio, role, statut_utilisateur, score_reputation::float8, date_inscription`

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Pseudo, &u.Email, &u.GitHubLogin, &u.HasGitHub, &u.EmailVerified, &u.Bio, &u.Role, &u.Status, &u.Reputation, &u.JoinedAt)
	return u, err
}

// StartLogin crée un state OAuth à usage unique et renvoie l'URL GitHub.
func (s *Service) StartLogin(ctx context.Context) (authURL, state string, err error) {
	if s.oauth == nil {
		return "", "", errNoGitHub
	}
	state, hash, err := token.NewOpaque()
	if err != nil {
		return "", "", err
	}
	now := s.now()
	err = s.tx(ctx, func(tx pgx.Tx) error {
		// Ménage opportuniste des states expirés.
		if _, err := tx.Exec(ctx, `DELETE FROM oauth_state WHERE expire_le < $1`, now); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO oauth_state (hash, expire_le) VALUES ($1, $2)`, hash, now.Add(oauthStateTTL))
		return err
	})
	if err != nil {
		return "", "", err
	}
	return s.oauth.AuthorizeURL(state), state, nil
}

// CompleteLogin termine la connexion : le state reçu doit égaler celui du
// cookie du navigateur et exister en base (il est consommé).
func (s *Service) CompleteLogin(ctx context.Context, stateParam, stateCookie, code string) (Session, error) {
	if s.oauth == nil {
		return Session{}, errNoGitHub
	}
	if stateParam == "" || subtle.ConstantTimeCompare([]byte(stateParam), []byte(stateCookie)) != 1 {
		return Session{}, errOAuthState
	}
	// Un state de liaison (F18) porte le membre qui l'a demandé.
	var linkUser *uuid.UUID
	err := s.db.QueryRow(ctx, `DELETE FROM oauth_state WHERE hash = $1 AND expire_le >= $2 RETURNING id_utilisateur`,
		token.HashOpaque(stateParam), s.now()).Scan(&linkUser)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, errOAuthState
	}
	if err != nil {
		return Session{}, err
	}

	id, err := s.oauth.Exchange(ctx, code)
	if err != nil {
		// Cause côté serveur (secret client erroné, code expiré…) : seul le journal la voit.
		slog.WarnContext(ctx, "connexion GitHub refusée", slog.String("error", err.Error()))
		return Session{}, errGitHubRefused
	}
	if min := s.settings.MinGitHubAccountAge; min > 0 && s.now().Sub(id.CreatedAt) < min {
		return Session{}, errTooRecent
	}
	if linkUser != nil {
		return s.linkGitHub(ctx, *linkUser, id)
	}

	var sess Session
	err = s.tx(ctx, func(tx pgx.Tx) error {
		u, created, err := upsertUser(ctx, tx, id)
		if err != nil {
			return err
		}
		if u.Status != "ACTIF" {
			return errSuspended
		}
		if created {
			if err := s.onUserCreated(ctx, tx, u.ID); err != nil {
				return err
			}
		}
		sess, err = s.newSession(ctx, tx, u, uuid.New())
		return err
	})
	return sess, err
}

// upsertUser crée l'utilisateur à sa première connexion, sinon met à jour son
// login et son e-mail GitHub. Sûr en cas de connexions simultanées.
func upsertUser(ctx context.Context, tx pgx.Tx, id GitHubIdentity) (User, bool, error) {
	var email *string
	if id.Email != "" {
		email = &id.Email
	}
	pseudo := truncate(id.Login, 50)
	for attempt := 0; attempt < 5; attempt++ {
		u, err := scanUser(tx.QueryRow(ctx, `
			INSERT INTO utilisateur (pseudo, email, github_id, github_login, github_cree_le)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT DO NOTHING
			RETURNING `+userColumns, pseudo, email, id.ID, id.Login, id.CreatedAt))
		if err == nil {
			return u, true, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return User{}, false, err
		}
		// Conflit : utilisateur déjà connu, ou pseudo / e-mail déjà pris.
		u, err = scanUser(tx.QueryRow(ctx, `
			UPDATE utilisateur SET
			  github_login = $2,
			  email = CASE WHEN $3::text IS NULL OR EXISTS (
			            SELECT 1 FROM utilisateur o WHERE o.email = $3 AND o.github_id <> $1)
			          THEN email ELSE $3 END
			WHERE github_id = $1
			RETURNING `+userColumns, id.ID, id.Login, email))
		if err == nil {
			return u, false, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return User{}, false, err
		}
		var pseudoTaken, emailTaken bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM utilisateur WHERE pseudo = $1),
		                                  EXISTS (SELECT 1 FROM utilisateur WHERE email = $2)`, pseudo, email).Scan(&pseudoTaken, &emailTaken); err != nil {
			return User{}, false, err
		}
		if pseudoTaken {
			pseudo = truncate(id.Login, 45) + "-" + randomSuffix()
		}
		if emailTaken {
			email = nil
		}
	}
	return User{}, false, fmt.Errorf("création de l'utilisateur GitHub %d : conflits répétés", id.ID)
}

// newSession émet un jeton d'accès et un jeton de rafraîchissement dans la famille donnée.
func (s *Service) newSession(ctx context.Context, tx pgx.Tx, u User, family uuid.UUID) (Session, error) {
	access, accessExp, err := s.tokens.Issue(u.ID)
	if err != nil {
		return Session{}, err
	}
	refresh, hash, err := token.NewOpaque()
	if err != nil {
		return Session{}, err
	}
	refreshExp := s.now().Add(s.settings.RefreshTTL)
	_, err = tx.Exec(ctx, `
		INSERT INTO jeton_rafraichissement (hash, famille, id_utilisateur, cree_le, expire_le)
		VALUES ($1, $2, $3, $4, $5)`, hash, family, u.ID, s.now(), refreshExp)
	if err != nil {
		return Session{}, err
	}
	now := s.now()
	return Session{User: u, AccessToken: access, AccessExpires: accessExp, RefreshToken: refresh, RefreshExpires: refreshExp,
		AccessTTL: accessExp.Sub(now), RefreshTTL: refreshExp.Sub(now)}, nil
}

// Refresh fait tourner le jeton de rafraîchissement. Présenter un jeton déjà
// révoqué révoque toute sa famille.
func (s *Service) Refresh(ctx context.Context, refresh string) (Session, error) {
	var sess Session
	var outcome error
	err := s.tx(ctx, func(tx pgx.Tx) error {
		var (
			jetonID, family uuid.UUID
			expires         time.Time
			revoked         *time.Time
		)
		err := tx.QueryRow(ctx, `
			SELECT id_jeton, famille, expire_le, revoque_le
			FROM jeton_rafraichissement WHERE hash = $1 FOR UPDATE`, token.HashOpaque(refresh)).
			Scan(&jetonID, &family, &expires, &revoked)
		if errors.Is(err, pgx.ErrNoRows) {
			outcome = ErrUnauthenticated
			return nil
		}
		if err != nil {
			return err
		}
		if revoked != nil {
			outcome = errRefreshReused
			return revokeFamily(ctx, tx, family, s.now()) // la révocation est validée
		}
		if !expires.After(s.now()) {
			outcome = ErrUnauthenticated
			return nil
		}
		u, err := scanUser(tx.QueryRow(ctx, `SELECT `+userColumns+` FROM utilisateur u
			JOIN jeton_rafraichissement j USING (id_utilisateur) WHERE j.id_jeton = $1`, jetonID))
		if err != nil {
			return err
		}
		if u.Status != "ACTIF" {
			outcome = ErrUnauthenticated
			return revokeFamily(ctx, tx, family, s.now())
		}
		if _, err := tx.Exec(ctx, `UPDATE jeton_rafraichissement SET revoque_le = $2 WHERE id_jeton = $1`, jetonID, s.now()); err != nil {
			return err
		}
		sess, err = s.newSession(ctx, tx, u, family)
		return err
	})
	if err != nil {
		return Session{}, err
	}
	return sess, outcome
}

// Logout révoque la famille du jeton présenté. Sans effet si le jeton est inconnu.
func (s *Service) Logout(ctx context.Context, refresh string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE jeton_rafraichissement SET revoque_le = $2
		WHERE revoque_le IS NULL AND famille = (SELECT famille FROM jeton_rafraichissement WHERE hash = $1)`,
		token.HashOpaque(refresh), s.now())
	return err
}

func revokeFamily(ctx context.Context, tx pgx.Tx, family uuid.UUID, now time.Time) error {
	_, err := tx.Exec(ctx, `UPDATE jeton_rafraichissement SET revoque_le = $2 WHERE famille = $1 AND revoque_le IS NULL`, family, now)
	return err
}

// Authenticate vérifie un jeton d'accès et relit le rôle et le statut en base,
// pour qu'une suspension ou un changement de rôle s'applique immédiatement.
func (s *Service) Authenticate(ctx context.Context, access string) (Actor, error) {
	id, err := s.tokens.Parse(access)
	if err != nil {
		return Actor{}, ErrUnauthenticated
	}
	var role Role
	var status string
	err = s.db.QueryRow(ctx, `SELECT role, statut_utilisateur FROM utilisateur WHERE id_utilisateur = $1`, id).Scan(&role, &status)
	if err != nil || status != "ACTIF" {
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return Actor{}, err
		}
		return Actor{}, ErrUnauthenticated
	}
	return Actor{ID: id, Role: role}, nil
}

// Me renvoie le profil complet de l'acteur.
func (s *Service) Me(ctx context.Context, a Actor) (User, error) {
	u, err := scanUser(s.db.QueryRow(ctx, `SELECT `+userColumns+` FROM utilisateur WHERE id_utilisateur = $1`, a.ID))
	return u, notFoundIfNoRows(err)
}

func randomSuffix() string {
	b := make([]byte, 2)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
