package token

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var key = []byte("0123456789abcdef0123456789abcdef")

func newIssuer(t *testing.T, now time.Time) *Issuer {
	t.Helper()
	iss, err := NewIssuer(key, 15*time.Minute, func() time.Time { return now })
	require.NoError(t, err)
	return iss
}

func TestIssueAndParse(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	iss := newIssuer(t, now)
	id := uuid.New()

	tok, exp, err := iss.Issue(id)
	require.NoError(t, err)
	assert.Equal(t, now.Add(15*time.Minute), exp)

	got, err := iss.Parse(tok)
	require.NoError(t, err)
	assert.Equal(t, id, got)
}

func TestRejectsShortKey(t *testing.T) {
	_, err := NewIssuer([]byte("trop-court"), time.Minute, time.Now)
	require.Error(t, err)
}

func TestParseRejects(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	iss := newIssuer(t, now)
	id := uuid.New()
	valid, _, err := iss.Issue(id)
	require.NoError(t, err)

	sign := func(method jwt.SigningMethod, k any, claims jwt.RegisteredClaims) string {
		s, err := jwt.NewWithClaims(method, claims).SignedString(k)
		require.NoError(t, err)
		return s
	}
	good := jwt.RegisteredClaims{
		Subject:   id.String(),
		Issuer:    issuerName,
		Audience:  jwt.ClaimStrings{audience},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
	}
	with := func(f func(*jwt.RegisteredClaims)) jwt.RegisteredClaims {
		c := good
		f(&c)
		return c
	}

	cases := map[string]string{
		"vide":               "",
		"illisible":          "pas.un.jwt",
		"alg none":           sign(jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, good),
		"autre clé":          sign(jwt.SigningMethodHS256, []byte("ffffffffffffffffffffffffffffffff"), good),
		"HS512 non autorisé": sign(jwt.SigningMethodHS512, key, good),
		"expiré":             sign(jwt.SigningMethodHS256, key, with(func(c *jwt.RegisteredClaims) { c.ExpiresAt = jwt.NewNumericDate(now.Add(-time.Second)) })),
		"sans expiration":    sign(jwt.SigningMethodHS256, key, with(func(c *jwt.RegisteredClaims) { c.ExpiresAt = nil })),
		"mauvais émetteur":   sign(jwt.SigningMethodHS256, key, with(func(c *jwt.RegisteredClaims) { c.Issuer = "autre" })),
		"mauvaise audience":  sign(jwt.SigningMethodHS256, key, with(func(c *jwt.RegisteredClaims) { c.Audience = jwt.ClaimStrings{"autre"} })),
		"sujet non uuid":     sign(jwt.SigningMethodHS256, key, with(func(c *jwt.RegisteredClaims) { c.Subject = "admin" })),
		"signature altérée":  valid[:len(valid)-2] + flip(valid[len(valid)-2:]),
		"charge modifiée":    tamperPayload(valid),
	}
	for name, tok := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := iss.Parse(tok)
			require.ErrorIs(t, err, ErrInvalid)
		})
	}
}

func flip(s string) string {
	if s[0] == 'A' {
		return "B" + s[1:]
	}
	return "A" + s[1:]
}

func tamperPayload(tok string) string {
	parts := strings.Split(tok, ".")
	parts[1] = flip(parts[1])
	return strings.Join(parts, ".")
}

func TestNewOpaque(t *testing.T) {
	a, ha, err := NewOpaque()
	require.NoError(t, err)
	b, _, err := NewOpaque()
	require.NoError(t, err)

	assert.NotEqual(t, a, b)
	assert.GreaterOrEqual(t, len(a), 43, "32 octets en base64url")
	assert.Equal(t, ha, HashOpaque(a))
	assert.Len(t, ha, 32)
}
