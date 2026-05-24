package internal

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
)

type globals struct {
	server, token        string
	json, quiet, verbose bool
}
type Config struct {
	ActiveContext string              `json:"active_context"`
	Contexts      map[string]*Context `json:"contexts"`
}
type Context struct {
	Server     string            `json:"server"`
	Credential map[string]any    `json:"credential,omitempty"`
	DefaultDoc *string           `json:"default_doc"`
	Pulled     map[string]string `json:"pulled"`
}

func Execute(args []string) (string, error) {
	var out bytes.Buffer
	root := NewRoot(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}
func NewRoot(out io.Writer) *cobra.Command {
	g := &globals{}
	root := &cobra.Command{Use: "aidocs", Short: "CLI for aidocs HTML document review", Long: "CLI for aidocs HTML document review.\n\nDefault output is compact, agent-friendly key=value text. Use --json for machine-readable JSON.", SilenceUsage: true, SilenceErrors: true}
	root.SetOut(out)
	root.SetErr(os.Stderr)
	root.PersistentFlags().StringVar(&g.server, "server", "", "API server URL")
	root.PersistentFlags().StringVar(&g.token, "token", "", "Bearer token")
	root.PersistentFlags().BoolVar(&g.json, "json", false, "JSON output")
	root.PersistentFlags().BoolVarP(&g.quiet, "quiet", "q", false, "quiet")
	root.PersistentFlags().BoolVarP(&g.verbose, "verbose", "v", false, "verbose")
	root.AddCommand(authCmd(g, out), contextCmd(g, out), docsCmd(g, out), versionsCmd(g, out), saRoot(g, out), openCmd(g, out))
	return root
}

func client(g *globals) (*Client, error) {
	srv := first(g.server, os.Getenv("AIDOCS_SERVER"))
	tok := first(g.token, os.Getenv("AIDOCS_TOKEN"))
	cfg, _ := loadConfig()
	if srv == "" && cfg.ActiveContext != "" {
		if c := cfg.Contexts[cfg.ActiveContext]; c != nil {
			srv = c.Server
			if tok == "" {
				tok = credentialToken(ctxName(srv), c.Credential)
			}
		}
	}
	if srv == "" {
		if tok == "" {
			return nil, errors.New("not logged in; run aidocs auth login [server]")
		}
		srv = "https://aidocs.anuragrao.dev"
	}
	return &Client{Base: normalizeServer(srv), Token: tok, HTTP: http.DefaultClient}, nil
}

type Client struct {
	Base, Token string
	HTTP        *http.Client
}

type APIError struct {
	Status  int
	Code    string
	Message string
	Body    string
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("api %d %s: %s", e.Status, e.Code, e.Message)
	}
	return fmt.Sprintf("api %d: %s", e.Status, e.Message)
}

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	msg := err.Error()
	if strings.Contains(msg, "unknown command") || strings.Contains(msg, "unknown flag") || strings.Contains(msg, "requires") || strings.Contains(msg, "accepts") || strings.Contains(msg, "required") {
		return 2
	}
	if strings.Contains(msg, "disabled by AIDOCS_DISABLE_SA_COMMANDS") {
		return 2
	}
	var ae *APIError
	if errors.As(err, &ae) {
		switch ae.Status {
		case http.StatusUnauthorized, http.StatusForbidden:
			return 3
		case http.StatusNotFound:
			return 4
		case http.StatusConflict:
			return 5
		}
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		return 6
	}
	return 1
}

func mustClient(g *globals) (*Client, error) { return client(g) }

func (c *Client) do(method, path string, body io.Reader, ct string) ([]byte, error) {
	req, err := http.NewRequest(method, c.Base+path, body)
	if err != nil {
		return nil, err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		ae := &APIError{Status: res.StatusCode, Message: strings.TrimSpace(string(b)), Body: string(b)}
		var payload struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(b, &payload) == nil && payload.Error.Message != "" {
			ae.Code = payload.Error.Code
			ae.Message = payload.Error.Message
		}
		return nil, ae
	}
	return b, nil
}
func (c *Client) json(method, path string, in any) ([]byte, error) {
	var r io.Reader
	if in != nil {
		b, _ := json.Marshal(in)
		r = bytes.NewReader(b)
	}
	return c.do(method, path, r, "application/json")
}
func (c *Client) multipart(path string, fields map[string]string, fileField, fileName string, data []byte) ([]byte, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	fw, _ := mw.CreateFormFile(fileField, fileName)
	fw.Write(data)
	mw.Close()
	return c.do("POST", path, &buf, mw.FormDataContentType())
}

