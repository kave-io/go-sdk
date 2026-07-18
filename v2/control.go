package kave

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	connect "connectrpc.com/connect"
	kernelv2 "github.com/kave-io/kave/sdk/go/v2/internal/gen"
	"github.com/kave-io/kave/sdk/go/v2/internal/gen/kernelv2connect"
	"google.golang.org/protobuf/types/known/emptypb"
)

type AgentKind string

const (
	AgentLLM       AgentKind = "llm"
	AgentEmbedding AgentKind = "embedding"
)

type Route struct {
	Name            string
	Provider        string
	BaseURL         string
	Secret          string
	AllowedModels   []string
	DefaultModel    string
	PricingRevision int64
	Pricing         []ModelPrice
}

// ModelPrice declares an immutable USD pricing revision for admission and
// settlement of provider-cost budgets.
type ModelPrice struct {
	Model                       Ref
	InputNanosPerMillionTokens  int64
	OutputNanosPerMillionTokens int64
}

type AgentSpec struct {
	Name    Agent
	Kind    AgentKind
	Route   string
	Enabled bool
}

type LimitWindow string

const (
	WindowAllTime LimitWindow = "all_time"
	WindowDay     LimitWindow = "day"
	WindowMonth   LimitWindow = "month"
)

type LimitSelector struct {
	Tenant  Ref
	Actor   Ref
	BillTo  Ref
	Agent   Agent
	Model   Ref
	Feature Ref
}

type Limit struct {
	Key      Ref
	Metric   Metric
	Selector LimitSelector
	Window   LimitWindow
	HardCap  int64
	SoftCap  *int64
	Enabled  bool
}

type Manifest struct {
	Namespace Namespace
	Routes    []Route
	Agents    []AgentSpec
	Limits    []Limit
}

const (
	maxManifestRoutes = 128
	maxManifestAgents = 128
	maxManifestLimits = 512
	maxRouteModels    = 256

	maxServiceKeyOperations = 8
	maxServiceKeyAgents     = 64
)

type Change struct {
	Kind         string
	ResourceKind string
	Name         Ref
	Fields       []string
}

type ApplyResult struct {
	NamespaceID string
	Revision    int64
	Applied     bool
	Changes     []Change
}

type ApplyOption interface{ applyApply(*applyOptions) }
type applyOption func(*applyOptions)

func (option applyOption) applyApply(options *applyOptions) { option(options) }

type applyOptions struct {
	dryRun, prune    bool
	expectedRevision int64
}

func DryRun() ApplyOption { return applyOption(func(options *applyOptions) { options.dryRun = true }) }
func Prune() ApplyOption  { return applyOption(func(options *applyOptions) { options.prune = true }) }
func ExpectRevision(revision int64) ApplyOption {
	return applyOption(func(options *applyOptions) { options.expectedRevision = revision })
}

type Operation string

const (
	OperationConfigApply  Operation = "config.apply"
	OperationSecretsWrite Operation = "secrets.write"
	OperationKeysManage   Operation = "keys.manage"
	OperationLimitsSync   Operation = "limits.sync"
	OperationUsageRead    Operation = "usage.read"
	OperationAuditRead    Operation = "audit.read"
	OperationConsume      Operation = "consume"
	OperationInvoke       Operation = "invoke"

	// OperationApply is the source-compatible name for manifest apply.
	OperationApply = OperationConfigApply
)

type ServiceKeySpec struct {
	Name           Ref
	Operations     []Operation
	AllowedAgents  []Agent
	CanAssertScope bool
	ExpiresAt      time.Time
	// RawKey is normally empty, causing the SDK to generate a new credential.
	// Reuse the returned value only to retry an ambiguous issuance result.
	RawKey string `json:"-"`
}

type IssuedServiceKey struct {
	ID        string
	Name      Ref
	Prefix    string
	RawKey    string `json:"-"`
	Created   bool
	CreatedAt time.Time
	ExpiresAt time.Time
}

