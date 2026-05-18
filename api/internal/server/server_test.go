package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/anuragrao/aidocs/api/internal/auth"
	"github.com/anuragrao/aidocs/api/internal/repo"
	"github.com/anuragrao/aidocs/api/internal/server"
	"golang.org/x/oauth2"
)

func TestDiscovery(t *testing.T) {
	rr := do(t, newTestServer(), http.MethodGet, "/.well-known/aidocs.json", "", nil)
	assertStatus(t, rr, http.StatusOK)
	assertJSON(t, rr.Body.Bytes(), `{
	  "name": "aidocs",
	  "api_version": "v1",
	  "auth": { "modes": ["web", "cli"] }
	}`)
}

func TestAPIDocsRoutes(t *testing.T) {
	h := newTestServer()

	openapi := do(t, h, http.MethodGet, "/openapi.json", "", nil)
	assertStatus(t, openapi, http.StatusOK)
	var spec map[string]any
	if err := json.Unmarshal(openapi.Body.Bytes(), &spec); err != nil {
		t.Fatalf("openapi response is not JSON: %v", err)
	}
	if spec["swagger"] != "2.0" {
		t.Fatalf("swagger version = %v, want 2.0", spec["swagger"])
	}
	paths, ok := spec["paths"].(map[string]any)
	if !ok || paths["/v1/documents"] == nil || paths["/v1/service-accounts"] == nil {
		t.Fatalf("openapi paths missing expected endpoints: %#v", spec["paths"])
	}

	docs := do(t, h, http.MethodGet, "/api-docs", "", nil)
	assertStatus(t, docs, http.StatusFound)
	if loc := docs.Header().Get("Location"); loc != "/api-docs/index.html" {
		t.Fatalf("Location = %q, want /api-docs/index.html", loc)
	}
}

func TestMeRequiresAuth(t *testing.T) {
	rr := do(t, newTestServer(), http.MethodGet, "/v1/me", "", nil)
	assertStatus(t, rr, http.StatusUnauthorized)
	assertJSON(t, rr.Body.Bytes(), `{
	  "error": { "code": "unauthorized", "message": "authentication required" }
	}`)
}

func TestMeWithUserPrincipal(t *testing.T) {
	rr := do(t, newTestServer(), http.MethodGet, "/v1/me", "", map[string]string{
		"X-Test-Principal": "user:usr_1:anurag@example.com:Anurag",
	})
	assertStatus(t, rr, http.StatusOK)
	assertJSON(t, rr.Body.Bytes(), `{
	  "principal": { "type": "user", "id": "usr_1" },
	  "user": { "id": "usr_1", "email": "anurag@example.com", "name": "Anurag" }
	}`)
}

func TestMeWithServiceAccountPrincipal(t *testing.T) {
	rr := do(t, newTestServer(), http.MethodGet, "/v1/me", "", map[string]string{
		"X-Test-Principal": "service_account:sa_1",
	})
	assertStatus(t, rr, http.StatusOK)
	assertJSON(t, rr.Body.Bytes(), `{
	  "principal": { "type": "service_account", "id": "sa_1" },
	  "service_account": {
	    "id": "sa_1",
	    "name": "fixture",
	    "disabled": false,
	    "owner": { "id": "owner_1", "email": "owner@example.com", "name": "Owner" }
	  }
	}`)
}

func TestCreateDocument(t *testing.T) {
	body, contentType := multipartBody(t, map[string]string{
		"title":      "Q3 business review",
		"visibility": "private",
	}, "file", "review.html", "<html><body>Hello</body></html>")

	rr := doRaw(t, newTestServer(), http.MethodPost, "/v1/documents", body, map[string]string{
		"Content-Type":     contentType,
		"X-Test-Principal": "user:usr_1:anurag@example.com:Anurag",
	})

	assertStatus(t, rr, http.StatusCreated)
	assertJSON(t, rr.Body.Bytes(), `{
	  "id": "doc_1",
	  "current_version_id": "ver_1"
	}`)
}

