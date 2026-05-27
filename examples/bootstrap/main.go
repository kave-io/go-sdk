package main

import (
	"context"
	"fmt"
	"log"
	"os"

	kave "github.com/kave-io/kave/sdk/go"
)

func main() {
	ctx := context.Background()
	client, err := kave.NewFromConfig(kave.ClientConfig{
		Addr:  env("KAVE_ADDR", "http://localhost:18080"),
		Token: os.Getenv("KAVE_TOKEN"),
	})
	must("create client", err)

	budget := kave.MonthlyBudget("clinic-assistant", kave.AmountUSD("50"))
	softCap := kave.AmountUSD("40")
	budget.SoftCap = &softCap

	result, err := client.Bootstrap(ctx, kave.BootstrapInput{
		Organization: kave.OrganizationInput{Name: "Simorq", Slug: "simorq"},
		Project:      kave.ProjectInput{Name: "simorq", Slug: "simorq"},
		Environments: []kave.EnvironmentInput{kave.Development()},
		Policies: []kave.PolicyInput{
			{
				Env:         "development",
				Name:        "simorq-default",
				Description: "Default PHI-safe policy for Simorq development",
				Mode:        kave.PolicyModeEnforce,
			},
		},
		Agents: []kave.AgentInput{
			{
				Env:         "development",
				Name:        "clinic-assistant",
				Description: "Default Simorq clinic assistant",
				Policy:      "simorq-default",
			},
		},
		Budgets: []kave.BudgetInput{budget},
		Tokens: []kave.TokenInput{
			{Agent: "clinic-assistant", Name: "simorq-dev-bootstrap"},
		},
	})
	must("bootstrap", err)

	agent := result.Agents["clinic-assistant"]
	token := result.Tokens["simorq-dev-bootstrap"]
	fmt.Printf("org=%s project=%s env=%s agent=%s token=%s\n",
		result.Organization.ID,
		result.Project.ID,
		result.Environments["development"].ID,
		agent.ID,
		token.RawToken)
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
