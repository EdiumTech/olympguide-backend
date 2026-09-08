package utils

import (
	"strings"
	"testing"
)

func validConfigEnv(t *testing.T) {
	t.Helper()
	for key, value := range map[string]string{
		"DB_PORT": "5432", "API_PORT": "8080", "REDIS_PORT": "6379", "STORAGE_SERVICE_PORT": "50051",
		"SESSION_SECRET": strings.Repeat("s", 64), "TOKEN_SECRET": strings.Repeat("t", 64),
		"BEARER_DATA_LOADER_TOKEN": strings.Repeat("b", 64), "GIN_MODE": "release", "SESSION_COOKIE_SECURE": "true",
	} {
		t.Setenv(key, value)
	}
}

func TestConfigRejectsMissingOrWeakSecrets(t *testing.T) {
	for _, key := range []string{"SESSION_SECRET", "TOKEN_SECRET", "BEARER_DATA_LOADER_TOKEN"} {
		t.Run(key, func(t *testing.T) {
			validConfigEnv(t)
			t.Setenv(key, "weak")
			if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), key) {
				t.Fatalf("expected a configuration error naming %s, got %v", key, err)
			}
		})
	}
}

func TestConfigSecureCookies(t *testing.T) {
	validConfigEnv(t)
	cfg, err := LoadConfig()
	if err != nil || !cfg.SessionSecure {
		t.Fatalf("secure cookies not configured: %v", err)
	}
	t.Setenv("SESSION_COOKIE_SECURE", "false")
	cfg, err = LoadConfig()
	if err != nil || cfg.SessionSecure {
		t.Fatalf("explicit local HTTP setting not honored: %v", err)
	}
}