func TestServiceAccountCannotCreateDocument(t *testing.T) {
	body, contentType := multipartBody(t, map[string]string{"title": "Bot doc"}, "file", "bot.html", "<html></html>")
	rr := doRaw(t, newTestServer(), http.MethodPost, "/v1/documents", body, map[string]string{
		"Content-Type":     contentType,
		"X-Test-Principal": "service_account:sa_1",
	})

	assertStatus(t, rr, http.StatusForbidden)
	assertJSON(t, rr.Body.Bytes(), `{
	  "error": { "code": "forbidden", "message": "service accounts cannot own documents" }
	}`)
}

func TestCreateServiceAccountStartsWithoutDocumentGrants(t *testing.T) {
	rr := do(t, newTestServer(), http.MethodPost, "/v1/service-accounts", `{"name":"razorpay-report-bot"}`, map[string]string{
		"Content-Type":     "application/json",
		"X-Test-Principal": "user:usr_1:owner@example.com:Owner",
	})

	assertStatus(t, rr, http.StatusCreated)
	assertJSON(t, rr.Body.Bytes(), `{
	  "id": "sa_1",
	  "name": "razorpay-report-bot",
	  "owner": { "id": "usr_1", "email": "owner@example.com" },
	  "disabled": false,
	  "grants": []
	}`)
}

func TestGrantDocumentAccessToServiceAccount(t *testing.T) {
	rr := do(t, newTestServer(), http.MethodPost, "/v1/documents/doc_1/grants", `{
	  "principal": { "type": "service_account", "id": "sa_1" },
	  "role": "editor"
	}`, map[string]string{
		"Content-Type":     "application/json",
		"X-Test-Principal": "user:owner_1:owner@example.com:Owner",
	})

	assertStatus(t, rr, http.StatusCreated)
	assertJSON(t, rr.Body.Bytes(), `{
	  "id": "gr_1",
	  "resource": { "type": "document", "id": "doc_1" },
	  "principal": { "type": "service_account", "id": "sa_1" },
	  "role": "editor",
	  "granted_by": { "id": "owner_1", "email": "owner@example.com" }
	}`)
}

func TestGrantDocumentAccessToUserByEmail(t *testing.T) {
	rr := do(t, newTestServer(), http.MethodPost, "/v1/documents/doc_1/grants", `{
	  "principal": { "type": "user", "email": "reviewer@example.com" },
	  "role": "commenter"
	}`, map[string]string{
		"Content-Type":     "application/json",
		"X-Test-Principal": "user:owner_1:owner@example.com:Owner",
	})

	assertStatus(t, rr, http.StatusCreated)
	assertJSON(t, rr.Body.Bytes(), `{
	  "id": "gr_1",
	  "resource": { "type": "document", "id": "doc_1" },
	  "principal": { "type": "user", "id": "usr_2", "email": "reviewer@example.com" },
	  "role": "commenter",
	  "granted_by": { "id": "owner_1", "email": "owner@example.com" }
	}`)
}

func TestViewerCannotUploadVersion(t *testing.T) {
	body, contentType := multipartBody(t, map[string]string{
		"base_version_id": "ver_1",
	}, "file", "review.html", "<html><body>Updated</body></html>")

	rr := doRaw(t, newTestServer(), http.MethodPost, "/v1/documents/doc_1/versions", body, map[string]string{
		"Content-Type":     contentType,
		"X-Test-Principal": "user:viewer_1:viewer@example.com:Viewer",
	})

	assertStatus(t, rr, http.StatusForbidden)
	assertJSON(t, rr.Body.Bytes(), `{
	  "error": { "code": "forbidden", "message": "editor role required" }
	}`)
}

