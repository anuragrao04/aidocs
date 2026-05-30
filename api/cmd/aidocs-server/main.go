package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/anuragrao/aidocs/api/internal/auth"
	"github.com/anuragrao/aidocs/api/internal/blob"
	"github.com/anuragrao/aidocs/api/internal/db"
	pgrepo "github.com/anuragrao/aidocs/api/internal/repo/postgres"
	"github.com/anuragrao/aidocs/api/internal/server"
)

// @title aidocs API
// @version v1
// @description This API powers a review tool where people can upload one self-contained HTML file, view it, and leave comments anchored to specific text ranges inside that HTML.
// @BasePath /
// @securityDefinitions.apikey cookieAuth
// @in cookie
// @name aidocs_session
// @securityDefinitions.apikey bearerAuth
// @in header
// @name Authorization
func main() {
	addr := os.Getenv("AIDOCS_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	metricsPort := os.Getenv("AIDOCS_METRICS_PORT")
	if metricsPort == "" {
		metricsPort = "9090"
	}
	metricsAddr := ":" + metricsPort

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	if os.Getenv("AIDOCS_MIGRATE") != "false" {
		if err := db.Migrate(databaseURL); err != nil {
			log.Fatal(err)
		}
	}
	blobs, err := newBlobStore(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	store, err := pgrepo.ConnectWithBlob(context.Background(), databaseURL, blobs)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	appOrigin := os.Getenv("APP_ORIGIN")
	if appOrigin == "" {
		log.Fatal("APP_ORIGIN is required")
	}
	if err := validateOrigin("APP_ORIGIN", appOrigin); err != nil {
		log.Fatal(err)
	}
	clientID := os.Getenv("GOOGLE_OAUTH_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		log.Fatal("GOOGLE_OAUTH_CLIENT_ID and GOOGLE_OAUTH_CLIENT_SECRET are required")
	}
	sessionSecret := os.Getenv("SESSION_SECRET")
	if len(sessionSecret) < 32 {
		log.Fatal("SESSION_SECRET must be at least 32 bytes")
	}

	renderOrigin := os.Getenv("RENDER_ORIGIN")
	if renderOrigin == "" {
		log.Fatal("RENDER_ORIGIN is required")
	}
	if err := validateOrigin("RENDER_ORIGIN", renderOrigin); err != nil {
		log.Fatal(err)
	}

	// Deployment type. An org deployment gates Google login to the org's
	// domains; those domains double as the OAuth allow-list. A public
	// deployment lets any Google account in (subject to ALLOWED_OAUTH_DOMAINS).
	deployment := os.Getenv("AIDOCS_DEPLOYMENT")
	if deployment == "" {
		deployment = server.DeploymentPublic
	}
	orgDomains := splitCSV(os.Getenv("AIDOCS_ORG_DOMAINS"))
	allowedOAuthDomains := splitCSV(os.Getenv("ALLOWED_OAUTH_DOMAINS"))
	orgName := os.Getenv("AIDOCS_ORG_NAME")
	switch deployment {
	case server.DeploymentPublic:
	case server.DeploymentOrg:
		if len(orgDomains) == 0 {
			log.Fatal("AIDOCS_ORG_DOMAINS is required when AIDOCS_DEPLOYMENT=org")
		}
		// On an org deployment the org's domains are the login gate.
		allowedOAuthDomains = orgDomains
	default:
		log.Fatalf("AIDOCS_DEPLOYMENT must be %q or %q (got %q)", server.DeploymentPublic, server.DeploymentOrg, deployment)
	}

	srv := server.New(server.Config{
		Environment:         server.EnvProduction,
		AppOrigin:           appOrigin,
		RenderOrigin:        renderOrigin,
		GoogleOAuth:         auth.NewGoogleOAuth(clientID, clientSecret, appOrigin+"/v1/auth/google/callback"),
		SessionSecret:       sessionSecret,
		AllowedOAuthDomains: allowedOAuthDomains,
		Deployment:          deployment,
		OrgName:             orgName,
	},
		server.WithRepository(store),
		server.WithAuthenticator(auth.DBAuthenticator{Resolver: store, SessionSecret: sessionSecret}),
		server.WithStateStore(auth.NewPostgresStateStore(store.Pool())),
	)

	// Serve Prometheus metrics on a separate listener so /metrics is kept off
	// the public app port (expose only `addr` publicly; scrape metricsAddr over
	// private networking).
	go func() {
		log.Printf("metrics listening on %s", metricsAddr)
		if err := http.ListenAndServe(metricsAddr, server.MetricsHandler()); err != nil {
			log.Fatalf("metrics server: %v", err)
		}
	}()

	if err := srv.Run(addr); err != nil {
		log.Fatal(err)
	}
}

func validateOrigin(name, value string) error {
	if strings.HasSuffix(value, "/") {
		return fmt.Errorf("%s must not end with a trailing slash (got %q). Set it to something like https://example.com", name, value)
	}
	u, err := url.Parse(value)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("%s must be an absolute URL with scheme http:// or https:// (got %q). Set it to something like https://example.com", name, value)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%s must use http:// or https:// (got scheme %q in %q)", name, u.Scheme, value)
	}
	if u.Path != "" && u.Path != "/" {
		return fmt.Errorf("%s must not include a path (got %q). Set it to something like https://example.com", name, value)
	}
	return nil
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func newBlobStore(ctx context.Context) (blob.Store, error) {
	bucket := os.Getenv("BLOB_BUCKET")
	if bucket == "" {
		return nil, fmt.Errorf("BLOB_BUCKET is required")
	}
	region := os.Getenv("BLOB_REGION")
	if region == "" {
		region = os.Getenv("AWS_REGION")
	}
	if region == "" {
		region = "us-east-1"
	}
	return blob.NewS3(ctx, blob.S3Config{Bucket: bucket, Region: region, Endpoint: os.Getenv("BLOB_ENDPOINT"), AccessKeyID: os.Getenv("BLOB_ACCESS_KEY_ID"), SecretAccessKey: os.Getenv("BLOB_SECRET_ACCESS_KEY"), ForcePathStyle: os.Getenv("BLOB_FORCE_PATH_STYLE") == "true"})
}
