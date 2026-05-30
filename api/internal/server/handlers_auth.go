package server

import (
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"

	"github.com/anuragrao/aidocs/api/internal/auth"
)

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
	mode := c.DefaultQuery("mode", authModeWeb)
	state := c.Query("state")
	if state == "" {
		var err error
		state, err = auth.RandomURLToken()
		if err != nil {
			internalErr(c, err)
			return
		}
	}
	st := auth.LoginState{Mode: mode, Redirect: c.Query("redirect"), CLIRedirect: c.Query("cli_redirect"), CodeChallenge: c.Query("code_challenge"), ExpiresAt: time.Now().Add(10 * time.Minute)}
	if mode == authModeWeb && !safeWebRedirect(st.Redirect) {
		badRequest(c, "invalid redirect")
		return
	}
	if mode == authModeCLI {
		if st.CodeChallenge == "" || !auth.LoopbackRedirectAllowed(st.CLIRedirect) {
			badRequest(c, "invalid cli oauth parameters")
			return
		}
	}
	if err := h.deps.stateStore.PutState(c.Request.Context(), state, st); err != nil {
		internalErr(c, err)
		return
	}
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
	st, ok, err := h.deps.stateStore.TakeState(c.Request.Context(), c.Query("state"))
	if err != nil {
		internalErr(c, err)
		return
	}
	if !ok {
		badRequest(c, "invalid oauth state")
		return
	}
	gu, err := h.deps.googleOAuth.ExchangeAndVerify(c.Request.Context(), c.Query("code"))
	if err != nil {
		incAuth("google", "failure")
		c.JSON(http.StatusUnauthorized, errorResponse("unauthorized", "google oauth failed", nil))
		return
	}
	if !emailDomainAllowed(gu.Email, h.deps.allowedOAuthDomains) {
		incAuth("google", "forbidden_domain")
		c.JSON(http.StatusForbidden, errorResponse("forbidden", "google account domain is not allowed", nil))
		return
	}
	uid := googleUserIDPrefix + gu.Sub
	p, err := h.deps.repository.UpsertGoogleUser(c.Request.Context(), uid, gu.Email, gu.Name, gu.Sub, gu.PictureURL)
	if err != nil {
		internalErr(c, err)
		return
	}
	if st.Mode == authModeCLI {
		code, err := auth.RandomURLToken()
		if err != nil {
			internalErr(c, err)
			return
		}
		st.GoogleUser = gu
		st.UserID = p.ID
		if err := h.deps.stateStore.PutCode(c.Request.Context(), code, st); err != nil {
			internalErr(c, err)
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
	incAuth("google", "success")
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
	st, ok, err := h.deps.stateStore.TakeCode(c.Request.Context(), in.Code)
	if err != nil {
		internalErr(c, err)
		return
	}
	if !ok || !auth.VerifyPKCES256(in.CodeVerifier, st.CodeChallenge) {
		incAuth("cli_exchange", "failure")
		c.JSON(http.StatusUnauthorized, errorResponse("unauthorized", "invalid cli code", nil))
		return
	}
	token, hash, err := auth.NewBearerToken("aidocs_cli_")
	if err != nil {
		internalErr(c, err)
		return
	}
	name := in.Name
	if name == "" {
		name = "cli"
	}
	id, err := h.deps.repository.CreateCLICredential(c.Request.Context(), st.UserID, name, hash)
	if err != nil {
		internalErr(c, err)
		return
	}
	incAuth("cli_exchange", "success")
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
		internalErr(c, err)
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
		internalErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h handlers) setSessionCookie(c *gin.Context, p auth.Principal) {
	cookie := (auth.SessionCodec{Secret: []byte(h.deps.sessionSecret)}).Sign(p.ID)
	// The Secure flag is set only in production; local http logins need it off.
	secure := h.deps.appOrigin != "" && strings.HasPrefix(h.deps.appOrigin, "https://")
	c.SetCookie(sessionCookieName, cookie, sessionTTLSeconds, "/", "", secure, true)
}

// Discovery godoc
// @Summary Deployment discovery
// @Tags discovery
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /.well-known/aidocs.json [get]
func (h handlers) discovery(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"name": "aidocs", "api_version": "v1", "auth": gin.H{"modes": []string{authModeWeb, authModeCLI}}})
}

// everyoneLabel is the human label for the "anyone" grant on this deployment.
// Public servers grant anyone with the link; org servers grant the org's
// members (scoped by the login gate, not the network).
func everyoneLabel(deployment, orgName string) string {
	if deployment == DeploymentOrg {
		if orgName != "" {
			return "Anyone in " + orgName
		}
		return "Anyone in the org"
	}
	return "Anyone with the link"
}

// Config godoc
// @Summary Deployment configuration
// @Tags discovery
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /v1/config [get]
func (h handlers) config(c *gin.Context) {
	deployment := h.deps.deployment
	if deployment == "" {
		deployment = DeploymentPublic
	}
	c.JSON(http.StatusOK, gin.H{
		"deployment":     deployment,
		"org_name":       h.deps.orgName,
		"everyone_label": everyoneLabel(deployment, h.deps.orgName),
	})
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
		out["user"] = userJSON(*p)
	} else {
		sa, err := h.deps.repository.GetServiceAccount(c.Request.Context(), p.ID)
		if err != nil {
			internalErr(c, err)
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
