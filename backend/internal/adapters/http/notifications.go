package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/gitb2pic/codetroc/backend/internal/app"
)

type notificationHandlers struct{ svc *app.Service }

type notificationJSON struct {
	ID         uuid.UUID      `json:"id"`
	Kind       string         `json:"type"`
	ExchangeID *uuid.UUID     `json:"id_echange"`
	Data       map[string]any `json:"donnees"`
	Read       bool           `json:"lue"`
	At         time.Time      `json:"date"`
}

func (h notificationHandlers) list(c *gin.Context) {
	limit, ok := queryInt(c, "limit", 20)
	if !ok {
		return
	}
	p, err := h.svc.ListNotifications(c.Request.Context(), actor(c), c.Query("non_lues") == "true", limit, c.Query("cursor"))
	if err != nil {
		fail(c, err)
		return
	}
	items := make([]notificationJSON, len(p.Items))
	for i, n := range p.Items {
		items[i] = notificationJSON(n)
	}
	c.JSON(http.StatusOK, pageJSON(items, p.NextCursor))
}

func (h notificationHandlers) count(c *gin.Context) {
	n, err := h.svc.UnreadCount(c.Request.Context(), actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"non_lues": n})
}

func (h notificationHandlers) markRead(c *gin.Context) {
	var b struct {
		IDs []uuid.UUID `json:"ids"`
	}
	if !bindJSON(c, &b) {
		return
	}
	n, err := h.svc.MarkNotificationsRead(c.Request.Context(), actor(c), b.IDs)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"marquees": n})
}
