package main

import (
	"context"
	"fmt"
	"log"
	"os"

	commonv1 "github.com/kave-io/kave/proto/gen/kave/common/v1"
	controlv1 "github.com/kave-io/kave/proto/gen/kave/control/v1"
	kave "github.com/kave-io/kave/sdk/go"
)

func main() {
	ctx := context.Background()
	client := kave.New(
		kave.WithAddr(env("KAVE_ADDR", "http://localhost:18080")),
		kave.WithToken(os.Getenv("KAVE_TOKEN")),
	)

	org, err := client.EnsureOrganization(ctx, &controlv1.CreateOrganizationRequest{
		Name: "Simorq",
		Slug: "simorq",
	})
	must("ensure org", err)

	project, err := client.EnsureProject(ctx, &controlv1.CreateProjectRequest{
		OrgId: org.GetId(),
		Name:  "simorq",
		Slug:  "simorq",
	})
	must("ensure project", err)

	envRecord, err := client.EnsureEnvironment(ctx, &controlv1.CreateEnvironmentRequest{
		ProjectId: project.GetId(),
		Name:      "development",
		Slug:      "development",
		Type:      controlv1.EnvironmentType_ENVIRONMENT_TYPE_DEV,
	})
	must("ensure environment", err)

	agent, err := client.EnsureAgent(ctx, &controlv1.CreateAgentRequest{
		EnvId:       envRecord.GetId(),
		Name:        "clinic-assistant",
		Description: "Default Simorq clinic assistant",
	})
	must("ensure agent", err)

	_, err = client.EnsurePolicy(ctx, &controlv1.CreatePolicyRequest{
		EnvId:       envRecord.GetId(),
		Name:        "simorq-default",
		Description: "Default PHI-safe policy for Simorq development",
		Mode:        controlv1.PolicyMode_POLICY_MODE_ENFORCE,
	})
	must("ensure policy", err)

	_, err = client.EnsureBudget(ctx, &controlv1.CreateBudgetRequest{
		AgentId: agent.GetId(),
		HardCap: &commonv1.Amount{Currency: "USD", Decimal: "50"},
		SoftCap: &commonv1.Amount{Currency: "USD", Decimal: "40"},
		Period:  controlv1.BudgetPeriod_BUDGET_PERIOD_MONTHLY,
	})
	must("ensure budget", err)

	token, err := client.CreateAgentToken(ctx, &controlv1.CreateTokenRequest{
		AgentId: agent.GetId(),
		Name:    "simorq-dev-bootstrap",
	})
	must("create token", err)

	fmt.Printf("org=%s project=%s env=%s agent=%s token=%s\n",
		org.GetId(), project.GetId(), envRecord.GetId(), agent.GetId(), token.GetRawToken())
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
