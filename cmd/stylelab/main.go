package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"stylelab/internal/config"
	"stylelab/internal/httpapi"
	"stylelab/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	st, err := store.Open(cfg.DataDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer st.Close()

	handler := httpapi.New(st, cfg)
	log.Fatal(http.ListenAndServe(cfg.Addr, handler))
}
