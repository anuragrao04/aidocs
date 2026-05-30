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
	if !h.needDocRole(c, c.Param("id"), repo.RoleEditor) {
		return
	}
	p := current(c)
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
			var latestID string
			var vce *repo.VersionConflictError
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
// @Param id path string true "Document ID"
// @Param version_id path string true "Version ID"
// @Success 200 {object} map[string]interface{}
// @Router /v1/documents/{id}/versions/{version_id}/render-token [post]
func (h handlers) createRenderToken(c *gin.Context) {
	v, ok := h.loadVersionForViewer(c)
	if !ok {
		return
	}
	subject := renderAudiencePrefix + v.DocumentID + "/" + v.ID
	token := (auth.SessionCodec{Secret: []byte(h.deps.sessionSecret)}).SignForAudience(subject, "render", 5*time.Minute)
	path := "/v/" + v.DocumentID + "/" + v.ID + "?token=" + url.QueryEscape(token)
	if h.deps.renderOrigin != "" {
		path = strings.TrimRight(h.deps.renderOrigin, "/") + path
	}
	incRender("token_created", "success")
	c.JSON(http.StatusOK, gin.H{"token": token, "url": path})
}

// RenderVersion godoc
// @Summary Render version wrapper
// @Tags render
// @Param id path string true "Document ID"
// @Param version_id path string true "Version ID"
// @Param token query string true "Render token"
// @Produce html
// @Success 200 {string} string
// @Router /v/{id}/{version_id} [get]
func (h handlers) renderVersion(c *gin.Context) {
	if h.deps.renderOrigin != "" && !hostMatchesOrigin(c.Request.Host, h.deps.renderOrigin) {
		notFound(c)
		return
	}
	docID, vid := c.Param("id"), c.Param("version_id")
	uid, ok := (auth.SessionCodec{Secret: []byte(h.deps.sessionSecret)}).VerifyAudience(c.Query("token"), "render")
	if !ok || uid != renderAudiencePrefix+docID+"/"+vid {
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
// @Param id path string true "Document ID"
// @Param version_id path string true "Version ID"
// @Success 200 {object} repo.Version
// @Router /v1/documents/{id}/versions/{version_id} [get]
func (h handlers) getVersion(c *gin.Context) {
	v, ok := h.loadVersionForViewer(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, v)
}

// loadVersionForViewer fetches the version named by the :version_id param,
// scoped to the document named by :id, and verifies the caller has at least
// viewer access to that document. It writes the appropriate error response
// and returns ok=false when any check fails.
func (h handlers) loadVersionForViewer(c *gin.Context) (repo.Version, bool) {
	docID := c.Param("id")
	if !h.needDocRole(c, docID, repo.RoleViewer) {
		return repo.Version{}, false
	}
	v, err := h.deps.repository.GetVersion(c.Request.Context(), c.Param("version_id"))
	if err != nil || v.DocumentID != docID {
		notFound(c)
		return repo.Version{}, false
	}
	return v, true
}

// GetVersionHTML godoc
// @Summary Get version HTML
// @Tags versions
// @Security bearerAuth
// @Security cookieAuth
// @Param id path string true "Document ID"
// @Param version_id path string true "Version ID"
// @Produce html
// @Success 200 {string} string
// @Router /v1/documents/{id}/versions/{version_id}/html [get]
func (h handlers) getVersionHTML(c *gin.Context) {
	v, ok := h.loadVersionForViewer(c)
	if !ok {
		return
	}
	b, err := h.deps.repository.GetVersionHTML(c.Request.Context(), v.ID)
	if err != nil {
		internalErr(c, err)
		return
	}
	observeHTML("version_download", len(b))
	incVersion("html_downloaded", actorType(c))
	c.Data(http.StatusOK, "text/html; charset=utf-8", b)
}
