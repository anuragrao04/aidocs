package server

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/anuragrao/aidocs/api/internal/auth"
	"github.com/anuragrao/aidocs/api/internal/repo"
)

// Config holds static configuration for the server.
type Config struct {
	Environment         string
	AppOrigin           string
	RenderOrigin        string
	GoogleOAuth         auth.GoogleOAuth
	SessionSecret       string
	AllowedOAuthDomains []string
	// Deployment is "public" or "org". OrgName labels the org in UI copy.
	Deployment string
	OrgName    string
}

// Server wraps the gin engine.
type Server struct{ engine *gin.Engine }

type dependencies struct {
	authenticator       auth.Authenticator
	repository          repo.Repository
	stateStore          auth.LoginStateStore
	googleOAuth         auth.GoogleOAuth
	sessionSecret       string
	appOrigin           string
	renderOrigin        string
	allowedOAuthDomains []string
	deployment          string
	orgName             string
}

// Option configures the Server.
type Option func(*dependencies)

func WithAuthenticator(a auth.Authenticator) Option {
	return func(d *dependencies) { d.authenticator = a }
}

func WithRepository(r repo.Repository) Option { return func(d *dependencies) { d.repository = r } }

func WithStateStore(st auth.LoginStateStore) Option {
	return func(d *dependencies) { d.stateStore = st }
}

// denyAllAuthenticator rejects every request. It is the default authenticator,
// so a server constructed without WithAuthenticator denies all requests instead
// of silently trusting them.
type denyAllAuthenticator struct{}

func (denyAllAuthenticator) Authenticate(_ *http.Request) (*auth.Principal, error) {
	return nil, auth.ErrUnauthorized
}

// logFormatter formats a single gin access-log line.
func logFormatter(param gin.LogFormatterParams) string {
	return param.TimeStamp.Format(time.RFC3339) + " " +
		param.Method + " " +
		param.Path + " " +
		param.ClientIP + " " +
		param.StatusCodeColor() + http.StatusText(param.StatusCode) + param.ResetColor() + " " +
		param.Latency.String() + "\n"
}

// New constructs and returns a configured Server. The authenticator defaults to
// one that denies every request, and a repository must be supplied via
// WithRepository outside of tests.
func New(cfg Config, opts ...Option) *Server {
	gin.SetMode(gin.ReleaseMode)
	deps := dependencies{
		authenticator:       denyAllAuthenticator{},
		stateStore:          auth.NewStateStore(),
		googleOAuth:         cfg.GoogleOAuth,
		sessionSecret:       cfg.SessionSecret,
		appOrigin:           cfg.AppOrigin,
		renderOrigin:        cfg.RenderOrigin,
		allowedOAuthDomains: cfg.AllowedOAuthDomains,
		deployment:          cfg.Deployment,
		orgName:             cfg.OrgName,
	}
	for _, opt := range opts {
		opt(&deps)
	}
	// Outside tests a repository is mandatory; falling back to an in-memory
	// store would silently discard data on restart.
	if deps.repository == nil {
		if cfg.Environment == envTest {
			deps.repository = repo.NewMemory()
		} else {
			panic("server.New: repository is required (use server.WithRepository)")
		}
	}
	r := gin.New()
	r.RedirectTrailingSlash = false
	if cfg.Environment != envTest {
		r.Use(gin.LoggerWithFormatter(logFormatter))
	}
	r.Use(gin.Recovery())
	r.Use(prometheusMiddleware())
	s := &Server{engine: r}
	s.routes(deps)
	return s
}

func (s *Server) Handler() http.Handler { return s.engine }

func (s *Server) Run(addr string) error { return s.engine.Run(addr) }

