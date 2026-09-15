package cli

import (
	"github.com/AnshGajera/CTX/internal/cli/commands"
	"github.com/AnshGajera/CTX/internal/version"
	"github.com/urfave/cli/v2"
)

// NewApp builds the ctx CLI app.
func NewApp() *cli.App {
	return &cli.App{
		Name:    "ctx",
		Usage:   "Context engine for software development",
		Version: version.Version,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "config", Usage: "config file path"},
			&cli.BoolFlag{Name: "verbose", Aliases: []string{"V"}, Usage: "verbose output"},
			&cli.BoolFlag{Name: "json", Usage: "machine-readable JSON output"},
		},
		Commands: []*cli.Command{
			commands.InitCommand(),
			commands.ExtractCommand(),
			commands.StatusCommand(),
			commands.PushCommand(),
			commands.PullCommand(),
			commands.DiffCommand(),
			commands.LogCommand(),
			commands.ServeCommand(),
			commands.WatchCommand(),
			commands.ShareCommand(),
			commands.LoginCommand(),
			commands.SearchCommand(),
			commands.EvalCommand(),
			commands.ExportCommand(),
			commands.DashboardCommand(),
		},
	}
}
