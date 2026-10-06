package main

import (
	"os"

	"github.com/AnshGajera/CTX/internal/cli"
	"github.com/AnshGajera/CTX/internal/version"
)

func main() {
	app := cli.NewApp()
	app.Version = version.Version
	if err := app.Run(os.Args); err != nil {
		os.Exit(1)
	}
}