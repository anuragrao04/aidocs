//go:build testauth

package auth

import (
	"net/http"
	"strings"
)

// TestHeaderAuthenticator is intended for API tests only. It is compiled only
// under the `testauth` build tag so it can never be linked into a production
// binary, preventing accidental identity spoofing via the X-Test-Principal
// header. Production wiring must use a DB-backed authenticator instead.
type TestHeaderAuthenticator struct{}

func (TestHeaderAuthenticator) Authenticate(r *http.Request) (*Principal, error) {
	raw := r.Header.Get("X-Test-Principal")
	if raw == "" {
		return nil, ErrUnauthorized
	}

	parts := strings.Split(raw, ":")
	if len(parts) < 2 {
		return nil, ErrUnauthorized
	}

	p := &Principal{Type: PrincipalType(parts[0]), ID: parts[1]}
	if len(parts) >= 3 {
		p.Email = parts[2]
	}
	if len(parts) >= 4 {
		p.Name = parts[3]
	}
	return p, nil
}
