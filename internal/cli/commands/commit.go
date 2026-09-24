package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	projctx "github.com/AnshGajera/CTX/internal/context"
	"github.com/AnshGajera/CTX/internal/versioning"
	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
)

// CommitCommand implements `ctx commit -m "message"` (checkpoint).
func CommitCommand() *cli.Command {
	return &cli.Command{
		Name:    "commit",
		Aliases: []string{"checkpoint", "ci"},
		Usage:   "Record a context checkpoint / milestone with a reasoning description",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "message", Aliases: []string{"m"}, Required: true, Usage: "checkpoint / milestone description"},
			jsonFlag(),
		},
		Action: func(c *cli.Context) error {
			cwd, _ := os.Getwd()
			msg := c.String("message")
			if msg == "" {
				return fmt.Errorf("commit message cannot be empty (-m \"...\")")
			}

			ctx, err := projctx.LoadContext(cwd)
			if err != nil {
				return fmt.Errorf("no context to commit (run ctx init or ctx extract first): %w", err)
			}

			store := versioning.NewContextStore(filepath.Join(cwd, ".ctx"))
			snap, err := store.CommitCheckpoint(ctx, msg)
			if err != nil {
				return err
			}

			curBranch, detached, _ := store.CurrentBranch()
			branchLabel := curBranch
			if detached {
				branchLabel = "detached HEAD"
			}

			if c.Bool("json") {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"hash":    snap.Hash,
					"branch":  branchLabel,
					"message": snap.Message,
					"author":  snap.Author,
				})
			}

			color.Green("[%s %s] %s", branchLabel, shortHash(snap.Hash), snap.Message)
			return nil
		},
	}
}