type SecretMetadata struct {
	ID        string
	Name      Ref
	Source    string
	Version   int64
	Status    string
	UpdatedAt time.Time
}

type SyncLimitsResult struct {
	Revision int64
	Created  int32
	Updated  int32
	Disabled int32
}

type kernelController interface {
	Apply(context.Context, *connect.Request[kernelv2.ApplyRequest]) (*connect.Response[kernelv2.ApplyResponse], error)
	PutSecret(context.Context, *connect.Request[kernelv2.PutSecretRequest]) (*connect.Response[kernelv2.SecretMetadata], error)
	IssueServiceKey(context.Context, *connect.Request[kernelv2.IssueServiceKeyRequest]) (*connect.Response[kernelv2.IssuedServiceKey], error)
	RevokeServiceKey(context.Context, *connect.Request[kernelv2.RevokeServiceKeyRequest]) (*connect.Response[emptypb.Empty], error)
	RevokeSecret(context.Context, *connect.Request[kernelv2.RevokeSecretRequest]) (*connect.Response[emptypb.Empty], error)
	SyncLimits(context.Context, *connect.Request[kernelv2.SyncLimitsRequest]) (*connect.Response[kernelv2.SyncLimitsResponse], error)
}

func newKernelController(client *Client) kernelController {
	httpClient := client.baseClient
	httpClient.Jar = nil
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return ErrRedirectNotAllowed }
	return kernelv2connect.NewKernelServiceClient(&httpClient, client.endpoint.String())
}

func (c *Client) Apply(ctx context.Context, manifest Manifest, once Idempotency, options ...ApplyOption) (ApplyResult, error) {
	if err := c.validateControl(ctx, once); err != nil {
		return ApplyResult{}, err
	}
	if err := manifest.Namespace.Validate(); err != nil {
		return ApplyResult{}, err
	}
	settings := applyOptions{}
	for _, option := range options {
		if option == nil {
			return ApplyResult{}, fmt.Errorf("%w: apply option is nil", ErrInvalidArgument)
		}
		option.applyApply(&settings)
	}
	if settings.expectedRevision < 0 {
		return ApplyResult{}, fmt.Errorf("%w: expected revision must not be negative", ErrInvalidArgument)
	}
	protoManifest, err := manifestToProto(manifest)
	if err != nil {
		return ApplyResult{}, err
	}
	req := connect.NewRequest(&kernelv2.ApplyRequest{
		Manifest: protoManifest, DryRun: settings.dryRun, Prune: settings.prune,
		ExpectedRevision: settings.expectedRevision, IdempotencyKey: string(once.key),
	})
	c.authorize(req)
	response, err := c.controller.Apply(ctx, req)
	if err != nil {
		return ApplyResult{}, normalizeError(err)
	}
	if response == nil || response.Msg == nil || response.Msg.GetNamespaceId() == "" {
		return ApplyResult{}, ErrInvalidResponse
	}
	changes := make([]Change, 0, len(response.Msg.GetChanges()))
	for _, change := range response.Msg.GetChanges() {
		if change == nil {
			continue
		}
		changes = append(changes, Change{Kind: change.GetKind().String(), ResourceKind: change.GetResourceKind(), Name: Ref(change.GetName()), Fields: slices.Clone(change.GetFields())})
	}
	return ApplyResult{NamespaceID: response.Msg.GetNamespaceId(), Revision: response.Msg.GetRevision(), Applied: response.Msg.GetApplied(), Changes: changes}, nil
}

func (c *Client) PutEncryptedSecret(ctx context.Context, namespaceID string, name Ref, plaintext []byte, once Idempotency) (SecretMetadata, error) {
	if err := c.validateSecretInput(ctx, namespaceID, name, once); err != nil {
		return SecretMetadata{}, err
	}
	if len(plaintext) == 0 {
		return SecretMetadata{}, fmt.Errorf("%w: secret plaintext is required", ErrInvalidArgument)
	}
	copyOfSecret := slices.Clone(plaintext)
	defer clear(copyOfSecret)
	req := connect.NewRequest(&kernelv2.PutSecretRequest{
		NamespaceId: namespaceID, Name: string(name), IdempotencyKey: string(once.key),
		Value: &kernelv2.PutSecretRequest_Plaintext{Plaintext: copyOfSecret},
	})
	c.authorize(req)
	response, err := c.controller.PutSecret(ctx, req)
	req.Msg.Value = nil
	if err != nil {
		return SecretMetadata{}, normalizeError(err)
	}
	return secretMetadataFromProto(response)
}

