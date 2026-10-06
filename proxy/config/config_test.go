package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigReadsProxyAndEnvironmentValues(test *testing.T) {
	path := filepath.Join(test.TempDir(), "config.yaml")
	contents := "proxy:\n  address: 127.0.0.1\n  port: 28080\nbots:\n  simple_bot:\n    url: http://simple_bot:28080\n"
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		test.Fatalf("write config: %v", err)
	}
	test.Setenv("ADMIN_USER", "admin")
	test.Setenv("ADMIN_PASSWORD", "secret")

	config, err := LoadConfig(path)
	if err != nil {
		test.Fatalf("LoadConfig() error = %v", err)
	}
	if config.Proxy.Address != "127.0.0.1" || config.Proxy.Port != 28080 || config.Bots["simple_bot"].URL != "http://simple_bot:28080" {
		test.Errorf("unexpected config: %+v", config)
	}
	if config.AdminUser != "admin" || config.AdminPass != "secret" {
		test.Errorf("environment settings = %q/%q", config.AdminUser, config.AdminPass)
	}
}

func TestLoadConfigReturnsFileAndYAMLErrors(test *testing.T) {
	if _, err := LoadConfig(filepath.Join(test.TempDir(), "missing.yaml")); err == nil {
		test.Error("LoadConfig() accepted a missing file")
	}
	path := filepath.Join(test.TempDir(), "invalid.yaml")
	if err := os.WriteFile(path, []byte("proxy: ["), 0600); err != nil {
		test.Fatalf("write config: %v", err)
	}
	if _, err := LoadConfig(path); err == nil {
		test.Error("LoadConfig() accepted invalid YAML")
	}
}
