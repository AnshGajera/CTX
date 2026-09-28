package tui

import (
	"fmt"
	"os"

	"github.com/fatih/color"
)

var bannerLines = []string{
	` ██████╗████████╗██╗  ██╗`,
	`██╔════╝╚══██╔══╝╚██╗██╔╝`,
	`██║        ██║    ╚███╔╝ `,
	`██║        ██║    ██╔██╗ `,
	`╚██████╗   ██║   ██╔╝ ██╗`,
	` ╚═════╝   ╚═╝   ╚═╝  ╚═╝`,
}

// PrintBanner prints the ctx banner and tagline (human output only;
// callers must skip it in --json / non-TTY mode).
func PrintBanner(version string) {
	fmt.Println("CTX Engine ", version)
	cyan := color.New(color.FgCyan, color.Bold)
	for _, l := range bannerLines {
		cyan.Println(l)
	}
	dim := color.New(color.Faint)
	dim.Println("  Context engine for software development")
	fmt.Println()
}

// IsTerminal reports whether f is a character device (interactive shell).
func IsTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