func (c *Client) PutExternalSecret(ctx context.Context, namespaceID string, name Ref, uri string, once Idempotency) (SecretMetadata, error) {
	if err := c.validateSecretInput(ctx, namespaceID, name, once); err != nil {
		return SecretMetadata{}, err
	}
	if uri == "" {
		return SecretMetadata{}, fmt.Errorf("%w: external secret URI is required", ErrInvalidArgument)
	}
	req := connect.NewRequest(&kernelv2.PutSecretRequest{
		NamespaceId: namespaceID, Name: string(name), IdempotencyKey: string(once.key),
		Value: &kernelv2.PutSecretRequest_ExternalUri{ExternalUri: uri},
	})
	c.authorize(req)
	response, err := c.controller.PutSecret(ctx, req)
	if err != nil {
		return SecretMetadata{}, normalizeError(err)
	}
	return secretMetadataFromProto(response)
}

func (c *Client) IssueServiceKey(ctx context.Context, namespaceID string, spec ServiceKeySpec, once Idempotency) (IssuedServiceKey, error) {
	if err := c.validateControl(ctx, once); err != nil {
		return IssuedServiceKey{}, err
	}
	if err := Ref(namespaceID).Validate(); err != nil {
		return IssuedServiceKey{}, fmt.Errorf("%w: namespace ID is invalid", ErrInvalidArgument)
	}
	if err := validateIdentifier(string(spec.Name)); err != nil {
		return IssuedServiceKey{}, fmt.Errorf("%w: service-key name is invalid", ErrInvalidArgument)
	}
	operations, agents, err := serviceKeySpecToWire(spec)
	if err != nil {
		return IssuedServiceKey{}, err
	}
	material, err := generateServiceKeyMaterial()
	if spec.RawKey != "" {
		material, err = parseServiceKeyMaterial(spec.RawKey)
	}
	if err != nil {
		return IssuedServiceKey{}, err
	}
	pending := IssuedServiceKey{Name: spec.Name, Prefix: rawServiceKeyPrefix + material.lookupPrefix, RawKey: material.rawKey}
	var expiresAtMS int64
	if !spec.ExpiresAt.IsZero() {
		expiresAtMS = spec.ExpiresAt.UTC().UnixMilli()
	}
	req := connect.NewRequest(&kernelv2.IssueServiceKeyRequest{
		NamespaceId: namespaceID, Name: string(spec.Name), Operations: operations,
		AllowedAgents: agents, CanAssertScope: spec.CanAssertScope, ExpiresAtMs: expiresAtMS,
		IdempotencyKey: string(once.key),
		LookupPrefix:   material.lookupPrefix, SecretHash: append([]byte(nil), material.secretHash[:]...),
	})
	c.authorize(req)
	response, err := c.controller.IssueServiceKey(ctx, req)
	if err != nil {
		return pending, normalizeError(err)
	}
	if response == nil || response.Msg == nil || response.Msg.GetId() == "" {
		return pending, ErrInvalidResponse
	}
	if response.Msg.GetPrefix() != pending.Prefix || Ref(response.Msg.GetName()) != spec.Name {
		return pending, ErrInvalidResponse
	}
	return IssuedServiceKey{
		ID: response.Msg.GetId(), Name: Ref(response.Msg.GetName()), Prefix: response.Msg.GetPrefix(), RawKey: pending.RawKey,
		Created: response.Msg.GetCreated(), CreatedAt: unixMilli(response.Msg.GetCreatedAtMs()), ExpiresAt: unixMilli(response.Msg.GetExpiresAtMs()),
	}, nil
}

