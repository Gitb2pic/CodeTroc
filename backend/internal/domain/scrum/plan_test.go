package scrum

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/gitb2pic/codetroc/backend/internal/domain/exchange"
)

var t0 = time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)

func day(n int) time.Time { return t0.AddDate(0, 0, n) }

type span struct{ from, to int }

func spans(ss []Sprint) []span {
	out := make([]span, len(ss))
	for i, s := range ss {
		out[i] = span{int(s.Start.Sub(t0).Hours() / 24), int(s.End.Sub(t0).Hours() / 24)}
	}
	return out
}

func TestSplit_Rules(t *testing.T) {
	cases := map[string]struct {
		window []int // échéances en jours
		d      int
		want   []span
	}{
		"L ≤ d : un seul sprint":         {[]int{5}, 7, []span{{0, 5}}},
		"L = d":                          {[]int{7}, 7, []span{{0, 7}}},
		"reste nul":                      {[]int{21}, 7, []span{{0, 7}, {7, 14}, {14, 21}}},
		"reste < d/2 absorbé":            {[]int{15}, 7, []span{{0, 7}, {7, 15}}},
		"reste ≥ d/2 en sprint court":    {[]int{11}, 7, []span{{0, 7}, {7, 11}}},
		"reste de 3 j sur 7 : absorbé":   {[]int{10}, 7, []span{{0, 10}}},
		"sprints de 14 j":                {[]int{30}, 14, []span{{0, 14}, {14, 30}}},
		"exemple de la spec (2 jalons)":  {[]int{15, 30}, 7, []span{{0, 7}, {7, 15}, {15, 22}, {22, 30}}},
		"jalon de fenêtre vide : ignoré": {[]int{1, 1}, 7, []span{{0, 1}}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			ws := make([]Window, len(c.window))
			prev := t0
			for i, d := range c.window {
				ws[i] = Window{Start: prev, End: day(d)}
				prev = day(d)
			}
			got := Split(ws, c.d, 1)
			assert.Equal(t, c.want, spans(got))
			for i, s := range got {
				assert.Equal(t, i+1, s.Number)
			}
		})
	}
}

func TestSplit_SpecExampleMilestones(t *testing.T) {
	ws := MilestoneWindows(t0, exchange.Deadlines(t0, 30, 2))
	got := Split(ws, 7, 1)
	require.Len(t, got, 4)
	assert.Equal(t, []int{0, 0, 1, 1}, []int{got[0].Milestone, got[1].Milestone, got[2].Milestone, got[3].Milestone})
}

// Propriétés R-S3 sur 1 à 10 jalons, délais 1 à 90 j, sprints de 7 ou 14 j.
func TestSplit_Properties(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		days := rapid.IntRange(1, 90).Draw(t, "days")
		n := rapid.IntRange(1, 10).Draw(t, "milestones")
		d := rapid.SampledFrom([]int{7, 14}).Draw(t, "d")
		ws := MilestoneWindows(t0, exchange.Deadlines(t0, days, n))
		got := Split(ws, d, 1)

		require.NotEmpty(t, got)
		for i, s := range got {
			require.Equal(t, i+1, s.Number, "numérotation 1..N sans trou")
			require.True(t, s.End.After(s.Start), "sprint de durée positive")
			w := ws[s.Milestone]
			require.False(t, s.Start.Before(w.Start), "dans la fenêtre de son jalon")
			require.False(t, s.End.After(w.End), "dans la fenêtre de son jalon")
			if i > 0 {
				require.Equal(t, got[i-1].End, s.Start, "contigus, sans chevauchement")
				require.GreaterOrEqual(t, s.Milestone, got[i-1].Milestone)
			}
			L := w.End.Sub(w.Start)
			if L >= time.Duration(d)*24*time.Hour/2 {
				require.GreaterOrEqual(t, s.End.Sub(s.Start), time.Duration(d)*24*time.Hour/2, "aucun sprint < d/2")
			}
		}
		for mi, w := range ws {
			var last *Sprint
			for i := range got {
				if got[i].Milestone == mi {
					last = &got[i]
				}
			}
			if w.End.After(w.Start) {
				require.NotNil(t, last, "chaque jalon non vide a au moins un sprint")
				require.Equal(t, w.End, last.End, "dernier sprint = échéance du jalon")
			} else {
				require.Nil(t, last)
			}
		}
		require.Equal(t, t0, got[0].Start)
		require.Equal(t, ws[len(ws)-1].End, got[len(got)-1].End)
		require.LessOrEqual(t, len(got), 50)
	})
}

func TestReplan_KeepsActiveSprintAndRenumbers(t *testing.T) {
	ws := MilestoneWindows(t0, []time.Time{day(15), day(30)})
	plan := Split(ws, 7, 1)
	// Sprint 1 terminé, sprint 2 actif ; avenant : jalon 1 → J+20, jalon 2 → J+40.
	kept := []Sprint{plan[0], plan[1]}
	nw := MilestoneWindows(t0, []time.Time{day(20), day(40)})
	got := Replan(nw, kept, []bool{false, false}, 7)

	assert.Equal(t, []span{{15, 20}, {20, 27}, {27, 34}, {34, 40}}, spans(got))
	assert.Equal(t, []int{3, 4, 5, 6}, []int{got[0].Number, got[1].Number, got[2].Number, got[3].Number})
	assert.Equal(t, []int{0, 1, 1, 1}, []int{got[0].Milestone, got[1].Milestone, got[2].Milestone, got[3].Milestone})

	// Un jalon soldé ne reçoit plus de sprint.
	got = Replan(nw, kept, []bool{true, false}, 7)
	assert.Equal(t, []span{{20, 27}, {27, 34}, {34, 40}}, spans(got))
}
