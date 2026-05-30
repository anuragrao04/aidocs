package auth

import (
	"errors"
	"net/http"
)

type PrincipalType string

const (
	PrincipalUser           PrincipalType = "user"
	PrincipalServiceAccount PrincipalType = "service_account"
)

type Principal struct {
	Type       PrincipalType
	ID         string
	Email      string
	Name       string
	PictureURL string
}

type Authenticator interface {
	Authenticate(r *http.Request) (*Principal, error)
}

var ErrUnauthorized = errors.New("unauthorized")

// ErrNotFound is the canonical "principal/record not found" sentinel. The repo
// layer's repo.ErrNotFound wraps this so that lower-level packages (e.g. this
// auth package, which must not import repo to avoid an import cycle) can still
// detect not-found results via errors.Is.
var ErrNotFound = errors.New("not found")
