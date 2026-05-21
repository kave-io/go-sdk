package main

import (
	"context"
	"log"
	"os"

	kave "github.com/kave-io/go-sdk"
	runtimev1 "github.com/kave-io/kave/proto/gen/kave/runtime/v1"
)

func main() {
	ctx := context.Background()
	client := kave.New(
		kave.WithAddr(env("KAVE_ADDR", "http://localhost:18080")),
		kave.WithToken(os.Getenv("KAVE_TOKEN")),
	)

	envID := os.Getenv("KAVE_ENV_ID")
	if envID == "" {
		log.Fatal("KAVE_ENV_ID is required")
	}

	for run, err := range client.WatchRuns(ctx, &runtimev1.WatchRunsRequest{EnvId: envID}) {
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("run id=%s status=%s", run.GetId(), run.GetStatus())
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