func print(out io.Writer, g *globals, b []byte) error {
	return printWithHint(out, g, b, "")
}

func printWithHint(out io.Writer, g *globals, b []byte, hint string) error {
	if g.quiet {
		return nil
	}
	var x any
	if json.Unmarshal(b, &x) == nil {
		if g.json {
			enc := json.NewEncoder(out)
			enc.SetIndent("", "  ")
			return enc.Encode(x)
		}
		fmt.Fprintln(out, humanize(hint, x))
		return nil
	}
	_, err := out.Write(append(bytes.TrimSpace(b), '\n'))
	return err
}

func printPushedVersion(out io.Writer, g *globals, docID, server string, b []byte) error {
	if g.quiet {
		return nil
	}
	if g.json {
		return print(out, g, b)
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return print(out, g, b)
	}
	m["document_id"] = docID
	m["url"] = strings.TrimRight(server, "/") + "/documents/" + docID
	fmt.Fprintln(out, compactRow(m))
	return nil
}

func humanize(hint string, x any) string {
	m, _ := x.(map[string]any)
	if p, ok := m["principal"].(map[string]any); ok {
		if u, ok := m["user"].(map[string]any); ok {
			return compactRow(mergeMaps(p, u))
		}
		return compactRow(p)
	}
	if ctxs, ok := m["contexts"].(map[string]any); ok {
		active, _ := m["active_context"].(string)
		if len(ctxs) == 0 {
			return "No contexts."
		}
		var lines []string
		for name, raw := range ctxs {
			cx, _ := raw.(map[string]any)
			prefix := " "
			if name == active {
				prefix = "*"
			}
			lines = append(lines, strings.TrimSpace(prefix+" "+name+"  server="+fmt.Sprint(value(cx, "server"))))
		}
		return strings.Join(lines, "\n")
	}
	if items, ok := value(m, "items").([]any); ok {
		if len(items) == 0 {
			return "No items."
		}
		var lines []string
		for _, it := range items {
			row, _ := it.(map[string]any)
			lines = append(lines, compactRow(row))
		}
		return strings.Join(lines, "\n")
	}
	if hint != "" {
		return hint + ": " + compactRow(m)
	}
	return compactRow(m)
}

func mergeMaps(a, b map[string]any) map[string]any {
	r := map[string]any{}
	for k, v := range a {
		r[k] = v
	}
	for k, v := range b {
		r[k] = v
	}
	return r
}
func value(m map[string]any, key string) any {
	if m == nil {
		return nil
	}
	if v, ok := m[key]; ok {
		return v
	}
	if key == "id" {
		if v, ok := m["ID"]; ok {
			return v
		}
	}
	if strings.HasSuffix(key, "_id") {
		camelID := strings.TrimSuffix(key, "_id") + "_ID"
		parts := strings.Split(camelID, "_")
		for i, p := range parts {
			if p != "" {
				parts[i] = strings.ToUpper(p[:1]) + p[1:]
			}
		}
		if v, ok := m[strings.Join(parts, "")]; ok {
			return v
		}
	}
	parts := strings.Split(key, "_")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return m[strings.Join(parts, "")]
}

func compactRow(m map[string]any) string {
	if m == nil {
		return "OK"
	}
	parts := []string{}
	for _, k := range []string{"id", "document_id", "title", "name", "email", "type", "role", "status", "disabled", "current_version_id", "number", "sha256", "token", "selected_text", "body", "url"} {
		if v := value(m, k); v != nil && fmt.Sprint(v) != "" {
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
	}
	if owner, ok := value(m, "owner").(map[string]any); ok {
		if email := value(owner, "email"); email != nil && fmt.Sprint(email) != "" {
			parts = append(parts, "owner="+fmt.Sprint(email))
		}
	}
	if len(parts) == 0 {
		return fmt.Sprintf("%v", m)
	}
	return strings.Join(parts, "  ")
}
func first(v ...string) string {
	for _, x := range v {
		if x != "" {
			return x
		}
	}
	return ""
}
func normalizeServer(s string) string {
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		host := s
		if i := strings.Index(host, "/"); i >= 0 {
			host = host[:i]
		}
		if strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.0.0.1") || strings.HasPrefix(host, "[::1]") {
			s = "http://" + s
		} else {
			s = "https://" + s
		}
	}
	return strings.TrimRight(s, "/")
}
func ctxName(s string) string {
	u, err := url.Parse(normalizeServer(s))
	if err == nil && u.Host != "" {
		return u.Host
	}
	return s
}

func configPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "aidocs", "config.json")
}
func loadConfig() (Config, error) {
	c := Config{Contexts: map[string]*Context{}}
	b, err := os.ReadFile(configPath())
	if err != nil {
		return c, nil
	}
	err = json.Unmarshal(b, &c)
	if c.Contexts == nil {
		c.Contexts = map[string]*Context{}
	}
	return c, err
}
func saveConfig(c Config) error {
	p := configPath()
	os.MkdirAll(filepath.Dir(p), 0700)
	b, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(p, b, 0600)
}

const keychainService = "aidocs"

func keychainDisabled() bool             { return os.Getenv("AIDOCS_NO_KEYCHAIN") != "" }
func tokenRef(contextName string) string { return contextName + ":token" }

func credentialToken(contextName string, cred map[string]any) string {
	if cred == nil {
		return ""
	}
	if t, ok := cred["token"].(string); ok && t != "" {
		return t
	}
	ref, _ := cred["token_ref"].(string)
	if ref == "" {
		ref = tokenRef(contextName)
	}
	if keychainDisabled() {
		return ""
	}
	t, err := keyring.Get(keychainService, ref)
	if err != nil {
		return ""
	}
	return t
}

func storeCredentialToken(contextName string, cred map[string]any) map[string]any {
	if cred == nil {
		return cred
	}
	tok, _ := cred["token"].(string)
	if tok == "" {
		return cred
	}
	if keychainDisabled() {
		return cred
	}
	ref := tokenRef(contextName)
	if err := keyring.Set(keychainService, ref, tok); err != nil {
		return cred
	}
	delete(cred, "token")
	cred["token_ref"] = ref
	return cred
}

func deleteCredentialToken(contextName string, cred map[string]any) {
	if keychainDisabled() {
		return
	}
	ref := tokenRef(contextName)
	if cred != nil {
		if r, ok := cred["token_ref"].(string); ok && r != "" {
			ref = r
		}
	}
	_ = keyring.Delete(keychainService, ref)
}

func currentContext(g *globals) (string, *Context, Config) {
	cfg, _ := loadConfig()
	srv := first(g.server, os.Getenv("AIDOCS_SERVER"), cfg.ActiveContext, "https://aidocs.anuragrao.dev")
	name := ctxName(srv)
	cx := cfg.Contexts[name]
	if cx == nil {
		cx = &Context{Server: normalizeServer(srv), Pulled: map[string]string{}}
	}
	if cx.Pulled == nil {
		cx.Pulled = map[string]string{}
	}
	return name, cx, cfg
}

