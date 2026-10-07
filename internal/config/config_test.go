package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
