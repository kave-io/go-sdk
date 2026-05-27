package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"connectrpc.com/connect"
	runtimev1 "github.com/kave-io/kave/proto/gen/kave/runtime/v1"
	kave "github.com/kave-io/kave/sdk/go"
)

func main() {
	ctx := context.Background()
	client := kave.New(
		kave.WithAddr(env("KAVE_ADDR", "http://localhost:18080")),
		kave.WithToken(os.Getenv("KAVE_TOKEN")),
	)

	envID := os.Getenv("KAVE_ENV_ID")
	agentID := os.Getenv("KAVE_AGENT_ID")
	if envID == "" || agentID == "" {
		log.Fatal("KAVE_ENV_ID and KAVE_AGENT_ID are required")
	}

	run, err := client.CreateRun(ctx, kave.RunInput{
		EnvID:       envID,
		AgentID:     agentID,
		Name:        "go-sdk-runtime-example",
		TriggerType: kave.TriggerTypeManual,
	})
	must("create run", err)

	action, err := client.CreateAction(ctx, kave.ActionInput{
		RunID:      run.ID,
		EnvID:      envID,
		AgentID:    agentID,
		ActionType: kave.ActionTypeLLM,
		Connector:  "openai",
		Method:     "chat.completions",
	})
	must("create action", err)

	err = client.WithSpan(ctx, kave.SpanInput{
		EnvID:     envID,
		AgentID:   agentID,
		RunID:     run.ID,
		ActionID:  action.ID,
		Name:      "runtime-example-span",
		Kind:      kave.SpanKindAction,
		Source:    kave.SpanSourceReport,
		Connector: "openai",
	}, func(context.Context, *kave.Span) error {
		return nil
	})
	must("with span", err)

	completed := kave.RunStatusCompleted
	_, err = client.UpdateRun(ctx, kave.RunUpdateInput{ID: run.ID, Status: &completed})
	must("complete run", err)

	// GetSpendReport is not part of the typed surface; reach for the raw client.
	report, err := client.Raw().Runtime.GetSpendReport(ctx, connect.NewRequest(&runtimev1.GetSpendReportRequest{
		Filter: &runtimev1.SpendFilter{EnvId: envID, AgentId: agentID},
	}))
	must("get spend report", err)
	fmt.Printf("run=%s action=%s report=%v\n", run.ID, action.ID, report.Msg)
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func must(step string, err error) {
	if err != nil {
		log.Fatalf("%s: %v", step, err)
	}
}
