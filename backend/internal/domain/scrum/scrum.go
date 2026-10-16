package scrum

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// SprintStatus est l'état d'un sprint.
type SprintStatus string

const (
	SprintPlanned   SprintStatus = "PLANIFIE"
	SprintActive    SprintStatus = "ACTIF"
	SprintDone      SprintStatus = "TERMINE"
	SprintCancelled SprintStatus = "ANNULE"
)

// StoryStatus est la colonne d'une story sur le tableau.
type StoryStatus string

const (
	StoryBacklog StoryStatus = "BACKLOG"
	StoryTodo    StoryStatus = "A_FAIRE"
	StoryDoing   StoryStatus = "EN_COURS"
	StoryReview  StoryStatus = "EN_REVUE"
	StoryDone    StoryStatus = "TERMINE"
)

// Role est le rôle Scrum de l'auteur d'un changement (R-S6).
type Role string

const (
	PO     Role = "PO"      // demandeur : backlog, priorités, acceptation
	Dev    Role = "DEV"     // prestataire : estimation, avancement
	System Role = "SYSTEME" // worker et webhooks GitHub
)

// ErrTransition signale un changement d'état interdit.
var ErrTransition = errors.New("transition interdite")

var sprintMoves = map[SprintStatus][]SprintStatus{
	SprintPlanned: {SprintActive, SprintCancelled},
	SprintActive:  {SprintDone, SprintCancelled},
}

// CheckSprint vérifie une transition de sprint.
func CheckSprint(from, to SprintStatus) error {
	if slices.Contains(sprintMoves[from], to) {
		return nil
	}
	return fmt.Errorf("%w : sprint %s → %s", ErrTransition, from, to)
}

type storyMove struct {
	from, to StoryStatus
}

// storyMoves dit qui peut déplacer une story d'une colonne à une autre.
var storyMoves = map[storyMove][]Role{
	{StoryBacklog, StoryTodo}:   {PO},
	{StoryTodo, StoryBacklog}:   {PO, System},
	{StoryDoing, StoryBacklog}:  {PO, System},
	{StoryReview, StoryBacklog}: {PO, System},
	{StoryTodo, StoryDoing}:     {Dev, System},
	{StoryDoing, StoryTodo}:     {Dev},
	{StoryDoing, StoryReview}:   {Dev, System},
	{StoryReview, StoryDoing}:   {Dev, PO, System},
	{StoryReview, StoryDone}:    {PO, System},
	{StoryDoing, StoryDone}:     {System},
	{StoryTodo, StoryDone}:      {System},
	{StoryDone, StoryDoing}:     {PO, System},
}

// CheckStory vérifie qu'un rôle peut déplacer une story (R-S6) : le
// prestataire fait avancer, le demandeur planifie et accepte en « Terminé ».
// Le système applique les événements GitHub (issue fermée, rouverte, label).
func CheckStory(from, to StoryStatus, by Role) error {
	if slices.Contains(storyMoves[storyMove{from, to}], by) {
		return nil
	}
	return fmt.Errorf("%w : story %s → %s par %s", ErrTransition, from, to, by)
}

// ValidPoints : estimation en suite de Fibonacci (template).
func ValidPoints(p int) bool { return slices.Contains(Standard.Points, p) }

// RiskLevel est le risque de retard d'un jalon (R-S4).
type RiskLevel string

const (
	RiskNone   RiskLevel = "AUCUN"
	RiskMedium RiskLevel = "MOYEN"
	RiskHigh   RiskLevel = "ELEVE"
)

// Risk compare le temps écoulé du jalon aux points faits : MOYEN au-delà de
// 60 % du temps avec moins de 40 % des points, ÉLEVÉ au-delà de 85 % avec
// moins de 70 %. Sans point planifié, rien à mesurer.
func Risk(now, start, deadline time.Time, done, total int) RiskLevel {
	if total <= 0 || !deadline.After(start) {
		return RiskNone
	}
	elapsed := float64(now.Sub(start)) / float64(deadline.Sub(start))
	ratio := float64(done) / float64(total)
	switch {
	case elapsed > 0.85 && ratio < 0.7:
		return RiskHigh
	case elapsed > 0.6 && ratio < 0.4:
		return RiskMedium
	}
	return RiskNone
}