func serviceKeySpecToWire(spec ServiceKeySpec) ([]string, []string, error) {
	if len(spec.Operations) == 0 || len(spec.Operations) > maxServiceKeyOperations {
		return nil, nil, fmt.Errorf("%w: service-key operations must contain between 1 and %d entries", ErrInvalidArgument, maxServiceKeyOperations)
	}
	operations := make([]string, len(spec.Operations))
	seenOperations := make(map[Operation]struct{}, len(spec.Operations))
	requiresAgentAllowlist := false
	for i, operation := range spec.Operations {
		switch operation {
		case OperationConfigApply, OperationSecretsWrite, OperationKeysManage,
			OperationLimitsSync, OperationUsageRead, OperationAuditRead,
			OperationConsume, OperationInvoke:
		default:
			return nil, nil, fmt.Errorf("%w: unsupported service-key operation %q", ErrInvalidArgument, operation)
		}
		if _, exists := seenOperations[operation]; exists {
			return nil, nil, fmt.Errorf("%w: duplicate service-key operation %q", ErrInvalidArgument, operation)
		}
		seenOperations[operation] = struct{}{}
		if operation == OperationConsume || operation == OperationInvoke {
			requiresAgentAllowlist = true
		}
		operations[i] = string(operation)
	}

	if len(spec.AllowedAgents) > maxServiceKeyAgents {
		return nil, nil, fmt.Errorf("%w: service-key allowed agents must contain at most %d entries", ErrInvalidArgument, maxServiceKeyAgents)
	}
	agents := make([]string, len(spec.AllowedAgents))
	seenAgents := make(map[Agent]struct{}, len(spec.AllowedAgents))
	for i, agent := range spec.AllowedAgents {
		if err := agent.Validate(); err != nil {
			return nil, nil, fmt.Errorf("%w: service-key allowed agent %q is invalid", ErrInvalidArgument, agent)
		}
		if _, exists := seenAgents[agent]; exists {
			return nil, nil, fmt.Errorf("%w: duplicate service-key allowed agent %q", ErrInvalidArgument, agent)
		}
		seenAgents[agent] = struct{}{}
		agents[i] = string(agent)
	}
	if requiresAgentAllowlist && len(agents) == 0 {
		return nil, nil, fmt.Errorf("%w: consume and invoke service keys require an explicit agent allowlist", ErrInvalidArgument)
	}
	return operations, agents, nil
}

// RevokeServiceKey idempotently revokes a namespace-bound machine credential.
func (c *Client) RevokeServiceKey(ctx context.Context, id Ref, reason string) error {
	if err := c.validateControlRequest(ctx); err != nil {
		return err
	}
	if err := id.Validate(); err != nil {
		return fmt.Errorf("%w: service-key ID is invalid", ErrInvalidArgument)
	}
	if err := validateRevokeReason(reason); err != nil {
		return err
	}
	req := connect.NewRequest(&kernelv2.RevokeServiceKeyRequest{Id: string(id), Reason: reason})
	c.authorize(req)
	response, err := c.controller.RevokeServiceKey(ctx, req)
	if err != nil {
		return normalizeError(err)
	}
	if response == nil || response.Msg == nil {
		return ErrInvalidResponse
	}
	return nil
}

// RevokeSecret idempotently revokes a secret in the service key's namespace.
func (c *Client) RevokeSecret(ctx context.Context, id Ref, reason string) error {
	if err := c.validateControlRequest(ctx); err != nil {
		return err
	}
	if err := id.Validate(); err != nil {
		return fmt.Errorf("%w: secret ID is invalid", ErrInvalidArgument)
	}
	if err := validateRevokeReason(reason); err != nil {
		return err
	}
	req := connect.NewRequest(&kernelv2.RevokeSecretRequest{Id: string(id), Reason: reason})
	c.authorize(req)
	response, err := c.controller.RevokeSecret(ctx, req)
	if err != nil {
		return normalizeError(err)
	}
	if response == nil || response.Msg == nil {
		return ErrInvalidResponse
	}
	return nil
}

