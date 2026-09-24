package tui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Terminal styling stubs (replace with fatih/color or your chosen TUI styling library)
type style struct{}
func (s style) Sprint(a ...interface{}) string { return fmt.Sprint(a...) }
func (s style) Fprintln(w io.Writer, a ...interface{}) { fmt.Fprintln(w, a...) }
func (s style) Fprintf(w io.Writer, format string, a ...interface{}) { fmt.Fprintf(w, format, a...) }

var (
	dashDim = style{}
	dashErr = style{}
)

// DashboardBackend defines the operations the TUI can request from the core engine.
type DashboardBackend interface {
	GetVersion() string
	GetStatus() string
	PerformAction(actionKey string) error
}

type action struct {
	key   string
	title string
	desc  string
}

func dashActions() []action {
	return []action{
		{"health", "Check System Health", "Verify SQLite and Extractor status"},
		{"sync", "Sync Context State", "Push/Pull the latest codebase context"},
	}
}

func printDashHeader(out io.Writer, backend DashboardBackend) {
	fmt.Fprintf(out, "\n=== CTX Engine v%s ===\n", backend.GetVersion())
	fmt.Fprintf(out, "Status: %s\n\n", backend.GetStatus())
}

func runDashAction(out io.Writer, in *bufio.Reader, backend DashboardBackend, key string) error {
	fmt.Fprintf(out, "\n[System] Executing %s...\n", key)
	return backend.PerformAction(key)
}

func readLine(in *bufio.Reader) string {
	str, _ := in.ReadString('\n')
	return str
}

// RunDashboard launches the interactive terminal UI loop.
func RunDashboard(backend DashboardBackend) error {
	in := bufio.NewReader(os.Stdin)
	out := os.Stdout
	actions := dashActions()

	fmt.Println("[System] Dashboard initialized.")
	fmt.Println("[System] Awaiting context extraction data...")

	for {
		printDashHeader(out, backend)
		for i, a := range actions {
			fmt.Fprintf(out, "  %d) %s — %s\n", i+1, a.title, dashDim.Sprint(a.desc))
		}
		fmt.Fprintf(out, "  q) Quit\n")

		choice := strings.ToLower(strings.TrimSpace(readLine(in)))
		if choice == "q" || choice == "quit" || choice == "8" {
			fmt.Fprintln(out, "Bye.")
			return nil
		}

		idx, err := strconv.Atoi(choice)
		if err != nil || idx < 1 || idx > len(actions) {
			dashErr.Fprintln(out, "Pick a number 1-"+strconv.Itoa(len(actions))+" or q.")
			continue
		}

		if err := runDashAction(out, in, backend, actions[idx-1].key); err != nil {
			dashErr.Fprintf(out, "Error: %v\n", err)
		}

		fmt.Fprintln(out, dashDim.Sprint("— press Enter to continue —"))
		_, _ = in.ReadString('\n')
	}
}