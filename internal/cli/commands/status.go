package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	projctx "github.com/AnshGajera/CTX/internal/context"
	"github.com/AnshGajera/CTX/internal/versioning"
	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli/v2"
)

// StatusCommand implements `ctx status`.
func StatusCommand() *cli.Command {
	return &cli.Command{
		Name:  "status",
		Usage: "Show context status",
		Flags: []cli.Flag{
			jsonFlag(),
		},
		Action: func(c *cli.Context) error {
			cwd, _ := os.Getwd()
			ctxDir := filepath.Join(cwd, ".ctx")
			ctx, err := projctx.LoadContext(cwd)
			if err != nil {
				return fmt.Errorf("no context (run ctx init): %w", err)
			}
			if c.Bool("json") {
				return json.NewEncoder(os.Stdout).Encode(ctx)
			}
			store := versioning.NewContextStore(ctxDir)
			head, _ := store.GetHead()
			nEP, nModels, nEnv, nDeps := 0, 0, 0, 0
			if ctx.APIs != nil {
				nEP = len(ctx.APIs.Endpoints)
			}
			if ctx.Database != nil {
				nModels = len(ctx.Database.Models)
			}
			if ctx.Environment != nil {
				nEnv = len(ctx.Environment.Variables)
			}
			if ctx.Dependencies != nil {
				nDeps = len(ctx.Dependencies.Direct) + len(ctx.Dependencies.Dev)
			}
			ctxBranch, detached, _ := store.CurrentBranch()
			branchLabel := ctxBranch
			if detached {
				branchLabel = "detached HEAD (" + shortHash(ctxBranch) + ")"
			}
			gitBranch := ""
			if ctx.CurrentState != nil {
				gitBranch = ctx.CurrentState.GitBranch
			}
			color.Cyan("Project: %s", ctx.ProjectName)
			fmt.Printf("Context Branch: %s  Hash: %s  Git: %s  Updated: %s ago\n",
				color.GreenString(branchLabel), shortHash(head), gitBranch, ago(ctx.ExtractedAt))
			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"Metric", "Count"})
			table.Append([]string{"Endpoints", fmt.Sprint(nEP)})
			table.Append([]string{"Models", fmt.Sprint(nModels)})
			table.Append([]string{"Env vars", fmt.Sprint(nEnv)})
			table.Append([]string{"Dependencies", fmt.Sprint(nDeps)})
			table.Render()
			// diff vs parent
			if head != "" {
				if snap, err := store.LoadSnapshot(head); err == nil && snap.ParentHash != "" {
					if parent, err := store.LoadSnapshot(snap.ParentHash); err == nil {
						oldCtx, _ := versioning.SnapshotToContext(parent)
						d := versioning.ComputeDiff(oldCtx, ctx)
						if !d.IsEmpty() {
							color.Yellow("Changes since last snapshot: %s", d.Summary)
						} else {
							fmt.Println("No changes since last snapshot")
						}
					}
				}
			}
			return nil
		},
	}
}

func shortHash(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}

func ago(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
