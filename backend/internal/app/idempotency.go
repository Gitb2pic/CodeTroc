package app

import (
	"bytes"
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// StoredResponse est une réponse enregistrée pour une clé Idempotency-Key.
type StoredResponse struct {
	Status int
	Body   []byte
}

var (
	errKeyReused  = unprocessable("cle-idempotence-reutilisee", "Cette clé Idempotency-Key a déjà servi pour une autre requête.")
	errKeyPending = conflict("requete-en-cours", "Une requête avec cette clé Idempotency-Key est en cours de traitement.")
)

// BeginIdempotent réserve une clé pour l'utilisateur. Renvoie la réponse
// enregistrée si la même requête a déjà abouti (rejeu).
func (s *Service) BeginIdempotent(ctx context.Context, userID uuid.UUID, key, route string, fingerprint []byte) (*StoredResponse, error) {
	tag, err := s.db.Exec(ctx, `
		INSERT INTO cle_idempotence (id_utilisateur, cle, route, empreinte, cree_le) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT DO NOTHING`, userID, key, route, fingerprint, s.now())
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 1 {
		return nil, nil
	}
	var storedRoute string
	var storedFP []byte
	var status *int
	var body []byte
	err = s.db.QueryRow(ctx, `SELECT route, empreinte, statut_http, corps FROM cle_idempotence WHERE id_utilisateur = $1 AND cle = $2`, userID, key).
		Scan(&storedRoute, &storedFP, &status, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errKeyPending // supprimée entre-temps après une erreur serveur : réessayer
	}
	if err != nil {
		return nil, err
	}
	if storedRoute != route || !bytes.Equal(storedFP, fingerprint) {
		return nil, errKeyReused
	}
	if status == nil {
		return nil, errKeyPending
	}
	return &StoredResponse{Status: *status, Body: body}, nil
}

// FinishIdempotent enregistre la réponse associée à la clé.
func (s *Service) FinishIdempotent(ctx context.Context, userID uuid.UUID, key string, status int, body []byte) error {
	_, err := s.db.Exec(ctx, `UPDATE cle_idempotence SET statut_http = $3, corps = $4 WHERE id_utilisateur = $1 AND cle = $2`, userID, key, status, body)
	return err
}

// ReleaseIdempotent libère la clé après une erreur serveur, pour permettre un nouvel essai.
func (s *Service) ReleaseIdempotent(ctx context.Context, userID uuid.UUID, key string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM cle_idempotence WHERE id_utilisateur = $1 AND cle = $2`, userID, key)
	return err
}
