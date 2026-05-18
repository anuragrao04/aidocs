package auth

import (
	"context"
	"net/http"
)

type TokenResolver interface {
	ResolveBearerToken(ctx context.Context, tokenHash string) (Principal, error)
}
type UserResolver interface {
	ResolveUser(ctx context.Context, id string) (Principal, error)
}

type DBAuthenticator struct {
	Resolver interface {
		TokenResolver
		UserResolver
	}
	SessionSecret string
}

func (a DBAuthenticator) Authenticate(r *http.Request) (*Principal, error) {
	if token := BearerFromHeader(r.Header.Get("Authorization")); token != "" {
		p, err := a.Resolver.ResolveBearerToken(r.Context(), HashToken(token))
		if err != nil {
			return nil, ErrUnauthorized
		}
		return &p, nil
	}
	if a.SessionSecret != "" {
		if ck, err := r.Cookie("aidocs_session"); err == nil {
			uid, ok := (SessionCodec{Secret: []byte(a.SessionSecret)}).VerifySession(ck.Value)
			if ok {
				p, err := a.Resolver.ResolveUser(r.Context(), uid)
				if err == nil {
					return &p, nil
				}
			}
		}
	}
	return nil, ErrUnauthorized
}
