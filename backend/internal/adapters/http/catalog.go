package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/gitb2pic/codetroc/backend/internal/app"
	"github.com/gitb2pic/codetroc/backend/internal/domain/catalog"
)

type catalogHandlers struct{ svc *app.Service }

// ---------- Profil (F3) ----------

func (h catalogHandlers) updateMe(c *gin.Context) {
	var in struct {
		Pseudo *string `json:"pseudo"`
		Bio    *string `json:"bio"`
	}
	if !bindJSON(c, &in) {
		return
	}
	u, err := h.svc.UpdateProfile(c.Request.Context(), actor(c), app.ProfilePatch{Pseudo: in.Pseudo, Bio: in.Bio})
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, toUserJSON(u))
}

type skillJSON struct {
	ID       uuid.UUID `json:"id"`
	Label    string    `json:"libelle"`
	Category string    `json:"categorie"`
}

type userSkillJSON struct {
	ID    uuid.UUID `json:"id"`
	Label string    `json:"libelle"`
	Level string    `json:"niveau"`
}

func toUserSkills(in []app.UserSkill) []userSkillJSON {
	out := make([]userSkillJSON, len(in))
	for i, s := range in {
		out[i] = userSkillJSON(s)
	}
	return out
}

func (h catalogHandlers) setSkills(c *gin.Context) {
	var in struct {
		Skills []struct {
			ID    uuid.UUID `json:"id_competence"`
			Level string    `json:"niveau"`
		} `json:"competences"`
	}
	if !bindJSON(c, &in) {
		return
	}
	levels := make([]app.SkillLevel, len(in.Skills))
	for i, s := range in.Skills {
		levels[i] = app.SkillLevel{SkillID: s.ID, Level: s.Level}
	}
	got, err := h.svc.SetMySkills(c.Request.Context(), actor(c), levels)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, toUserSkills(got))
}

func (h catalogHandlers) skills(c *gin.Context) {
	sk, err := h.svc.Skills(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	out := make([]skillJSON, len(sk))
	for i, s := range sk {
		out[i] = skillJSON(s)
	}
	c.JSON(http.StatusOK, out)
}

func (h catalogHandlers) publicProfile(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	p, err := h.svc.PublicProfile(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id": p.ID, "pseudo": p.Pseudo, "github_login": p.GitHubLogin, "bio": p.Bio, "score_reputation": p.Reputation,
		"date_inscription": p.JoinedAt, "competences": toUserSkills(p.Skills),
		"livraisons_verifiees": p.VerifiedDeliveries, "avis_recus": p.ReviewsCount,
	})
}

// ---------- Packs (F4) ----------

type packJSON struct {
	ID          uuid.UUID           `json:"id"`
	Title       string              `json:"titre"`
	Description string              `json:"description"`
	Deliverable string              `json:"livrable"`
	ProofType   catalog.ProofType   `json:"type_preuve"`
	Criteria    []catalog.Criterion `json:"criteres"`
	MinPrice    int64               `json:"prix_min"`
	MaxPrice    int64               `json:"prix_max"`
	Active      bool                `json:"actif"`
	Skills      []skillJSON         `json:"competences"`
	CreatedAt   time.Time           `json:"date_creation"`
}

func toPackJSON(p app.PackView) packJSON {
	sk := make([]skillJSON, len(p.Skills))
	for i, s := range p.Skills {
		sk[i] = skillJSON(s)
	}
	return packJSON{ID: p.ID, Title: p.Title, Description: p.Description, Deliverable: p.Deliverable, ProofType: p.ProofType,
		Criteria: p.Criteria, MinPrice: p.MinPrice, MaxPrice: p.MaxPrice, Active: p.Active, Skills: sk, CreatedAt: p.CreatedAt}
}

func (h catalogHandlers) listPacks(c *gin.Context) {
	limit, ok := queryInt(c, "limit", 20)
	if !ok {
		return
	}
	f := app.PackFilter{Q: c.Query("q")}
	if v := c.Query("competence"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			fail(c, &app.Error{Kind: app.KindInvalid, Code: "parametre-invalide", Title: "Requête invalide", Detail: "competence doit être un identifiant."})
			return
		}
		f.SkillID = &id
	}
	p, err := h.svc.ListPacks(c.Request.Context(), f, limit, c.Query("cursor"))
	if err != nil {
		fail(c, err)
		return
	}
	items := make([]packJSON, len(p.Items))
	for i, x := range p.Items {
		items[i] = toPackJSON(x)
	}
	c.JSON(http.StatusOK, pageJSON(items, p.NextCursor))
}

