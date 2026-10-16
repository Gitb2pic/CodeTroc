package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gitb2pic/codetroc/backend/internal/platform/password"
	"github.com/gitb2pic/codetroc/backend/internal/platform/token"
)

// F18 — comptes par email et mot de passe. Un compte email confirme son
// adresse par lien avant de pouvoir se connecter ; le bonus de bienvenue est
// versé à la confirmation. Sans GitHub lié, le membre est demandeur seulement
// (annonces, demandes de packs) : proposer une offre ou candidater exige de
// lier un compte GitHub, qui reste la preuve d'identité des livraisons.

const (
	confirmTTL = 48 * time.Hour
	resetTTL   = time.Hour
	tokConfirm = "CONFIRMATION"
	tokReset   = "REINITIALISATION"
)

var (
	errEmailDisabled  = newErr(KindUnavailable, "inscription-email-indisponible", "Inscription par email indisponible", "L'envoi d'emails n'est pas configuré sur ce serveur : utilise la connexion GitHub.")
	errBadCredentials = newErr(KindUnauthenticated, "identifiants-invalides", "Connexion refusée", "Email ou mot de passe incorrect.")
	errNotVerified    = newErr(KindForbidden, "email-non-confirme", "Adresse non confirmée", "Confirme ton adresse avec le lien reçu par email, ou demande un nouveau lien.")
	errBadEmailToken  = newErr(KindInvalid, "lien-invalide", "Lien invalide", "Ce lien est invalide, expiré ou déjà utilisé : demande-en un nouveau.")
	errEmailFormat    = invalid("donnees-invalides", "email : adresse invalide")
	errWeakPassword   = invalid("mot-de-passe-faible", password.ErrWeak.Error())
	errGitHubRequired = forbidden("github-requis", "Lie ton compte GitHub (profil) pour proposer une offre ou candidater : les livraisons sont prouvées par GitHub.")
	errGitHubTaken    = conflict("github-deja-lie", "Ce compte GitHub est déjà associé à un autre membre.")
	errGitHubLinked   = conflict("github-deja-present", "Ton compte est déjà lié à GitHub.")
)

// normalizeEmail valide une adresse et la met en minuscules.
func normalizeEmail(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	a, err := mail.ParseAddress(raw)
	if err != nil || a.Address != raw || len(raw) > 254 || !strings.Contains(raw[strings.LastIndex(raw, "@")+1:], ".") {
		return "", errEmailFormat
	}
	return strings.ToLower(raw), nil
}

// RegisterInput décrit une inscription par email.
type RegisterInput struct {
	Email    string
	Password string
	Pseudo   string // facultatif : déduit de l'adresse sinon
}

// EmailAuthEnabled indique si l'inscription et la connexion par email sont ouvertes.
func (s *Service) EmailAuthEnabled() bool { return s.mailer != nil }

// GitHubAuthEnabled indique si la connexion GitHub est configurée.
func (s *Service) GitHubAuthEnabled() bool { return s.oauth != nil }

