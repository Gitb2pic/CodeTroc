package proof

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePRURL(t *testing.T) {
	pr, err := ParsePRURL("https://github.com/acme/shop-api/pull/42")
	require.NoError(t, err)
	assert.Equal(t, PRRef{Owner: "acme", Repo: "shop-api", Number: 42}, pr)

	for _, bad := range []string{
		"", "https://github.com/acme/shop-api/pull/0", "https://github.com/acme/shop-api/pull/-1",
		"https://github.com/acme/shop-api/issues/42", "https://github.com/acme/shop-api/pull/42/files",
		"https://gitlab.com/acme/shop-api/pull/42", "http://github.com/acme/shop-api/pull/42",
		"https://github.com/acme/shop-api/pull/99999999999", "https://github.com/acme/pull/42",
	} {
		_, err := ParsePRURL(bad)
		assert.ErrorIs(t, err, ErrPRURL, bad)
	}
}

func FuzzParsePRURL(f *testing.F) {
	f.Add("https://github.com/acme/shop-api/pull/42")
	f.Fuzz(func(t *testing.T, s string) {
		pr, err := ParsePRURL(s)
		if err != nil {
			return
		}
		if pr.Number <= 0 {
			t.Fatalf("numéro invalide accepté : %q", s)
		}
		again, err := ParsePRURL(pr.URL())
		if err != nil || again != pr {
			t.Fatalf("%q : forme canonique instable", s)
		}
	})
}

var accepted = time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)

func facts() Facts {
	return Facts{
		RepoID: 100, ProviderGitHubID: 7, AcceptedAt: accepted,
		PR:     PR{ID: 555, BaseRepoID: 100, AuthorID: 7, CreatedAt: accepted.Add(time.Hour), Merged: true, HeadSHA: "a3f9c1e"},
		Checks: []Check{{Name: "lint", Status: "completed", Conclusion: "success"}, {Name: "test", Status: "completed", Conclusion: "success"}},
	}
}

func TestEvaluate(t *testing.T) {
	v := Evaluate(facts())
	assert.Equal(t, Valid, v.Status)
	assert.Contains(t, v.Detail, "2/2")

	cases := []struct {
		name   string
		mutate func(*Facts)
		status Status
		detail string
	}{
		{"PR d'un autre dépôt", func(f *Facts) { f.PR.BaseRepoID = 999 }, Rejected, "dépôt"},
		{"PR d'un autre auteur", func(f *Facts) { f.PR.AuthorID = 8 }, Rejected, "auteur"},
		{"PR antérieure à l'acceptation", func(f *Facts) { f.PR.CreatedAt = accepted.Add(-time.Minute) }, Rejected, "antérieure"},
		{"PR déjà utilisée", func(f *Facts) { f.AlreadyUsed = true }, Rejected, "déjà"},
		{"PR non mergée", func(f *Facts) { f.PR.Merged = false }, Pending, "mergée"},
		{"aucun check", func(f *Facts) { f.Checks = nil }, Rejected, "aucun check"},
		{"CI en cours", func(f *Facts) { f.Checks[1] = Check{Name: "test", Status: "in_progress"} }, Pending, "en cours"},
		{"check en échec", func(f *Facts) { f.Checks[1].Conclusion = "failure" }, Rejected, "test"},
		{"check annulé", func(f *Facts) { f.Checks[0].Conclusion = "cancelled" }, Rejected, "lint"},
		{"aucun succès (tout ignoré)", func(f *Facts) {
			f.Checks = []Check{{Name: "a", Status: "completed", Conclusion: "skipped"}, {Name: "b", Status: "completed", Conclusion: "neutral"}}
		}, Rejected, "succès"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := facts()
			f.Checks = append([]Check(nil), f.Checks...)
			c.mutate(&f)
			v := Evaluate(f)
			assert.Equal(t, c.status, v.Status)
			assert.Contains(t, v.Detail, c.detail)
		})
	}

	// L'ordre compte : une PR d'un autre auteur est rejetée même si elle n'est pas encore mergée.
	f := facts()
	f.PR.AuthorID, f.PR.Merged = 8, false
	assert.Equal(t, Rejected, Evaluate(f).Status)

	f = facts()
	f.Checks = append(f.Checks, Check{Name: "docs", Status: "completed", Conclusion: "skipped"})
	assert.Equal(t, Valid, Evaluate(f).Status, "un check ignoré n'empêche pas la validation")
}
