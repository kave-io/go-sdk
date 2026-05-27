//go:build contracts

package kave_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	kave "github.com/kave-io/kave/sdk/go"
)

func newContractClient(t *testing.T) *kave.Client {
	t.Helper()
	addr := os.Getenv("KAVE_CONTRACT_ADDR")
	if addr == "" {
		addr = "http://localhost:18080"
	}
	token := os.Getenv("KAVE_CONTRACT_TOKEN")
	opts := []kave.Option{kave.WithAddr(addr)}
	if token != "" {
		opts = append(opts, kave.WithToken(token))
	}
	return kave.New(opts...)
}

// uniqueName returns a name that is unlikely to collide across test runs.
func uniqueName(base string) string {
	return fmt.Sprintf("%s-%d", base, time.Now().UnixMilli())
}

// TestContractEnsureOrganization mirrors ensure-organization.yaml
func TestContractEnsureOrganization(t *testing.T) {
	ctx := context.Background()
	c := newContractClient(t)

	slug := uniqueName("ct-org")
	in := kave.OrganizationInput{Name: slug, Slug: slug}

	org1, err := c.EnsureOrganization(ctx, in)
	if err != nil {
		t.Fatalf("first EnsureOrganization: %v", err)
	}
	if org1.Slug != slug {
		t.Fatalf("slug mismatch: got %q, want %q", org1.Slug, slug)
	}

	org2, err := c.EnsureOrganization(ctx, in)
	if err != nil {
		t.Fatalf("second EnsureOrganization: %v", err)
	}
	if org2.ID != org1.ID {
		t.Fatal("EnsureOrganization not idempotent: second call returned a different org")
	}
}

// TestContractEnsureProject mirrors ensure-project.yaml
func TestContractEnsureProject(t *testing.T) {
	ctx := context.Background()
	c := newContractClient(t)

	orgSlug := uniqueName("ct-proj-org")
	org, err := c.EnsureOrganization(ctx, kave.OrganizationInput{Name: orgSlug, Slug: orgSlug})
	if err != nil {
		t.Fatalf("setup EnsureOrganization: %v", err)
	}

	slug := uniqueName("ct-proj")
	in := kave.ProjectInput{OrgID: org.ID, Name: slug, Slug: slug}

	p1, err := c.EnsureProject(ctx, in)
	if err != nil {
		t.Fatalf("first EnsureProject: %v", err)
	}
	p2, err := c.EnsureProject(ctx, in)
	if err != nil {
		t.Fatalf("second EnsureProject: %v", err)
	}
	if p2.ID != p1.ID {
		t.Fatal("EnsureProject not idempotent")
	}
}

// TestContractEnsureEnvironment mirrors ensure-environment.yaml
func TestContractEnsureEnvironment(t *testing.T) {
	ctx := context.Background()
	c := newContractClient(t)

	env := setupEnv(t, ctx, c, "ct-env")
	in := kave.EnvironmentInput{ProjectID: env.ProjectID, Name: env.Name, Slug: env.Slug}

	e2, err := c.EnsureEnvironment(ctx, in)
	if err != nil {
		t.Fatalf("re-EnsureEnvironment: %v", err)
	}
	if e2.ID != env.ID {
		t.Fatal("EnsureEnvironment not idempotent")
	}
}

// TestContractEnsureAgent mirrors ensure-agent.yaml
func TestContractEnsureAgent(t *testing.T) {
	ctx := context.Background()
	c := newContractClient(t)

	env := setupEnv(t, ctx, c, "ct-agent")

	name := uniqueName("bot")
	in := kave.AgentInput{EnvID: env.ID, Name: name}

	a1, err := c.EnsureAgent(ctx, in)
	if err != nil {
		t.Fatalf("first EnsureAgent: %v", err)
	}
	a2, err := c.EnsureAgent(ctx, in)
	if err != nil {
		t.Fatalf("second EnsureAgent: %v", err)
	}
	if a2.ID != a1.ID {
		t.Fatal("EnsureAgent not idempotent")
	}
}

