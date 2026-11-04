package oauth

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigFromEnvReadsDirectCredentials(t *testing.T) {
	setOAuthEnv(t, "http://127.0.0.1:28080/auth/google/callback")
	t.Setenv("GOOGLE_CLIENT_ID", "client-id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "client-secret")

	flow, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("LoadConfigFromEnv() error = %v", err)
	}
	if flow.Config.ClientID != "client-id" || flow.Config.ClientSecret != "client-secret" {
		t.Fatalf("unexpected client credentials: ID=%q secret configured=%v", flow.Config.ClientID, flow.Config.ClientSecret != "")
	}
	if flow.SecureCookie() {
		t.Fatal("127.0.0.1 HTTP callback should not require a Secure cookie when IS_PROD=false")
	}
}

func TestLoadConfigFromEnvReadsConfiguredCredentialsFile(t *testing.T) {
	setOAuthEnv(t, "https://proxy.example/auth/google/callback")
	path := filepath.Join(t.TempDir(), "google-client.json")
	if err := os.WriteFile(path, []byte(`{"web":{"client_id":"file-id","client_secret":"file-secret"}}`), 0600); err != nil {
		t.Fatalf("write credentials: %v", err)
	}
	t.Setenv("GOOGLE_CREDENTIALS_JSON", path)
	t.Setenv("GOOGLE_CLIENT_ID", "ignored-id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "ignored-secret")

	flow, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("LoadConfigFromEnv() error = %v", err)
	}
	if flow.Config.ClientID != "file-id" || flow.Config.ClientSecret != "file-secret" {
		t.Fatal("credentials file was not used")
	}
	if !flow.SecureCookie() {
		t.Fatal("production callback should use a Secure cookie")
	}
}

func TestLoadConfigFromEnvFailsWhenCredentialsAreMissing(t *testing.T) {
	setOAuthEnv(t, "http://127.0.0.1:28080/auth/google/callback")
	t.Setenv("GOOGLE_CLIENT_ID", "")
	t.Setenv("GOOGLE_CLIENT_SECRET", "")
	if _, err := LoadConfigFromEnv(); err == nil {
		t.Fatal("LoadConfigFromEnv() succeeded without credentials")
	}
}

func TestNewStateEncodesThirtyTwoRandomBytes(t *testing.T) {
	state, err := NewState()
	if err != nil {
		t.Fatalf("NewState() error = %v", err)
	}
	decoded, err := base64.URLEncoding.DecodeString(state)
	if err != nil {
		t.Fatalf("decode state: %v", err)
	}
	if len(decoded) != 32 {
		t.Fatalf("decoded state length = %d; want 32", len(decoded))
	}
}

func setOAuthEnv(t *testing.T, redirectURL string) {
	t.Helper()
	t.Setenv("GOOGLE_REDIRECT_URL", redirectURL)
	t.Setenv("FRONTEND_URL", "http://127.0.0.1:8080")
	t.Setenv("GOOGLE_CREDENTIALS_JSON", "")
	t.Setenv("IS_PROD", "false")
}
