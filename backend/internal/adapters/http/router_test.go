package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gitb2pic/codetroc/backend/api"
)

type fakeDB struct{ err error }

func (f fakeDB) Ping(context.Context) error { return f.err }

type harness struct {
	router *gin.Engine
	logs   *bytes.Buffer
}

func newHarness(t *testing.T, mutate func(*Options)) harness {
	t.Helper()
	logs := &bytes.Buffer{}
	opts := Options{
		Logger:       slog.New(slog.NewJSONHandler(logs, nil)),
		DB:           fakeDB{},
		MaxBodyBytes: 64,
		ServeSpec:    true,
		Spec:         api.Spec,
	}
	if mutate != nil {
		mutate(&opts)
	}
	r, err := NewRouter(opts)
	require.NoError(t, err)
	return harness{router: r, logs: logs}
}

func (h harness) do(req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	return w
}

func decodeProblem(t *testing.T, w *httptest.ResponseRecorder) Problem {
	t.Helper()
	assert.Equal(t, "application/problem+json", w.Header().Get("Content-Type"))
	var p Problem
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &p))
	return p
}

func TestHealth(t *testing.T) {
	w := newHarness(t, nil).do(httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"status":"ok"}`, w.Body.String())
}

func TestReady(t *testing.T) {
	t.Run("base joignable", func(t *testing.T) {
		w := newHarness(t, nil).do(httptest.NewRequest(http.MethodGet, "/api/v1/ready", nil))
		assert.Equal(t, http.StatusOK, w.Code)
		assert.JSONEq(t, `{"status":"ready"}`, w.Body.String())
	})
	t.Run("base injoignable", func(t *testing.T) {
		h := newHarness(t, func(o *Options) { o.DB = fakeDB{err: errors.New("dial tcp 10.0.0.5:5432: connection refused")} })
		w := h.do(httptest.NewRequest(http.MethodGet, "/api/v1/ready", nil))
		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		p := decodeProblem(t, w)
		assert.Equal(t, "/problems/service-indisponible", p.Type)
		assert.NotContains(t, w.Body.String(), "10.0.0.5", "l'erreur interne ne doit pas fuiter")
		assert.Contains(t, h.logs.String(), "connection refused", "l'erreur interne doit être journalisée")
	})
}

func TestProblemFormatOnUnknownRouteAndMethod(t *testing.T) {
	h := newHarness(t, nil)

	w := h.do(httptest.NewRequest(http.MethodGet, "/api/v1/inconnu", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)
	p := decodeProblem(t, w)
	assert.Equal(t, "/problems/introuvable", p.Type)
	assert.Equal(t, 404, p.Status)
	assert.Equal(t, "/api/v1/inconnu", p.Instance)
	assert.Equal(t, w.Header().Get(RequestIDHeader), p.RequestID)

	w = h.do(httptest.NewRequest(http.MethodDelete, "/api/v1/health", nil))
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	assert.Equal(t, "/problems/methode-non-autorisee", decodeProblem(t, w).Type)
}

func TestPanicIsRecovered(t *testing.T) {
	h := newHarness(t, nil)
	h.router.GET("/api/v1/boom", func(*gin.Context) { panic("mot de passe = hunter2") })

	w := h.do(httptest.NewRequest(http.MethodGet, "/api/v1/boom", nil))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	p := decodeProblem(t, w)
	assert.Equal(t, "/problems/erreur-interne", p.Type)
	assert.NotContains(t, w.Body.String(), "hunter2")
	assert.Contains(t, h.logs.String(), "hunter2")
}

func TestRequestID(t *testing.T) {
	h := newHarness(t, nil)

	t.Run("généré si absent", func(t *testing.T) {
		w := h.do(httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
		assert.Regexp(t, `^[0-9a-f]{32}$`, w.Header().Get(RequestIDHeader))
	})
	t.Run("repris s'il est valide", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		req.Header.Set(RequestIDHeader, "traefik-7f3a.91c2_x")
		assert.Equal(t, "traefik-7f3a.91c2_x", h.do(req).Header().Get(RequestIDHeader))
	})
	for name, bad := range map[string]string{
		"trop long":         strings.Repeat("a", 65),
		"retour à la ligne": "abc\ninjected=1",
		"espace":            "abc def",
	} {
		t.Run("remplacé si invalide : "+name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
			req.Header.Set(RequestIDHeader, bad)
			assert.Regexp(t, `^[0-9a-f]{32}$`, h.do(req).Header().Get(RequestIDHeader))
		})
	}
}

func TestSecurityHeaders(t *testing.T) {
	h := newHarness(t, nil)
	for _, path := range []string{"/api/v1/health", "/api/v1/inconnu"} {
		w := h.do(httptest.NewRequest(http.MethodGet, path, nil))
		assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"), path)
		assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"), path)
		assert.Equal(t, "no-referrer", w.Header().Get("Referrer-Policy"), path)
		assert.Equal(t, "default-src 'none'; frame-ancestors 'none'", w.Header().Get("Content-Security-Policy"), path)
		assert.Equal(t, "no-store", w.Header().Get("Cache-Control"), path)
	}
}

func TestBodyLimit(t *testing.T) {
	h := newHarness(t, nil) // limite : 64 octets
	h.router.POST("/api/v1/echo", func(c *gin.Context) {
		if _, err := io.ReadAll(c.Request.Body); err != nil {
			AbortWithError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	})
	big := strings.Repeat("x", 65)

	t.Run("longueur annoncée trop grande", func(t *testing.T) {
		w := h.do(httptest.NewRequest(http.MethodPost, "/api/v1/echo", strings.NewReader(big)))
		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
		assert.Equal(t, "/problems/corps-trop-volumineux", decodeProblem(t, w).Type)
	})
	t.Run("longueur inconnue (chunked)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/echo", io.NopCloser(strings.NewReader(big)))
		req.ContentLength = -1
		w := h.do(req)
		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
		assert.Equal(t, "/problems/corps-trop-volumineux", decodeProblem(t, w).Type)
	})
	t.Run("sous la limite", func(t *testing.T) {
		w := h.do(httptest.NewRequest(http.MethodPost, "/api/v1/echo", strings.NewReader("petit")))
		assert.Equal(t, http.StatusNoContent, w.Code)
	})
}

func TestSpecIsHiddenInProduction(t *testing.T) {
	w := newHarness(t, nil).do(httptest.NewRequest(http.MethodGet, "/api/v1/openapi.yaml", nil))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "openapi: 3.0.3")

	w = newHarness(t, func(o *Options) { o.ServeSpec = false }).do(httptest.NewRequest(http.MethodGet, "/api/v1/openapi.yaml", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAccessLogOmitsQueryAndHeaders(t *testing.T) {
	h := newHarness(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health?token=secret-query", nil)
	req.Header.Set("Authorization", "Bearer secret-header")
	w := h.do(req)

	var line map[string]any
	require.NoError(t, json.Unmarshal(lastLine(h.logs), &line))
	assert.Equal(t, "requête", line["msg"])
	assert.Equal(t, "/api/v1/health", line["path"])
	assert.Equal(t, float64(200), line["status"])
	assert.Equal(t, w.Header().Get(RequestIDHeader), line["request_id"])
	assert.NotContains(t, h.logs.String(), "secret-query")
	assert.NotContains(t, h.logs.String(), "secret-header")
}

func TestOptionsAreValidated(t *testing.T) {
	_, err := NewRouter(Options{Logger: slog.Default(), DB: fakeDB{}, MaxBodyBytes: 1, TrustedProxies: []string{"pas-une-ip"}})
	require.Error(t, err)
	_, err = NewRouter(Options{Logger: slog.Default(), MaxBodyBytes: 1})
	require.Error(t, err, "DB est obligatoire")
}

func lastLine(b *bytes.Buffer) []byte {
	lines := bytes.Split(bytes.TrimSpace(b.Bytes()), []byte("\n"))
	return lines[len(lines)-1]
}