func (h catalogHandlers) getPack(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	p, err := h.svc.GetPack(c.Request.Context(), optionalActor(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, toPackJSON(p))
}

type packBody struct {
	Title       *string             `json:"titre"`
	Description *string             `json:"description"`
	Deliverable *string             `json:"livrable"`
	ProofType   *catalog.ProofType  `json:"type_preuve"`
	Criteria    []catalog.Criterion `json:"criteres"`
	MinPrice    *int64              `json:"prix_min"`
	MaxPrice    *int64              `json:"prix_max"`
	Skills      []uuid.UUID         `json:"competences"`
	Active      *bool               `json:"actif"`
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

func (h catalogHandlers) createPack(c *gin.Context) {
	var b packBody
	if !bindJSON(c, &b) {
		return
	}
	if b.Active != nil {
		fail(c, &app.Error{Kind: app.KindInvalid, Code: "corps-invalide", Title: "Requête invalide", Detail: "actif ne se règle qu'en modification."})
		return
	}
	p, err := h.svc.CreatePack(c.Request.Context(), actor(c), app.PackInput{
		Title: deref(b.Title), Description: deref(b.Description), Deliverable: deref(b.Deliverable), ProofType: deref(b.ProofType),
		Criteria: b.Criteria, MinPrice: deref(b.MinPrice), MaxPrice: deref(b.MaxPrice), SkillIDs: b.Skills})
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, toPackJSON(p))
}

func (h catalogHandlers) updatePack(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var b packBody
	if !bindJSON(c, &b) {
		return
	}
	p, err := h.svc.UpdatePack(c.Request.Context(), actor(c), id, app.PackPatch{
		Title: b.Title, Description: b.Description, Deliverable: b.Deliverable, ProofType: b.ProofType, Criteria: b.Criteria,
		MinPrice: b.MinPrice, MaxPrice: b.MaxPrice, SkillIDs: b.Skills, Active: b.Active})
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, toPackJSON(p))
}

// ---------- Offres (F5) ----------

type memberSummaryJSON struct {
	ID          uuid.UUID `json:"id"`
	Pseudo      string    `json:"pseudo"`
	GitHubLogin string    `json:"github_login"`
	Reputation  float64   `json:"score_reputation"`
	Deliveries  int       `json:"livraisons_verifiees"`
}

type offerJSON struct {
	ID          uuid.UUID           `json:"id"`
	Title       string              `json:"titre"`
	Description string              `json:"description"`
	Price       int64               `json:"prix_credits"`
	Days        int                 `json:"delai_jours"`
	Milestones  []catalog.Milestone `json:"jalons"`
	Status      string              `json:"statut"`
	PublishedAt time.Time           `json:"date_publication"`
	Pack        struct {
		ID        uuid.UUID         `json:"id"`
		Title     string            `json:"titre"`
		ProofType catalog.ProofType `json:"type_preuve"`
		Skills    []string          `json:"competences"`
	} `json:"pack"`
	Provider memberSummaryJSON `json:"prestataire"`
}

func toOfferJSON(o app.OfferView) offerJSON {
	j := offerJSON{ID: o.ID, Title: o.Title, Description: o.Description, Price: o.Price, Days: o.Days, Milestones: o.Milestones,
		Status: o.Status, PublishedAt: o.PublishedAt, Provider: memberSummaryJSON(o.Provider)}
	j.Pack.ID, j.Pack.Title, j.Pack.ProofType, j.Pack.Skills = o.Pack.ID, o.Pack.Title, o.Pack.ProofType, o.Pack.Skills
	return j
}

func (h catalogHandlers) listOffers(c *gin.Context) {
	limit, ok := queryInt(c, "limit", 20)
	if !ok {
		return
	}
	maxPrice, ok := queryInt(c, "prix_max", 0)
	if !ok {
		return
	}
	maxDays, ok := queryInt(c, "delai_max", 0)
	if !ok {
		return
	}
	f := app.OfferFilter{Skill: c.Query("competence"), Q: c.Query("q"), MaxPrice: int64(maxPrice), MaxDays: maxDays}
	for name, dst := range map[string]**uuid.UUID{"pack": &f.PackID, "prestataire": &f.ProviderID} {
		if v := c.Query(name); v != "" {
			id, err := uuid.Parse(v)
			if err != nil {
				fail(c, &app.Error{Kind: app.KindInvalid, Code: "parametre-invalide", Title: "Requête invalide", Detail: name + " doit être un identifiant."})
				return
			}
			*dst = &id
		}
	}
	if v := c.Query("reputation_min"); v != "" {
		r, err := strconv.ParseFloat(v, 64)
		if err != nil {
			fail(c, &app.Error{Kind: app.KindInvalid, Code: "parametre-invalide", Title: "Requête invalide", Detail: "reputation_min doit être un nombre."})
			return
		}
		f.MinRep = r
	}
	p, err := h.svc.ListOffers(c.Request.Context(), f, limit, c.Query("cursor"))
	if err != nil {
		fail(c, err)
		return
	}
	items := make([]offerJSON, len(p.Items))
	for i, o := range p.Items {
		items[i] = toOfferJSON(o)
	}
	c.JSON(http.StatusOK, pageJSON(items, p.NextCursor))
}

func (h catalogHandlers) getOffer(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	o, err := h.svc.GetOffer(c.Request.Context(), optionalActor(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, toOfferJSON(o))
}

func (h catalogHandlers) createOffer(c *gin.Context) {
	var b struct {
		PackID      uuid.UUID           `json:"id_pack"`
		Title       string              `json:"titre"`
		Description string              `json:"description"`
		Price       int64               `json:"prix_credits"`
		Days        int                 `json:"delai_jours"`
		Milestones  []catalog.Milestone `json:"jalons"`
	}
	if !bindJSON(c, &b) {
		return
	}
	o, err := h.svc.CreateOffer(c.Request.Context(), actor(c), app.OfferInput(b))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, toOfferJSON(o))
}

func (h catalogHandlers) withdrawOffer(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.WithdrawOffer(c.Request.Context(), actor(c), id); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
