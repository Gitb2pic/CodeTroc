package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Problem est une erreur au format RFC 9457.
type Problem struct {
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Detail    string `json:"detail,omitempty"`
	Instance  string `json:"instance,omitempty"`
	RequestID string `json:"request_id"`
}

const problemContentType = "application/problem+json"

// Types d'erreur communs. Les features ajoutent les leurs (solde-insuffisant…).
var (
	ErrNotFound         = Problem{Type: "/problems/introuvable", Title: "Ressource introuvable", Status: http.StatusNotFound}
	ErrMethodNotAllowed = Problem{Type: "/problems/methode-non-autorisee", Title: "Méthode non autorisée", Status: http.StatusMethodNotAllowed}
	ErrBodyTooLarge     = Problem{Type: "/problems/corps-trop-volumineux", Title: "Corps de requête trop volumineux", Status: http.StatusRequestEntityTooLarge}
	ErrUnavailable      = Problem{Type: "/problems/service-indisponible", Title: "Service momentanément indisponible", Status: http.StatusServiceUnavailable}
	ErrInternal         = Problem{Type: "/problems/erreur-interne", Title: "Erreur interne", Status: http.StatusInternalServerError}
)

// WriteProblem interrompt la requête et renvoie p, complété par le chemin et
// l'identifiant de requête.
func WriteProblem(c *gin.Context, p Problem) {
	p.Instance = c.Request.URL.Path
	p.RequestID = RequestID(c)
	c.Header("Content-Type", problemContentType)
	c.AbortWithStatusJSON(p.Status, p)
}

// AbortWithError traduit une erreur technique en problème. Les erreurs non
// reconnues deviennent une 500 générique ; leur détail reste dans les logs.
func AbortWithError(c *gin.Context, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		WriteProblem(c, ErrBodyTooLarge)
		return
	}
	_ = c.Error(err)
	WriteProblem(c, ErrInternal)
}
