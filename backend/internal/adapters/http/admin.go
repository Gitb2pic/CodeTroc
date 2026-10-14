package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/gitb2pic/codetroc/backend/internal/app"
)

type adminHandlers struct{ svc *app.Service }

type adminUserJSON struct {
	userJSON
	Status string `json:"statut"`
}

func toAdminUserJSON(u app.User) adminUserJSON {
	return adminUserJSON{userJSON: toUserJSON(u), Status: u.Status}
}

func (h adminHandlers) listUsers(c *gin.Context) {
	limit, ok := queryInt(c, "limit", 20)
	if !ok {
		return
	}
	p, err := h.svc.AdminListUsers(c.Request.Context(), actor(c), c.Query("q"), limit, c.Query("cursor"))
	if err != nil {
		fail(c, err)
		return
	}
	items := make([]adminUserJSON, len(p.Items))
	for i, u := range p.Items {
		items[i] = toAdminUserJSON(u)
	}
	c.JSON(http.StatusOK, pageJSON(items, p.NextCursor))
}

func (h adminHandlers) updateUser(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var b struct {
		Role   *app.Role `json:"role"`
		Status *string   `json:"statut"`
	}
	if !bindJSON(c, &b) {
		return
	}
	u, err := h.svc.AdminUpdateUser(c.Request.Context(), actor(c), id, app.AdminPatch{Role: b.Role, Status: b.Status})
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, toAdminUserJSON(u))
}

func (h adminHandlers) disputeQueue(c *gin.Context) {
	ds, err := h.svc.AdminDisputeQueue(c.Request.Context(), actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	out := make([]disputeJSON, len(ds))
	for i, d := range ds {
		out[i] = disputeJSON(d)
	}
	c.JSON(http.StatusOK, out)
}

// deleteMe anonymise le compte (D13) et efface le cookie de session.
func (h authHandlers) deleteMe(c *gin.Context) {
	if err := h.svc.DeleteAccount(c.Request.Context(), actor(c)); err != nil {
		fail(c, err)
		return
	}
	h.setCookie(c, refreshCookie, "", authPath, -1, http.SameSiteStrictMode)
	c.Status(http.StatusNoContent)
}
