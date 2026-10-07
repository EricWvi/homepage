// Package config loads the YAML configuration file.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the application configuration.
type Config struct {
	// Listen is the HTTP listen address, e.g. ":8080".
	Listen string `yaml:"listen"`
	// DataDir holds the SQLite database and uploaded icon files.
	DataDir string `yaml:"data_dir"`
}

// Default returns the configuration used when no file is present.
func Default() Config {
	return Config{
		Listen:  ":8080",
		DataDir: "./data",
	}
}

// Load reads the YAML file at path on top of the defaults.
// A missing file is not an error; the defaults are returned.
func Load(path string) (Config, error) {
	cfg := Default()
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %s: %w", path, err)
	}
	if cfg.Listen == "" {
		return cfg, errors.New("config: listen must not be empty")
	}
	if cfg.DataDir == "" {
		return cfg, errors.New("config: data_dir must not be empty")
	}
	return cfg, nil
}
