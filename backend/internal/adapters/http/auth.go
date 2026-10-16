package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gitb2pic/codetroc/backend/internal/app"
)

const (
	refreshCookie = "ct_refresh"
	stateCookie   = "ct_oauth_state"
	authPath      = "/api/v1/auth"
	actorKey      = "actor"
)

type authHandlers struct {
	svc         *app.Service
	frontendURL string
	secure      bool
}

type userJSON struct {
	ID            string    `json:"id"`
	Pseudo        string    `json:"pseudo"`
	Email         *string   `json:"email"`
	GitHubLogin   string    `json:"github_login"`
	GitHubLinked  bool      `json:"github_lie"`
	EmailVerified bool      `json:"email_verifie"`
	Bio           *string   `json:"bio"`
	Role          app.Role  `json:"role"`
	Reputation    float64   `json:"score_reputation"`
	JoinedAt      time.Time `json:"date_inscription"`
}

func toUserJSON(u app.User) userJSON {
	return userJSON{ID: u.ID.String(), Pseudo: u.Pseudo, Email: u.Email, GitHubLogin: u.GitHubLogin, GitHubLinked: u.HasGitHub, EmailVerified: u.EmailVerified, Bio: u.Bio,
		Role: u.Role, Reputation: u.Reputation, JoinedAt: u.JoinedAt}
}

func (h authHandlers) setCookie(c *gin.Context, name, value, path string, maxAge int, sameSite http.SameSite) {
	// Secure suit COOKIE_SECURE, refusé à false en production par la configuration.
	http.SetCookie(c.Writer, &http.Cookie{Name: name, Value: value, Path: path, MaxAge: maxAge, // #nosec G124 -- voir ci-dessus
		HttpOnly: true, Secure: h.secure, SameSite: sameSite})
}

func (h authHandlers) login(c *gin.Context) {
	authURL, state, err := h.svc.StartLogin(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	// Lax : le cookie doit revenir lors de la redirection depuis GitHub (navigation GET).
	h.setCookie(c, stateCookie, state, authPath+"/github", 600, http.SameSiteLaxMode)
	c.Redirect(http.StatusFound, authURL)
}

func (h authHandlers) callback(c *gin.Context) {
	h.setCookie(c, stateCookie, "", authPath+"/github", -1, http.SameSiteLaxMode)
	if c.Query("error") != "" {
		h.redirectFront(c, "acces-refuse")
		return
	}
	cookie, _ := c.Cookie(stateCookie)
	sess, err := h.svc.CompleteLogin(c.Request.Context(), c.Query("state"), cookie, c.Query("code"))
	if err != nil {
		var ae *app.Error
		if errors.As(err, &ae) {
			h.redirectFront(c, ae.Code)
			return
		}
		_ = c.Error(err)
		h.redirectFront(c, "erreur-interne")
		return
	}
	h.setRefresh(c, sess)
	h.redirectFront(c, "ok")
}

func (h authHandlers) redirectFront(c *gin.Context, outcome string) {
	u, err := url.Parse(h.frontendURL)
	if err != nil {
		u = &url.URL{Path: "/"}
	}
	q := u.Query()
	q.Set("connexion", outcome)
	u.RawQuery = q.Encode()
	c.Redirect(http.StatusFound, u.String())
}

func (h authHandlers) setRefresh(c *gin.Context, s app.Session) {
	h.setCookie(c, refreshCookie, s.RefreshToken, authPath, int(s.RefreshTTL.Seconds()), http.SameSiteStrictMode)
}

func (h authHandlers) refresh(c *gin.Context) {
	cookie, err := c.Cookie(refreshCookie)
	if err != nil || cookie == "" {
		fail(c, app.ErrUnauthenticated)
		return
	}
	sess, err := h.svc.Refresh(c.Request.Context(), cookie)
	if err != nil {
		h.setCookie(c, refreshCookie, "", authPath, -1, http.SameSiteStrictMode)
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

func (h authHandlers) logout(c *gin.Context) {
	if cookie, err := c.Cookie(refreshCookie); err == nil && cookie != "" {
		if err := h.svc.Logout(c.Request.Context(), cookie); err != nil {
			AbortWithError(c, err)
			return
		}
	}
	h.setCookie(c, refreshCookie, "", authPath, -1, http.SameSiteStrictMode)
	c.Status(http.StatusNoContent)
}

func (h authHandlers) me(c *gin.Context) {
	u, err := h.svc.Me(c.Request.Context(), actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, toUserJSON(u))
}

// requireAuth exige un jeton d'accès valide et un compte actif.
func requireAuth(svc *app.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		tok, ok := strings.CutPrefix(h, "Bearer ")
		if !ok || tok == "" {
			fail(c, app.ErrUnauthenticated)
			return
		}
		a, err := svc.Authenticate(c.Request.Context(), tok)
		if err != nil {
			fail(c, err)
			return
		}
		c.Set(actorKey, a)
		c.Next()
	}
}

// optionalAuth reconnaît un jeton s'il est présent ; un jeton invalide reste une erreur 401.
func optionalAuth(svc *app.Service) gin.HandlerFunc {
	required := requireAuth(svc)
	return func(c *gin.Context) {
		if c.GetHeader("Authorization") == "" {
			c.Next()
			return
		}
		required(c)
	}
}

// optionalActor renvoie l'acteur s'il est authentifié, nil sinon.
func optionalActor(c *gin.Context) *app.Actor {
	if a, ok := c.Get(actorKey); ok {
		act := a.(app.Actor)
		return &act
	}
	return nil
}

func actor(c *gin.Context) app.Actor {
	a, _ := c.Get(actorKey)
	return a.(app.Actor)
}
