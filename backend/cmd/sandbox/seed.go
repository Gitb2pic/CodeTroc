//go:build js && wasm

package main

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gitb2pic/codetroc/backend/internal/app"
	"github.com/gitb2pic/codetroc/backend/internal/domain/catalog"
)

// Personas du bac à sable, proposés sur la fausse page de connexion.
var personas = []struct{ login, role, bio string }{
	{"sami", "MEMBRE", "Dev front, je cherche de l'aide sur l'infra et la CI."},
	{"lea", "MEMBRE", "DevOps. GitHub Actions, Docker, un peu de Kubernetes."},
	{"karim", "MEMBRE", "Conteneurs, observabilité, tests de bout en bout."},
	{"ines", "MEMBRE", "React et accessibilité, revues de code."},
	{"arbitre", "ARBITRE", "Arbitre des litiges de la communauté."},
	{"admin", "ADMIN", "Administratrice de la plateforme."},
}

type offerSeed struct {
	provider, packPrefix, title string
	days                        int
	milestones                  []catalog.Milestone
}

var offers = []offerSeed{
	{"lea", "Pipeline CI/CD", "Pipeline CI/CD GitHub Actions pour app Node", 5, []catalog.Milestone{{Title: "Workflow lint + tests", Amount: 40}, {Title: "Build et push de l'image", Amount: 40}, {Title: "Déploiement staging et doc", Amount: 40}}},
	{"karim", "Dockeriser", "Dockeriser une app Next.js + Postgres", 3, []catalog.Milestone{{Title: "Dockerfile multi-étapes", Amount: 45}, {Title: "Compose avec la base", Amount: 45}}},
	{"ines", "Intégration de maquette", "Formulaire React accessible (WCAG AA)", 4, []catalog.Milestone{{Title: "Composants et validation", Amount: 50}, {Title: "Audit clavier et lecteur d'écran", Amount: 50}}},
	{"lea", "Tests unitaires", "Couvrir un module critique en tests unitaires", 3, []catalog.Milestone{{Title: "Cas nominaux", Amount: 30}, {Title: "Cas limites et CI", Amount: 30}}},
	{"karim", "Suite de tests E2E", "Suite de tests E2E Playwright", 3, []catalog.Milestone{{Title: "Parcours de connexion", Amount: 35}, {Title: "Parcours de paiement", Amount: 35}}},
	{"ines", "Revue de code", "Revue de sécurité d'une PR", 2, []catalog.Milestone{{Title: "Revue commentée", Amount: 40}}},
}

var skills = map[string][]string{
	"lea":   {"GitHub Actions", "Docker", "Kubernetes"},
	"karim": {"Docker", "Playwright", "Postgres"},
	"ines":  {"React", "TypeScript"},
	"sami":  {"React", "TypeScript"},
}

func login(ctx context.Context, svc *app.Service, name string) (app.Actor, error) {
	_, state, err := svc.StartLogin(ctx)
	if err != nil {
		return app.Actor{}, err
	}
	sess, err := svc.CompleteLogin(ctx, state, state, name)
	if err != nil {
		return app.Actor{}, err
	}
	return svc.Authenticate(ctx, sess.AccessToken)
}

// seed remplit une base neuve ; une base déjà remplie (rechargement) est laissée telle quelle.
func seed(ctx context.Context, svc *app.Service, pool *pgxpool.Pool) error {
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM utilisateur`).Scan(&n); err != nil || n > 0 {
		return err
	}
	actors := map[string]app.Actor{}
	for _, p := range personas {
		a, err := login(ctx, svc, p.login)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, `UPDATE utilisateur SET role = $2, bio = $3 WHERE id_utilisateur = $1`, a.ID, p.role, p.bio); err != nil {
			return err
		}
		actors[p.login] = a
	}

	catalogSkills, err := svc.Skills(ctx)
	if err != nil {
		return err
	}
	for who, labels := range skills {
		var levels []app.SkillLevel
		for _, l := range labels {
			for _, s := range catalogSkills {
				if s.Label == l {
					levels = append(levels, app.SkillLevel{SkillID: s.ID, Level: "CONFIRME"})
				}
			}
		}
		if _, err := svc.SetMySkills(ctx, actors[who], levels); err != nil {
			return err
		}
	}

	packs, err := svc.ListPacks(ctx, app.PackFilter{}, 100, "")
	if err != nil {
		return err
	}
	var firstOffer *app.OfferView
	for _, o := range offers {
		for _, p := range packs.Items {
			if !strings.HasPrefix(p.Title, o.packPrefix) {
				continue
			}
			var price int64
			for _, m := range o.milestones {
				price += m.Amount
			}
			view, err := svc.CreateOffer(ctx, actors[o.provider], app.OfferInput{PackID: p.ID, Title: o.title, Price: price, Days: o.days, Milestones: o.milestones})
			if err != nil {
				return err
			}
			if firstOffer == nil {
				firstOffer = &view
			}
			break
		}
	}

	// Un échange déjà en cours : Sami a demandé le pipeline de Léa, qui a accepté.
	if firstOffer != nil {
		x, err := svc.RequestExchange(ctx, actors["sami"], app.ExchangeRequest{OfferID: firstOffer.ID, RepoURL: "https://github.com/acme/shop-api",
			Need: "Les tests sont dans /test, npm test suffit. Vise Node 18 et 20."})
		if err != nil {
			return err
		}
		if _, err := svc.AcceptExchange(ctx, actors["lea"], x.ID); err != nil {
			return err
		}
		if _, err := svc.PostMessage(ctx, actors["lea"], x.ID, "Salut ! Je commence par le workflow de base, PR prévue demain."); err != nil {
			return err
		}
	}
	return nil
}
