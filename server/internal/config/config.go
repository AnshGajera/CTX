package config

// Config holds all server configuration.
type Config struct {
	Bind        string `json:"bind"`
	Port        int    `json:"port"`
	DatabaseURL string `json:"database_url"`
	JWTSecret   string `json:"jwt_secret"`
	Debug       bool   `json:"debug"`

	// CORS
	AllowedOrigins []string `json:"allowed_origins"`

	// Rate limiting
	RateLimit    int `json:"rate_limit"`    // requests per minute
	RateBurst    int `json:"rate_burst"`    // burst capacity

	// Context limits
	MaxContextSize int64 `json:"max_context_size"` // max snapshot size in bytes
	MaxSnapshots   int   `json:"max_snapshots"`    // max snapshots per project
}

// DefaultConfig returns sensible defaults for self-hosting.
func DefaultConfig() *Config {
	return &Config{
		Bind:           "127.0.0.1",
		Port:           3200,
		DatabaseURL:    "ctx-server.db",
		JWTSecret:      "change-me-in-production",
		Debug:          false,
		AllowedOrigins: []string{"*"},
		RateLimit:      60,
		RateBurst:      10,
		MaxContextSize: 10 * 1024 * 1024, // 10 MiB
		MaxSnapshots:   100,
	}
}
