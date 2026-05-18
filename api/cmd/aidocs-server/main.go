package main

import (
	"context"
	"log"
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
