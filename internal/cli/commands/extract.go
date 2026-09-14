package commands

import (
	"encoding/json"
	"os"

	"github.com/urfave/cli/v2"
)

// ExtractCommand implements `ctx extract`.
func ExtractCommand() *cli.Command {
	return &cli.Command{
		Name:  "extract",
		Usage: "Extract project context",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "quiet", Usage: "suppress output"},
			&cli.BoolFlag{Name: "on-commit", Usage: "mark extraction as on-commit"},
			&cli.StringSliceFlag{Name: "sections", Usage: "limit sections (repeatable)"},
			jsonFlag(),
		},
		Action: func(c *cli.Context) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			jsonMode := c.Bool("json")
			deduped, err := runExtract(cwd, c.Bool("quiet") || jsonMode, c.Bool("on-commit"), c.StringSlice("sections"))
			if err != nil {
				return err
			}
			if jsonMode {
				summary, err := extractSummary(cwd, deduped)
				if err != nil {
					return err
				}
				return json.NewEncoder(os.Stdout).Encode(summary)
			}
			return nil
		},
	}
}
