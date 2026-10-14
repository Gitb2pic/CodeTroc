package app

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gitb2pic/codetroc/backend/internal/domain/ledger"
)

// requireLedgerConsistent vérifie RG04 en base : somme nulle et soldes
// conformes aux écritures. Appelée à la fin des tests qui bougent des crédits.
func requireLedgerConsistent(t *testing.T, e *env) {
	t.Helper()
	report, err := e.svc.LedgerIntegrity(context.Background())
	require.NoError(t, err)
	require.Zero(t, report.TotalBalance, "la somme des soldes doit être nulle")
	require.Empty(t, report.Mismatches, "solde différent de crédits − débits")
}

func TestWelcomeBonusOnFirstLoginOnly(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	id := GitHubIdentity{ID: 300, Login: "nouveau", CreatedAt: e.clock.Now().AddDate(-1, 0, 0)}
	s := e.loginAs(t, id)
	a := e.actor(t, s)

	bal, err := e.svc.Balance(ctx, a)
	require.NoError(t, err)
	assert.Equal(t, int64(50), bal.Available)

	e.loginAs(t, id) // seconde connexion : pas de second bonus
	bal, err = e.svc.Balance(ctx, a)
	require.NoError(t, err)
	assert.Equal(t, int64(50), bal.Available)

	page, err := e.svc.Movements(ctx, a, 20, "")
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	assert.Equal(t, ledger.Bonus, page.Items[0].Kind)
	assert.Equal(t, int64(50), page.Items[0].Amount, "positif : crédité sur le compte")
	requireLedgerConsistent(t, e)
}

func TestConcurrentFirstLoginGivesOneBonus(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	id := GitHubIdentity{ID: 301, Login: "pressé", CreatedAt: e.clock.Now().AddDate(-1, 0, 0)}
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		code := "bonus-" + string(rune('a'+i))
		e.oauth.add(code, id)
		_, st, err := e.svc.StartLogin(ctx)
		require.NoError(t, err)
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := e.svc.CompleteLogin(ctx, st, st, code)
			assert.NoError(t, err)
		}()
	}
	wg.Wait()
	var n int
	require.NoError(t, e.db.QueryRow(ctx, `SELECT count(*) FROM mouvement WHERE type_mouvement = 'BONUS'`).Scan(&n))
	assert.Equal(t, 1, n)
	requireLedgerConsistent(t, e)
}

// transferForTest exécute un transfert dans sa propre transaction.
func transferForTest(e *env, from, to uuid.UUID, amount int64, key string) error {
	return e.svc.tx(context.Background(), func(tx pgx.Tx) error {
		_, err := e.svc.transfer(context.Background(), tx, transferReq{From: from, To: to, Amount: amount, Kind: ledger.Block, Key: key})
		return err
	})
}

func userAccount(t *testing.T, e *env, userID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, e.db.QueryRow(context.Background(), `SELECT id_compte FROM compte WHERE id_utilisateur = $1`, userID).Scan(&id))
	return id
}

func TestTransfer_IdempotentAndGuarded(t *testing.T) {
	e := newEnv(t)
	a := e.login(t, "a")
	b := e.login(t, "b")
	from, to := userAccount(t, e, a.User.ID), userAccount(t, e, b.User.ID)

	require.NoError(t, transferForTest(e, from, to, 20, "test:1"))
	require.NoError(t, transferForTest(e, from, to, 20, "test:1"), "rejouer la même clé ne crée pas de second mouvement")
	err := transferForTest(e, from, to, 31, "test:2")
	requireKind(t, err, KindUnprocessable, "solde-insuffisant")

	var balA, balB int64
	require.NoError(t, e.db.QueryRow(context.Background(), `SELECT solde FROM compte WHERE id_compte = $1`, from).Scan(&balA))
	require.NoError(t, e.db.QueryRow(context.Background(), `SELECT solde FROM compte WHERE id_compte = $1`, to).Scan(&balB))
	assert.Equal(t, int64(30), balA)
	assert.Equal(t, int64(70), balB)
	requireLedgerConsistent(t, e)
}

func TestTransfer_ConcurrentDebitsNeverOverdraw(t *testing.T) {
	e := newEnv(t)
	a := e.login(t, "source")
	b := e.login(t, "puits")
	from, to := userAccount(t, e, a.User.ID), userAccount(t, e, b.User.ID)

	// 100 débits de 1 cr en parallèle sur un solde de 50 : 50 réussissent exactement.
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := transferForTest(e, from, to, 1, "conc:"+uuid.NewString()); err == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	assert.Equal(t, 50, ok)
	requireLedgerConsistent(t, e)
}

func TestTransfer_CrossedTransfersDoNotDeadlock(t *testing.T) {
	e := newEnv(t)
	a := e.login(t, "x")
	b := e.login(t, "y")
	ax, bx := userAccount(t, e, a.User.ID), userAccount(t, e, b.User.ID)
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); _ = transferForTest(e, ax, bx, 1, "x:"+uuid.NewString()) }()
		go func() { defer wg.Done(); _ = transferForTest(e, bx, ax, 1, "y:"+uuid.NewString()) }()
	}
	wg.Wait()
	requireLedgerConsistent(t, e)
}

func TestLedgerIsAppendOnly(t *testing.T) {
	e := newEnv(t)
	e.login(t, "z")
	_, err := e.db.Exec(context.Background(), `UPDATE mouvement SET montant = 1000`)
	require.ErrorContains(t, err, "ajout seul")
	_, err = e.db.Exec(context.Background(), `DELETE FROM mouvement`)
	require.ErrorContains(t, err, "ajout seul")
}

func TestDatabaseRefusesNegativeUserBalance(t *testing.T) {
	e := newEnv(t)
	s := e.login(t, "contrainte")
	_, err := e.db.Exec(context.Background(), `UPDATE compte SET solde = -1 WHERE id_utilisateur = $1`, s.User.ID)
	require.ErrorContains(t, err, "ck_solde_positif", "double filet : la base refuse aussi un solde négatif")
}

func TestMovementsPagination(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	a := e.login(t, "pages")
	b := e.login(t, "autre")
	from, to := userAccount(t, e, a.User.ID), userAccount(t, e, b.User.ID)
	for i := 0; i < 5; i++ {
		e.clock.Advance(time.Second)
		require.NoError(t, transferForTest(e, from, to, 1, "p:"+uuid.NewString()))
	}
	actor := e.actor(t, a)

	p1, err := e.svc.Movements(ctx, actor, 4, "")
	require.NoError(t, err)
	require.Len(t, p1.Items, 4)
	require.NotEmpty(t, p1.NextCursor)
	assert.Equal(t, int64(-1), p1.Items[0].Amount, "débit : montant négatif")

	p2, err := e.svc.Movements(ctx, actor, 4, p1.NextCursor)
	require.NoError(t, err)
	require.Len(t, p2.Items, 2, "5 transferts + 1 bonus")
	assert.Empty(t, p2.NextCursor)
	assert.Equal(t, ledger.Bonus, p2.Items[1].Kind)

	_, err = e.svc.Movements(ctx, actor, 4, "curseur-forgé")
	requireKind(t, err, KindInvalid, "curseur-invalide")
}
