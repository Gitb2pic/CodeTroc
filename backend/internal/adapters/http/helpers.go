package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/gitb2pic/codetroc/backend/internal/app"
)

var kindStatus = map[app.Kind]int{
	app.KindInvalid:         http.StatusBadRequest,
	app.KindUnauthenticated: http.StatusUnauthorized,
	app.KindForbidden:       http.StatusForbidden,
	app.KindNotFound:        http.StatusNotFound,
	app.KindConflict:        http.StatusConflict,
	app.KindUnprocessable:   http.StatusUnprocessableEntity,
	app.KindGone:            http.StatusGone,
	app.KindUnavailable:     http.StatusServiceUnavailable,
}

// fail traduit une erreur en problème RFC 9457. Les erreurs métier sont
// présentées telles quelles ; toute autre erreur devient une 500 générique.
func fail(c *gin.Context, err error) {
	var ae *app.Error
	if errors.As(err, &ae) {
		if ae.Kind == app.KindUnauthenticated {
			c.Header("WWW-Authenticate", `Bearer realm="codetroc"`)
		}
		WriteProblem(c, Problem{Type: "/problems/" + ae.Code, Title: ae.Title, Status: kindStatus[ae.Kind], Detail: ae.Detail})
		return
	}
	AbortWithError(c, err)
}

var errBadBody = &app.Error{Kind: app.KindInvalid, Code: "corps-invalide", Title: "Requête invalide"}

// bindJSON décode un objet JSON strict : champ inconnu, type erroné ou
// contenu après l'objet sont refusés (protection contre l'affectation de
// masse : un client ne peut pas glisser « statut » ou « montant »).
func bindJSON(c *gin.Context, dst any) bool {
	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			WriteProblem(c, ErrBodyTooLarge)
			return false
		}
		e := *errBadBody
		e.Detail = "Corps JSON invalide : " + err.Error()
		fail(c, &e)
		return false
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		e := *errBadBody
		e.Detail = "Un seul objet JSON est attendu."
		fail(c, &e)
		return false
	}
	return true
}

// pathUUID lit un identifiant de chemin ; un identifiant mal formé vaut 404.
func pathUUID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		WriteProblem(c, ErrNotFound)
		return uuid.Nil, false
	}
	return id, true
}

// queryInt lit un entier de la chaîne de requête, avec valeur par défaut.
func queryInt(c *gin.Context, name string, def int) (int, bool) {
	v := c.Query(name)
	if v == "" {
		return def, true
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		e := *errBadBody
		e.Code = "parametre-invalide"
		e.Detail = "Le paramètre " + name + " doit être un entier."
		fail(c, &e)
		return 0, false
	}
	return n, true
}

// jsonBody est utilisé par les tests pour construire un corps de requête.
func jsonBody(v any) *bytes.Reader {
	b, _ := json.Marshal(v)
	return bytes.NewReader(b)
}
