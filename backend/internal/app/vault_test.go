package app

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSealer « chiffre » en inversant les octets et refuse un contexte différent.
type fakeSealer struct{}

func (fakeSealer) Seal(p, aad []byte) (Sealed, error) {
	c := bytes.Clone(p)
	for i, j := 0, len(c)-1; i < j; i, j = i+1, j-1 {
		c[i], c[j] = c[j], c[i]
	}
	return Sealed{Ciphertext: c, Nonce: bytes.Clone(aad), WrappedDEK: []byte("dek"), DEKNonce: []byte("n"), KEKVersion: 1}, nil
}

func (f fakeSealer) Open(s Sealed, aad []byte) ([]byte, error) {
	if !bytes.Equal(s.Nonce, aad) {
		return nil, errors.New("contexte")
	}
	p, _ := f.Seal(s.Ciphertext, aad)
	return p.Ciphertext, nil
}

func withVault(d *Deps) { d.Vault = fakeSealer{} }

func secretInput(e *env) SecretInput {
	return SecretInput{Label: "Token de déploiement", Kind: "JETON", Scope: "repo:acme/shop-api, lecture seule",
		Value: "ghp_tresSecret", ExpiresAt: e.clock.Now().Add(7 * 24 * time.Hour)}
}

func journalActions(t *testing.T, e *env, id uuid.UUID) []string {
	t.Helper()
	rows, err := e.db.Query(context.Background(), `SELECT action FROM secret_journal WHERE id_secret = $1 ORDER BY id_entree`, id)
	require.NoError(t, err)
	var out []string
	for rows.Next() {
		var a string
		require.NoError(t, rows.Scan(&a))
		out = append(out, a)
	}
	return out
}

func TestVault_ShareRevealRevoke(t *testing.T) {
	e := newEnv(t, withVault)
	ctx := context.Background()
	d := newDeliverable(t, e)

	sec, err := e.svc.ShareSecret(ctx, d.sami, d.x.ID, secretInput(e), "203.0.113.7")
	require.NoError(t, err)
	assert.Equal(t, "ACTIF", sec.Status)

	var stored []byte
	require.NoError(t, e.db.QueryRow(ctx, `SELECT valeur_chiffree FROM secret WHERE id_secret = $1`, sec.ID).Scan(&stored))
	assert.NotContains(t, string(stored), "ghp_tresSecret", "jamais en clair en base")

	list, err := e.svc.ListSecrets(ctx, d.lea, d.x.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "Token de déploiement", list[0].Label)

	v, err := e.svc.RevealSecret(ctx, d.lea, sec.ID, "198.51.100.4")
	require.NoError(t, err)
	assert.Equal(t, "ghp_tresSecret", v)

	_, err = e.svc.RevealSecret(ctx, d.sami, sec.ID, "")
	requireKind(t, err, KindForbidden, "acces-refuse") // le déposant connaît déjà sa valeur
	_, err = e.svc.RevealSecret(ctx, d.tiers, sec.ID, "")
	requireKind(t, err, KindNotFound, "")

	_, err = e.svc.RevokeSecret(ctx, d.lea, sec.ID, "")
	requireKind(t, err, KindForbidden, "acces-refuse")
	sec, err = e.svc.RevokeSecret(ctx, d.sami, sec.ID, "203.0.113.7")
	require.NoError(t, err)
	assert.Equal(t, "REVOQUE", sec.Status)
	_, err = e.svc.RevealSecret(ctx, d.lea, sec.ID, "")
	requireKind(t, err, KindGone, "secret-indisponible")

	var wiped bool
	require.NoError(t, e.db.QueryRow(ctx, `SELECT valeur_chiffree IS NULL AND dek_chiffree IS NULL FROM secret WHERE id_secret = $1`, sec.ID).Scan(&wiped))
	assert.True(t, wiped, "la révocation efface la matière chiffrée")

	assert.Equal(t, []string{"CREATION", "REVELATION", "REFUS", "REFUS", "REVOCATION", "REFUS"}, journalActions(t, e, sec.ID))
	journal, err := e.svc.SecretJournal(ctx, d.sami, sec.ID)
	require.NoError(t, err)
	assert.Equal(t, "198.51.100.4", journal[1].IP)
	_, err = e.db.Exec(ctx, `DELETE FROM secret_journal WHERE id_secret = $1`, sec.ID)
	assert.Error(t, err, "journal en ajout seul")
}

