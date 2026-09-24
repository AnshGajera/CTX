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

// TagCommand implements `ctx tag [name] [target]`.
func TagCommand() *cli.Command {
	return &cli.Command{
		Name:  "tag",
		Usage: "Tag context snapshots as milestones or releases",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "delete", Aliases: []string{"d"}, Usage: "delete a tag"},
			&cli.BoolFlag{Name: "list", Aliases: []string{"l"}, Usage: "list all tags"},
			jsonFlag(),
		},
		Action: func(c *cli.Context) error {
			cwd, _ := os.Getwd()
			store := versioning.NewContextStore(filepath.Join(cwd, ".ctx"))

			// Delete tag
			if delTag := c.String("delete"); delTag != "" {
				if err := store.DeleteTag(delTag); err != nil {
					return err
				}
				if c.Bool("json") {
					return json.NewEncoder(os.Stdout).Encode(map[string]any{"deleted_tag": delTag})
				}
				color.Green("Deleted context tag %q", delTag)
				return nil
			}

			// Create tag
			if c.NArg() >= 1 && !c.Bool("list") {
				tagName := c.Args().Get(0)
				targetRef := "HEAD"
				if c.NArg() >= 2 {
					targetRef = c.Args().Get(1)
				}
				if err := store.CreateTag(tagName, targetRef); err != nil {
					return err
				}
				if c.Bool("json") {
					return json.NewEncoder(os.Stdout).Encode(map[string]any{
						"created_tag": tagName,
						"target_ref":  targetRef,
					})
				}
				color.Green("Created context tag %q at %s", tagName, targetRef)
				return nil
			}

			// List tags
			tags, err := store.ListTags()
			if err != nil {
				return err
			}

			if c.Bool("json") {
				return json.NewEncoder(os.Stdout).Encode(tags)
			}

			if len(tags) == 0 {
				fmt.Println("No context tags found")
				return nil
			}

			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"Tag", "Commit", "Message", "Age"})
			for _, t := range tags {
				msg := t.Message
				if len(msg) > 40 {
					msg = msg[:37] + "..."
				}
				ageStr := ""
				if !t.Timestamp.IsZero() {
					ageStr = ago(t.Timestamp)
				}
				table.Append([]string{t.Name, shortHash(t.Hash), msg, ageStr})
			}
			table.Render()
			return nil
		},
	}
}
