package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// e2eMux builds the mock aidocs API used by the CLI end-to-end tests.
func e2eMux(t *testing.T) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/me", jsonH(map[string]any{"principal": map[string]any{"type": "user", "id": "usr_1"}, "user": map[string]any{"id": "usr_1", "email": "me@example.com", "name": "Me"}}))
	mux.HandleFunc("/v1/config", jsonH(map[string]any{"deployment": "public", "org_name": "", "everyone_label": "Anyone with the link"}))
	mux.HandleFunc("/v1/documents", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			json.NewEncoder(w).Encode(map[string]any{"items": []any{map[string]any{"id": "doc_1", "title": "Doc", "current_version_id": "ver_1"}}})
		case "POST":
			requireMultipart(t, r)
			json.NewEncoder(w).Encode(map[string]any{"id": "doc_new", "current_version_id": "ver_new"})
		default:
			w.WriteHeader(405)
		}
	})
	mux.HandleFunc("/v1/documents/doc_1", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"id": "doc_1", "title": "Doc", "current_version_id": "ver_1"})
	})
	mux.HandleFunc("/v1/documents/doc_1/versions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			requireMultipart(t, r)
			json.NewEncoder(w).Encode(map[string]any{"id": "ver_2", "number": 2, "sha256": "abc"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"items": []any{map[string]any{"id": "ver_1", "number": 1}}})
	})
	mux.HandleFunc("/v1/documents/doc_1/versions/ver_1", jsonH(map[string]any{"id": "ver_1", "number": 1}))
	mux.HandleFunc("/v1/documents/doc_1/versions/ver_1/html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, "<h1>ok</h1>")
	})
	mux.HandleFunc("/v1/documents/doc_1/comments", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var in map[string]any
			_ = json.NewDecoder(r.Body).Decode(&in)
			if in["version_id"] != "ver_1" {
				t.Fatalf("comment create used wrong version payload: %#v", in)
			}
			json.NewEncoder(w).Encode(map[string]any{"id": "cmt_new", "status": "open"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"items": []any{map[string]any{"id": "cmt_1", "body": "fix", "status": "open"}}})
	})
	mux.HandleFunc("/v1/documents/doc_1/comments/cmt_1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(204)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"id": "cmt_1", "status": "ok"})
	})
	mux.HandleFunc("/v1/documents/doc_1/grants", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			json.NewEncoder(w).Encode(map[string]any{"id": "gr_1"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"items": []any{map[string]any{"id": "gr_1", "role": "viewer"}}})
	})
	mux.HandleFunc("/v1/documents/doc_1/grants/gr_1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(204)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"id": "gr_1", "role": "editor"})
	})
	mux.HandleFunc("/v1/service-accounts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			json.NewEncoder(w).Encode(map[string]any{"id": "sa_1", "name": "bot"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"items": []any{map[string]any{"id": "sa_1", "name": "bot"}}})
	})
	mux.HandleFunc("/v1/service-accounts/sa_1", jsonH(map[string]any{"id": "sa_1", "name": "bot"}))
	mux.HandleFunc("/v1/service-accounts/sa_1/keys", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			json.NewEncoder(w).Encode(map[string]any{"id": "sak_1", "token": "aidocs_sa_x"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"items": []any{map[string]any{"id": "sak_1", "name": "k"}}})
	})
	mux.HandleFunc("/v1/service-accounts/sa_1/keys/sak_1", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	mux.HandleFunc("/v1/service-accounts/sa_1/transfer", func(w http.ResponseWriter, r *http.Request) {
		var in map[string]any
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in["to_user_email"] != "new@example.com" {
			t.Fatalf("transfer used wrong payload: %#v", in)
		}
		json.NewEncoder(w).Encode(map[string]any{"id": "xfer_1", "status": "pending"})
	})
	mux.HandleFunc("/v1/service-accounts/transfers", jsonH(map[string]any{"items": []any{map[string]any{"id": "xfer_1"}}}))
	mux.HandleFunc("/v1/service-accounts/transfers/xfer_1/accept", jsonH(map[string]any{"id": "xfer_1", "status": "accepted"}))
	mux.HandleFunc("/v1/service-accounts/transfers/xfer_1/decline", jsonH(map[string]any{"id": "xfer_1", "status": "declined"}))
	return mux
}

