package mail

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gitb2pic/codetroc/backend/internal/app"
)

func TestBuild(t *testing.T) {
	msg, err := Build("CodeTroc <no-reply@codetroc.test>", app.Email{To: "lea@example.com", Subject: "Confirme ton adresse", Text: "Bonjour,\nclique ici : https://codetroc.test/confirmer-email?jeton=abc"}, time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	s := string(msg)
	assert.Contains(t, s, "From: \"CodeTroc\" <no-reply@codetroc.test>\r\n")
	assert.Contains(t, s, "To: <lea@example.com>\r\n")
	assert.Contains(t, s, "Subject: Confirme ton adresse\r\n")
	assert.Contains(t, s, "@codetroc.test>\r\n")
	assert.Contains(t, s, "Content-Type: text/plain; charset=utf-8")
	assert.Contains(t, s, "jeton=3Dabc", "quoted-printable")
}

func TestBuildRejectsHeaderInjection(t *testing.T) {
	_, err := Build("a@b.c", app.Email{To: "x@y.z\r\nBcc: victime@example.com", Subject: "s"}, time.Now())
	assert.ErrorIs(t, err, errHeader)
	_, err = Build("a@b.c", app.Email{To: "x@y.z", Subject: "s\nBcc: v@e.com"}, time.Now())
	assert.ErrorIs(t, err, errHeader)
	_, err = Build("a@b.c", app.Email{To: "pas une adresse", Subject: "s"}, time.Now())
	assert.Error(t, err)
}

func TestBuildEncodesAccents(t *testing.T) {
	msg, err := Build("a@b.c", app.Email{To: "x@y.z", Subject: "Réinitialise ton mot de passe", Text: "é"}, time.Now())
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(msg), "Subject: =?utf-8?q?R=C3=A9initialise"))
}