// Template est un cadre Scrum versionné en code, copié dans le projet à sa
// création : une évolution du template ne touche pas les projets en cours.
type Template struct {
	Name              string
	SprintDays        int
	Columns           []StoryStatus
	Points            []int
	StoryPattern      string
	DefinitionOfReady []string
	DefinitionOfDone  []string
}

// Standard est le template « scrum-standard-v1 ».
var Standard = Template{
	Name:         "scrum-standard-v1",
	SprintDays:   7,
	Columns:      []StoryStatus{StoryBacklog, StoryTodo, StoryDoing, StoryReview, StoryDone},
	Points:       []int{0, 1, 2, 3, 5, 8, 13, 21},
	StoryPattern: "En tant que <rôle>, je veux <action> afin de <bénéfice>",
	DefinitionOfReady: []string{
		"Story rédigée", "Critères d'acceptation présents", "Story estimée", "Sans dépendance bloquante",
	},
	DefinitionOfDone: []string{
		"Code revu", "Checks CI au vert", "Critères d'acceptation vérifiés par le PO", "Documentation à jour si besoin",
	},
}

// ---------- Rendu GitHub (issues, labels) ----------

// Label est un label GitHub.
type Label struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

// ProjectLabels liste les labels d'un projet de n jalons et s sprints.
func ProjectLabels(milestones, sprints int) []Label {
	ls := []Label{
		{"codetroc", "5a4bfe"}, {"type:sprint", "9a90ff"}, {"type:story", "00c2a8"},
		{"statut:en-revue", "fbca04"}, {"bloqué", "d73a4a"},
	}
	for i := 1; i <= milestones; i++ {
		ls = append(ls, Label{"jalon:" + strconv.Itoa(i), "0e1116"})
	}
	for i := 1; i <= sprints; i++ {
		ls = append(ls, Label{"sprint:" + strconv.Itoa(i), "c5def5"})
	}
	for _, p := range Standard.Points[1:] {
		ls = append(ls, Label{"points:" + strconv.Itoa(p), "ededed"})
	}
	return ls
}

const maxLine = 300

func truncate(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max-1]) + "…"
}

// neutralize empêche les mentions (@) de notifier des tiers.
func neutralize(s string) string { return strings.ReplaceAll(s, "@", "@​") }

