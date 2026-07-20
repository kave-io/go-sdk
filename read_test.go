package kave

import (
	"context"
	"errors"
	"testing"
	"time"

	connect "connectrpc.com/connect"
	kernelv2 "github.com/kave-io/go-sdk/v2/internal/gen"
)

type fakeKernelReader struct {
	getState         func(context.Context, *connect.Request[kernelv2.GetStateRequest]) (*connect.Response[kernelv2.State], error)
	getLimitStatus   func(context.Context, *connect.Request[kernelv2.GetLimitStatusRequest]) (*connect.Response[kernelv2.GetLimitStatusResponse], error)
	queryUsage       func(context.Context, *connect.Request[kernelv2.QueryUsageRequest]) (*connect.Response[kernelv2.QueryUsageResponse], error)
	queryInvocations func(context.Context, *connect.Request[kernelv2.QueryInvocationsRequest]) (*connect.Response[kernelv2.QueryInvocationsResponse], error)
	listTenants      func(context.Context, *connect.Request[kernelv2.ListTenantsRequest]) (*connect.Response[kernelv2.ListTenantsResponse], error)
	queryAudit       func(context.Context, *connect.Request[kernelv2.QueryAuditEventsRequest]) (*connect.Response[kernelv2.QueryAuditEventsResponse], error)
}

func (f fakeKernelReader) GetState(ctx context.Context, req *connect.Request[kernelv2.GetStateRequest]) (*connect.Response[kernelv2.State], error) {
	return f.getState(ctx, req)
}
func (f fakeKernelReader) GetLimitStatus(ctx context.Context, req *connect.Request[kernelv2.GetLimitStatusRequest]) (*connect.Response[kernelv2.GetLimitStatusResponse], error) {
	return f.getLimitStatus(ctx, req)
}
func (f fakeKernelReader) QueryUsage(ctx context.Context, req *connect.Request[kernelv2.QueryUsageRequest]) (*connect.Response[kernelv2.QueryUsageResponse], error) {
	return f.queryUsage(ctx, req)
}
func (f fakeKernelReader) QueryInvocations(ctx context.Context, req *connect.Request[kernelv2.QueryInvocationsRequest]) (*connect.Response[kernelv2.QueryInvocationsResponse], error) {
	return f.queryInvocations(ctx, req)
}
func (f fakeKernelReader) ListTenants(ctx context.Context, req *connect.Request[kernelv2.ListTenantsRequest]) (*connect.Response[kernelv2.ListTenantsResponse], error) {
	return f.listTenants(ctx, req)
}
func (f fakeKernelReader) QueryAuditEvents(ctx context.Context, req *connect.Request[kernelv2.QueryAuditEventsRequest]) (*connect.Response[kernelv2.QueryAuditEventsResponse], error) {
	return f.queryAudit(ctx, req)
}

func readTestClient(reader kernelReader) *Client {
	return &Client{serviceKey: "kv2_test.secret", reader: reader}
}

func readTestContext() context.Context {
	return WithScope(context.Background(), Scope{Tenant: "clinic/opaque", Actor: "user/opaque", BillTo: "clinic/opaque"})
}

func readTestRange() QueryRange {
	return QueryRange{From: time.Unix(1_700_000_000, 0).UTC(), To: time.Unix(1_700_003_600, 0).UTC()}
}

func TestGetStateReturnsTypedManifestWithPricing(t *testing.T) {
	t.Parallel()
	reader := fakeKernelReader{getState: func(_ context.Context, req *connect.Request[kernelv2.GetStateRequest]) (*connect.Response[kernelv2.State], error) {
		if req.Header().Get("Authorization") != "Bearer kv2_test.secret" || req.Msg.GetNamespaceId() != "nsp_prod" {
			t.Fatalf("request = %+v headers=%v", req.Msg, req.Header())
		}
		return connect.NewResponse(&kernelv2.State{
			NamespaceId: "nsp_prod", Revision: 4,
			Manifest: &kernelv2.Manifest{
				Namespace: &kernelv2.NamespaceSpec{Account: "account/acme", Application: "simorq", Environment: "prod"},
				Routes: []*kernelv2.RouteSpec{{
					Name: "openai", Provider: "openai", Secret: "provider-key", AllowedModels: []string{"gpt-safe"},
					DefaultModel: "gpt-safe", PricingRevision: 7,
					Pricing: []*kernelv2.ModelPrice{{
						Model: "gpt-safe", InputNanosPerMillionTokens: 2, OutputNanosPerMillionTokens: 8,
						CacheReadNanosPerMillionTokens: 1, CacheWriteNanosPerMillionTokens: 3,
						ReasoningNanosPerMillionTokens: 10,
					}},
				}},
				Agents: []*kernelv2.AgentSpec{{Name: "assistant", Kind: kernelv2.AgentKind_AGENT_KIND_LLM, Route: "openai", Enabled: true}},
			},
		}), nil
	}}
	state, err := readTestClient(reader).GetState(context.Background(), "nsp_prod")
	if err != nil {
		t.Fatal(err)
	}
	if state.Revision != 4 || len(state.Manifest.Routes) != 1 || state.Manifest.Routes[0].PricingRevision != 7 ||
		state.Manifest.Routes[0].Pricing[0].OutputNanosPerMillionTokens != 8 ||
		state.Manifest.Routes[0].Pricing[0].CacheReadNanosPerMillionTokens != 1 ||
		state.Manifest.Routes[0].Pricing[0].CacheWriteNanosPerMillionTokens != 3 ||
		state.Manifest.Routes[0].Pricing[0].ReasoningNanosPerMillionTokens != 10 ||
		state.Manifest.Agents[0].Name != "assistant" {
		t.Fatalf("state = %+v", state)
	}
}

