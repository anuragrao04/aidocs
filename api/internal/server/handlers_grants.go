package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/anuragrao/aidocs/api/internal/auth"
	"github.com/anuragrao/aidocs/api/internal/bots"
	"github.com/anuragrao/aidocs/api/internal/repo"
)

// resolveGrantPrincipal turns a grant request's address or inline principal
// spec into a concrete auth.Principal.
// Sentinel errors returned by resolveGrantPrincipal so callers can map them to
// HTTP responses without matching on error message text.
var (
	errInvalidAddress      = errors.New("invalid grant address")
	errPrincipalIDRequired = errors.New("principal id required")
)

func resolveGrantPrincipal(c *gin.Context, r repo.Repository, address string, principal auth.Principal) (auth.Principal, error) {
	// The "anyone" principal is the whole-server audience; it has no address and
	// no backing record to resolve.
	if principal.Type == auth.PrincipalAnyone {
		return auth.Principal{Type: auth.PrincipalAnyone}, nil
	}
	if address != "" {
		_, domain, ok := bots.Split(address)
		if !ok {
			return auth.Principal{}, errInvalidAddress
		}
		if strings.HasSuffix(domain, ".bot") {
			principal = auth.Principal{Type: auth.PrincipalServiceAccount, Name: address}
		} else {
			principal = auth.Principal{Type: auth.PrincipalUser, Email: address}
		}
	}
	switch {
	case principal.Type == auth.PrincipalUser && principal.ID == "" && principal.Email != "":
		p, err := r.EnsureUserByEmail(c.Request.Context(), principal.Email)
		if err != nil {
			return auth.Principal{}, err
		}
		return p, nil
	case principal.Type == auth.PrincipalServiceAccount && principal.ID == "" && principal.Name != "":
		sa, err := r.GetServiceAccountByName(c.Request.Context(), principal.Name)
		if err != nil {
			return auth.Principal{}, err
		}
		return auth.Principal{Type: auth.PrincipalServiceAccount, ID: sa.ID, Name: sa.Name}, nil
	default:
		if principal.ID == "" {
			return auth.Principal{}, errPrincipalIDRequired
		}
		exists, err := r.PrincipalExists(c.Request.Context(), principal)
		if err != nil {
			return auth.Principal{}, err
		}
		if !exists {
			return auth.Principal{}, repo.ErrNotFound
		}
		return principal, nil
	}
}

// CreateGrant godoc
// @Summary Create document grant
// @Tags grants
// @Security bearerAuth
// @Security cookieAuth
// @Param id path string true "Document ID"
// @Success 201 {object} map[string]interface{}
// @Router /v1/documents/{id}/grants [post]
func (h handlers) createGrant(c *gin.Context) {
	if !h.needDocRole(c, c.Param("id"), repo.RoleOwner) {
		return
	}
	p := current(c)
	var in struct {
		Address   string `json:"address"`
		Principal struct {
			Type  auth.PrincipalType `json:"type"`
			ID    string             `json:"id"`
			Email string             `json:"email"`
			Name  string             `json:"name"`
		} `json:"principal"`
		Role repo.Role `json:"role"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		badRequest(c, "invalid grant")
		return
	}
	if !validGrantRole(in.Role) {
		badRequest(c, "invalid role")
		return
	}
	principal := auth.Principal{Type: in.Principal.Type, ID: in.Principal.ID, Email: in.Principal.Email, Name: in.Principal.Name}
	resolved, err := resolveGrantPrincipal(c, h.deps.repository, in.Address, principal)
	if err != nil {
		switch {
		case errors.Is(err, errInvalidAddress):
			badRequest(c, "That doesn't look like an email or bot address.")
		case errors.Is(err, errPrincipalIDRequired):
			badRequest(c, "principal id, user email, or service account name is required")
		case errors.Is(err, repo.ErrNotFound):
			notFound(c)
		default:
			internalErr(c, err)
		}
		return
	}
	g, err := h.deps.repository.CreateGrant(c.Request.Context(), c.Param("id"), resolved, in.Role, *p)
	if err != nil {
		internalErr(c, err)
		return
	}
	pr := gin.H{"type": g.Principal.Type, "id": g.Principal.ID}
	if g.Principal.Email != "" {
		pr["email"] = g.Principal.Email
	}
	if g.Principal.Name != "" {
		pr["name"] = g.Principal.Name
	}
	incGrant("created", string(g.Role), string(g.Principal.Type), actorType(c))
	c.JSON(http.StatusCreated, gin.H{"id": g.ID, "resource": gin.H{"type": "document", "id": g.DocumentID}, "principal": pr, "role": g.Role, "granted_by": gin.H{"id": g.GrantedBy.ID, "email": g.GrantedBy.Email}})
}

// ListGrants godoc
// @Summary List document grants
// @Tags grants
// @Security bearerAuth
// @Security cookieAuth
// @Param id path string true "Document ID"
// @Success 200 {object} map[string]interface{}
// @Router /v1/documents/{id}/grants [get]
func (h handlers) listGrants(c *gin.Context) {
	if !h.needDocRole(c, c.Param("id"), repo.RoleOwner) {
		return
	}
	items, err := h.deps.repository.ListGrants(c.Request.Context(), c.Param("id"))
	if err != nil {
		internalErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// PatchGrant godoc
// @Summary Update document grant
// @Tags grants
// @Security bearerAuth
// @Security cookieAuth
// @Param id path string true "Document ID"
// @Param grant_id path string true "Grant ID"
// @Success 200 {object} repo.Grant
// @Router /v1/documents/{id}/grants/{grant_id} [patch]
func (h handlers) patchGrant(c *gin.Context) {
	if !h.needDocRole(c, c.Param("id"), repo.RoleOwner) {
		return
	}
	var in struct {
		Role repo.Role `json:"role"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || !validGrantRole(in.Role) {
		badRequest(c, "invalid grant")
		return
	}
	g, err := h.deps.repository.UpdateGrant(c.Request.Context(), c.Param("id"), c.Param("grant_id"), in.Role)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			notFound(c)
			return
		}
		internalErr(c, err)
		return
	}
	incGrant("updated", string(g.Role), string(g.Principal.Type), actorType(c))
	c.JSON(http.StatusOK, g)
}

// DeleteGrant godoc
// @Summary Delete document grant
// @Tags grants
// @Security bearerAuth
// @Security cookieAuth
// @Param id path string true "Document ID"
// @Param grant_id path string true "Grant ID"
// @Success 204
// @Router /v1/documents/{id}/grants/{grant_id} [delete]
func (h handlers) deleteGrant(c *gin.Context) {
	if !h.needDocRole(c, c.Param("id"), repo.RoleOwner) {
		return
	}
	if err := h.deps.repository.DeleteGrant(c.Request.Context(), c.Param("id"), c.Param("grant_id")); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			notFound(c)
			return
		}
		internalErr(c, err)
		return
	}
	incGrant("deleted", labelUnknown, labelUnknown, actorType(c))
	c.Status(http.StatusNoContent)
}
