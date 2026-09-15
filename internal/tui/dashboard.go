package tui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/fatih/color"
)

// DashboardBackend supplies data and actions; implemented by the CLI layer
// so this package stays free of engine/versioning imports.
type DashboardBackend interface {
	Title() string
	Status() (string, error)
	Extract() (string, error)
	Search(query string) (string, error)
	Diff() (string, error)
	History() (string, error)
	ToggleServe(port int) (string, error)
	Export() (string, error)
}

var (
	dashTitle = color.New(color.FgCyan, color.Bold)
	dashDim   = color.New(color.Faint)
	dashErr   = color.New(color.FgRed)
	dashOK    = color.New(color.FgGreen)
)

type dashAction struct {
	key, title, desc string
}

func dashActions() []dashAction {
	return []dashAction{
		{key: "extract", title: "Extract now", desc: "Re-scan the repo and snapshot context"},
		{key: "search", title: "Search context", desc: "Semantic search over the project"},
		{key: "diff", title: "View changes", desc: "Diff HEAD~1 against HEAD"},
		{key: "history", title: "History", desc: "Browse snapshot timeline"},
		{key: "serve", title: "Serve MCP", desc: "Start the MCP/HTTP server (runs until quit)"},
		{key: "export", title: "Export OpenAPI", desc: "Write openapi.json for this project"},
		{key: "status", title: "Refresh status", desc: "Reload project summary"},
	}
}

// RunDashboard runs a plain stdin/stdout task menu. It uses no raw-mode
// console APIs, so it works everywhere the CLI runs (including locked-down
// Windows consoles). Requires a TTY.
func RunDashboard(backend DashboardBackend) error {
	in := bufio.NewReader(os.Stdin)
	out := os.Stdout
	actions := dashActions()
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

func printDashHeader(out io.Writer, backend DashboardBackend) {
	fmt.Fprintln(out)
	dashTitle.Fprintln(out, "ctx dashboard")
	dashDim.Fprintln(out, backend.Title())
	fmt.Fprintln(out)
}

func readLine(in *bufio.Reader) string {
	line, err := in.ReadString('\n')
	if err != nil {
		return ""
	}
	return line
}

func runDashAction(out io.Writer, in *bufio.Reader, backend DashboardBackend, key string) error {
	var title, body string
	var err error
	switch key {
	case "extract":
		fmt.Fprintln(out, "Extracting…")
		body, err = backend.Extract()
		title = "Extract"
	case "search":
		fmt.Fprint(out, "Query: ")
		q := strings.TrimSpace(readLine(in))
		if q == "" {
			return nil
		}
		body, err = backend.Search(q)
		title = "Search: " + q
	case "diff":
		body, err = backend.Diff()
		title = "Changes (HEAD~1..HEAD)"
	case "history":
		body, err = backend.History()
		title = "History"
	case "serve":
		fmt.Fprint(out, "Port [3100]: ")
		port := 3100
		if line := strings.TrimSpace(readLine(in)); line != "" {
			if _, serr := fmt.Sscanf(line, "%d", &port); serr != nil || port <= 0 {
				return fmt.Errorf("bad port %q", line)
			}
		}
		body, err = backend.ToggleServe(port)
		title = "Serve"
	case "export":
		body, err = backend.Export()
		title = "Export"
	case "status":
		body, err = backend.Status()
		title = "Status"
	default:
		return fmt.Errorf("unknown action %q", key)
	}
	if err != nil {
		return err
	}
	fmt.Fprintln(out)
	dashOK.Fprintln(out, "— "+title+" —")
	fmt.Fprintln(out, body)
	return nil
}
