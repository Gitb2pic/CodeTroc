package vault

import (
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func key(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, 32)
	_, err := rand.Read(k)
	require.NoError(t, err)
	return k
}

func TestEnvelope_RoundTrip(t *testing.T) {
	e, err := New(map[int][]byte{1: key(t)}, 1)
	require.NoError(t, err)
	aad := []byte("echange|secret")
	s, err := e.Seal([]byte("ghp_tokenTresSecret"), aad)
	require.NoError(t, err)
	assert.Equal(t, 1, s.KEKVersion)
	assert.NotContains(t, string(s.Ciphertext), "ghp_tokenTresSecret")
	assert.Len(t, s.Nonce, 12)
	assert.Len(t, s.DEKNonce, 12)

	plain, err := e.Open(s, aad)
	require.NoError(t, err)
	assert.Equal(t, "ghp_tokenTresSecret", string(plain))
}

func TestEnvelope_FreshKeyAndNoncePerSecret(t *testing.T) {
	e, _ := New(map[int][]byte{1: key(t)}, 1)
	a, _ := e.Seal([]byte("même valeur"), nil)
	b, _ := e.Seal([]byte("même valeur"), nil)
	assert.False(t, bytes.Equal(a.Ciphertext, b.Ciphertext))
	assert.False(t, bytes.Equal(a.WrappedDEK, b.WrappedDEK))
	assert.False(t, bytes.Equal(a.Nonce, b.Nonce))
}

func TestEnvelope_RejectsTamperingAndWrongContext(t *testing.T) {
	e, _ := New(map[int][]byte{1: key(t)}, 1)
	aad := []byte("echange-A|secret-1")
	s, _ := e.Seal([]byte("valeur"), aad)

	_, err := e.Open(s, []byte("echange-B|secret-1"))
	assert.Error(t, err, "un chiffré déplacé vers un autre échange ne s'ouvre pas")

	for name, mutate := range map[string]func(*Sealed){
		"chiffré":     func(x *Sealed) { x.Ciphertext[0] ^= 1 },
		"clé enrobée": func(x *Sealed) { x.WrappedDEK[0] ^= 1 },
		"nonce":       func(x *Sealed) { x.Nonce[0] ^= 1 },
		"version":     func(x *Sealed) { x.KEKVersion = 9 },
	} {
		c := Sealed{Ciphertext: bytes.Clone(s.Ciphertext), Nonce: bytes.Clone(s.Nonce), WrappedDEK: bytes.Clone(s.WrappedDEK), DEKNonce: bytes.Clone(s.DEKNonce), KEKVersion: s.KEKVersion}
		mutate(&c)
		_, err := e.Open(c, aad)
		assert.Error(t, err, name)
	}
}

func TestEnvelope_KeyRotation(t *testing.T) {
	k1, k2 := key(t), key(t)
	old, _ := New(map[int][]byte{1: k1}, 1)
	s, _ := old.Seal([]byte("ancien"), nil)

	rotated, err := New(map[int][]byte{1: k1, 2: k2}, 2)
	require.NoError(t, err)
	plain, err := rotated.Open(s, nil)
	require.NoError(t, err, "les secrets chiffrés avec l'ancienne KEK restent lisibles")
	assert.Equal(t, "ancien", string(plain))
	n, _ := rotated.Seal([]byte("nouveau"), nil)
	assert.Equal(t, 2, n.KEKVersion)
}

func TestNew_Validation(t *testing.T) {
	_, err := New(map[int][]byte{1: make([]byte, 16)}, 1)
	assert.Error(t, err, "AES-256 uniquement")
	_, err = New(map[int][]byte{1: key(t)}, 2)
	assert.Error(t, err, "version courante absente")
}
