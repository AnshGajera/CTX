package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AnshGajera/CTX/internal/versioning"
	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
)

// MergeCommand implements `ctx merge <branch> [-m message]`.
func MergeCommand() *cli.Command {
	return &cli.Command{
		Name:  "merge",
		Usage: "Merge context from another branch into the active branch",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "message", Aliases: []string{"m"}, Usage: "merge commit message"},
			jsonFlag(),
		},
		Action: func(c *cli.Context) error {
			if c.NArg() < 1 {
				return fmt.Errorf("merge requires a source branch or ref (e.g. ctx merge feature-auth)")
			}

			sourceRef := c.Args().Get(0)
			msg := c.String("message")

			cwd, _ := os.Getwd()
			store := versioning.NewContextStore(filepath.Join(cwd, ".ctx"))

			snap, report, err := store.Merge(sourceRef, msg)
			if err != nil {
				return err
			}

			if c.Bool("json") {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"merged_snapshot": snap.Hash,
					"report":          report,
				})
			}

			color.Green("Merged %s into %s (new HEAD at %s)", sourceRef, report.TargetBranch, shortHash(snap.Hash))
			if len(report.Details) > 0 {
				for _, d := range report.Details {
					fmt.Printf("  • %s\n", d)
				}
			}
			fmt.Printf("Summary: +%d endpoints, +%d models, +%d env vars, +%d deps, +%d patterns\n",
				report.AddedEndpoints, report.AddedModels, report.AddedEnvVars, report.AddedDeps, report.MergedPatterns)
			return nil
		},
	}
}
