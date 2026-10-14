package postgres

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"io"
	"log/slog"

	"github.com/gitb2pic/codetroc/backend/api"
	httpapi "github.com/gitb2pic/codetroc/backend/internal/adapters/http"
	"github.com/gitb2pic/codetroc/backend/internal/testutil"
)

func TestMigrationsUpDownUp(t *testing.T) {
	url := testutil.DatabaseURL(t)

	require.NoError(t, MigrateUp(url))
	require.NoError(t, MigrateUp(url), "relancer les migrations à jour ne doit rien faire")
	require.NoError(t, MigrateDown(url))
	require.NoError(t, MigrateUp(url))
}

func TestReadyAgainstRealDatabase(t *testing.T) {
	url := testutil.DatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := Open(ctx, url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	r, err := httpapi.NewRouter(httpapi.Options{
		Logger:       slog.New(slog.NewJSONHandler(io.Discard, nil)),
		DB:           pool,
		MaxBodyBytes: 1 << 20,
		Spec:         api.Spec,
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/ready", nil))
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestOpenRejectsInvalidURL(t *testing.T) {
	_, err := Open(context.Background(), "://pas une url")
	require.Error(t, err)
}
