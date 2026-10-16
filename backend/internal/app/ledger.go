package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gitb2pic/codetroc/backend/internal/domain/ledger"
)

var errInsufficient = unprocessable("solde-insuffisant", "Le solde disponible ne couvre pas ce montant.")

type transferReq struct {
	From, To    uuid.UUID
	Amount      int64
	Kind        ledger.Kind
	Key         string // clé d'idempotence dérivée par le serveur (D4)
	ExchangeID  *uuid.UUID
	MilestoneID *uuid.UUID
}

// transfer enregistre un mouvement dans la transaction tx. Les deux comptes
// sont verrouillés dans l'ordre de leurs identifiants pour éviter les
// interblocages. Une clé déjà utilisée ne produit aucun effet (renvoie false).
func (s *Service) transfer(ctx context.Context, tx pgx.Tx, r transferReq) (bool, error) {
	ids := []uuid.UUID{r.From, r.To}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	accounts := map[uuid.UUID]*ledger.Account{}
	for _, id := range ids {
		a := &ledger.Account{ID: id}
		if err := tx.QueryRow(ctx, `SELECT type_compte, solde FROM compte WHERE id_compte = $1 FOR UPDATE`, id).Scan(&a.Type, &a.Balance); err != nil {
			return false, fmt.Errorf("verrouillage du compte %s : %w", id, err)
		}
		accounts[id] = a
	}

	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM mouvement WHERE cle_idempotence = $1)`, r.Key).Scan(&exists); err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}

	from, to := accounts[r.From], accounts[r.To]
	m, err := ledger.Transfer(from, to, r.Amount, r.Kind, r.Key)
	if errors.Is(err, ledger.ErrInsufficientFunds) {
		return false, errInsufficient
	}
	if err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO mouvement (type_mouvement, montant, cle_idempotence, date_mouvement, id_compte_debit, id_compte_credit, id_echange, id_jalon)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		m.Kind, m.Amount, m.Key, s.now(), m.Debit, m.Credit, r.ExchangeID, r.MilestoneID); err != nil {
		return false, err
	}
	for _, a := range []*ledger.Account{from, to} {
		if _, err := tx.Exec(ctx, `UPDATE compte SET solde = $2 WHERE id_compte = $1`, a.ID, a.Balance); err != nil {
			return false, err
		}
	}
	return true, nil
}

// createUserAccount ouvre le compte de crédits d'un utilisateur et verse le
// bonus de bienvenue depuis ÉMISSION (RG01, RG02, D1).
func (s *Service) createUserAccount(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	if _, err := tx.Exec(ctx, `INSERT INTO compte (type_compte, id_utilisateur) VALUES ('UTILISATEUR', $1)`, userID); err != nil {
		return err
	}
	return s.grantWelcomeBonus(ctx, tx, userID)
}

// grantWelcomeBonus verse le bonus de bienvenue (D1), une seule fois par
// compte (clé bonus:<id>). D13 : pas de second bonus pour un compte GitHub ou
// une adresse email déjà inscrits puis supprimés.
func (s *Service) grantWelcomeBonus(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	if s.settings.WelcomeBonus <= 0 {
		return nil
	}
	var returning bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM github_supprime g JOIN utilisateur u
		                 ON g.empreinte IN (sha256(u.github_id::text::bytea), sha256(('email:' || lower(u.email))::bytea))
		               WHERE u.id_utilisateur = $1)`, userID).Scan(&returning); err != nil {
		return err
	}
	if returning {
		return nil
	}
	var issuance, accountID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT id_compte FROM compte WHERE type_compte = 'EMISSION'`).Scan(&issuance); err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, `SELECT id_compte FROM compte WHERE id_utilisateur = $1`, userID).Scan(&accountID); err != nil {
		return err
	}
	_, err := s.transfer(ctx, tx, transferReq{From: issuance, To: accountID, Amount: s.settings.WelcomeBonus, Kind: ledger.Bonus, Key: "bonus:" + userID.String()})
	return err
}

// Balance résume les crédits d'un membre.
type Balance struct {
	Available int64
	EscrowOut int64 // bloqués pour des échanges où il est demandeur
	EscrowIn  int64 // bloqués pour des échanges où il est prestataire
}

// Balance renvoie le solde disponible et les séquestres en cours.
func (s *Service) Balance(ctx context.Context, a Actor) (Balance, error) {
	var b Balance
	err := s.db.QueryRow(ctx, balanceSQL, a.ID).Scan(&b.Available, &b.EscrowOut, &b.EscrowIn)
	return b, notFoundIfNoRows(err)
}

// MovementLine est une écriture vue depuis le compte du membre.
type MovementLine struct {
	ID          uuid.UUID
	Kind        ledger.Kind
	Amount      int64 // positif si crédité, négatif si débité
	At          time.Time
	ExchangeID  *uuid.UUID
	MilestoneID *uuid.UUID
}

// Movements renvoie l'historique paginé du compte du membre (UC15).
func (s *Service) Movements(ctx context.Context, a Actor, limit int, after string) (Page[MovementLine], error) {
	cur, err := decodeCursor(after)
	if err != nil {
		return Page[MovementLine]{}, err
	}
	limit = clampLimit(limit)
	at, id := cursorArgs(cur)
	rows, err := s.db.Query(ctx, `
		SELECT m.id_mouvement, m.type_mouvement,
		       CASE WHEN m.id_compte_credit = c.id_compte THEN m.montant ELSE -m.montant END,
		       m.date_mouvement, m.id_echange, m.id_jalon
		FROM compte c
		JOIN mouvement m ON c.id_compte IN (m.id_compte_debit, m.id_compte_credit)
		WHERE c.id_utilisateur = $1
		  AND ($2::timestamptz IS NULL OR (m.date_mouvement, m.id_mouvement) < ($2, $3))
		ORDER BY m.date_mouvement DESC, m.id_mouvement DESC
		LIMIT $4`, a.ID, at, id, limit+1)
	if err != nil {
		return Page[MovementLine]{}, err
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (MovementLine, error) {
		var m MovementLine
		err := r.Scan(&m.ID, &m.Kind, &m.Amount, &m.At, &m.ExchangeID, &m.MilestoneID)
		return m, err
	})
	if err != nil {
		return Page[MovementLine]{}, err
	}
	return paginate(items, limit, func(m MovementLine) cursor { return cursor{At: m.At, ID: m.ID} }), nil
}

// paginate coupe la ligne en trop (limit+1) et calcule le curseur suivant.
func paginate[T any](items []T, limit int, key func(T) cursor) Page[T] {
	p := Page[T]{Items: items}
	if len(items) > limit {
		p.Items = items[:limit]
		p.NextCursor = key(p.Items[limit-1]).encode()
	}
	if p.Items == nil {
		p.Items = []T{}
	}
	return p
}

// IntegrityReport est le résultat du contrôle du grand livre.
type IntegrityReport struct {
	TotalBalance int64
	Mismatches   []uuid.UUID
}

// LedgerIntegrity vérifie RG04 : somme des soldes nulle, et chaque solde égal
// à ses crédits moins ses débits. Exécuté en CI et par le worker.
func (s *Service) LedgerIntegrity(ctx context.Context) (IntegrityReport, error) {
	var r IntegrityReport
	if err := s.db.QueryRow(ctx, `SELECT COALESCE(SUM(solde), 0) FROM compte`).Scan(&r.TotalBalance); err != nil {
		return r, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT c.id_compte FROM compte c
		LEFT JOIN (SELECT id_compte_credit AS id, SUM(montant) AS total FROM mouvement GROUP BY 1) cr ON cr.id = c.id_compte
		LEFT JOIN (SELECT id_compte_debit  AS id, SUM(montant) AS total FROM mouvement GROUP BY 1) db ON db.id = c.id_compte
		WHERE c.solde <> COALESCE(cr.total, 0) - COALESCE(db.total, 0)`)
	if err != nil {
		return r, err
	}
	r.Mismatches, err = pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	return r, err
}