func loginCmd(g *globals, out io.Writer) *cobra.Command {
	var name string
	cmd := &cobra.Command{Use: "login [server]", Short: "Login with Google through an aidocs server", Example: "  aidocs auth login\n  aidocs auth login aidocs.razorpay.com", RunE: func(cmd *cobra.Command, args []string) error {
		srv := "https://aidocs.anuragrao.dev"
		if len(args) > 0 {
			srv = args[0]
		}
		srv = normalizeServer(srv)
		verifier := randomToken(32)
		challenge := pkceChallenge(verifier)
		state := randomToken(18)
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return err
		}
		defer ln.Close()
		codeCh := make(chan string, 1)
		redirect := "http://" + ln.Addr().String() + "/callback"
		mux := http.NewServeMux()
		mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("state") != state {
				http.Error(w, "bad state", 400)
				return
			}
			codeCh <- r.URL.Query().Get("code")
			fmt.Fprintln(w, "aidocs auth login complete. You can close this tab.")
		})
		srvHTTP := &http.Server{Handler: mux}
		go srvHTTP.Serve(ln)
		loginURL := srv + "/v1/auth/google/start?mode=cli&state=" + url.QueryEscape(state) + "&cli_redirect=" + url.QueryEscape(redirect) + "&code_challenge=" + url.QueryEscape(challenge)
		fmt.Fprintf(out, "Opening %s\n", loginURL)
		openBrowser(loginURL)
		code := <-codeCh
		_ = srvHTTP.Close()
		cl := &Client{Base: srv, HTTP: http.DefaultClient}
		b, err := cl.json("POST", "/v1/auth/cli/exchange", map[string]any{"code": code, "code_verifier": verifier, "name": first(name, ctxName(srv))})
		if err != nil {
			return err
		}
		var cred map[string]any
		_ = json.Unmarshal(b, &cred)
		cfg, _ := loadConfig()
		if cfg.Contexts == nil {
			cfg.Contexts = map[string]*Context{}
		}
		cn := ctxName(srv)
		cfg.ActiveContext = cn
		cfg.Contexts[cn] = &Context{Server: srv, Credential: storeCredentialToken(cn, cred), Pulled: map[string]string{}}
		if err := saveConfig(cfg); err != nil {
			return err
		}
		if !g.quiet {
			fmt.Fprintln(out, "Logged in")
		}
		return nil
	}}
	cmd.Flags().StringVar(&name, "name", "", "credential name")
	return cmd
}
func whoamiCmd(g *globals, out io.Writer) *cobra.Command {
	return &cobra.Command{Use: "whoami", Short: "Show the authenticated principal", RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client(g)
		if err != nil {
			return err
		}
		b, err := c.do("GET", "/v1/me", nil, "")
		if err != nil {
			return err
		}
		return print(out, g, b)
	}}
}
func logoutCmd(g *globals, out io.Writer) *cobra.Command {
	var localOnly, keepKeychain bool
	cmd := &cobra.Command{Use: "logout", Short: "Revoke and remove the stored credential for the active context", RunE: func(cmd *cobra.Command, args []string) error {
		name, cx, cfg := currentContext(g)
		if !localOnly {
			if id, ok := cx.Credential["id"].(string); ok && id != "" {
				cl, err := client(g)
				if err != nil {
					return err
				}
				if _, err := cl.do("DELETE", "/v1/auth/cli/credentials/"+url.PathEscape(id), nil, ""); err != nil {
					return err
				}
			}
		}
		if !keepKeychain {
			deleteCredentialToken(name, cx.Credential)
		}
		delete(cfg.Contexts, name)
		if cfg.ActiveContext == name {
			cfg.ActiveContext = ""
		}
		if err := saveConfig(cfg); err != nil {
			return err
		}
		if !g.quiet {
			fmt.Fprintln(out, "Logged out")
		}
		return nil
	}}
	cmd.Flags().BoolVar(&localOnly, "local", false, "only remove the local credential; do not revoke on the server")
	cmd.Flags().BoolVar(&keepKeychain, "keep-keychain", false, "preserve keychain token entry while removing config (for backups/tests)")
	return cmd
}
func authCmd(g *globals, out io.Writer) *cobra.Command {
	c := &cobra.Command{Use: "auth", Short: "Authentication commands"}
	c.AddCommand(loginCmd(g, out), whoamiCmd(g, out), logoutCmd(g, out))
	return c
}

func contextCmd(g *globals, out io.Writer) *cobra.Command {
	c := &cobra.Command{Use: "context", Short: "Manage saved server contexts"}
	c.AddCommand(&cobra.Command{Use: "list", Short: "List saved contexts", RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := loadConfig()
		b, _ := json.Marshal(map[string]any{"active_context": cfg.ActiveContext, "contexts": cfg.Contexts})
		return print(out, g, b)
	}}, &cobra.Command{Use: "use <server>", Short: "Switch the active context", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := loadConfig()
		name := ctxName(args[0])
		if cfg.Contexts == nil {
			cfg.Contexts = map[string]*Context{}
		}
		if cfg.Contexts[name] == nil {
			cfg.Contexts[name] = &Context{Server: normalizeServer(args[0]), Pulled: map[string]string{}}
		}
		cfg.ActiveContext = name
		return saveConfig(cfg)
	}})
	return c
}

