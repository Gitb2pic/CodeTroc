package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/gitb2pic/codetroc/backend/internal/app"
	"github.com/gitb2pic/codetroc/backend/internal/domain/catalog"
	"github.com/gitb2pic/codetroc/backend/internal/domain/exchange"
)

type exchangeHandlers struct{ svc *app.Service }

type proofJSON struct {
	ID        uuid.UUID  `json:"id"`
	Kind      string     `json:"type"`
	PRURL     *string    `json:"pr_url"`
	PRNumber  *int       `json:"pr_numero"`
	CommitSHA *string    `json:"sha_commit"`
	Status    string     `json:"statut"`
	Detail    *string    `json:"detail"`
	Note      *string    `json:"note"`
	CheckedAt *time.Time `json:"date_verification"`
	CreatedAt time.Time  `json:"date_creation"`
}

type milestoneJSON struct {
	ID          uuid.UUID                `json:"id"`
	Order       int                      `json:"ordre"`
	Title       string                   `json:"titre"`
	Amount      int64                    `json:"montant"`
	Status      exchange.MilestoneStatus `json:"statut"`
	Deadline    *time.Time               `json:"echeance"`
	DeliveredAt *time.Time               `json:"date_livraison"`
	ClosedAt    *time.Time               `json:"date_cloture"`
	Proofs      []proofJSON              `json:"preuves"`
}

type exchangeJSON struct {
	ID                uuid.UUID         `json:"id"`
	Status            exchange.Status   `json:"statut"`
	Total             int64             `json:"montant_total"`
	Days              int               `json:"delai_jours"`
	RepoURL           string            `json:"depot_url"`
	Need              string            `json:"besoin"`
	CreatedAt         time.Time         `json:"date_creation"`
	AcceptedAt        *time.Time        `json:"date_acceptation"`
	ClosedAt          *time.Time        `json:"date_cloture"`
	CancelRequestedBy *uuid.UUID        `json:"annulation_demandee_par"`
	Requester         memberSummaryJSON `json:"demandeur"`
	Provider          memberSummaryJSON `json:"prestataire"`
	Offer             struct {
		ID        *uuid.UUID        `json:"id"`
		Title     string            `json:"titre"`
		PackTitle string            `json:"pack"`
		ProofType catalog.ProofType `json:"type_preuve"`
	} `json:"offre"`
	AnnouncementID *uuid.UUID      `json:"id_annonce"`
	Milestones     []milestoneJSON `json:"jalons"`
	Escrow         struct {
		Locked   int64 `json:"bloque"`
		Released int64 `json:"libere"`
		Refunded int64 `json:"rembourse"`
	} `json:"sequestre"`
	NextActor exchange.Actor `json:"a_qui_de_jouer"`
}

func toExchangeJSON(x app.ExchangeView) exchangeJSON {
	j := exchangeJSON{ID: x.ID, Status: x.Status, Total: x.Total, Days: x.Days, RepoURL: x.Repo.URL(), Need: x.Need,
		CreatedAt: x.CreatedAt, AcceptedAt: x.AcceptedAt, ClosedAt: x.ClosedAt, CancelRequestedBy: x.CancelRequestedBy,
		Requester: memberSummaryJSON(x.Requester), Provider: memberSummaryJSON(x.Provider), NextActor: x.NextActor,
		AnnouncementID: x.Offer.AnnouncementID}
	j.Offer.ID, j.Offer.Title, j.Offer.PackTitle, j.Offer.ProofType = x.Offer.ID, x.Offer.Title, x.Offer.PackTitle, x.Offer.ProofType
	j.Escrow.Locked, j.Escrow.Released, j.Escrow.Refunded = x.Escrow.Locked, x.Escrow.Released, x.Escrow.Refunded
	j.Milestones = make([]milestoneJSON, len(x.Milestones))
	for i, m := range x.Milestones {
		proofs := make([]proofJSON, len(m.Proofs))
		for k, p := range m.Proofs {
			proofs[k] = proofJSON(p)
		}
		j.Milestones[i] = milestoneJSON{ID: m.ID, Order: m.Order, Title: m.Title, Amount: m.Amount, Status: m.Status,
			Deadline: m.Deadline, DeliveredAt: m.DeliveredAt, ClosedAt: m.ClosedAt, Proofs: proofs}
	}
	return j
}

func (h exchangeHandlers) create(c *gin.Context) {
	var b struct {
		OfferID    uuid.UUID           `json:"id_offre"`
		RepoURL    string              `json:"depot_url"`
		Need       string              `json:"besoin"`
		Milestones []catalog.Milestone `json:"jalons"`
	}
	if !bindJSON(c, &b) {
		return
	}
	x, err := h.svc.RequestExchange(c.Request.Context(), actor(c), app.ExchangeRequest{OfferID: b.OfferID, RepoURL: b.RepoURL, Need: b.Need, Milestones: b.Milestones})
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, toExchangeJSON(x))
}

func (h exchangeHandlers) list(c *gin.Context) {
	limit, ok := queryInt(c, "limit", 20)
	if !ok {
		return
	}
	p, err := h.svc.ListExchanges(c.Request.Context(), actor(c), app.ExchangeFilter{Role: c.Query("role"), Status: exchange.Status(c.Query("statut"))}, limit, c.Query("cursor"))
	if err != nil {
		fail(c, err)
		return
	}
	items := make([]exchangeJSON, len(p.Items))
	for i, x := range p.Items {
		items[i] = toExchangeJSON(x)
	}
	c.JSON(http.StatusOK, pageJSON(items, p.NextCursor))
}

func (h exchangeHandlers) get(c *gin.Context) {
	h.respond(c, func(a app.Actor, id uuid.UUID) (app.ExchangeView, error) {
		return h.svc.GetExchange(c.Request.Context(), a, id)
	})
}

func (h exchangeHandlers) respond(c *gin.Context, fn func(app.Actor, uuid.UUID) (app.ExchangeView, error)) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	x, err := fn(actor(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, toExchangeJSON(x))
}

func (h exchangeHandlers) accept(c *gin.Context) {
	h.respond(c, func(a app.Actor, id uuid.UUID) (app.ExchangeView, error) {
		return h.svc.AcceptExchange(c.Request.Context(), a, id)
	})
}

func (h exchangeHandlers) refuse(c *gin.Context) {
	h.respond(c, func(a app.Actor, id uuid.UUID) (app.ExchangeView, error) {
		return h.svc.RefuseExchange(c.Request.Context(), a, id)
	})
}

func (h exchangeHandlers) cancel(c *gin.Context) {
	h.respond(c, func(a app.Actor, id uuid.UUID) (app.ExchangeView, error) {
		return h.svc.CancelExchange(c.Request.Context(), a, id)
	})
}
