package app

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduledJobs(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	d := newDeliverable(t, e)
	d.submit(t, e, 0, 42)
	e.clock.Advance(73 * time.Hour)

	r, err := e.svc.RunScheduledJobs(ctx)
	require.NoError(t, err)
	assert.True(t, r.Ran)
	assert.Equal(t, 1, r.TacitReleases)
	assert.Zero(t, r.LedgerMismatches)
	assert.Zero(t, r.LedgerTotal)
}

func TestScheduledJobs_SingleInstance(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	conn, err := e.db.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, schedulerLockKey)
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := e.svc.RunScheduledJobs(ctx)
			assert.NoError(t, err)
			assert.False(t, r.Ran, "une autre instance tient le verrou")
		}()
	}
	wg.Wait()
	_, err = conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, schedulerLockKey)
	require.NoError(t, err)
	r, err := e.svc.RunScheduledJobs(ctx)
	require.NoError(t, err)
	assert.True(t, r.Ran)
}
