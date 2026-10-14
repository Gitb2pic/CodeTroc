// Package token émet et vérifie les jetons d'accès (JWT) et fabrique les
// jetons opaques (rafraîchissement, state OAuth).
package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	issuerName = "codetroc"
	audience   = "codetroc-api"
	minKeyLen  = 32
)

// ErrInvalid regroupe toutes les causes de rejet : le client n'a pas à savoir laquelle.
var ErrInvalid = errors.New("jeton invalide")

// Issuer émet et vérifie les jetons d'accès HS256.
type Issuer struct {
	key []byte
	ttl time.Duration
	now func() time.Time
}

// NewIssuer exige une clé d'au moins 32 octets.
func NewIssuer(key []byte, ttl time.Duration, now func() time.Time) (*Issuer, error) {
	if len(key) < minKeyLen {
		return nil, fmt.Errorf("token : clé de signature trop courte (%d octets, %d minimum)", len(key), minKeyLen)
	}
	return &Issuer{key: key, ttl: ttl, now: now}, nil
}

// Issue renvoie un jeton d'accès pour l'utilisateur et son expiration.
func (i *Issuer) Issue(userID uuid.UUID) (string, time.Time, error) {
	now := i.now()
	exp := now.Add(i.ttl)
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		Issuer:    issuerName,
		Audience:  jwt.ClaimStrings{audience},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(exp),
		ID:        uuid.NewString(),
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(i.key)
	return s, exp, err
}

// Parse vérifie la signature (HS256 uniquement), l'expiration, l'émetteur et
// l'audience, puis renvoie l'identifiant de l'utilisateur.
func (i *Issuer) Parse(tok string) (uuid.UUID, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(tok, &claims,
		func(*jwt.Token) (any, error) { return i.key, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuerName),
		jwt.WithAudience(audience),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(i.now),
	)
	if err != nil {
		return uuid.Nil, ErrInvalid
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, ErrInvalid
	}
	return id, nil
}

// NewOpaque renvoie un jeton aléatoire de 32 octets (base64url) et son empreinte
// SHA-256, seule valeur stockée en base.
func NewOpaque() (string, []byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	s := base64.RawURLEncoding.EncodeToString(b)
	return s, HashOpaque(s), nil
}

// HashOpaque calcule l'empreinte stockée d'un jeton opaque.
func HashOpaque(s string) []byte {
	h := sha256.Sum256([]byte(s))
	return h[:]
}
