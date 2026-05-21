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

	run, err := client.CreateRun(ctx, &runtimev1.CreateRunRequest{
		EnvId:       envID,
		AgentId:     agentID,
		Name:        "go-sdk-runtime-example",
		TriggerType: runtimev1.TriggerType_TRIGGER_TYPE_MANUAL,
	})
	must("create run", err)

	action, err := client.CreateAction(ctx, &runtimev1.CreateActionRequest{
		RunId:      run.GetId(),
		EnvId:      envID,
		AgentId:    agentID,
		ActionType: runtimev1.ActionType_ACTION_TYPE_LLM,
		Connector:  "openai",
		Method:     "chat.completions",
	})
	must("create action", err)

	err = client.WithSpan(ctx, &runtimev1.OpenSpanRequest{
		Span: &runtimev1.SpanInput{
			EnvId:     envID,
			AgentId:   agentID,
			RunId:     run.GetId(),
			ActionId:  action.GetId(),
			Name:      "runtime-example-span",
			Kind:      runtimev1.SpanKind_SPAN_KIND_ACTION,
			Source:    runtimev1.SpanSource_SPAN_SOURCE_REPORT,
			Connector: "openai",
		},
	}, func(context.Context, *runtimev1.SpanRow) error {
		return nil
	})
	must("with span", err)

	status := runtimev1.RunStatus_RUN_STATUS_COMPLETED
	_, err = client.UpdateRun(ctx, &runtimev1.UpdateRunRequest{
		Id:     run.GetId(),
		Update: &runtimev1.RunUpdate{Status: &status},
	})
	must("complete run", err)

	report, err := client.Runtime.GetSpendReport(ctx, connect.NewRequest(&runtimev1.GetSpendReportRequest{
		Filter: &runtimev1.SpendFilter{EnvId: envID, AgentId: agentID},
	}))
	must("get spend report", err)
	fmt.Printf("run=%s action=%s report=%v\n", run.GetId(), action.GetId(), report.Msg)
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
