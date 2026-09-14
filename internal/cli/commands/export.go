package commands

import (
	"encoding/json"
	"fmt"
	"os"

	projctx "github.com/AnshGajera/CTX/internal/context"
	"github.com/AnshGajera/CTX/internal/openapi"
	"github.com/urfave/cli/v2"
)

// ExportCommand implements `ctx export`.
func ExportCommand() *cli.Command {
	return &cli.Command{
		Name:  "export",
		Usage: "Export context to interchange formats",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "format", Value: "openapi", Usage: "openapi|json"},
			&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Usage: "output file (default stdout)"},
		},
		Action: func(c *cli.Context) error {
			cwd, _ := os.Getwd()
			ctx, err := projctx.LoadContext(cwd)
			if err != nil {
				return fmt.Errorf("no context (run ctx init): %w", err)
			}
			var v any
			switch c.String("format") {
			case "openapi":
				var eps []projctx.APIEndpoint
				if ctx.APIs != nil {
					eps = ctx.APIs.Endpoints
				}
				v = openapi.Build(ctx.ProjectName, eps)
			case "json":
				v = ctx
			default:
				return fmt.Errorf("unknown format: %s", c.String("format"))
			}
			data, err := json.MarshalIndent(v, "", "  ")
			if err != nil {
				return err
			}
			if out := c.String("output"); out != "" {
				if err := os.WriteFile(out, data, 0o644); err != nil {
					return err
				}
				fmt.Printf("Wrote %s (%d endpoints)\n", out, endpointCount(ctx))
				return nil
			}
			fmt.Println(string(data))
			return nil
		},
	}
}

func endpointCount(ctx *projctx.ProjectContext) int {
	if ctx.APIs == nil {
		return 0
	}
	return len(ctx.APIs.Endpoints)
}
