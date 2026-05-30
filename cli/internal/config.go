package internal

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

// File permission constants for config storage.
const (
	configDirPerm  os.FileMode = 0700
	configFilePerm os.FileMode = 0600
	pulledFilePerm os.FileMode = 0644
)

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
	if err := os.MkdirAll(filepath.Dir(p), configDirPerm); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, configFilePerm)
}

func ctxName(s string) string {
	u, err := url.Parse(normalizeServer(s))
	if err == nil && u.Host != "" {
		return u.Host
	}
	return s
}

func currentContext(g *globals) (string, *Context, Config) {
	cfg, _ := loadConfig()
	srv := first(g.server, os.Getenv("AIDOCS_SERVER"), cfg.ActiveContext, defaultServer)
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
