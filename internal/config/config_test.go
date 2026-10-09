package config

import "testing"

func TestLoad(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when DATABASE_URL is missing")
	}

	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("PORT", "9000")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 9000 || cfg.DatabaseURL != "postgres://x" {
		t.Fatalf("got %+v", cfg)
	}
}
