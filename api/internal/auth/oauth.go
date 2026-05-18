package auth

import (
	"context"
	"errors"
	"net/url"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/idtoken"
)

type GoogleUser struct{ Sub, Email, Name, PictureURL string }

type GoogleOAuth struct {
	Config   *oauth2.Config
	ClientID string
}

func NewGoogleOAuth(clientID, clientSecret, redirectURL string) GoogleOAuth {
	return GoogleOAuth{ClientID: clientID, Config: &oauth2.Config{ClientID: clientID, ClientSecret: clientSecret, RedirectURL: redirectURL, Scopes: []string{"openid", "email", "profile"}, Endpoint: google.Endpoint}}
}

func (g GoogleOAuth) AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string {
	return g.Config.AuthCodeURL(state, opts...)
}
func (g GoogleOAuth) ExchangeAndVerify(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (GoogleUser, error) {
	tok, err := g.Config.Exchange(ctx, code, opts...)
	if err != nil {
		return GoogleUser{}, err
	}
	raw, ok := tok.Extra("id_token").(string)
	if !ok || raw == "" {
		return GoogleUser{}, errors.New("missing id_token")
	}
	payload, err := idtoken.Validate(ctx, raw, g.ClientID)
	if err != nil {
		return GoogleUser{}, err
	}
	email, _ := payload.Claims["email"].(string)
	name, _ := payload.Claims["name"].(string)
	picture, _ := payload.Claims["picture"].(string)
	return GoogleUser{Sub: payload.Subject, Email: email, Name: name, PictureURL: picture}, nil
}

func LoopbackRedirectAllowed(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost")
}
