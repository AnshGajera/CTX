package main

import (
	"fmt"
	"os"

	"github.com/AnshGajera/CTX/internal/cli"
)

func main() {
	app := cli.NewApp()
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
