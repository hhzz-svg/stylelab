package config

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Addr              string
	DataDir           string
	MasterKey         []byte
	DevAutoLogin      bool
	WorkerConcurrency int
}

func Load() (Config, error) {
	cfg := Config{
		Addr:              envOr("STYLELAB_ADDR", ":8080"),
		DataDir:           envOr("STYLELAB_DATA_DIR", "./data"),
		DevAutoLogin:      os.Getenv("STYLELAB_DEV_AUTO_LOGIN") == "1",
		WorkerConcurrency: 2,
	}

	if raw := os.Getenv("STYLELAB_WORKERS"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("STYLELAB_WORKERS: %w", err)
		}
		if n < 1 {
			return Config{}, fmt.Errorf("STYLELAB_WORKERS must be >= 1")
		}
		cfg.WorkerConcurrency = n
	}

	key, err := loadMasterKey()
	if err != nil {
		return Config{}, err
	}
	cfg.MasterKey = key
	return cfg, nil
}

func loadMasterKey() ([]byte, error) {
	if os.Getenv("STYLELAB_DEV_INSECURE_KEY") == "1" {
		return make([]byte, 32), nil
	}
	raw := strings.TrimSpace(os.Getenv("STYLELAB_MASTER_KEY"))
	if len(raw) != 64 {
		return nil, fmt.Errorf("STYLELAB_MASTER_KEY must be 64 hex chars")
	}
	key, err := hex.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("STYLELAB_MASTER_KEY: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("STYLELAB_MASTER_KEY must decode to 32 bytes")
	}
	return key, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
