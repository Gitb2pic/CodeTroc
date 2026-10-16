package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/gitb2pic/codetroc/backend/internal/app"
	"github.com/gitb2pic/codetroc/backend/internal/domain/exchange"
	"github.com/gitb2pic/codetroc/backend/internal/domain/scrum"
)

// F16 — cadre Scrum d'un échange.
type scrumHandlers struct{ svc *app.Service }

type amendmentJSON struct {
	ID         uuid.UUID   `json:"id"`
	ProposedBy uuid.UUID   `json:"propose_par"`
	Deadlines  []time.Time `json:"echeances"`
	Reason     string      `json:"motif"`
	Status     string      `json:"statut"`
	ProposedAt time.Time   `json:"date_proposition"`
	DecidedAt  *time.Time  `json:"date_decision"`
	DecidedBy  *uuid.UUID  `json:"decide_par"`
}

func toAmendmentJSON(v app.AmendmentView) amendmentJSON { return amendmentJSON(v) }

type projectJSON struct {
	ExchangeID        uuid.UUID           `json:"id_echange"`
	Template          string              `json:"template"`
	Objective         string              `json:"objectif_produit"`
	DefinitionOfDone  []string            `json:"definition_of_done"`
	DefinitionOfReady []string            `json:"definition_of_ready"`
	Columns           []scrum.StoryStatus `json:"colonnes"`
	Points            []int               `json:"points_possibles"`
	StoryPattern      string              `json:"gabarit_story"`
	SprintDays        int                 `json:"duree_sprint_jours"`
	SyncStatus        string              `json:"statut_synchro"`
	SyncDetail        *string             `json:"detail_synchro"`
	PendingCalls      int                 `json:"appels_github_en_attente"`
	Role              app.ScrumRole       `json:"mon_role"`
	Amendment         *amendmentJSON      `json:"avenant_en_cours"`
	CreatedAt         time.Time           `json:"date_creation"`
}

func toProjectJSON(v app.ProjectView) projectJSON {
	j := projectJSON{ExchangeID: v.ExchangeID, Template: v.Template, Objective: v.Objective, DefinitionOfDone: v.DefinitionOfDone,
		DefinitionOfReady: v.DefinitionOfReady, Columns: v.Columns, Points: v.Points, StoryPattern: v.StoryPattern, SprintDays: v.SprintDays,
		SyncStatus: v.SyncStatus, SyncDetail: v.SyncDetail, PendingCalls: v.PendingCalls, Role: v.Role, CreatedAt: v.CreatedAt}
	if v.Amendment != nil {
		a := toAmendmentJSON(*v.Amendment)
		j.Amendment = &a
	}
	return j
}

type sprintJSON struct {
	ID             uuid.UUID          `json:"id"`
	Number         int                `json:"numero"`
	MilestoneOrder int                `json:"jalon"`
	Start          time.Time          `json:"debut_prevu"`
	End            time.Time          `json:"fin_prevue"`
	StartedAt      *time.Time         `json:"debut_reel"`
	EndedAt        *time.Time         `json:"fin_reelle"`
	Status         scrum.SprintStatus `json:"statut"`
	Objective      string             `json:"objectif"`
	Issue          *int               `json:"github_issue"`
	PointsPlanned  int                `json:"points_prevus"`
	PointsDone     int                `json:"points_faits"`
	Retro          struct {
		Good    *string `json:"bien"`
		Improve *string `json:"a_ameliorer"`
		Actions *string `json:"actions"`
	} `json:"retro"`
}

func toSprintJSON(v app.SprintView) sprintJSON {
	j := sprintJSON{ID: v.ID, Number: v.Number, MilestoneOrder: v.MilestoneOrder, Start: v.Start, End: v.End, StartedAt: v.StartedAt,
		EndedAt: v.EndedAt, Status: v.Status, Objective: v.Objective, Issue: v.Issue, PointsPlanned: v.PointsPlanned, PointsDone: v.PointsDone}
	j.Retro.Good, j.Retro.Improve, j.Retro.Actions = v.RetroGood, v.RetroImprove, v.RetroActions
	return j
}

type storyJSON struct {
	ID             uuid.UUID         `json:"id"`
	MilestoneOrder *int              `json:"jalon"`
	SprintNumber   *int              `json:"sprint"`
	Title          string            `json:"titre"`
	Description    string            `json:"description"`
	Criteria       []string          `json:"criteres"`
	Points         *int              `json:"points"`
	Priority       int               `json:"priorite"`
	Status         scrum.StoryStatus `json:"statut"`
	Issue          *int              `json:"github_issue"`
	CreatedAt      time.Time         `json:"date_creation"`
	DoneAt         *time.Time        `json:"date_terminee"`
	CreatedBy      uuid.UUID         `json:"cree_par"`
}

