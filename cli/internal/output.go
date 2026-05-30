package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// displayFields is the ordered set of keys rendered as a compact key=value row
// in the default (non-JSON) output. Fields absent from a payload are skipped;
// fields not listed here are omitted from the compact view (use --json for the
// full payload).
var displayFields = []string{
	"id", "document_id", "title", "name", "email", "type", "role", "status",
	"disabled", "current_version_id", "number", "sha256", "token",
	"selected_text", "body", "url",
}

// render writes b to out, honoring --quiet and --json. It is the single output
// path for API response bodies.
func render(out io.Writer, g *globals, b []byte) error {
	return renderHint(out, g, b, "")
}

func renderHint(out io.Writer, g *globals, b []byte, hint string) error {
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

// message writes a plain human-facing status line, honoring --quiet. It is the
// single path for non-payload status text (e.g. "Logged in").
func message(out io.Writer, g *globals, text string) {
	if g.quiet {
		return
	}
	fmt.Fprintln(out, text)
}

// confirm writes a conversational, action-indicative status line (prefixed with
// a check mark) so the caller can see what just happened rather than an
// API-shaped payload. Honors --quiet.
func confirm(out io.Writer, g *globals, text string) {
	if g.quiet {
		return
	}
	fmt.Fprintln(out, "\u2713 "+text)
}

// action performs a mutating call and reports it conversationally. With --json
// it prints the raw payload (machine-readable); with --quiet it prints nothing;
// otherwise it prints the confirmation built from the decoded response.
func action(g *globals, out io.Writer, fn func(*Client) ([]byte, error), msg func(m map[string]any) string) error {
	c, err := client(g)
	if err != nil {
		return err
	}
	b, err := fn(c)
	if err != nil {
		return err
	}
	if g.quiet {
		return nil
	}
	if g.json {
		return render(out, g, b)
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	confirm(out, g, msg(m))
	return nil
}

func printPushedVersion(out io.Writer, g *globals, docID, server string, b []byte) error {
	if g.quiet {
		return nil
	}
	if g.json {
		return render(out, g, b)
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return render(out, g, b)
	}
	num := fmt.Sprint(value(m, "number"))
	id := fmt.Sprint(value(m, "id"))
	url := browserURL(server, "/documents/%s", docID)
	confirm(out, g, fmt.Sprintf("Pushed version %s (%s) to %s. Review it at %s", num, id, docID, url))
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

// value looks up a snake_case key in m, returning nil if m is nil or the key
// is absent.
func value(m map[string]any, key string) any {
	if m == nil {
		return nil
	}
	return m[key]
}

// compactRow renders a resource map as an ordered key=value line using the
// centralized displayFields schema.
func compactRow(m map[string]any) string {
	if m == nil {
		return "OK"
	}
	parts := []string{}
	for _, k := range displayFields {
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