// Register crée un compte email non confirmé et envoie le lien de
// confirmation. La réponse ne révèle jamais si l'adresse est déjà inscrite :
// le titulaire d'une adresse connue reçoit un email « compte existant ».
func (s *Service) Register(ctx context.Context, in RegisterInput) error {
	if s.mailer == nil {
		return errEmailDisabled
	}
	email, err := normalizeEmail(in.Email)
	if err != nil {
		return err
	}
	if password.Check(in.Password) != nil {
		return errWeakPassword
	}
	if strings.EqualFold(in.Password, email) {
		return errWeakPassword
	}
	pseudo := strings.TrimSpace(in.Pseudo)
	if pseudo == "" {
		pseudo = strings.Map(func(r rune) rune {
			if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
				return r
			}
			return '-'
		}, email[:strings.Index(email, "@")])
		pseudo = strings.TrimLeft(pseudo, "-_")
		if len(pseudo) < 3 {
			pseudo = "membre-" + pseudo
		}
		pseudo = strings.TrimRight(truncate(pseudo, 50), "-_")
	} else if !validPseudo.MatchString(pseudo) {
		return fieldInvalid("pseudo", "3 à 50 caractères : lettres, chiffres, - et _, sans tiret initial")
	}
	hash, err := password.Hash(in.Password)
	if err != nil {
		return err
	}

	var existing bool
	var tok string
	err = s.tx(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM utilisateur WHERE lower(email) = $1)`, email).Scan(&existing); err != nil || existing {
			return err
		}
		var id uuid.UUID
		for attempt := 0; ; attempt++ {
			err := tx.QueryRow(ctx, `
				INSERT INTO utilisateur (pseudo, email, mot_de_passe_hash, date_inscription)
				VALUES ($1, $2, $3, $4) ON CONFLICT (pseudo) DO NOTHING RETURNING id_utilisateur`,
				pseudo, email, hash, s.now()).Scan(&id)
			if err == nil {
				break
			}
			if !errors.Is(err, pgx.ErrNoRows) || attempt == 4 {
				return err
			}
			pseudo = truncate(pseudo, 45) + "-" + randomSuffix()
		}
		// Compte de crédits sans bonus : il est versé à la confirmation.
		if _, err := tx.Exec(ctx, `INSERT INTO compte (type_compte, id_utilisateur) VALUES ('UTILISATEUR', $1)`, id); err != nil {
			return err
		}
		if err := recordTransition(ctx, tx, "UTILISATEUR", id, "", "ACTIF", &id, "inscription par email", s.now()); err != nil {
			return err
		}
		tok, err = s.newEmailToken(ctx, tx, id, tokConfirm, confirmTTL)
		return err
	})
	if err != nil {
		return err
	}
	if existing {
		return s.send(ctx, Email{To: email, Subject: "Ton compte CodeTroc existe déjà",
			Text: "Bonjour,\n\nQuelqu'un (peut-être toi) a voulu créer un compte CodeTroc avec cette adresse, qui en a déjà un.\n\n" +
				"Connecte-toi : " + s.appURL + "/\nMot de passe oublié : " + s.appURL + "/?vue=oubli\n\nSi ce n'était pas toi, ignore ce message.\n"})
	}
	return s.sendConfirmation(ctx, email, tok)
}

func (s *Service) newEmailToken(ctx context.Context, tx pgx.Tx, userID uuid.UUID, kind string, ttl time.Duration) (string, error) {
	tok, hash, err := token.NewOpaque()
	if err != nil {
		return "", err
	}
	// Un nouveau lien invalide les précédents du même type.
	if _, err := tx.Exec(ctx, `UPDATE jeton_email SET utilise_le = $3 WHERE id_utilisateur = $1 AND type_jeton = $2 AND utilise_le IS NULL`,
		userID, kind, s.now()); err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, `INSERT INTO jeton_email (hash, id_utilisateur, type_jeton, cree_le, expire_le) VALUES ($1, $2, $3, $4, $5)`,
		hash, userID, kind, s.now(), s.now().Add(ttl))
	return tok, err
}

// send envoie un email ; un échec est journalisé sans être renvoyé au client
// (l'utilisateur peut redemander un lien), pour ne rien révéler.
func (s *Service) send(ctx context.Context, m Email) error {
	if err := s.mailer.Send(ctx, m); err != nil {
		slog.ErrorContext(ctx, "envoi d'email impossible", slog.String("error", err.Error()))
	}
	return nil
}

func (s *Service) sendConfirmation(ctx context.Context, email, tok string) error {
	return s.send(ctx, Email{To: email, Subject: "Confirme ton adresse CodeTroc",
		Text: "Bonjour,\n\nConfirme ton adresse pour activer ton compte CodeTroc (et recevoir tes " +
			fmt.Sprint(s.settings.WelcomeBonus) + " crédits de bienvenue) :\n\n" + s.appURL + "/confirmer-email?jeton=" + tok +
			"\n\nCe lien est valable 48 heures. Si tu n'as pas créé de compte, ignore ce message.\n"})
}

// consumeEmailToken valide et consomme un jeton ; renvoie l'utilisateur.
func (s *Service) consumeEmailToken(ctx context.Context, tx pgx.Tx, raw, kind string) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `
		UPDATE jeton_email SET utilise_le = $3
		WHERE hash = $1 AND type_jeton = $2 AND utilise_le IS NULL AND expire_le > $3
		RETURNING id_utilisateur`, token.HashOpaque(raw), kind, s.now()).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return id, errBadEmailToken
	}
	return id, err
}

// ConfirmEmail confirme l'adresse et verse le bonus de bienvenue (une seule fois).
func (s *Service) ConfirmEmail(ctx context.Context, raw string) error {
	if raw == "" || len(raw) > 200 {
		return errBadEmailToken
	}
	return s.tx(ctx, func(tx pgx.Tx) error {
		id, err := s.consumeEmailToken(ctx, tx, raw, tokConfirm)
		if err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE utilisateur SET email_verifie_le = $2 WHERE id_utilisateur = $1 AND email_verifie_le IS NULL AND statut_utilisateur = 'ACTIF'`, id, s.now())
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		return s.grantWelcomeBonus(ctx, tx, id)
	})
}

