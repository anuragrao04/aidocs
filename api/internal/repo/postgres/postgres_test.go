package postgres_test

import (
	"context"
	"os"
	"testing"

	"github.com/anuragrao/aidocs/api/internal/auth"
	"github.com/anuragrao/aidocs/api/internal/blob"
	"github.com/anuragrao/aidocs/api/internal/db"
	"github.com/anuragrao/aidocs/api/internal/repo"
	pgrepo "github.com/anuragrao/aidocs/api/internal/repo/postgres"
)

func TestPostgresRepositoryDocumentGrantVersionCommentFlow(t *testing.T) {
	url := os.Getenv("AIDOCS_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set AIDOCS_TEST_DATABASE_URL to run postgres integration tests")
	}
	if err := db.Migrate(url); err != nil {
		t.Fatal(err)
	}
	store, err := pgrepo.Connect(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	owner := auth.Principal{Type: auth.PrincipalUser, ID: "usr_pg_owner", Email: "pg-owner@example.com", Name: "Owner"}
	doc, ver, err := store.CreateDocument(context.Background(), owner, "Postgres doc", "private", []byte("<html>v1</html>"))
	if err != nil {
		t.Fatal(err)
	}
	if doc.CurrentVersionID != ver.ID {
		t.Fatalf("current version mismatch")
	}

	role, err := store.RoleForDocument(context.Background(), owner, doc.ID)
	if err != nil || role != repo.RoleOwner {
		t.Fatalf("role=%s err=%v", role, err)
	}

	sa, err := store.CreateServiceAccount(context.Background(), owner, "pg-bot")
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.CreateGrant(context.Background(), doc.ID, auth.Principal{Type: auth.PrincipalServiceAccount, ID: sa.ID}, repo.RoleEditor, owner)
	if err != nil {
		t.Fatal(err)
	}
	role, err = store.RoleForDocument(context.Background(), auth.Principal{Type: auth.PrincipalServiceAccount, ID: sa.ID}, doc.ID)
	if err != nil || role != repo.RoleEditor {
		t.Fatalf("sa role=%s err=%v", role, err)
	}

	_, err = store.CreateVersion(context.Background(), doc.ID, "stale", "bad", []byte("<html>bad</html>"), owner)
	if err == nil || err.Error() != "version_conflict" {
		t.Fatalf("want version_conflict, got %v", err)
	}

	ver2, err := store.CreateVersion(context.Background(), doc.ID, ver.ID, "updated", []byte("<html>v2</html>"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if ver2.Number != 2 {
		t.Fatalf("number=%d", ver2.Number)
	}

	commenter := auth.Principal{Type: auth.PrincipalUser, ID: "usr_pg_commenter", Email: "c@example.com", Name: "Commenter"}
	cm, err := store.CreateComment(context.Background(), doc.ID, ver2.ID, "Looks good", repo.Anchor{Quote: "v2", Prefix: "<html>", Suffix: "</html>", DOMPath: "body", StartOffset: 0, EndOffset: 2}, commenter)
	if err != nil {
		t.Fatal(err)
	}
	if cm.SelectedText != "v2" || cm.Status != "open" {
		t.Fatalf("bad comment: %+v", cm)
	}
}

func TestPostgresCreateDocumentDeletesBlobWhenDBFailsAfterPut(t *testing.T) {
	url := os.Getenv("AIDOCS_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set AIDOCS_TEST_DATABASE_URL to run postgres integration tests")
	}
	if err := db.Migrate(url); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	blobs := &cancelAfterPutBlob{cancel: cancel}
	store, err := pgrepo.ConnectWithBlob(context.Background(), url, blobs)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	owner := auth.Principal{Type: auth.PrincipalUser, ID: "usr_blob_cleanup_doc", Email: "blob-cleanup-doc@example.com", Name: "Owner"}
	_, _, err = store.CreateDocument(ctx, owner, "cleanup doc", "private", []byte("<html>cleanup</html>"))
	if err == nil {
		t.Fatal("CreateDocument succeeded; want context cancellation after blob put")
	}
	if blobs.puts != 1 || blobs.deletes != 1 {
		t.Fatalf("blob puts=%d deletes=%d, want 1/1", blobs.puts, blobs.deletes)
	}
	if blobs.deletedKey != blobs.putKey {
		t.Fatalf("deleted key %q, want put key %q", blobs.deletedKey, blobs.putKey)
	}
}

func TestPostgresCreateVersionDeletesBlobWhenDBFailsAfterPut(t *testing.T) {
	url := os.Getenv("AIDOCS_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set AIDOCS_TEST_DATABASE_URL to run postgres integration tests")
	}
	if err := db.Migrate(url); err != nil {
		t.Fatal(err)
	}
	blobs := &cancelAfterPutBlob{}
	store, err := pgrepo.ConnectWithBlob(context.Background(), url, blobs)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	owner := auth.Principal{Type: auth.PrincipalUser, ID: "usr_blob_cleanup_ver", Email: "blob-cleanup-ver@example.com", Name: "Owner"}
	doc, ver, err := store.CreateDocument(context.Background(), owner, "cleanup version", "private", []byte("<html>v1</html>"))
	if err != nil {
		t.Fatal(err)
	}
	blobs.puts, blobs.deletes, blobs.putKey, blobs.deletedKey = 0, 0, "", ""
	ctx, cancel := context.WithCancel(context.Background())
	blobs.cancel = cancel
	_, err = store.CreateVersion(ctx, doc.ID, ver.ID, "will fail", []byte("<html>v2</html>"), owner)
	if err == nil {
		t.Fatal("CreateVersion succeeded; want context cancellation after blob put")
	}
	if blobs.puts != 1 || blobs.deletes != 1 {
		t.Fatalf("blob puts=%d deletes=%d, want 1/1", blobs.puts, blobs.deletes)
	}
	if blobs.deletedKey != blobs.putKey {
		t.Fatalf("deleted key %q, want put key %q", blobs.deletedKey, blobs.putKey)
	}
}

type cancelAfterPutBlob struct {
	cancel     context.CancelFunc
	puts       int
	deletes    int
	putKey     string
	deletedKey string
}

func (b *cancelAfterPutBlob) Put(ctx context.Context, key, contentType string, body []byte) error {
	b.puts++
	b.putKey = key
	if b.cancel != nil {
		b.cancel()
	}
	return nil
}
func (b *cancelAfterPutBlob) Get(ctx context.Context, key string) ([]byte, string, error) {
	return nil, "", blob.ErrNotFound
}
func (b *cancelAfterPutBlob) Delete(ctx context.Context, key string) error {
	b.deletes++
	b.deletedKey = key
	return nil
}
