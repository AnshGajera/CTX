package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ctxdev/ctx/internal/config"
	projctx "github.com/ctxdev/ctx/internal/context"
	csync "github.com/ctxdev/ctx/internal/sync"
	"github.com/ctxdev/ctx/internal/versioning"
	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
)

// PushCommand implements `ctx push`.
func PushCommand() *cli.Command {
	return &cli.Command{
		Name:  "push",
		Usage: "Push context to remote",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "message", Aliases: []string{"m"}, Usage: "push message"},
		},
		Action: func(c *cli.Context) error {
			msg := c.String("message")
			if msg == "" {
				return fmt.Errorf("push message required (-m)")
			}
			cwd, _ := os.Getwd()
			ctxDir := filepath.Join(cwd, ".ctx")
			ctx, err := projctx.LoadContext(cwd)
			if err != nil {
				return err
			}
			store := versioning.NewContextStore(ctxDir)
			snap, err := store.Commit(ctx, msg)
			if err != nil {
				return err
			}
			cfg, _ := config.Load(config.ProjectConfigPath(cwd))
			if cfg.Core.APIURL == "" {
				cfg = config.Default()
			}
			client := csync.NewSyncClient(cfg.Core.APIURL, csync.LoadToken())
			manifest, _ := projctx.LoadManifest(filepath.Join(ctxDir, "manifest.json"))
			projectID := ""
			if manifest != nil {
				projectID = manifest.ContextID
			}
			if err := client.Push(projectID, snap); err != nil {
				color.Yellow("Committed locally (%s); remote push failed: %v", snap.Hash, err)
				color.Yellow("Remote sync needs a real backend at %s", cfg.Core.APIURL)
				return nil
			}
			color.Green("Pushed %s", snap.Hash)
			return nil
		},
	}
}

// PullCommand implements `ctx pull`.
func PullCommand() *cli.Command {
	return &cli.Command{
		Name:  "pull",
		Usage: "Pull latest context from remote",
		Action: func(c *cli.Context) error {
			cwd, _ := os.Getwd()
			ctxDir := filepath.Join(cwd, ".ctx")
			oldCtx, _ := projctx.LoadContext(cwd)
			cfg, _ := config.Load(config.ProjectConfigPath(cwd))
			if cfg.Core.APIURL == "" {
				cfg = config.Default()
			}
			client := csync.NewSyncClient(cfg.Core.APIURL, csync.LoadToken())
			manifest, _ := projctx.LoadManifest(filepath.Join(ctxDir, "manifest.json"))
			projectID := ""
			if manifest != nil {
				projectID = manifest.ContextID
			}
			store := versioning.NewContextStore(ctxDir)
			snap, err := csync.PullSnapshot(client, store, ctxDir, projectID)
			if err != nil {
				return fmt.Errorf("pull failed (backend %s may not exist yet): %w", cfg.Core.APIURL, err)
			}
			newCtx, _ := versioning.SnapshotToContext(snap)
			d := versioning.ComputeDiff(oldCtx, newCtx)
			color.Green("Pulled %s: %s", snap.Hash, d.Summary)
			return nil
		},
	}
}
