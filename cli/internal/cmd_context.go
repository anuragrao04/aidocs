package internal

import (
	"encoding/json"
	"io"

	"github.com/spf13/cobra"
)

func contextCmd(g *globals, out io.Writer) *cobra.Command {
	c := &cobra.Command{Use: "context", Short: "Manage saved server contexts"}
	c.AddCommand(&cobra.Command{Use: "list", Short: "List saved contexts", RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := loadConfig()
		b, err := json.Marshal(map[string]any{"active_context": cfg.ActiveContext, "contexts": cfg.Contexts})
		if err != nil {
			return err
		}
		return render(out, g, b)
	}}, &cobra.Command{Use: "use <server>", Short: "Switch the active context", Args: exactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := loadConfig()
		name := ctxName(args[0])
		if cfg.Contexts == nil {
			cfg.Contexts = map[string]*Context{}
		}
		if cfg.Contexts[name] == nil {
			cfg.Contexts[name] = &Context{Server: normalizeServer(args[0]), Pulled: map[string]string{}}
		}
		cfg.ActiveContext = name
		return saveConfig(cfg)
	}})
	return c
}
