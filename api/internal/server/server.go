package server

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"

	"github.com/anuragrao/aidocs/api/internal/auth"
	"github.com/anuragrao/aidocs/api/internal/repo"
)

type Config struct {
	Environment         string
	AppOrigin           string
	RenderOrigin        string
	GoogleOAuth         auth.GoogleOAuth
	SessionSecret       string
	AllowedOAuthDomains []string
}
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
}
type Option func(*dependencies)

func WithAuthenticator(a auth.Authenticator) Option {
	return func(d *dependencies) { d.authenticator = a }
}
func WithRepository(r repo.Repository) Option { return func(d *dependencies) { d.repository = r } }
func WithStateStore(st auth.LoginStateStore) Option {
	return func(d *dependencies) { d.stateStore = st }
}

func New(cfg Config, opts ...Option) *Server {
	gin.SetMode(gin.ReleaseMode)
	deps := dependencies{authenticator: auth.TestHeaderAuthenticator{}, repository: repo.NewMemory(), stateStore: auth.NewStateStore(), googleOAuth: cfg.GoogleOAuth, sessionSecret: cfg.SessionSecret, appOrigin: cfg.AppOrigin, renderOrigin: cfg.RenderOrigin, allowedOAuthDomains: cfg.AllowedOAuthDomains}
	for _, opt := range opts {
		opt(&deps)
	}
	r := gin.New()
	r.RedirectTrailingSlash = false
	if cfg.Environment != "test" {
		r.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
			return param.TimeStamp.Format(time.RFC3339) + " " + param.Method + " " + param.Path + " " + param.ClientIP + " " + param.StatusCodeColor() + http.StatusText(param.StatusCode) + param.ResetColor() + " " + param.Latency.String() + "\n"
		}))
	}
	r.Use(gin.Recovery())
	s := &Server{engine: r}
	s.routes(deps)
	return s
}
func (s *Server) Handler() http.Handler { return s.engine }
func (s *Server) Run(addr string) error { return s.engine.Run(addr) }

func (s *Server) routes(deps dependencies) {
	h := handlers{deps: deps}
	s.engine.GET("/.well-known/aidocs.json", h.discovery)
	s.engine.GET("/commit.txt", h.commitTXT)
	registerAPIDocsRoutes(s.engine)
	s.engine.GET("/v/:version_id", h.renderVersion)
	registerFrontendRoutes(s.engine)
	v1 := s.engine.Group("/v1")
	v1.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	v1.GET("/me", requireAuth(deps.authenticator), h.me)
	v1.GET("/auth/google/start", h.googleStart)
	v1.GET("/auth/google/callback", h.googleCallback)
	v1.POST("/auth/cli/exchange", h.cliExchange)
	v1.GET("/auth/cli/credentials", requireAuth(deps.authenticator), h.listCLICredentials)
	v1.DELETE("/auth/cli/credentials/:id", requireAuth(deps.authenticator), h.revokeCLICredential)
	v1.POST("/service-accounts", requireAuth(deps.authenticator), h.createServiceAccount)
	v1.POST("/documents", requireAuth(deps.authenticator), h.createDocument)
	v1.POST("/documents/:id/grants", requireAuth(deps.authenticator), h.createGrant)
	v1.POST("/documents/:id/versions", requireAuth(deps.authenticator), h.createVersion)
	v1.POST("/documents/:id/comments", requireAuth(deps.authenticator), h.createComment)

	v1.GET("/service-accounts", requireAuth(deps.authenticator), h.listServiceAccounts)
	v1.PATCH("/service-accounts/:id", requireAuth(deps.authenticator), h.patchServiceAccount)
	v1.POST("/service-accounts/:id/keys", requireAuth(deps.authenticator), h.createServiceAccountKey)
	v1.GET("/service-accounts/:id/keys", requireAuth(deps.authenticator), h.listServiceAccountKeys)
	v1.DELETE("/service-accounts/:id/keys/:key_id", requireAuth(deps.authenticator), h.revokeServiceAccountKey)
	v1.POST("/service-accounts/:id/transfer", requireAuth(deps.authenticator), h.createOwnershipTransfer)
	v1.GET("/service-accounts/transfers", requireAuth(deps.authenticator), h.listOwnershipTransfers)
	v1.POST("/service-accounts/transfers/:id/accept", requireAuth(deps.authenticator), h.acceptOwnershipTransfer)
	v1.POST("/service-accounts/transfers/:id/decline", requireAuth(deps.authenticator), h.declineOwnershipTransfer)
	v1.GET("/documents", requireAuth(deps.authenticator), h.listDocuments)
	v1.GET("/documents/:id", requireAuth(deps.authenticator), h.getDocument)
	v1.PATCH("/documents/:id", requireAuth(deps.authenticator), h.patchDocument)
	v1.DELETE("/documents/:id", requireAuth(deps.authenticator), h.deleteDocument)
	v1.GET("/documents/:id/grants", requireAuth(deps.authenticator), h.listGrants)
	v1.PATCH("/documents/:id/grants/:grant_id", requireAuth(deps.authenticator), h.patchGrant)
	v1.DELETE("/documents/:id/grants/:grant_id", requireAuth(deps.authenticator), h.deleteGrant)
	v1.GET("/documents/:id/versions", requireAuth(deps.authenticator), h.listVersions)
	v1.GET("/versions/:id", requireAuth(deps.authenticator), h.getVersion)
	v1.GET("/versions/:id/html", requireAuth(deps.authenticator), h.getVersionHTML)
	v1.POST("/versions/:id/render-token", requireAuth(deps.authenticator), h.createRenderToken)
	v1.GET("/documents/:id/comments", requireAuth(deps.authenticator), h.listComments)
	v1.PATCH("/documents/:id/comments/:comment_id", requireAuth(deps.authenticator), h.patchComment)
	v1.DELETE("/documents/:id/comments/:comment_id", requireAuth(deps.authenticator), h.deleteComment)
}