func toStoryJSON(v app.StoryView) storyJSON { return storyJSON(v) }

func (h scrumHandlers) project(c *gin.Context, fn func(app.Actor, uuid.UUID) (app.ProjectView, error)) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	v, err := fn(actor(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, toProjectJSON(v))
}

func (h scrumHandlers) get(c *gin.Context) {
	h.project(c, func(a app.Actor, id uuid.UUID) (app.ProjectView, error) {
		return h.svc.GetScrum(c.Request.Context(), a, id)
	})
}

func (h scrumHandlers) patch(c *gin.Context) {
	var b struct {
		Objective *string  `json:"objectif_produit"`
		DoD       []string `json:"definition_of_done"`
	}
	if !bindJSON(c, &b) {
		return
	}
	h.project(c, func(a app.Actor, id uuid.UUID) (app.ProjectView, error) {
		return h.svc.PatchScrum(c.Request.Context(), a, id, app.ProjectPatch{Objective: b.Objective, DefinitionOfDone: b.DoD})
	})
}

func (h scrumHandlers) resync(c *gin.Context) {
	h.project(c, func(a app.Actor, id uuid.UUID) (app.ProjectView, error) {
		return h.svc.ResyncScrum(c.Request.Context(), a, id)
	})
}

func (h scrumHandlers) sprints(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	vs, err := h.svc.ListSprints(c.Request.Context(), actor(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	items := make([]sprintJSON, len(vs))
	for i, v := range vs {
		items[i] = toSprintJSON(v)
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// sprintNumber lit le numéro de sprint du chemin (404 s'il n'est pas un entier).
func sprintNumber(c *gin.Context) (uuid.UUID, int, bool) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return id, 0, false
	}
	n, err := strconv.Atoi(c.Param("numero"))
	if err != nil || n < 1 || n > 50 {
		WriteProblem(c, ErrNotFound)
		return id, 0, false
	}
	return id, n, true
}

func (h scrumHandlers) patchSprint(c *gin.Context) {
	id, n, ok := sprintNumber(c)
	if !ok {
		return
	}
	var b struct {
		Objective string `json:"objectif"`
	}
	if !bindJSON(c, &b) {
		return
	}
	v, err := h.svc.UpdateSprintObjective(c.Request.Context(), actor(c), id, n, b.Objective)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, toSprintJSON(v))
}

func (h scrumHandlers) retro(c *gin.Context) {
	id, n, ok := sprintNumber(c)
	if !ok {
		return
	}
	var b struct {
		Good    string `json:"bien"`
		Improve string `json:"a_ameliorer"`
		Actions string `json:"actions"`
	}
	if !bindJSON(c, &b) {
		return
	}
	v, err := h.svc.SaveRetro(c.Request.Context(), actor(c), id, n, app.Retro(b))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, toSprintJSON(v))
}

func (h scrumHandlers) stories(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	f := app.StoryFilter{Status: scrum.StoryStatus(c.Query("statut")), Backlog: c.Query("sprint") == "backlog"}
	for name, dst := range map[string]**int{"sprint": &f.Sprint, "jalon": &f.Milestone} {
		if v := c.Query(name); v != "" && (name != "sprint" || v != "backlog") {
			n, err := strconv.Atoi(v)
			if err != nil {
				fail(c, &app.Error{Kind: app.KindInvalid, Code: "parametre-invalide", Title: "Requête invalide", Detail: name + " doit être un entier."})
				return
			}
			*dst = &n
		}
	}
	vs, err := h.svc.ListStories(c.Request.Context(), actor(c), id, f)
	if err != nil {
		fail(c, err)
		return
	}
	items := make([]storyJSON, len(vs))
	for i, v := range vs {
		items[i] = toStoryJSON(v)
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h scrumHandlers) createStory(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var b struct {
		Title       string   `json:"titre"`
		Description string   `json:"description"`
		Criteria    []string `json:"criteres"`
		Milestone   *int     `json:"jalon"`
		Sprint      *int     `json:"sprint"`
	}
	if !bindJSON(c, &b) {
		return
	}
	v, err := h.svc.CreateStory(c.Request.Context(), actor(c), id, app.StoryInput{Title: b.Title, Description: b.Description,
		Criteria: b.Criteria, MilestoneOrder: b.Milestone, SprintNumber: b.Sprint})
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, toStoryJSON(v))
}

