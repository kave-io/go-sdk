//go:build integration

package kave

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestLiveKernelContract is the release compatibility gate between the Go SDK
// and a real Kave server backed by PostgreSQL. Unit tests intentionally mock
// the generated client; this test proves the complete public contract,
// authorization boundary, RLS scope, idempotency, and atomic quota path.
func TestLiveKernelContract(t *testing.T) {
	url := os.Getenv("KAVE_TEST_URL")
	adminKey := os.Getenv("KAVE_TEST_SERVICE_KEY")
	if url == "" || adminKey == "" {
		t.Skip("KAVE_TEST_URL and KAVE_TEST_SERVICE_KEY are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	admin, err := Open(Config{URL: url, ServiceKey: adminKey})
	if err != nil {
		t.Fatalf("open admin client: %v", err)
	}

	run := randomTestSuffix(t)
	agent := Agent("sdk-contract")
	tenant := Ref("tenant/sdk-contract-" + run)
	providerSecret := "sdk-provider-" + run
	var modelChecks atomic.Int32
	var providerCalls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer "+providerSecret {
			http.Error(response, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/v1/models/fixture-model":
			modelChecks.Add(1)
			response.Header().Set("Content-Type", "application/json")
			response.Header().Set("X-Request-Id", "sdk-probe-"+run)
			_, _ = io.WriteString(response, `{"id":"fixture-model","object":"model"}`)
		case request.Method == http.MethodPost && request.URL.Path == "/v1/responses":
			providerCalls.Add(1)
			if request.Header.Get("Content-Type") != "application/json" {
				http.Error(response, "content type required", http.StatusUnsupportedMediaType)
				return
			}
			body, readErr := io.ReadAll(io.LimitReader(request.Body, 1024))
			if readErr != nil || !strings.Contains(string(body), `"model":"fixture-model"`) {
				http.Error(response, "invalid request", http.StatusBadRequest)
				return
			}
			response.Header().Set("Content-Type", "application/json")
			response.Header().Set("X-Request-Id", "sdk-response-"+run)
			_, _ = io.WriteString(response, `{
  "id":"resp_sdk_contract",
  "object":"response",
  "model":"fixture-model",
  "usage":{
    "input_tokens":20,
    "output_tokens":7,
    "input_tokens_details":{"cached_tokens":4,"cache_creation_tokens":3},
    "output_tokens_details":{"reasoning_tokens":2}
  }
}`)
		default:
			http.NotFound(response, request)
		}
	}))
	defer provider.Close()

	namespace := Namespace{
		Account:     envOr("KAVE_TEST_ACCOUNT", "account/sdk-contract"),
		Application: envOr("KAVE_TEST_APPLICATION", "sdk-contract"),
		Environment: envOr("KAVE_TEST_ENVIRONMENT", "test"),
	}
	initialized, err := admin.Apply(ctx, Manifest{Namespace: namespace}, Once("sdk-contract/"+run+"/initialize"), Prune())
	if err != nil {
		t.Fatalf("initialize namespace: %v", err)
	}
	if initialized.NamespaceID == "" || initialized.Revision <= 0 {
		t.Fatalf("invalid initialization result: %+v", initialized)
	}

	secretBytes := []byte(providerSecret)
	secret, err := admin.PutEncryptedSecret(
		ctx, initialized.NamespaceID, "sdk-fixture-secret", secretBytes,
		Once("sdk-contract/"+run+"/secret"),
	)
	clear(secretBytes)
	if err != nil {
		t.Fatalf("put provider secret: %v", err)
	}
	if secret.ID == "" || secret.Name != "sdk-fixture-secret" || secret.Source != SecretEncrypted ||
		secret.Version <= 0 || secret.Status != "active" || secret.UpdatedAt.IsZero() {
		t.Fatalf("invalid secret metadata: %+v", secret)
	}

	manifest := Manifest{
		Namespace: namespace,
		Routes: []Route{{
			Name:            "sdk-fixture",
			Provider:        "openai",
			BaseURL:         provider.URL + "/v1",
			Secret:          "sdk-fixture-secret",
			AllowedModels:   []string{"fixture-model"},
			DefaultModel:    "fixture-model",
			PricingRevision: 1,
			Pricing: []ModelPrice{{
				Model: "fixture-model", InputNanosPerMillionTokens: 2_000_000,
				OutputNanosPerMillionTokens:     8_000_000,
				CacheReadNanosPerMillionTokens:  500_000,
				CacheWriteNanosPerMillionTokens: 3_000_000,
				ReasoningNanosPerMillionTokens:  12_000_000,
			}},
		}},
		Agents: []AgentSpec{{Name: agent, Kind: AgentLLM, Route: "sdk-fixture", Enabled: true}},
		Limits: []Limit{{
			Key: "sdk-contract-cap", Metric: MetricRequests,
			Selector: LimitSelector{Tenant: tenant, Agent: agent},
			Window:   WindowAllTime, HardCap: 7, Enabled: true,
		}},
	}
	apply, err := admin.Apply(ctx, manifest, Once("sdk-contract/"+run+"/apply"), Prune())
	if err != nil {
		t.Fatalf("apply manifest: %v", err)
	}
	if !apply.Applied || apply.NamespaceID != initialized.NamespaceID || apply.Revision <= initialized.Revision {
		t.Fatalf("invalid apply result: %+v", apply)
	}
	activation, err := admin.ActivateProviderRoute(ctx, apply.NamespaceID, "sdk-fixture", "fixture-model")
	if err != nil {
		t.Fatalf("activate provider route: %v", err)
	}
	if activation.Route != "sdk-fixture" || activation.Provider != "openai" || activation.Model != "fixture-model" ||
		activation.Status != ProviderRouteActive || activation.RouteRevision <= 0 || activation.SecretVersion != secret.Version ||
		activation.ValidatedAt.IsZero() || activation.ProviderRequestID != "sdk-probe-"+run || modelChecks.Load() != 1 {
		t.Fatalf("invalid activation: %+v checks=%d", activation, modelChecks.Load())
	}

	issued, err := admin.IssueServiceKey(ctx, apply.NamespaceID, ServiceKeySpec{
		Name:           Ref("sdk-contract-" + run),
		Operations:     []Operation{OperationConsume, OperationInvoke, OperationUsageRead},
		AllowedAgents:  []Agent{agent},
		CanAssertScope: true,
		ExpiresAt:      time.Now().UTC().Add(5 * time.Minute),
	}, Once("sdk-contract/"+run+"/key"))
	if err != nil {
		t.Fatalf("issue scoped key: %v", err)
	}
	if !issued.Created || issued.ID == "" || issued.RawKey == "" {
		t.Fatalf("invalid issued key metadata: created=%v id_present=%v raw_present=%v", issued.Created, issued.ID != "", issued.RawKey != "")
	}
	t.Cleanup(func() {
		revokeCtx, revokeCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer revokeCancel()
		_ = admin.RevokeServiceKey(revokeCtx, Ref(issued.ID), "SDK contract cleanup")
	})

	worker, err := Open(Config{URL: url, ServiceKey: issued.RawKey})
	if err != nil {
		t.Fatalf("open scoped client: %v", err)
	}
	scoped := WithScope(ctx, Scope{Tenant: tenant, BillTo: tenant, Session: Ref("run/" + run)})
	invocationContext := WithInvocation(scoped, Once("sdk-contract/"+run+"/provider"))
	providerRequest, err := http.NewRequestWithContext(
		invocationContext,
		http.MethodPost,
		"https://api.openai.com/v1/responses",
		strings.NewReader(`{"model":"fixture-model","input":"sdk contract"}`),
	)
	if err != nil {
		t.Fatalf("create provider request: %v", err)
	}
	providerRequest.Header.Set("Content-Type", "application/json")
	providerResponse, err := worker.HTTPClient(agent).Do(providerRequest)
	if err != nil {
		t.Fatalf("invoke provider route: %v", err)
	}
	responseBody, readErr := io.ReadAll(providerResponse.Body)
	closeErr := providerResponse.Body.Close()
	if readErr != nil || closeErr != nil || providerResponse.StatusCode != http.StatusOK ||
		!strings.Contains(string(responseBody), `"id":"resp_sdk_contract"`) || providerCalls.Load() != 1 {
		t.Fatalf("provider response status=%d body=%q read=%v close=%v calls=%d", providerResponse.StatusCode, responseBody, readErr, closeErr, providerCalls.Load())
	}

	first, err := worker.Consume(scoped, agent, MetricRequests, 1, Once("sdk-contract/"+run+"/first"))
	if err != nil || first.Status != DecisionAdmitted || first.Replayed {
		t.Fatalf("first consume = %+v, %v", first, err)
	}
	replay, err := worker.Consume(scoped, agent, MetricRequests, 1, Once("sdk-contract/"+run+"/first"))
	if err != nil || replay.Status != DecisionAdmitted || !replay.Replayed || replay.InvocationID != first.InvocationID {
		t.Fatalf("replayed consume = %+v, %v; first = %+v", replay, err, first)
	}

	// The provider call and first exact consumption used two units. Five of
	// sixteen concurrent admissions must fill the remaining capacity;
	// the rest must receive structured rejections without overshooting the cap.
	var admitted atomic.Int32
	var rejected atomic.Int32
	var unexpected atomic.Value
	var group sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 16; i++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			<-start
			decision, consumeErr := worker.Consume(
				scoped, agent, MetricRequests, 1,
				Once("sdk-contract/"+run+"/parallel-"+twoDigits(index)),
			)
			switch {
			case consumeErr == nil && decision.Status == DecisionAdmitted:
				admitted.Add(1)
			case errors.Is(consumeErr, ErrLimitExceeded) && decision.Status == DecisionRejected && len(decision.Violations) == 1:
				rejected.Add(1)
			default:
				unexpected.CompareAndSwap(nil, liveContractFailure{decision: decision, err: consumeErr})
			}
		}(i)
	}
	close(start)
	group.Wait()
	if failure := unexpected.Load(); failure != nil {
		value := failure.(liveContractFailure)
		t.Fatalf("unexpected concurrent result: %+v, %v", value.decision, value.err)
	}
	if admitted.Load() != 5 || rejected.Load() != 11 {
		t.Fatalf("concurrent results admitted=%d rejected=%d, want 5/11", admitted.Load(), rejected.Load())
	}

	statuses, err := worker.GetLimitStatus(scoped, agent, MetricRequests)
	if err != nil {
		t.Fatalf("get limit status: %v", err)
	}
	if len(statuses) != 1 || statuses[0].Used != 7 || statuses[0].Reserved != 0 || statuses[0].HardCap != 7 {
		t.Fatalf("limit status = %+v", statuses)
	}

	usage, err := worker.QueryUsage(scoped, UsageQuery{
		Agent: agent, Metric: MetricRequests,
		Range: QueryRange{From: time.Now().UTC().Add(-5 * time.Minute), To: time.Now().UTC().Add(5 * time.Minute)},
		Page:  Page{Size: MaxPageSize},
	})
	if err != nil {
		t.Fatalf("query usage: %v", err)
	}
	var logicalQuantity int64
	var providerSettlement *UsageEntry
	for _, entry := range usage.Entries {
		if entry.EventKind == "consume" && entry.Metric == MetricRequests {
			logicalQuantity += entry.Quantity
		}
		if entry.EventKind == "settlement" && entry.Provider == "openai" && entry.Model == "fixture-model" {
			copy := entry
			providerSettlement = &copy
		}
	}
	if logicalQuantity < 6 {
		t.Fatalf("usage contains %d consumed request units, want at least 6: %+v", logicalQuantity, usage.Entries)
	}
	if providerSettlement == nil || providerSettlement.Estimated || providerSettlement.RequestCount != 1 ||
		providerSettlement.InputTokens != 20 || providerSettlement.OutputTokens != 7 ||
		providerSettlement.CacheReadTokens != 4 || providerSettlement.CacheWriteTokens != 3 ||
		providerSettlement.ReasoningTokens != 2 || providerSettlement.CostNanoUSD != 107 {
		t.Fatalf("provider settlement = %+v", providerSettlement)
	}

	tenants, err := worker.ListTenants(ctx, TenantQuery{
		Range: QueryRange{From: time.Now().UTC().Add(-5 * time.Minute), To: time.Now().UTC().Add(5 * time.Minute)},
		Page:  Page{Size: MaxPageSize},
	})
	if err != nil {
		t.Fatalf("list tenants: %v", err)
	}
	var tenantSummary *TenantSummary
	for _, summary := range tenants.Tenants {
		if summary.Tenant == tenant && summary.BillTo == tenant {
			copy := summary
			tenantSummary = &copy
			break
		}
	}
	if tenantSummary == nil || tenantSummary.Status != TenantStatusActive || tenantSummary.LastSeenAt == nil ||
		tenantSummary.RequestCount != 7 || tenantSummary.CostNanoUSD != 107 || tenantSummary.ActiveLimits != 1 {
		t.Fatalf("tenant summary = %+v; page=%+v", tenantSummary, tenants)
	}

	if err := admin.RevokeServiceKey(ctx, Ref(issued.ID), "SDK contract revocation check"); err != nil {
		t.Fatalf("revoke scoped key: %v", err)
	}
	_, err = worker.GetLimitStatus(scoped, agent, MetricRequests)
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("request after revocation error = %v, want unauthenticated", err)
	}
}

type liveContractFailure struct {
	decision Decision
	err      error
}

func randomTestSuffix(t *testing.T) string {
	t.Helper()
	var raw [6]byte
	if _, err := rand.Read(raw[:]); err != nil {
		t.Fatalf("generate test suffix: %v", err)
	}
	return hex.EncodeToString(raw[:])
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func twoDigits(value int) string {
	const digits = "0123456789"
	return string([]byte{digits[(value/10)%10], digits[value%10]})
}
