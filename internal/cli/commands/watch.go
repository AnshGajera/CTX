package commands

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	projctx "github.com/ctxdev/ctx/internal/context"
	"github.com/ctxdev/ctx/internal/engine"
	"github.com/ctxdev/ctx/internal/versioning"
	"github.com/ctxdev/ctx/internal/watcher"
	"github.com/urfave/cli/v2"
)

// WatchCommand implements `ctx watch`.
func WatchCommand() *cli.Command {
	return &cli.Command{
		Name:  "watch",
		Usage: "Watch files and re-extract on change",
		Flags: []cli.Flag{
			&cli.DurationFlag{Name: "debounce", Value: 5 * time.Second, Usage: "debounce interval"},
			&cli.BoolFlag{Name: "auto-push", Usage: "auto-commit snapshots"},
		},
		Action: func(c *cli.Context) error {
			cwd, _ := os.Getwd()
			manifest, err := projctx.LoadManifest(filepath.Join(cwd, ".ctx", "manifest.json"))
			if err != nil {
				return err
			}
			engine := engine.NewExtractionEngine(cwd, &manifest.Profile)
			store := versioning.NewContextStore(filepath.Join(cwd, ".ctx"))
			w := watcher.NewWatcher(cwd, engine, store, c.Duration("debounce"))
			w.SetAutoPush(c.Bool("auto-push"))
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()
			return w.Watch(ctx)
		},
	}
}
