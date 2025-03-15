package main

import (
	"encoding/json"
	"fmt"
	"io"
)

// Config holds the configuration for an App.
type Config struct {
	Players []Player `json:"players"`
}

// DefaultConfiguration returns the default configuration values.
func DefaultConfiguration() Config {
	return Config{
		Players: make([]Player, 0),
	}
}

// loadConfig reads the provided configuration file or input, validates and
// returns a configuration object ready for use by the App.
func loadConfig(input io.Reader) (Config, error) {
	cfg := DefaultConfiguration()

	if err := json.NewDecoder(input).Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("failed to parse json config: %w", err)
	}

	return cfg, nil
}