func TestCLICommandsE2E(t *testing.T) {
	srv := httptest.NewServer(e2eMux(t))
	defer srv.Close()
	t.Setenv("AIDOCS_TOKEN", "tok")
	t.Setenv("AIDOCS_SERVER", srv.URL)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AIDOCS_NO_BROWSER", "1")
	html := filepath.Join(t.TempDir(), "x.html")
	os.WriteFile(html, []byte("<h1>x</h1>"), 0644)
	out := filepath.Join(t.TempDir(), "out.html")
	cmds := [][]string{{"auth", "whoami"}, {"docs", "list"}, {"docs", "create", html}, {"docs", "show", "doc_1"}, {"docs", "update", "doc_1", "--title", "New"}, {"docs", "versions", "list", "doc_1"}, {"docs", "versions", "show", "doc_1", "ver_1"}, {"docs", "versions", "html", "doc_1", "ver_1"}, {"docs", "pull", "doc_1", "--out", out}, {"docs", "push", "doc_1", html}, {"docs", "comments", "list", "doc_1"}, {"docs", "comments", "create", "doc_1", "--body", "hi", "--quote", "x", "--version", "ver_1"}, {"docs", "comments", "update", "doc_1", "cmt_1", "--status", "resolved"}, {"docs", "comments", "delete", "doc_1", "cmt_1"}, {"docs", "comments", "resolve", "doc_1", "cmt_1"}, {"docs", "comments", "reopen", "doc_1", "cmt_1"}, {"docs", "grants", "list", "doc_1"}, {"docs", "grants", "add", "doc_1", "--principal", "user:a@b.com", "--role", "viewer"}, {"docs", "grants", "update", "doc_1", "gr_1", "--role", "editor"}, {"docs", "grants", "revoke", "doc_1", "gr_1"}, {"sa", "list"}, {"sa", "create", "bot"}, {"sa", "update", "sa_1", "--disable"}, {"sa", "key", "list", "sa_1"}, {"sa", "key", "create", "sa_1"}, {"sa", "key", "revoke", "sa_1", "sak_1"}, {"sa", "transfer", "sa_1", "--to", "new@example.com"}, {"sa", "transfers", "list"}, {"sa", "transfer", "accept", "xfer_1"}, {"sa", "transfer", "decline", "xfer_1"}, {"context", "list"}, {"context", "use", srv.URL}, {"open", "doc_1"}}
	for _, c := range cmds {
		t.Run(strings.Join(c, " "), func(t *testing.T) {
			if _, err := Execute(c); err != nil {
				t.Fatalf("%v", err)
			}
		})
	}
	if b, _ := os.ReadFile(out); string(b) != "<h1>ok</h1>" {
		t.Fatalf("pull wrote %q", b)
	}
}

// TestCommandOutputsAreConversational locks in the action-indicative output
// of mutating commands so it can't silently regress to raw API rows. The
// default (non-JSON) output must tell the caller what just happened.
func TestCommandOutputsAreConversational(t *testing.T) {
	srv := httptest.NewServer(e2eMux(t))
	defer srv.Close()
	t.Setenv("AIDOCS_TOKEN", "tok")
	t.Setenv("AIDOCS_SERVER", srv.URL)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AIDOCS_NO_BROWSER", "1")
	html := filepath.Join(t.TempDir(), "x.html")
	os.WriteFile(html, []byte("<h1>x</h1>"), 0644)
	out := filepath.Join(t.TempDir(), "out.html")

	cases := []struct {
		args []string
		want []string
	}{
		{[]string{"auth", "whoami"}, []string{"me@example.com", "server=" + srv.URL}},
		{[]string{"docs", "create", html}, []string{"Created document", "doc_new"}},
		{[]string{"docs", "update", "doc_1", "--title", "New"}, []string{"Renamed document", "doc_1"}},
		{[]string{"docs", "pull", "doc_1", "--out", out}, []string{"Pulled version", "doc_1"}},
		{[]string{"docs", "push", "doc_1", html}, []string{"Pushed version", "ver_2"}},
		{[]string{"docs", "comments", "create", "doc_1", "--body", "hi", "--quote", "x", "--version", "ver_1"}, []string{"Added comment"}},
		{[]string{"docs", "comments", "delete", "doc_1", "cmt_1"}, []string{"Deleted comment", "cmt_1"}},
		{[]string{"docs", "comments", "resolve", "doc_1", "cmt_1"}, []string{"Resolved comment", "cmt_1"}},
		{[]string{"docs", "comments", "reopen", "doc_1", "cmt_1"}, []string{"Reopened comment", "cmt_1"}},
		{[]string{"docs", "grants", "add", "doc_1", "--to", "a@b.com", "--role", "viewer"}, []string{"Shared", "a@b.com", "viewer"}},
		{[]string{"docs", "grants", "add", "doc_1", "--everyone", "--role", "viewer"}, []string{"Shared", "viewer"}},
		{[]string{"docs", "grants", "revoke", "doc_1", "gr_1"}, []string{"Revoked grant", "gr_1"}},
		{[]string{"sa", "update", "sa_1", "--disable"}, []string{"Disabled service account", "sa_1"}},
		{[]string{"sa", "key", "create", "sa_1"}, []string{"Created key", "Copy this key"}},
		{[]string{"sa", "key", "revoke", "sa_1", "sak_1"}, []string{"Revoked key", "sak_1"}},
		{[]string{"sa", "transfer", "sa_1", "--to", "new@example.com"}, []string{"Requested transfer", "new@example.com"}},
		{[]string{"sa", "transfer", "accept", "xfer_1"}, []string{"Accepted transfer", "xfer_1"}},
		{[]string{"sa", "transfer", "decline", "xfer_1"}, []string{"Declined transfer", "xfer_1"}},
		{[]string{"open", "doc_1"}, []string{"doc_1"}},
		{[]string{"context", "use", srv.URL}, []string{"Switched to context"}},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			got, err := Execute(tc.args)
			if err != nil {
				t.Fatalf("%v", err)
			}
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Fatalf("output %q does not contain %q", got, w)
				}
			}
		})
	}
}

