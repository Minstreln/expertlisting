// Package config loads application configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strings"
)

// Config holds the runtime configuration for the application.
type Config struct {
	DatabaseURL string
	Port        string
}

// Load reads configuration from environment variables and returns a Config.
// It returns an error if any required variable is missing.
func Load() (Config, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	// Accept "8080" or ":8080"
	if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}

	return Config{
		DatabaseURL: dsn,
		Port:        port,
	}, nil
}
