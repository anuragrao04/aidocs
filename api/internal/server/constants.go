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

// Comment placement statuses relative to the rendered version.
const (
	placementAttached = "attached"
	placementOrphaned = "orphaned"
)

// Render audience prefix used in render tokens.
const renderAudiencePrefix = "render:"

// googleUserIDPrefix prefixes user IDs minted from a Google subject.
const googleUserIDPrefix = "usr_google_"

// Environment values for Config.Environment.
const (
	EnvProduction = "production"
	envTest       = "test"
)

// Retry bounds for allocating a unique service-account name.
const (
	saNameMaxAttempts         = 8
	saNameShortDomainAttempts = 5
)
