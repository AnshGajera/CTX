package main

import (
	"flag"
	"log"
	"os"

	"github.com/AnshGajera/CTX/internal/tui"
	// Import the package where your DashboardBackend interface is implemented
	"github.com/AnshGajera/CTX/internal/engine" 
)

const version = "0.1.6"
func main() {
	dashboardFlag := flag.Bool("dashboard", false, "Launch the interactive terminal dashboard")
	flag.Parse()

	tui.PrintBanner(version)

	if *dashboardFlag || len(os.Args) == 1 {
		// Initialize the backend dependency required by the TUI
		backend := engine.NewEngine(version) 

		// Launch the dashboard and catch any lifecycle errors
		if err := tui.RunDashboard(backend); err != nil {
			log.Fatalf("Dashboard exited with error: %v", err)
		}
		
		// Exit gracefully when the user quits the dashboard
		os.Exit(0)
	}

	// Remainder of your standard CLI subcommand routing (extract, diff, merge, etc.)
}