func docsCmd(g *globals, out io.Writer) *cobra.Command {
	c := &cobra.Command{Use: "docs", Short: "Manage documents"}
	var title, vis string
	create := &cobra.Command{Use: "create <file>", Short: "Create a document from an HTML file", Example: "  aidocs docs create report.html --title 'Report' --visibility private", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		data, fn, err := readFileArg(args[0])
		if err != nil {
			return err
		}
		if title == "" {
			title = fn
		}
		cl, err := client(g)
		if err != nil {
			return err
		}
		b, e := cl.multipart("/v1/documents", map[string]string{"title": title, "visibility": first(vis, "private")}, "file", fn, data)
		if e != nil {
			return e
		}
		return print(out, g, b)
	}}
	create.Flags().StringVar(&title, "title", "", "")
	create.Flags().StringVar(&vis, "visibility", "private", "")
	var ut, uv string
	update := &cobra.Command{Use: "update <doc_id>", Short: "Update document metadata", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := client(g)
		if err != nil {
			return err
		}
		b, e := cl.json("PATCH", "/v1/documents/"+args[0], map[string]any{"title": ut, "visibility": uv})
		if e != nil {
			return e
		}
		return print(out, g, b)
	}}
	update.Flags().StringVar(&ut, "title", "", "")
	update.Flags().StringVar(&uv, "visibility", "", "")
	c.AddCommand(simple(g, out, "list", "GET", "/v1/documents", 0), create, simplePath(g, out, "show", "GET", "/v1/documents/%s"), update, simplePath(g, out, "delete", "DELETE", "/v1/documents/%s"), pullCmd(g, out), docsPushCmd(g, out), commentsCmd(g, out), grantsCmd(g, out))
	return c
}
func grantsCmd(g *globals, out io.Writer) *cobra.Command {
	c := &cobra.Command{Use: "grants", Short: "Manage document grants"}
	var principal, address, role string
	add := &cobra.Command{
		Use:   "add <doc_id>",
		Short: "Share a doc with a person or bot",
		Long: "Share a doc by passing an email or bot address.\n\n" +
			"Examples:\n" +
			"  aidocs grants add doc_… --to anurag@razorpay.com --role commenter\n" +
			"  aidocs grants add doc_… --to n8n-prod@brave.otter.bot --role editor",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, err := client(g)
			if err != nil {
				return err
			}
			body := map[string]any{"role": role}
			switch {
			case address != "":
				body["address"] = address
			case principal != "":
				body["principal"] = parsePrincipal(principal)
			default:
				return errors.New("pass --to <email-or-bot-address>")
			}
			b, e := cl.json("POST", "/v1/documents/"+args[0]+"/grants", body)
			if e != nil {
				return e
			}
			return print(out, g, b)
		},
	}
	add.Flags().StringVar(&address, "to", "", "email or bot address (e.g. anurag@razorpay.com or n8n@brave.otter.bot)")
	add.Flags().StringVar(&principal, "principal", "", "legacy: sa:<id> or user:<email>")
	add.Flags().StringVar(&role, "role", "viewer", "viewer, commenter, editor, or owner")
	var r string
	upd := &cobra.Command{Use: "update <doc_id> <grant_id>", Short: "Update a grant role", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := client(g)
		if err != nil {
			return err
		}
		b, e := cl.json("PATCH", fmt.Sprintf("/v1/documents/%s/grants/%s", args[0], args[1]), map[string]any{"role": r})
		if e != nil {
			return e
		}
		return print(out, g, b)
	}}
	upd.Flags().StringVar(&r, "role", "viewer", "")
	c.AddCommand(simplePath(g, out, "list", "GET", "/v1/documents/%s/grants"), add, upd, &cobra.Command{Use: "revoke <doc_id> <grant_id>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := client(g)
		if err != nil {
			return err
		}
		b, e := cl.do("DELETE", fmt.Sprintf("/v1/documents/%s/grants/%s", args[0], args[1]), nil, "")
		if e != nil {
			return e
		}
		return print(out, g, b)
	}})
	return c
}

func versionsCmd(g *globals, out io.Writer) *cobra.Command {
	c := &cobra.Command{Use: "versions", Short: "Inspect document versions"}
	c.AddCommand(simplePath(g, out, "list", "GET", "/v1/documents/%s/versions"), simplePath(g, out, "show", "GET", "/v1/versions/%s"), simplePath(g, out, "html", "GET", "/v1/versions/%s/html"))
	return c
}
func pullCmd(g *globals, out io.Writer) *cobra.Command {
	var ver, outp string
	cmd := &cobra.Command{Use: "pull <doc_id>", Short: "Download a document HTML version", Example: "  aidocs docs pull doc_123 --out report.html\n  aidocs docs pull doc_123 --version ver_123", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := client(g)
		if err != nil {
			return err
		}
		v := ver
		if v == "" {
			b, e := cl.do("GET", "/v1/documents/"+args[0], nil, "")
			if e != nil {
				return e
			}
			var m map[string]any
			json.Unmarshal(b, &m)
			v = fmt.Sprint(value(m, "current_version_id"))
			if v == "" || v == "<nil>" {
				return errors.New("document response did not include current_version_id")
			}
		}
		b, e := cl.do("GET", "/v1/versions/"+v+"/html", nil, "")
		if e != nil {
			return e
		}
		name, cx, cfg := currentContext(g)
		cx.Pulled[args[0]] = v
		cfg.Contexts[name] = cx
		if cfg.ActiveContext == "" {
			cfg.ActiveContext = name
		}
		saveConfig(cfg)
		if outp != "" {
			return os.WriteFile(outp, b, 0644)
		}
		_, e = out.Write(b)
		return e
	}}
	cmd.Flags().StringVar(&ver, "version", "", "")
	cmd.Flags().StringVar(&outp, "out", "", "")
	return cmd
}
func docsPushCmd(g *globals, out io.Writer) *cobra.Command {
	return pushVersionCmd(g, out, "push <doc_id> <file>", "Upload a new document version", "  aidocs docs push doc_123 report.html --base-version ver_123\n  aidocs docs push doc_123 report.html --summary 'Address comments'", func(args []string) (string, string, error) {
		return args[0], args[1], nil
	})
}

