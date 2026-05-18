package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anuragrao/aidocs/api/internal/auth"
)

type resolver struct {
	p auth.Principal
	h string
}

func (r resolver) ResolveBearerToken(ctx context.Context, h string) (auth.Principal, error) {
	if h == r.h {
		return r.p, nil
	}
	return auth.Principal{}, auth.ErrUnauthorized
}
func (r resolver) ResolveUser(ctx context.Context, id string) (auth.Principal, error) {
	if id == r.p.ID {
		return r.p, nil
	}
	return auth.Principal{}, auth.ErrUnauthorized
}

func TestDBAuthenticatorBearerToken(t *testing.T) {
	tok, hash, err := auth.NewBearerToken("aidocs_cli_")
	if err != nil {
		t.Fatal(err)
	}
	a := auth.DBAuthenticator{Resolver: resolver{p: auth.Principal{Type: auth.PrincipalUser, ID: "usr_1"}, h: hash}}
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	p, err := a.Authenticate(req)
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "usr_1" {
		t.Fatalf("principal=%+v", p)
	}
}

func TestSessionTokenCarriesExpiryMetadata(t *testing.T) {
	tok := (auth.SessionCodec{Secret: []byte("test-secret")}).Sign("render:ver_1")
	parts := strings.Split(tok, ".")
	if len(parts) < 3 {
		t.Fatalf("signed token has %d parts, want payload with expiry plus signature", len(parts))
	}
}

func TestDBAuthenticatorSignedSession(t *testing.T) {
	secret := "test-secret"
	cookie := (auth.SessionCodec{Secret: []byte(secret)}).Sign("usr_1")
	a := auth.DBAuthenticator{Resolver: resolver{p: auth.Principal{Type: auth.PrincipalUser, ID: "usr_1"}}, SessionSecret: secret}
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "aidocs_session", Value: cookie})
	p, err := a.Authenticate(req)
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "usr_1" {
		t.Fatalf("principal=%+v", p)
	}
}
