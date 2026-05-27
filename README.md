# Kave Go SDK

Go SDK for the Kave control, runtime, and audit APIs.

## Install

```bash
go get github.com/kave-io/kave/sdk/go@v0.2.0
```

For local monorepo development, use an uncommitted `go.work` that includes this
module and `../../core/proto/gen`. Release builds must not use local `replace`
directives.

## Quickstart

```go
client, err := kave.NewFromConfig(kave.ClientConfig{
	Addr:  "http://localhost:18080",
	Token: os.Getenv("KAVE_TOKEN"),
})
if err != nil {
	return err
}

result, err := client.Bootstrap(ctx, kave.BootstrapSpec{
	Organization: kave.OrganizationSpec{Name: "Acme", Slug: "acme"},
	Project:      kave.ProjectSpec{Name: "acme-ai", Slug: "acme-ai"},
	Environments: []kave.EnvironmentSpec{kave.Development()},
	Policies: []kave.PolicySpec{
		kave.EnforcePolicy("development", "default-ai-policy"),
	},
	Agents: []kave.AgentSpec{
		kave.Agent("development", "clinic-assistant",
			kave.WithAgentPolicy("default-ai-policy"),
		),
	},
	Budgets: []kave.BudgetSpec{
		kave.MonthlyBudget("clinic-assistant", kave.AmountUSD("50")),
	},
})
if err != nil {
	return err
}
_ = result.Agents["clinic-assistant"].GetId()
```

The quickstart uses SDK-native specs, so application setup code does not need
to import generated proto request packages. The generated clients are still
available for advanced calls.

## Startup provisioning

Use `Bootstrap` for startup setup:

- organization and project
- environments
- policies
- agents
- per-agent budgets
- optional agent tokens
- optional RBAC roles and bindings

`Bootstrap` resolves environment, policy, and agent references by name within
the same spec. RBAC bindings can reference roles by name. Ensure-style
resources are idempotent; tokens are intentionally created only when listed
because raw token secrets are returned once.

For one-off calls, use the matching spec helpers:

```go
agent, err := client.EnsureAgentSpec(ctx,
	kave.Agent("env_123", "clinic-assistant",
		kave.WithAgentDescription("Handles clinic AI workflows"),
	),
)
```

RBAC can use the same style:

```go
role, err := client.EnsureRoleSpec(ctx, kave.Role("ai-admin", "agents:read", "agents:write"))
binding, err := client.EnsureRBACBindingSpec(ctx, kave.Binding(role.GetId(), "user:123", "project:abc:*"))
```

## Auth

`WithToken` sends `Authorization: Bearer <token>` on every call. `WithUserAgent`
overrides the default `kave-go-sdk/dev` user agent.

## TLS

```go
client := kave.New(kave.WithTLS(&tls.Config{MinVersion: tls.VersionTLS12}))
```

Passing `nil` uses the system roots.

## Retry

Reads whose RPC name starts with `List`, `Get`, or `Watch` retry transient
`Unavailable` and `DeadlineExceeded` failures by default.

```go
client := kave.New(kave.WithRetry(kave.NoRetry))
```

## Logging

```go
client := kave.New(kave.WithLogger(slog.Default()))
```

Debug logs include RPC name, latency, and unified SDK code.

## Tracing

```go
client := kave.New(kave.WithTracer(otel.Tracer("simorq")))
```

The SDK creates `kave.<rpc>` spans and injects `traceparent` metadata.

## Examples

- `examples/bootstrap`: org -> project -> env -> agent -> policy -> budget -> token.
- `examples/runtime`: run/action/span lifecycle and spend report lookup.
- `examples/streaming`: watch run updates.

The low-level generated clients are exposed as `client.Control`,
`client.Runtime`, `client.Audit`, and `client.RBAC`. High-level helper
semantics are defined in `../CONTRACT.md`.
