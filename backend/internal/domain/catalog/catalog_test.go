package catalog

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validPack() Pack {
	return Pack{
		Title: "Pipeline CI/CD GitHub Actions", Description: "Lint, tests, build et déploiement.",
		Deliverable: "Workflow dans .github/workflows", ProofType: ProofPRCI,
		Criteria: []Criterion{{Text: "Pipeline vert sur main", Check: CheckCI}},
		MinPrice: 80, MaxPrice: 160, SkillIDs: 1,
	}
}

func TestValidatePack(t *testing.T) {
	require.NoError(t, validPack().Validate())

	cases := map[string]func(*Pack){
		"titre trop court":           func(p *Pack) { p.Title = "CI" },
		"titre trop long":            func(p *Pack) { p.Title = strings.Repeat("a", 121) },
		"description vide":           func(p *Pack) { p.Description = " " },
		"livrable vide":              func(p *Pack) { p.Deliverable = "" },
		"sans critère":               func(p *Pack) { p.Criteria = nil },
		"critère vide":               func(p *Pack) { p.Criteria = []Criterion{{Text: " ", Check: CheckCI}} },
		"vérification inconnue":      func(p *Pack) { p.Criteria = []Criterion{{Text: "x", Check: "MAGIE"}} },
		"trop de critères":           func(p *Pack) { p.Criteria = make([]Criterion, 21) },
		"prix minimum nul":           func(p *Pack) { p.MinPrice = 0 },
		"fourchette inversée":        func(p *Pack) { p.MinPrice, p.MaxPrice = 100, 50 },
		"type de preuve inconnu":     func(p *Pack) { p.ProofType = "PHOTO" },
		"sans compétence (RG07)":     func(p *Pack) { p.SkillIDs = 0 },
		"prix maximum déraisonnable": func(p *Pack) { p.MaxPrice = 100_001 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			p := validPack()
			mutate(&p)
			var verr *ValidationError
			require.ErrorAs(t, p.Validate(), &verr)
			assert.NotEmpty(t, verr.Field)
		})
	}
}

func TestValidateMilestones(t *testing.T) {
	ok := []Milestone{{Title: "Workflow CI", Amount: 40}, {Title: "Build", Amount: 40}, {Title: "Déploiement", Amount: 40}}
	require.NoError(t, ValidateMilestones(120, ok))

	cases := map[string][]Milestone{
		"aucun jalon":       nil,
		"somme différente":  {{Title: "A", Amount: 40}, {Title: "B", Amount: 40}},
		"montant nul":       {{Title: "A", Amount: 0}, {Title: "B", Amount: 120}},
		"titre vide":        {{Title: "  ", Amount: 120}},
		"titre trop long":   {{Title: strings.Repeat("x", 121), Amount: 120}},
		"plus de 10 jalons": make([]Milestone, 11),
		"montant négatif":   {{Title: "A", Amount: -10}, {Title: "B", Amount: 130}},
	}
	for name, ms := range cases {
		t.Run(name, func(t *testing.T) {
			var verr *ValidationError
			require.ErrorAs(t, ValidateMilestones(120, ms), &verr)
		})
	}
	err := ValidateMilestones(120, []Milestone{{Title: "A", Amount: 40}, {Title: "B", Amount: 40}})
	var verr *ValidationError
	require.ErrorAs(t, err, &verr)
	assert.Equal(t, "jalons-incoherents", verr.Code)
}

func TestValidateOffer(t *testing.T) {
	p := validPack()
	o := Offer{Title: "Pipeline pour app Node", Description: "Sur ton dépôt existant.", Price: 120, Days: 5,
		Milestones: []Milestone{{Title: "CI", Amount: 60}, {Title: "Déploiement", Amount: 60}}}
	require.NoError(t, o.Validate(p))

	cases := map[string]func(*Offer){
		"prix sous la fourchette (RG08)":  func(o *Offer) { o.Price = 79; o.Milestones = []Milestone{{Title: "A", Amount: 79}} },
		"prix au-dessus de la fourchette": func(o *Offer) { o.Price = 161; o.Milestones = []Milestone{{Title: "A", Amount: 161}} },
		"délai nul":                       func(o *Offer) { o.Days = 0 },
		"délai trop long":                 func(o *Offer) { o.Days = 91 },
		"jalons qui ne font pas le prix":  func(o *Offer) { o.Milestones = []Milestone{{Title: "A", Amount: 100}} },
		"titre vide":                      func(o *Offer) { o.Title = "" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			c := o
			c.Milestones = append([]Milestone(nil), o.Milestones...)
			mutate(&c)
			require.Error(t, c.Validate(p))
		})
	}
}

func TestDefaultSplit(t *testing.T) {
	ms := DefaultMilestones(100, []string{"A", "B", "C"})
	require.NoError(t, ValidateMilestones(100, ms))
	assert.Equal(t, []int64{33, 33, 34}, []int64{ms[0].Amount, ms[1].Amount, ms[2].Amount})
}
