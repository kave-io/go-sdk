# Kave Go SDK

Go SDK for the Kave control, runtime, and audit APIs.

## Install

```bash
go get github.com/kave-io/kave/sdk/go@v0.1.0
```

For local monorepo development this module uses a narrow `replace` for
`github.com/kave-io/kave/proto/gen`; do not rely on a repo-root `go.work`.

## Quickstart

```go
client := kave.New(
	kave.WithAddr("http://localhost:18080"),
	kave.WithToken(os.Getenv("KAVE_TOKEN")),
)
org, err := client.EnsureOrganization(ctx, &controlv1.CreateOrganizationRequest{Name: "Acme", Slug: "acme"})
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