func TestVault_Rules(t *testing.T) {
	e := newEnv(t, withVault)
	ctx := context.Background()
	d := newDeliverable(t, e)

	_, err := e.svc.ShareSecret(ctx, d.lea, d.x.ID, secretInput(e), "")
	requireKind(t, err, KindForbidden, "acces-refuse") // seul le demandeur dépose (RG19)
	_, err = e.svc.ShareSecret(ctx, d.tiers, d.x.ID, secretInput(e), "")
	requireKind(t, err, KindNotFound, "")

	for name, mutate := range map[string]func(*SecretInput){
		"expiration passée":  func(in *SecretInput) { in.ExpiresAt = e.clock.Now().Add(-time.Minute) },
		"expiration > 30 j":  func(in *SecretInput) { in.ExpiresAt = e.clock.Now().Add(31 * 24 * time.Hour) },
		"valeur vide":        func(in *SecretInput) { in.Value = "" },
		"valeur trop longue": func(in *SecretInput) { in.Value = strings.Repeat("x", 8193) },
		"type inconnu":       func(in *SecretInput) { in.Kind = "CERTIFICAT" },
		"libellé vide":       func(in *SecretInput) { in.Label = " " },
		"portée trop longue": func(in *SecretInput) { in.Scope = strings.Repeat("x", 301) },
	} {
		in := secretInput(e)
		mutate(&in)
		_, err := e.svc.ShareSecret(ctx, d.sami, d.x.ID, in, "")
		requireKind(t, err, KindInvalid, "donnees-invalides")
		_ = name
	}

	// Expiration : le worker révoque, la révélation échoue.
	sec, err := e.svc.ShareSecret(ctx, d.sami, d.x.ID, secretInput(e), "")
	require.NoError(t, err)
	e.clock.Advance(7*24*time.Hour + time.Second)
	_, err = e.svc.RevealSecret(ctx, d.lea, sec.ID, "")
	requireKind(t, err, KindGone, "secret-indisponible")
	n, err := e.svc.ExpireSecrets(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Contains(t, journalActions(t, e, sec.ID), "EXPIRATION")
	list, err := e.svc.ListSecrets(ctx, d.sami, d.x.ID)
	require.NoError(t, err)
	assert.Equal(t, "EXPIRE", list[0].Status)
}

func TestVault_RevokedWhenExchangeCloses(t *testing.T) {
	e := newEnv(t, withVault)
	ctx := context.Background()
	d := newDeliverable(t, e)
	sec, err := e.svc.ShareSecret(ctx, d.sami, d.x.ID, secretInput(e), "")
	require.NoError(t, err)

	_, err = e.svc.CancelExchange(ctx, d.sami, d.x.ID)
	require.NoError(t, err)
	_, err = e.svc.CancelExchange(ctx, d.lea, d.x.ID)
	require.NoError(t, err)

	list, err := e.svc.ListSecrets(ctx, d.sami, d.x.ID)
	require.NoError(t, err)
	assert.Equal(t, "REVOQUE", list[0].Status, "RG19 : clôture ⇒ révocation")
	_, err = e.svc.RevealSecret(ctx, d.lea, sec.ID, "")
	requireKind(t, err, KindGone, "secret-indisponible")
	_, err = e.svc.ShareSecret(ctx, d.sami, d.x.ID, secretInput(e), "")
	requireKind(t, err, KindConflict, "echange-inactif")
}

func TestVault_NotConfigured(t *testing.T) {
	e := newEnv(t)
	d := newDeliverable(t, e)
	_, err := e.svc.ShareSecret(context.Background(), d.sami, d.x.ID, secretInput(e), "")
	requireKind(t, err, KindUnavailable, "coffre-non-configure")
}