// TestJSONOutputStaysMachineReadable ensures --json keeps returning the raw
// API payload (not the conversational line) for mutating commands.
func TestJSONOutputStaysMachineReadable(t *testing.T) {
	srv := httptest.NewServer(e2eMux(t))
	defer srv.Close()
	t.Setenv("AIDOCS_TOKEN", "tok")
	t.Setenv("AIDOCS_SERVER", srv.URL)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	got, err := Execute([]string{"--json", "docs", "update", "doc_1", "--title", "New"})
	if err != nil {
		t.Fatalf("%v", err)
	}
	if strings.Contains(got, "\u2713") || strings.Contains(got, "Renamed document") {
		t.Fatalf("--json output should be the raw payload, got %q", got)
	}
	var m map[string]any
	if json.Unmarshal([]byte(got), &m) != nil {
		t.Fatalf("--json output is not valid JSON: %q", got)
	}
}

func TestLogoutRevokeCallsServer(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && r.URL.Path == "/v1/auth/cli/credentials/cred_1" {
			called = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfg := Config{ActiveContext: "example", Contexts: map[string]*Context{"example": {Server: srv.URL, Credential: map[string]any{"id": "cred_1", "token": "tok"}, Pulled: map[string]string{}}}}
	if err := saveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := Execute([]string{"auth", "logout"}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("expected server revoke call")
	}
}

func TestNormalizeServerUsesHTTPForLocalhost(t *testing.T) {
	cases := map[string]string{
		"localhost:8080":        "http://localhost:8080",
		"127.0.0.1:8080":        "http://127.0.0.1:8080",
		"aidocs.example.com":    "https://aidocs.example.com",
		"http://localhost:8080": "http://localhost:8080",
	}
	for in, want := range cases {
		if got := normalizeServer(in); got != want {
			t.Fatalf("normalizeServer(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExitCodeMapping(t *testing.T) {
	cases := []struct{ status, want int }{{401, 3}, {403, 3}, {404, 4}, {409, 5}, {500, 1}}
	for _, tc := range cases {
		if got := ExitCode(&APIError{Status: tc.status, Message: "x"}); got != tc.want {
			t.Fatalf("status %d got %d want %d", tc.status, got, tc.want)
		}
	}
	if got := ExitCode(usageError{fmt.Errorf("unknown flag: --wat")}); got != 2 {
		t.Fatalf("usage error got %d", got)
	}
	if got := ExitCode(asUsageError(fmt.Errorf(`unknown command "wat" for "aidocs"`))); got != 2 {
		t.Fatalf("unknown command got %d", got)
	}
}

func TestSACommandsCanBeDisabled(t *testing.T) {
	t.Setenv("AIDOCS_DISABLE_SA_COMMANDS", "1")
	_, err := Execute([]string{"sa", "list"})
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("expected disabled err, got %v", err)
	}
}

func jsonH(v any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(v)
	}
}
func requireMultipart(t *testing.T, r *http.Request) {
	t.Helper()
	if _, ok := r.Body.(*multipart.Part); ok {
		fmt.Println()
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		t.Fatalf("multipart: %v", err)
	}
}