var mdEscaper = strings.NewReplacer(
	`\`, `\\`, "`", "\\`", "*", `\*`, "_", `\_`, "[", `\[`, "]", `\]`, "|", `\|`, "~", `\~`,
	"<", "&lt;", ">", "&gt;",
)

func escapeLine(s string) string {
	s = mdEscaper.Replace(neutralize(s))
	if strings.HasPrefix(s, "#") {
		s = `\` + s
	}
	return s
}

// Escape rend un texte saisi par un utilisateur inoffensif dans du Markdown
// GitHub : une seule ligne, bornée, sans HTML, mentions neutralisées.
func Escape(s string) string {
	s = strings.Join(strings.Fields(strings.ReplaceAll(strings.ReplaceAll(s, "\r", " "), "\n", " ")), " ")
	return escapeLine(truncate(s, maxLine))
}

// escapeBlock échappe un texte multiligne borné à max caractères.
func escapeBlock(s string, max int) string {
	lines := strings.Split(strings.ReplaceAll(truncate(s, max), "\r", ""), "\n")
	for i, l := range lines {
		lines[i] = escapeLine(l)
	}
	return strings.Join(lines, "\n")
}

// title borne et neutralise un titre d'issue (les titres ne sont pas rendus en Markdown).
func title(s string) string {
	return neutralize(truncate(strings.Join(strings.Fields(s), " "), 200))
}

// Issue est une issue GitHub à créer ou mettre à jour.
type Issue struct {
	Title  string   `json:"title"`
	Body   string   `json:"body"`
	Labels []string `json:"labels"`
}

// IssueStory est une story listée dans l'issue d'un sprint.
type IssueStory struct {
	Issue  int // 0 tant que son issue n'existe pas
	Title  string
	Points *int
}

// SprintIssueData alimente l'issue d'un sprint.
type SprintIssueData struct {
	Number         int
	MilestoneOrder int
	MilestoneTitle string
	Objective      string
	Start, End     time.Time
	Deadline       time.Time
	Amount         int64
	Index, Count   int // rang du sprint dans son jalon
	Stories        []IssueStory
	DoD            []string
	Link           string
}

func date(t time.Time) string { return t.UTC().Format("02/01/2006") }

// SprintIssue rend l'issue « Sprint n » générée au démarrage du sprint.
func SprintIssue(d SprintIssueData) Issue {
	var b strings.Builder
	obj := strings.TrimSpace(d.Objective)
	if obj == "" {
		obj = "à définir au Sprint Planning"
	} else {
		obj = Escape(obj)
	}
	fmt.Fprintf(&b, "**Objectif du sprint** : %s\n", obj)
	fmt.Fprintf(&b, "**Période** : %s → %s · **Jalon** : %s (échéance %s, %d crédits)\n",
		date(d.Start), date(d.End), Escape(d.MilestoneTitle), date(d.Deadline), d.Amount)
	fmt.Fprintf(&b, "**Sprint** %d sur %d de ce jalon\n\n", d.Index, d.Count)
	if len(d.Stories) == 0 {
		b.WriteString("### Sprint Planning\n- [ ] Choisir les stories du sprint dans le backlog\n- [ ] Estimer les stories\n- [ ] Fixer l'objectif du sprint\n")
	} else {
		b.WriteString("### Stories du sprint\n")
		for _, s := range d.Stories {
			b.WriteString("- [ ] ")
			if s.Issue > 0 {
				fmt.Fprintf(&b, "#%d ", s.Issue)
			}
			b.WriteString(Escape(s.Title))
			if s.Points != nil {
				fmt.Fprintf(&b, " (%d pts)", *s.Points)
			}
			b.WriteString("\n")
		}
	}
	if len(d.DoD) > 0 {
		b.WriteString("\n### Definition of Done\n")
		for _, c := range d.DoD {
			fmt.Fprintf(&b, "- [ ] %s\n", Escape(c))
		}
	}
	if d.Link != "" {
		fmt.Fprintf(&b, "\n> %s\n", d.Link)
	}
	return Issue{
		Title:  fmt.Sprintf("[CodeTroc] Sprint %d — Jalon %d : %s", d.Number, d.MilestoneOrder, title(d.MilestoneTitle)),
		Body:   b.String(),
		Labels: []string{"codetroc", "type:sprint", "sprint:" + strconv.Itoa(d.Number), "jalon:" + strconv.Itoa(d.MilestoneOrder)},
	}
}

// StoryIssueData alimente l'issue d'une story.
type StoryIssueData struct {
	Title          string
	Description    string
	Criteria       []string
	Sprint         int
	MilestoneOrder int
	Points         *int
	Link           string
}

// StoryIssue rend l'issue d'une story affectée à un sprint.
func StoryIssue(d StoryIssueData) Issue {
	var b strings.Builder
	if desc := strings.TrimSpace(d.Description); desc != "" {
		b.WriteString(escapeBlock(desc, 4000))
		b.WriteString("\n\n")
	}
	if len(d.Criteria) > 0 {
		b.WriteString("### Critères d'acceptation\n")
		for _, c := range d.Criteria {
			fmt.Fprintf(&b, "- [ ] %s\n", Escape(c))
		}
	}
	if d.Link != "" {
		fmt.Fprintf(&b, "\n> %s\n", d.Link)
	}
	labels := []string{"codetroc", "type:story", "sprint:" + strconv.Itoa(d.Sprint), "jalon:" + strconv.Itoa(d.MilestoneOrder)}
	if d.Points != nil && *d.Points > 0 {
		labels = append(labels, "points:"+strconv.Itoa(*d.Points))
	}
	return Issue{Title: title(d.Title), Body: b.String(), Labels: labels}
}

// SprintReview est le commentaire de bilan posté à la fin d'un sprint.
func SprintReview(number, done, planned int, unfinished int) string {
	s := fmt.Sprintf("**Bilan du sprint %d** : %d points faits sur %d prévus.", number, done, planned)
	if unfinished > 0 {
		s += fmt.Sprintf(" %d story(s) non terminée(s) retournent au backlog.", unfinished)
	}
	return s
}
