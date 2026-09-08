package config

import "testing"

func TestPublicStorageURLDoesNotExposeInternalPort(t *testing.T) {
	t.Setenv("STORAGE_SERVICE_PORT", "50051")
	t.Setenv("MINIO_PORT", "9000")
	t.Setenv("PUBLIC_STORAGE_URL", "https://api.olympguide.ru/")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PublicStorageURL != "https://api.olympguide.ru" {
		t.Fatalf("unexpected public URL: %s", cfg.PublicStorageURL)
	}
	t.Setenv("PUBLIC_STORAGE_URL", "api.olympguide.ru")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("accepted a public URL without HTTPS/HTTP scheme")
	}
}
