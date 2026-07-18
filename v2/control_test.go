package kave

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	connect "connectrpc.com/connect"
	kernelv2 "github.com/kave-io/kave/sdk/go/v2/internal/gen"
	"google.golang.org/protobuf/types/known/emptypb"
)

type fakeKernelController struct {
	apply        func(context.Context, *connect.Request[kernelv2.ApplyRequest]) (*connect.Response[kernelv2.ApplyResponse], error)
	put          func(context.Context, *connect.Request[kernelv2.PutSecretRequest]) (*connect.Response[kernelv2.SecretMetadata], error)
	issue        func(context.Context, *connect.Request[kernelv2.IssueServiceKeyRequest]) (*connect.Response[kernelv2.IssuedServiceKey], error)
	revokeKey    func(context.Context, *connect.Request[kernelv2.RevokeServiceKeyRequest]) (*connect.Response[emptypb.Empty], error)
	revokeSecret func(context.Context, *connect.Request[kernelv2.RevokeSecretRequest]) (*connect.Response[emptypb.Empty], error)
	sync         func(context.Context, *connect.Request[kernelv2.SyncLimitsRequest]) (*connect.Response[kernelv2.SyncLimitsResponse], error)
}

func (f fakeKernelController) Apply(ctx context.Context, req *connect.Request[kernelv2.ApplyRequest]) (*connect.Response[kernelv2.ApplyResponse], error) {
	return f.apply(ctx, req)
}
func (f fakeKernelController) PutSecret(ctx context.Context, req *connect.Request[kernelv2.PutSecretRequest]) (*connect.Response[kernelv2.SecretMetadata], error) {
	return f.put(ctx, req)
}
func (f fakeKernelController) IssueServiceKey(ctx context.Context, req *connect.Request[kernelv2.IssueServiceKeyRequest]) (*connect.Response[kernelv2.IssuedServiceKey], error) {
	return f.issue(ctx, req)
}
func (f fakeKernelController) RevokeServiceKey(ctx context.Context, req *connect.Request[kernelv2.RevokeServiceKeyRequest]) (*connect.Response[emptypb.Empty], error) {
	return f.revokeKey(ctx, req)
}
func (f fakeKernelController) RevokeSecret(ctx context.Context, req *connect.Request[kernelv2.RevokeSecretRequest]) (*connect.Response[emptypb.Empty], error) {
	return f.revokeSecret(ctx, req)
}
func (f fakeKernelController) SyncLimits(ctx context.Context, req *connect.Request[kernelv2.SyncLimitsRequest]) (*connect.Response[kernelv2.SyncLimitsResponse], error) {
	return f.sync(ctx, req)
}

func controlTestClient(controller kernelController) *Client {
	return &Client{serviceKey: "kv2_test.secret", controller: controller}
}

func TestApplySendsOneDeclarativeManifest(t *testing.T) {
	t.Parallel()
	controller := fakeKernelController{}
	controller.apply = func(_ context.Context, req *connect.Request[kernelv2.ApplyRequest]) (*connect.Response[kernelv2.ApplyResponse], error) {
		if req.Header().Get("Authorization") != "Bearer kv2_test.secret" {
			t.Fatal("service key was not attached")
		}
		if req.Msg.GetIdempotencyKey() != "deploy/1" || len(req.Msg.GetManifest().GetAgents()) != 1 || len(req.Msg.GetManifest().GetLimits()) != 1 {
			t.Fatalf("request = %+v", req.Msg)
		}
		price := req.Msg.GetManifest().GetRoutes()[0].GetPricing()
		if len(price) != 1 || price[0].GetModel() != "gpt-5" || price[0].GetInputNanosPerMillionTokens() != 1_250_000_000 {
			t.Fatalf("pricing = %+v", price)
		}
		return connect.NewResponse(&kernelv2.ApplyResponse{NamespaceId: "nsp_test", Revision: 2, Applied: true}), nil
	}
	client := controlTestClient(controller)
	result, err := client.Apply(context.Background(), Manifest{
		Namespace: Namespace{Account: "account/acme", Application: "simorq", Environment: "production"},
		Routes: []Route{{
			Name: "openai", Provider: "openai", Secret: "openai", AllowedModels: []string{"gpt-5"}, DefaultModel: "gpt-5",
			PricingRevision: 1, Pricing: []ModelPrice{{Model: "gpt-5", InputNanosPerMillionTokens: 1_250_000_000, OutputNanosPerMillionTokens: 10_000_000_000}},
		}},
		Agents: []AgentSpec{{Name: "clinic-assistant", Kind: AgentLLM, Route: "openai", Enabled: true}},
		Limits: []Limit{{Key: "requests", Metric: MetricRequests, Selector: LimitSelector{Agent: "clinic-assistant"}, Window: WindowMonth, HardCap: 10, Enabled: true}},
	}, Once("deploy/1"))
	if err != nil || result.NamespaceID != "nsp_test" || result.Revision != 2 {
		t.Fatalf("Apply() = %+v, %v", result, err)
	}
}