func TestListTenantsReturnsBoundedOpaqueOperationalSummaries(t *testing.T) {
	t.Parallel()
	rangeFilter := readTestRange()
	lastSeen := rangeFilter.From.Add(5 * time.Minute)
	reader := fakeKernelReader{listTenants: func(_ context.Context, req *connect.Request[kernelv2.ListTenantsRequest]) (*connect.Response[kernelv2.ListTenantsResponse], error) {
		if req.Header().Get("Authorization") != "Bearer kv2_test.secret" || req.Msg.GetFromMs() != rangeFilter.From.UnixMilli() ||
			req.Msg.GetToMs() != rangeFilter.To.UnixMilli() || req.Msg.GetPageSize() != 25 || req.Msg.GetPageToken() != "opaque-in" {
			t.Fatalf("request = %+v headers=%v", req.Msg, req.Header())
		}
		return connect.NewResponse(&kernelv2.ListTenantsResponse{
			Tenants: []*kernelv2.TenantSummary{
				{Tenant: "tenant/opaque-a", BillTo: "billing/opaque-a", Status: "active", LastSeenAtMs: lastSeen.UnixMilli(), InvocationCount: 7, RequestCount: 8, CostNanoUsd: 900, ActiveLimits: 2},
				{Tenant: "tenant/opaque-b", BillTo: "billing/opaque-b", Status: "observed", LastSeenAtMs: lastSeen.UnixMilli(), InvocationCount: 1, RequestCount: 1},
			},
			NextPageToken: "opaque-out",
		}), nil
	}}

	page, err := readTestClient(reader).ListTenants(context.Background(), TenantQuery{
		Range: rangeFilter, Page: Page{Size: 25, Token: "opaque-in"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.NextPageToken != "opaque-out" || len(page.Tenants) != 2 || page.Tenants[0].Status != TenantStatusActive ||
		page.Tenants[0].Tenant != "tenant/opaque-a" || page.Tenants[0].BillTo != "billing/opaque-a" ||
		page.Tenants[0].LastSeenAt == nil || !page.Tenants[0].LastSeenAt.Equal(lastSeen) ||
		page.Tenants[0].InvocationCount != 7 || page.Tenants[0].RequestCount != 8 ||
		page.Tenants[0].CostNanoUSD != 900 || page.Tenants[0].ActiveLimits != 2 ||
		page.Tenants[1].Status != TenantStatusObserved || page.Tenants[1].ActiveLimits != 0 {
		t.Fatalf("page = %+v", page)
	}
}

func TestListTenantsRejectsContradictoryServerSummaries(t *testing.T) {
	t.Parallel()
	rangeFilter := readTestRange()
	tests := []struct {
		name    string
		summary *kernelv2.TenantSummary
	}{
		{name: "missing opaque tenant", summary: &kernelv2.TenantSummary{BillTo: "billing/a", Status: "active", ActiveLimits: 1}},
		{name: "active without limit", summary: &kernelv2.TenantSummary{Tenant: "tenant/a", BillTo: "billing/a", Status: "active"}},
		{name: "observed without sighting", summary: &kernelv2.TenantSummary{Tenant: "tenant/a", BillTo: "billing/a", Status: "observed"}},
		{name: "observed with active limit", summary: &kernelv2.TenantSummary{Tenant: "tenant/a", BillTo: "billing/a", Status: "observed", LastSeenAtMs: rangeFilter.From.Add(time.Minute).UnixMilli(), ActiveLimits: 1}},
		{name: "sighting outside range", summary: &kernelv2.TenantSummary{Tenant: "tenant/a", BillTo: "billing/a", Status: "observed", LastSeenAtMs: rangeFilter.To.UnixMilli()}},
		{name: "negative aggregate", summary: &kernelv2.TenantSummary{Tenant: "tenant/a", BillTo: "billing/a", Status: "active", ActiveLimits: 1, CostNanoUsd: -1}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader := fakeKernelReader{listTenants: func(context.Context, *connect.Request[kernelv2.ListTenantsRequest]) (*connect.Response[kernelv2.ListTenantsResponse], error) {
				return connect.NewResponse(&kernelv2.ListTenantsResponse{Tenants: []*kernelv2.TenantSummary{test.summary}}), nil
			}}
			if _, err := readTestClient(reader).ListTenants(context.Background(), TenantQuery{Range: rangeFilter}); !errors.Is(err, ErrInvalidResponse) {
				t.Fatalf("ListTenants() error = %v, want invalid response", err)
			}
		})
	}
}

func TestQueryUsageRequiresScopeAndPreservesOpaquePagination(t *testing.T) {
	t.Parallel()
	rangeFilter := readTestRange()
	reader := fakeKernelReader{queryUsage: func(_ context.Context, req *connect.Request[kernelv2.QueryUsageRequest]) (*connect.Response[kernelv2.QueryUsageResponse], error) {
		if req.Msg.GetScope().GetTenant() != "clinic/opaque" || req.Msg.GetScope().GetBillTo() != "clinic/opaque" ||
			req.Msg.GetPageSize() != 25 || req.Msg.GetPageToken() != "opaque-in" ||
			req.Msg.GetFromMs() != rangeFilter.From.UnixMilli() || req.Msg.GetToMs() != rangeFilter.To.UnixMilli() {
			t.Fatalf("request = %+v", req.Msg)
		}
		return connect.NewResponse(&kernelv2.QueryUsageResponse{
			Entries: []*kernelv2.UsageEntry{{
				Id: "use_1", InvocationId: "ivk_1", Metric: "input_tokens", Units: 12, CostNanoUsd: 42,
				RequestCount: 1, InputTokens: 12, OutputTokens: 3, CacheReadTokens: 4,
				CacheWriteTokens: 2, ReasoningTokens: 1,
				Provider: "openai", Model: "gpt-safe", Attempt: 2, EventKind: "settlement", Estimated: true,
				CreatedAtMs: rangeFilter.From.Add(time.Minute).UnixMilli(),
			}},
			NextPageToken: "opaque-out",
		}), nil
	}}
	page, err := readTestClient(reader).QueryUsage(readTestContext(), UsageQuery{
		Range: rangeFilter, Page: Page{Size: 25, Token: "opaque-in"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Entries) != 1 || page.NextPageToken != "opaque-out" || page.Entries[0].Quantity != 12 ||
		page.Entries[0].InputTokens != 12 || page.Entries[0].CacheReadTokens != 4 ||
		page.Entries[0].CacheWriteTokens != 2 || page.Entries[0].ReasoningTokens != 1 ||
		page.Entries[0].CostNanoUSD != 42 || !page.Entries[0].Estimated || page.Entries[0].Attempt != 2 || page.Entries[0].Provider != "openai" {
		t.Fatalf("page = %+v", page)
	}

	if _, err := readTestClient(reader).QueryUsage(context.Background(), UsageQuery{Range: rangeFilter}); !errors.Is(err, ErrInvalidScope) {
		t.Fatalf("missing scope error = %v", err)
	}
	invalidContext := WithScope(context.Background(), Scope{Tenant: "clinic/opaque"})
	if _, err := readTestClient(reader).QueryUsage(invalidContext, UsageQuery{Range: rangeFilter}); !errors.Is(err, ErrInvalidScope) {
		t.Fatalf("missing bill-to error = %v", err)
	}
}

func TestQueryUsageRejectsImpossibleDetailedAccounting(t *testing.T) {
	t.Parallel()
	rangeFilter := readTestRange()
	for _, entry := range []*kernelv2.UsageEntry{
		{Id: "use_1", InvocationId: "ivk_1", EventKind: "settlement", CreatedAtMs: rangeFilter.From.Add(time.Minute).UnixMilli(), InputTokens: 1, CacheReadTokens: 2},
		{Id: "use_1", InvocationId: "ivk_1", EventKind: "settlement", CreatedAtMs: rangeFilter.From.Add(time.Minute).UnixMilli(), InputTokens: 1, CacheWriteTokens: 2},
		{Id: "use_1", InvocationId: "ivk_1", EventKind: "settlement", CreatedAtMs: rangeFilter.From.Add(time.Minute).UnixMilli(), OutputTokens: 1, ReasoningTokens: 2},
	} {
		entry.Provider = "openai"
		entry.Model = "gpt-safe"
		reader := fakeKernelReader{queryUsage: func(context.Context, *connect.Request[kernelv2.QueryUsageRequest]) (*connect.Response[kernelv2.QueryUsageResponse], error) {
			return connect.NewResponse(&kernelv2.QueryUsageResponse{Entries: []*kernelv2.UsageEntry{entry}}), nil
		}}
		if _, err := readTestClient(reader).QueryUsage(readTestContext(), UsageQuery{Range: rangeFilter}); !errors.Is(err, ErrInvalidResponse) {
			t.Fatalf("QueryUsage(%+v) error = %v, want invalid response", entry, err)
		}
	}
}

func TestReadQueriesRejectUnboundedInputsBeforeTransport(t *testing.T) {
	t.Parallel()
	called := false
	reader := fakeKernelReader{queryInvocations: func(context.Context, *connect.Request[kernelv2.QueryInvocationsRequest]) (*connect.Response[kernelv2.QueryInvocationsResponse], error) {
		called = true
		return nil, nil
	}}
	client := readTestClient(reader)
	query := InvocationQuery{
		Range: QueryRange{From: time.Now().Add(-MaxQueryRange - time.Hour), To: time.Now()},
		Page:  Page{Size: MaxPageSize + 1},
	}
	if _, err := client.QueryInvocations(readTestContext(), query); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("unbounded query error = %v", err)
	}
	if called {
		t.Fatal("invalid query reached transport")
	}
}

func TestQueryInvocationsRejectsUnspecifiedServerDecision(t *testing.T) {
	t.Parallel()
	rangeFilter := readTestRange()
	reader := fakeKernelReader{queryInvocations: func(context.Context, *connect.Request[kernelv2.QueryInvocationsRequest]) (*connect.Response[kernelv2.QueryInvocationsResponse], error) {
		return connect.NewResponse(&kernelv2.QueryInvocationsResponse{Invocations: []*kernelv2.Invocation{{
			Id: "ivk_1", Agent: "assistant", Status: "settled", IdempotencyKey: "run/one",
			Scope:       &kernelv2.Scope{Tenant: "clinic/opaque", BillTo: "clinic/opaque"},
			CreatedAtMs: rangeFilter.From.Add(time.Minute).UnixMilli(),
		}}}), nil
	}}
	if _, err := readTestClient(reader).QueryInvocations(readTestContext(), InvocationQuery{Range: rangeFilter}); !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("unspecified decision error = %v, want invalid response", err)
	}
}

func TestAuditReadNormalizesErrorsAndClonesMetadata(t *testing.T) {
	t.Parallel()
	rangeFilter := readTestRange()
	metadata := map[string]string{"safe": "visible"}
	reader := fakeKernelReader{queryAudit: func(context.Context, *connect.Request[kernelv2.QueryAuditEventsRequest]) (*connect.Response[kernelv2.QueryAuditEventsResponse], error) {
		return connect.NewResponse(&kernelv2.QueryAuditEventsResponse{Events: []*kernelv2.AuditEvent{{
			Id: "aud_1", EventKind: "config.apply", ActorKind: "service_key", ActorId: "key_admin",
			ResourceKind: "namespace", ResourceId: "nsp_prod", Outcome: "succeeded", Metadata: metadata,
			CreatedAtMs: rangeFilter.From.Add(time.Minute).UnixMilli(),
		}}}), nil
	}}
	page, err := readTestClient(reader).QueryAuditEvents(context.Background(), AuditQuery{Range: rangeFilter})
	if err != nil {
		t.Fatal(err)
	}
	metadata["safe"] = "mutated"
	if page.Events[0].Metadata["safe"] != "visible" {
		t.Fatalf("metadata alias leaked: %+v", page.Events[0].Metadata)
	}

	reader.queryAudit = func(context.Context, *connect.Request[kernelv2.QueryAuditEventsRequest]) (*connect.Response[kernelv2.QueryAuditEventsResponse], error) {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("denied"))
	}
	_, err = readTestClient(reader).QueryAuditEvents(context.Background(), AuditQuery{Range: rangeFilter})
	if !IsPermissionDenied(err) {
		t.Fatalf("normalized error = %v", err)
	}
	var connectErr *connect.Error
	if errors.As(err, &connectErr) {
		t.Fatalf("Connect error leaked: %v", connectErr)
	}
}
