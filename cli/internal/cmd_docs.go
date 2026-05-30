package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/spf13/cobra"
)

func docsCmd(g *globals, out io.Writer) *cobra.Command {
	c := &cobra.Command{Use: "docs", Short: "Manage documents"}
	var title, vis string
	create := &cobra.Command{Use: "create <file>", Short: "Create a document from an HTML file", Long: "Create a document from a single self-contained HTML file.\n\nRun 'aidocs guidelines' for authoring rules (single-file, base64 images, reader theme).", Example: "  aidocs docs create report.html --title 'Report' --visibility private", Args: exactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		data, fn, err := readFileArg(args[0])
		if err != nil {
			return err
		}
		if title == "" {
			title = fn
		}
		return run(g, out, func(c *Client) ([]byte, error) {
			return c.multipart("/v1/documents", map[string]string{"title": title, "visibility": first(vis, "private")}, "file", fn, data)
		})
	}}
	create.Flags().StringVar(&title, "title", "", "document title (defaults to the file name)")
	create.Flags().StringVar(&vis, "visibility", "private", "document visibility: private or public")
	var ut, uv string
	update := &cobra.Command{Use: "update <doc_id>", Short: "Update document metadata", Args: exactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		body := map[string]any{}
		if cmd.Flags().Changed("title") {
			body["title"] = ut
		}
		if cmd.Flags().Changed("visibility") {
			body["visibility"] = uv
		}
		if len(body) == 0 {
			return errors.New("nothing to update; pass --title and/or --visibility")
		}
		return run(g, out, func(c *Client) ([]byte, error) {
			return c.doJSON("PATCH", apiPath("/v1/documents/%s", args[0]), body)
		})
	}}
	update.Flags().StringVar(&ut, "title", "", "new document title")
	update.Flags().StringVar(&uv, "visibility", "", "new visibility: private or public")
	c.AddCommand(simple(g, out, "list", "GET", "/v1/documents", 0), create, simplePath(g, out, "show", "GET", "/v1/documents/%s"), update, simplePath(g, out, "delete", "DELETE", "/v1/documents/%s"), pullCmd(g, out), docsPushCmd(g, out), versionsCmd(g, out), commentsCmd(g, out), grantsCmd(g, out))
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
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{"role": role}
			switch {
			case address != "":
				body["address"] = address
			case principal != "":
				body["principal"] = parsePrincipal(principal)
			default:
				return errors.New("pass --to <email-or-bot-address>")
			}
			return run(g, out, func(c *Client) ([]byte, error) {
				return c.doJSON("POST", apiPath("/v1/documents/%s/grants", args[0]), body)
			})
		},
	}
	add.Flags().StringVar(&address, "to", "", "email or bot address (e.g. anurag@razorpay.com or n8n@brave.otter.bot)")
	add.Flags().StringVar(&principal, "principal", "", "legacy: sa:<id> or user:<email>")
	add.Flags().StringVar(&role, "role", "viewer", "viewer, commenter, editor, or owner")
	var r string
	upd := &cobra.Command{Use: "update <doc_id> <grant_id>", Short: "Update a grant role", Args: exactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		return run(g, out, func(c *Client) ([]byte, error) {
			return c.doJSON("PATCH", apiPath("/v1/documents/%s/grants/%s", args[0], args[1]), map[string]any{"role": r})
		})
	}}
	upd.Flags().StringVar(&r, "role", "viewer", "new grant role: viewer, commenter, editor, or owner")
	revoke := &cobra.Command{Use: "revoke <doc_id> <grant_id>", Short: "Revoke a document grant", Args: exactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		return run(g, out, func(c *Client) ([]byte, error) {
			return c.do("DELETE", apiPath("/v1/documents/%s/grants/%s", args[0], args[1]), nil, "")
		})
	}}
	c.AddCommand(simplePath(g, out, "list", "GET", "/v1/documents/%s/grants"), add, upd, revoke)
	return c
}

