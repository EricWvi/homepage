package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func write(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const validOIDC = `
public_url: https://home.test
oidc:
  issuer: https://auth.test
  client_id: homepage
  client_secret: secret
`

func TestLoadRequiresAuthentication(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err == nil || !strings.Contains(err.Error(), "public_url") {
		t.Fatalf("err = %v, want public_url error", err)
	}
}

func TestLoadOIDC(t *testing.T) {
	cfg, err := Load(write(t, "listen: \":9000\"\n"+validOIDC))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		Listen:    ":9000",
		DataDir:   "./data",
		IdleWait:  Duration(5 * time.Minute),
		PublicURL: "https://home.test",
		OIDC:      OIDC{Issuer: "https://auth.test", ClientID: "homepage", ClientSecret: "secret"},
	}
	if cfg != want {
		t.Fatalf("got %+v, want %+v", cfg, want)
	}
}

func TestLoadClientSecretFromEnv(t *testing.T) {
	t.Setenv(ClientSecretEnv, "from-env")
	cfg, err := Load(write(t, strings.Replace(validOIDC, "  client_secret: secret\n", "", 1)))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OIDC.ClientSecret != "from-env" {
		t.Fatalf("secret = %q", cfg.OIDC.ClientSecret)
	}
}

func TestLoadDevUserNeedsNoOIDC(t *testing.T) {
	if _, err := Load(write(t, "dev_user: eric\n")); err != nil {
		t.Fatal(err)
	}
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	// A misplaced dev_user under oidc must not silently fall back to OIDC.
	_, err := Load(write(t, validOIDC+"  dev_user: eric\n"))
	if err == nil || !strings.Contains(err.Error(), "dev_user") {
		t.Fatalf("err = %v, want unknown dev_user error", err)
	}
}

func TestLoadEmptyFile(t *testing.T) {
	_, err := Load(write(t, ""))
	if err == nil || !strings.Contains(err.Error(), "public_url") {
		t.Fatalf("err = %v, want public_url error", err)
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	for _, content := range []string{
		"data_dir: \"\"\ndev_user: eric\n",
		strings.Replace(validOIDC, "https://home.test", "https://home.test/sub", 1),
		strings.Replace(validOIDC, "https://home.test", "home.test", 1),
		strings.Replace(validOIDC, "  issuer: https://auth.test\n", "", 1),
	} {
		if _, err := Load(write(t, content)); err == nil {
			t.Errorf("expected error for:\n%s", content)
		}
	}
}

func TestLoadIdleWait(t *testing.T) {
	for in, want := range map[string]time.Duration{
		"30s":   30 * time.Second,
		"10m":   10 * time.Minute,
		"2h":    2 * time.Hour,
		"1h30m": 90 * time.Minute,
	} {
		cfg, err := Load(write(t, "dev_user: eric\nidle_wait: "+in+"\n"))
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if time.Duration(cfg.IdleWait) != want {
			t.Errorf("%s: got %v, want %v", in, time.Duration(cfg.IdleWait), want)
		}
	}
	for _, in := range []string{"0s", "5", "500ms", "1.5h", "-5m", "5 m", "abc"} {
		if _, err := Load(write(t, "dev_user: eric\nidle_wait: "+in+"\n")); err == nil || !strings.Contains(err.Error(), "duration") {
			t.Errorf("%s: err = %v, want duration error", in, err)
		}
	}
}
