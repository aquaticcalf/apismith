package environments

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "openapi.yaml")
	if err := os.WriteFile(spec, []byte("openapi: 3.0.3\ninfo: {title: t, version: '1'}\npaths: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "environments.yaml")
	body := "listen: \":8090\"\nopenapi_spec: " + spec + "\ndefault_environment: dev\nenvironments:\n  - id: dev\n    name: DEV\n    base_url: http://localhost:8080/api/v1\n    production: false\n"
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Find("dev") == nil {
		t.Fatal("dev missing")
	}
	if cfg.Find("dev").BaseURL != "http://localhost:8080/api/v1" {
		t.Fatalf("base url %s", cfg.Find("dev").BaseURL)
	}
	pub := cfg.Public()
	if len(pub) != 1 || pub[0].HasUser {
		t.Fatalf("public: %+v", pub)
	}
}
