package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func saRoot(g *globals, out io.Writer) *cobra.Command {
	c := &cobra.Command{Use: "sa", Short: "Manage service accounts", PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if os.Getenv("AIDOCS_DISABLE_SA_COMMANDS") != "" {
			return errSADisabled
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
			b, err := cl.json("POST", "/v1/service-accounts", body)
			if err != nil {
				return err
			}
			if g.json {
				return render(out, g, b)
			}
			var resp struct {
				Name string `json:"name"`
				Key  struct {
					Token string `json:"token"`
				} `json:"key"`
			}
			if err := json.Unmarshal(b, &resp); err != nil {
				return render(out, g, b)
			}
			if g.quiet {
				return nil
			}
			fmt.Fprintf(out, "\u2713 Created %s\n\n", resp.Name)
			fmt.Fprintln(out, "  ⚠  Copy this key now \u2014 you won't see it again.")
			fmt.Fprintln(out)
			fmt.Fprintf(out, "    %s\n", resp.Key.Token)
			return nil
		},
	}
	var newName string
	var enable, disable bool
	upd := &cobra.Command{Use: "update <sa_id>", Short: "Update a service account", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
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
		return run(g, out, func(c *Client) ([]byte, error) {
			return c.json("PATCH", apiPath("/v1/service-accounts/%s", args[0]), body)
		})
	}}
	upd.Flags().StringVar(&newName, "name", "", "new service account name")
	upd.Flags().BoolVar(&enable, "enable", false, "enable (un-disable) the service account")
	upd.Flags().BoolVar(&disable, "disable", false, "disable the service account")
	key := &cobra.Command{Use: "key", Short: "Manage service account keys"}
	keyCreate := &cobra.Command{Use: "create <sa_id>", Short: "Create a service account key", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return run(g, out, func(c *Client) ([]byte, error) {
			return c.json("POST", apiPath("/v1/service-accounts/%s/keys", args[0]), map[string]any{"name": first(name, "default")})
		})
	}}
	keyCreate.Flags().StringVar(&name, "name", "default", "key name")
	keyRevoke := &cobra.Command{Use: "revoke <sa_id> <key_id>", Short: "Revoke a service account key", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		return run(g, out, func(c *Client) ([]byte, error) {
			return c.do("DELETE", apiPath("/v1/service-accounts/%s/keys/%s", args[0], args[1]), nil, "")
		})
	}}
	key.AddCommand(simplePath(g, out, "list", "GET", "/v1/service-accounts/%s/keys"), keyCreate, keyRevoke)
	c.AddCommand(simple(g, out, "list", "GET", "/v1/service-accounts", 0), create, upd, key, transferCmd(g, out), transfersCmd(g, out))
	return c
}

func transferCmd(g *globals, out io.Writer) *cobra.Command {
	var to string
	c := &cobra.Command{Use: "transfer <sa_id>", Short: "Transfer service account ownership", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return run(g, out, func(c *Client) ([]byte, error) {
			return c.json("POST", apiPath("/v1/service-accounts/%s/transfer", args[0]), map[string]any{"to_user_email": to})
		})
	}}
	c.Flags().StringVar(&to, "to", "", "email of the user to transfer ownership to")
	c.AddCommand(simplePath(g, out, "accept", "POST", "/v1/service-accounts/transfers/%s/accept"), simplePath(g, out, "decline", "POST", "/v1/service-accounts/transfers/%s/decline"))
	return c
}

func transfersCmd(g *globals, out io.Writer) *cobra.Command {
	c := &cobra.Command{Use: "transfers", Short: "List incoming service account transfers"}
	c.AddCommand(simple(g, out, "list", "GET", "/v1/service-accounts/transfers", 0))
	return c
}