func (s *Server) routes(deps dependencies) {
	h := handlers{deps: deps}
	s.engine.GET("/metrics", metricsHandler())
	s.engine.GET("/.well-known/aidocs.json", h.discovery)
	s.engine.GET("/commit.txt", h.commitTXT)
	registerAPIDocsRoutes(s.engine)
	s.engine.GET("/v/:id/:version_id", h.renderVersion)
	registerFrontendRoutes(s.engine, deps.appOrigin)

	v1 := s.engine.Group("/v1")
	v1.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	v1.GET("/config", h.config)
	v1.GET("/auth/google/start", h.googleStart)
	v1.GET("/auth/google/callback", h.googleCallback)
	v1.POST("/auth/cli/exchange", h.cliExchange)

	// Every route below requires authentication.
	authed := v1.Group("", requireAuth(deps.authenticator))
	authed.GET("/me", h.me)
	authed.GET("/auth/cli/credentials", h.listCLICredentials)
	authed.DELETE("/auth/cli/credentials/:id", h.revokeCLICredential)
	authed.POST("/service-accounts", h.createServiceAccount)
	authed.GET("/service-accounts", h.listServiceAccounts)
	authed.PATCH("/service-accounts/:id", h.patchServiceAccount)
	authed.POST("/service-accounts/:id/keys", h.createServiceAccountKey)
	authed.GET("/service-accounts/:id/keys", h.listServiceAccountKeys)
	authed.DELETE("/service-accounts/:id/keys/:key_id", h.revokeServiceAccountKey)
	authed.POST("/service-accounts/:id/transfer", h.createOwnershipTransfer)
	authed.GET("/service-accounts/transfers", h.listOwnershipTransfers)
	authed.POST("/service-accounts/transfers/:id/accept", h.acceptOwnershipTransfer)
	authed.POST("/service-accounts/transfers/:id/decline", h.declineOwnershipTransfer)
	authed.POST("/documents", h.createDocument)
	authed.GET("/documents", h.listDocuments)
	authed.GET("/documents/:id", h.getDocument)
	authed.PATCH("/documents/:id", h.patchDocument)
	authed.DELETE("/documents/:id", h.deleteDocument)
	authed.POST("/documents/:id/grants", h.createGrant)
	authed.GET("/documents/:id/grants", h.listGrants)
	authed.PATCH("/documents/:id/grants/:grant_id", h.patchGrant)
	authed.DELETE("/documents/:id/grants/:grant_id", h.deleteGrant)
	authed.POST("/documents/:id/versions", h.createVersion)
	authed.GET("/documents/:id/versions", h.listVersions)
	authed.GET("/documents/:id/versions/:version_id", h.getVersion)
	authed.GET("/documents/:id/versions/:version_id/html", h.getVersionHTML)
	authed.POST("/documents/:id/versions/:version_id/render-token", h.createRenderToken)
	authed.POST("/documents/:id/comments", h.createComment)
	authed.GET("/documents/:id/comments", h.listComments)
	authed.PATCH("/documents/:id/comments/:comment_id", h.patchComment)
	authed.DELETE("/documents/:id/comments/:comment_id", h.deleteComment)
}

type handlers struct{ deps dependencies }

func (h handlers) needDocRole(c *gin.Context, docID string, need repo.Role) bool {
	role, _ := h.deps.repository.RoleForDocument(c.Request.Context(), *current(c), docID)
	if !atLeast(role, need) {
		forbidden(c, string(need)+" role required")
		return false
	}
	return true
}

func requireAuth(a auth.Authenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, err := a.Authenticate(c.Request)
		if err != nil {
			incAuth("request", "failure")
			c.AbortWithStatusJSON(http.StatusUnauthorized, errorResponse("unauthorized", "authentication required", nil))
			return
		}
		incAuth("request", "success")
		c.Set("principal", p)
		c.Next()
	}
}

func current(c *gin.Context) *auth.Principal {
	p, ok := c.Get("principal")
	if !ok {
		return nil
	}
	principal, _ := p.(*auth.Principal)
	return principal
}

var errPayloadTooLarge = errors.New("payload too large")

const maxHTMLBytes = 10 * 1024 * 1024

func readMultipartFile(c *gin.Context, field string) ([]byte, error) {
	f, err := c.FormFile(field)
	if err != nil {
		return nil, err
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	b, err := io.ReadAll(io.LimitReader(rc, maxHTMLBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxHTMLBytes {
		return nil, errPayloadTooLarge
	}
	return b, nil
}
