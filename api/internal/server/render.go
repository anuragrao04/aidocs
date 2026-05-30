package server

import (
	_ "embed"
	"encoding/json"
	"html/template"
	"strings"
)

//go:embed bridge.js
var bridgeJSSource string

// wrapperTmpl renders the sandboxed iframe that hosts untrusted user HTML.
// Using html/template ensures the srcdoc attribute value is correctly escaped
// (including </script> sequences inside inline scripts).
var wrapperTmpl = template.Must(template.New("wrapper").Parse(`<!doctype html><html><head><meta charset="utf-8"><title>aidocs render</title><style>html,body{margin:0;height:100%;overflow:hidden}#aidocs-doc{border:0;width:100%;height:100vh}.aidocs-mark{background-color:#fef08a!important;color:inherit!important;border-bottom:2px solid #ca8a04!important;cursor:pointer;transition:background-color .15s}.aidocs-mark-active{background-color:#fbbf24!important}</style></head><body><iframe id="aidocs-doc" sandbox="allow-scripts allow-same-origin" srcdoc="{{.SrcDoc}}"></iframe><script>window.__AIDOCS_APP_ORIGIN__={{.AppOriginJS}};({{.BridgeJS}})();</script></body></html>`))

type wrapperData struct {
	// SrcDoc is pre-escaped HTML for use as a double-quoted attribute value.
	// Marked template.HTML so html/template does not double-escape it.
	SrcDoc      template.HTML
	AppOriginJS template.JS
	BridgeJS    template.JS
}

// renderWrapperHTML returns the full wrapper page that embeds userHTML inside
// a sandboxed iframe. appOrigin is used for the Content-Security-Policy and
// for the bridge JS postMessage target origin.
func renderWrapperHTML(userHTML []byte, appOrigin string) []byte {
	originJSON, _ := json.Marshal(appOrigin)
	data := wrapperData{
		SrcDoc:      template.HTML(htmlEscapeAttr(string(userHTML))),
		AppOriginJS: template.JS(originJSON),
		BridgeJS:    template.JS(bridgeJSSource),
	}
	var sb strings.Builder
	if err := wrapperTmpl.Execute(&sb, data); err != nil {
		// Fallback: should never happen with a valid template.
		return []byte("<!doctype html><html><body>render error</body></html>")
	}
	return []byte(sb.String())
}

// htmlEscapeAttr escapes a string for use as an HTML attribute value
// (double-quoted). The standard escapes cover everything needed for srcdoc.
func htmlEscapeAttr(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// jsString JSON-encodes a string for safe embedding inside a JS expression.
func jsString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