// ResendConfirmation renvoie un lien de confirmation si l'adresse attend une
// confirmation ; sans effet visible sinon.
func (s *Service) ResendConfirmation(ctx context.Context, rawEmail string) error {
	if s.mailer == nil {
		return errEmailDisabled
	}
	email, err := normalizeEmail(rawEmail)
	if err != nil {
		return err
	}
	var tok string
	err = s.tx(ctx, func(tx pgx.Tx) error {
		var id uuid.UUID
		err := tx.QueryRow(ctx, `
			SELECT id_utilisateur FROM utilisateur
			WHERE lower(email) = $1 AND mot_de_passe_hash IS NOT NULL AND email_verifie_le IS NULL AND statut_utilisateur = 'ACTIF'`, email).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		tok, err = s.newEmailToken(ctx, tx, id, tokConfirm, confirmTTL)
		return err
	})
	if err != nil || tok == "" {
		return err
	}
	return s.sendConfirmation(ctx, email, tok)
}

// LoginWithPassword ouvre une session pour un compte email confirmé. Le
// temps de réponse est le même que l'adresse existe ou non.
func (s *Service) LoginWithPassword(ctx context.Context, rawEmail, pw string) (Session, error) {
	if s.mailer == nil {
		return Session{}, errEmailDisabled
	}
	email, err := normalizeEmail(rawEmail)
	if err != nil || len(pw) > 4*password.MaxLength {
		password.VerifyDummy(pw)
		return Session{}, errBadCredentials
	}
	var id uuid.UUID
	var hash *string
	err = s.db.QueryRow(ctx, `SELECT id_utilisateur, mot_de_passe_hash FROM utilisateur WHERE lower(email) = $1`, email).Scan(&id, &hash)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && hash == nil) {
		password.VerifyDummy(pw)
		return Session{}, errBadCredentials
	}
	if err != nil {
		return Session{}, err
	}
	ok, err := password.Verify(pw, *hash)
	if err != nil || !ok {
		return Session{}, errBadCredentials
	}
	var sess Session
	err = s.tx(ctx, func(tx pgx.Tx) error {
		u, err := scanUser(tx.QueryRow(ctx, `SELECT `+userColumns+` FROM utilisateur WHERE id_utilisateur = $1`, id))
		if err != nil {
			return err
		}
		if u.Status != "ACTIF" {
			return errSuspended
		}
		if !u.EmailVerified {
			return errNotVerified
		}
		sess, err = s.newSession(ctx, tx, u, uuid.New())
		return err
	})
	return sess, err
}

// RequestPasswordReset envoie un lien de réinitialisation si l'adresse
// correspond à un compte email ; la réponse est identique sinon.
func (s *Service) RequestPasswordReset(ctx context.Context, rawEmail string) error {
	if s.mailer == nil {
		return errEmailDisabled
	}
	email, err := normalizeEmail(rawEmail)
	if err != nil {
		return err
	}
	var tok string
	err = s.tx(ctx, func(tx pgx.Tx) error {
		var id uuid.UUID
		err := tx.QueryRow(ctx, `
			SELECT id_utilisateur FROM utilisateur
			WHERE lower(email) = $1 AND mot_de_passe_hash IS NOT NULL AND statut_utilisateur = 'ACTIF'`, email).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		tok, err = s.newEmailToken(ctx, tx, id, tokReset, resetTTL)
		return err
	})
	if err != nil || tok == "" {
		return err
	}
	return s.send(ctx, Email{To: email, Subject: "Réinitialise ton mot de passe CodeTroc",
		Text: "Bonjour,\n\nPour choisir un nouveau mot de passe :\n\n" + s.appURL + "/reinitialiser-mot-de-passe?jeton=" + tok +
			"\n\nCe lien est valable 1 heure. Si tu n'as rien demandé, ignore ce message : ton mot de passe ne change pas.\n"})
}

