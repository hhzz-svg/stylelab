package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"stylelab/internal/audit"
	"stylelab/internal/bible"
	"stylelab/internal/config"
	"stylelab/internal/extract"
	"stylelab/internal/fuse"
	"stylelab/internal/httpapi"
	"stylelab/internal/job"
	"stylelab/internal/llm"
	"stylelab/internal/sample"
	"stylelab/internal/store"
	"stylelab/internal/write"
	"stylelab/web"
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
	runner.Register(job.KindFuse, fuse.JobHandler(st, llmClient, cfg.MasterKey))
	runner.Register(job.KindAudit, audit.JobHandler(st, llmClient, cfg.MasterKey))
	runner.Register(job.KindSample, sample.JobHandler(st, llmClient, cfg.MasterKey))
	runner.Register(job.KindWrite, write.JobHandler(st, llmClient, cfg.MasterKey))
	runner.Register(job.KindBibleSync, bible.JobHandler(st, llmClient, cfg.MasterKey))
	if _, err := runner.RecoverInterrupted(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner.Start(ctx)

	handler := web.Handler(httpapi.New(st, cfg, runner))
	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: handler,
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		log.Println("StyleLab shutting down gracefully...")
		cancel()
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutCancel()
		_ = srv.Shutdown(shutCtx)
	}()

	log.Printf("StyleLab listening on %s", cfg.Addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("StyleLab server failed: %v", err)
	}
}
