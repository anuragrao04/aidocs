package server

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/anuragrao/aidocs/api/internal/auth"
	"github.com/anuragrao/aidocs/api/internal/repo"
)

// CreateVersion godoc
// @Summary Create version
// @Tags versions
// @Security bearerAuth
// @Security cookieAuth
// @Accept multipart/form-data
// @Param id path string true "Document ID"
// @Param base_version_id formData string true "Base version ID"
// @Param change_summary formData string false "Change summary"
// @Param file formData file true "HTML file"
// @Success 201 {object} map[string]interface{}
// @Router /v1/documents/{id}/versions [post]
func (h handlers) createVersion(c *gin.Context) {
	p := current(c)
	role, _ := h.deps.repository.RoleForDocument(c.Request.Context(), *p, c.Param("id"))
	if !atLeast(role, repo.RoleEditor) {
		forbidden(c, "editor role required")
		return
	}
	html, err := readMultipartFile(c, "file")
	if errors.Is(err, errPayloadTooLarge) {
		c.JSON(http.StatusRequestEntityTooLarge, errorResponse("payload_too_large", "HTML file exceeds 10 MiB", nil))
		return
	}
	if err != nil {
		badRequest(c, "file is required")
		return
	}
	observeHTML("version_create", len(html))
	v, err := h.deps.repository.CreateVersion(c.Request.Context(), c.Param("id"), c.PostForm("base_version_id"), c.PostForm("change_summary"), html, *p)
	if err != nil {
		// A conflict means the base version was stale; report the current one.
		if errors.Is(err, repo.ErrVersionConflict) {
			var vce *repo.VersionConflictError
			latestID := v.ID // fallback: still populated by Memory/Postgres
			if errors.As(err, &vce) {
				latestID = vce.LatestVersionID
			}
			c.JSON(http.StatusConflict, errorResponse("version_conflict", "base_version_id is stale", gin.H{"current_version_id": latestID}))
			return
		}
		internalErr(c, err)
		return
	}
	incVersion("created", actorType(c))
	c.JSON(http.StatusCreated, gin.H{"id": v.ID, "number": v.Number, "sha256": v.SHA256})
}

// CreateRenderToken godoc
// @Summary Create render token
// @Tags versions
// @Security bearerAuth
// @Security cookieAuth
// @Param id path string true "Version ID"
// @Success 200 {object} map[string]interface{}
// @Router /v1/versions/{id}/render-token [post]
func (h handlers) createRenderToken(c *gin.Context) {
	v, err := h.deps.repository.GetVersion(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFound(c)
		return
	}
	if !h.needDocRole(c, v.DocumentID, repo.RoleViewer) {
		return
	}
	token := (auth.SessionCodec{Secret: []byte(h.deps.sessionSecret)}).SignForAudience(renderAudiencePrefix+v.ID, "render", 5*time.Minute)
	path := "/v/" + v.ID + "?token=" + url.QueryEscape(token)
	if h.deps.renderOrigin != "" {
		path = strings.TrimRight(h.deps.renderOrigin, "/") + path
	}
	incRender("token_created", "success")
	c.JSON(http.StatusOK, gin.H{"token": token, "url": path})
}

// RenderVersion godoc
// @Summary Render version wrapper
// @Tags render
// @Param version_id path string true "Version ID"
// @Param token query string true "Render token"
// @Produce html
// @Success 200 {string} string
// @Router /v/{version_id} [get]
func (h handlers) renderVersion(c *gin.Context) {
	if h.deps.renderOrigin != "" && !hostMatchesOrigin(c.Request.Host, h.deps.renderOrigin) {
		notFound(c)
		return
	}
	vid := c.Param("version_id")
	uid, ok := (auth.SessionCodec{Secret: []byte(h.deps.sessionSecret)}).VerifyAudience(c.Query("token"), "render")
	if !ok || uid != renderAudiencePrefix+vid {
		c.JSON(http.StatusUnauthorized, errorResponse("unauthorized", "invalid render token", nil))
		return
	}
	b, err := h.deps.repository.GetVersionHTML(c.Request.Context(), vid)
	if err != nil {
		incRender("rendered", "not_found")
		notFound(c)
		return
	}
	appOrigin := h.deps.appOrigin
	if appOrigin == "" {
		appOrigin = "'self'"
	}
	c.Header("Content-Security-Policy", "default-src 'none'; img-src data: https:; style-src 'unsafe-inline'; script-src 'unsafe-inline'; frame-ancestors "+appOrigin)
	c.Header("X-Content-Type-Options", "nosniff")
	observeHTML("render", len(b))
	incRender("rendered", "success")
	c.Data(http.StatusOK, "text/html; charset=utf-8", renderWrapperHTML(b, appOrigin))
}

// ListVersions godoc
// @Summary List versions
// @Tags versions
// @Security bearerAuth
// @Security cookieAuth
// @Param id path string true "Document ID"
// @Success 200 {object} map[string]interface{}
// @Router /v1/documents/{id}/versions [get]
func (h handlers) listVersions(c *gin.Context) {
	if !h.needDocRole(c, c.Param("id"), repo.RoleViewer) {
		return
	}
	items, err := h.deps.repository.ListVersions(c.Request.Context(), c.Param("id"))
	if err != nil {
		internalErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// GetVersion godoc
// @Summary Get version metadata
// @Tags versions
// @Security bearerAuth
// @Security cookieAuth
// @Param id path string true "Version ID"
// @Success 200 {object} repo.Version
// @Router /v1/versions/{id} [get]
func (h handlers) getVersion(c *gin.Context) {
	v, err := h.deps.repository.GetVersion(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFound(c)
		return
	}
	if !h.needDocRole(c, v.DocumentID, repo.RoleViewer) {
		return
	}
	c.JSON(http.StatusOK, v)
}

// GetVersionHTML godoc
// @Summary Get version HTML
// @Tags versions
// @Security bearerAuth
// @Security cookieAuth
// @Param id path string true "Version ID"
// @Produce html
// @Success 200 {string} string
// @Router /v1/versions/{id}/html [get]
func (h handlers) getVersionHTML(c *gin.Context) {
	v, err := h.deps.repository.GetVersion(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFound(c)
		return
	}
	if !h.needDocRole(c, v.DocumentID, repo.RoleViewer) {
		return
	}
	b, err := h.deps.repository.GetVersionHTML(c.Request.Context(), c.Param("id"))
	if err != nil {
		internalErr(c, err)
		return
	}
	observeHTML("version_download", len(b))
	incVersion("html_downloaded", actorType(c))
	c.Data(http.StatusOK, "text/html; charset=utf-8", b)
}
