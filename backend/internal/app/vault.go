package app

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gitb2pic/codetroc/backend/internal/domain/exchange"
)

var (
	errVaultOff      = newErr(KindUnavailable, "coffre-non-configure", "Coffre indisponible", "Le coffre n'est pas configuré sur ce serveur.")
	errSecretGone    = newErr(KindGone, "secret-indisponible", "Secret indisponible", "Ce secret a été révoqué, a expiré, ou l'échange est clos.")
	errExchangeIdle  = conflict("echange-inactif", "Les secrets ne se partagent que sur un échange en cours ou en litige.")
	validSecretKinds = map[string]bool{"JETON": true, "CLE_API": true, "MOT_DE_PASSE": true, "AUTRE": true}
)

const maxSecretBytes = 8192

// SecretInput décrit un secret déposé par le demandeur (UC12).
type SecretInput struct {
	Label     string
	Kind      string
	Scope     string
	Value     string
	ExpiresAt time.Time
}

// Secret présente les métadonnées d'un secret ; la valeur n'en fait jamais partie.
type Secret struct {
	ID          uuid.UUID
	ExchangeID  uuid.UUID
	DepositorID uuid.UUID
	Label       string
	Kind        string
	Scope       *string
	Status      string // ACTIF, EXPIRE, REVOQUE
	ExpiresAt   time.Time
	RevokedAt   *time.Time
	CreatedAt   time.Time
}

// SecretAccess est une entrée du journal d'accès.
type SecretAccess struct {
	Action string
	UserID *uuid.UUID
	Pseudo *string
	IP     string
	At     time.Time
}

func secretAAD(exchangeID, secretID uuid.UUID) []byte {
	return append(exchangeID[:], secretID[:]...)
}

func parseIP(s string) *netip.Addr {
	a, err := netip.ParseAddr(s)
	if err != nil {
		return nil
	}
	return &a
}

func (in SecretInput) validate(now time.Time, maxLifetime time.Duration) error {
	switch {
	case utf8.RuneCountInString(strings.TrimSpace(in.Label)) == 0 || utf8.RuneCountInString(in.Label) > 100:
		return fieldInvalid("libelle", "1 à 100 caractères")
	case !validSecretKinds[in.Kind]:
		return fieldInvalid("type", "JETON, CLE_API, MOT_DE_PASSE ou AUTRE")
	case utf8.RuneCountInString(in.Scope) > 300:
		return fieldInvalid("portee", "300 caractères maximum")
	case in.Value == "" || len(in.Value) > maxSecretBytes:
		return fieldInvalid("valeur", "1 à 8192 octets")
	case !in.ExpiresAt.After(now) || in.ExpiresAt.After(now.Add(maxLifetime)):
		return fieldInvalid("date_expiration", "dans le futur, 30 jours au plus")
	}
	return nil
}

// ShareSecret chiffre et dépose un secret d'accès ; seul le demandeur dépose,
// sur un échange en cours ou en litige (RG19).
func (s *Service) ShareSecret(ctx context.Context, a Actor, exchangeID uuid.UUID, in SecretInput, ip string) (Secret, error) {
	if s.vault == nil {
		return Secret{}, errVaultOff
	}
	if err := in.validate(s.now(), s.settings.MaxSecretLifetime); err != nil {
		return Secret{}, err
	}
	id := uuid.New()
	err := s.tx(ctx, func(tx pgx.Tx) error {
		r, err := requireParty(ctx, tx, a, exchangeID, true)
		if err != nil {
			return err
		}
		if a.ID != r.Requester {
			return errAccessDenie
		}
		if r.Status != exchange.InProgress && r.Status != exchange.InDispute {
			return errExchangeIdle
		}
		sealed, err := s.vault.Seal([]byte(in.Value), secretAAD(r.ID, id))
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO secret (id_secret, id_echange, id_deposant, libelle, type_secret, portee,
			                    valeur_chiffree, nonce, dek_chiffree, dek_nonce, kek_version, date_expiration, date_creation)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
			id, r.ID, a.ID, strings.TrimSpace(in.Label), in.Kind, nilIfEmpty(strings.TrimSpace(in.Scope)),
			sealed.Ciphertext, sealed.Nonce, sealed.WrappedDEK, sealed.DEKNonce, sealed.KEKVersion, in.ExpiresAt, s.now()); err != nil {
			return err
		}
		return s.logAccess(ctx, tx, id, &a.ID, "CREATION", ip)
	})
	if err != nil {
		return Secret{}, err
	}
	return s.secretByID(ctx, id)
}

