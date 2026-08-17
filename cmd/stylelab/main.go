package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"stylelab/internal/config"
	"stylelab/internal/extract"
	"stylelab/internal/httpapi"
	"stylelab/internal/job"
	"stylelab/internal/llm"
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

	llmClient := &llm.Client{}
	runner := job.NewRunner(st, cfg.WorkerConcurrency)
	runner.Register(job.KindExtract, extract.JobHandler(st, llmClient, cfg.MasterKey))
	if _, err := runner.RecoverInterrupted(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner.Start(ctx)

	handler := httpapi.New(st, cfg, runner)
	log.Fatal(http.ListenAndServe(cfg.Addr, handler))
}
