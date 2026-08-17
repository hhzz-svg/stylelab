package main

import (
	"context"
	"encoding/json"
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
	runner.Register(job.KindExtract, func(ctx context.Context, rec job.Record, prog func(int, string)) (json.RawMessage, error) {
		var in extract.ExtractInput
		if err := json.Unmarshal(rec.Payload, &in); err != nil {
			return nil, err
		}
		in.ProjectID = rec.ProjectID
		c, err := extract.Run(ctx, st, llmClient, cfg.MasterKey, rec.UserID, in, prog)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]string{"card_id": c.ID})
	})
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
