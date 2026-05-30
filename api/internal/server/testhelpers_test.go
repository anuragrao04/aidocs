package server_test

import (
	"net/http"
	"strings"

	"github.com/anuragrao/aidocs/api/internal/auth"
	"github.com/anuragrao/aidocs/api/internal/repo"
	"github.com/anuragrao/aidocs/api/internal/server"
	"golang.org/x/oauth2"
)

// testPrincipalAuth is a test-only authenticator that trusts the
// X-Test-Principal header. It is defined here (in the server test package)
// rather than in api/internal/auth so it never ships in a production binary,
// even without a build tag. The format is:
//
//	<type>:<id>[:<email>[:<name>]]
type testPrincipalAuth struct{}

func (testPrincipalAuth) Authenticate(r *http.Request) (*auth.Principal, error) {
	raw := r.Header.Get("X-Test-Principal")
	if raw == "" {
		return nil, auth.ErrUnauthorized
	}
	parts := strings.Split(raw, ":")
	if len(parts) < 2 {
		return nil, auth.ErrUnauthorized
	}
	p := &auth.Principal{Type: auth.PrincipalType(parts[0]), ID: parts[1]}
	if len(parts) >= 3 {
		p.Email = parts[2]
	}
	if len(parts) >= 4 {
		p.Name = parts[3]
	}
	return p, nil
}

func newTestServer() http.Handler {
	return newConfiguredTestServer(server.Config{Environment: "test"})
}

func newConfiguredTestServer(cfg server.Config) http.Handler {
	return server.New(cfg, server.WithRepository(repo.NewMemorySeeded()), server.WithAuthenticator(testPrincipalAuth{})).Handler()
}

func newOAuthTestServer() http.Handler {
	return server.New(server.Config{
		Environment: "test",
		AppOrigin:   "https://app.example",
		GoogleOAuth: auth.GoogleOAuth{Config: &oauth2.Config{
			ClientID:    "client-id",
			RedirectURL: "https://app.example/v1/auth/google/callback",
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://accounts.example/auth",
				TokenURL: "https://accounts.example/token",
			},
		}},
	}, server.WithAuthenticator(testPrincipalAuth{})).Handler()
}