func pushVersionCmd(g *globals, out io.Writer, use, short, example string, resolve func([]string) (string, string, error)) *cobra.Command {
	var base, summary string
	argc := 1
	if strings.Count(use, "<") == 2 {
		argc = 2
	}
	cmd := &cobra.Command{Use: use, Short: short, Example: example, Args: cobra.ExactArgs(argc), RunE: func(cmd *cobra.Command, args []string) error {
		doc, file, err := resolve(args)
		if err != nil {
			return err
		}
		data, fn, err := readFileArg(file)
		if err != nil {
			return err
		}
		if base == "" {
			_, cx, _ := currentContext(g)
			base = cx.Pulled[doc]
		}
		if base == "" {
			return errors.New("no base version known; run aidocs docs pull <doc_id> first or pass --base-version")
		}
		cl, err := client(g)
		if err != nil {
			return err
		}
		b, e := cl.multipart("/v1/documents/"+doc+"/versions", map[string]string{"base_version_id": base, "change_summary": summary}, "file", fn, data)
		if e != nil {
			return e
		}
		return printPushedVersion(out, g, doc, cl.Base, b)
	}}
	cmd.Flags().StringVar(&base, "base-version", "", "base version ID")
	cmd.Flags().StringVar(&summary, "summary", "", "change summary")
	return cmd
}

func commentsCmd(g *globals, out io.Writer) *cobra.Command {
	c := &cobra.Command{Use: "comments", Short: "Manage document review comments"}
	var status, version, body, quote, prefix, suffix, ustatus string
	list := &cobra.Command{Use: "list <doc_id>", Short: "List document comments", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		q := url.Values{}
		if status != "" {
			q.Set("status", status)
		}
		if version != "" {
			q.Set("version_id", version)
		}
		path := "/v1/documents/" + args[0] + "/comments"
		if len(q) > 0 {
			path += "?" + q.Encode()
		}
		cl, err := client(g)
		if err != nil {
			return err
		}
		b, e := cl.do("GET", path, nil, "")
		if e != nil {
			return e
		}
		return print(out, g, b)
	}}
	list.Flags().StringVar(&status, "status", "", "")
	list.Flags().StringVar(&version, "version", "", "")
	create := &cobra.Command{Use: "create <doc_id>", Short: "Create a document comment", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := client(g)
		if err != nil {
			return err
		}
		b, e := cl.json("POST", "/v1/documents/"+args[0]+"/comments", map[string]any{"body": body, "version_id": version, "anchor": map[string]any{"quote": quote, "prefix": prefix, "suffix": suffix}})
		if e != nil {
			return e
		}
		return print(out, g, b)
	}}
	create.Flags().StringVar(&body, "body", "", "")
	create.Flags().StringVar(&quote, "quote", "", "")
	create.Flags().StringVar(&prefix, "prefix", "", "")
	create.Flags().StringVar(&suffix, "suffix", "", "")
	create.Flags().StringVar(&version, "version", "", "")
	update := &cobra.Command{Use: "update <doc_id> <comment_id>", Short: "Update a document comment", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := client(g)
		if err != nil {
			return err
		}
		b, e := cl.json("PATCH", "/v1/documents/"+args[0]+"/comments/"+args[1], map[string]any{"body": body, "status": ustatus})
		if e != nil {
			return e
		}
		return print(out, g, b)
	}}
	update.Flags().StringVar(&body, "body", "", "")
	update.Flags().StringVar(&ustatus, "status", "", "")
	c.AddCommand(list, create, update, deleteCommentCmd(g, out), resolveCmd(g, out, "resolve", "resolved"), resolveCmd(g, out, "reopen", "open"))
	return c
}
func deleteCommentCmd(g *globals, out io.Writer) *cobra.Command {
	return &cobra.Command{Use: "delete <doc_id> <comment_id>", Short: "Delete a document comment", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := client(g)
		if err != nil {
			return err
		}
		b, e := cl.do("DELETE", "/v1/documents/"+args[0]+"/comments/"+args[1], nil, "")
		if e != nil {
			return e
		}
		return print(out, g, b)
	}}
}

