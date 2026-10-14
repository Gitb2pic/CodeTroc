// Package testutil fournit les aides communes aux tests d'intégration.
package testutil

import (
	"os"
	"testing"
)

// DatabaseURL renvoie l'URL de la base de test (CODETROC_TEST_DATABASE_URL).
// Sans base, le test est ignoré en local ; en CI, CODETROC_REQUIRE_DB=1 le
// fait échouer pour qu'aucun test d'intégration ne soit sauté sans bruit.
func DatabaseURL(t testing.TB) string {
	t.Helper()
	url := os.Getenv("CODETROC_TEST_DATABASE_URL")
	if url == "" {
		if os.Getenv("CODETROC_REQUIRE_DB") == "1" {
			t.Fatal("CODETROC_TEST_DATABASE_URL est requis (CODETROC_REQUIRE_DB=1)")
		}
		t.Skip("test d'intégration : définir CODETROC_TEST_DATABASE_URL pour l'exécuter")
	}
	return url
}
