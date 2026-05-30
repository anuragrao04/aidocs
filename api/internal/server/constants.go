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

// Deployment types. A public deployment lets anyone with a Google account in;
// an org deployment gates login to the org's domains. The only authz
// difference is the login gate — the document ACL is identical.
const (
	DeploymentPublic = "public"
	DeploymentOrg    = "org"
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

// labelUnknown is the metric label value used when a dimension is not
// meaningful for an event (e.g. a comment's status after it is deleted).
const labelUnknown = "unknown"

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