func TestVersionConflict(t *testing.T) {
	body, contentType := multipartBody(t, map[string]string{
		"base_version_id": "ver_old",
		"change_summary":  "addressed comments",
	}, "file", "review.html", "<html><body>Updated</body></html>")

	rr := doRaw(t, newTestServer(), http.MethodPost, "/v1/documents/doc_1/versions", body, map[string]string{
		"Content-Type":     contentType,
		"X-Test-Principal": "user:editor_1:editor@example.com:Editor",
	})

	assertStatus(t, rr, http.StatusConflict)
	assertJSON(t, rr.Body.Bytes(), `{
	  "error": {
	    "code": "version_conflict",
	    "message": "base_version_id is stale",
	    "details": { "current_version_id": "ver_1" }
	  }
	}`)
}

func TestCreateComment(t *testing.T) {
	rr := do(t, newTestServer(), http.MethodPost, "/v1/documents/doc_1/comments", `{
	  "version_id": "ver_1",
	  "body": "Add a number here.",
	  "anchor": {
	    "quote": "higher payment success rates",
	    "prefix": "increased due to ",
	    "suffix": " and improved checkout latency",
	    "dom_path": "main/section[2]/p[1]",
	    "start_offset": 24,
	    "end_offset": 53
	  }
	}`, map[string]string{
		"Content-Type":     "application/json",
		"X-Test-Principal": "user:commenter_1:commenter@example.com:Commenter",
	})

	assertStatus(t, rr, http.StatusCreated)
	assertJSON(t, rr.Body.Bytes(), `{
	  "id": "cmt_1",
	  "author": { "type": "user", "id": "commenter_1", "email": "commenter@example.com", "name": "Commenter" },
	  "body": "Add a number here.",
	  "selected_text": "higher payment success rates",
	  "anchor": {
	    "quote": "higher payment success rates",
	    "prefix": "increased due to ",
	    "suffix": " and improved checkout latency",
	    "dom_path": "main/section[2]/p[1]",
	    "start_offset": 24,
	    "end_offset": 53
	  },
	  "status": "open",
	  "created_on_version_id": "ver_1",
	  "current_placement": {
	    "version_id": "ver_1",
	    "status": "attached",
	    "anchor": {
	      "quote": "higher payment success rates",
	      "prefix": "increased due to ",
	      "suffix": " and improved checkout latency",
	      "dom_path": "main/section[2]/p[1]",
	      "start_offset": 24,
	      "end_offset": 53
	    },
	    "matched_text": "higher payment success rates",
	    "confidence": 1
	  }
	}`)
}

func TestNonOwnerCannotCreateServiceAccountKey(t *testing.T) {
	rr := do(t, newTestServer(), http.MethodPost, "/v1/service-accounts/sa_999/keys", `{"name":"stolen"}`, map[string]string{
		"Content-Type":     "application/json",
		"X-Test-Principal": "user:attacker_1:attacker@example.com:Attacker",
	})

	assertStatus(t, rr, http.StatusForbidden)
}

func TestNonOwnerCannotListServiceAccountKeys(t *testing.T) {
	rr := do(t, newTestServer(), http.MethodGet, "/v1/service-accounts/sa_999/keys", "", map[string]string{
		"X-Test-Principal": "user:attacker_1:attacker@example.com:Attacker",
	})

	assertStatus(t, rr, http.StatusForbidden)
}

func TestNonOwnerCannotRevokeServiceAccountKey(t *testing.T) {
	rr := do(t, newTestServer(), http.MethodDelete, "/v1/service-accounts/sa_999/keys/sak_1", "", map[string]string{
		"X-Test-Principal": "user:attacker_1:attacker@example.com:Attacker",
	})

	assertStatus(t, rr, http.StatusForbidden)
}

func TestNonOwnerCannotPatchServiceAccount(t *testing.T) {
	rr := do(t, newTestServer(), http.MethodPatch, "/v1/service-accounts/sa_999", `{"disabled":true}`, map[string]string{
		"Content-Type":     "application/json",
		"X-Test-Principal": "user:attacker_1:attacker@example.com:Attacker",
	})

	assertStatus(t, rr, http.StatusForbidden)
}