// TestContractEnsurePolicy mirrors ensure-policy.yaml
func TestContractEnsurePolicy(t *testing.T) {
	ctx := context.Background()
	c := newContractClient(t)

	env := setupEnv(t, ctx, c, "ct-policy")

	name := uniqueName("pol")
	in := kave.PolicyInput{EnvID: env.ID, Name: name}

	p1, err := c.EnsurePolicy(ctx, in)
	if err != nil {
		t.Fatalf("first EnsurePolicy: %v", err)
	}
	p2, err := c.EnsurePolicy(ctx, in)
	if err != nil {
		t.Fatalf("second EnsurePolicy: %v", err)
	}
	if p2.ID != p1.ID {
		t.Fatal("EnsurePolicy not idempotent")
	}
}

// TestContractCreateAgentToken mirrors create-token.yaml
func TestContractCreateAgentToken(t *testing.T) {
	ctx := context.Background()
	c := newContractClient(t)

	env := setupEnv(t, ctx, c, "ct-token")
	agent, err := c.EnsureAgent(ctx, kave.AgentInput{EnvID: env.ID, Name: uniqueName("bot")})
	if err != nil {
		t.Fatalf("setup agent: %v", err)
	}

	issued, err := c.CreateAgentToken(ctx, kave.TokenInput{AgentID: agent.ID, Name: "test-token"})
	if err != nil {
		t.Fatalf("CreateAgentToken: %v", err)
	}
	if issued.RawToken == "" {
		t.Fatal("CreateAgentToken: empty raw token")
	}
}

// TestContractRunLifecycle mirrors run-lifecycle.yaml
func TestContractRunLifecycle(t *testing.T) {
	ctx := context.Background()
	c := newContractClient(t)

	env := setupEnv(t, ctx, c, "ct-run")
	agent, err := c.EnsureAgent(ctx, kave.AgentInput{EnvID: env.ID, Name: uniqueName("runner")})
	if err != nil {
		t.Fatalf("setup agent: %v", err)
	}

	run, err := c.CreateRun(ctx, kave.RunInput{AgentID: agent.ID, EnvID: env.ID, ProjectID: env.ProjectID})
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if run.ID == "" {
		t.Fatal("CreateRun: empty id")
	}

	completed := kave.RunStatusCompleted
	updated, err := c.UpdateRun(ctx, kave.RunUpdateInput{ID: run.ID, Status: &completed})
	if err != nil {
		t.Fatalf("UpdateRun: %v", err)
	}
	if updated.Status != kave.RunStatusCompleted {
		t.Fatalf("UpdateRun: expected completed, got %v", updated.Status)
	}
}

// TestContractSpanLifecycle mirrors span-lifecycle.yaml
func TestContractSpanLifecycle(t *testing.T) {
	ctx := context.Background()
	c := newContractClient(t)

	env := setupEnv(t, ctx, c, "ct-span")
	agent, err := c.EnsureAgent(ctx, kave.AgentInput{EnvID: env.ID, Name: uniqueName("spanner")})
	if err != nil {
		t.Fatalf("setup agent: %v", err)
	}
	run, err := c.CreateRun(ctx, kave.RunInput{AgentID: agent.ID, EnvID: env.ID, ProjectID: env.ProjectID})
	if err != nil {
		t.Fatalf("setup run: %v", err)
	}

	err = c.WithSpan(ctx, kave.SpanInput{RunID: run.ID, Name: "test-span"}, func(ctx context.Context, span *kave.Span) error {
		if span.ID == "" {
			return fmt.Errorf("span id is empty")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WithSpan: %v", err)
	}
}

// setupEnv is a test helper that creates a fresh org -> project -> env chain.
func setupEnv(t *testing.T, ctx context.Context, c *kave.Client, prefix string) *kave.Environment {
	t.Helper()
	orgSlug := uniqueName(prefix + "-org")
	org, err := c.EnsureOrganization(ctx, kave.OrganizationInput{Name: orgSlug, Slug: orgSlug})
	if err != nil {
		t.Fatalf("setup org: %v", err)
	}
	projSlug := uniqueName(prefix + "-proj")
	proj, err := c.EnsureProject(ctx, kave.ProjectInput{OrgID: org.ID, Name: projSlug, Slug: projSlug})
	if err != nil {
		t.Fatalf("setup project: %v", err)
	}
	envSlug := uniqueName(prefix + "-env")
	env, err := c.EnsureEnvironment(ctx, kave.EnvironmentInput{ProjectID: proj.ID, Name: envSlug, Slug: envSlug})
	if err != nil {
		t.Fatalf("setup env: %v", err)
	}
	return env
}