type handlers struct{ deps dependencies }

// GoogleStart godoc
// @Summary Start Google OAuth
// @Tags auth
// @Param mode query string false "web or cli"
// @Param redirect query string false "web redirect"
// @Param cli_redirect query string false "CLI loopback redirect"
// @Param code_challenge query string false "PKCE challenge"
// @Param state query string false "OAuth state"
// @Success 302
// @Router /v1/auth/google/start [get]
func (h handlers) googleStart(c *gin.Context) {
	mode := c.DefaultQuery("mode", "web")
	state := c.Query("state")
	if state == "" {
		var err error
		state, err = auth.RandomURLToken()
		if err != nil {
			internal(c)
			return
		}
	}
	st := auth.LoginState{Mode: mode, Redirect: c.Query("redirect"), CLIRedirect: c.Query("cli_redirect"), CodeChallenge: c.Query("code_challenge"), ExpiresAt: time.Now().Add(10 * time.Minute)}
	if mode == "web" && !safeWebRedirect(st.Redirect) {
		badRequest(c, "invalid redirect")
		return
	}
	if mode == "cli" {
		if st.CodeChallenge == "" || !auth.LoopbackRedirectAllowed(st.CLIRedirect) {
			badRequest(c, "invalid cli oauth parameters")
			return
		}
	}
	if err := h.deps.stateStore.PutState(state, st); err != nil {
		internal(c)
		return
	}
	// PKCE is between the CLI and aidocs. Do not forward the CLI's
	// code_challenge to Google, because aidocs performs the Google code
	// exchange and does not have the CLI's code_verifier at that point.
	c.Redirect(http.StatusFound, h.deps.googleOAuth.AuthCodeURL(state, oauth2.AccessTypeOffline))
}