func (c *Client) SyncLimits(ctx context.Context, namespaceID string, owner Ref, revision int64, limits []Limit, once Idempotency) (SyncLimitsResult, error) {
	if err := c.validateControl(ctx, once); err != nil {
		return SyncLimitsResult{}, err
	}
	if err := Ref(namespaceID).Validate(); err != nil {
		return SyncLimitsResult{}, fmt.Errorf("%w: namespace ID is invalid", ErrInvalidArgument)
	}
	if err := owner.Validate(); err != nil {
		return SyncLimitsResult{}, fmt.Errorf("%w: limit owner is invalid", ErrInvalidArgument)
	}
	if owner == "operator" {
		return SyncLimitsResult{}, fmt.Errorf("%w: limit owner %q is reserved", ErrInvalidArgument, owner)
	}
	if revision <= 0 {
		return SyncLimitsResult{}, fmt.Errorf("%w: source revision must be positive", ErrInvalidArgument)
	}
	if len(limits) > maxManifestLimits {
		return SyncLimitsResult{}, fmt.Errorf("%w: at most %d limits may be synchronized", ErrInvalidArgument, maxManifestLimits)
	}
	protoLimits := make([]*kernelv2.LimitSpec, 0, len(limits))
	seenLimits := make(map[Ref]struct{}, len(limits))
	for _, limit := range limits {
		value, err := limitToProto(limit)
		if err != nil {
			return SyncLimitsResult{}, err
		}
		if _, exists := seenLimits[limit.Key]; exists {
			return SyncLimitsResult{}, fmt.Errorf("%w: duplicate limit key %q", ErrInvalidArgument, limit.Key)
		}
		seenLimits[limit.Key] = struct{}{}
		protoLimits = append(protoLimits, value)
	}
	req := connect.NewRequest(&kernelv2.SyncLimitsRequest{
		NamespaceId: namespaceID, Owner: string(owner), Revision: revision, Limits: protoLimits, IdempotencyKey: string(once.key),
	})
	c.authorize(req)
	response, err := c.controller.SyncLimits(ctx, req)
	if err != nil {
		return SyncLimitsResult{}, normalizeError(err)
	}
	if response == nil || response.Msg == nil {
		return SyncLimitsResult{}, ErrInvalidResponse
	}
	return SyncLimitsResult{Revision: response.Msg.GetRevision(), Created: response.Msg.GetCreated(), Updated: response.Msg.GetUpdated(), Disabled: response.Msg.GetDisabled()}, nil
}

func (c *Client) validateControl(ctx context.Context, once Idempotency) error {
	if err := c.validateControlRequest(ctx); err != nil {
		return err
	}
	if err := once.key.Validate(); err != nil {
		return fmt.Errorf("%w: idempotency key is invalid", ErrInvalidArgument)
	}
	return nil
}

func (c *Client) validateControlRequest(ctx context.Context) error {
	if c == nil || c.controller == nil {
		return fmt.Errorf("%w: client is not configured", ErrInvalidConfig)
	}
	if ctx == nil {
		return fmt.Errorf("%w: context is nil", ErrInvalidArgument)
	}
	return nil
}

func validateRevokeReason(reason string) error {
	if len(reason) > 256 || strings.ContainsAny(reason, "\r\n") {
		return fmt.Errorf("%w: revoke reason must be at most 256 bytes on one line", ErrInvalidArgument)
	}
	return nil
}

func (c *Client) validateSecretInput(ctx context.Context, namespaceID string, name Ref, once Idempotency) error {
	if err := c.validateControl(ctx, once); err != nil {
		return err
	}
	if err := Ref(namespaceID).Validate(); err != nil {
		return fmt.Errorf("%w: namespace ID is invalid", ErrInvalidArgument)
	}
	if err := validateIdentifier(string(name)); err != nil {
		return fmt.Errorf("%w: secret name is invalid", ErrInvalidArgument)
	}
	return nil
}

