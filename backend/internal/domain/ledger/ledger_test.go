package ledger

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func account(t AccountType, balance int64) *Account {
	return &Account{ID: uuid.New(), Type: t, Balance: balance}
}

func TestTransfer_MovesAmount(t *testing.T) {
	from := account(User, 100)
	to := account(Escrow, 0)

	m, err := Transfer(from, to, 40, Block, "blocage:ex1")
	require.NoError(t, err)

	assert.Equal(t, int64(60), from.Balance)
	assert.Equal(t, int64(40), to.Balance)
	assert.Equal(t, Movement{Kind: Block, Amount: 40, Debit: from.ID, Credit: to.ID, Key: "blocage:ex1"}, m)
}

func TestTransfer_Rejects(t *testing.T) {
	cases := map[string]struct {
		from, to *Account
		amount   int64
		key      string
		err      error
	}{
		"montant nul":            {account(User, 10), account(User, 0), 0, "k", ErrInvalidAmount},
		"montant négatif":        {account(User, 10), account(User, 0), -5, "k", ErrInvalidAmount},
		"solde insuffisant":      {account(User, 10), account(Escrow, 0), 11, "k", ErrInsufficientFunds},
		"séquestre insuffisant":  {account(Escrow, 10), account(User, 0), 11, "k", ErrInsufficientFunds},
		"même compte":            {nil, nil, 1, "k", ErrSameAccount},
		"clé d'idempotence vide": {account(User, 10), account(User, 0), 1, "", ErrMissingKey},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			from, to := tc.from, tc.to
			if from == nil {
				from = account(User, 10)
				to = from
			}
			before := [2]int64{from.Balance, to.Balance}
			_, err := Transfer(from, to, tc.amount, Bonus, tc.key)
			require.ErrorIs(t, err, tc.err)
			assert.Equal(t, before, [2]int64{from.Balance, to.Balance}, "aucun solde ne bouge en cas d'erreur")
		})
	}
}

func TestTransfer_IssuanceMayGoNegative(t *testing.T) {
	issuance := account(Issuance, 0)
	user := account(User, 0)
	_, err := Transfer(issuance, user, 50, Bonus, "bonus:u1")
	require.NoError(t, err)
	assert.Equal(t, int64(-50), issuance.Balance)
}

func TestSplit(t *testing.T) {
	cases := []struct {
		amount, pct, provider, requester int64
	}{
		{110, 60, 66, 44},
		{3, 50, 1, 2}, // arrondi à l'inférieur pour le prestataire (D10)
		{1, 99, 0, 1},
		{100, 1, 1, 99},
	}
	for _, c := range cases {
		p, r, err := Split(c.amount, c.pct)
		require.NoError(t, err)
		assert.Equal(t, c.provider, p)
		assert.Equal(t, c.requester, r)
	}
	for _, bad := range []int64{0, 100, -1, 101} {
		_, _, err := Split(10, bad)
		require.ErrorIs(t, err, ErrInvalidShare)
	}
}

// TestProperty_ZeroSumAndNoNegativeUser vérifie RG03 et RG04 sur des suites
// aléatoires de transferts : la somme des soldes reste nulle et aucun compte
// utilisateur ni séquestre ne passe sous zéro.
func TestProperty_ZeroSumAndNoNegativeUser(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		accounts := []*Account{account(Issuance, 0)}
		for i := 0; i < rapid.IntRange(1, 6).Draw(t, "users"); i++ {
			accounts = append(accounts, account(User, 0))
		}
		for i := 0; i < rapid.IntRange(0, 3).Draw(t, "escrows"); i++ {
			accounts = append(accounts, account(Escrow, 0))
		}
		steps := rapid.IntRange(1, 60).Draw(t, "steps")
		for i := 0; i < steps; i++ {
			from := rapid.SampledFrom(accounts).Draw(t, "from")
			to := rapid.SampledFrom(accounts).Draw(t, "to")
			amount := rapid.Int64Range(-5, 200).Draw(t, "amount")
			_, _ = Transfer(from, to, amount, Bonus, "k")

			var sum int64
			for _, a := range accounts {
				sum += a.Balance
				if a.Type != Issuance && a.Balance < 0 {
					t.Fatalf("solde négatif sur un compte %s : %d", a.Type, a.Balance)
				}
			}
			if sum != 0 {
				t.Fatalf("somme des soldes = %d, attendu 0", sum)
			}
		}
	})
}

func TestProperty_SplitPreservesAmount(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		amount := rapid.Int64Range(1, 1_000_000).Draw(t, "amount")
		pct := rapid.Int64Range(1, 99).Draw(t, "pct")
		p, r, err := Split(amount, pct)
		if err != nil {
			t.Fatal(err)
		}
		if p+r != amount || p < 0 || r < 0 {
			t.Fatalf("Split(%d, %d) = %d + %d", amount, pct, p, r)
		}
	})
}
