# Kave Go SDK

This is the supported Go client for the tenant-scoped Kave kernel. The module
contains a small typed control, consumption, reporting, and OpenAI-compatible
HTTP surface:

```sh
go get github.com/kave-io/go-sdk/v2
```

The module requires Go 1.26.5 or newer.

Clients are safe for concurrent use. A service key fixes the account and
namespace on the server; application code can assert only the opaque workload
scope permitted by that key. Kave does not accept provider credentials through
the invocation transport and the SDK never retries requests automatically.

## Provision a route safely

Provisioning is an explicit four-step lifecycle: establish the namespace,
write its credential, apply the declarative route, and validate and activate
that exact route revision against the provider. A route cannot be applied
against a missing credential or receive traffic before successful activation.

```go
manifest := kave.Manifest{
	Namespace: kave.Namespace{
		Account:     "account/acme",
		Application: "checkout",
		Environment: "production",
	},
}

initialized, err := client.Apply(
	ctx, manifest, kave.Once("deploy/2026-07-20/namespace"),
)
if err != nil {
	return err
}

credential := []byte(os.Getenv("OPENAI_API_KEY"))
defer clear(credential)
secret, err := client.PutEncryptedSecret(
	ctx,
	initialized.NamespaceID,
	kave.Ref("openai-production"),
	credential,
	kave.Once("deploy/2026-07-20/openai-secret"),
)
if err != nil {
	return err
}

manifest.Routes = []kave.Route{{
		Name:            "openai",
		Provider:        "openai",
		Secret:          "openai-production",
		AllowedModels:   []string{"gpt-5"},
		DefaultModel:    "gpt-5",
		PricingRevision: 7,
		Pricing: []kave.ModelPrice{{
			Model:                            kave.Ref("gpt-5"),
			InputNanosPerMillionTokens:       250_000_000,
			OutputNanosPerMillionTokens:      2_000_000_000,
			CacheReadNanosPerMillionTokens:     25_000_000,
			CacheWriteNanosPerMillionTokens:   312_500_000,
			ReasoningNanosPerMillionTokens:   2_000_000_000,
		}},
	}}
manifest.Agents = []kave.AgentSpec{{
		Name: "checkout-assistant", Kind: kave.AgentLLM,
		Route: "openai", Enabled: true,
	}}

applied, err := client.Apply(
	ctx, manifest, kave.Once("deploy/2026-07-20/manifest"),
)
if err != nil {
	return err
}

activation, err := client.ActivateProviderRoute(
	ctx,
	applied.NamespaceID,
	kave.Ref("openai"),
	kave.Ref("gpt-5"),
)
if err != nil {
	return err
}
log.Printf("route %s active at revision %d with secret version %d",
	activation.Route, activation.RouteRevision, secret.Version)
```

`ActivateProviderRoute` performs a payload-free authenticated model lookup. Its
result is bound to the route revision, secret version, and selected model. A
route-changing `Apply` or a secret rotation makes the prior activation stale;
activate it again before sending traffic. Passing an empty model selects the
route's default model. Treat the returned provider request ID as operational
evidence, not as application identity.

Every allowed model needs a non-negative price snapshot and the route needs a
positive pricing revision. Rates are integer nano-USD per million tokens. The
input and output fields price ordinary tokens; cache-read tokens replace their
input rate, reasoning tokens replace their output rate, and cache-write tokens
are an additional provider-reported dimension. A zero detailed rate inherits
the corresponding ordinary input or output rate. Increase `PricingRevision`
whenever any rate changes so admissions and immutable settlements retain the
price that was actually in force.

Provider secret values never belong in a manifest; routes contain only the
opaque secret name. `PutEncryptedSecret` copies the provided bytes for the
request and clears that internal copy; the caller still owns and must clear its
original buffer. Kave encrypts the credential and does not persist it in audit
or usage records.

## Invoke an agent

Set `KAVE_URL` and `KAVE_SERVICE_KEY`, then create a client. Production origins
must use HTTPS; plain HTTP is accepted for loopback development unless
`KAVE_ALLOW_INSECURE` is explicitly enabled.

```go
client, err := kave.OpenFromEnv()
if err != nil {
	return err
}

ctx = kave.WithScope(ctx, kave.Scope{
	Tenant:  kave.Ref("tenant/" + tenantRef),
	Actor:   kave.Ref("user/" + actorRef),
	BillTo:  kave.Ref("account/" + billingRef),
	Session: kave.Ref(runRef),
})
ctx = kave.WithInvocation(ctx, kave.Once(stepRef))

req, err := http.NewRequestWithContext(
	ctx,
	http.MethodPost,
	"https://api.openai.com/v1/responses",
	strings.NewReader(`{"model":"gpt-5","input":"hello"}`),
)
if err != nil {
	return err
}
resp, err := client.HTTPClient(kave.Agent("checkout-assistant")).Do(req)
if err != nil {
	return err
}
defer resp.Body.Close()
```

