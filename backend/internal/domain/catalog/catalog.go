// Package catalog porte les règles du catalogue : packs, offres et jalons.
package catalog

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// ProofType est la manière de prouver la livraison d'un jalon (D7).
type ProofType string

const (
	ProofPRCI   ProofType = "PR_CI"    // PR mergée et CI au vert, vérifiées via GitHub
	ProofManual ProofType = "MANUELLE" // revue, audit : validation par le demandeur
)

// CheckKind indique comment un critère d'acceptation est vérifié.
type CheckKind string

const (
	CheckCI     CheckKind = "CI"
	CheckScan   CheckKind = "SCAN"
	CheckReview CheckKind = "REVUE"
)

// Criterion est un critère d'acceptation.
type Criterion struct {
	Text  string    `json:"texte"`
	Check CheckKind `json:"verification"`
}

// Pack est un service standardisé du catalogue.
type Pack struct {
	Title       string
	Description string
	Deliverable string
	ProofType   ProofType
	Criteria    []Criterion
	MinPrice    int64
	MaxPrice    int64
	SkillIDs    int // nombre de compétences requises (RG07)
}

// Milestone est un jalon proposé : intitulé et montant.
type Milestone struct {
	Title  string `json:"titre"`
	Amount int64  `json:"montant"`
}

// Offer est la proposition d'un membre de réaliser un pack.
type Offer struct {
	Title       string
	Description string
	Price       int64
	Days        int
	Milestones  []Milestone
}

const (
	maxPrice      = 100_000
	maxCriteria   = 20
	maxMilestones = 10
	maxDays       = 90
)

// ValidationError désigne le champ fautif et un code de problème.
type ValidationError struct {
	Code   string
	Field  string
	Detail string
}

func (e *ValidationError) Error() string { return e.Field + " : " + e.Detail }

func fieldErr(field, detail string) error {
	return &ValidationError{Code: "donnees-invalides", Field: field, Detail: detail}
}

func checkText(field, v string, min, max int) error {
	n := utf8.RuneCountInString(strings.TrimSpace(v))
	if n < min || utf8.RuneCountInString(v) > max {
		return fieldErr(field, fmt.Sprintf("entre %d et %d caractères", min, max))
	}
	return nil
}

// Validate vérifie un pack (RG07).
func (p Pack) Validate() error {
	for _, c := range []struct {
		field, v string
		min, max int
	}{
		{"titre", p.Title, 5, 120},
		{"description", p.Description, 1, 4000},
		{"livrable", p.Deliverable, 1, 2000},
	} {
		if err := checkText(c.field, c.v, c.min, c.max); err != nil {
			return err
		}
	}
	if p.ProofType != ProofPRCI && p.ProofType != ProofManual {
		return fieldErr("type_preuve", "PR_CI ou MANUELLE")
	}
	if len(p.Criteria) == 0 || len(p.Criteria) > maxCriteria {
		return fieldErr("criteres", fmt.Sprintf("entre 1 et %d critères", maxCriteria))
	}
	for _, c := range p.Criteria {
		if err := checkText("criteres", c.Text, 1, 300); err != nil {
			return err
		}
		if c.Check != CheckCI && c.Check != CheckScan && c.Check != CheckReview {
			return fieldErr("criteres", "vérification CI, SCAN ou REVUE")
		}
	}
	if p.MinPrice <= 0 || p.MaxPrice < p.MinPrice || p.MaxPrice > maxPrice {
		return fieldErr("prix", fmt.Sprintf("0 < prix_min ≤ prix_max ≤ %d", maxPrice))
	}
	if p.SkillIDs == 0 {
		return fieldErr("competences", "au moins une compétence requise")
	}
	return nil
}

// ValidateMilestones vérifie une liste de jalons : 1 à 10, montants
// strictement positifs, somme égale au prix (RG10).
func ValidateMilestones(price int64, ms []Milestone) error {
	if len(ms) == 0 || len(ms) > maxMilestones {
		return fieldErr("jalons", fmt.Sprintf("entre 1 et %d jalons", maxMilestones))
	}
	var sum int64
	for _, m := range ms {
		if err := checkText("jalons", m.Title, 1, 120); err != nil {
			return err
		}
		if m.Amount <= 0 {
			return fieldErr("jalons", "montant strictement positif")
		}
		sum += m.Amount
	}
	if sum != price {
		return &ValidationError{Code: "jalons-incoherents", Field: "jalons",
			Detail: fmt.Sprintf("la somme des jalons (%d cr) doit égaler le prix (%d cr)", sum, price)}
	}
	return nil
}

// Validate vérifie une offre sur un pack (RG08).
func (o Offer) Validate(p Pack) error {
	if err := checkText("titre", o.Title, 5, 120); err != nil {
		return err
	}
	if err := checkText("description", o.Description, 0, 4000); err != nil {
		return err
	}
	if o.Price < p.MinPrice || o.Price > p.MaxPrice {
		return &ValidationError{Code: "prix-hors-fourchette", Field: "prix_credits",
			Detail: fmt.Sprintf("le prix doit être compris entre %d et %d cr", p.MinPrice, p.MaxPrice)}
	}
	if o.Days < 1 || o.Days > maxDays {
		return fieldErr("delai_jours", fmt.Sprintf("entre 1 et %d jours", maxDays))
	}
	return ValidateMilestones(o.Price, o.Milestones)
}

// DefaultMilestones répartit un prix en parts égales, le reste au dernier jalon.
func DefaultMilestones(price int64, titles []string) []Milestone {
	n := int64(len(titles))
	base := price / n
	ms := make([]Milestone, len(titles))
	for i, t := range titles {
		ms[i] = Milestone{Title: t, Amount: base}
	}
	ms[len(ms)-1].Amount = price - base*(n-1)
	return ms
}
