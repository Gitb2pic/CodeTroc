// Package password hache et vérifie les mots de passe avec Argon2id (format
// PHC : $argon2id$v=19$m=…,t=…,p=…$sel$hash), paramètres OWASP 2024.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

// Paramètres Argon2id (OWASP : m = 19 Mio, t = 2, p = 1).
const (
	memory  = 19 * 1024
	time    = 2
	threads = 1
	keyLen  = 32
	saltLen = 16

	MinLength = 10
	MaxLength = 128
)

// ErrWeak signale un mot de passe hors des règles.
var ErrWeak = fmt.Errorf("le mot de passe doit faire entre %d et %d caractères", MinLength, MaxLength)

// Check vérifie la longueur (en caractères) d'un mot de passe.
func Check(pw string) error {
	n := utf8.RuneCountInString(pw)
	if n < MinLength || n > MaxLength {
		return ErrWeak
	}
	return nil
}

var b64 = base64.RawStdEncoding

// Hash renvoie l'empreinte PHC d'un mot de passe, avec un sel aléatoire.
func Hash(pw string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(pw), salt, time, memory, threads, keyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, memory, time, threads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

var errFormat = errors.New("empreinte de mot de passe illisible")

// Verify compare un mot de passe à une empreinte PHC, en temps constant.
func Verify(pw, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errFormat
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, errFormat
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil || m == 0 || t == 0 || p == 0 || m > 1<<20 || t > 10 {
		return false, errFormat
	}
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return false, errFormat
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil || len(want) == 0 || len(want) > 64 {
		return false, errFormat
	}
	got := argon2.IDKey([]byte(pw), salt, t, m, p, uint32(len(want))) // #nosec G115 -- longueur bornée à 64
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// dummy est une empreinte réelle, vérifiée quand le compte n'existe pas : le
// temps de réponse ne révèle pas si un email est inscrit.
var dummy, _ = Hash("codetroc-mot-de-passe-factice")

// VerifyDummy consomme le même temps qu'une vérification réelle.
func VerifyDummy(pw string) { _, _ = Verify(pw, dummy) }
