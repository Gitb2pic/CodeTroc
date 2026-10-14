package httpapi

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLimiter_TokenBucket(t *testing.T) {
	now := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	l := newLimiter(RateRule{Burst: 3, Per: time.Minute}, func() time.Time { return now })

	for range 3 {
		ok, _ := l.allow("a")
		assert.True(t, ok)
	}
	ok, retry := l.allow("a")
	assert.False(t, ok, "rafale épuisée")
	assert.Equal(t, 20*time.Second, retry, "un jeton revient toutes les 20 s")

	ok, _ = l.allow("b")
	assert.True(t, ok, "les clés sont indépendantes")

	now = now.Add(20 * time.Second)
	ok, _ = l.allow("a")
	assert.True(t, ok, "un jeton est revenu")
	ok, _ = l.allow("a")
	assert.False(t, ok)

	now = now.Add(time.Hour)
	for range 3 {
		ok, _ = l.allow("a")
		assert.True(t, ok, "le seau se remplit jusqu'à la rafale, pas au-delà")
	}
	ok, _ = l.allow("a")
	assert.False(t, ok)
}

func TestLimiter_ForgetsIdleKeys(t *testing.T) {
	now := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	l := newLimiter(RateRule{Burst: 2, Per: time.Minute}, func() time.Time { return now })
	for i := range maxTrackedKeys + 10 {
		l.allow(string(rune(i)))
	}
	now = now.Add(time.Hour)
	l.allow("nouveau")
	assert.LessOrEqual(t, len(l.buckets), maxTrackedKeys, "les seaux pleins sont oubliés : mémoire bornée")
}