func resolveCmd(g *globals, out io.Writer, name, status string) *cobra.Command {
	short := "Resolve document comments"
	if status == "open" {
		short = "Reopen document comments"
	}
	return &cobra.Command{Use: name + " <doc_id> <comment_id>...", Short: short, Args: cobra.MinimumNArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := client(g)
		if err != nil {
			return err
		}
		docID := args[0]
		for _, id := range args[1:] {
			b, e := cl.json("PATCH", "/v1/documents/"+docID+"/comments/"+id, map[string]any{"status": status})
			if e != nil {
				return e
			}
			if err := print(out, g, b); err != nil {
				return err
			}
		}
		return nil
	}}
}

func saRoot(g *globals, out io.Writer) *cobra.Command {
	c := &cobra.Command{Use: "sa", Short: "Manage service accounts", PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if os.Getenv("AIDOCS_DISABLE_SA_COMMANDS") != "" {
			return errors.New("service account commands are disabled by AIDOCS_DISABLE_SA_COMMANDS")
		}
		return nil
	}}
	var name string
	create := &cobra.Command{
		Use:   "create <name>[@<address>]",
		Short: "Create a bot",
		Long: "Create a bot.\n\n" +
			"  <name>      What appears before the @. Letters, numbers, hyphens.\n" +
			"  <address>   Optional. Must end in .bot. If you skip it, we'll\n" +
			"              pick something memorable for you.\n\n" +
			"Examples:\n" +
			"  aidocs sa create n8n-prod\n" +
			"  aidocs sa create ci-runner@ops.team.bot\n" +
			"  aidocs sa create nightly@crew.bot",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, err := client(g)
			if err != nil {
				return err
			}
			label := args[0]
			body := map[string]any{}
			if at := strings.Index(label, "@"); at >= 0 {
				domain := label[at+1:]
				label = label[:at]
				if !strings.HasSuffix(domain, ".bot") {
					return errors.New("Addresses must end in .bot. That's how aidocs tells bots apart from people.")
				}
				body["domain"] = domain
			}
			body["label"] = label
			b, e := cl.json("POST", "/v1/service-accounts", body)
			if e != nil {
				return e
			}
			if g.json {
				return print(out, g, b)
			}
			var resp struct {
				Name string `json:"name"`
				Key  struct {
					Token string `json:"token"`
				} `json:"key"`
			}
			if e := json.Unmarshal(b, &resp); e != nil {
				return print(out, g, b)
			}
			fmt.Fprintf(out, "\u2713 Created %s\n\n", resp.Name)
			fmt.Fprintln(out, "  This is the only time you'll see this key. Save it where your")
			fmt.Fprintln(out, "  automation can read it \u2014 not on your own machine.")
			fmt.Fprintln(out)
			fmt.Fprintf(out, "    %s\n", resp.Key.Token)
			return nil
		},
	}
	var newName string
	var enable, disable bool
	upd := &cobra.Command{Use: "update <sa_id>", Short: "Update a service account", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := client(g)
		if err != nil {
			return err
		}
		body := map[string]any{}
		if newName != "" {
			body["name"] = newName
		}
		if enable {
			body["disabled"] = false
		}
		if disable {
			body["disabled"] = true
		}
		b, e := cl.json("PATCH", "/v1/service-accounts/"+args[0], body)
		if e != nil {
			return e
		}
		return print(out, g, b)
	}}
	upd.Flags().StringVar(&newName, "name", "", "")
	upd.Flags().BoolVar(&enable, "enable", false, "")
	upd.Flags().BoolVar(&disable, "disable", false, "")
	key := &cobra.Command{Use: "key", Short: "Manage service account keys"}
	keyCreate := &cobra.Command{Use: "create <sa_id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := client(g)
		if err != nil {
			return err
		}
		b, e := cl.json("POST", "/v1/service-accounts/"+args[0]+"/keys", map[string]any{"name": first(name, "default")})
		if e != nil {
			return e
		}
		return print(out, g, b)
	}}
	keyCreate.Flags().StringVar(&name, "name", "default", "")
	key.AddCommand(simplePath(g, out, "list", "GET", "/v1/service-accounts/%s/keys"), keyCreate, &cobra.Command{Use: "revoke <sa_id> <key_id>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := client(g)
		if err != nil {
			return err
		}
		b, e := cl.do("DELETE", "/v1/service-accounts/"+args[0]+"/keys/"+args[1], nil, "")
		if e != nil {
			return e
		}
		return print(out, g, b)
	}})
	c.AddCommand(simple(g, out, "list", "GET", "/v1/service-accounts", 0), create, upd, key, transferCmd(g, out), transfersCmd(g, out))
	return c
}
func transferCmd(g *globals, out io.Writer) *cobra.Command {
	var to string
	c := &cobra.Command{Use: "transfer <sa_id>", Short: "Transfer service account ownership", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := client(g)
		if err != nil {
			return err
		}
		b, e := cl.json("POST", "/v1/service-accounts/"+args[0]+"/transfer", map[string]any{"to_user_email": to})
		if e != nil {
			return e
		}
		return print(out, g, b)
	}}
	c.Flags().StringVar(&to, "to", "", "")
	c.AddCommand(simplePath(g, out, "accept", "POST", "/v1/service-accounts/transfers/%s/accept"), simplePath(g, out, "decline", "POST", "/v1/service-accounts/transfers/%s/decline"))
	return c
}

