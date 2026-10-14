package app

import (
	"context"
	"time"
)

// schedulerLockKey identifie le verrou consultatif du planificateur.
const schedulerLockKey = 7_317_001

// JobReport résume un passage du planificateur.
type JobReport struct {
	Ran              bool // false si une autre instance tenait le verrou
	TacitReleases    int
	ProofsRechecked  int
	SecretsExpired   int
	LedgerMismatches int
	LedgerTotal      int64
}

// RunScheduledJobs exécute les traitements différés (validation tacite,
// réconciliation des preuves, expiration des secrets, ménage, contrôle du
// grand livre) si aucune autre instance ne le fait déjà (pg_try_advisory_lock).
func (s *Service) RunScheduledJobs(ctx context.Context) (JobReport, error) {
	conn, err := s.db.Acquire(ctx)
	if err != nil {
		return JobReport{}, err
	}
	defer conn.Release()
	var locked bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, schedulerLockKey).Scan(&locked); err != nil {
		return JobReport{}, err
	}
	if !locked {
		return JobReport{}, nil
	}
	defer func() {
		_, _ = conn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, schedulerLockKey)
	}()

	r := JobReport{Ran: true}
	if r.TacitReleases, err = s.RunTacitValidation(ctx); err != nil {
		return r, err
	}
	if r.ProofsRechecked, err = s.ReconcilePendingProofs(ctx, 5*time.Minute); err != nil {
		return r, err
	}
	if r.SecretsExpired, err = s.ExpireSecrets(ctx); err != nil {
		return r, err
	}
	if err := s.purge(ctx); err != nil {
		return r, err
	}
	report, err := s.LedgerIntegrity(ctx)
	if err != nil {
		return r, err
	}
	r.LedgerMismatches, r.LedgerTotal = len(report.Mismatches), report.TotalBalance
	return r, nil
}

// purge supprime les données techniques périmées.
func (s *Service) purge(ctx context.Context) error {
	now := s.now()
	for _, q := range []struct {
		sql string
		at  time.Time
	}{
		{`DELETE FROM oauth_state WHERE expire_le < $1`, now},
		{`DELETE FROM cle_idempotence WHERE cree_le < $1`, now.Add(-24 * time.Hour)},
		{`DELETE FROM webhook_livraison WHERE recu_le < $1`, now.Add(-30 * 24 * time.Hour)},
		{`DELETE FROM jeton_rafraichissement WHERE expire_le < $1`, now.Add(-7 * 24 * time.Hour)},
	} {
		if _, err := s.db.Exec(ctx, q.sql, q.at); err != nil {
			return err
		}
	}
	return nil
}
