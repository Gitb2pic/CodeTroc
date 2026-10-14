package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"

	"github.com/gitb2pic/codetroc/backend/internal/app"
)

const idempotencyHeader = "Idempotency-Key"

var validIdemKey = regexp.MustCompile(`^[A-Za-z0-9_-]{8,100}$`)

var errIdemRequired = &app.Error{Kind: app.KindInvalid, Code: "cle-idempotence-requise", Title: "Requête invalide",
	Detail: "L'en-tête Idempotency-Key (8 à 100 caractères : lettres, chiffres, - et _) est obligatoire pour cette action."}

type recordingWriter struct {
	gin.ResponseWriter
	buf bytes.Buffer
}

func (w *recordingWriter) Write(b []byte) (int, error) {
	w.buf.Write(b)
	return w.ResponseWriter.Write(b)
}

// idempotent rend une action financière rejouable sans double effet (D4).
// La clé est propre à l'utilisateur authentifié ; le même corps rejoué
// renvoie la réponse enregistrée, un corps différent est refusé.
func idempotent(svc *app.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader(idempotencyHeader)
		if !validIdemKey.MatchString(key) {
			fail(c, errIdemRequired)
			return
		}
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			AbortWithError(c, err)
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		h := sha256.New()
		h.Write([]byte(c.Request.Method + "\n" + c.Request.URL.Path + "\n"))
		h.Write(body)

		a := actor(c)
		stored, err := svc.BeginIdempotent(c.Request.Context(), a.ID, key, c.FullPath(), h.Sum(nil))
		if err != nil {
			fail(c, err)
			return
		}
		if stored != nil {
			ct := "application/json; charset=utf-8"
			if stored.Status >= 400 {
				ct = problemContentType
			}
			c.Header("Idempotent-Replayed", "true")
			c.Data(stored.Status, ct, stored.Body)
			c.Abort()
			return
		}

		rw := &recordingWriter{ResponseWriter: c.Writer}
		c.Writer = rw
		c.Next()

		ctx := context.WithoutCancel(c.Request.Context())
		if rw.Status() >= http.StatusInternalServerError {
			_ = svc.ReleaseIdempotent(ctx, a.ID, key)
			return
		}
		if err := svc.FinishIdempotent(ctx, a.ID, key, rw.Status(), rw.buf.Bytes()); err != nil {
			_ = c.Error(err)
		}
	}
}
