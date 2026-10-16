// Package scrum porte les règles du cadre Scrum d'un échange (F16) :
// découpage des jalons en sprints, cycles de vie du sprint et de la story,
// risque de retard, template et rendu des issues GitHub.
package scrum

import (
	"math"
	"time"
)

// Window est la fenêtre contractuelle d'un jalon : [début, échéance].
type Window struct {
	Start, End time.Time
}

// Sprint est un sprint planifié : numéro global au projet, index du jalon
// (0-based) et dates prévues.
type Sprint struct {
	Number    int
	Milestone int
	Start     time.Time
	End       time.Time
}

const dayDur = 24 * time.Hour

// MilestoneWindows déduit les fenêtres des jalons de leurs échéances :
// le jalon 1 commence à l'acceptation, le jalon i à l'échéance du jalon i-1.
func MilestoneWindows(acceptedAt time.Time, deadlines []time.Time) []Window {
	out := make([]Window, len(deadlines))
	prev := acceptedAt
	for i, d := range deadlines {
		out[i] = Window{Start: prev, End: d}
		prev = d
	}
	return out
}

// splitWindow découpe une fenêtre en sprints de d jours (R-S3) :
//   - L ≤ d : un seul sprint ;
//   - sinon k = ⌊L/d⌋ sprints de d jours et un reste r : r < d/2 est ajouté
//     au dernier sprint, r ≥ d/2 forme un sprint court.
//
// Le dernier sprint se termine exactement à la fin de la fenêtre. Une
// fenêtre vide (délai inférieur au nombre de jalons) ne produit aucun sprint.
func splitWindow(w Window, d int) [][2]time.Time {
	if !w.End.After(w.Start) {
		return nil
	}
	L := int(math.Round(w.End.Sub(w.Start).Hours() / 24))
	if L <= d {
		return [][2]time.Time{{w.Start, w.End}}
	}
	k, r := L/d, L%d
	if r > 0 && 2*r >= d {
		k++
	}
	out := make([][2]time.Time, k)
	for i := range k {
		start := w.Start.Add(time.Duration(i*d) * dayDur)
		end := w.Start.Add(time.Duration((i+1)*d) * dayDur)
		if i == k-1 {
			end = w.End
		}
		out[i] = [2]time.Time{start, end}
	}
	return out
}

// Split découpe tous les jalons ; les sprints sont numérotés globalement à
// partir de first.
func Split(ws []Window, d, first int) []Sprint {
	var out []Sprint
	n := first
	for mi, w := range ws {
		for _, p := range splitWindow(w, d) {
			out = append(out, Sprint{Number: n, Milestone: mi, Start: p[0], End: p[1]})
			n++
		}
	}
	return out
}

// Replan redécoupe les sprints à venir après un avenant (R-S10) : les
// sprints conservés (terminés, annulés, actif) gardent leurs dates ; pour
// chaque jalon non soldé, la partie de sa nouvelle fenêtre située après son
// dernier sprint conservé est redécoupée. La numérotation reprend après le
// plus grand numéro conservé.
func Replan(ws []Window, kept []Sprint, closed []bool, d int) []Sprint {
	next := 1
	lastEnd := make([]time.Time, len(ws))
	for _, s := range kept {
		if s.Number >= next {
			next = s.Number + 1
		}
		if s.Milestone < len(ws) && s.End.After(lastEnd[s.Milestone]) {
			lastEnd[s.Milestone] = s.End
		}
	}
	var out []Sprint
	for mi, w := range ws {
		if closed[mi] {
			continue
		}
		if lastEnd[mi].After(w.Start) {
			w.Start = lastEnd[mi]
		}
		for _, p := range splitWindow(w, d) {
			out = append(out, Sprint{Number: next, Milestone: mi, Start: p[0], End: p[1]})
			next++
		}
	}
	return out
}