func transfersCmd(g *globals, out io.Writer) *cobra.Command {
	c := &cobra.Command{Use: "transfers", Short: "List incoming service account transfers"}
	c.AddCommand(simple(g, out, "list", "GET", "/v1/service-accounts/transfers", 0))
	return c
}

func openCmd(g *globals, out io.Writer) *cobra.Command {
	return &cobra.Command{Use: "open <doc_id>", Short: "Open a document in the browser", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client(g)
		if err != nil {
			return err
		}
		u := c.Base + "/documents/" + args[0]
		if !g.quiet {
			fmt.Fprintln(out, u)
		}
		return openBrowserErr(u)
	}}
}
func simple(g *globals, out io.Writer, use, method, path string, n int) *cobra.Command {
	return &cobra.Command{Use: use, Short: shortFor(use), Args: cobra.ExactArgs(n), RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := client(g)
		if err != nil {
			return err
		}
		b, e := cl.do(method, path, nil, "")
		if e != nil {
			return e
		}
		return print(out, g, b)
	}}
}
func simplePath(g *globals, out io.Writer, use, method, tmpl string) *cobra.Command {
	return &cobra.Command{Use: use + " <id>", Short: shortFor(use), Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := client(g)
		if err != nil {
			return err
		}
		b, e := cl.do(method, fmt.Sprintf(tmpl, args[0]), nil, "")
		if e != nil {
			return e
		}
		return print(out, g, b)
	}}
}
func shortFor(use string) string {
	switch use {
	case "list":
		return "List resources"
	case "show":
		return "Show resource details"
	case "delete":
		return "Delete a resource"
	case "html":
		return "Print version HTML"
	case "revoke":
		return "Revoke a resource"
	}
	return ""
}

func readFileArg(p string) ([]byte, string, error) {
	if p == "-" {
		b, e := io.ReadAll(os.Stdin)
		return b, "stdin.html", e
	}
	b, e := os.ReadFile(p)
	return b, filepath.Base(p), e
}
func randomToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
func pkceChallenge(v string) string {
	s := sha256.Sum256([]byte(v))
	return base64.RawURLEncoding.EncodeToString(s[:])
}
func openBrowser(u string) { _ = openBrowserErr(u) }
func openBrowserErr(u string) error {
	if os.Getenv("AIDOCS_NO_BROWSER") != "" {
		return nil
	}
	cmds := [][]string{{"open", u}, {"xdg-open", u}, {"rundll32", "url.dll,FileProtocolHandler", u}}
	var last error
	for _, c := range cmds {
		if err := exec.Command(c[0], c[1:]...).Start(); err == nil {
			return nil
		} else {
			last = err
		}
	}
	return fmt.Errorf("could not open browser: %w", last)
}

func parsePrincipal(s string) map[string]any {
	if strings.HasPrefix(s, "sa:") {
		return map[string]any{"type": "service_account", "id": strings.TrimPrefix(s, "sa:")}
	}
	v := strings.TrimPrefix(s, "user:")
	if strings.Contains(v, "@") {
		return map[string]any{"type": "user", "email": v}
	}
	return map[string]any{"type": "user", "id": v}
}