type headerRequest interface{ Header() http.Header }

func (c *Client) authorize(request headerRequest) {
	request.Header().Set("Authorization", "Bearer "+c.serviceKey)
}

func manifestToProto(manifest Manifest) (*kernelv2.Manifest, error) {
	if len(manifest.Routes) > maxManifestRoutes {
		return nil, fmt.Errorf("%w: at most %d routes may be applied", ErrInvalidArgument, maxManifestRoutes)
	}
	if len(manifest.Agents) > maxManifestAgents {
		return nil, fmt.Errorf("%w: at most %d agents may be applied", ErrInvalidArgument, maxManifestAgents)
	}
	if len(manifest.Limits) > maxManifestLimits {
		return nil, fmt.Errorf("%w: at most %d limits may be applied", ErrInvalidArgument, maxManifestLimits)
	}

	routes := make([]*kernelv2.RouteSpec, 0, len(manifest.Routes))
	seenRoutes := make(map[string]struct{}, len(manifest.Routes))
	for _, route := range manifest.Routes {
		prices, err := routeToProtoPrices(route)
		if err != nil {
			return nil, err
		}
		if _, exists := seenRoutes[route.Name]; exists {
			return nil, fmt.Errorf("%w: duplicate route name %q", ErrInvalidArgument, route.Name)
		}
		seenRoutes[route.Name] = struct{}{}
		routes = append(routes, &kernelv2.RouteSpec{Name: route.Name, Provider: route.Provider, BaseUrl: route.BaseURL, Secret: route.Secret, AllowedModels: slices.Clone(route.AllowedModels), DefaultModel: route.DefaultModel, PricingRevision: route.PricingRevision, Pricing: prices})
	}
	agents := make([]*kernelv2.AgentSpec, 0, len(manifest.Agents))
	seenAgents := make(map[Agent]struct{}, len(manifest.Agents))
	for _, agent := range manifest.Agents {
		if err := agent.Name.Validate(); err != nil {
			return nil, fmt.Errorf("%w: agent name is invalid", ErrInvalidArgument)
		}
		if err := validateIdentifier(agent.Route); err != nil {
			return nil, fmt.Errorf("%w: agent route %v", ErrInvalidArgument, err)
		}
		if _, exists := seenAgents[agent.Name]; exists {
			return nil, fmt.Errorf("%w: duplicate agent name %q", ErrInvalidArgument, agent.Name)
		}
		if _, exists := seenRoutes[agent.Route]; !exists {
			return nil, fmt.Errorf("%w: agent %q references unknown route %q", ErrInvalidArgument, agent.Name, agent.Route)
		}
		kind := kernelv2.AgentKind_AGENT_KIND_UNSPECIFIED
		switch agent.Kind {
		case AgentLLM:
			kind = kernelv2.AgentKind_AGENT_KIND_LLM
		case AgentEmbedding:
			kind = kernelv2.AgentKind_AGENT_KIND_EMBEDDING
		default:
			return nil, fmt.Errorf("%w: unsupported agent kind", ErrInvalidArgument)
		}
		seenAgents[agent.Name] = struct{}{}
		agents = append(agents, &kernelv2.AgentSpec{Name: string(agent.Name), Kind: kind, Route: agent.Route, Enabled: agent.Enabled})
	}
	limits := make([]*kernelv2.LimitSpec, 0, len(manifest.Limits))
	seenLimits := make(map[Ref]struct{}, len(manifest.Limits))
	for _, limit := range manifest.Limits {
		value, err := limitToProto(limit)
		if err != nil {
			return nil, err
		}
		if _, exists := seenLimits[limit.Key]; exists {
			return nil, fmt.Errorf("%w: duplicate limit key %q", ErrInvalidArgument, limit.Key)
		}
		if limit.Selector.Agent != "" {
			if _, exists := seenAgents[limit.Selector.Agent]; !exists {
				return nil, fmt.Errorf("%w: limit %q references unknown agent %q", ErrInvalidArgument, limit.Key, limit.Selector.Agent)
			}
		}
		seenLimits[limit.Key] = struct{}{}
		limits = append(limits, value)
	}
	return &kernelv2.Manifest{Namespace: &kernelv2.NamespaceSpec{Account: manifest.Namespace.Account, Application: manifest.Namespace.Application, Environment: manifest.Namespace.Environment}, Routes: routes, Agents: agents, Limits: limits}, nil
}

