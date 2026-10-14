// Package ledger implémente le grand livre des crédits en partie double.
//
// Chaque mouvement débite un compte et en crédite un autre du même montant
// strictement positif : la somme de tous les soldes reste nulle (RG04). Seul
// le compte ÉMISSION peut être négatif ; il représente les crédits mis en
// circulation par les bonus de bienvenue (RG02).
package ledger

import (
	"errors"

	"github.com/google/uuid"
)

// AccountType est la nature d'un compte.
type AccountType string

const (
	User     AccountType = "UTILISATEUR"
	Escrow   AccountType = "SEQUESTRE" // un compte par échange (D2)
	Issuance AccountType = "EMISSION"
)

// Kind est la nature d'un mouvement.
type Kind string

const (
	Bonus   Kind = "BONUS"
	Block   Kind = "BLOCAGE"
	Release Kind = "LIBERATION"
	Refund  Kind = "REMBOURSEMENT"
)

var (
	ErrInvalidAmount     = errors.New("ledger : montant strictement positif attendu")
	ErrInsufficientFunds = errors.New("ledger : solde insuffisant")
	ErrSameAccount       = errors.New("ledger : débit et crédit sur le même compte")
	ErrMissingKey        = errors.New("ledger : clé d'idempotence manquante")
	ErrInvalidShare      = errors.New("ledger : part entre 1 et 99 % attendue")
)

// Account est un compte et son solde courant.
type Account struct {
	ID      uuid.UUID
	Type    AccountType
	Balance int64
}

// Movement est une écriture du grand livre ; elle n'est jamais modifiée.
type Movement struct {
	Kind   Kind
	Amount int64
	Debit  uuid.UUID
	Credit uuid.UUID
	Key    string
}

// Transfer déplace amount de from vers to et renvoie l'écriture. En cas
// d'erreur, aucun solde n'est modifié.
func Transfer(from, to *Account, amount int64, kind Kind, key string) (Movement, error) {
	switch {
	case amount <= 0:
		return Movement{}, ErrInvalidAmount
	case from.ID == to.ID:
		return Movement{}, ErrSameAccount
	case key == "":
		return Movement{}, ErrMissingKey
	case from.Type != Issuance && from.Balance < amount:
		return Movement{}, ErrInsufficientFunds
	}
	from.Balance -= amount
	to.Balance += amount
	return Movement{Kind: kind, Amount: amount, Debit: from.ID, Credit: to.ID, Key: key}, nil
}

// Split répartit un montant entre prestataire et demandeur selon la part du
// prestataire en pourcentage (1 à 99). La part du prestataire est arrondie à
// l'inférieur ; le reste revient au demandeur (D10).
func Split(amount, providerPct int64) (provider, requester int64, err error) {
	if providerPct < 1 || providerPct > 99 {
		return 0, 0, ErrInvalidShare
	}
	provider = amount * providerPct / 100
	return provider, amount - provider, nil
}
