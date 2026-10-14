package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/gitb2pic/codetroc/backend/internal/app"
)

type socialHandlers struct{ svc *app.Service }

type reviewJSON struct {
	ID           uuid.UUID `json:"id"`
	ExchangeID   uuid.UUID `json:"id_echange"`
	AuthorID     uuid.UUID `json:"id_auteur"`
	AuthorPseudo string    `json:"auteur"`
	RecipientID  uuid.UUID `json:"id_destinataire"`
	Rating       int       `json:"note"`
	Comment      *string   `json:"commentaire"`
	At           time.Time `json:"date"`
}

type messageJSON struct {
	ID           uuid.UUID `json:"id"`
	AuthorID     uuid.UUID `json:"id_auteur"`
	AuthorPseudo string    `json:"auteur"`
	Text         string    `json:"contenu"`
	At           time.Time `json:"date"`
}

func (h socialHandlers) leaveReview(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var b struct {
		Rating  int    `json:"note"`
		Comment string `json:"commentaire"`
	}
	if !bindJSON(c, &b) {
		return
	}
	r, err := h.svc.LeaveReview(c.Request.Context(), actor(c), id, b.Rating, b.Comment)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, reviewJSON(r))
}

func (h socialHandlers) listReviews(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	limit, ok := queryInt(c, "limit", 20)
	if !ok {
		return
	}
	p, err := h.svc.ListReviews(c.Request.Context(), id, limit, c.Query("cursor"))
	if err != nil {
		fail(c, err)
		return
	}
	items := make([]reviewJSON, len(p.Items))
	for i, r := range p.Items {
		items[i] = reviewJSON(r)
	}
	c.JSON(http.StatusOK, pageJSON(items, p.NextCursor))
}

func (h socialHandlers) postMessage(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var b struct {
		Text string `json:"contenu"`
	}
	if !bindJSON(c, &b) {
		return
	}
	m, err := h.svc.PostMessage(c.Request.Context(), actor(c), id, b.Text)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, messageJSON(m))
}

func (h socialHandlers) listMessages(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	limit, ok := queryInt(c, "limit", 50)
	if !ok {
		return
	}
	p, err := h.svc.ListMessages(c.Request.Context(), actor(c), id, limit, c.Query("cursor"))
	if err != nil {
		fail(c, err)
		return
	}
	items := make([]messageJSON, len(p.Items))
	for i, m := range p.Items {
		items[i] = messageJSON(m)
	}
	c.JSON(http.StatusOK, pageJSON(items, p.NextCursor))
}
