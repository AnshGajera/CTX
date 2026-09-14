package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ctxdev/ctx/internal/mcp"
	"github.com/ctxdev/ctx/internal/versioning"
	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
)

// ServeCommand implements `ctx serve`.
func ServeCommand() *cli.Command {
	return &cli.Command{
		Name:  "serve",
		Usage: "Serve context over MCP (HTTP or stdio)",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "port", Value: 3100, Usage: "http port"},
			&cli.BoolFlag{Name: "stdio", Usage: "MCP over stdio"},
		},
		Action: func(c *cli.Context) error {
			cwd, _ := os.Getwd()
			store := versioning.NewContextStore(filepath.Join(cwd, ".ctx"))
			if c.Bool("stdio") {
				srv := mcp.NewStdioServer(cwd, store)
				return srv.Serve()
			}
			port := c.Int("port")
			srv := mcp.NewMCPServer(cwd, store, port)
			color.Green("Serving MCP on http://localhost:%d", port)
			fmt.Println("Cursor: add http://localhost:" + fmt.Sprint(port) + "/mcp as MCP server")
			fmt.Println("Claude Desktop: use `ctx serve --stdio` in mcpServers config")
			return srv.Start()
		},
	}
}
