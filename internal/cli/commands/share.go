package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/AnshGajera/CTX/internal/config"
	csync "github.com/AnshGajera/CTX/internal/sync"
	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"
)

// ShareCommand implements `ctx share`.
func ShareCommand() *cli.Command {
	return &cli.Command{
		Name:  "share",
		Usage: "Create a share token",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "create-token", Usage: "create token via API"},
			&cli.StringFlag{Name: "name", Usage: "token name"},
			&cli.BoolFlag{Name: "read-only", Value: true, Usage: "read-only token"},
		},
		Action: func(c *cli.Context) error {
			if !c.Bool("create-token") {
				fmt.Println("Usage: ctx share --create-token --name <name>")
				return nil
			}
			cwd, _ := os.Getwd()
			cfg, _ := config.Load(config.ProjectConfigPath(cwd))
			if cfg.Core.APIURL == "" {
				cfg = config.Default()
			}
			client := csync.NewSyncClient(cfg.Core.APIURL, csync.LoadToken())
			token, err := client.CreateToken(c.String("name"), c.Bool("read-only"))
			if err != nil {
				return fmt.Errorf("create share token: %w", err)
			}
			color.Green("Token: %s", token)
			fmt.Printf("curl -H \"Authorization: Bearer %s\" %s/api/v1/projects/<id>/pull\n", token, cfg.Core.APIURL)
			return nil
		},
	}
}

// LoginCommand implements `ctx login`.
func LoginCommand() *cli.Command {
	return &cli.Command{
		Name:  "login",
		Usage: "Login and store credentials",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "token", Usage: "store API token directly (skips email/password prompt)"},
		},
		Action: func(c *cli.Context) error {
			if tok := c.String("token"); tok != "" {
				if err := csync.SaveToken(tok); err != nil {
					return err
				}
				color.Green("Logged in, credentials stored")
				return nil
			}

			fmt.Print("Email: ")
			var email string
			if _, err := fmt.Scanln(&email); err != nil {
				return fmt.Errorf("read email: %w", err)
			}
			fmt.Print("Password: ")
			passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println()
			if err != nil {
				return fmt.Errorf("read password: %w", err)
			}
			password := strings.TrimSpace(string(passwordBytes))

			cfg := config.Default()
			if cwd, err := os.Getwd(); err == nil {
				if loaded, err := config.Load(config.ProjectConfigPath(cwd)); err == nil && loaded.Core.APIURL != "" {
					cfg = loaded
				}
			}
			client := csync.NewSyncClient(cfg.Core.APIURL, "")
			token, err := client.Login(email, password)
			if err != nil {
				return fmt.Errorf("login failed: %w", err)
			}
			if err := csync.SaveToken(token); err != nil {
				return err
			}
			color.Green("Logged in, credentials stored")
			return nil
		},
	}
}
