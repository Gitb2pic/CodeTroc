package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/gitb2pic/codetroc/backend/internal/app"
	"github.com/gitb2pic/codetroc/backend/internal/domain/catalog"
)

// F17 — annonces et candidatures.
type announcementHandlers struct{ svc *app.Service }

type announcementJSON struct {
	ID           uuid.UUID           `json:"id"`
	Title        string              `json:"titre"`
	Need         string              `json:"besoin"`
	RepoURL      string              `json:"depot_url"`
	Budget       int64               `json:"budget_credits"`
	Days         int                 `json:"delai_jours"`
	Milestones   []catalog.Milestone `json:"jalons"`
	Status       string              `json:"statut"`
	PublishedAt  time.Time           `json:"date_publication"`
	ExpiresAt    time.Time           `json:"date_expiration"`
	Pack         offerPackJSON       `json:"pack"`
	Author       memberSummaryJSON   `json:"auteur"`
	Applications int                 `json:"candidatures_en_attente"`
	MyApp        *uuid.UUID          `json:"ma_candidature"`
	ExchangeID   *uuid.UUID          `json:"id_echange"`
}

type offerPackJSON struct {
	ID        uuid.UUID         `json:"id"`
	Title     string            `json:"titre"`
	ProofType catalog.ProofType `json:"type_preuve"`
	Skills    []string          `json:"competences"`
}

func toAnnouncementJSON(v app.AnnouncementView) announcementJSON {
	return announcementJSON{ID: v.ID, Title: v.Title, Need: v.Need, RepoURL: v.Repo.URL(), Budget: v.Budget, Days: v.Days,
		Milestones: v.Milestones, Status: v.Status, PublishedAt: v.PublishedAt, ExpiresAt: v.ExpiresAt,
		Pack:   offerPackJSON{ID: v.Pack.ID, Title: v.Pack.Title, ProofType: v.Pack.ProofType, Skills: v.Pack.Skills},
		Author: memberSummaryJSON(v.Author), Applications: v.Applications, MyApp: v.MyApp, ExchangeID: v.ExchangeID}
}

type applicationJSON struct {
	ID             uuid.UUID           `json:"id"`
	AnnouncementID uuid.UUID           `json:"id_annonce"`
	Price          int64               `json:"prix_credits"`
	Days           int                 `json:"delai_jours"`
	Message        string              `json:"message"`
	Milestones     []catalog.Milestone `json:"jalons"`
	Status         string              `json:"statut"`
	CreatedAt      time.Time           `json:"date_candidature"`
	DecidedAt      *time.Time          `json:"date_decision"`
	Provider       memberSummaryJSON   `json:"prestataire"`
}

func toApplicationJSON(v app.ApplicationView) applicationJSON {
	return applicationJSON{ID: v.ID, AnnouncementID: v.AnnouncementID, Price: v.Price, Days: v.Days, Message: v.Message,
		Milestones: v.Milestones, Status: v.Status, CreatedAt: v.CreatedAt, DecidedAt: v.DecidedAt, Provider: memberSummaryJSON(v.Provider)}
}

func (h announcementHandlers) create(c *gin.Context) {
	var b struct {
		PackID     uuid.UUID           `json:"id_pack"`
		Title      string              `json:"titre"`
		Need       string              `json:"besoin"`
		RepoURL    string              `json:"depot_url"`
		Budget     int64               `json:"budget_credits"`
		Days       int                 `json:"delai_jours"`
		Milestones []catalog.Milestone `json:"jalons"`
	}
	if !bindJSON(c, &b) {
		return
	}
	v, err := h.svc.PublishAnnouncement(c.Request.Context(), actor(c), app.AnnouncementInput(b))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, toAnnouncementJSON(v))
}

func (h announcementHandlers) list(c *gin.Context) {
	limit, ok := queryInt(c, "limit", 20)
	if !ok {
		return
	}
	f := app.AnnouncementFilter{Q: c.Query("q"), Mine: c.Query("mes") == "true"}
	if v := c.Query("pack"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			fail(c, &app.Error{Kind: app.KindInvalid, Code: "parametre-invalide", Title: "Requête invalide", Detail: "pack doit être un identifiant."})
			return
		}
		f.PackID = &id
	}
	p, err := h.svc.ListAnnouncements(c.Request.Context(), optionalActor(c), f, limit, c.Query("cursor"))
	if err != nil {
		fail(c, err)
		return
	}
	items := make([]announcementJSON, len(p.Items))
	for i, v := range p.Items {
		items[i] = toAnnouncementJSON(v)
	}
	c.JSON(http.StatusOK, pageJSON(items, p.NextCursor))
}

func (h announcementHandlers) get(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	v, err := h.svc.GetAnnouncement(c.Request.Context(), optionalActor(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, toAnnouncementJSON(v))
}

func (h announcementHandlers) withdraw(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	v, err := h.svc.WithdrawAnnouncement(c.Request.Context(), actor(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, toAnnouncementJSON(v))
}

func (h announcementHandlers) apply(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var b struct {
		Price      int64               `json:"prix_credits"`
		Days       int                 `json:"delai_jours"`
		Message    string              `json:"message"`
		Milestones []catalog.Milestone `json:"jalons"`
	}
	if !bindJSON(c, &b) {
		return
	}
	v, err := h.svc.Apply(c.Request.Context(), actor(c), id, app.ApplicationInput(b))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, toApplicationJSON(v))
}

func (h announcementHandlers) applications(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	vs, err := h.svc.ListApplications(c.Request.Context(), actor(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	items := make([]applicationJSON, len(vs))
	for i, v := range vs {
		items[i] = toApplicationJSON(v)
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// decide extrait les deux identifiants d'une route /annonces/:id/candidatures/:cid/….
func decide(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return id, id, false
	}
	cid, ok := pathUUID(c, "cid")
	return id, cid, ok
}

func (h announcementHandlers) accept(c *gin.Context) {
	id, cid, ok := decide(c)
	if !ok {
		return
	}
	x, err := h.svc.AcceptApplication(c.Request.Context(), actor(c), id, cid)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, toExchangeJSON(x))
}

func (h announcementHandlers) refuse(c *gin.Context) {
	h.respondApplication(c, h.svc.RefuseApplication)
}

func (h announcementHandlers) withdrawApplication(c *gin.Context) {
	h.respondApplication(c, h.svc.WithdrawApplication)
}

func (h announcementHandlers) respondApplication(c *gin.Context, fn func(context.Context, app.Actor, uuid.UUID, uuid.UUID) (app.ApplicationView, error)) {
	id, cid, ok := decide(c)
	if !ok {
		return
	}
	v, err := fn(c.Request.Context(), actor(c), id, cid)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, toApplicationJSON(v))
}