// GoogleCallback godoc
// @Summary Google OAuth callback
// @Tags auth
// @Param code query string true "OAuth code"
// @Param state query string true "OAuth state"
// @Success 302
// @Router /v1/auth/google/callback [get]
func (h handlers) googleCallback(c *gin.Context) {
	st, ok := h.deps.stateStore.TakeState(c.Query("state"))
	if !ok {
		badRequest(c, "invalid oauth state")
		return
	}
	gu, err := h.deps.googleOAuth.ExchangeAndVerify(c.Request.Context(), c.Query("code"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, errorResponse("unauthorized", "google oauth failed", nil))
		return
	}
	if !emailDomainAllowed(gu.Email, h.deps.allowedOAuthDomains) {
		c.JSON(http.StatusForbidden, errorResponse("forbidden", "google account domain is not allowed", nil))
		return
	}
	uid := "usr_google_" + gu.Sub
	p, err := h.deps.repository.UpsertGoogleUser(c.Request.Context(), uid, gu.Email, gu.Name, gu.Sub, gu.PictureURL)
	if err != nil {
		internal(c)
		return
	}
	if st.Mode == "cli" {
		code, err := auth.RandomURLToken()
		if err != nil {
			internal(c)
			return
		}
		st.GoogleUser = gu
		st.UserID = p.ID
		if err := h.deps.stateStore.PutCode(code, st); err != nil {
			internal(c)
			return
		}
		u, _ := url.Parse(st.CLIRedirect)
		q := u.Query()
		q.Set("code", code)
		q.Set("state", c.Query("state"))
		u.RawQuery = q.Encode()
		c.Redirect(http.StatusFound, u.String())
		return
	}
	h.setSessionCookie(c, p)
	redir := st.Redirect
	if redir == "" {
		redir = "/"
	}
	c.Redirect(http.StatusFound, redir)
}

