package catalog

import "fmt"

// Durées de sprint permises (D20).
const (
	SprintWeek     = 7
	SprintTwoWeeks = 14
)

// Announcement est la demande d'aide d'un membre (F17) : il décrit son
// besoin, son budget et les jalons souhaités ; des prestataires candidatent.
type Announcement struct {
	Title      string
	Need       string
	Budget     int64
	Days       int
	SprintDays int
	Milestones []Milestone
}

// Application est la candidature d'un prestataire à une annonce : ses
// propres prix, délai et jalons, éventuellement ajustés.
type Application struct {
	Price      int64
	Days       int
	Message    string
	Milestones []Milestone
}

func checkPrice(field string, v int64, p Pack) error {
	if v < p.MinPrice || v > p.MaxPrice {
		return &ValidationError{Code: "prix-hors-fourchette", Field: field,
			Detail: fmt.Sprintf("doit être compris entre %d et %d cr", p.MinPrice, p.MaxPrice)}
	}
	return nil
}

func checkDays(v int) error {
	if v < 1 || v > maxDays {
		return fieldErr("delai_jours", fmt.Sprintf("entre 1 et %d jours", maxDays))
	}
	return nil
}

// Validate vérifie une annonce sur un pack : budget dans la fourchette du
// pack, délai, durée de sprint et jalons dont la somme égale le budget (RG10).
func (a Announcement) Validate(p Pack) error {
	if err := checkText("titre", a.Title, 5, 120); err != nil {
		return err
	}
	if err := checkText("besoin", a.Need, 1, 4000); err != nil {
		return err
	}
	if err := checkPrice("budget_credits", a.Budget, p); err != nil {
		return err
	}
	if err := checkDays(a.Days); err != nil {
		return err
	}
	if err := ValidateSprintDays(a.SprintDays); err != nil {
		return err
	}
	return ValidateMilestones(a.Budget, a.Milestones)
}

// Validate vérifie une candidature sur le pack de l'annonce (RG08, RG10).
func (c Application) Validate(p Pack) error {
	if err := checkPrice("prix_credits", c.Price, p); err != nil {
		return err
	}
	if err := checkDays(c.Days); err != nil {
		return err
	}
	if err := checkText("message", c.Message, 0, 2000); err != nil {
		return err
	}
	return ValidateMilestones(c.Price, c.Milestones)
}

// ValidateSprintDays : sprints de 7 ou 14 jours (D20).
func ValidateSprintDays(d int) error {
	if d != SprintWeek && d != SprintTwoWeeks {
		return fieldErr("duree_sprint_jours", "7 ou 14 jours")
	}
	return nil
}
