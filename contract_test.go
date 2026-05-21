//go:build contracts

package kave_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	controlv1 "github.com/kave-io/kave/proto/gen/kave/control/v1"
	runtimev1 "github.com/kave-io/kave/proto/gen/kave/runtime/v1"
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
	req := &controlv1.CreateOrganizationRequest{Name: slug, Slug: slug}

	org1, err := c.EnsureOrganization(ctx, req)
	if err != nil {
		t.Fatalf("first EnsureOrganization: %v", err)
	}
	if org1.GetSlug() != slug {
		t.Fatalf("slug mismatch: got %q, want %q", org1.GetSlug(), slug)
	}

	org2, err := c.EnsureOrganization(ctx, req)
	if err != nil {
		t.Fatalf("second EnsureOrganization: %v", err)
	}
	if org2.GetId() != org1.GetId() {
		t.Fatal("EnsureOrganization not idempotent: second call returned a different org")
	}
}

// TestContractEnsureProject mirrors ensure-project.yaml
func TestContractEnsureProject(t *testing.T) {
	ctx := context.Background()
	c := newContractClient(t)

	orgSlug := uniqueName("ct-proj-org")
	org, err := c.EnsureOrganization(ctx, &controlv1.CreateOrganizationRequest{
		Name: orgSlug, Slug: orgSlug,
	})
	if err != nil {
		t.Fatalf("setup EnsureOrganization: %v", err)
	}

	slug := uniqueName("ct-proj")
	req := &controlv1.CreateProjectRequest{OrgId: org.GetId(), Name: slug, Slug: slug}

	p1, err := c.EnsureProject(ctx, req)
	if err != nil {
		t.Fatalf("first EnsureProject: %v", err)
	}
	p2, err := c.EnsureProject(ctx, req)
	if err != nil {
		t.Fatalf("second EnsureProject: %v", err)
	}
	if p2.GetId() != p1.GetId() {
		t.Fatal("EnsureProject not idempotent")
	}
}

// TestContractEnsureEnvironment mirrors ensure-environment.yaml
func TestContractEnsureEnvironment(t *testing.T) {
	ctx := context.Background()
	c := newContractClient(t)

	orgSlug := uniqueName("ct-env-org")
	org, err := c.EnsureOrganization(ctx, &controlv1.CreateOrganizationRequest{Name: orgSlug, Slug: orgSlug})
	if err != nil {
		t.Fatalf("setup org: %v", err)
	}
	projSlug := uniqueName("ct-env-proj")
	proj, err := c.EnsureProject(ctx, &controlv1.CreateProjectRequest{OrgId: org.GetId(), Name: projSlug, Slug: projSlug})
	if err != nil {
		t.Fatalf("setup project: %v", err)
	}

	slug := uniqueName("ct-env")
	req := &controlv1.CreateEnvironmentRequest{ProjectId: proj.GetId(), Name: slug, Slug: slug}

	e1, err := c.EnsureEnvironment(ctx, req)
	if err != nil {
		t.Fatalf("first EnsureEnvironment: %v", err)
	}
	e2, err := c.EnsureEnvironment(ctx, req)
	if err != nil {
		t.Fatalf("second EnsureEnvironment: %v", err)
	}
	if e2.GetId() != e1.GetId() {
		t.Fatal("EnsureEnvironment not idempotent")
	}
}

// TestContractEnsureAgent mirrors ensure-agent.yaml
func TestContractEnsureAgent(t *testing.T) {
	ctx := context.Background()
	c := newContractClient(t)

	env := setupEnv(t, ctx, c, "ct-agent")

	name := uniqueName("bot")
	req := &controlv1.CreateAgentRequest{EnvId: env.GetId(), Name: name}

	a1, err := c.EnsureAgent(ctx, req)
	if err != nil {
		t.Fatalf("first EnsureAgent: %v", err)
	}
	a2, err := c.EnsureAgent(ctx, req)
	if err != nil {
		t.Fatalf("second EnsureAgent: %v", err)
	}
	if a2.GetId() != a1.GetId() {
		t.Fatal("EnsureAgent not idempotent")
	}
}

// TestContractEnsurePolicy mirrors ensure-policy.yaml
func TestContractEnsurePolicy(t *testing.T) {
	ctx := context.Background()
	c := newContractClient(t)

	env := setupEnv(t, ctx, c, "ct-policy")

	name := uniqueName("pol")
	req := &controlv1.CreatePolicyRequest{EnvId: env.GetId(), Name: name}

	p1, err := c.EnsurePolicy(ctx, req)
	if err != nil {
		t.Fatalf("first EnsurePolicy: %v", err)
	}
	p2, err := c.EnsurePolicy(ctx, req)
	if err != nil {
		t.Fatalf("second EnsurePolicy: %v", err)
	}
	if p2.GetId() != p1.GetId() {
		t.Fatal("EnsurePolicy not idempotent")
	}
}

