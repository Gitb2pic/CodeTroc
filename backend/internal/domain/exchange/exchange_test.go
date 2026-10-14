package exchange

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExchangeTransitions(t *testing.T) {
	allowed := map[Status][]Status{
		Proposed:   {Refused, Cancelled, InProgress},
		InProgress: {InDispute, Done, Cancelled},
		InDispute:  {InProgress, Done},
	}
	all := []Status{Proposed, Refused, InProgress, InDispute, Done, Cancelled}
	for _, from := range all {
		for _, to := range all {
			want := contains(allowed[from], to)
			err := CheckExchange(from, to)
			if want {
				assert.NoError(t, err, "%s → %s", from, to)
			} else {
				assert.ErrorIs(t, err, ErrTransition, "%s → %s", from, to)
			}
		}
	}
}

func TestMilestoneTransitions(t *testing.T) {
	allowed := map[MilestoneStatus][]MilestoneStatus{
		Pending:   {Active, Refunded},
		Active:    {Delivered, Refunded},
		Delivered: {Released, Disputed},
		Disputed:  {Released, Refunded, Split},
	}
	all := []MilestoneStatus{Pending, Active, Delivered, Disputed, Released, Refunded, Split}
	for _, from := range all {
		for _, to := range all {
			err := CheckMilestone(from, to)
			if contains(allowed[from], to) {
				assert.NoError(t, err, "%s → %s", from, to)
			} else {
				assert.ErrorIs(t, err, ErrTransition, "%s → %s", from, to)
			}
		}
	}
	assert.True(t, Released.Final())
	assert.False(t, Delivered.Final())
}

func contains[T comparable](xs []T, x T) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func TestParseRepoURL(t *testing.T) {
	ok := map[string]Repo{
		"https://github.com/acme/shop-api":     {Owner: "acme", Name: "shop-api"},
		"https://github.com/acme/shop-api/":    {Owner: "acme", Name: "shop-api"},
		"https://github.com/acme/shop-api.git": {Owner: "acme", Name: "shop-api"},
		"https://github.com/Nora-D/blog.v2":    {Owner: "Nora-D", Name: "blog.v2"},
	}
	for in, want := range ok {
		got, err := ParseRepoURL(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got)
		assert.Equal(t, "https://github.com/"+want.Owner+"/"+want.Name, got.URL())
	}
	bad := []string{
		"", "http://github.com/acme/api", "https://gitlab.com/acme/api", "https://github.com.evil.io/acme/api",
		"https://user:pass@github.com/acme/api", "https://github.com/acme", "https://github.com/acme/api/pull/1",
		"https://github.com/acme/api?x=1", "https://github.com/acme/api#readme", "https://github.com:8443/acme/api",
		"https://github.com/-acme/api", "https://github.com/acme/..", "https://github.com/acme/.", "https://169.254.169.254/acme/api",
		"https://github.com/ac me/api", "https://github.com/acme/" + string(make([]byte, 101)),
	}
	for _, in := range bad {
		_, err := ParseRepoURL(in)
		assert.ErrorIs(t, err, ErrRepoURL, "%q doit être refusé", in)
	}
}

func FuzzParseRepoURL(f *testing.F) {
	for _, s := range []string{"https://github.com/acme/api", "https://github.com/a/b.git", "https://x"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		r, err := ParseRepoURL(s)
		if err != nil {
			return
		}
		// Toute URL acceptée se réécrit en une URL GitHub canonique et stable.
		again, err := ParseRepoURL(r.URL())
		if err != nil || again != r {
			t.Fatalf("%q accepté mais sa forme canonique %q ne l'est pas", s, r.URL())
		}
	})
}

func TestDeadlines(t *testing.T) {
	start := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	d := Deadlines(start, 5, 3)
	require.Len(t, d, 3)
	assert.Equal(t, start.AddDate(0, 0, 2), d[0]) // ceil(5×1/3)
	assert.Equal(t, start.AddDate(0, 0, 4), d[1]) // ceil(5×2/3)
	assert.Equal(t, start.AddDate(0, 0, 5), d[2], "le dernier jalon tombe au délai annoncé")
	assert.Equal(t, []time.Time{start.AddDate(0, 0, 1)}, Deadlines(start, 1, 1))
}

func TestNextActor(t *testing.T) {
	cases := []struct {
		st   Status
		ms   []MilestoneStatus
		want Actor
	}{
		{Proposed, []MilestoneStatus{Pending}, ActorProvider},
		{InProgress, []MilestoneStatus{Active, Pending}, ActorProvider},
		{InProgress, []MilestoneStatus{Delivered, Active}, ActorRequester},
		{InDispute, []MilestoneStatus{Disputed, Active}, ActorArbiter},
		{Done, []MilestoneStatus{Released}, ActorNone},
		{Cancelled, []MilestoneStatus{Refunded}, ActorNone},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, NextActor(c.st, c.ms), "%s %v", c.st, c.ms)
	}
}
