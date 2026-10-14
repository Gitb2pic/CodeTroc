// Package exchange porte les règles d'un échange : machines à états de
// l'échange et des jalons, dépôt GitHub cible et échéances.
package exchange

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Status est l'état d'un échange.
type Status string

const (
	Proposed   Status = "PROPOSE"
	Refused    Status = "REFUSE"
	InProgress Status = "EN_COURS"
	InDispute  Status = "EN_LITIGE"
	Done       Status = "TERMINE"
	Cancelled  Status = "ANNULE"
)

// MilestoneStatus est l'état d'un jalon.
type MilestoneStatus string

const (
	Pending   MilestoneStatus = "EN_ATTENTE"
	Active    MilestoneStatus = "EN_COURS"
	Delivered MilestoneStatus = "LIVRE"
	Disputed  MilestoneStatus = "CONTESTE"
	Released  MilestoneStatus = "LIBERE"
	Refunded  MilestoneStatus = "REMBOURSE"
	Split     MilestoneStatus = "PARTAGE"
)

// Final indique qu'un jalon est soldé : ses crédits ont quitté le séquestre.
func (m MilestoneStatus) Final() bool { return m == Released || m == Refunded || m == Split }

// ErrTransition signale une transition absente des diagrammes d'états.
var ErrTransition = errors.New("transition interdite")

var exchangeMoves = map[Status][]Status{
	Proposed:   {Refused, Cancelled, InProgress},
	InProgress: {InDispute, Done, Cancelled},
	InDispute:  {InProgress, Done},
}

// Une preuve rejetée laisse le jalon EN_COURS : ce n'est pas une transition.
var milestoneMoves = map[MilestoneStatus][]MilestoneStatus{
	Pending:   {Active, Refunded},
	Active:    {Delivered, Refunded},
	Delivered: {Released, Disputed},
	Disputed:  {Released, Refunded, Split},
}

// CheckExchange vérifie une transition d'échange.
func CheckExchange(from, to Status) error {
	for _, s := range exchangeMoves[from] {
		if s == to {
			return nil
		}
	}
	return fmt.Errorf("%w : échange %s → %s", ErrTransition, from, to)
}

// CheckMilestone vérifie une transition de jalon.
func CheckMilestone(from, to MilestoneStatus) error {
	for _, s := range milestoneMoves[from] {
		if s == to {
			return nil
		}
	}
	return fmt.Errorf("%w : jalon %s → %s", ErrTransition, from, to)
}

// Repo est le dépôt GitHub cible d'un échange.
type Repo struct {
	Owner string
	Name  string
}

// URL renvoie la forme canonique du dépôt.
func (r Repo) URL() string { return "https://github.com/" + r.Owner + "/" + r.Name }

// ErrRepoURL signale une URL de dépôt refusée.
var ErrRepoURL = errors.New("URL de dépôt invalide : https://github.com/<propriétaire>/<dépôt> attendu")

var (
	ownerRe = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})$`)
	nameRe  = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)
)

// ParseRepoURL n'accepte qu'une URL https://github.com/<owner>/<repo> :
// pas d'autre hôte, de port, d'identifiants, de requête ni de fragment.
// Aucune requête n'est jamais émise vers une URL fournie par un utilisateur
// (protection SSRF) : seuls owner et repo sont conservés.
func ParseRepoURL(s string) (Repo, error) {
	u, err := url.Parse(s)
	if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil ||
		u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" || strings.Contains(s, "#") || strings.Contains(s, "?") {
		return Repo{}, ErrRepoURL
	}
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), "/"), "/")
	if len(parts) != 2 {
		return Repo{}, ErrRepoURL
	}
	owner, name := parts[0], strings.TrimSuffix(parts[1], ".git")
	if !ownerRe.MatchString(owner) || !nameRe.MatchString(name) || name == "." || name == ".." {
		return Repo{}, ErrRepoURL
	}
	return Repo{Owner: owner, Name: name}, nil
}

// Deadlines répartit le délai de l'offre entre les jalons : le jalon i (1..n)
// est dû à ceil(jours × i / n) jours après l'acceptation (D8).
func Deadlines(acceptedAt time.Time, days, n int) []time.Time {
	out := make([]time.Time, n)
	for i := 1; i <= n; i++ {
		d := int(math.Ceil(float64(days*i) / float64(n)))
		out[i-1] = acceptedAt.AddDate(0, 0, d)
	}
	return out
}

// Actor désigne la partie qui doit agir sur l'échange.
type Actor string

const (
	ActorRequester Actor = "DEMANDEUR"
	ActorProvider  Actor = "PRESTATAIRE"
	ActorArbiter   Actor = "ARBITRE"
	ActorNone      Actor = "AUCUN"
)

// NextActor indique à qui de jouer (carte « Maintenant » de l'UI).
func NextActor(st Status, milestones []MilestoneStatus) Actor {
	switch st {
	case Proposed:
		return ActorProvider
	case InProgress, InDispute:
		for _, m := range milestones {
			if m == Disputed {
				return ActorArbiter
			}
		}
		for _, m := range milestones {
			if m == Delivered {
				return ActorRequester
			}
		}
		for _, m := range milestones {
			if m == Active {
				return ActorProvider
			}
		}
	}
	return ActorNone
}
