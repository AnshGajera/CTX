package commands

import (
	"encoding/json"
	"fmt"
	"os"

	projctx "github.com/AnshGajera/CTX/internal/context"
	"github.com/AnshGajera/CTX/internal/health"
	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
)

// HealthCommand implements `ctx health`.
func HealthCommand() *cli.Command {
	return &cli.Command{
		Name:  "health",
		Usage: "Score context quality (0-100) with improvement tips",
		Flags: []cli.Flag{
			jsonFlag(),
		},
		Action: func(c *cli.Context) error {
			cwd, _ := os.Getwd()
			ctx, err := projctx.LoadContext(cwd)
			if err != nil {
				return fmt.Errorf("no context (run ctx init): %w", err)
			}
			rep := health.Score(ctx)
			if c.Bool("json") {
				return json.NewEncoder(os.Stdout).Encode(rep)
			}
			color.Cyan("Context Health: %d/100 (grade %s, %s)", rep.Score, rep.Grade, rep.Staleness)
			for _, ch := range rep.Checks {
				mark := "✅"
				switch ch.Status {
				case "warn":
					mark = "⚠️ "
				case "fail":
					mark = "❌"
				}
				fmt.Printf("  %s %s: %.0f/%d\n", mark, ch.Name, ch.Score, ch.Max)
			}
			for _, tip := range rep.Tips {
				fmt.Printf("  💡 %s\n", tip)
			}
			return nil
		},
	}
}