func versionsCmd(g *globals, out io.Writer) *cobra.Command {
	c := &cobra.Command{Use: "versions", Short: "Inspect document versions"}
	list := simplePath(g, out, "list", "GET", "/v1/documents/%s/versions")
	list.Use = "list <doc_id>"
	c.AddCommand(
		list,
		docVersionCmd(g, out, "show", "GET", "/v1/documents/%s/versions/%s"),
		docVersionCmd(g, out, "html", "GET", "/v1/documents/%s/versions/%s/html"),
	)
	return c
}

// docVersionCmd builds a versions subcommand scoped to a document, taking
// <doc_id> <version_id> and filling a path template with two %s segments.
func docVersionCmd(g *globals, out io.Writer, use, method, tmpl string) *cobra.Command {
	return &cobra.Command{Use: use + " <doc_id> <version_id>", Short: shortFor(use), Args: exactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		return run(g, out, func(c *Client) ([]byte, error) {
			return c.do(method, apiPath(tmpl, args[0], args[1]), nil, "")
		})
	}}
}

func pullCmd(g *globals, out io.Writer) *cobra.Command {
	var ver, outp string
	cmd := &cobra.Command{Use: "pull <doc_id>", Short: "Download a document HTML version", Example: "  aidocs docs pull doc_123 --out report.html\n  aidocs docs pull doc_123 --version ver_123", Args: exactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := client(g)
		if err != nil {
			return err
		}
		v := ver
		if v == "" {
			b, err := cl.do("GET", apiPath("/v1/documents/%s", args[0]), nil, "")
			if err != nil {
				return err
			}
			var m map[string]any
			if err := json.Unmarshal(b, &m); err != nil {
				return fmt.Errorf("could not parse document response: %w", err)
			}
			v = fmt.Sprint(value(m, "current_version_id"))
			if v == "" || v == "<nil>" {
				return errors.New("document response did not include current_version_id")
			}
		}
		b, err := cl.do("GET", apiPath("/v1/documents/%s/versions/%s/html", args[0], v), nil, "")
		if err != nil {
			return err
		}
		name, cx, cfg := currentContext(g)
		cx.Pulled[args[0]] = v
		cfg.Contexts[name] = cx
		if cfg.ActiveContext == "" {
			cfg.ActiveContext = name
		}
		if err := saveConfig(cfg); err != nil {
			return err
		}
		if outp != "" {
			return os.WriteFile(outp, b, pulledFilePerm)
		}
		_, err = out.Write(b)
		return err
	}}
	cmd.Flags().StringVar(&ver, "version", "", "specific version ID to pull (defaults to the current version)")
	cmd.Flags().StringVar(&outp, "out", "", "write HTML to this file instead of stdout")
	return cmd
}

func docsPushCmd(g *globals, out io.Writer) *cobra.Command {
	var base, summary string
	cmd := &cobra.Command{Use: "push <doc_id> <file>", Short: "Upload a new document version", Example: "  aidocs docs push doc_123 report.html --base-version ver_123\n  aidocs docs push doc_123 report.html --summary 'Address comments'", Args: exactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		doc, file := args[0], args[1]
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
		b, err := cl.multipart(apiPath("/v1/documents/%s/versions", doc), map[string]string{"base_version_id": base, "change_summary": summary}, "file", fn, data)
		if err != nil {
			return err
		}
		return printPushedVersion(out, g, doc, cl.Base, b)
	}}
	cmd.Flags().StringVar(&base, "base-version", "", "base version ID")
	cmd.Flags().StringVar(&summary, "summary", "", "change summary")
	return cmd
}

