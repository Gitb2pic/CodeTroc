package catalog

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validAnnouncement() Announcement {
	return Announcement{
		Title: "Pipeline CI pour notre API", Need: "Lint, tests et image Docker sur chaque PR.",
		Budget: 120, Days: 20,
		Milestones: []Milestone{{Title: "CI de base", Amount: 60}, {Title: "Image Docker", Amount: 60}},
	}
}

func TestValidateAnnouncement(t *testing.T) {
	require.NoError(t, validAnnouncement().Validate(validPack()))

	cases := map[string]struct {
		mutate func(*Announcement)
		code   string
	}{
		"titre trop court":       {func(a *Announcement) { a.Title = "CI" }, "donnees-invalides"},
		"besoin vide":            {func(a *Announcement) { a.Need = " " }, "donnees-invalides"},
		"besoin trop long":       {func(a *Announcement) { a.Need = strings.Repeat("a", 4001) }, "donnees-invalides"},
		"budget sous le pack":    {func(a *Announcement) { a.Budget = 79; a.Milestones = []Milestone{{Title: "x", Amount: 79}} }, "prix-hors-fourchette"},
		"budget au-dessus":       {func(a *Announcement) { a.Budget = 161; a.Milestones = []Milestone{{Title: "x", Amount: 161}} }, "prix-hors-fourchette"},
		"délai nul":              {func(a *Announcement) { a.Days = 0 }, "donnees-invalides"},
		"délai au-delà de 90 j":  {func(a *Announcement) { a.Days = 91 }, "donnees-invalides"},
		"jalons ≠ budget (RG10)": {func(a *Announcement) { a.Milestones[1].Amount = 50 }, "jalons-incoherents"},
		"sans jalon":             {func(a *Announcement) { a.Milestones = nil }, "donnees-invalides"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			a := validAnnouncement()
			c.mutate(&a)
			var ve *ValidationError
			require.ErrorAs(t, a.Validate(validPack()), &ve)
			assert.Equal(t, c.code, ve.Code)
		})
	}
}

func validApplication() Application {
	return Application{Price: 100, Days: 15, Message: "Je l'ai déjà fait pour deux API Node.",
		Milestones: []Milestone{{Title: "CI", Amount: 40}, {Title: "Docker", Amount: 60}}}
}

func TestValidateApplication(t *testing.T) {
	require.NoError(t, validApplication().Validate(validPack()))

	cases := map[string]struct {
		mutate func(*Application)
		code   string
	}{
		"prix hors fourchette": {func(a *Application) { a.Price = 200; a.Milestones = []Milestone{{Title: "x", Amount: 200}} }, "prix-hors-fourchette"},
		"délai nul":            {func(a *Application) { a.Days = 0 }, "donnees-invalides"},
		"message trop long":    {func(a *Application) { a.Message = strings.Repeat("a", 2001) }, "donnees-invalides"},
		"jalons ≠ prix (RG10)": {func(a *Application) { a.Milestones[0].Amount = 1 }, "jalons-incoherents"},
		"onze jalons":          {func(a *Application) { a.Milestones = make([]Milestone, 11) }, "donnees-invalides"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			a := validApplication()
			c.mutate(&a)
			var ve *ValidationError
			require.ErrorAs(t, a.Validate(validPack()), &ve)
			assert.Equal(t, c.code, ve.Code)
		})
	}
}