func TestNonOwnerCannotTransferServiceAccount(t *testing.T) {
	rr := do(t, newTestServer(), http.MethodPost, "/v1/service-accounts/sa_999/transfer", `{"to_user_email":"attacker@example.com"}`, map[string]string{
		"Content-Type":     "application/json",
		"X-Test-Principal": "user:attacker_1:attacker@example.com:Attacker",
	})

	assertStatus(t, rr, http.StatusForbidden)
}

func TestRandomUserCannotPatchAnotherUsersComment(t *testing.T) {
	rr := do(t, newTestServer(), http.MethodPatch, "/v1/documents/doc_1/comments/cmt_1", `{"body":"hacked"}`, map[string]string{
		"Content-Type":     "application/json",
		"X-Test-Principal": "user:attacker_1:attacker@example.com:Attacker",
	})

	assertStatus(t, rr, http.StatusForbidden)
}

func TestRandomUserCannotDeleteAnotherUsersComment(t *testing.T) {
	rr := do(t, newTestServer(), http.MethodDelete, "/v1/documents/doc_1/comments/cmt_1", "", map[string]string{
		"X-Test-Principal": "user:attacker_1:attacker@example.com:Attacker",
	})

	assertStatus(t, rr, http.StatusForbidden)
}

func TestPatchGrantRejectsOwnerRole(t *testing.T) {
	h := newTestServer()
	_ = do(t, h, http.MethodPost, "/v1/documents/doc_1/grants", `{
	  "principal": { "type": "service_account", "id": "sa_1" },
	  "role": "viewer"
	}`, map[string]string{
		"Content-Type":     "application/json",
		"X-Test-Principal": "user:owner_1:owner@example.com:Owner",
	})

	rr := do(t, h, http.MethodPatch, "/v1/documents/doc_1/grants/gr_1", `{"role":"owner"}`, map[string]string{
		"Content-Type":     "application/json",
		"X-Test-Principal": "user:owner_1:owner@example.com:Owner",
	})

	assertStatus(t, rr, http.StatusBadRequest)
}

func TestGrantRejectsNonexistentServiceAccount(t *testing.T) {
	rr := do(t, newTestServer(), http.MethodPost, "/v1/documents/doc_1/grants", `{
	  "principal": { "type": "service_account", "id": "sa_missing" },
	  "role": "viewer"
	}`, map[string]string{
		"Content-Type":     "application/json",
		"X-Test-Principal": "user:owner_1:owner@example.com:Owner",
	})

	assertStatus(t, rr, http.StatusNotFound)
}

func TestOAuthStartRejectsExternalWebRedirect(t *testing.T) {
	rr := do(t, newOAuthTestServer(), http.MethodGet, "/v1/auth/google/start?mode=web&redirect=https://evil.example/callback", "", nil)

	assertStatus(t, rr, http.StatusBadRequest)
}

func TestRenderCSPAllowsConfiguredAppOrigin(t *testing.T) {
	h := newConfiguredTestServer(server.Config{Environment: "test", AppOrigin: "https://app.example", SessionSecret: "test-secret"})
	body, contentType := multipartBody(t, map[string]string{"title": "Doc"}, "file", "doc.html", "<html><body>x</body></html>")
	_ = doRaw(t, h, http.MethodPost, "/v1/documents", body, map[string]string{
		"Content-Type":     contentType,
		"X-Test-Principal": "user:owner_1:owner@example.com:Owner",
	})

	tokRR := do(t, h, http.MethodPost, "/v1/versions/ver_1/render-token", "", map[string]string{
		"X-Test-Principal": "user:owner_1:owner@example.com:Owner",
	})
	assertStatus(t, tokRR, http.StatusOK)
	var tok struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(tokRR.Body.Bytes(), &tok); err != nil {
		t.Fatal(err)
	}

	rr := do(t, h, http.MethodGet, "/v/ver_1?token="+tok.Token, "", nil)
	assertStatus(t, rr, http.StatusOK)
	if got := rr.Header().Get("Content-Security-Policy"); !strings.Contains(got, "frame-ancestors https://app.example") {
		t.Fatalf("CSP = %q, want frame-ancestors for configured app origin", got)
	}
}

