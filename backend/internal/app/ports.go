package app

import (
	"context"
	"errors"
	"time"
)

// GitHubIdentity est le compte GitHub d'une personne qui se connecte.
type GitHubIdentity struct {
	ID        int64
	Login     string
	Email     string // adresse principale vérifiée, vide sinon
	CreatedAt time.Time
}

// GitHubOAuth réalise la connexion OAuth avec GitHub.
type GitHubOAuth interface {
	AuthorizeURL(state string) string
	Exchange(ctx context.Context, code string) (GitHubIdentity, error)
}

// Repository décrit un dépôt GitHub.
type Repository struct {
	ID       int64
	FullName string
}

// PullRequest regroupe les faits d'une PR utiles à la vérification d'une preuve.
type PullRequest struct {
	ID         int64
	Number     int
	BaseRepoID int64
	AuthorID   int64
	CreatedAt  time.Time
	Merged     bool
	HeadSHA    string
	MergeSHA   string
}

// CheckRun est un check CI sur un commit.
type CheckRun struct {
	Name       string
	Status     string // queued, in_progress, completed
	Conclusion string // success, failure, neutral, skipped, cancelled, timed_out, action_required
}

// ErrGitHubNotFound signale une ressource GitHub absente ou inaccessible à l'App.
var ErrGitHubNotFound = errors.New("github : ressource introuvable ou App non installée")

// GitHubAPI lit les dépôts, PR et checks via la GitHub App (lecture seule).
type GitHubAPI interface {
	Repository(ctx context.Context, owner, name string) (Repository, error)
	PullRequest(ctx context.Context, owner, name string, number int) (PullRequest, error)
	CheckRuns(ctx context.Context, owner, name, sha string) ([]CheckRun, error)
}

// Sealed est une valeur chiffrée par enveloppe.
type Sealed struct {
	Ciphertext []byte
	Nonce      []byte
	WrappedDEK []byte
	DEKNonce   []byte
	KEKVersion int
}

// Sealer chiffre les secrets du coffre (AES-256-GCM, clé de données par secret).
type Sealer interface {
	Seal(plaintext, aad []byte) (Sealed, error)
	Open(s Sealed, aad []byte) ([]byte, error)
}

// Email est un message texte envoyé à un membre (F18).
type Email struct {
	To      string
	Subject string
	Text    string
}

// Mailer envoie les emails de confirmation et de réinitialisation.
type Mailer interface {
	Send(ctx context.Context, m Email) error
}
