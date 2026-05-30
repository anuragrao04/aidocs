package server

import (
	"strings"
	"testing"
)

func TestRenderWrapperPreservesInlineScriptClosingTag(t *testing.T) {
	userHTML := []byte(`<html><body><script>window.__aidocsRegression=42;</script></body></html>`)
	out := string(renderWrapperHTML(userHTML, "https://app.example"))

	for _, needle := range []string{`<\/script`, `&lt;\/script`, `<\\/script`} {
		if strings.Contains(out, needle) {
			t.Fatalf("rendered wrapper contains %q, which corrupts user inline <script>: %s", needle, out)
		}
	}

	if !strings.Contains(out, `&lt;/script&gt;`) {
		t.Fatalf("expected user's </script> close to be HTML-entity-encoded inside srcdoc; got: %s", out)
	}
}

func TestBridgeImplementsThemeContract(t *testing.T) {
	for _, needle := range []string{
		"data-aidocs-theme",
		"aidocs_theme",
		"aidocs:theme",
	} {
		if !strings.Contains(bridgeJSSource, needle) {
			t.Fatalf("bridge.js missing theme-handshake hook %q", needle)
		}
	}
}

func TestRenderWrapperLeavesScriptSubstringInJSAlone(t *testing.T) {
	userHTML := []byte(`<html><body><script>var s = "</script>oops";</script></body></html>`)
	out := string(renderWrapperHTML(userHTML, "https://app.example"))
	if strings.Contains(out, `\/script`) {
		t.Fatalf("rendered wrapper still injects backslash before /script: %s", out)
	}
	if !strings.Contains(out, `&lt;/script&gt;oops&#34;;&lt;/script&gt;`) {
		t.Fatalf("expected both inline </script> occurrences to be HTML-entity-encoded; got: %s", out)
	}
}
