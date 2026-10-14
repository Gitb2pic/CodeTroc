package app

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gitb2pic/codetroc/backend/internal/domain/catalog"
)

func skillID(t *testing.T, e *env, label string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, e.db.QueryRow(context.Background(), `SELECT id_competence FROM competence WHERE libelle_competence = $1`, label).Scan(&id))
	return id
}

func packID(t *testing.T, e *env, prefix string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, e.db.QueryRow(context.Background(), `SELECT id_pack FROM pack WHERE titre_pack LIKE $1 || '%'`, prefix).Scan(&id))
	return id
}

// ---------- F3 : profil et compétences ----------

func TestUpdateProfile(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	a := e.actor(t, e.login(t, "lea"))
	b := e.actor(t, e.login(t, "karim"))

	pseudo, bio := "lea-devops", "DevOps freelance."
	u, err := e.svc.UpdateProfile(ctx, a, ProfilePatch{Pseudo: &pseudo, Bio: &bio})
	require.NoError(t, err)
	assert.Equal(t, "lea-devops", u.Pseudo)
	assert.Equal(t, "DevOps freelance.", *u.Bio)

	empty := ""
	u, err = e.svc.UpdateProfile(ctx, a, ProfilePatch{Bio: &empty})
	require.NoError(t, err)
	assert.Nil(t, u.Bio, "une bio vide efface la bio")

	_, err = e.svc.UpdateProfile(ctx, b, ProfilePatch{Pseudo: &pseudo})
	requireKind(t, err, KindConflict, "pseudo-pris")

	for _, bad := range []string{"ab", "<script>", strings.Repeat("a", 51), "-tiret", "é"} {
		bad := bad
		_, err = e.svc.UpdateProfile(ctx, b, ProfilePatch{Pseudo: &bad})
		requireKind(t, err, KindInvalid, "donnees-invalides")
	}
	long := strings.Repeat("x", 1001)
	_, err = e.svc.UpdateProfile(ctx, b, ProfilePatch{Bio: &long})
	requireKind(t, err, KindInvalid, "donnees-invalides")
}

