// Package proof décide si une pull request prouve la livraison d'un jalon (RG13, D6).
package proof

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Status est le résultat d'une vérification.
type Status string

const (
	Pending  Status = "EN_ATTENTE"
	Valid    Status = "VALIDE"
	Rejected Status = "REJETEE"
)

// PRRef identifie une PR à partir de son URL.
type PRRef struct {
	Owner  string
	Repo   string
	Number int
}

// URL renvoie l'URL canonique de la PR.
func (p PRRef) URL() string {
	return fmt.Sprintf("https://github.com/%s/%s/pull/%d", p.Owner, p.Repo, p.Number)
}

// ErrPRURL signale une URL de PR refusée.
var ErrPRURL = errors.New("URL de PR invalide : https://github.com/<propriétaire>/<dépôt>/pull/<numéro> attendue")

var prPath = regexp.MustCompile(`^/([A-Za-z0-9](?:[A-Za-z0-9-]{0,38}))/([A-Za-z0-9._-]{1,100})/pull/([1-9][0-9]{0,8})$`)

// ParsePRURL n'accepte qu'une URL https://github.com/<owner>/<repo>/pull/<n>.
func ParsePRURL(s string) (PRRef, error) {
	u, err := url.Parse(s)
	if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || u.RawQuery != "" ||
		u.Fragment != "" || strings.ContainsAny(s, "?#") {
		return PRRef{}, ErrPRURL
	}
	m := prPath.FindStringSubmatch(u.Path)
	if m == nil || m[2] == "." || m[2] == ".." {
		return PRRef{}, ErrPRURL
	}
	n, err := strconv.Atoi(m[3])
	if err != nil {
		return PRRef{}, ErrPRURL
	}
	return PRRef{Owner: m[1], Repo: m[2], Number: n}, nil
}

// PR regroupe les faits lus sur GitHub.
type PR struct {
	ID         int64
	BaseRepoID int64
	AuthorID   int64
	CreatedAt  time.Time
	Merged     bool
	HeadSHA    string
}

// Check est un check CI du commit livré.
type Check struct {
	Name       string
	Status     string
	Conclusion string
}

// Facts réunit tout ce qu'il faut pour juger une preuve.
type Facts struct {
	RepoID           int64     // dépôt de l'échange (identifiant GitHub)
	ProviderGitHubID int64     // compte GitHub du prestataire
	AcceptedAt       time.Time // acceptation de l'échange
	PR               PR
	Checks           []Check // checks du commit de tête de la PR
	AlreadyUsed      bool    // PR déjà retenue comme preuve d'un autre jalon
}

// Verdict est le résultat d'une évaluation.
type Verdict struct {
	Status Status
	Detail string
}

var okConclusions = map[string]bool{"success": true, "neutral": true, "skipped": true}

// Evaluate applique D6. Les contrôles d'identité (dépôt, auteur, date, unicité)
// passent avant l'état de la PR : une PR illégitime est rejetée tout de suite.
func Evaluate(f Facts) Verdict {
	switch {
	case f.PR.BaseRepoID != f.RepoID:
		return Verdict{Rejected, "La PR ne cible pas le dépôt de l'échange."}
	case f.PR.AuthorID != f.ProviderGitHubID:
		return Verdict{Rejected, "La PR n'a pas pour auteur le prestataire de l'échange."}
	case f.PR.CreatedAt.Before(f.AcceptedAt):
		return Verdict{Rejected, "La PR est antérieure à l'acceptation de l'échange."}
	case f.AlreadyUsed:
		return Verdict{Rejected, "Cette PR a déjà servi de preuve pour un autre jalon."}
	case !f.PR.Merged:
		return Verdict{Pending, "La PR n'est pas encore mergée."}
	case len(f.Checks) == 0:
		return Verdict{Rejected, "La PR a été mergée sans aucun check CI sur le commit livré."}
	}
	success := 0
	for _, c := range f.Checks {
		if c.Status != "completed" {
			return Verdict{Pending, fmt.Sprintf("CI en cours : %s n'est pas terminé.", c.Name)}
		}
	}
	for _, c := range f.Checks {
		if !okConclusions[c.Conclusion] {
			return Verdict{Rejected, fmt.Sprintf("Le check %s est en échec (%s).", c.Name, c.Conclusion)}
		}
		if c.Conclusion == "success" {
			success++
		}
	}
	if success == 0 {
		return Verdict{Rejected, "Aucun check CI en succès sur le commit livré."}
	}
	return Verdict{Valid, fmt.Sprintf("PR mergée, CI réussie (%d/%d checks en succès).", success, len(f.Checks))}
}
