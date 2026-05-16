package main

import (
	"context"
	"fmt"
	"log"

	kave "github.com/kave-io/go-sdk"
	controlv1 "github.com/kave-io/kave/proto/gen/kave/control/v1"
	"connectrpc.com/connect"
)

func main() {
	client := kave.New(
		kave.WithAddr("http://localhost:8080"),
		kave.WithToken("your-kave-token"),
	)

	ctx := context.Background()

	// Create an organization
	org, err := client.Control.CreateOrganization(ctx, connect.NewRequest(&controlv1.CreateOrganizationRequest{
		Name: "acme",
		Slug: "acme",
	}))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("org:", org.Msg.Id)

	// Create a project
	proj, err := client.Control.CreateProject(ctx, connect.NewRequest(&controlv1.CreateProjectRequest{
		OrgId: org.Msg.Id,
		Name:  "my-agent",
		Slug:  "my-agent",
	}))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("project:", proj.Msg.Id)

	// List agents
	agents, err := client.Control.ListAgents(ctx, connect.NewRequest(&controlv1.ListAgentsRequest{
		EnvId: "env-id",
		Limit: 10,
	}))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%d agents\n", len(agents.Msg.Agents))
}
