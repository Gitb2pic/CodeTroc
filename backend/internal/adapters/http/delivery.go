package httpapi

import (
	"io"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"

	ghadapter "github.com/gitb2pic/codetroc/backend/internal/adapters/github"
	"github.com/gitb2pic/codetroc/backend/internal/app"
)

type deliveryHandlers struct {
	svc      *app.Service
	webhooks *WebhookConfig
}

func (h deliveryHandlers) submitProof(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var b struct {
		PRURL string `json:"pr_url"`
		Note  string `json:"note"`
	}
	if !bindJSON(c, &b) {
		return
	}
	x, err := h.svc.SubmitProof(c.Request.Context(), actor(c), id, app.ProofInput{PRURL: b.PRURL, Note: b.Note})
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, toExchangeJSON(x))
}

func (h deliveryHandlers) reverify(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	x, err := h.svc.ReverifyMilestone(c.Request.Context(), actor(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, toExchangeJSON(x))
}

func (h deliveryHandlers) validate(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	x, err := h.svc.ValidateMilestone(c.Request.Context(), actor(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, toExchangeJSON(x))
}

var validDelivery = regexp.MustCompile(`^[A-Za-z0-9-]{1,100}$`)

var (
	errBadSignature = &app.Error{Kind: app.KindUnauthenticated, Code: "signature-invalide", Title: "Webhook refusé", Detail: "Signature X-Hub-Signature-256 absente ou invalide."}
	errNoWebhooks   = &app.Error{Kind: app.KindUnavailable, Code: "webhooks-non-configures", Title: "Webhooks indisponibles", Detail: "La GitHub App n'est pas configurée sur ce serveur."}
	errBadDelivery  = &app.Error{Kind: app.KindInvalid, Code: "livraison-invalide", Title: "Webhook refusé", Detail: "En-têtes X-GitHub-Delivery ou X-GitHub-Event absents ou invalides."}
)

// githubWebhook authentifie le webhook (HMAC SHA-256 en temps constant)
// avant toute lecture du contenu, puis le traite (UC10).
func (h deliveryHandlers) githubWebhook(c *gin.Context) {
	if h.webhooks == nil {
		fail(c, errNoWebhooks)
		return
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		AbortWithError(c, err)
		return
	}
	if !ghadapter.VerifySignature(h.webhooks.Secret, body, c.GetHeader("X-Hub-Signature-256")) {
		fail(c, errBadSignature)
		return
	}
	delivery, event := c.GetHeader("X-GitHub-Delivery"), c.GetHeader("X-GitHub-Event")
	if !validDelivery.MatchString(delivery) || event == "" || len(event) > 50 {
		fail(c, errBadDelivery)
		return
	}
	res, err := h.svc.HandleGitHubWebhook(c.Request.Context(), delivery, event, body)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"doublon": res.Duplicate, "preuves_verifiees": res.Verified})
}
