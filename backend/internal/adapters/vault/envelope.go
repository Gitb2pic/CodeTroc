// Package vault chiffre les secrets du coffre par enveloppe : une clé de données
// (DEK) AES-256-GCM aléatoire par secret, elle-même chiffrée par la clé maître
// (KEK). Le contexte (AAD) lie le chiffré à son échange et à son secret.
package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/gitb2pic/codetroc/backend/internal/app"
)

// Sealed est l'alias du type du port applicatif.
type Sealed = app.Sealed

// Envelope implémente app.Sealer.
type Envelope struct {
	keks    map[int]cipher.AEAD
	current int
}

var errOpen = errors.New("vault: déchiffrement impossible")

// New construit le chiffreur ; keks associe une version à une KEK de 32 octets,
// current est la version utilisée pour chiffrer (les autres servent à relire).
func New(keks map[int][]byte, current int) (*Envelope, error) {
	e := &Envelope{keks: map[int]cipher.AEAD{}, current: current}
	for v, k := range keks {
		aead, err := gcm(k)
		if err != nil {
			return nil, fmt.Errorf("vault: KEK v%d : %w", v, err)
		}
		e.keks[v] = aead
	}
	if _, ok := e.keks[current]; !ok {
		return nil, fmt.Errorf("vault: KEK v%d absente", current)
	}
	return e, nil
}

func gcm(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, errors.New("clé de 32 octets attendue")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func nonce(a cipher.AEAD) ([]byte, error) {
	n := make([]byte, a.NonceSize())
	_, err := rand.Read(n)
	return n, err
}

// Seal chiffre plaintext avec une DEK neuve.
func (e *Envelope) Seal(plaintext, aad []byte) (Sealed, error) {
	dek := make([]byte, 32)
	if _, err := rand.Read(dek); err != nil {
		return Sealed{}, err
	}
	defer clear(dek)
	data, err := gcm(dek)
	if err != nil {
		return Sealed{}, err
	}
	kek := e.keks[e.current]
	n, err := nonce(data)
	if err != nil {
		return Sealed{}, err
	}
	dn, err := nonce(kek)
	if err != nil {
		return Sealed{}, err
	}
	return Sealed{
		Ciphertext: data.Seal(nil, n, plaintext, aad),
		Nonce:      n,
		WrappedDEK: kek.Seal(nil, dn, dek, aad),
		DEKNonce:   dn,
		KEKVersion: e.current,
	}, nil
}

// Open déchiffre ; toute altération (chiffré, clé, nonce, contexte) échoue.
func (e *Envelope) Open(s Sealed, aad []byte) ([]byte, error) {
	kek, ok := e.keks[s.KEKVersion]
	if !ok || len(s.DEKNonce) != kek.NonceSize() {
		return nil, errOpen
	}
	dek, err := kek.Open(nil, s.DEKNonce, s.WrappedDEK, aad)
	if err != nil {
		return nil, errOpen
	}
	defer clear(dek)
	data, err := gcm(dek)
	if err != nil || len(s.Nonce) != data.NonceSize() {
		return nil, errOpen
	}
	plain, err := data.Open(nil, s.Nonce, s.Ciphertext, aad)
	if err != nil {
		return nil, errOpen
	}
	return plain, nil
}
