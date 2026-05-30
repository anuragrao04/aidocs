package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// defaultServer is the single source of truth for the default API server URL.
const defaultServer = "https://aidocs.anuragrao.dev"

// Client is a thin HTTP transport for the aidocs API.
type Client struct {
	Base, Token string
	HTTP        *http.Client
}

// client builds a Client from flags, environment, and the active config context.
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
		srv = defaultServer
	}
	return &Client{Base: normalizeServer(srv), Token: tok, HTTP: http.DefaultClient}, nil
}

// APIError represents a non-2xx response from the API.
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
		b, err := json.Marshal(in)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(b)
	}
	return c.do(method, path, r, "application/json")
}

func (c *Client) multipart(path string, fields map[string]string, fileField, fileName string, data []byte) ([]byte, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			return nil, err
		}
	}
	fw, err := mw.CreateFormFile(fileField, fileName)
	if err != nil {
		return nil, err
	}
	if _, err := fw.Write(data); err != nil {
		return nil, err
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}
	return c.do("POST", path, &buf, mw.FormDataContentType())
}

// apiPath builds a request path, URL-escaping every caller-supplied ID segment
// so raw IDs cannot break out of their path position.
func apiPath(format string, ids ...string) string {
	esc := make([]any, len(ids))
	for i, id := range ids {
		esc[i] = url.PathEscape(id)
	}
	return fmt.Sprintf(format, esc...)
}

// browserURL builds an absolute URL from a server base and a path template,
// URL-escaping every ID segment.
func browserURL(base, format string, ids ...string) string {
	return strings.TrimRight(base, "/") + apiPath(format, ids...)
}
