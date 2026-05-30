package server

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/anuragrao/aidocs/api/internal/auth"
	"github.com/anuragrao/aidocs/api/internal/blob"
	"github.com/anuragrao/aidocs/api/internal/repo"
)

// CreateDocument godoc
// @Summary Create document
// @Tags documents
// @Security bearerAuth
// @Security cookieAuth
// @Accept multipart/form-data
// @Produce json
// @Param title formData string true "Document title"
// @Param visibility formData string false "private|org|link"
// @Param file formData file true "Single-file HTML"
// @Success 201 {object} map[string]interface{}
// @Router /v1/documents [post]
func (h handlers) createDocument(c *gin.Context) {
	p := current(c)
	if p.Type == auth.PrincipalServiceAccount {
		forbidden(c, "service accounts cannot own documents")
		return
	}
	title := c.PostForm("title")
	vis := c.PostForm("visibility")
	if vis == "" {
		vis = visibilityPrivate
	}
	if !validVisibility(vis) {
		badRequest(c, "invalid visibility")
		return
	}
	html, err := readMultipartFile(c, "file")
	if errors.Is(err, errPayloadTooLarge) {
		c.JSON(http.StatusRequestEntityTooLarge, errorResponse("payload_too_large", "HTML file exceeds 10 MiB", nil))
		return
	}
	if err != nil || title == "" {
		badRequest(c, "invalid document upload")
		return
	}
	observeHTML("document_create", len(html))
	d, _, err := h.deps.repository.CreateDocument(c.Request.Context(), *p, title, vis, html)
	if err != nil {
		if errors.Is(err, blob.ErrStorage) {
			c.JSON(http.StatusBadGateway, errorResponse("blob_storage_failed", "could not upload HTML to blob storage", nil))
			return
		}
		internalErr(c, err)
		return
	}
	incDocument("created", d.Visibility, actorType(c))
	incVersion("created_initial", actorType(c))
	c.JSON(http.StatusCreated, gin.H{"id": d.ID, "current_version_id": d.CurrentVersionID})
}

// ListDocuments godoc
// @Summary List documents
// @Tags documents
// @Security bearerAuth
// @Security cookieAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /v1/documents [get]
func (h handlers) listDocuments(c *gin.Context) {
	items, err := h.deps.repository.ListDocuments(c.Request.Context(), *current(c))
	if err != nil {
		internalErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// GetDocument godoc
// @Summary Get document
// @Tags documents
// @Security bearerAuth
// @Security cookieAuth
// @Param id path string true "Document ID"
// @Success 200 {object} repo.Document
// @Router /v1/documents/{id} [get]
func (h handlers) getDocument(c *gin.Context) {
	if !h.needDocRole(c, c.Param("id"), repo.RoleViewer) {
		return
	}
	d, err := h.deps.repository.GetDocument(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFound(c)
		return
	}
	c.JSON(http.StatusOK, d)
}

// PatchDocument godoc
// @Summary Update document
// @Tags documents
// @Security bearerAuth
// @Security cookieAuth
// @Param id path string true "Document ID"
// @Success 200 {object} repo.Document
// @Router /v1/documents/{id} [patch]
func (h handlers) patchDocument(c *gin.Context) {
	if !h.needDocRole(c, c.Param("id"), repo.RoleOwner) {
		return
	}
	var in struct{ Title, Visibility string }
	if err := c.ShouldBindJSON(&in); err != nil {
		badRequest(c, "invalid body")
		return
	}
	if in.Visibility != "" && !validVisibility(in.Visibility) {
		badRequest(c, "invalid visibility")
		return
	}
	d, err := h.deps.repository.UpdateDocument(c.Request.Context(), c.Param("id"), in.Title, in.Visibility)
	if err != nil {
		internalErr(c, err)
		return
	}
	c.JSON(http.StatusOK, d)
}

// DeleteDocument godoc
// @Summary Delete document
// @Tags documents
// @Security bearerAuth
// @Security cookieAuth
// @Param id path string true "Document ID"
// @Success 204
// @Router /v1/documents/{id} [delete]
func (h handlers) deleteDocument(c *gin.Context) {
	if !h.needDocRole(c, c.Param("id"), repo.RoleOwner) {
		return
	}
	if err := h.deps.repository.DeleteDocument(c.Request.Context(), c.Param("id")); err != nil {
		internalErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
