package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/gitb2pic/codetroc/backend/internal/app"
)

type vaultHandlers struct{ svc *app.Service }

type secretJSON struct {
	ID          uuid.UUID  `json:"id"`
	ExchangeID  uuid.UUID  `json:"id_echange"`
	DepositorID uuid.UUID  `json:"id_deposant"`
	Label       string     `json:"libelle"`
	Kind        string     `json:"type"`
	Scope       *string    `json:"portee"`
	Status      string     `json:"statut"`
	ExpiresAt   time.Time  `json:"date_expiration"`
	RevokedAt   *time.Time `json:"date_revocation"`
	CreatedAt   time.Time  `json:"date_creation"`
}

type secretAccessJSON struct {
	Action string     `json:"action"`
	UserID *uuid.UUID `json:"id_utilisateur"`
	Pseudo *string    `json:"pseudo"`
	IP     string     `json:"ip"`
	At     time.Time  `json:"date"`
}

func (h vaultHandlers) share(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var b struct {
		Label     string    `json:"libelle"`
		Kind      string    `json:"type"`
		Scope     string    `json:"portee"`
		Value     string    `json:"valeur"`
		ExpiresAt time.Time `json:"date_expiration"`
	}
	if !bindJSON(c, &b) {
		return
	}
	s, err := h.svc.ShareSecret(c.Request.Context(), actor(c), id, app.SecretInput(b), c.ClientIP())
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, secretJSON(s))
}

func (h vaultHandlers) list(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	ss, err := h.svc.ListSecrets(c.Request.Context(), actor(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	out := make([]secretJSON, len(ss))
	for i, s := range ss {
		out[i] = secretJSON(s)
	}
	c.JSON(http.StatusOK, out)
}

// reveal renvoie la valeur en clair : jamais mise en cache, jamais rejouée par
// le middleware d'idempotence (qui conserve les réponses en base).
func (h vaultHandlers) reveal(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	c.Header("Cache-Control", "no-store")
	v, err := h.svc.RevealSecret(c.Request.Context(), actor(c), id, c.ClientIP())
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"valeur": v})
}

func (h vaultHandlers) revoke(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	s, err := h.svc.RevokeSecret(c.Request.Context(), actor(c), id, c.ClientIP())
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, secretJSON(s))
}

func (h vaultHandlers) journal(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	js, err := h.svc.SecretJournal(c.Request.Context(), actor(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	out := make([]secretAccessJSON, len(js))
	for i, j := range js {
		out[i] = secretAccessJSON(j)
	}
	c.JSON(http.StatusOK, out)
}