The transport accepts only `POST /v1/responses`,
`POST /v1/chat/completions`, and `POST /v1/embeddings`. It rewrites the request
to `/v2/agents/{agent}/openai/*` at the configured Kave origin, replaces
`Authorization` with the Kave service key, and attaches only the validated
pseudonymous scope and stable invocation identity. Redirects are refused.
Query parameters, cookies, baggage, caller credentials, and non-allowlisted
headers are rejected or removed.

## Consume quota exactly once

Use `Consume` before starting a retryable product workflow. The required
idempotency key makes the admission decision stable across retries.

```go
decision, err := client.Consume(
	ctx,
	kave.Agent("checkout-assistant"),
	kave.Metric("ai_actions"),
	1,
	kave.Once(runRef),
)
if errors.Is(err, kave.ErrLimitExceeded) {
	var exceeded *kave.LimitExceededError
	if errors.As(err, &exceeded) {
		// exceeded.Decision.Violations contains structured limit data.
	}
}
```

Repeating an identical request returns the original decision with `Replayed`
set. Reusing its idempotency key for different input is rejected. `SyncLimits`
is the corresponding transactionally versioned API for publishing dynamic
entitlements from an application outbox.

## Read usage and tenants

Reporting keys should carry `usage.read` without provisioning or invocation
capabilities. `ListTenants` returns only opaque tenant/billing references and
namespace-scoped aggregates, ordered behind an opaque cursor:

```go
now := time.Now().UTC()
page, err := client.ListTenants(ctx, kave.TenantQuery{
	Range: kave.QueryRange{From: now.Add(-24 * time.Hour), To: now},
	Page:  kave.Page{Size: 100},
})
if err != nil {
	return err
}
for _, tenant := range page.Tenants {
	log.Printf("tenant=%s bill_to=%s requests=%d cost_nano_usd=%d status=%s",
		tenant.Tenant, tenant.BillTo, tenant.RequestCount,
		tenant.CostNanoUSD, tenant.Status)
}
```

`active` means at least one current targeted limit applies to the tenant and
billing pair; `observed` means the pair appears in the requested activity
window without a current targeted limit. Limit-only pairs are included with
zero activity. Use `NextPageToken` unchanged with the same time range to fetch
the next page. Query ranges are bounded to 366 days and page sizes to 200.

`QueryUsage` exposes request count, ordinary input/output, cache-read,
cache-write, reasoning tokens, and cost in nano-USD. An entry with
`Estimated=true` is a conservative settlement for an uncertain or abandoned
provider attempt. Keep it in billing totals and surface the marker during
reconciliation.

## Service keys and failure semantics

Create separate keys for control, reporting, consumption, and invocation.
Consumption and invocation keys require an immutable agent allowlist and
should enable scope assertion only for the workloads that need it.

Service-key raw material is generated by this SDK. Only its random lookup
prefix and SHA-256 verifier cross the control API, so Kave cannot recover or
return the credential. `IssueServiceKey` returns the locally held `RawKey` on
creation and on an idempotent replay. If transport fails after a possibly
committed issuance, the partial result still contains `RawKey`; put that value
on `ServiceKeySpec.RawKey` when retrying with the same idempotency key.
Different material for the same key is rejected rather than silently rotated.
`Config`, `Client`, `ServiceKeySpec`, and `IssuedServiceKey` redact credential
material from standard formatting and JSON; never log `RawKey` explicitly.

`RevokeServiceKey` and `RevokeSecret` are typed, idempotent operations. Both
validate identifiers and single-line audit reasons before sending a request.

The SDK deliberately has narrow failure behavior:

- it validates identifiers, ranges, pages, origins, and capability-shaped
  input before making a request;
- it never retries control, admission, reporting, or provider requests;
- it refuses endpoint user info, paths, queries, fragments, redirects, custom
  post-sanitization transports, and non-loopback plaintext by default;
- it returns typed sentinel errors and preserves structured limit violations;
  and
- it treats malformed or contradictory server responses as
  `ErrInvalidResponse` instead of guessing.

Always give requests a deadline appropriate to the operation. On any ambiguous
mutating failure, retry only with the identical payload and original
`kave.Once` value.
