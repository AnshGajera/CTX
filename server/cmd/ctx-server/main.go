package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/AnshGajera/CTX/server/internal/api"
	"github.com/AnshGajera/CTX/server/internal/config"
	"github.com/AnshGajera/CTX/server/internal/store"
)

func main() {
	cfg := config.DefaultConfig()

	flag.StringVar(&cfg.Bind, "bind", cfg.Bind, "listen address")
	flag.IntVar(&cfg.Port, "port", cfg.Port, "listen port")
	flag.StringVar(&cfg.DatabaseURL, "db", cfg.DatabaseURL, "database path (SQLite file)")
	flag.StringVar(&cfg.JWTSecret, "jwt-secret", cfg.JWTSecret, "JWT signing secret (required in production)")
	flag.BoolVar(&cfg.Debug, "debug", cfg.Debug, "enable debug logging")
	flag.Parse()

	// Allow env overrides
	if v := os.Getenv("CTX_BIND"); v != "" {
		cfg.Bind = v
	}
	if v := os.Getenv("CTX_PORT"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.Port)
	}
	if v := os.Getenv("CTX_DB"); v != "" {
		cfg.DatabaseURL = v
	}
	if v := os.Getenv("CTX_JWT_SECRET"); v != "" {
		cfg.JWTSecret = v
	}

	if cfg.JWTSecret == "" || cfg.JWTSecret == "change-me-in-production" {
		log.Println("WARNING: Using default JWT secret. Set -jwt-secret or CTX_JWT_SECRET for production.")
	}

	db, err := store.NewSQLiteStore(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	srv := api.NewServer(cfg, db)
	addr := fmt.Sprintf("%s:%d", cfg.Bind, cfg.Port)
	log.Printf("CTX Server starting on %s", addr)
	if err := srv.ListenAndServe(addr); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
