package httpapi

import "github.com/gin-gonic/gin"

// WebhookConfig configure la réception des webhooks GitHub (F7).
type WebhookConfig struct {
	Secret []byte
}

// mountRoutes déclare les routes métier. Chaque feature ajoute les siennes ici,
// ce qui donne une vue d'ensemble de la surface exposée.
func mountRoutes(v1 *gin.RouterGroup, o Options) {
	auth := authHandlers{svc: o.App, frontendURL: o.FrontendURL, secure: o.CookieSecure}
	v1.GET("/auth/github/login", auth.login)
	v1.GET("/auth/github/callback", auth.callback)
	v1.POST("/auth/refresh", auth.refresh)
	v1.POST("/auth/logout", auth.logout)

	m := v1.Group("", requireAuth(o.App))
	m.GET("/me", auth.me)

	cat := catalogHandlers{svc: o.App}
	m.PATCH("/me", cat.updateMe)
	m.PUT("/me/competences", cat.setSkills)
	m.POST("/packs", cat.createPack)
	m.PATCH("/packs/:id", cat.updatePack)
	m.POST("/offres", cat.createOffer)
	m.POST("/offres/:id/retirer", cat.withdrawOffer)

	pub := v1.Group("", optionalAuth(o.App))
	pub.GET("/competences", cat.skills)
	pub.GET("/utilisateurs/:id", cat.publicProfile)
	pub.GET("/packs", cat.listPacks)
	pub.GET("/packs/:id", cat.getPack)
	pub.GET("/offres", cat.listOffers)
	pub.GET("/offres/:id", cat.getOffer)

	credits := creditHandlers{svc: o.App}
	m.GET("/me/compte", credits.balance)
	m.GET("/me/compte/mouvements", credits.movements)
}
