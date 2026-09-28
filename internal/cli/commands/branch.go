package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AnshGajera/CTX/internal/versioning"
	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli/v2"
)

// BranchCommand implements `ctx branch [name]`.
func BranchCommand() *cli.Command {
	return &cli.Command{
		Name:    "branch",
		Aliases: []string{"br"},
		Usage:   "Manage context branches (list, create, delete)",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "delete", Aliases: []string{"d"}, Usage: "delete branch by name"},
			jsonFlag(),
		},
		Action: func(c *cli.Context) error {
			cwd, _ := os.Getwd()
			store := versioning.NewContextStore(filepath.Join(cwd, ".ctx"))

			// Handle branch deletion
			delName := c.String("delete")
			if delName != "" {
				if err := store.DeleteBranch(delName); err != nil {
					return err
				}
				if c.Bool("json") {
					return json.NewEncoder(os.Stdout).Encode(map[string]any{"deleted": delName})
				}
				color.Green("Deleted context branch %q", delName)
				return nil
			}

			// Handle branch creation
			if c.NArg() >= 1 {
				branchName := c.Args().Get(0)
				startPoint := "HEAD"
				if c.NArg() >= 2 {
					startPoint = c.Args().Get(1)
				}
				if err := store.CreateBranch(branchName, startPoint); err != nil {
					return err
				}
				if c.Bool("json") {
					return json.NewEncoder(os.Stdout).Encode(map[string]any{
						"created":     branchName,
						"start_point": startPoint,
					})
				}
				color.Green("Created context branch %q at %s", branchName, startPoint)
				return nil
			}

			// List branches
			branches, err := store.ListBranches()
			if err != nil {
				return err
			}

			if c.Bool("json") {
				return json.NewEncoder(os.Stdout).Encode(branches)
			}

			if len(branches) == 0 {
				fmt.Println("No context branches found (run ctx extract first)")
				return nil
			}

			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"Current", "Branch", "Commit", "Message", "Age"})
			for _, b := range branches {
				marker := " "
				nameStr := b.Name
				if b.IsCurrent {
					marker = "*"
					nameStr = color.GreenString(b.Name)
				}
				msg := b.Message
				if len(msg) > 40 {
					msg = msg[:37] + "..."
				}
				ageStr := ""
				if !b.Timestamp.IsZero() {
					ageStr = ago(b.Timestamp)
				}
				table.Append([]string{marker, nameStr, shortHash(b.Hash), msg, ageStr})
			}
			table.Render()
			return nil
		},
	}
}
