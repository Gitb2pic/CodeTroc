package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/gitb2pic/codetroc/backend/internal/app"
	"github.com/gitb2pic/codetroc/backend/internal/domain/ledger"
)

type creditHandlers struct{ svc *app.Service }

func (h creditHandlers) balance(c *gin.Context) {
	b, err := h.svc.Balance(c.Request.Context(), actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"disponible": b.Available, "sequestre_sortant": b.EscrowOut, "sequestre_entrant": b.EscrowIn})
}

type movementJSON struct {
	ID          uuid.UUID   `json:"id"`
	Kind        ledger.Kind `json:"type"`
	Amount      int64       `json:"montant"`
	At          time.Time   `json:"date"`
	ExchangeID  *uuid.UUID  `json:"id_echange"`
	MilestoneID *uuid.UUID  `json:"id_jalon"`
}

func (h creditHandlers) movements(c *gin.Context) {
	limit, ok := queryInt(c, "limit", 20)
	if !ok {
		return
	}
	p, err := h.svc.Movements(c.Request.Context(), actor(c), limit, c.Query("cursor"))
	if err != nil {
		fail(c, err)
		return
	}
	items := make([]movementJSON, len(p.Items))
	for i, m := range p.Items {
		items[i] = movementJSON(m)
	}
	c.JSON(http.StatusOK, pageJSON(items, p.NextCursor))
}

// pageJSON met en forme une page : { items, next_cursor } (null à la fin).
func pageJSON[T any](items []T, next string) gin.H {
	var cur *string
	if next != "" {
		cur = &next
	}
	return gin.H{"items": items, "next_cursor": cur}
}
