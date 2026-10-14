package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/gitb2pic/codetroc/backend/internal/app"
)

type disputeHandlers struct{ svc *app.Service }

type disputeJSON struct {
	ID            uuid.UUID  `json:"id"`
	ExchangeID    uuid.UUID  `json:"id_echange"`
	MilestoneID   uuid.UUID  `json:"id_jalon"`
	MilestoneName string     `json:"jalon"`
	Amount        int64      `json:"montant"`
	OpenedBy      uuid.UUID  `json:"ouvert_par"`
	Reason        string     `json:"motif"`
	Response      *string    `json:"reponse"`
	ArbiterID     *uuid.UUID `json:"id_arbitre"`
	Status        string     `json:"statut"`
	Decision      *string    `json:"decision"`
	ProviderShare *int       `json:"part_prestataire"`
	Justification *string    `json:"justification"`
	OpenedAt      time.Time  `json:"date_ouverture"`
	ClosedAt      *time.Time `json:"date_cloture"`
}

func (h disputeHandlers) reply(c *gin.Context, code int, d app.DisputeView, err error) {
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(code, disputeJSON(d))
}

func (h disputeHandlers) open(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var b struct {
		Reason string `json:"motif"`
	}
	if !bindJSON(c, &b) {
		return
	}
	d, err := h.svc.OpenDispute(c.Request.Context(), actor(c), id, b.Reason)
	h.reply(c, http.StatusCreated, d, err)
}

func (h disputeHandlers) list(c *gin.Context) {
	ds, err := h.svc.ListDisputes(c.Request.Context(), actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	out := make([]disputeJSON, len(ds))
	for i, d := range ds {
		out[i] = disputeJSON(d)
	}
	c.JSON(http.StatusOK, out)
}

func (h disputeHandlers) get(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	d, err := h.svc.GetDispute(c.Request.Context(), actor(c), id)
	h.reply(c, http.StatusOK, d, err)
}

func (h disputeHandlers) respond(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var b struct {
		Text string `json:"reponse"`
	}
	if !bindJSON(c, &b) {
		return
	}
	d, err := h.svc.RespondToDispute(c.Request.Context(), actor(c), id, b.Text)
	h.reply(c, http.StatusOK, d, err)
}

func (h disputeHandlers) assign(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var b struct {
		ArbiterID uuid.UUID `json:"id_arbitre"`
	}
	if !bindJSON(c, &b) {
		return
	}
	d, err := h.svc.AssignDispute(c.Request.Context(), actor(c), id, b.ArbiterID)
	h.reply(c, http.StatusOK, d, err)
}

func (h disputeHandlers) decide(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var b struct {
		Kind          string `json:"decision"`
		ProviderShare int    `json:"part_prestataire"`
		Justification string `json:"justification"`
	}
	if !bindJSON(c, &b) {
		return
	}
	d, err := h.svc.DecideDispute(c.Request.Context(), actor(c), id, app.Decision(b))
	h.reply(c, http.StatusOK, d, err)
}
