package internal

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// guidelinesText is authoring guidance for the agents that produce the HTML
// documents aidocs hosts. It is intentionally plain text so an agent can read
// it straight from `aidocs guidelines` before writing a document.
const guidelinesText = `aidocs document authoring guidelines

A document is ONE HTML file.
  - Anything that fits in a single .html file is fine, including external
    stylesheets, fonts, scripts, and other CDN assets.
  - aidocs does NOT host assets for you. There is no image/file hosting,
    so embed your own images as base64 data URLs (data:image/png;base64,...)
    or reference them from a URL you already control.

Theme: follow the reader's aidocs theme.
  The viewer chooses light or dark in the aidocs UI. The render bridge sets
  this attribute on <html> inside your document and keeps it in sync:

    <html data-aidocs-theme="light">
    <html data-aidocs-theme="dark">

  Opt in by defining your palette against the attribute and driving the page
  from variables:

    :root[data-aidocs-theme="dark"] {
      --bg: #0b0d12; --ink: #e7ecf3;
    }
    :root { --bg: #fff; --ink: #111; }
    body { background: var(--bg); color: var(--ink); }
    html { color-scheme: light; }
    :root[data-aidocs-theme="dark"] { color-scheme: dark; }

  Notes:
  - "system" is resolved to a concrete light/dark before it reaches you.
  - The value updates live when the reader toggles; CSS reacts automatically.
  - For JS, listen for the event on your own window:
      window.addEventListener('aidocs:theme', e => {
        // e.detail.theme is "light" or "dark"
      });
  - Do NOT ship your own theme toggle. The aidocs UI is the single control.
  - A document that never references data-aidocs-theme keeps its hard-coded
    look, so theming is strictly opt-in.

Publishing:
  aidocs docs create report.html --title 'Report' --visibility private
  aidocs docs push <doc_id> report.html --summary 'Address comments'
`

func guidelinesCmd(_ *globals, out io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "guidelines",
		Short: "Print HTML authoring guidelines (single-file, images, theme)",
		Args:  exactArgs(0),
		RunE: func(_ *cobra.Command, _ []string) error {
			fmt.Fprint(out, guidelinesText)
			return nil
		},
	}
}
