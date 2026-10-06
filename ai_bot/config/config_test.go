package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(test *testing.T) {
	path := filepath.Join(test.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("ai_bot:\n  address: 127.0.0.1\n  port: 8083\n"), 0600); err != nil {
		test.Fatalf("write config: %v", err)
	}
	config, err := LoadConfig(path)
	if err != nil || config.BotCfg.Address != "127.0.0.1" || config.BotCfg.Port != 8083 {
		test.Fatalf("LoadConfig() = %+v, %v", config, err)
	}
}

func TestLoadConfigRejectsMissingAndInvalidFiles(test *testing.T) {
	if _, err := LoadConfig(filepath.Join(test.TempDir(), "missing.yaml")); err == nil {
		test.Error("LoadConfig() accepted a missing file")
	}
	path := filepath.Join(test.TempDir(), "invalid.yaml")
	if err := os.WriteFile(path, []byte("ai_bot: ["), 0600); err != nil {
		test.Fatalf("write config: %v", err)
	}
	if _, err := LoadConfig(path); err == nil {
		test.Error("LoadConfig() accepted invalid YAML")
	}
}