func routeToProtoPrices(route Route) ([]*kernelv2.ModelPrice, error) {
	if err := validateIdentifier(route.Name); err != nil {
		return nil, fmt.Errorf("%w: route name %v", ErrInvalidArgument, err)
	}
	if err := validateIdentifier(route.Provider); err != nil {
		return nil, fmt.Errorf("%w: route provider %v", ErrInvalidArgument, err)
	}
	if err := validateIdentifier(route.Secret); err != nil {
		return nil, fmt.Errorf("%w: route secret %v", ErrInvalidArgument, err)
	}
	if err := validateProviderBaseURL(route.Provider, route.BaseURL); err != nil {
		return nil, err
	}
	if len(route.AllowedModels) == 0 || len(route.AllowedModels) > maxRouteModels {
		return nil, fmt.Errorf("%w: route allowed models must contain between 1 and %d entries", ErrInvalidArgument, maxRouteModels)
	}
	if len(route.Pricing) > maxRouteModels {
		return nil, fmt.Errorf("%w: route pricing may contain at most %d entries", ErrInvalidArgument, maxRouteModels)
	}
	if route.PricingRevision <= 0 {
		return nil, fmt.Errorf("%w: route pricing revision must be positive", ErrInvalidArgument)
	}
	if err := Ref(route.DefaultModel).Validate(); err != nil {
		return nil, fmt.Errorf("%w: route default model is invalid", ErrInvalidArgument)
	}

	allowed := make(map[Ref]struct{}, len(route.AllowedModels))
	for _, rawModel := range route.AllowedModels {
		model := Ref(rawModel)
		if err := model.Validate(); err != nil {
			return nil, fmt.Errorf("%w: route allowed model %q is invalid", ErrInvalidArgument, rawModel)
		}
		if _, exists := allowed[model]; exists {
			return nil, fmt.Errorf("%w: duplicate route allowed model %q", ErrInvalidArgument, model)
		}
		allowed[model] = struct{}{}
	}
	if _, exists := allowed[Ref(route.DefaultModel)]; !exists {
		return nil, fmt.Errorf("%w: route default model must be allowed", ErrInvalidArgument)
	}

	prices := make([]*kernelv2.ModelPrice, 0, len(route.Pricing))
	seenPrices := make(map[Ref]struct{}, len(route.Pricing))
	for _, price := range route.Pricing {
		if err := price.Model.Validate(); err != nil {
			return nil, fmt.Errorf("%w: route price model is invalid", ErrInvalidArgument)
		}
		if price.InputNanosPerMillionTokens < 0 || price.OutputNanosPerMillionTokens < 0 {
			return nil, fmt.Errorf("%w: route token prices must not be negative", ErrInvalidArgument)
		}
		if _, exists := seenPrices[price.Model]; exists {
			return nil, fmt.Errorf("%w: duplicate route price model %q", ErrInvalidArgument, price.Model)
		}
		if _, exists := allowed[price.Model]; !exists {
			return nil, fmt.Errorf("%w: priced model %q is not allowed", ErrInvalidArgument, price.Model)
		}
		seenPrices[price.Model] = struct{}{}
		prices = append(prices, &kernelv2.ModelPrice{
			Model:                       string(price.Model),
			InputNanosPerMillionTokens:  price.InputNanosPerMillionTokens,
			OutputNanosPerMillionTokens: price.OutputNanosPerMillionTokens,
		})
	}
	for model := range allowed {
		if _, exists := seenPrices[model]; !exists {
			return nil, fmt.Errorf("%w: route pricing does not cover allowed model %q", ErrInvalidArgument, model)
		}
	}
	return prices, nil
}

