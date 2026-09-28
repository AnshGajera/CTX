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

// CheckoutCommand implements `ctx checkout <branch|hash> [-b new_branch]`.
func CheckoutCommand() *cli.Command {
	return &cli.Command{
		Name:    "checkout",
		Aliases: []string{"co"},
		Usage:   "Switch active context branch or restore context snapshot",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "branch", Aliases: []string{"b"}, Usage: "create and checkout a new branch"},
			jsonFlag(),
		},
		Action: func(c *cli.Context) error {
			cwd, _ := os.Getwd()
			store := versioning.NewContextStore(filepath.Join(cwd, ".ctx"))

			newBranch := c.String("branch")
			if newBranch != "" {
				startRef := "HEAD"
				if c.NArg() >= 1 {
					startRef = c.Args().Get(0)
				}
				snap, err := store.CheckoutNewBranch(newBranch, startRef)
				if err != nil {
					return err
				}
				if c.Bool("json") {
					return json.NewEncoder(os.Stdout).Encode(map[string]any{
						"branch":   newBranch,
						"snapshot": snap.Hash,
						"message":  snap.Message,
					})
				}
				color.Green("Switched to a new context branch %q (HEAD at %s)", newBranch, shortHash(snap.Hash))
				return nil
			}

			if c.NArg() < 1 {
				return fmt.Errorf("checkout requires a branch name or snapshot hash (e.g. ctx checkout main)")
			}

			target := c.Args().Get(0)
			snap, err := store.Checkout(target)
			if err != nil {
				return err
			}

			curBranch, detached, _ := store.CurrentBranch()
			if c.Bool("json") {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"target":      target,
					"branch":      curBranch,
					"is_detached": detached,
					"snapshot":    snap.Hash,
					"message":     snap.Message,
				})
			}

			if detached {
				color.Yellow("Note: switching to %s (detached HEAD mode)", shortHash(snap.Hash))
			} else {
				color.Green("Switched to context branch %q", curBranch)
			}
			fmt.Printf("HEAD is now at %s (%s)\n", shortHash(snap.Hash), snap.Message)
			return nil
		},
	}
}
