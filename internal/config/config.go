// Package config loads the YAML configuration file.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"

	"gopkg.in/yaml.v3"
)

// ClientSecretEnv overrides oidc.client_secret so the secret can stay out
// of the config file.
const ClientSecretEnv = "HOMEPAGE_OIDC_CLIENT_SECRET"

// Config is the application configuration.
type Config struct {
	// Listen is the HTTP listen address, e.g. ":36749".
	Listen string `yaml:"listen"`
	// DataDir holds the SQLite database and uploaded icon files.
	DataDir string `yaml:"data_dir"`
	// PublicURL is the origin browsers use to reach the site, e.g.
	// "https://home.example.com". It forms the OIDC redirect URI.
	PublicURL string `yaml:"public_url"`
	OIDC      OIDC   `yaml:"oidc"`
	// DevUser, when set, signs every request in as this local user and
	// disables OIDC. For local development only.
	DevUser string `yaml:"dev_user"`
}

// OIDC configures the identity provider (Authelia).
type OIDC struct {
	Issuer       string `yaml:"issuer"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
}

// Default returns the configuration used when no file is present.
func Default() Config {
	return Config{
		Listen:  ":36749",
		DataDir: "./data",
	}
}

// Load reads the YAML file at path on top of the defaults.
// A missing file is not an error; the defaults are used. Unknown keys are
// rejected so a misplaced setting such as dev_user cannot be ignored.
func Load(path string) (Config, error) {
	cfg := Default()
	raw, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return cfg, fmt.Errorf("read config: %w", err)
	default:
		dec := yaml.NewDecoder(bytes.NewReader(raw))
		dec.KnownFields(true)
		if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
			return cfg, fmt.Errorf("parse config %s: %w", path, err)
		}
	}
	if secret := os.Getenv(ClientSecretEnv); secret != "" {
		cfg.OIDC.ClientSecret = secret
	}
	return cfg, cfg.validate()
}

func (c Config) validate() error {
	if c.Listen == "" {
		return errors.New("config: listen must not be empty")
	}
	if c.DataDir == "" {
		return errors.New("config: data_dir must not be empty")
	}
	if c.DevUser != "" {
		return nil
	}
	u, err := url.Parse(c.PublicURL)
	if c.PublicURL == "" || err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
		(u.Path != "" && u.Path != "/") {
		return errors.New("config: public_url must be an origin such as https://home.example.com")
	}
	if c.OIDC.Issuer == "" || c.OIDC.ClientID == "" || c.OIDC.ClientSecret == "" {
		return fmt.Errorf("config: oidc.issuer, oidc.client_id and oidc.client_secret (or %s) are required unless dev_user is set", ClientSecretEnv)
	}
	return nil
}
