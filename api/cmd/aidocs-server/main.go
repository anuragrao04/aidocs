package main

import (
	"context"
	"fmt"
	"log"
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

	srv := server.New(server.Config{
		Environment:         "production",
		AppOrigin:           appOrigin,
		RenderOrigin:        renderOrigin,
		GoogleOAuth:         auth.NewGoogleOAuth(clientID, clientSecret, appOrigin+"/v1/auth/google/callback"),
		SessionSecret:       sessionSecret,
		AllowedOAuthDomains: splitCSV(os.Getenv("ALLOWED_OAUTH_DOMAINS")),
	},
		server.WithRepository(store),
		server.WithAuthenticator(auth.DBAuthenticator{Resolver: store, SessionSecret: sessionSecret}),
		server.WithStateStore(auth.NewPostgresStateStore(store.Pool())),
	)

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
		log.Fatal("BLOB_BUCKET is required")
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
