package server

import (
	"strings"

	"github.com/anuragrao/aidocs/api/internal/auth"
	"github.com/anuragrao/aidocs/api/internal/repo"
	"github.com/gin-gonic/gin"
)

func commentsJSON(items []repo.Comment, placementVersionID string, placementHTML []byte) []gin.H {
	// Stringify the version body once for all placement checks below.
	haystack := string(placementHTML)
	out := make([]gin.H, 0, len(items))
	for _, cm := range items {
		out = append(out, commentJSON(cm, placementVersionID, haystack))
	}
	return out
}

// commentPlacement reports whether a comment's selected text still appears in
// the given version, returning its placement status, a confidence score, and
// the matched text. placementHTML is the already-stringified version body.
func commentPlacement(cm repo.Comment, placementVersionID, placementHTML string) (status string, confidence float64, matched string) {
	status = placementAttached
	confidence = 1.0
	matched = cm.SelectedText
	if placementVersionID != cm.VersionID && len(placementHTML) > 0 && !strings.Contains(placementHTML, cm.SelectedText) {
		status = placementOrphaned
		confidence = 0
		matched = ""
	}
	return
}

func principalJSON(p auth.Principal) gin.H {
	out := gin.H{"type": p.Type, "id": p.ID}
	if p.Email != "" {
		out["email"] = p.Email
	}
	if p.Name != "" {
		out["name"] = p.Name
	}
	if p.PictureURL != "" {
		out["picture_url"] = p.PictureURL
	}
	return out
}

func commentJSON(cm repo.Comment, placementVersionID, placementHTML string) gin.H {
	if placementVersionID == "" {
		placementVersionID = cm.VersionID
	}
	status, confidence, matched := commentPlacement(cm, placementVersionID, placementHTML)
	return gin.H{
		"id":                    cm.ID,
		"author":                principalJSON(cm.Author),
		"body":                  cm.Body,
		"selected_text":         cm.SelectedText,
		"anchor":                cm.Anchor,
		"status":                cm.Status,
		"created_on_version_id": cm.VersionID,
		"current_placement": gin.H{
			"version_id":   placementVersionID,
			"status":       status,
			"anchor":       cm.Anchor,
			"matched_text": matched,
			"confidence":   confidence,
		},
	}
}
