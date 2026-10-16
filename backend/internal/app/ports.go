package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gitb2pic/codetroc/backend/internal/domain/scrum"
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

// RepoRef désigne un dépôt par son identifiant GitHub (jamais par son seul
// nom, R-S2) ; owner et name servent à trouver l'installation de l'App.
type RepoRef struct {
	ID          int64
	Owner, Name string
}

// IssueInput décrit une issue à créer.
type IssueInput struct {
	Title     string
	Body      string
	Labels    []string
	Milestone int    // 0 : aucun
	Assignee  string // login GitHub, vide : personne
}

// IssueUpdate modifie une issue ; un champ vide reste inchangé.
type IssueUpdate struct {
	Title  string
	Body   string
	State  string // "open", "closed" ou ""
	Labels []string
}

// ErrGitHubPermission signale une permission manquante (issues:write non accordée).
var ErrGitHubPermission = errors.New("github : permission refusée à l'App")

// RateLimitError signale un quota GitHub épuisé ; RetryAfter vient de l'en-tête Retry-After.
type RateLimitError struct{ RetryAfter time.Duration }

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("github : quota épuisé, réessayer dans %s", e.RetryAfter)
}

// GitHubIssues écrit labels, milestones et issues dans le dépôt d'un échange
// (permission issues:write, D18). Jamais de contents:write.
type GitHubIssues interface {
	EnsureLabels(ctx context.Context, repo RepoRef, labels []scrum.Label) error
	EnsureMilestone(ctx context.Context, repo RepoRef, title string, due time.Time) (int, error)
	CreateIssue(ctx context.Context, repo RepoRef, in IssueInput) (int, error)
	UpdateIssue(ctx context.Context, repo RepoRef, number int, in IssueUpdate) error
	Comment(ctx context.Context, repo RepoRef, number int, body string) error
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