// ResetPassword change le mot de passe et ferme toutes les sessions du compte.
// Le lien prouve la possession de l'adresse : il la confirme aussi.
func (s *Service) ResetPassword(ctx context.Context, raw, pw string) error {
	if raw == "" || len(raw) > 200 {
		return errBadEmailToken
	}
	if password.Check(pw) != nil {
		return errWeakPassword
	}
	hash, err := password.Hash(pw)
	if err != nil {
		return err
	}
	return s.tx(ctx, func(tx pgx.Tx) error {
		id, err := s.consumeEmailToken(ctx, tx, raw, tokReset)
		if err != nil {
			return err
		}
		var wasVerified bool
		if err := tx.QueryRow(ctx, `SELECT email_verifie_le IS NOT NULL FROM utilisateur WHERE id_utilisateur = $1`, id).Scan(&wasVerified); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE utilisateur SET mot_de_passe_hash = $2, email_verifie_le = COALESCE(email_verifie_le, $3)
			WHERE id_utilisateur = $1`, id, hash, s.now()); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE jeton_rafraichissement SET revoque_le = $2 WHERE id_utilisateur = $1 AND revoque_le IS NULL`, id, s.now()); err != nil {
			return err
		}
		if !wasVerified {
			return s.grantWelcomeBonus(ctx, tx, id)
		}
		return nil
	})
}

// StartGitHubLink prépare la liaison d'un compte GitHub au compte connecté :
// le state OAuth porte l'utilisateur, le retour GitHub lie au lieu de créer.
func (s *Service) StartGitHubLink(ctx context.Context, a Actor) (authURL, state string, err error) {
	if s.oauth == nil {
		return "", "", errNoGitHub
	}
	var has bool
	if err := s.db.QueryRow(ctx, `SELECT github_id IS NOT NULL FROM utilisateur WHERE id_utilisateur = $1`, a.ID).Scan(&has); err != nil {
		return "", "", notFoundIfNoRows(err)
	}
	if has {
		return "", "", errGitHubLinked
	}
	state, hash, err := token.NewOpaque()
	if err != nil {
		return "", "", err
	}
	if _, err := s.db.Exec(ctx, `INSERT INTO oauth_state (hash, expire_le, id_utilisateur) VALUES ($1, $2, $3)`,
		hash, s.now().Add(oauthStateTTL), a.ID); err != nil {
		return "", "", err
	}
	return s.oauth.AuthorizeURL(state), state, nil
}

// linkGitHub associe l'identité GitHub au compte qui a lancé la liaison.
func (s *Service) linkGitHub(ctx context.Context, userID uuid.UUID, id GitHubIdentity) (Session, error) {
	var sess Session
	err := s.tx(ctx, func(tx pgx.Tx) error {
		var taken bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM utilisateur WHERE github_id = $1 AND id_utilisateur <> $2)`, id.ID, userID).Scan(&taken); err != nil {
			return err
		}
		if taken {
			return errGitHubTaken
		}
		u, err := scanUser(tx.QueryRow(ctx, `
			UPDATE utilisateur SET github_id = $2, github_login = $3, github_cree_le = $4
			WHERE id_utilisateur = $1 AND github_id IS NULL AND statut_utilisateur = 'ACTIF'
			RETURNING `+userColumns, userID, id.ID, id.Login, id.CreatedAt))
		if errors.Is(err, pgx.ErrNoRows) {
			return errGitHubLinked
		}
		if err != nil {
			return err
		}
		if err := recordTransition(ctx, tx, "UTILISATEUR", userID, "", "GITHUB_LIE", &userID, id.Login, s.now()); err != nil {
			return err
		}
		sess, err = s.newSession(ctx, tx, u, uuid.New())
		return err
	})
	return sess, err
}

// requireGitHub réserve une action aux membres dont le compte GitHub est lié.
func (s *Service) requireGitHub(ctx context.Context, a Actor) error {
	var has bool
	if err := s.db.QueryRow(ctx, `SELECT github_id IS NOT NULL FROM utilisateur WHERE id_utilisateur = $1`, a.ID).Scan(&has); err != nil {
		return notFoundIfNoRows(err)
	}
	if !has {
		return errGitHubRequired
	}
	return nil
}