func commentsCmd(g *globals, out io.Writer) *cobra.Command {
	c := &cobra.Command{Use: "comments", Short: "Manage document review comments"}

	var listStatus, listVersion string
	list := &cobra.Command{Use: "list <doc_id>", Short: "List document comments", Args: exactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		q := url.Values{}
		if listStatus != "" {
			q.Set("status", listStatus)
		}
		if listVersion != "" {
			q.Set("version_id", listVersion)
		}
		path := apiPath("/v1/documents/%s/comments", args[0])
		if len(q) > 0 {
			path += "?" + q.Encode()
		}
		return run(g, out, func(c *Client) ([]byte, error) {
			return c.do("GET", path, nil, "")
		})
	}}
	list.Flags().StringVar(&listStatus, "status", "", "filter by status: open or resolved")
	list.Flags().StringVar(&listVersion, "version", "", "filter by version ID")

	var createBody, createQuote, createPrefix, createSuffix, createVersion string
	create := &cobra.Command{Use: "create <doc_id>", Short: "Create a document comment", Args: exactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if createBody == "" || createVersion == "" || createQuote == "" {
			return errors.New("--body, --version, and --quote are required")
		}
		return run(g, out, func(c *Client) ([]byte, error) {
			return c.doJSON("POST", apiPath("/v1/documents/%s/comments", args[0]), map[string]any{"body": createBody, "version_id": createVersion, "anchor": map[string]any{"quote": createQuote, "prefix": createPrefix, "suffix": createSuffix}})
		})
	}}
	create.Flags().StringVar(&createBody, "body", "", "comment body text")
	create.Flags().StringVar(&createQuote, "quote", "", "anchor: the exact quoted text")
	create.Flags().StringVar(&createPrefix, "prefix", "", "anchor: text immediately before the quote")
	create.Flags().StringVar(&createSuffix, "suffix", "", "anchor: text immediately after the quote")
	create.Flags().StringVar(&createVersion, "version", "", "version ID the comment anchors to")

	var updateBody, updateStatus string
	update := &cobra.Command{Use: "update <doc_id> <comment_id>", Short: "Update a document comment", Args: exactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		if updateBody == "" && updateStatus == "" {
			return errors.New("nothing to update; pass --body and/or --status")
		}
		return run(g, out, func(c *Client) ([]byte, error) {
			return c.doJSON("PATCH", apiPath("/v1/documents/%s/comments/%s", args[0], args[1]), map[string]any{"body": updateBody, "status": updateStatus})
		})
	}}
	update.Flags().StringVar(&updateBody, "body", "", "new comment body text")
	update.Flags().StringVar(&updateStatus, "status", "", "new status: open or resolved")

	c.AddCommand(list, create, update, deleteCommentCmd(g, out), resolveCmd(g, out, "resolve", "resolved"), resolveCmd(g, out, "reopen", "open"))
	return c
}

func deleteCommentCmd(g *globals, out io.Writer) *cobra.Command {
	return &cobra.Command{Use: "delete <doc_id> <comment_id>", Short: "Delete a document comment", Args: exactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		return run(g, out, func(c *Client) ([]byte, error) {
			return c.do("DELETE", apiPath("/v1/documents/%s/comments/%s", args[0], args[1]), nil, "")
		})
	}}
}

func resolveCmd(g *globals, out io.Writer, name, status string) *cobra.Command {
	short := "Resolve document comments"
	if status == "open" {
		short = "Reopen document comments"
	}
	return &cobra.Command{Use: name + " <doc_id> <comment_id>...", Short: short, Args: minArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := client(g)
		if err != nil {
			return err
		}
		docID := args[0]
		for _, id := range args[1:] {
			b, err := cl.doJSON("PATCH", apiPath("/v1/documents/%s/comments/%s", docID, id), map[string]any{"status": status})
			if err != nil {
				return err
			}
			if err := render(out, g, b); err != nil {
				return err
			}
		}
		return nil
	}}
}

func openCmd(g *globals, out io.Writer) *cobra.Command {
	return &cobra.Command{Use: "open <doc_id>", Short: "Open a document in the browser", Args: exactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client(g)
		if err != nil {
			return err
		}
		u := browserURL(c.Base, "/documents/%s", args[0])
		message(out, g, u)
		return openBrowserErr(u)
	}}
}
