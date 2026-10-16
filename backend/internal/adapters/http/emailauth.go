package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/gitb2pic/codetroc/backend/internal/app"
)

// F18 — inscription et connexion par email.

func (h authHandlers) methods(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"github": h.svc.GitHubAuthEnabled(), "email": h.svc.EmailAuthEnabled()})
}

func (h authHandlers) register(c *gin.Context) {
	var body struct {
		Email      string `json:"email"`
		MotDePasse string `json:"mot_de_passe"`
		Pseudo     string `json:"pseudo"`
	}
	if !bindJSON(c, &body) {
		return
	}
	if err := h.svc.Register(c.Request.Context(), app.RegisterInput{Email: body.Email, Password: body.MotDePasse, Pseudo: body.Pseudo}); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusAccepted)
}

func (h authHandlers) passwordLogin(c *gin.Context) {
	var body struct {
		Email      string `json:"email"`
		MotDePasse string `json:"mot_de_passe"`
	}
	if !bindJSON(c, &body) {
		return
	}
	sess, err := h.svc.LoginWithPassword(c.Request.Context(), body.Email, body.MotDePasse)
	if err != nil {
		fail(c, err)
		return
	}
	h.setRefresh(c, sess)
	c.JSON(http.StatusOK, gin.H{
		"access_token": sess.AccessToken,
		"token_type":   "Bearer",
		"expires_in":   int(sess.AccessTTL.Seconds()),
		"user":         toUserJSON(sess.User),
	})
}

func (h authHandlers) confirmEmail(c *gin.Context) {
	var body struct {
		Jeton string `json:"jeton"`
	}
	if !bindJSON(c, &body) {
		return
	}
	if err := h.svc.ConfirmEmail(c.Request.Context(), body.Jeton); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h authHandlers) resendConfirmation(c *gin.Context) {
	var body struct {
		Email string `json:"email"`
	}
	if !bindJSON(c, &body) {
		return
	}
	if err := h.svc.ResendConfirmation(c.Request.Context(), body.Email); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusAccepted)
}

func (h authHandlers) forgotPassword(c *gin.Context) {
	var body struct {
		Email string `json:"email"`
	}
	if !bindJSON(c, &body) {
		return
	}
	if err := h.svc.RequestPasswordReset(c.Request.Context(), body.Email); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusAccepted)
}

func (h authHandlers) resetPassword(c *gin.Context) {
	var body struct {
		Jeton      string `json:"jeton"`
		MotDePasse string `json:"mot_de_passe"`
	}
	if !bindJSON(c, &body) {
		return
	}
	if err := h.svc.ResetPassword(c.Request.Context(), body.Jeton, body.MotDePasse); err != nil {
		fail(c, err)
		return
	}
	h.setCookie(c, refreshCookie, "", authPath, -1, http.SameSiteStrictMode)
	c.Status(http.StatusNoContent)
}

// linkGitHub prépare la liaison GitHub du compte connecté : le navigateur
// suit ensuite l'URL renvoyée, et le retour GitHub passe par le callback.
func (h authHandlers) linkGitHub(c *gin.Context) {
	authURL, state, err := h.svc.StartGitHubLink(c.Request.Context(), actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	h.setCookie(c, stateCookie, state, authPath+"/github", 600, http.SameSiteLaxMode)
	c.JSON(http.StatusOK, gin.H{"url": authURL})
}
