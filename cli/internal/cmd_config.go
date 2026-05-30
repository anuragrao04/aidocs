package internal

import (
	"encoding/json"
	"io"

	"github.com/spf13/cobra"
)

// configCmd reports the connected server's deployment type and the label it
// uses for the "everyone" grant, so an agent knows the scope/wording of the
// context it is talking to.
func configCmd(g *globals, out io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Show the connected server's deployment configuration",
		Args:  exactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(g, out, func(c *Client) ([]byte, error) {
				return c.do("GET", "/v1/config", nil, "")
			})
		},
	}
}

// everyoneLabelFor fetches /v1/config to resolve the human label for the
// "everyone" grant on the connected server. It falls back to a generic phrase
// if the server is unreachable or pre-dates the endpoint, so sharing still
// reports something sensible.
func everyoneLabelFor(g *globals) string {
	const fallback = "everyone on this server"
	cl, err := client(g)
	if err != nil {
		return fallback
	}
	b, err := cl.do("GET", "/v1/config", nil, "")
	if err != nil {
		return fallback
	}
	var cfg struct {
		EveryoneLabel string `json:"everyone_label"`
	}
	if json.Unmarshal(b, &cfg) != nil || cfg.EveryoneLabel == "" {
		return fallback
	}
	return cfg.EveryoneLabel
}
