package commands

import (
	"github.com/urfave/cli/v2"
)

// jsonFlag returns a fresh --json flag for commands with machine-readable output.
func jsonFlag() cli.Flag {
	return &cli.BoolFlag{Name: "json", Usage: "machine-readable JSON output"}
}