func validateProviderBaseURL(provider, raw string) error {
	if raw == "" {
		if strings.EqualFold(provider, "openai") {
			return nil
		}
		return fmt.Errorf("%w: route base URL is required for provider %q", ErrInvalidArgument, provider)
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawPath != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return fmt.Errorf("%w: route base URL must be absolute HTTP(S) without userinfo, query, fragment, or encoded path", ErrInvalidArgument)
	}
	if u.Scheme == "http" && !isLoopbackProviderHost(u.Hostname()) {
		return fmt.Errorf("%w: plain HTTP provider URLs are allowed only for loopback hosts", ErrInvalidArgument)
	}
	return nil
}

func isLoopbackProviderHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func limitToProto(limit Limit) (*kernelv2.LimitSpec, error) {
	if err := limit.Key.Validate(); err != nil {
		return nil, fmt.Errorf("%w: limit key is invalid", ErrInvalidArgument)
	}
	if err := limit.Metric.Validate(); err != nil {
		return nil, fmt.Errorf("%w: limit metric is invalid", ErrInvalidArgument)
	}
	for name, value := range map[string]Ref{
		"tenant": limit.Selector.Tenant, "actor": limit.Selector.Actor, "bill-to": limit.Selector.BillTo,
		"model": limit.Selector.Model, "feature": limit.Selector.Feature,
	} {
		if value != "" {
			if err := value.Validate(); err != nil {
				return nil, fmt.Errorf("%w: limit selector %s is invalid", ErrInvalidArgument, name)
			}
		}
	}
	if limit.Selector.Agent != "" {
		if err := limit.Selector.Agent.Validate(); err != nil {
			return nil, fmt.Errorf("%w: limit selector agent is invalid", ErrInvalidArgument)
		}
	}
	if limit.HardCap < 0 || (limit.SoftCap != nil && (*limit.SoftCap < 0 || *limit.SoftCap > limit.HardCap)) {
		return nil, fmt.Errorf("%w: invalid limit cap", ErrInvalidArgument)
	}
	window := kernelv2.LimitWindow_LIMIT_WINDOW_UNSPECIFIED
	switch limit.Window {
	case WindowAllTime:
		window = kernelv2.LimitWindow_LIMIT_WINDOW_ALL_TIME
	case WindowDay:
		window = kernelv2.LimitWindow_LIMIT_WINDOW_DAY
	case WindowMonth:
		window = kernelv2.LimitWindow_LIMIT_WINDOW_MONTH
	default:
		return nil, fmt.Errorf("%w: invalid limit window", ErrInvalidArgument)
	}
	return &kernelv2.LimitSpec{Key: string(limit.Key), Metric: string(limit.Metric), Selector: &kernelv2.LimitSelector{Tenant: string(limit.Selector.Tenant), Actor: string(limit.Selector.Actor), BillTo: string(limit.Selector.BillTo), Agent: string(limit.Selector.Agent), Model: string(limit.Selector.Model), Feature: string(limit.Selector.Feature)}, Window: window, HardCap: limit.HardCap, SoftCap: limit.SoftCap, Enabled: limit.Enabled}, nil
}

func secretMetadataFromProto(response *connect.Response[kernelv2.SecretMetadata]) (SecretMetadata, error) {
	if response == nil || response.Msg == nil || response.Msg.GetId() == "" {
		return SecretMetadata{}, ErrInvalidResponse
	}
	return SecretMetadata{ID: response.Msg.GetId(), Name: Ref(response.Msg.GetName()), Source: response.Msg.GetSource().String(), Version: response.Msg.GetVersion(), Status: response.Msg.GetStatus(), UpdatedAt: unixMilli(response.Msg.GetUpdatedAtMs())}, nil
}

func unixMilli(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(value).UTC()
}
