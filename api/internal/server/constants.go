package server

import "time"

// Cookie / session
const (
	sessionCookieName = "aidocs_session"
	sessionTTLSeconds = int(30 * 24 * time.Hour / time.Second) // 30 days
)

// Auth modes
const (
	authModeWeb = "web"
	authModeCLI = "cli"
)

// Document visibility values
const (
	visibilityPrivate = "private"
	visibilityOrg     = "org"
	visibilityLink    = "link"
)

// Comment / transfer statuses
const (
	commentStatusOpen     = "open"
	commentStatusResolved = "resolved"
)

// Render audience prefix used in render tokens.
const renderAudiencePrefix = "render:"

// Environment values
const (
	envProduction = "production"
	envTest       = "test"
)

// SA name-allocation retry parameters (server-09).
const (
	saNameMaxAttempts         = 8
	saNameShortDomainAttempts = 5
)
