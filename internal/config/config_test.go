package config_test

import (
	"bytes"
	"testing"

	"stylelab/internal/config"
)

func TestLoadDefaultsWithInsecureKey(t *testing.T) {
	t.Setenv("STYLELAB_ADDR", "")
	t.Setenv("STYLELAB_DATA_DIR", "")
	t.Setenv("STYLELAB_MASTER_KEY", "")
	t.Setenv("STYLELAB_DEV_AUTO_LOGIN", "")
	t.Setenv("STYLELAB_WORKERS", "")
	t.Setenv("STYLELAB_DEV_INSECURE_KEY", "1")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Addr != ":8080" {
		t.Fatalf("Addr: got %q want :8080", cfg.Addr)
	}
	if cfg.DataDir != "./data" {
		t.Fatalf("DataDir: got %q want ./data", cfg.DataDir)
	}
	if cfg.WorkerConcurrency != 2 {
		t.Fatalf("WorkerConcurrency: got %d want 2", cfg.WorkerConcurrency)
	}
	if len(cfg.MasterKey) != 32 {
		t.Fatalf("MasterKey length: got %d want 32", len(cfg.MasterKey))
	}
	if !bytes.Equal(cfg.MasterKey, make([]byte, 32)) {
		t.Fatalf("MasterKey: want 32 zero bytes, got %x", cfg.MasterKey)
	}
}

func TestLoadRejectsInvalidMasterKey(t *testing.T) {
	t.Setenv("STYLELAB_DEV_INSECURE_KEY", "")
	t.Setenv("STYLELAB_MASTER_KEY", "not-64-hex")

	if _, err := config.Load(); err == nil {
		t.Fatal("expected error for invalid master key")
	}
}

func TestLoadRejectsZeroWorkers(t *testing.T) {
	t.Setenv("STYLELAB_DEV_INSECURE_KEY", "1")
	t.Setenv("STYLELAB_WORKERS", "0")

	if _, err := config.Load(); err == nil {
		t.Fatal("expected error for STYLELAB_WORKERS=0")
	}
}

func TestLoadDevAutoLogin(t *testing.T) {
	t.Setenv("STYLELAB_DEV_INSECURE_KEY", "1")
	t.Setenv("STYLELAB_DEV_AUTO_LOGIN", "1")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.DevAutoLogin {
		t.Fatal("expected DevAutoLogin true")
	}
}
