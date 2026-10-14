package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestIDHeader porte l'identifiant de corrélation, posé par Traefik ou par l'API.
const RequestIDHeader = "X-Request-Id"

const requestIDKey = "request_id"

// Un id entrant n'est repris que s'il est court et sans caractère spécial,
// pour qu'il ne puisse pas injecter de contenu dans les logs.
var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(RequestIDHeader)
		if !validRequestID.MatchString(id) {
			id = newRequestID()
		}
		c.Set(requestIDKey, id)
		c.Header(RequestIDHeader, id)
		c.Next()
	}
}

// RequestID renvoie l'identifiant de la requête en cours.
func RequestID(c *gin.Context) string { return c.GetString(requestIDKey) }

func newRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // crypto/rand ne renvoie pas d'erreur depuis Go 1.24
	return hex.EncodeToString(b)
}

// accessLog écrit une ligne par requête. Seul le chemin est journalisé :
// la chaîne de requête et les en-têtes peuvent contenir des jetons.
func accessLog(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		attrs := []any{
			slog.String("request_id", RequestID(c)),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.String("route", c.FullPath()),
			slog.Int("status", c.Writer.Status()),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("error", c.Errors.String()))
		}
		level := slog.LevelInfo
		if c.Writer.Status() >= http.StatusInternalServerError {
			level = slog.LevelError
		}
		logger.Log(c.Request.Context(), level, "requête", attrs...)
	}
}

// recovery transforme une panique en 500 générique et journalise sa cause.
func recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(rec) // connexion interrompue volontairement : comportement standard
			}
			logger.Error("panique",
				slog.String("request_id", RequestID(c)),
				slog.Any("panic", rec),
				slog.String("stack", string(debug.Stack())),
			)
			WriteProblem(c, ErrInternal)
		}()
		c.Next()
	}
}

// securityHeaders pose les en-têtes adaptés à une API JSON.
func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		h.Set("Cache-Control", "no-store")
		c.Next()
	}
}

// bodyLimit refuse d'emblée un corps annoncé trop gros et borne la lecture
// des corps de longueur inconnue.
func bodyLimit(max int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > max {
			WriteProblem(c, ErrBodyTooLarge)
			return
		}
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, max)
		}
		c.Next()
	}
}
