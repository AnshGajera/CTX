package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/AnshGajera/CTX/internal/version"
	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
)

// BumpCommand implements `ctx bump [patch|minor|major|<version>]`.
func BumpCommand() *cli.Command {
	return &cli.Command{
		Name:    "bump",
		Aliases: []string{"version-bump"},
		Usage:   "Bump semantic version for Go and project version files",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "tag", Aliases: []string{"t"}, Usage: "create git tag for the new version"},
			&cli.BoolFlag{Name: "dry-run", Aliases: []string{"n"}, Usage: "preview version bump without writing files"},
			&cli.StringSliceFlag{Name: "file", Usage: "additional version file to update"},
			jsonFlag(),
		},
		Action: func(c *cli.Context) error {
			cwd, _ := os.Getwd()

			// If no arguments, display current version
			if c.NArg() == 0 || c.Args().Get(0) == "current" {
				cur := version.DetectCurrentVersion(cwd)
				if c.Bool("json") {
					return json.NewEncoder(os.Stdout).Encode(map[string]string{"version": cur})
				}
				fmt.Printf("Current version: %s\n", cur)
				return nil
			}

			bumpType := c.Args().Get(0)
			dryRun := c.Bool("dry-run")
			createTag := c.Bool("tag")
			customFiles := c.StringSlice("file")

			res, err := version.Bump(cwd, bumpType, dryRun, createTag, customFiles)
			if err != nil {
				return err
			}

			if c.Bool("json") {
				return json.NewEncoder(os.Stdout).Encode(res)
			}

			if res.DryRun {
				color.Yellow("[DRY-RUN] Version: %s -> %s (%s)", res.CurrentVersion, res.NextVersion, res.BumpType)
			} else {
				color.Green("Version bumped: %s -> %s (%s)", res.CurrentVersion, res.NextVersion, res.BumpType)
			}

			if len(res.FilesUpdated) > 0 {
				fmt.Println("Files updated:")
				for _, f := range res.FilesUpdated {
					fmt.Printf("  • %s\n", f)
				}
			}

			if res.TagCreated != "" {
				color.Cyan("Git tag: %s", res.TagCreated)
			}

			return nil
		},
	}
}
