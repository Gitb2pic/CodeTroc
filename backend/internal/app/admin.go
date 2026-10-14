package app

import (
	"context"
	"crypto/rand"
	"math"
	"math/big"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	errSelfAdmin   = unprocessable("auto-modification", "Un administrateur ne modifie ni son propre rôle ni son propre statut.")
	errDeletedUser = conflict("compte-supprime", "Ce compte est supprimé.")
	errBusyAccount = conflict("echanges-en-cours", "Termine ou annule tes échanges en cours avant de supprimer ton compte.")
	validRoles     = map[Role]bool{RoleMember: true, RoleArbiter: true, RoleAdmin: true}
)

// AdminPatch modifie le rôle et/ou le statut d'un membre ; nil = inchangé.
type AdminPatch struct {
	Role   *Role
	Status *string // ACTIF ou SUSPENDU ; la suppression passe par DeleteAccount
}

// AdminListUsers recherche les membres par pseudo ou login GitHub (administrateur).
func (s *Service) AdminListUsers(ctx context.Context, a Actor, q string, limit int, after string) (Page[User], error) {
	if a.Role != RoleAdmin {
		return Page[User]{}, errAdminOnly
	}
	cur, err := decodeCursor(after)
	if err != nil {
		return Page[User]{}, err
	}
	limit = clampLimit(limit)
	at, id := cursorArgs(cur)
	pattern := "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(strings.TrimSpace(q)) + "%"
	rows, err := s.db.Query(ctx, `
		SELECT `+userColumns+` FROM utilisateur
		WHERE (pseudo ILIKE $1 OR github_login ILIKE $1)
		  AND ($2::timestamptz IS NULL OR (date_inscription, id_utilisateur) < ($2, $3))
		ORDER BY date_inscription DESC, id_utilisateur DESC LIMIT $4`, pattern, at, id, limit+1)
	if err != nil {
		return Page[User]{}, err
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (User, error) { return scanUser(r) })
	if err != nil {
		return Page[User]{}, err
	}
	return paginate(items, limit, func(u User) cursor { return cursor{At: u.JoinedAt, ID: u.ID} }), nil
}

// AdminUpdateUser change le rôle ou le statut d'un membre. Une suspension
// révoque ses sessions ; un arbitre suspendu ou rétrogradé rend ses litiges
// ouverts à la file d'attente.
func (s *Service) AdminUpdateUser(ctx context.Context, a Actor, id uuid.UUID, p AdminPatch) (User, error) {
	if a.Role != RoleAdmin {
		return User{}, errAdminOnly
	}
	if p.Role != nil && !validRoles[*p.Role] {
		return User{}, fieldInvalid("role", "MEMBRE, ARBITRE ou ADMIN")
	}
	if p.Status != nil && *p.Status != "ACTIF" && *p.Status != "SUSPENDU" {
		return User{}, fieldInvalid("statut", "ACTIF ou SUSPENDU")
	}
	if id == a.ID {
		return User{}, errSelfAdmin
	}
	var u User
	err := s.tx(ctx, func(tx pgx.Tx) error {
		before, err := scanUser(tx.QueryRow(ctx, `SELECT `+userColumns+` FROM utilisateur WHERE id_utilisateur = $1 FOR UPDATE`, id))
		if err != nil {
			return notFoundIfNoRows(err)
		}
		if before.Status == "SUPPRIME" {
			return errDeletedUser
		}
		if u, err = scanUser(tx.QueryRow(ctx, `
			UPDATE utilisateur SET role = COALESCE($2, role), statut_utilisateur = COALESCE($3::statut_utilisateur, statut_utilisateur)
			WHERE id_utilisateur = $1 RETURNING `+userColumns, id, p.Role, p.Status)); err != nil {
			return err
		}
		if u.Role != before.Role {
			if err := recordTransition(ctx, tx, "UTILISATEUR", id, "ROLE_"+string(before.Role), "ROLE_"+string(u.Role), &a.ID, "", s.now()); err != nil {
				return err
			}
		}
		if u.Status != before.Status {
			if err := recordTransition(ctx, tx, "UTILISATEUR", id, before.Status, u.Status, &a.ID, "", s.now()); err != nil {
				return err
			}
		}
		if u.Status == "SUSPENDU" {
			if err := s.revokeAllSessions(ctx, tx, id); err != nil {
				return err
			}
		}
		if u.Status != "ACTIF" || u.Role == RoleMember {
			return unassignDisputes(ctx, tx, id)
		}
		return nil
	})
	return u, err
}