func TestSetMySkillsReplacesAtomically(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	a := e.actor(t, e.login(t, "sami"))
	docker, gha := skillID(t, e, "Docker"), skillID(t, e, "GitHub Actions")

	got, err := e.svc.SetMySkills(ctx, a, []SkillLevel{{SkillID: docker, Level: "EXPERT"}, {SkillID: gha, Level: "CONFIRME"}})
	require.NoError(t, err)
	assert.Len(t, got, 2)

	_, err = e.svc.SetMySkills(ctx, a, []SkillLevel{{SkillID: docker, Level: "EXPERT"}, {SkillID: uuid.New(), Level: "EXPERT"}})
	requireKind(t, err, KindUnprocessable, "competence-inconnue")
	_, err = e.svc.SetMySkills(ctx, a, []SkillLevel{{SkillID: docker, Level: "GOUROU"}})
	requireKind(t, err, KindInvalid, "donnees-invalides")
	_, err = e.svc.SetMySkills(ctx, a, []SkillLevel{{SkillID: docker, Level: "EXPERT"}, {SkillID: docker, Level: "DEBUTANT"}})
	requireKind(t, err, KindInvalid, "donnees-invalides")

	prof, err := e.svc.PublicProfile(ctx, a.ID)
	require.NoError(t, err)
	assert.Len(t, prof.Skills, 2, "les remplacements refusés n'ont rien changé")

	got, err = e.svc.SetMySkills(ctx, a, nil)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestPublicProfileHidesEmail(t *testing.T) {
	e := newEnv(t)
	s := e.loginAs(t, GitHubIdentity{ID: 900, Login: "public", Email: "secret@example.com", CreatedAt: e.clock.Now().AddDate(-1, 0, 0)})
	p, err := e.svc.PublicProfile(context.Background(), s.User.ID)
	require.NoError(t, err)
	assert.Equal(t, "public", p.Pseudo)
	_, err = e.svc.PublicProfile(context.Background(), uuid.New())
	requireKind(t, err, KindNotFound, "")
}

// ---------- F4 : catalogue ----------

func newPackInput(t *testing.T, e *env) PackInput {
	return PackInput{
		Title: "Migration vers PostgreSQL 16", Description: "Montée de version sans perte.", Deliverable: "Scripts et rapport",
		ProofType: catalog.ProofPRCI, Criteria: []catalog.Criterion{{Text: "Migrations vertes en CI", Check: catalog.CheckCI}},
		MinPrice: 50, MaxPrice: 150, SkillIDs: []uuid.UUID{skillID(t, e, "Postgres")},
	}
}

func TestPacks_AdminOnlyWrites(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	member := e.actor(t, e.login(t, "membre"))
	admin := e.setRole(t, e.actor(t, e.login(t, "admin")), RoleAdmin)

	_, err := e.svc.CreatePack(ctx, member, newPackInput(t, e))
	requireKind(t, err, KindForbidden, "reserve-admin")

	p, err := e.svc.CreatePack(ctx, admin, newPackInput(t, e))
	require.NoError(t, err)
	assert.Equal(t, []string{"Postgres"}, skillLabels(p.Skills))

	in := newPackInput(t, e)
	in.MinPrice, in.MaxPrice = 100, 10
	_, err = e.svc.CreatePack(ctx, admin, in)
	requireKind(t, err, KindInvalid, "donnees-invalides")
	in = newPackInput(t, e)
	in.SkillIDs = []uuid.UUID{uuid.New()}
	_, err = e.svc.CreatePack(ctx, admin, in)
	requireKind(t, err, KindUnprocessable, "competence-inconnue")

	inactive := false
	_, err = e.svc.UpdatePack(ctx, member, p.ID, PackPatch{Active: &inactive})
	requireKind(t, err, KindForbidden, "reserve-admin")
	_, err = e.svc.UpdatePack(ctx, admin, p.ID, PackPatch{Active: &inactive})
	require.NoError(t, err)

	_, err = e.svc.GetPack(ctx, &member, p.ID)
	requireKind(t, err, KindNotFound, "")
	_, err = e.svc.GetPack(ctx, nil, p.ID)
	requireKind(t, err, KindNotFound, "")
	got, err := e.svc.GetPack(ctx, &admin, p.ID)
	require.NoError(t, err)
	assert.False(t, got.Active)
}

func skillLabels(skills []Skill) []string {
	out := make([]string, len(skills))
	for i, s := range skills {
		out[i] = s.Label
	}
	return out
}

func TestListPacks(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	all, err := e.svc.ListPacks(ctx, PackFilter{}, 100, "")
	require.NoError(t, err)
	assert.Len(t, all.Items, 8, "catalogue de départ")

	docker, err := e.svc.ListPacks(ctx, PackFilter{SkillID: ptr(skillID(t, e, "Docker"))}, 100, "")
	require.NoError(t, err)
	assert.Len(t, docker.Items, 2)

	q, err := e.svc.ListPacks(ctx, PackFilter{Q: "pIpElInE"}, 100, "")
	require.NoError(t, err)
	require.Len(t, q.Items, 1)

	// Les jokers SQL ne sont pas interprétés.
	q, err = e.svc.ListPacks(ctx, PackFilter{Q: "%"}, 100, "")
	require.NoError(t, err)
	assert.Empty(t, q.Items)

	p1, err := e.svc.ListPacks(ctx, PackFilter{}, 5, "")
	require.NoError(t, err)
	p2, err := e.svc.ListPacks(ctx, PackFilter{}, 5, p1.NextCursor)
	require.NoError(t, err)
	assert.Len(t, p2.Items, 3)
}

func ptr[T any](v T) *T { return &v }

// ---------- F5 : offres ----------

func offerInput(t *testing.T, e *env) OfferInput {
	return OfferInput{
		PackID: packID(t, e, "Pipeline"), Title: "Pipeline CI/CD GitHub Actions pour app Node",
		Description: "Lint, tests, image Docker et déploiement staging.", Price: 120, Days: 5,
		Milestones: []catalog.Milestone{{Title: "Workflow CI de base", Amount: 40}, {Title: "Build & push image", Amount: 40}, {Title: "Déploiement staging", Amount: 40}},
	}
}

func TestOffers_CreateValidateList(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	lea := e.actor(t, e.login(t, "lea"))

	o, err := e.svc.CreateOffer(ctx, lea, offerInput(t, e))
	require.NoError(t, err)
	assert.Equal(t, "PUBLIEE", o.Status)
	assert.Equal(t, "lea", o.Provider.Pseudo)
	assert.Equal(t, catalog.ProofPRCI, o.Pack.ProofType)
	assert.ElementsMatch(t, []string{"GitHub Actions", "Docker"}, o.Pack.Skills)

	bad := offerInput(t, e)
	bad.Price = 500
	bad.Milestones = []catalog.Milestone{{Title: "Tout", Amount: 500}}
	_, err = e.svc.CreateOffer(ctx, lea, bad)
	requireKind(t, err, KindInvalid, "prix-hors-fourchette")

	bad = offerInput(t, e)
	bad.Milestones = bad.Milestones[:2]
	_, err = e.svc.CreateOffer(ctx, lea, bad)
	requireKind(t, err, KindInvalid, "jalons-incoherents")

	bad = offerInput(t, e)
	bad.PackID = uuid.New()
	_, err = e.svc.CreateOffer(ctx, lea, bad)
	requireKind(t, err, KindUnprocessable, "pack-indisponible")

	list, err := e.svc.ListOffers(ctx, OfferFilter{MaxPrice: 100}, 20, "")
	require.NoError(t, err)
	assert.Empty(t, list.Items)
	list, err = e.svc.ListOffers(ctx, OfferFilter{Q: "node", Skill: "Docker", MaxDays: 5}, 20, "")
	require.NoError(t, err)
	assert.Len(t, list.Items, 1)
}

func TestOffers_Withdraw(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	lea := e.actor(t, e.login(t, "lea"))
	karim := e.actor(t, e.login(t, "karim"))
	o, err := e.svc.CreateOffer(ctx, lea, offerInput(t, e))
	require.NoError(t, err)

	err = e.svc.WithdrawOffer(ctx, karim, o.ID)
	requireKind(t, err, KindForbidden, "acces-refuse")
	require.NoError(t, e.svc.WithdrawOffer(ctx, lea, o.ID))
	require.NoError(t, e.svc.WithdrawOffer(ctx, lea, o.ID), "idempotent")

	_, err = e.svc.GetOffer(ctx, &karim, o.ID)
	requireKind(t, err, KindNotFound, "")
	_, err = e.svc.GetOffer(ctx, nil, o.ID)
	requireKind(t, err, KindNotFound, "")
	got, err := e.svc.GetOffer(ctx, &lea, o.ID)
	require.NoError(t, err)
	assert.Equal(t, "RETIREE", got.Status)

	list, err := e.svc.ListOffers(ctx, OfferFilter{}, 20, "")
	require.NoError(t, err)
	assert.Empty(t, list.Items)
}
