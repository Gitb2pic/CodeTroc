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

	ex := exchangeHandlers{svc: o.App}
	idem := idempotent(o.App)
	m.POST("/echanges", ex.create)
	m.GET("/echanges", ex.list)
	m.GET("/echanges/:id", ex.get)
	m.POST("/echanges/:id/accepter", idem, ex.accept)
	m.POST("/echanges/:id/refuser", idem, ex.refuse)
	m.POST("/echanges/:id/annuler", idem, ex.cancel)

	dl := deliveryHandlers{svc: o.App, webhooks: o.Webhooks}
	m.POST("/jalons/:id/preuves", dl.submitProof)
	m.POST("/jalons/:id/reverifier", dl.reverify)
	m.POST("/jalons/:id/valider", idem, dl.validate)
	v1.POST("/webhooks/github", dl.githubWebhook) // authentifié par signature HMAC

	disp := disputeHandlers{svc: o.App}
	m.POST("/jalons/:id/litiges", disp.open)
	m.GET("/litiges", disp.list)
	m.GET("/litiges/:id", disp.get)
	m.POST("/litiges/:id/reponse", disp.respond)
	m.POST("/litiges/:id/affecter", disp.assign)
	m.POST("/litiges/:id/decision", idem, disp.decide)

	soc := socialHandlers{svc: o.App}
	m.POST("/echanges/:id/avis", soc.leaveReview)
	pub.GET("/utilisateurs/:id/avis", soc.listReviews)
	m.GET("/echanges/:id/messages", soc.listMessages)
	m.POST("/echanges/:id/messages", soc.postMessage)

	vault := vaultHandlers{svc: o.App}
	m.GET("/echanges/:id/secrets", vault.list)
	m.POST("/echanges/:id/secrets", vault.share)
	m.POST("/secrets/:id/reveler", vault.reveal) // pas d'idempotence : la réponse contient le secret
	m.POST("/secrets/:id/revoquer", vault.revoke)
	m.GET("/secrets/:id/journal", vault.journal)

	credits := creditHandlers{svc: o.App}
	m.GET("/me/compte", credits.balance)
	m.GET("/me/compte/mouvements", credits.movements)
}
