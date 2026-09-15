package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/AnshGajera/CTX/internal/config"
	csync "github.com/AnshGajera/CTX/internal/sync"
	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
)

// ShareCommand implements `ctx share`.
func ShareCommand() *cli.Command {
	return &cli.Command{
		Name:  "share",
		Usage: "[preview] Create a share token (needs ctx cloud backend)",
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
				color.Yellow("Token creation needs a real backend at %s: %v", cfg.Core.APIURL, err)
				return nil
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
		Usage: "[preview] Login and store credentials (needs ctx cloud backend; note: interactive password prompt echoes input, no masking)",
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
			reader := bufio.NewReader(os.Stdin)
			fmt.Print("Email: ")
			email, _ := reader.ReadString('\n')
			email = strings.TrimSpace(email)
			fmt.Print("Password: ")
			password, _ := reader.ReadString('\n')
			password = strings.TrimSpace(password)
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