func (s *Service) logAccess(ctx context.Context, q querier, secretID uuid.UUID, userID *uuid.UUID, action, ip string) error {
	_, err := q.Exec(ctx, `INSERT INTO secret_journal (id_secret, id_utilisateur, action, ip, date_action) VALUES ($1, $2, $3, $4, $5)`,
		secretID, userID, action, parseIP(ip), s.now())
	return err
}

const vaultSelect = `
	SELECT id_secret, id_echange, id_deposant, libelle, type_secret, portee,
	       CASE WHEN revoque AND date_revocation >= date_expiration THEN 'EXPIRE'
	            WHEN revoque THEN 'REVOQUE'
	            WHEN date_expiration <= $1 THEN 'EXPIRE'
	            ELSE 'ACTIF' END,
	       date_expiration, date_revocation, date_creation
	FROM secret`

func (s *Service) secretByID(ctx context.Context, id uuid.UUID) (Secret, error) {
	rows, err := s.db.Query(ctx, vaultSelect+` WHERE id_secret = $2`, s.now(), id)
	if err != nil {
		return Secret{}, err
	}
	sec, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByPos[Secret])
	return sec, notFoundIfNoRows(err)
}

// ListSecrets renvoie les métadonnées des secrets de l'échange aux parties.
func (s *Service) ListSecrets(ctx context.Context, a Actor, exchangeID uuid.UUID) ([]Secret, error) {
	if _, err := requireParty(ctx, s.db, a, exchangeID, false); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, vaultSelect+` WHERE id_echange = $2 ORDER BY date_creation DESC, id_secret`, s.now(), exchangeID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Secret])
}

type secretRow struct {
	ExchangeID uuid.UUID
	Depositor  uuid.UUID
	Revoked    bool
	ExpiresAt  time.Time
	Sealed     Sealed
	Exchange   exRow
}

// loadSecret charge un secret et son échange ; 404 pour qui n'en est pas partie.
func (s *Service) loadSecret(ctx context.Context, q querier, a Actor, id uuid.UUID, lock string) (secretRow, error) {
	var r secretRow
	var version *int
	err := q.QueryRow(ctx, `
		SELECT id_echange, id_deposant, revoque, date_expiration, valeur_chiffree, nonce, dek_chiffree, dek_nonce, kek_version
		FROM secret WHERE id_secret = $1 `+lock, id).
		Scan(&r.ExchangeID, &r.Depositor, &r.Revoked, &r.ExpiresAt, &r.Sealed.Ciphertext, &r.Sealed.Nonce, &r.Sealed.WrappedDEK, &r.Sealed.DEKNonce, &version)
	if err != nil {
		return r, notFoundIfNoRows(err)
	}
	if version != nil {
		r.Sealed.KEKVersion = *version
	}
	r.Exchange, err = requireParty(ctx, q, a, r.ExchangeID, false)
	return r, err
}

func (r secretRow) available(now time.Time) bool {
	open := r.Exchange.Status == exchange.InProgress || r.Exchange.Status == exchange.InDispute
	return open && !r.Revoked && now.Before(r.ExpiresAt)
}

// refused journalise un refus opposé à une partie, puis renvoie l'erreur.
func (s *Service) refused(ctx context.Context, err error, secretID uuid.UUID, a Actor, ip string) error {
	var e *Error
	if errors.As(err, &e) && e.Kind != KindNotFound {
		if logErr := s.logAccess(ctx, s.db, secretID, &a.ID, "REFUS", ip); logErr != nil {
			return logErr
		}
	}
	return err
}

