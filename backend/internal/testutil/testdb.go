// Package testutil fournit les aides communes aux tests d'intégration.
package testutil

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gitb2pic/codetroc/backend/migrations"
)

// DatabaseURL renvoie l'URL de la base de test (CODETROC_TEST_DATABASE_URL).
// Sans base, le test est ignoré en local ; en CI, CODETROC_REQUIRE_DB=1 le
// fait échouer pour qu'aucun test d'intégration ne soit sauté sans bruit.
func DatabaseURL(t testing.TB) string {
	t.Helper()
	u := os.Getenv("CODETROC_TEST_DATABASE_URL")
	if u == "" {
		if os.Getenv("CODETROC_REQUIRE_DB") == "1" {
			t.Fatal("CODETROC_TEST_DATABASE_URL est requis (CODETROC_REQUIRE_DB=1)")
		}
		t.Skip("test d'intégration : définir CODETROC_TEST_DATABASE_URL pour l'exécuter")
	}
	return u
}

// Migrator applique les migrations sur une URL ; fourni par le paquet postgres
// pour éviter une dépendance circulaire.
type Migrator func(databaseURL string) error

// NewDB crée une base vide et migrée, propre au test, détruite à la fin.
// Les bases sont clonées depuis un modèle migré une seule fois par version
// des migrations, ce qui rend chaque création quasi instantanée.
func NewDB(t testing.TB, migrate Migrator) (*pgxpool.Pool, string) {
	t.Helper()
	base := DatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatalf("connexion à la base de test : %v", err)
	}
	defer func() { _ = admin.Close(context.Background()) }()

	tpl := "codetroc_tpl_" + migrationsHash(t)
	ensureTemplate(ctx, t, admin, base, tpl, migrate)

	name := "codetroc_t_" + randomHex(t, 6)
	if _, err := admin.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s", name, tpl)); err != nil {
		t.Fatalf("création de la base %s : %v", name, err)
	}
	dbURL := withDatabase(t, base, name)
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("pool : %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		c, err := pgx.Connect(context.Background(), base)
		if err != nil {
			return
		}
		defer func() { _ = c.Close(context.Background()) }()
		_, _ = c.Exec(context.Background(), fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", name))
	})
	return pool, dbURL
}

func ensureTemplate(ctx context.Context, t testing.TB, admin *pgx.Conn, base, tpl string, migrate Migrator) {
	t.Helper()
	// Verrou consultatif : plusieurs paquets de tests tournent en parallèle.
	if _, err := admin.Exec(ctx, "SELECT pg_advisory_lock(424242)"); err != nil {
		t.Fatalf("verrou du modèle : %v", err)
	}
	defer func() { _, _ = admin.Exec(context.Background(), "SELECT pg_advisory_unlock(424242)") }()

	var exists bool
	if err := admin.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)", tpl).Scan(&exists); err != nil {
		t.Fatalf("recherche du modèle : %v", err)
	}
	if exists {
		return
	}
	building := tpl + "_build"
	_, _ = admin.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", building))
	if _, err := admin.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", building)); err != nil {
		t.Fatalf("création du modèle : %v", err)
	}
	if err := migrate(withDatabase(t, base, building)); err != nil {
		t.Fatalf("migration du modèle : %v", err)
	}
	if _, err := admin.Exec(ctx, fmt.Sprintf("ALTER DATABASE %s RENAME TO %s", building, tpl)); err != nil {
		t.Fatalf("finalisation du modèle : %v", err)
	}
}

func migrationsHash(t testing.TB) string {
	t.Helper()
	names, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(names)
	h := sha256.New()
	for _, n := range names {
		b, err := fs.ReadFile(migrations.FS, n)
		if err != nil {
			t.Fatal(err)
		}
		h.Write([]byte(n))
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}

func withDatabase(t testing.TB, base, name string) string {
	t.Helper()
	u, err := url.Parse(base)
	if err != nil {
		t.Fatalf("URL de test invalide : %v", err)
	}
	u.Path = "/" + name
	return u.String()
}

func randomHex(t testing.TB, n int) string {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}
