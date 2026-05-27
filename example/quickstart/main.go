package main

import (
	"context"
	"fmt"
	"log"

	kave "github.com/kave-io/kave/sdk/go"
)

func main() {
	client := kave.New(
		kave.WithAddr("http://localhost:18080"),
		kave.WithToken("your-kave-token"),
	)

	ctx := context.Background()

	// Ensure an organization (idempotent).
	org, err := client.EnsureOrganization(ctx, kave.OrganizationInput{
		Name: "acme",
		Slug: "acme",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("org:", org.ID)

	// Ensure a project within it.
	proj, err := client.EnsureProject(ctx, kave.ProjectInput{
		OrgID: org.ID,
		Name:  "my-agent",
		Slug:  "my-agent",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("project:", proj.ID)

	// List agents in an environment.
	count := 0
	for agent, err := range client.IterateAgents(ctx, "env-id") {
		if err != nil {
			log.Fatal(err)
		}
		_ = agent
		count++
	}
	fmt.Printf("%d agents\n", count)
}