func TestRenderRouteRequiresRenderHostWhenConfigured(t *testing.T) {
	h := newConfiguredTestServer(server.Config{Environment: "test", AppOrigin: "https://app.example", RenderOrigin: "https://doc.example", SessionSecret: "test-secret"})
	token := (auth.SessionCodec{Secret: []byte("test-secret")}).SignForAudience("render:ver_1", "render", 5*time.Minute)
	req := httptest.NewRequest(http.MethodGet, "/v/ver_1?token="+token, nil)
	req.Host = "app.example"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	assertStatus(t, rr, http.StatusNotFound)
}

func TestFrontendFallbackServesEmbeddedIndex(t *testing.T) {
	rr := do(t, newTestServer(), http.MethodGet, "/docs/doc_1", "", nil)
	assertStatus(t, rr, http.StatusOK)
	if !strings.Contains(rr.Body.String(), "<title>aidocs</title>") || !strings.Contains(rr.Body.String(), "id=\"root\"") {
		t.Fatalf("body = %q, want embedded frontend", rr.Body.String())
	}
}

func TestCreateDocumentRejectsInvalidVisibility(t *testing.T) {
	body, contentType := multipartBody(t, map[string]string{"title": "Doc", "visibility": "public-internet"}, "file", "doc.html", "<html></html>")
	rr := doRaw(t, newTestServer(), http.MethodPost, "/v1/documents", body, map[string]string{
		"Content-Type":     contentType,
		"X-Test-Principal": "user:usr_1:anurag@example.com:Anurag",
	})
	assertStatus(t, rr, http.StatusBadRequest)
}

func TestGrantPatchIsScopedToDocument(t *testing.T) {
	h := newTestServer()
	_ = do(t, h, http.MethodPost, "/v1/documents/doc_1/grants", `{
	  "principal": { "type": "service_account", "id": "sa_1" },
	  "role": "viewer"
	}`, map[string]string{
		"Content-Type":     "application/json",
		"X-Test-Principal": "user:owner_1:owner@example.com:Owner",
	})

	rr := do(t, h, http.MethodPatch, "/v1/documents/doc_2/grants/gr_1", `{"role":"editor"}`, map[string]string{
		"Content-Type":     "application/json",
		"X-Test-Principal": "user:owner_1:owner@example.com:Owner",
	})
	assertStatus(t, rr, http.StatusNotFound)
}

func TestUploadRejectsPayloadOver10MiB(t *testing.T) {
	largeHTML := "<html>" + strings.Repeat("x", 10*1024*1024+1) + "</html>"
	body, contentType := multipartBody(t, map[string]string{"title": "Too large"}, "file", "large.html", largeHTML)

	rr := doRaw(t, newTestServer(), http.MethodPost, "/v1/documents", body, map[string]string{
		"Content-Type":     contentType,
		"X-Test-Principal": "user:usr_1:anurag@example.com:Anurag",
	})

	assertStatus(t, rr, http.StatusRequestEntityTooLarge)
}

func TestViewerCannotCreateComment(t *testing.T) {
	rr := do(t, newTestServer(), http.MethodPost, "/v1/documents/doc_1/comments", `{
	  "version_id": "ver_1",
	  "body": "should not be allowed",
	  "anchor": { "quote": "hello" }
	}`, map[string]string{
		"Content-Type":     "application/json",
		"X-Test-Principal": "user:viewer_1:viewer@example.com:Viewer",
	})

	assertStatus(t, rr, http.StatusForbidden)
}

