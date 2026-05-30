package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/spf13/cobra"
)

// loginTimeout bounds how long we wait for the browser OAuth flow to complete.
const loginTimeout = 5 * time.Minute

type callbackResult struct {
	code string
	err  error
}

func loginCmd(g *globals, out io.Writer) *cobra.Command {
	var name string
	cmd := &cobra.Command{Use: "login [server]", Short: "Login with Google through an aidocs server", Example: "  aidocs auth login\n  aidocs auth login aidocs.razorpay.com", RunE: func(cmd *cobra.Command, args []string) error {
		srv := defaultServer
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
		resultCh := make(chan callbackResult, 1)
		redirect := "http://" + ln.Addr().String() + "/callback"
		mux := http.NewServeMux()
		mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			if q.Get("state") != state {
				http.Error(w, "bad state", http.StatusBadRequest)
				select {
				case resultCh <- callbackResult{err: errors.New("login callback returned an unexpected state value")}:
				default:
				}
				return
			}
			if e := q.Get("error"); e != "" {
				http.Error(w, "login failed: "+e, http.StatusBadRequest)
				select {
				case resultCh <- callbackResult{err: fmt.Errorf("login failed: %s", e)}:
				default:
				}
				return
			}
			code := q.Get("code")
			if code == "" {
				http.Error(w, "missing code", http.StatusBadRequest)
				select {
				case resultCh <- callbackResult{err: errors.New("login callback did not include an authorization code")}:
				default:
				}
				return
			}
			fmt.Fprintln(w, "aidocs auth login complete. You can close this tab.")
			select {
			case resultCh <- callbackResult{code: code}:
			default:
			}
		})
		srvHTTP := &http.Server{Handler: mux}
		go srvHTTP.Serve(ln)
		defer srvHTTP.Close()
		loginURL := srv + "/v1/auth/google/start?mode=cli&state=" + url.QueryEscape(state) + "&cli_redirect=" + url.QueryEscape(redirect) + "&code_challenge=" + url.QueryEscape(challenge)
		message(out, g, "Opening "+loginURL)
		openBrowser(loginURL)

		ctx, cancel := context.WithTimeout(cmd.Context(), loginTimeout)
		defer cancel()
		var code string
		select {
		case res := <-resultCh:
			if res.err != nil {
				return res.err
			}
			code = res.code
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for login callback after %s; run aidocs auth login again", loginTimeout)
		}

		cl := &Client{Base: srv, HTTP: http.DefaultClient}
		b, err := cl.doJSON("POST", "/v1/auth/cli/exchange", map[string]any{"code": code, "code_verifier": verifier, "name": first(name, ctxName(srv))})
		if err != nil {
			return err
		}
		var cred map[string]any
		if err := json.Unmarshal(b, &cred); err != nil {
			return fmt.Errorf("could not parse login response: %w", err)
		}
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		if cfg.Contexts == nil {
			cfg.Contexts = map[string]*Context{}
		}
		cn := ctxName(srv)
		cfg.ActiveContext = cn
		cfg.Contexts[cn] = &Context{Server: srv, Credential: storeCredentialToken(cn, cred), Pulled: map[string]string{}}
		if err := saveConfig(cfg); err != nil {
			return err
		}
		confirm(out, g, "Logged in to "+srv+".")
		return nil
	}}
	cmd.Flags().StringVar(&name, "name", "", "credential name")
	return cmd
}

func whoamiCmd(g *globals, out io.Writer) *cobra.Command {
	return &cobra.Command{Use: "whoami", Short: "Show the authenticated principal", RunE: func(cmd *cobra.Command, args []string) error {
		return run(g, out, func(c *Client) ([]byte, error) {
			return c.do("GET", "/v1/me", nil, "")
		})
	}}
}

func logoutCmd(g *globals, out io.Writer) *cobra.Command {
	var localOnly, keepKeychain bool
	cmd := &cobra.Command{Use: "logout", Short: "Revoke and remove the stored credential for the active context", RunE: func(cmd *cobra.Command, args []string) error {
		name, cx, cfg, err := currentContextE(g)
		if err != nil {
			return err
		}
		if !localOnly {
			if id, ok := cx.Credential["id"].(string); ok && id != "" {
				cl, err := client(g)
				if err != nil {
					return err
				}
				if _, err := cl.do("DELETE", apiPath("/v1/auth/cli/credentials/%s", id), nil, ""); err != nil {
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
		confirm(out, g, "Logged out of "+name+".")
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