// TestContractCreateAgentToken mirrors create-token.yaml
func TestContractCreateAgentToken(t *testing.T) {
	ctx := context.Background()
	c := newContractClient(t)

	env := setupEnv(t, ctx, c, "ct-token")
	agent, err := c.EnsureAgent(ctx, &controlv1.CreateAgentRequest{
		EnvId: env.GetId(), Name: uniqueName("bot"),
	})
	if err != nil {
		t.Fatalf("setup agent: %v", err)
	}

	resp, err := c.CreateAgentToken(ctx, &controlv1.CreateTokenRequest{
		AgentId: agent.GetId(),
		Name:    "test-token",
	})
	if err != nil {
		t.Fatalf("CreateAgentToken: %v", err)
	}
	if resp.GetRawToken() == "" {
		t.Fatal("CreateAgentToken: empty raw token")
	}
}

// TestContractRunLifecycle mirrors run-lifecycle.yaml
func TestContractRunLifecycle(t *testing.T) {
	ctx := context.Background()
	c := newContractClient(t)

	env := setupEnv(t, ctx, c, "ct-run")
	agent, err := c.EnsureAgent(ctx, &controlv1.CreateAgentRequest{
		EnvId: env.GetId(), Name: uniqueName("runner"),
	})
	if err != nil {
		t.Fatalf("setup agent: %v", err)
	}

	run, err := c.CreateRun(ctx, &runtimev1.CreateRunRequest{
		AgentId: agent.GetId(),
		EnvId:   env.GetId(),
	})
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if run.GetId() == "" {
		t.Fatal("CreateRun: empty id")
	}

	completedStatus := runtimev1.RunStatus_RUN_STATUS_COMPLETED
	updated, err := c.UpdateRun(ctx, &runtimev1.UpdateRunRequest{
		Id:     run.GetId(),
		Update: &runtimev1.RunUpdate{Status: &completedStatus},
	})
	if err != nil {
		t.Fatalf("UpdateRun: %v", err)
	}
	if updated.GetStatus() != runtimev1.RunStatus_RUN_STATUS_COMPLETED {
		t.Fatalf("UpdateRun: expected COMPLETED, got %v", updated.GetStatus())
	}
}

// TestContractSpanLifecycle mirrors span-lifecycle.yaml
func TestContractSpanLifecycle(t *testing.T) {
	ctx := context.Background()
	c := newContractClient(t)

	env := setupEnv(t, ctx, c, "ct-span")
	agent, err := c.EnsureAgent(ctx, &controlv1.CreateAgentRequest{
		EnvId: env.GetId(), Name: uniqueName("spanner"),
	})
	if err != nil {
		t.Fatalf("setup agent: %v", err)
	}
	run, err := c.CreateRun(ctx, &runtimev1.CreateRunRequest{
		AgentId: agent.GetId(),
		EnvId:   env.GetId(),
	})
	if err != nil {
		t.Fatalf("setup run: %v", err)
	}

	err = c.WithSpan(ctx, &runtimev1.OpenSpanRequest{
		Span: &runtimev1.SpanInput{RunId: run.GetId(), Name: "test-span"},
	}, func(ctx context.Context, span *runtimev1.SpanRow) error {
		if span.GetId() == "" {
			return fmt.Errorf("span id is empty")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WithSpan: %v", err)
	}
}

// setupEnv is a test helper that creates a fresh org → project → env chain.
func setupEnv(t *testing.T, ctx context.Context, c *kave.Client, prefix string) *controlv1.Environment {
	t.Helper()
	orgSlug := uniqueName(prefix + "-org")
	org, err := c.EnsureOrganization(ctx, &controlv1.CreateOrganizationRequest{Name: orgSlug, Slug: orgSlug})
	if err != nil {
		t.Fatalf("setup org: %v", err)
	}
	projSlug := uniqueName(prefix + "-proj")
	proj, err := c.EnsureProject(ctx, &controlv1.CreateProjectRequest{OrgId: org.GetId(), Name: projSlug, Slug: projSlug})
	if err != nil {
		t.Fatalf("setup project: %v", err)
	}
	envSlug := uniqueName(prefix + "-env")
	env, err := c.EnsureEnvironment(ctx, &controlv1.CreateEnvironmentRequest{ProjectId: proj.GetId(), Name: envSlug, Slug: envSlug})
	if err != nil {
		t.Fatalf("setup env: %v", err)
	}
	return env
}
