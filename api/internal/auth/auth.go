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