// optInt distingue un champ absent, null et un entier.
type optInt struct {
	Set   bool
	Value *int
}

func (o *optInt) UnmarshalJSON(b []byte) error {
	o.Set = true
	if bytes.Equal(b, []byte("null")) {
		return nil
	}
	return json.Unmarshal(b, &o.Value)
}

func (h scrumHandlers) patchStory(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	sid, ok := pathUUID(c, "sid")
	if !ok {
		return
	}
	var b struct {
		Title       *string            `json:"titre"`
		Description *string            `json:"description"`
		Criteria    []string           `json:"criteres"`
		Priority    *int               `json:"priorite"`
		Milestone   *int               `json:"jalon"`
		Sprint      optInt             `json:"sprint"` // null : retour au backlog
		Status      *scrum.StoryStatus `json:"statut"`
		Points      *int               `json:"points"`
	}
	if !bindJSON(c, &b) {
		return
	}
	p := app.StoryPatch{Title: b.Title, Description: b.Description, Criteria: b.Criteria, Priority: b.Priority,
		MilestoneOrder: b.Milestone, Status: b.Status, Points: b.Points}
	if b.Sprint.Set {
		p.SprintNumber, p.ToBacklog = b.Sprint.Value, b.Sprint.Value == nil
	}
	v, err := h.svc.PatchStory(c.Request.Context(), actor(c), id, sid, p)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, toStoryJSON(v))
}

type planningSprintJSON struct {
	Number        int                `json:"numero"`
	Start         time.Time          `json:"debut_prevu"`
	End           time.Time          `json:"fin_prevue"`
	StartedAt     *time.Time         `json:"debut_reel"`
	EndedAt       *time.Time         `json:"fin_reelle"`
	Status        scrum.SprintStatus `json:"statut"`
	PointsPlanned int                `json:"points_prevus"`
	PointsDone    int                `json:"points_faits"`
	Issue         *int               `json:"github_issue"`
}

type planningMilestoneJSON struct {
	Order      int                      `json:"ordre"`
	Title      string                   `json:"titre"`
	Start      time.Time                `json:"debut"`
	Deadline   time.Time                `json:"echeance"`
	Status     exchange.MilestoneStatus `json:"statut_jalon"`
	Amount     int64                    `json:"montant"`
	Late       bool                     `json:"en_retard"`
	Risk       scrum.RiskLevel          `json:"risque"`
	GitHubNo   *int                     `json:"github_milestone"`
	PointsDone int                      `json:"points_faits"`
	PointsAll  int                      `json:"points_prevus"`
	Sprints    []planningSprintJSON     `json:"sprints"`
}

type markerJSON struct {
	Kind      string    `json:"type"`
	At        time.Time `json:"date"`
	Milestone int       `json:"jalon"`
	PR        *int      `json:"pr"`
}

func (h scrumHandlers) planning(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	p, err := h.svc.GetPlanning(c.Request.Context(), actor(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	ms := make([]planningMilestoneJSON, len(p.Milestones))
	for i, m := range p.Milestones {
		ss := make([]planningSprintJSON, len(m.Sprints))
		for k, s := range m.Sprints {
			ss[k] = planningSprintJSON(s)
		}
		ms[i] = planningMilestoneJSON{Order: m.Order, Title: m.Title, Start: m.Start, Deadline: m.Deadline, Status: m.Status, Amount: m.Amount,
			Late: m.Late, Risk: m.Risk, GitHubNo: m.GitHubNo, PointsDone: m.PointsDone, PointsAll: m.PointsAll, Sprints: ss}
	}
	markers := make([]markerJSON, len(p.Markers))
	for i, m := range p.Markers {
		markers[i] = markerJSON(m)
	}
	c.JSON(http.StatusOK, gin.H{"debut": p.Start, "fin": p.End, "aujourd_hui": p.Today, "jalons": ms, "marqueurs": markers})
}

func (h scrumHandlers) propose(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var b struct {
		Deadlines []time.Time `json:"echeances"`
		Reason    string      `json:"motif"`
	}
	if !bindJSON(c, &b) {
		return
	}
	v, err := h.svc.ProposeAmendment(c.Request.Context(), actor(c), id, b.Deadlines, b.Reason)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, toAmendmentJSON(v))
}

func (h scrumHandlers) decide(accept bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := pathUUID(c, "id")
		if !ok {
			return
		}
		aid, ok := pathUUID(c, "aid")
		if !ok {
			return
		}
		v, err := h.svc.DecideAmendment(c.Request.Context(), actor(c), id, aid, accept)
		if err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusOK, toAmendmentJSON(v))
	}
}
