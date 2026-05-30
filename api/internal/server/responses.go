package server

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func notFound(c *gin.Context) {
	c.JSON(http.StatusNotFound, errorResponse("not_found", "not found", nil))
}

func badRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, errorResponse("bad_request", msg, nil))
}

func forbidden(c *gin.Context, msg string) {
	c.JSON(http.StatusForbidden, errorResponse("forbidden", msg, nil))
}

// internalErr logs err (if non-nil) and writes a 500 response.
func internalErr(c *gin.Context, err error) {
	if err != nil {
		log.Printf("internal error method=%s path=%s error=%v", c.Request.Method, c.Request.URL.Path, err)
	} else {
		log.Printf("internal error method=%s path=%s", c.Request.Method, c.Request.URL.Path)
	}
	c.JSON(http.StatusInternalServerError, errorResponse("internal", "internal server error", nil))
}

func errorResponse(code, message string, details any) gin.H {
	err := gin.H{"code": code, "message": message}
	if details != nil {
		err["details"] = details
	}
	return gin.H{"error": err}
}
