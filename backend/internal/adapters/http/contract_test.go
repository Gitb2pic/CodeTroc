package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/stretchr/testify/require"

	"github.com/gitb2pic/codetroc/backend/api"
)

// loadSpec charge le contrat et le rattache à l'hôte des requêtes httptest.
func loadSpec(t *testing.T) *openapi3.T {
	t.Helper()
	doc, err := openapi3.NewLoader().LoadFromData(api.Spec)
	require.NoError(t, err)
	require.NoError(t, doc.Validate(context.Background()), "openapi.yaml invalide")
	doc.Servers = openapi3.Servers{{URL: "http://example.com/api/v1"}}
	return doc
}

// TestResponsesMatchContract vérifie chaque réponse documentée contre openapi.yaml.
func TestResponsesMatchContract(t *testing.T) {
	doc := loadSpec(t)
	router, err := gorillamux.NewRouter(doc)
	require.NoError(t, err)

	cases := []struct {
		name string
		path string
		db   fakeDB
	}{
		{"health", "/api/v1/health", fakeDB{}},
		{"ready", "/api/v1/ready", fakeDB{}},
		{"ready indisponible", "/api/v1/ready", fakeDB{err: errors.New("down")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, func(o *Options) { o.DB = tc.db })
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := h.do(req)

			route, params, err := router.FindRoute(req)
			require.NoError(t, err)
			in := &openapi3filter.ResponseValidationInput{
				RequestValidationInput: &openapi3filter.RequestValidationInput{Request: req, PathParams: params, Route: route},
				Status:                 w.Code,
				Header:                 w.Header(),
				Body:                   io.NopCloser(w.Body),
				Options:                &openapi3filter.Options{IncludeResponseStatus: true},
			}
			require.NoError(t, openapi3filter.ValidateResponse(context.Background(), in))
		})
	}
}

// TestGenericProblemsMatchSchema couvre les erreurs émises hors des routes documentées.
func TestGenericProblemsMatchSchema(t *testing.T) {
	schema := loadSpec(t).Components.Schemas["Problem"].Value
	h := newHarness(t, nil)

	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/api/v1/inconnu", nil),
		httptest.NewRequest(http.MethodPut, "/api/v1/ready", nil),
	} {
		w := h.do(req)
		var body any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		require.NoError(t, schema.VisitJSON(body), "%s %s", req.Method, req.URL.Path)
	}
}
