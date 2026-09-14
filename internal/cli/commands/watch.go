package commands

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/AnshGajera/CTX/internal/config"
	projctx "github.com/AnshGajera/CTX/internal/context"
	"github.com/AnshGajera/CTX/internal/engine"
	"github.com/AnshGajera/CTX/internal/versioning"
	"github.com/AnshGajera/CTX/internal/watcher"
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
			cfg, _ := config.Load(config.ProjectConfigPath(cwd))
			engine := engine.NewExtractionEngine(cwd, &manifest.Profile, cfg.Core.MLURL)
			store := versioning.NewContextStore(filepath.Join(cwd, ".ctx"))
			w := watcher.NewWatcher(cwd, engine, store, c.Duration("debounce"))
			w.SetAutoPush(c.Bool("auto-push"))
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()
			return w.Watch(ctx)
		},
	}
}