// CLIExchange godoc
// @Summary Exchange CLI login code
// @Tags auth
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /v1/auth/cli/exchange [post]
func (h handlers) cliExchange(c *gin.Context) {
	var in struct {
		Code         string `json:"code"`
		CodeVerifier string `json:"code_verifier"`
		Name         string `json:"name"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.Code == "" || in.CodeVerifier == "" {
		badRequest(c, "invalid cli exchange")
		return
	}
	st, ok := h.deps.stateStore.TakeCode(in.Code)
	if !ok || !auth.VerifyPKCES256(in.CodeVerifier, st.CodeChallenge) {
		c.JSON(http.StatusUnauthorized, errorResponse("unauthorized", "invalid cli code", nil))
		return
	}
	token, hash, err := auth.NewBearerToken("aidocs_cli_")
	if err != nil {
		internal(c)
		return
	}
	name := in.Name
	if name == "" {
		name = "cli"
	}
	id, err := h.deps.repository.CreateCLICredential(c.Request.Context(), st.UserID, name, hash)
	if err != nil {
		internal(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "token": token, "principal": gin.H{"type": "user", "id": st.UserID, "email": st.GoogleUser.Email}})
}

// ListCLICredentials godoc
// @Summary List CLI credentials
// @Tags auth
// @Security bearerAuth
// @Security cookieAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /v1/auth/cli/credentials [get]
func (h handlers) listCLICredentials(c *gin.Context) {
	p := current(c)
	items, err := h.deps.repository.ListCLICredentials(c.Request.Context(), p.ID)
	if err != nil {
		internal(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// RevokeCLICredential godoc
// @Summary Revoke CLI credential
// @Tags auth
// @Security bearerAuth
// @Security cookieAuth
// @Param id path string true "Credential ID"
// @Success 204
// @Router /v1/auth/cli/credentials/{id} [delete]
func (h handlers) revokeCLICredential(c *gin.Context) {
	p := current(c)
	if err := h.deps.repository.RevokeCLICredential(c.Request.Context(), p.ID, c.Param("id")); err != nil {
		internal(c)
		return
	}
	c.Status(http.StatusNoContent)
}
func (h handlers) setSessionCookie(c *gin.Context, p auth.Principal) {
	cookie := (auth.SessionCodec{Secret: []byte(h.deps.sessionSecret)}).Sign(p.ID)
	c.SetCookie("aidocs_session", cookie, 86400*30, "/", "", true, true)
}

// Discovery godoc
// @Summary Deployment discovery
// @Tags discovery
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /.well-known/aidocs.json [get]
func (h handlers) discovery(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"name": "aidocs", "api_version": "v1", "auth": gin.H{"modes": []string{"web", "cli"}}})
}

// CommitTXT godoc
// @Summary Build commit SHA
// @Tags discovery
// @Produce plain
// @Success 200 {string} string
// @Router /commit.txt [get]
func (h handlers) commitTXT(c *gin.Context) {
	sha := os.Getenv("AIDOCS_COMMIT_SHA")
	if sha == "" {
		sha = os.Getenv("COMMIT_SHA")
	}
	c.String(http.StatusOK, sha)
}

// Me godoc
// @Summary Current principal
// @Tags auth
// @Produce json
// @Security bearerAuth
// @Security cookieAuth
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /v1/me [get]
func (h handlers) me(c *gin.Context) {
	p := current(c)
	out := gin.H{"principal": gin.H{"type": p.Type, "id": p.ID}}
	if p.Type == auth.PrincipalUser {
		user := principalJSON(*p)
		delete(user, "type")
		out["user"] = user
	} else {
		sa, err := h.deps.repository.GetServiceAccount(c.Request.Context(), p.ID)
		if err != nil {
			internal(c)
			return
		}
		out["service_account"] = gin.H{
			"id":       sa.ID,
			"name":     sa.Name,
			"disabled": sa.Disabled,
			"owner":    gin.H{"id": sa.Owner.ID, "email": sa.Owner.Email, "name": sa.Owner.Name},
		}
	}
	c.JSON(http.StatusOK, out)
}

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
		vis = "private"
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
	d, _, err := h.deps.repository.CreateDocument(c.Request.Context(), *p, title, vis, html)
	if err != nil {
		if isBlobStorageError(err) {
			c.JSON(http.StatusBadGateway, errorResponse("blob_storage_failed", "could not upload HTML to blob storage", nil))
			return
		}
		internalErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": d.ID, "current_version_id": d.CurrentVersionID})
}

// CreateServiceAccount godoc
// @Summary Create service account
// @Tags service-accounts
// @Security bearerAuth
// @Security cookieAuth
// @Accept json
// @Produce json
// @Success 201 {object} map[string]interface{}
// @Router /v1/service-accounts [post]
func (h handlers) createServiceAccount(c *gin.Context) {
	p := current(c)
	if p.Type != auth.PrincipalUser {
		forbidden(c, "user principal required")
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.Name == "" {
		badRequest(c, "name is required")
		return
	}
	sa, err := h.deps.repository.CreateServiceAccount(c.Request.Context(), *p, in.Name)
	if err != nil {
		internal(c)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": sa.ID, "name": sa.Name, "owner": gin.H{"id": sa.Owner.ID, "email": sa.Owner.Email}, "disabled": sa.Disabled, "grants": []any{}})
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
	p := current(c)
	role, _ := h.deps.repository.RoleForDocument(c.Request.Context(), *p, c.Param("id"))
	if role != repo.RoleOwner {
		forbidden(c, "owner role required")
		return
	}
	var in struct {
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
	if principal.Type == auth.PrincipalUser && principal.ID == "" && principal.Email != "" {
		var err error
		principal, err = h.deps.repository.EnsureUserByEmail(c.Request.Context(), principal.Email)
		if err != nil {
			internal(c)
			return
		}
	} else {
		if principal.ID == "" {
			badRequest(c, "principal id or user email is required")
			return
		}
		exists, err := h.deps.repository.PrincipalExists(c.Request.Context(), principal)
		if err != nil {
			internal(c)
			return
		}
		if !exists {
			notFound(c)
			return
		}
	}
	g, err := h.deps.repository.CreateGrant(c.Request.Context(), c.Param("id"), principal, in.Role, *p)
	if err != nil {
		internal(c)
		return
	}
	pr := gin.H{"type": g.Principal.Type, "id": g.Principal.ID}
	if g.Principal.Email != "" {
		pr["email"] = g.Principal.Email
	}
	c.JSON(http.StatusCreated, gin.H{"id": g.ID, "resource": gin.H{"type": "document", "id": g.DocumentID}, "principal": pr, "role": g.Role, "granted_by": gin.H{"id": g.GrantedBy.ID, "email": g.GrantedBy.Email}})
}

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
	v, err := h.deps.repository.CreateVersion(c.Request.Context(), c.Param("id"), c.PostForm("base_version_id"), c.PostForm("change_summary"), html, *p)
	if err != nil {
		if err.Error() == "version_conflict" {
			c.JSON(http.StatusConflict, errorResponse("version_conflict", "base_version_id is stale", gin.H{"current_version_id": v.ID}))
			return
		}
		internal(c)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": v.ID, "number": v.Number, "sha256": v.SHA256})
}

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
		internal(c)
		return
	}
	c.JSON(http.StatusCreated, commentJSON(cm))
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
	token := (auth.SessionCodec{Secret: []byte(h.deps.sessionSecret)}).SignForAudience("render:"+v.ID, "render", 5*time.Minute)
	path := "/v/" + v.ID + "?token=" + url.QueryEscape(token)
	if h.deps.renderOrigin != "" {
		path = strings.TrimRight(h.deps.renderOrigin, "/") + path
	}
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
	if !ok || uid != "render:"+vid {
		c.JSON(http.StatusUnauthorized, errorResponse("unauthorized", "invalid render token", nil))
		return
	}
	b, err := h.deps.repository.GetVersionHTML(c.Request.Context(), vid)
	if err != nil {
		notFound(c)
		return
	}
	appOrigin := h.deps.appOrigin
	if appOrigin == "" {
		appOrigin = "'self'"
	}
	c.Header("Content-Security-Policy", "default-src 'none'; img-src data: https:; style-src 'unsafe-inline'; script-src 'unsafe-inline'; frame-ancestors "+appOrigin)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "text/html; charset=utf-8", renderWrapperHTML(b, appOrigin))
}

// CreateOwnershipTransfer godoc
// @Summary Initiate service account ownership transfer
// @Tags service-accounts
// @Security bearerAuth
// @Security cookieAuth
// @Param id path string true "Service account ID"
// @Success 201 {object} repo.OwnershipTransfer
// @Router /v1/service-accounts/{id}/transfer [post]
func (h handlers) createOwnershipTransfer(c *gin.Context) {
	if !h.needServiceAccountOwner(c, c.Param("id")) {
		return
	}
	var in struct {
		ToUserEmail string `json:"to_user_email"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.ToUserEmail == "" {
		badRequest(c, "to_user_email is required")
		return
	}
	x, err := h.deps.repository.CreateOwnershipTransfer(c.Request.Context(), c.Param("id"), *current(c), in.ToUserEmail)
	if err != nil {
		internal(c)
		return
	}
	c.JSON(http.StatusCreated, x)
}

// ListOwnershipTransfers godoc
// @Summary List service account ownership transfers
// @Tags service-accounts
// @Security bearerAuth
// @Security cookieAuth
// @Success 200 {object} map[string]interface{}
// @Router /v1/service-accounts/transfers [get]
func (h handlers) listOwnershipTransfers(c *gin.Context) {
	items, err := h.deps.repository.ListOwnershipTransfers(c.Request.Context(), *current(c))
	if err != nil {
		internal(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// AcceptOwnershipTransfer godoc
// @Summary Accept ownership transfer
// @Tags service-accounts
// @Security bearerAuth
// @Security cookieAuth
// @Param id path string true "Transfer ID"
// @Success 200 {object} repo.OwnershipTransfer
// @Router /v1/service-accounts/transfers/{id}/accept [post]
func (h handlers) acceptOwnershipTransfer(c *gin.Context) {
	x, err := h.deps.repository.AcceptOwnershipTransfer(c.Request.Context(), c.Param("id"), *current(c))
	if err != nil {
		forbidden(c, "not allowed")
		return
	}
	c.JSON(http.StatusOK, x)
}

// DeclineOwnershipTransfer godoc
// @Summary Decline ownership transfer
// @Tags service-accounts
// @Security bearerAuth
// @Security cookieAuth
// @Param id path string true "Transfer ID"
// @Success 204
// @Router /v1/service-accounts/transfers/{id}/decline [post]
func (h handlers) declineOwnershipTransfer(c *gin.Context) {
	if err := h.deps.repository.DeclineOwnershipTransfer(c.Request.Context(), c.Param("id"), *current(c)); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			c.JSON(http.StatusConflict, errorResponse("invalid_transfer_state", "transfer is not pending or not found", nil))
			return
		}
		internalErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
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
	_ = c.ShouldBindJSON(&in)
	if in.Visibility != "" && !validVisibility(in.Visibility) {
		badRequest(c, "invalid visibility")
		return
	}
	d, err := h.deps.repository.UpdateDocument(c.Request.Context(), c.Param("id"), in.Title, in.Visibility)
	if err != nil {
		internal(c)
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
		internal(c)
		return
	}
	c.Status(http.StatusNoContent)
}

// ListServiceAccounts godoc
// @Summary List service accounts
// @Tags service-accounts
// @Security bearerAuth
// @Security cookieAuth
// @Success 200 {object} map[string]interface{}
// @Router /v1/service-accounts [get]
func (h handlers) listServiceAccounts(c *gin.Context) {
	items, err := h.deps.repository.ListServiceAccounts(c.Request.Context(), *current(c))
	if err != nil {
		internal(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// PatchServiceAccount godoc
// @Summary Update service account
// @Tags service-accounts
// @Security bearerAuth
// @Security cookieAuth
// @Param id path string true "Service account ID"
// @Success 200 {object} repo.ServiceAccount
// @Router /v1/service-accounts/{id} [patch]
func (h handlers) patchServiceAccount(c *gin.Context) {
	if !h.needServiceAccountOwner(c, c.Param("id")) {
		return
	}
	var in struct {
		Name     string `json:"name"`
		Disabled *bool  `json:"disabled"`
	}
	_ = c.ShouldBindJSON(&in)
	sa, err := h.deps.repository.UpdateServiceAccount(c.Request.Context(), c.Param("id"), in.Name, in.Disabled)
	if err != nil {
		internal(c)
		return
	}
	c.JSON(http.StatusOK, sa)
}

// CreateServiceAccountKey godoc
// @Summary Create service account key
// @Tags service-accounts
// @Security bearerAuth
// @Security cookieAuth
// @Param id path string true "Service account ID"
// @Success 201 {object} map[string]interface{}
// @Router /v1/service-accounts/{id}/keys [post]
func (h handlers) createServiceAccountKey(c *gin.Context) {
	if !h.needServiceAccountOwner(c, c.Param("id")) {
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	_ = c.ShouldBindJSON(&in)
	if in.Name == "" {
		in.Name = "default"
	}
	token, hash, err := auth.NewBearerToken("aidocs_sa_")
	if err != nil {
		internal(c)
		return
	}
	id, err := h.deps.repository.CreateServiceAccountKey(c.Request.Context(), c.Param("id"), in.Name, hash)
	if err != nil {
		internal(c)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id, "token": token})
}

// ListServiceAccountKeys godoc
// @Summary List service account keys
// @Tags service-accounts
// @Security bearerAuth
// @Security cookieAuth
// @Param id path string true "Service account ID"
// @Success 200 {object} map[string]interface{}
// @Router /v1/service-accounts/{id}/keys [get]
func (h handlers) listServiceAccountKeys(c *gin.Context) {
	if !h.needServiceAccountOwner(c, c.Param("id")) {
		return
	}
	items, err := h.deps.repository.ListServiceAccountKeys(c.Request.Context(), c.Param("id"))
	if err != nil {
		internal(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// RevokeServiceAccountKey godoc
// @Summary Revoke service account key
// @Tags service-accounts
// @Security bearerAuth
// @Security cookieAuth
// @Param id path string true "Service account ID"
// @Param key_id path string true "Key ID"
// @Success 204
// @Router /v1/service-accounts/{id}/keys/{key_id} [delete]
func (h handlers) revokeServiceAccountKey(c *gin.Context) {
	if !h.needServiceAccountOwner(c, c.Param("id")) {
		return
	}
	if err := h.deps.repository.RevokeServiceAccountKey(c.Request.Context(), c.Param("id"), c.Param("key_id")); err != nil {
		internal(c)
		return
	}
	c.Status(http.StatusNoContent)
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
		internal(c)
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
		internal(c)
		return
	}
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
		internal(c)
		return
	}
	c.Status(http.StatusNoContent)
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
		internal(c)
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
		internal(c)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", b)
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
	items, err := h.deps.repository.ListComments(c.Request.Context(), c.Param("id"), c.Query("status"), c.Query("version_id"))
	if err != nil {
		internal(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": commentsJSON(items)})
}

// PatchComment godoc
// @Summary Update comment
// @Tags comments
// @Security bearerAuth
// @Security cookieAuth
// @Param doc_id path string true "Document ID"
// @Param comment_id path string true "Comment ID"
// @Success 200 {object} map[string]interface{}
// @Router /v1/documents/{doc_id}/comments/{comment_id} [patch]
func (h handlers) patchComment(c *gin.Context) {
	if !h.canMutateComment(c, c.Param("id"), c.Param("comment_id")) {
		return
	}
	var in struct{ Body, Status string }
	_ = c.ShouldBindJSON(&in)
	if in.Status != "" && in.Status != "open" && in.Status != "resolved" {
		badRequest(c, "invalid comment status")
		return
	}
	cm, err := h.deps.repository.UpdateComment(c.Request.Context(), c.Param("comment_id"), in.Body, in.Status)
	if err != nil {
		internal(c)
		return
	}
	c.JSON(http.StatusOK, commentJSON(cm))
}

// DeleteComment godoc
// @Summary Delete comment
// @Tags comments
// @Security bearerAuth
// @Security cookieAuth
// @Param doc_id path string true "Document ID"
// @Param comment_id path string true "Comment ID"
// @Success 204
// @Router /v1/documents/{doc_id}/comments/{comment_id} [delete]
func (h handlers) deleteComment(c *gin.Context) {
	if !h.canMutateComment(c, c.Param("id"), c.Param("comment_id")) {
		return
	}
	if err := h.deps.repository.DeleteComment(c.Request.Context(), c.Param("comment_id")); err != nil {
		internal(c)
		return
	}
	c.Status(http.StatusNoContent)
}
func (h handlers) needServiceAccountOwner(c *gin.Context, saID string) bool {
	p := current(c)
	if p.Type != auth.PrincipalUser {
		forbidden(c, "service account owner required")
		return false
	}
	sa, err := h.deps.repository.GetServiceAccount(c.Request.Context(), saID)
	if err != nil || sa.Owner.ID != p.ID {
		forbidden(c, "service account owner required")
		return false
	}
	return true
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
func (h handlers) needDocRole(c *gin.Context, docID string, need repo.Role) bool {
	role, _ := h.deps.repository.RoleForDocument(c.Request.Context(), *current(c), docID)
	if !atLeast(role, need) {
		forbidden(c, string(need)+" role required")
		return false
	}
	return true
}
func notFound(c *gin.Context) {
	c.JSON(http.StatusNotFound, errorResponse("not_found", "not found", nil))
}

func requireAuth(a auth.Authenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, err := a.Authenticate(c.Request)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, errorResponse("unauthorized", "authentication required", nil))
			return
		}
		c.Set("principal", p)
		c.Next()
	}
}
func current(c *gin.Context) *auth.Principal { p, _ := c.Get("principal"); return p.(*auth.Principal) }

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
func atLeast(have repo.Role, need repo.Role) bool {
	rank := map[repo.Role]int{repo.RoleViewer: 1, repo.RoleCommenter: 2, repo.RoleEditor: 3, repo.RoleOwner: 4}
	return rank[have] >= rank[need]
}
func safeWebRedirect(redirect string) bool {
	if redirect == "" {
		return true
	}
	if strings.HasPrefix(redirect, "/") && !strings.HasPrefix(redirect, "//") {
		return true
	}
	u, err := url.Parse(redirect)
	return err == nil && u.Scheme == "" && u.Host == ""
}
func validGrantRole(role repo.Role) bool {
	return role == repo.RoleViewer || role == repo.RoleCommenter || role == repo.RoleEditor
}
func validVisibility(v string) bool {
	return v == "private" || v == "org" || v == "link"
}
func hostMatchesOrigin(host, origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(host, u.Host)
}
func emailDomainAllowed(email string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return false
	}
	domain := strings.ToLower(email[at+1:])
	for _, d := range allowed {
		if strings.EqualFold(domain, strings.TrimSpace(d)) {
			return true
		}
	}
	return false
}
func commentsJSON(items []repo.Comment) []gin.H {
	out := make([]gin.H, 0, len(items))
	for _, cm := range items {
		out = append(out, commentJSON(cm))
	}
	return out
}
func renderWrapperHTML(userHTML []byte, appOrigin string) []byte {
	payload := strings.ReplaceAll(string(userHTML), "</script", "<\\/script")
	return []byte(`<!doctype html><html><head><meta charset="utf-8"><title>aidocs render</title><style>html,body{margin:0;height:100%;overflow:hidden}#aidocs-doc{border:0;width:100%;height:100vh}.aidocs-mark{background:#ffe66d!important;box-shadow:0 0 0 2px rgba(255,214,10,.35);border-radius:3px}.aidocs-mark-active{background:#ffb703!important}</style></head><body><iframe id="aidocs-doc" sandbox="allow-scripts allow-same-origin" srcdoc="` + htmlEscape(payload) + `"></iframe><script>window.__AIDOCS_APP_ORIGIN__=` + jsString(appOrigin) + `;(` + renderBridgeJS + `)();</script></body></html>`)
}

const renderBridgeJS = `function(){
const appOrigin=window.__AIDOCS_APP_ORIGIN__||'*';
const frame=document.getElementById('aidocs-doc');
function doc(){try{return frame.contentDocument||frame.contentWindow.document}catch(e){return null}}
function textNodes(root){const w=(doc()||document).createTreeWalker(root,NodeFilter.SHOW_TEXT,{acceptNode:n=>n.nodeValue.trim()?NodeFilter.FILTER_ACCEPT:NodeFilter.FILTER_REJECT});const a=[];let n;while(n=w.nextNode())a.push(n);return a}
function clear(){const d=doc();if(!d)return;d.querySelectorAll('mark.aidocs-mark').forEach(m=>m.replaceWith(...m.childNodes));d.body&&d.body.normalize()}
function markQuote(q,active){const d=doc();if(!d||!q)return 0;let count=0;for(const n of textNodes(d.body)){const i=n.nodeValue.indexOf(q);if(i<0)continue;const r=d.createRange();r.setStart(n,i);r.setEnd(n,i+q.length);const m=d.createElement('mark');m.className='aidocs-mark'+(active?' aidocs-mark-active':'');try{r.surroundContents(m);count++;if(active)m.scrollIntoView({block:'center',behavior:'smooth'});break}catch(e){}}
return count}
function paint(items,active){clear();(items||[]).forEach(x=>markQuote(x.quote||x.selected_text,x.id===active))}
function selection(){const d=doc();if(!d)return;const s=d.getSelection();if(!s||s.isCollapsed)return;const q=s.toString().trim();if(!q)return;let pre='',suf='',start=0,end=q.length;try{const body=d.body.innerText||d.body.textContent||'';start=body.indexOf(q);end=start+q.length;pre=body.slice(Math.max(0,start-64),start);suf=body.slice(end,end+64)}catch(e){}parent.postMessage({type:'aidocs:selection',quote:q,prefix:pre,suffix:suf,start_offset:start,end_offset:end,dom_path:'body'},appOrigin==='self'?'*':appOrigin)}
frame.addEventListener('load',()=>{const d=doc();if(!d)return;d.addEventListener('mouseup',()=>setTimeout(selection,0));d.addEventListener('keyup',()=>setTimeout(selection,0));parent.postMessage({type:'aidocs:ready'},appOrigin==='self'?'*':appOrigin)})
window.addEventListener('message',e=>{if(e.data&&e.data.type==='aidocs:paint')paint(e.data.comments,e.data.active)})
}`

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func jsString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
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

func commentJSON(cm repo.Comment) gin.H {
	return gin.H{
		"id":                    cm.ID,
		"author":                principalJSON(cm.Author),
		"body":                  cm.Body,
		"selected_text":         cm.SelectedText,
		"anchor":                cm.Anchor,
		"status":                cm.Status,
		"created_on_version_id": cm.VersionID,
		"current_placement": gin.H{
			"version_id":   cm.VersionID,
			"status":       "attached",
			"anchor":       cm.Anchor,
			"matched_text": cm.SelectedText,
			"confidence":   1.0,
		},
	}
}
func badRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, errorResponse("bad_request", msg, nil))
}
func forbidden(c *gin.Context, msg string) {
	c.JSON(http.StatusForbidden, errorResponse("forbidden", msg, nil))
}
func isBlobStorageError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "s3:") || strings.Contains(msg, "putobject") || strings.Contains(msg, "blob") || strings.Contains(msg, "bucket")
}

func internal(c *gin.Context) {
	internalErr(c, nil)
}
func internalErr(c *gin.Context, err error) {
	if err != nil {
		log.Printf("internal error method=%s path=%s error=%v", c.Request.Method, c.Request.URL.Path, err)
	} else {
		log.Printf("internal error method=%s path=%s", c.Request.Method, c.Request.URL.Path)
	}
	c.JSON(http.StatusInternalServerError, errorResponse("internal", "internal server error", nil))
}
func notImplemented(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, errorResponse("not_implemented", "endpoint scaffolded but not implemented", nil))
}
func errorResponse(code, message string, details any) gin.H {
	err := gin.H{"code": code, "message": message}
	if details != nil {
		err["details"] = details
	}
	return gin.H{"error": err}
}

var _ = errors.Is
