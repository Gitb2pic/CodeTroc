package password

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashAndVerify(t *testing.T) {
	h, err := Hash("correct horse battery")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(h, "$argon2id$v=19$m=19456,t=2,p=1$"))

	ok, err := Verify("correct horse battery", h)
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = Verify("correct horse batterY", h)
	require.NoError(t, err)
	assert.False(t, ok)

	h2, err := Hash("correct horse battery")
	require.NoError(t, err)
	assert.NotEqual(t, h, h2, "sel aléatoire")
}

func TestVerifyRejectsMalformed(t *testing.T) {
	for _, bad := range []string{"", "plain", "$argon2i$v=19$m=1,t=1,p=1$c2Fs$aGFzaA", "$argon2id$v=19$m=99999999,t=1,p=1$c2Fs$aGFzaA", "$argon2id$v=19$m=1,t=1,p=1$!!$aGFzaA"} {
		_, err := Verify("x", bad)
		assert.Error(t, err, bad)
	}
}

func TestCheck(t *testing.T) {
	assert.ErrorIs(t, Check("court"), ErrWeak)
	assert.ErrorIs(t, Check(strings.Repeat("a", 129)), ErrWeak)
	assert.NoError(t, Check("éééééééééé"), "10 caractères, pas 10 octets")
}
