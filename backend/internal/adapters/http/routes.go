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
}