func TestCreateCommentRejectsVersionFromAnotherDocument(t *testing.T) {
	h := newTestServer()
	body, contentType := multipartBody(t, map[string]string{"title": "Other"}, "file", "other.html", "<html>other</html>")
	_ = doRaw(t, h, http.MethodPost, "/v1/documents", body, map[string]string{
		"Content-Type":     contentType,
		"X-Test-Principal": "user:owner_1:owner@example.com:Owner",
	})

	rr := do(t, h, http.MethodPost, "/v1/documents/doc_1/comments", `{
	  "version_id": "ver_2",
	  "body": "wrong doc",
	  "anchor": { "quote": "hello" }
	}`, map[string]string{
		"Content-Type":     "application/json",
		"X-Test-Principal": "user:commenter_1:commenter@example.com:Commenter",
	})

	assertStatus(t, rr, http.StatusNotFound)
}

func TestPatchCommentRejectsWrongDocument(t *testing.T) {
	rr := do(t, newTestServer(), http.MethodPatch, "/v1/documents/doc_wrong/comments/cmt_1", `{"status":"resolved"}`, map[string]string{
		"Content-Type":     "application/json",
		"X-Test-Principal": "user:commenter_1:commenter@example.com:Commenter",
	})

	assertStatus(t, rr, http.StatusNotFound)
}

func TestPatchCommentRejectsInvalidStatus(t *testing.T) {
	rr := do(t, newTestServer(), http.MethodPatch, "/v1/documents/doc_1/comments/cmt_1", `{"status":"closed"}`, map[string]string{
		"Content-Type":     "application/json",
		"X-Test-Principal": "user:commenter_1:commenter@example.com:Commenter",
	})

	assertStatus(t, rr, http.StatusBadRequest)
}

func TestAcceptDeclinedOwnershipTransferIsRejected(t *testing.T) {
	h := newTestServer()
	create := do(t, h, http.MethodPost, "/v1/service-accounts/sa_1/transfer", `{"to_user_email":"new_owner@example.com"}`, map[string]string{
		"Content-Type":     "application/json",
		"X-Test-Principal": "user:owner_1:owner@example.com:Owner",
	})
	assertStatus(t, create, http.StatusCreated)

	_ = do(t, h, http.MethodPost, "/v1/service-accounts/transfers/xfer_1/decline", "", map[string]string{
		"X-Test-Principal": "user:new_owner@example.com:new_owner@example.com:New Owner",
	})

	rr := do(t, h, http.MethodPost, "/v1/service-accounts/transfers/xfer_1/accept", "", map[string]string{
		"X-Test-Principal": "user:new_owner@example.com:new_owner@example.com:New Owner",
	})

	assertStatus(t, rr, http.StatusForbidden)
}

func TestRenderRouteReturnsWrapperNotRawHTML(t *testing.T) {
	h := newConfiguredTestServer(server.Config{Environment: "test", AppOrigin: "https://app.example", SessionSecret: "test-secret"})
	body, contentType := multipartBody(t, map[string]string{"title": "Doc"}, "file", "doc.html", "<html><body>x</body></html>")
	_ = doRaw(t, h, http.MethodPost, "/v1/documents", body, map[string]string{
		"Content-Type":     contentType,
		"X-Test-Principal": "user:owner_1:owner@example.com:Owner",
	})
	tokRR := do(t, h, http.MethodPost, "/v1/versions/ver_1/render-token", "", map[string]string{
		"X-Test-Principal": "user:owner_1:owner@example.com:Owner",
	})
	var tok struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(tokRR.Body.Bytes(), &tok); err != nil {
		t.Fatal(err)
	}

	rr := do(t, h, http.MethodGet, "/v/ver_1?token="+tok.Token, "", nil)
	assertStatus(t, rr, http.StatusOK)
	if !strings.Contains(rr.Body.String(), `<iframe id="aidocs-doc"`) || !strings.Contains(rr.Body.String(), "&lt;html&gt;&lt;/html&gt;") {
		t.Fatalf("body = %q, want render wrapper with escaped srcdoc", rr.Body.String())
	}
}