func TestApplyRejectsAmbiguousPricingBeforeNetwork(t *testing.T) {
	t.Parallel()
	called := false
	controller := fakeKernelController{apply: func(context.Context, *connect.Request[kernelv2.ApplyRequest]) (*connect.Response[kernelv2.ApplyResponse], error) {
		called = true
		return nil, nil
	}}
	manifest := Manifest{
		Namespace: Namespace{Account: "account/acme", Application: "simorq", Environment: "production"},
		Routes: []Route{{
			Name: "openai", Provider: "openai", Secret: "openai", AllowedModels: []string{"gpt-5"},
			Pricing: []ModelPrice{{Model: "gpt-5", InputNanosPerMillionTokens: 1}},
		}},
	}
	if _, err := controlTestClient(controller).Apply(context.Background(), manifest, Once("deploy/pricing")); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("Apply() error = %v, want invalid argument", err)
	}
	if called {
		t.Fatal("invalid pricing reached transport")
	}
}

func TestManifestValidationMatchesRouteActivationContract(t *testing.T) {
	t.Parallel()
	valid := func() Manifest {
		return Manifest{
			Namespace: Namespace{Account: "account/acme", Application: "simorq", Environment: "production"},
			Routes: []Route{{
				Name: "openai", Provider: "openai", BaseURL: "https://api.openai.example/v1", Secret: "openai",
				AllowedModels: []string{"gpt-5"}, DefaultModel: "gpt-5", PricingRevision: 1,
				Pricing: []ModelPrice{{Model: "gpt-5"}},
			}},
			Agents: []AgentSpec{{Name: "assistant", Kind: AgentLLM, Route: "openai", Enabled: true}},
			Limits: []Limit{{Key: "assistant-requests", Metric: MetricRequests, Selector: LimitSelector{Agent: "assistant"}, Window: WindowMonth, HardCap: 10, Enabled: true}},
		}
	}

	if _, err := manifestToProto(valid()); err != nil {
		t.Fatalf("valid manifest: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Manifest)
	}{
		{name: "no allowed models", mutate: func(m *Manifest) { m.Routes[0].AllowedModels = nil }},
		{name: "duplicate allowed model", mutate: func(m *Manifest) { m.Routes[0].AllowedModels = []string{"gpt-5", "gpt-5"} }},
		{name: "missing default model", mutate: func(m *Manifest) { m.Routes[0].DefaultModel = "" }},
		{name: "default model not allowed", mutate: func(m *Manifest) { m.Routes[0].DefaultModel = "gpt-other" }},
		{name: "missing pricing revision", mutate: func(m *Manifest) { m.Routes[0].PricingRevision = 0 }},
		{name: "incomplete pricing", mutate: func(m *Manifest) {
			m.Routes[0].AllowedModels = []string{"gpt-5", "gpt-other"}
		}},
		{name: "missing custom provider URL", mutate: func(m *Manifest) {
			m.Routes[0].Provider = "compatible-provider"
			m.Routes[0].BaseURL = ""
		}},
		{name: "external plain http", mutate: func(m *Manifest) { m.Routes[0].BaseURL = "http://provider.example/v1" }},
		{name: "URL userinfo", mutate: func(m *Manifest) { m.Routes[0].BaseURL = "https://user@provider.example/v1" }},
		{name: "URL query", mutate: func(m *Manifest) { m.Routes[0].BaseURL = "https://provider.example/v1?secret=value" }},
		{name: "URL fragment", mutate: func(m *Manifest) { m.Routes[0].BaseURL = "https://provider.example/v1#fragment" }},
		{name: "URL encoded path", mutate: func(m *Manifest) { m.Routes[0].BaseURL = "https://provider.example/%76%31" }},
		{name: "unknown agent route", mutate: func(m *Manifest) { m.Agents[0].Route = "missing" }},
		{name: "unknown limit agent", mutate: func(m *Manifest) { m.Limits[0].Selector.Agent = "missing" }},
		{name: "duplicate route", mutate: func(m *Manifest) { m.Routes = append(m.Routes, m.Routes[0]) }},
		{name: "duplicate agent", mutate: func(m *Manifest) { m.Agents = append(m.Agents, m.Agents[0]) }},
		{name: "duplicate limit", mutate: func(m *Manifest) { m.Limits = append(m.Limits, m.Limits[0]) }},
		{name: "too many models", mutate: func(m *Manifest) {
			m.Routes[0].AllowedModels = make([]string, maxRouteModels+1)
			for i := range m.Routes[0].AllowedModels {
				m.Routes[0].AllowedModels[i] = fmt.Sprintf("model-%d", i)
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest := valid()
			test.mutate(&manifest)
			if _, err := manifestToProto(manifest); !errors.Is(err, ErrInvalidArgument) {
				t.Fatalf("manifestToProto() error = %v, want invalid argument", err)
			}
		})
	}
}

func TestPutEncryptedSecretCopiesAndClearsRequestMaterial(t *testing.T) {
	t.Parallel()
	original := []byte("provider-secret")
	var captured []byte
	controller := fakeKernelController{}
	controller.put = func(_ context.Context, req *connect.Request[kernelv2.PutSecretRequest]) (*connect.Response[kernelv2.SecretMetadata], error) {
		captured = slices.Clone(req.Msg.GetPlaintext())
		return connect.NewResponse(&kernelv2.SecretMetadata{Id: "sec_1", Name: "openai", Status: "active"}), nil
	}
	metadata, err := controlTestClient(controller).PutEncryptedSecret(context.Background(), "nsp_test", "openai", original, Once("secret/1"))
	if err != nil || metadata.ID != "sec_1" || string(captured) != "provider-secret" {
		t.Fatalf("PutEncryptedSecret() = %+v, %v, captured=%q", metadata, err, captured)
	}
	if string(original) != "provider-secret" {
		t.Fatal("caller-owned secret was mutated")
	}
}

func TestIssueServiceKeyDistinguishesCreationFromReplay(t *testing.T) {
	t.Parallel()
	createdAt := time.Unix(1_700_000_000, 0).UTC()
	var sentLookup string
	var sentHash []byte
	controller := fakeKernelController{issue: func(_ context.Context, req *connect.Request[kernelv2.IssueServiceKeyRequest]) (*connect.Response[kernelv2.IssuedServiceKey], error) {
		if req.Msg.GetName() != "worker" || req.Msg.GetIdempotencyKey() != "key/worker/1" {
			t.Fatalf("request = %+v", req.Msg)
		}
		if len(req.Msg.GetLookupPrefix()) != 24 || len(req.Msg.GetSecretHash()) != sha256.Size {
			t.Fatalf("client key material = prefix %q hash length %d", req.Msg.GetLookupPrefix(), len(req.Msg.GetSecretHash()))
		}
		sentLookup = req.Msg.GetLookupPrefix()
		sentHash = slices.Clone(req.Msg.GetSecretHash())
		return connect.NewResponse(&kernelv2.IssuedServiceKey{
			Id: "key_1", Name: "worker", Prefix: "kv2_" + req.Msg.GetLookupPrefix(),
			Created: true, CreatedAtMs: createdAt.UnixMilli(),
		}), nil
	}}
	issued, err := controlTestClient(controller).IssueServiceKey(context.Background(), "nsp_test", ServiceKeySpec{
		Name: "worker", Operations: []Operation{OperationApply},
	}, Once("key/worker/1"))
	if err != nil || !issued.Created || issued.RawKey == "" || !issued.CreatedAt.Equal(createdAt) {
		t.Fatalf("IssueServiceKey() = %+v, %v", issued, err)
	}
	local, err := parseServiceKeyMaterial(issued.RawKey)
	if err != nil || local.lookupPrefix != sentLookup || !slices.Equal(local.secretHash[:], sentHash) {
		t.Fatalf("local/wire verifier mismatch: parse=%v prefix_equal=%v hash_equal=%v", err, local.lookupPrefix == sentLookup, slices.Equal(local.secretHash[:], sentHash))
	}
	encoded, err := json.Marshal(issued)
	if err != nil || bytes.Contains(encoded, []byte(issued.RawKey)) {
		t.Fatalf("JSON redaction failed: marshal=%v contains_raw=%v", err, bytes.Contains(encoded, []byte(issued.RawKey)))
	}

	controller.issue = func(_ context.Context, req *connect.Request[kernelv2.IssueServiceKeyRequest]) (*connect.Response[kernelv2.IssuedServiceKey], error) {
		material, err := parseServiceKeyMaterial(issued.RawKey)
		if err != nil || req.Msg.GetLookupPrefix() != material.lookupPrefix || !slices.Equal(req.Msg.GetSecretHash(), material.secretHash[:]) {
			t.Fatalf("replay did not reuse local material: %+v, %v", req.Msg, err)
		}
		return connect.NewResponse(&kernelv2.IssuedServiceKey{Id: "key_1", Name: "worker", Prefix: issued.Prefix, CreatedAtMs: createdAt.UnixMilli()}), nil
	}
	replayed, err := controlTestClient(controller).IssueServiceKey(context.Background(), "nsp_test", ServiceKeySpec{Name: "worker", Operations: []Operation{OperationApply}, RawKey: issued.RawKey}, Once("key/worker/1"))
	if err != nil || replayed.Created || replayed.RawKey != issued.RawKey || replayed.Prefix != issued.Prefix {
		t.Fatalf("replayed IssueServiceKey() = %+v, %v", replayed, err)
	}
}

func TestIssueServiceKeyReturnsLocalMaterialOnAmbiguousTransportError(t *testing.T) {
	t.Parallel()
	controller := fakeKernelController{issue: func(context.Context, *connect.Request[kernelv2.IssueServiceKeyRequest]) (*connect.Response[kernelv2.IssuedServiceKey], error) {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("response lost"))
	}}
	pending, err := controlTestClient(controller).IssueServiceKey(context.Background(), "nsp_test", ServiceKeySpec{
		Name: "worker", Operations: []Operation{OperationApply},
	}, Once("key/worker/ambiguous"))
	if !errors.Is(err, ErrUnavailable) || pending.RawKey == "" || pending.Prefix == "" {
		t.Fatalf("IssueServiceKey() = %+v, %v", pending, err)
	}
	if material, parseErr := parseServiceKeyMaterial(pending.RawKey); parseErr != nil || pending.Prefix != "kv2_"+material.lookupPrefix {
		t.Fatalf("pending material = %+v, parse error %v", pending, parseErr)
	}
}

func TestIssueServiceKeyValidatesOperationsAndAgentAllowlistBeforeNetwork(t *testing.T) {
	t.Parallel()
	allOperations := []Operation{
		OperationConfigApply, OperationSecretsWrite, OperationKeysManage, OperationLimitsSync,
		OperationUsageRead, OperationAuditRead, OperationConsume, OperationInvoke,
	}
	agents := func(count int) []Agent {
		values := make([]Agent, count)
		for i := range values {
			values[i] = Agent(fmt.Sprintf("agent-%02d", i))
		}
		return values
	}

	transportCalls := 0
	controller := fakeKernelController{issue: func(_ context.Context, req *connect.Request[kernelv2.IssueServiceKeyRequest]) (*connect.Response[kernelv2.IssuedServiceKey], error) {
		transportCalls++
		return connect.NewResponse(&kernelv2.IssuedServiceKey{
			Id: "key_worker", Name: req.Msg.GetName(), Prefix: "kv2_" + req.Msg.GetLookupPrefix(), Created: true,
		}), nil
	}}
	client := controlTestClient(controller)

	if _, err := client.IssueServiceKey(context.Background(), "nsp_test", ServiceKeySpec{
		Name: "all-capabilities", Operations: allOperations, AllowedAgents: []Agent{"assistant"},
	}, Once("key/all-capabilities")); err != nil {
		t.Fatalf("all supported operations: %v", err)
	}
	maxAgents := agents(maxServiceKeyAgents)
	if _, err := client.IssueServiceKey(context.Background(), "nsp_test", ServiceKeySpec{
		Name: "max-agents", Operations: []Operation{OperationConsume}, AllowedAgents: maxAgents,
	}, Once("key/max-agents")); err != nil {
		t.Fatalf("maximum allowed-agent count: %v", err)
	}
	if transportCalls != 2 {
		t.Fatalf("valid transport calls = %d, want 2", transportCalls)
	}

	tests := []struct {
		name        string
		namespaceID string
		spec        ServiceKeySpec
	}{
		{
			name: "missing operations", namespaceID: "nsp_test",
			spec: ServiceKeySpec{Name: "worker"},
		},
		{
			name: "too many operations", namespaceID: "nsp_test",
			spec: ServiceKeySpec{Name: "worker", Operations: []Operation{
				OperationUsageRead, OperationUsageRead, OperationUsageRead,
				OperationUsageRead, OperationUsageRead, OperationUsageRead,
				OperationUsageRead, OperationUsageRead, OperationUsageRead,
			}},
		},
		{
			name: "unsupported operation", namespaceID: "nsp_test",
			spec: ServiceKeySpec{Name: "worker", Operations: []Operation{"root"}},
		},
		{
			name: "duplicate operation", namespaceID: "nsp_test",
			spec: ServiceKeySpec{Name: "worker", Operations: []Operation{OperationUsageRead, OperationUsageRead}},
		},
		{
			name: "too many allowed agents", namespaceID: "nsp_test",
			spec: ServiceKeySpec{Name: "worker", Operations: []Operation{OperationUsageRead}, AllowedAgents: agents(maxServiceKeyAgents + 1)},
		},
		{
			name: "duplicate allowed agent", namespaceID: "nsp_test",
			spec: ServiceKeySpec{Name: "worker", Operations: []Operation{OperationUsageRead}, AllowedAgents: []Agent{"assistant", "assistant"}},
		},
		{
			name: "consume without allowlist", namespaceID: "nsp_test",
			spec: ServiceKeySpec{Name: "worker", Operations: []Operation{OperationConsume}},
		},
		{
			name: "invoke without allowlist", namespaceID: "nsp_test",
			spec: ServiceKeySpec{Name: "worker", Operations: []Operation{OperationInvoke}},
		},
		{
			name: "invalid namespace ref", namespaceID: "namespace invalid",
			spec: ServiceKeySpec{Name: "worker", Operations: []Operation{OperationUsageRead}},
		},
		{
			name: "resource ref used as service-key name", namespaceID: "nsp_test",
			spec: ServiceKeySpec{Name: "worker/one", Operations: []Operation{OperationUsageRead}},
		},
		{
			name: "resource ref used as allowed-agent name", namespaceID: "nsp_test",
			spec: ServiceKeySpec{Name: "worker", Operations: []Operation{OperationUsageRead}, AllowedAgents: []Agent{"assistant/one"}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			before := transportCalls
			_, err := client.IssueServiceKey(context.Background(), test.namespaceID, test.spec, Once("key/invalid"))
			if !errors.Is(err, ErrInvalidArgument) {
				t.Fatalf("IssueServiceKey() error = %v, want invalid argument", err)
			}
			if transportCalls != before {
				t.Fatal("invalid service-key request reached transport")
			}
		})
	}
}

func TestTypedRevocationsValidateAndAuthorize(t *testing.T) {
	t.Parallel()
	controller := fakeKernelController{
		revokeKey: func(_ context.Context, req *connect.Request[kernelv2.RevokeServiceKeyRequest]) (*connect.Response[emptypb.Empty], error) {
			if req.Msg.GetId() != "key_worker" || req.Msg.GetReason() != "rotation" || req.Header().Get("Authorization") == "" {
				t.Fatalf("revoke key request = %+v headers=%v", req.Msg, req.Header())
			}
			return connect.NewResponse(&emptypb.Empty{}), nil
		},
		revokeSecret: func(_ context.Context, req *connect.Request[kernelv2.RevokeSecretRequest]) (*connect.Response[emptypb.Empty], error) {
			if req.Msg.GetId() != "sec_openai" || req.Msg.GetReason() != "compromised" || req.Header().Get("Authorization") == "" {
				t.Fatalf("revoke secret request = %+v headers=%v", req.Msg, req.Header())
			}
			return connect.NewResponse(&emptypb.Empty{}), nil
		},
	}
	client := controlTestClient(controller)
	if err := client.RevokeServiceKey(context.Background(), "key_worker", "rotation"); err != nil {
		t.Fatal(err)
	}
	if err := client.RevokeSecret(context.Background(), "sec_openai", "compromised"); err != nil {
		t.Fatal(err)
	}
	if err := client.RevokeSecret(context.Background(), "sec_openai", "bad\nreason"); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("newline reason error = %v", err)
	}
}

func TestControlErrorsAreTransportNeutral(t *testing.T) {
	t.Parallel()
	controller := fakeKernelController{}
	controller.sync = func(context.Context, *connect.Request[kernelv2.SyncLimitsRequest]) (*connect.Response[kernelv2.SyncLimitsResponse], error) {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("denied"))
	}
	_, err := controlTestClient(controller).SyncLimits(context.Background(), "nsp_test", "simorq", 1, nil, Once("limits/1"))
	if !IsPermissionDenied(err) || !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("SyncLimits() error = %v", err)
	}
	var connectErr *connect.Error
	if errors.As(err, &connectErr) {
		t.Fatalf("Connect error leaked: %v", connectErr)
	}
}
