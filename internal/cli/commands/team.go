package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/AnshGajera/CTX/internal/config"
	csync "github.com/AnshGajera/CTX/internal/sync"
	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
)

// TeamCommand provides the website/documentation-compatible team command
// group while retaining the original top-level sync commands.
func TeamCommand() *cli.Command {
	return &cli.Command{
		Name:  "team",
		Usage: "Configure and synchronize context with a self-hosted team server",
		Subcommands: []*cli.Command{
			teamLoginCommand(),
			{
				Name:  "push",
				Usage: "Push the current context snapshot",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "message", Aliases: []string{"m"}, Value: "team push"},
				},
				Action: PushCommand().Action,
			},
			{
				Name:   "pull",
				Usage:  "Pull the latest context snapshot",
				Action: PullCommand().Action,
			},
		},
	}
}

func teamLoginCommand() *cli.Command {
	return &cli.Command{
		Name:      "login",
		Usage:     "Configure a team server URL and store an API token",
		ArgsUsage: "<server-url>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "token", Usage: "store API token directly"},
		},
		Action: func(c *cli.Context) error {
			url := strings.TrimRight(strings.TrimSpace(c.Args().First()), "/")
			if url == "" {
				return fmt.Errorf("team server URL is required")
			}
			if c.String("token") == "" {
				return fmt.Errorf("--token is required for non-interactive team login")
			}
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			cfg, err := config.Load(config.ProjectConfigPath(cwd))
			if err != nil {
				return err
			}
			cfg.Core.APIURL = url
			if err := cfg.Save(config.ProjectConfigPath(cwd)); err != nil {
				return fmt.Errorf("save team server configuration: %w", err)
			}
			if err := csync.SaveToken(c.String("token")); err != nil {
				return err
			}
			color.Green("Configured team server %s", url)
			return nil
		},
	}
}