func TestServiceAccountWithoutGrantCannotReadDocument(t *testing.T) {
	rr := do(t, newTestServer(), http.MethodGet, "/v1/documents/doc_1", "", map[string]string{
		"X-Test-Principal": "service_account:sa_1",
	})

	assertStatus(t, rr, http.StatusForbidden)
}

func TestMeWithMissingServiceAccountReturnsInternalError(t *testing.T) {
	rr := do(t, newTestServer(), http.MethodGet, "/v1/me", "", map[string]string{
		"X-Test-Principal": "service_account:sa_missing",
	})

	assertStatus(t, rr, http.StatusInternalServerError)
	assertJSON(t, rr.Body.Bytes(), `{
	  "error": { "code": "internal", "message": "internal server error" }
	}`)
}

func TestHandlersPassRequestContextToRepository(t *testing.T) {
	repo := &contextCheckingRepo{Memory: repo.NewMemory()}
	h := server.New(server.Config{Environment: "test"}, server.WithRepository(repo)).Handler()
	ctx := context.WithValue(context.Background(), contextMarkerKey{}, "seen")
	req := httptest.NewRequest(http.MethodGet, "/v1/documents", nil).WithContext(ctx)
	req.Header.Set("X-Test-Principal", "user:owner_1:owner@example.com:Owner")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assertStatus(t, rr, http.StatusOK)
	if !repo.sawMarker {
		t.Fatal("repository did not receive request context")
	}
}

type contextMarkerKey struct{}

type contextCheckingRepo struct {
	*repo.Memory
	sawMarker bool
}

func (r *contextCheckingRepo) ListDocuments(ctx context.Context, p auth.Principal) ([]repo.Document, error) {
	if ctx.Value(contextMarkerKey{}) == "seen" {
		r.sawMarker = true
	}
	return r.Memory.ListDocuments(ctx, p)
}

func newTestServer() http.Handler {
	return newConfiguredTestServer(server.Config{Environment: "test"})
}

func newConfiguredTestServer(cfg server.Config) http.Handler {
	return server.New(cfg).Handler()
}

func newOAuthTestServer() http.Handler {
	return server.New(server.Config{
		Environment: "test",
		AppOrigin:   "https://app.example",
		GoogleOAuth: auth.GoogleOAuth{Config: &oauth2.Config{
			ClientID:    "client-id",
			RedirectURL: "https://app.example/v1/auth/google/callback",
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://accounts.example/auth",
				TokenURL: "https://accounts.example/token",
			},
		}},
	}).Handler()
}

func do(t *testing.T, h http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	return doRaw(t, h, method, path, strings.NewReader(body), headers)
}

func doRaw(t *testing.T, h http.Handler, method, path string, body io.Reader, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func multipartBody(t *testing.T, fields map[string]string, fileField, fileName, fileBody string) (*bytes.Buffer, string) {
	t.Helper()
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	fw, err := w.CreateFormFile(fileField, fileName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte(fileBody)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return &b, w.FormDataContentType()
}

func assertStatus(t *testing.T, rr *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rr.Code != want {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, want, rr.Body.String())
	}
}

func assertJSON(t *testing.T, got []byte, want string) {
	t.Helper()
	var gotAny any
	if err := json.Unmarshal(got, &gotAny); err != nil {
		t.Fatalf("response is not JSON: %v; body=%s", err, string(got))
	}
	var wantAny any
	if err := json.Unmarshal([]byte(want), &wantAny); err != nil {
		t.Fatalf("bad test JSON: %v", err)
	}
	gotNorm, _ := json.Marshal(gotAny)
	wantNorm, _ := json.Marshal(wantAny)
	if string(gotNorm) != string(wantNorm) {
		t.Fatalf("json mismatch\n got: %s\nwant: %s", gotNorm, wantNorm)
	}
}
