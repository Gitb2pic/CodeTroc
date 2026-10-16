package scrum

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSprintTransitions(t *testing.T) {
	allowed := map[[2]SprintStatus]bool{
		{SprintPlanned, SprintActive}: true, {SprintPlanned, SprintCancelled}: true,
		{SprintActive, SprintDone}: true, {SprintActive, SprintCancelled}: true,
	}
	all := []SprintStatus{SprintPlanned, SprintActive, SprintDone, SprintCancelled}
	for _, from := range all {
		for _, to := range all {
			err := CheckSprint(from, to)
			if allowed[[2]SprintStatus{from, to}] {
				assert.NoError(t, err, "%s → %s", from, to)
			} else {
				assert.Error(t, err, "%s → %s", from, to)
			}
		}
	}
}

func TestStoryTransitions(t *testing.T) {
	cases := []struct {
		from, to StoryStatus
		by       Role
		ok       bool
	}{
		{StoryBacklog, StoryTodo, PO, true},
		{StoryBacklog, StoryTodo, Dev, false},
		{StoryTodo, StoryBacklog, PO, true},
		{StoryTodo, StoryDoing, Dev, true},
		{StoryTodo, StoryDoing, PO, false},
		{StoryDoing, StoryReview, Dev, true},
		{StoryReview, StoryDoing, Dev, true},
		{StoryDoing, StoryTodo, Dev, true},
		{StoryReview, StoryDone, PO, true},
		{StoryReview, StoryDone, Dev, false},
		{StoryDoing, StoryDone, PO, false},
		{StoryDoing, StoryDone, System, true}, // issue fermée sur GitHub
		{StoryDone, StoryDoing, System, true}, // issue rouverte
		{StoryDone, StoryDoing, PO, true},
		{StoryDone, StoryDoing, Dev, false},
		{StoryDoing, StoryReview, System, true}, // label statut:en-revue
		{StoryBacklog, StoryDone, System, false},
		{StoryDone, StoryBacklog, PO, false},
	}
	for _, c := range cases {
		err := CheckStory(c.from, c.to, c.by)
		if c.ok {
			assert.NoError(t, err, "%s → %s par %s", c.from, c.to, c.by)
		} else {
			assert.Error(t, err, "%s → %s par %s", c.from, c.to, c.by)
		}
	}
}

func TestRisk(t *testing.T) {
	start := t0
	end := t0.Add(100 * time.Hour)
	at := func(h int) time.Time { return t0.Add(time.Duration(h) * time.Hour) }
	cases := []struct {
		name        string
		now         time.Time
		done, total int
		want        RiskLevel
	}{
		{"début", at(10), 0, 10, RiskNone},
		{"60 % pile : pas encore", at(60), 0, 10, RiskNone},
		{"61 %, 39 % faits", at(61), 3, 10, RiskMedium},
		{"61 %, 40 % faits", at(61), 4, 10, RiskNone},
		{"85 % pile, 30 % faits : moyen", at(85), 3, 10, RiskMedium},
		{"86 %, 69 % faits", at(86), 6, 10, RiskHigh},
		{"86 %, 70 % faits", at(86), 7, 10, RiskNone},
		{"sans points planifiés", at(90), 0, 0, RiskNone},
		{"après l'échéance", at(120), 2, 10, RiskHigh},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, Risk(c.now, start, end, c.done, c.total))
		})
	}
}

func TestPoints(t *testing.T) {
	for _, p := range []int{0, 1, 2, 3, 5, 8, 13, 21} {
		assert.True(t, ValidPoints(p))
	}
	for _, p := range []int{-1, 4, 6, 20, 34} {
		assert.False(t, ValidPoints(p))
	}
}

func TestEscapeMarkdown(t *testing.T) {
	got := Escape("@octocat *gras* <script>alert(1)</script> [lien](http://x) `code` # titre")
	assert.NotContains(t, got, "@octocat", "mention neutralisée")
	assert.NotContains(t, got, "<script>")
	assert.Contains(t, got, `\*gras\*`)
	assert.Contains(t, got, `\[lien\]`)
	assert.Contains(t, got, "\\`code\\`")
	assert.NotContains(t, Escape("a\nb\r\nc"), "\n", "une ligne")
	assert.Equal(t, 300, len([]rune(Escape(strings.Repeat("é", 400)))))
}

func TestSprintIssue(t *testing.T) {
	is := SprintIssue(SprintIssueData{
		Number: 3, MilestoneOrder: 2, MilestoneTitle: "API @team", Objective: "",
		Start: t0, End: t0.AddDate(0, 0, 7), Deadline: t0.AddDate(0, 0, 15), Amount: 60,
		Index: 1, Count: 2, DoD: Standard.DefinitionOfDone,
		Stories: []IssueStory{{Issue: 12, Title: "Connexion *GitHub*", Points: ptr(5)}, {Title: "Sans issue"}},
		Link:    "https://codetroc.labtools.tech/echanges/x",
	})
	assert.Equal(t, "[CodeTroc] Sprint 3 — Jalon 2 : API @\u200bteam", is.Title)
	assert.ElementsMatch(t, []string{"codetroc", "type:sprint", "sprint:3", "jalon:2"}, is.Labels)
	assert.Contains(t, is.Body, "**Objectif du sprint** : à définir au Sprint Planning")
	assert.Contains(t, is.Body, "**Période** : 20/10/2026 → 27/10/2026")
	assert.Contains(t, is.Body, "**Sprint** 1 sur 2 de ce jalon")
	assert.Contains(t, is.Body, "- [ ] #12 Connexion \\*GitHub\\* (5 pts)")
	assert.Contains(t, is.Body, "- [ ] Sans issue")
	assert.Contains(t, is.Body, "- [ ] Code revu")
	assert.Contains(t, is.Body, "> https://codetroc.labtools.tech/echanges/x")

	empty := SprintIssue(SprintIssueData{Number: 1, MilestoneOrder: 1, MilestoneTitle: "M", Start: t0, End: t0, Deadline: t0, Index: 1, Count: 1})
	assert.Contains(t, empty.Body, "### Sprint Planning")
}

func TestStoryIssue(t *testing.T) {
	is := StoryIssue(StoryIssueData{Title: "Liste @all", Description: "<b>x</b>", Criteria: []string{"Given a"}, Sprint: 2, MilestoneOrder: 1, Points: ptr(3)})
	assert.Equal(t, "Liste @\u200ball", is.Title)
	assert.ElementsMatch(t, []string{"codetroc", "type:story", "sprint:2", "jalon:1", "points:3"}, is.Labels)
	assert.NotContains(t, is.Body, "<b>")
	assert.Contains(t, is.Body, "- [ ] Given a")
}

func TestLabels(t *testing.T) {
	ls := ProjectLabels(2, 4)
	names := map[string]bool{}
	for _, l := range ls {
		names[l.Name] = true
		require.Len(t, l.Color, 6)
	}
	for _, n := range []string{"codetroc", "type:sprint", "type:story", "jalon:1", "jalon:2", "sprint:4", "points:21", "statut:en-revue", "bloqué"} {
		assert.True(t, names[n], n)
	}
	assert.False(t, names["jalon:3"])
}

func ptr(i int) *int { return &i }