func (s *Service) revokeAllSessions(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	_, err := tx.Exec(ctx, `UPDATE jeton_rafraichissement SET revoque_le = $2 WHERE id_utilisateur = $1 AND revoque_le IS NULL`, userID, s.now())
	return err
}

func unassignDisputes(ctx context.Context, tx pgx.Tx, arbiterID uuid.UUID) error {
	_, err := tx.Exec(ctx, `UPDATE litige SET id_arbitre = NULL, date_affectation = NULL WHERE id_arbitre = $1 AND statut_litige = 'OUVERT'`, arbiterID)
	return err
}

// AdminDisputeQueue renvoie les litiges ouverts sans arbitre, du plus ancien au plus récent.
func (s *Service) AdminDisputeQueue(ctx context.Context, a Actor) ([]DisputeView, error) {
	if a.Role != RoleAdmin {
		return nil, errAdminOnly
	}
	rows, err := s.db.Query(ctx, disputeSelect+` WHERE l.statut_litige = 'OUVERT' AND l.id_arbitre IS NULL ORDER BY l.date_ouverture LIMIT 100`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (DisputeView, error) { return scanDispute(r) })
}

// DeleteAccount anonymise le compte du membre (D13) : pseudo, e-mail, bio et
// identité GitHub effacés, sessions révoquées, offres retirées. Le grand livre,
// les avis et les messages restent, rattachés au pseudo anonyme.
func (s *Service) DeleteAccount(ctx context.Context, a Actor) error {
	// Identifiant GitHub fictif, négatif pour ne jamais rencontrer un vrai.
	n, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return err
	}
	placeholderGitHubID := -n.Int64() - 1
	return s.tx(ctx, func(tx pgx.Tx) error {
		var status string
		if err := tx.QueryRow(ctx, `SELECT statut_utilisateur FROM utilisateur WHERE id_utilisateur = $1 FOR UPDATE`, a.ID).Scan(&status); err != nil {
			return notFoundIfNoRows(err)
		}
		if status == "SUPPRIME" {
			return errDeletedUser
		}
		var busy bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM echange WHERE $1 IN (id_demandeur, id_prestataire)
			               AND statut_echange IN ('PROPOSE', 'EN_COURS', 'EN_LITIGE'))`, a.ID).Scan(&busy); err != nil {
			return err
		}
		if busy {
			return errBusyAccount
		}
		for _, q := range []string{
			`INSERT INTO github_supprime (empreinte, date_suppression)
			   SELECT sha256(github_id::text::bytea), $2 FROM utilisateur WHERE id_utilisateur = $1
			 ON CONFLICT DO NOTHING`,
			`UPDATE offre SET statut_offre = 'RETIREE' WHERE id_prestataire = $1 AND statut_offre = 'PUBLIEE'`,
			`UPDATE jeton_rafraichissement SET revoque_le = $2 WHERE id_utilisateur = $1 AND revoque_le IS NULL`,
			`UPDATE litige SET id_arbitre = NULL, date_affectation = NULL WHERE id_arbitre = $1 AND statut_litige = 'OUVERT'`,
			`DELETE FROM posseder WHERE id_utilisateur = $1`,
			`DELETE FROM notification WHERE id_utilisateur = $1`,
		} {
			args := []any{a.ID}
			if strings.Contains(q, "$2") {
				args = append(args, s.now())
			}
			if _, err := tx.Exec(ctx, q, args...); err != nil {
				return err
			}
		}
		suffix := strings.ReplaceAll(a.ID.String(), "-", "")[:12]
		if _, err := tx.Exec(ctx, `
			UPDATE utilisateur SET pseudo = $2, email = NULL, bio = NULL, github_login = 'supprime', github_id = $3,
			  role = 'MEMBRE', statut_utilisateur = 'SUPPRIME', date_suppression = $4
			WHERE id_utilisateur = $1`, a.ID, "membre-supprime-"+suffix, placeholderGitHubID, s.now()); err != nil {
			return err
		}
		return recordTransition(ctx, tx, "UTILISATEUR", a.ID, status, "SUPPRIME", &a.ID, "suppression du compte", s.now())
	})
}
