package server

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/anuragrao/aidocs/api/internal/repo"
)

// CreateComment godoc
// @Summary Create comment
// @Tags comments
// @Security bearerAuth
// @Security cookieAuth
// @Accept json
// @Param id path string true "Document ID"
// @Success 201 {object} map[string]interface{}
// @Router /v1/documents/{id}/comments [post]
func (h handlers) createComment(c *gin.Context) {
	p := current(c)
	role, _ := h.deps.repository.RoleForDocument(c.Request.Context(), *p, c.Param("id"))
	if !atLeast(role, repo.RoleCommenter) {
		forbidden(c, "commenter role required")
		return
	}
	var in struct {
		VersionID string      `json:"version_id"`
		Body      string      `json:"body"`
		Anchor    repo.Anchor `json:"anchor"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.VersionID == "" || in.Body == "" || in.Anchor.Quote == "" {
		badRequest(c, "invalid comment")
		return
	}
	cm, err := h.deps.repository.CreateComment(c.Request.Context(), c.Param("id"), in.VersionID, in.Body, in.Anchor, *p)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			notFound(c)
			return
		}
		internalErr(c, err)
		return
	}
	incComment("created", cm.Status, actorType(c))
	c.JSON(http.StatusCreated, commentJSON(cm, cm.VersionID, ""))
}

// ListComments godoc
// @Summary List comments
// @Tags comments
// @Security bearerAuth
// @Security cookieAuth
// @Param id path string true "Document ID"
// @Param status query string false "open|resolved|stale|orphaned|all"
// @Param version_id query string false "Version ID"
// @Success 200 {object} map[string]interface{}
// @Router /v1/documents/{id}/comments [get]
func (h handlers) listComments(c *gin.Context) {
	if !h.needDocRole(c, c.Param("id"), repo.RoleViewer) {
		return
	}
	versionID := c.Query("version_id")
	items, err := h.deps.repository.ListComments(c.Request.Context(), c.Param("id"), c.Query("status"), versionID)
	if err != nil {
		internalErr(c, err)
		return
	}
	placementVersionID := versionID
	if placementVersionID == "" {
		d, err := h.deps.repository.GetDocument(c.Request.Context(), c.Param("id"))
		if err == nil {
			placementVersionID = d.CurrentVersionID
		}
	}
	var placementHTML []byte
	if placementVersionID != "" {
		placementHTML, _ = h.deps.repository.GetVersionHTML(c.Request.Context(), placementVersionID)
	}
	c.JSON(http.StatusOK, gin.H{"items": commentsJSON(items, placementVersionID, placementHTML)})
}

// PatchComment godoc
// @Summary Update comment
// @Tags comments
// @Security bearerAuth
// @Security cookieAuth
// @Param id path string true "Document ID"
// @Param comment_id path string true "Comment ID"
// @Success 200 {object} map[string]interface{}
// @Router /v1/documents/{id}/comments/{comment_id} [patch]
func (h handlers) patchComment(c *gin.Context) {
	if !h.canMutateComment(c, c.Param("id"), c.Param("comment_id")) {
		return
	}
	var in struct{ Body, Status string }
	_ = c.ShouldBindJSON(&in)
	if in.Status != "" && in.Status != commentStatusOpen && in.Status != commentStatusResolved {
		badRequest(c, "invalid comment status")
		return
	}
	cm, err := h.deps.repository.UpdateComment(c.Request.Context(), c.Param("comment_id"), in.Body, in.Status)
	if err != nil {
		internalErr(c, err)
		return
	}
	event := "updated"
	if in.Status == commentStatusResolved {
		event = "resolved"
	} else if in.Status == commentStatusOpen {
		event = "reopened"
	}
	incComment(event, cm.Status, actorType(c))
	c.JSON(http.StatusOK, commentJSON(cm, cm.VersionID, ""))
}

// DeleteComment godoc
// @Summary Delete comment
// @Tags comments
// @Security bearerAuth
// @Security cookieAuth
// @Param id path string true "Document ID"
// @Param comment_id path string true "Comment ID"
// @Success 204
// @Router /v1/documents/{id}/comments/{comment_id} [delete]
func (h handlers) deleteComment(c *gin.Context) {
	if !h.canMutateComment(c, c.Param("id"), c.Param("comment_id")) {
		return
	}
	if err := h.deps.repository.DeleteComment(c.Request.Context(), c.Param("comment_id")); err != nil {
		internalErr(c, err)
		return
	}
	incComment("deleted", "unknown", actorType(c))
	c.Status(http.StatusNoContent)
}

func (h handlers) canMutateComment(c *gin.Context, docID, commentID string) bool {
	p := current(c)
	cm, err := h.deps.repository.GetComment(c.Request.Context(), commentID)
	if err != nil {
		forbidden(c, "comment access required")
		return false
	}
	if cm.DocumentID != docID {
		c.JSON(http.StatusNotFound, errorResponse("not_found", "comment not found in document", nil))
		return false
	}
	if cm.Author.Type == p.Type && cm.Author.ID == p.ID {
		return true
	}
	role, _ := h.deps.repository.RoleForDocument(c.Request.Context(), *p, cm.DocumentID)
	if role == repo.RoleOwner || role == repo.RoleEditor {
		return true
	}
	forbidden(c, "comment access required")
	return false
}
