package server

import (
	"bytes"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

// frontendFS contains the Vite production build. The directory is
// populated by `make frontend` (or `npm run build` in frontend/) and is
// gitignored apart from .gitkeep placeholders, so this embed always has
// something to point at even when no build has happened yet. The
// fallback path in registerFrontendRoutes returns 404 for index.html in
// that case.
//
//go:embed all:frontend_dist
var frontendFS embed.FS

//go:embed onboarding/sample.html
var onboardingFS embed.FS

func registerFrontendRoutes(r *gin.Engine, publicURL string) {
	dist, err := fs.Sub(frontendFS, "frontend_dist")
	if err != nil {
		return
	}
	fileServer := http.FileServer(http.FS(dist))
	publicURL = strings.TrimRight(publicURL, "/")

	r.GET("/onboarding/sample.html", func(c *gin.Context) {
		b, err := fs.ReadFile(onboardingFS, "onboarding/sample.html")
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		c.Header("Content-Disposition", `attachment; filename="aidocs-sample.html"`)
		c.Data(http.StatusOK, "text/html; charset=utf-8", b)
	})

	r.GET("/assets/*filepath", gin.WrapH(http.StripPrefix("/assets/", http.FileServer(http.FS(subOrSelf(dist, "assets"))))))
	r.NoRoute(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Status(http.StatusNotFound)
			return
		}
		urlPath := strings.TrimPrefix(path.Clean(c.Request.URL.Path), "/")
		if urlPath != "." && urlPath != "" && fileExists(dist, urlPath) {
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}
		index, err := fs.ReadFile(dist, "index.html")
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		index = bytes.ReplaceAll(index, []byte("__AIDOCS_PUBLIC_URL_VALUE__"), []byte(publicURL))
		c.Data(http.StatusOK, "text/html; charset=utf-8", index)
	})
}

// subOrSelf returns a sub-filesystem rooted at dir, or falls back to fsys
// with a log warning if the subdirectory does not exist. (server-15)
func subOrSelf(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		log.Printf("subOrSelf: falling back to parent FS because %q not found: %v", dir, err)
		return fsys
	}
	return sub
}

func fileExists(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && !info.IsDir()
}