// RevealSecret déchiffre un secret pour le prestataire et journalise l'accès (RG19).
func (s *Service) RevealSecret(ctx context.Context, a Actor, id uuid.UUID, ip string) (string, error) {
	if s.vault == nil {
		return "", errVaultOff
	}
	var value []byte
	err := s.tx(ctx, func(tx pgx.Tx) error {
		r, err := s.loadSecret(ctx, tx, a, id, "FOR SHARE")
		if err != nil {
			return err
		}
		if a.ID != r.Exchange.Provider {
			return errAccessDenie
		}
		if !r.available(s.now()) {
			return errSecretGone
		}
		if value, err = s.vault.Open(r.Sealed, secretAAD(r.ExchangeID, id)); err != nil {
			return err
		}
		return s.logAccess(ctx, tx, id, &a.ID, "REVELATION", ip)
	})
	if err != nil {
		return "", s.refused(ctx, err, id, a, ip)
	}
	return string(value), nil
}

const wipeMaterial = `revoque = true, date_revocation = $1,
	valeur_chiffree = NULL, nonce = NULL, dek_chiffree = NULL, dek_nonce = NULL`

// RevokeSecret révoque un secret et efface sa matière chiffrée ; réservé au déposant.
func (s *Service) RevokeSecret(ctx context.Context, a Actor, id uuid.UUID, ip string) (Secret, error) {
	err := s.tx(ctx, func(tx pgx.Tx) error {
		r, err := s.loadSecret(ctx, tx, a, id, "FOR UPDATE")
		if err != nil {
			return err
		}
		if a.ID != r.Depositor {
			return errAccessDenie
		}
		if r.Revoked {
			return nil
		}
		if _, err := tx.Exec(ctx, `UPDATE secret SET `+wipeMaterial+` WHERE id_secret = $2`, s.now(), id); err != nil {
			return err
		}
		return s.logAccess(ctx, tx, id, &a.ID, "REVOCATION", ip)
	})
	if err != nil {
		return Secret{}, s.refused(ctx, err, id, a, ip)
	}
	return s.secretByID(ctx, id)
}

// SecretJournal renvoie le journal d'accès d'un secret aux parties.
func (s *Service) SecretJournal(ctx context.Context, a Actor, id uuid.UUID) ([]SecretAccess, error) {
	if _, err := s.loadSecret(ctx, s.db, a, id, ""); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT j.action, j.id_utilisateur, u.pseudo, COALESCE(host(j.ip), ''), j.date_action
		FROM secret_journal j LEFT JOIN utilisateur u USING (id_utilisateur)
		WHERE j.id_secret = $1 ORDER BY j.id_entree`, id)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[SecretAccess])
}

// revokeExchangeSecrets révoque les secrets actifs d'un échange qui se clôt (RG19).
func (s *Service) revokeExchangeSecrets(ctx context.Context, tx pgx.Tx, exchangeID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		WITH x AS (UPDATE secret SET `+wipeMaterial+` WHERE id_echange = $2 AND NOT revoque RETURNING id_secret)
		INSERT INTO secret_journal (id_secret, action, date_action) SELECT id_secret, 'REVOCATION', $1 FROM x`,
		s.now(), exchangeID)
	return err
}

// ExpireSecrets révoque les secrets arrivés à expiration ; appelé par le worker.
func (s *Service) ExpireSecrets(ctx context.Context) (int, error) {
	tag, err := s.db.Exec(ctx, `
		WITH x AS (UPDATE secret SET `+wipeMaterial+` WHERE NOT revoque AND date_expiration <= $1 RETURNING id_secret)
		INSERT INTO secret_journal (id_secret, action, date_action) SELECT id_secret, 'EXPIRATION', $1 FROM x`, s.now())
	return int(tag.RowsAffected()), err
}
