package kave

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	connect "connectrpc.com/connect"
	kernelv2 "github.com/kave-io/kave/sdk/go/v2/internal/gen"
	"github.com/kave-io/kave/sdk/go/v2/internal/gen/kernelv2connect"
)

const (
	DefaultPageSize = 50
	MaxPageSize     = 200
	MaxPageToken    = 512
	MaxQueryRange   = 366 * 24 * time.Hour
)

type State struct {
	NamespaceID string
	Revision    int64
	Manifest    Manifest
}

type LimitStatus struct {
	LimitID  string
	LimitKey Ref
	Metric   Metric
	Used     int64
	Reserved int64
	HardCap  int64
	SoftCap  *int64
	ResetAt  time.Time
}

type LimitStatusOption interface{ applyLimitStatus(*limitStatusOptions) }
type limitStatusOption func(*limitStatusOptions)

func (option limitStatusOption) applyLimitStatus(options *limitStatusOptions) { option(options) }

type limitStatusOptions struct{ model Ref }

// LimitStatusForModel restricts status matching to one optional model.
func LimitStatusForModel(model Ref) LimitStatusOption {
	return limitStatusOption(func(options *limitStatusOptions) { options.model = model })
}

type Page struct {
	Size  int
	Token string
}

func (p Page) validate() error {
	if p.Size < 0 || p.Size > MaxPageSize {
		return fmt.Errorf("%w: page size must be between zero and %d", ErrInvalidArgument, MaxPageSize)
	}
	if len(p.Token) > MaxPageToken || strings.ContainsAny(p.Token, "\r\n \t") {
		return fmt.Errorf("%w: page token is invalid", ErrInvalidArgument)
	}
	return nil
}

type QueryRange struct {
	From time.Time
	To   time.Time
}

func (r QueryRange) validate() error {
	if r.From.IsZero() || r.To.IsZero() || r.From.UnixMilli() == 0 || r.To.UnixMilli() == 0 {
		return fmt.Errorf("%w: query from and to are required", ErrInvalidArgument)
	}
	if !r.From.Before(r.To) {
		return fmt.Errorf("%w: query from must be before to", ErrInvalidArgument)
	}
	if r.To.Sub(r.From) > MaxQueryRange {
		return fmt.Errorf("%w: query range must not exceed 366 days", ErrInvalidArgument)
	}
	return nil
}

type UsageQuery struct {
	Agent  Agent
	Metric Metric
	Range  QueryRange
	Page   Page
}

type UsageEntry struct {
	ID               string
	InvocationID     string
	Metric           Metric
	Quantity         int64
	RequestCount     int64
	InputTokens      int64
	OutputTokens     int64
	CacheReadTokens  int64
	CacheWriteTokens int64
	ReasoningTokens  int64
	CostNanoUSD      int64
	Estimated        bool
	Provider         Ref
	Model            Ref
	Attempt          int32
	EventKind        Ref
	CreatedAt        time.Time
}

type UsagePage struct {
	Entries       []UsageEntry
	NextPageToken string
}

type InvocationQuery struct {
	Agent  Agent
	Status DecisionStatus
	Range  QueryRange
	Page   Page
}

type Invocation struct {
	ID             string
	Agent          Agent
	Model          Ref
	Scope          Scope
	Decision       DecisionStatus
	Status         Ref
	IdempotencyKey Ref
	CreatedAt      time.Time
	SettledAt      time.Time
}

type InvocationPage struct {
	Invocations   []Invocation
	NextPageToken string
}

type AuditQuery struct {
	EventKind Ref
	Range     QueryRange
	Page      Page
}

type AuditEvent struct {
	ID           string
	EventKind    Ref
	ActorKind    Ref
	ActorID      Ref
	ResourceKind Ref
	ResourceID   Ref
	Outcome      Ref
	Metadata     map[string]string
	CreatedAt    time.Time
}

type AuditPage struct {
	Events        []AuditEvent
	NextPageToken string
}

type kernelReader interface {
	GetState(context.Context, *connect.Request[kernelv2.GetStateRequest]) (*connect.Response[kernelv2.State], error)
	GetLimitStatus(context.Context, *connect.Request[kernelv2.GetLimitStatusRequest]) (*connect.Response[kernelv2.GetLimitStatusResponse], error)
	QueryUsage(context.Context, *connect.Request[kernelv2.QueryUsageRequest]) (*connect.Response[kernelv2.QueryUsageResponse], error)
	QueryInvocations(context.Context, *connect.Request[kernelv2.QueryInvocationsRequest]) (*connect.Response[kernelv2.QueryInvocationsResponse], error)
	QueryAuditEvents(context.Context, *connect.Request[kernelv2.QueryAuditEventsRequest]) (*connect.Response[kernelv2.QueryAuditEventsResponse], error)
}

func newKernelReader(client *Client) kernelReader {
	httpClient := client.baseClient
	httpClient.Jar = nil
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return ErrRedirectNotAllowed }
	return kernelv2connect.NewKernelServiceClient(&httpClient, client.endpoint.String())
}

func (c *Client) GetState(ctx context.Context, namespaceID string) (State, error) {
	if err := c.validateRead(ctx); err != nil {
		return State{}, err
	}
	if err := Ref(namespaceID).Validate(); err != nil {
		return State{}, fmt.Errorf("%w: namespace ID is invalid", ErrInvalidArgument)
	}
	req := connect.NewRequest(&kernelv2.GetStateRequest{NamespaceId: namespaceID})
	c.authorize(req)
	response, err := c.reader.GetState(ctx, req)
	if err != nil {
		return State{}, normalizeError(err)
	}
	if response == nil || response.Msg == nil || response.Msg.GetNamespaceId() != namespaceID || response.Msg.GetRevision() <= 0 {
		return State{}, ErrInvalidResponse
	}
	manifest, err := manifestFromProto(response.Msg.GetManifest())
	if err != nil {
		return State{}, err
	}
	return State{NamespaceID: response.Msg.GetNamespaceId(), Revision: response.Msg.GetRevision(), Manifest: manifest}, nil
}

func (c *Client) GetLimitStatus(ctx context.Context, agent Agent, metric Metric, options ...LimitStatusOption) ([]LimitStatus, error) {
	if err := c.validateRead(ctx); err != nil {
		return nil, err
	}
	if err := agent.Validate(); err != nil {
		return nil, err
	}
	if err := metric.Validate(); err != nil {
		return nil, err
	}
	scope, err := readScopeFromContext(ctx)
	if err != nil {
		return nil, err
	}
	settings := limitStatusOptions{}
	for _, option := range options {
		if option == nil {
			return nil, fmt.Errorf("%w: limit-status option is nil", ErrInvalidArgument)
		}
		option.applyLimitStatus(&settings)
	}
	if settings.model != "" {
		if err := settings.model.Validate(); err != nil {
			return nil, fmt.Errorf("%w: model is invalid", ErrInvalidArgument)
		}
	}
	req := connect.NewRequest(&kernelv2.GetLimitStatusRequest{
		Scope: scopeToProto(scope), Agent: string(agent), Model: string(settings.model), Metric: string(metric),
	})
	c.authorize(req)
	response, err := c.reader.GetLimitStatus(ctx, req)
	if err != nil {
		return nil, normalizeError(err)
	}
	if response == nil || response.Msg == nil {
		return nil, ErrInvalidResponse
	}
	result := make([]LimitStatus, 0, len(response.Msg.GetLimits()))
	for _, limit := range response.Msg.GetLimits() {
		if limit == nil || limit.GetLimitId() == "" || limit.GetLimitKey() == "" || limit.GetMetric() == "" || limit.GetResetAtMs() <= 0 {
			return nil, ErrInvalidResponse
		}
		var soft *int64
		if limit.SoftCap != nil {
			value := limit.GetSoftCap()
			soft = &value
		}
		if limit.GetUsed() < 0 || limit.GetReserved() < 0 || limit.GetHardCap() < 0 ||
			(soft != nil && (*soft < 0 || *soft > limit.GetHardCap())) {
			return nil, ErrInvalidResponse
		}
		result = append(result, LimitStatus{
			LimitID: limit.GetLimitId(), LimitKey: Ref(limit.GetLimitKey()), Metric: Metric(limit.GetMetric()),
			Used: limit.GetUsed(), Reserved: limit.GetReserved(), HardCap: limit.GetHardCap(),
			SoftCap: soft, ResetAt: unixMilli(limit.GetResetAtMs()),
		})
	}
	return result, nil
}

func (c *Client) QueryUsage(ctx context.Context, query UsageQuery) (UsagePage, error) {
	if err := c.validateRead(ctx); err != nil {
		return UsagePage{}, err
	}
	scope, err := readScopeFromContext(ctx)
	if err != nil {
		return UsagePage{}, err
	}
	if query.Agent != "" {
		if err := query.Agent.Validate(); err != nil {
			return UsagePage{}, err
		}
	}
	if query.Metric != "" {
		if err := query.Metric.Validate(); err != nil {
			return UsagePage{}, err
		}
	}
	if err := validateQueryBounds(query.Range, query.Page); err != nil {
		return UsagePage{}, err
	}
	req := connect.NewRequest(&kernelv2.QueryUsageRequest{
		Scope: scopeToProto(scope), Agent: string(query.Agent), Metric: string(query.Metric),
		FromMs: query.Range.From.UTC().UnixMilli(), ToMs: query.Range.To.UTC().UnixMilli(),
		PageSize: int32(query.Page.Size), PageToken: query.Page.Token,
	})
	c.authorize(req)
	response, err := c.reader.QueryUsage(ctx, req)
	if err != nil {
		return UsagePage{}, normalizeError(err)
	}
	if response == nil || response.Msg == nil {
		return UsagePage{}, ErrInvalidResponse
	}
	result := UsagePage{Entries: make([]UsageEntry, 0, len(response.Msg.GetEntries())), NextPageToken: response.Msg.GetNextPageToken()}
	if err := validateNextToken(result.NextPageToken); err != nil {
		return UsagePage{}, err
	}
	for _, entry := range response.Msg.GetEntries() {
		if entry == nil || entry.GetId() == "" || entry.GetInvocationId() == "" || entry.GetEventKind() == "" || entry.GetCreatedAtMs() <= 0 {
			return UsagePage{}, ErrInvalidResponse
		}
		if entry.GetUnits() < 0 || entry.GetRequestCount() < 0 || entry.GetInputTokens() < 0 ||
			entry.GetOutputTokens() < 0 || entry.GetCacheReadTokens() < 0 ||
			entry.GetCacheWriteTokens() < 0 || entry.GetReasoningTokens() < 0 ||
			entry.GetCostNanoUsd() < 0 || entry.GetAttempt() < 0 {
			return UsagePage{}, ErrInvalidResponse
		}
		result.Entries = append(result.Entries, UsageEntry{
			ID: entry.GetId(), InvocationID: entry.GetInvocationId(), Metric: Metric(entry.GetMetric()),
			Quantity: entry.GetUnits(), RequestCount: entry.GetRequestCount(), InputTokens: entry.GetInputTokens(),
			OutputTokens: entry.GetOutputTokens(), CacheReadTokens: entry.GetCacheReadTokens(),
			CacheWriteTokens: entry.GetCacheWriteTokens(), ReasoningTokens: entry.GetReasoningTokens(),
			CostNanoUSD: entry.GetCostNanoUsd(), Estimated: entry.GetEstimated(),
			Provider: Ref(entry.GetProvider()), Model: Ref(entry.GetModel()),
			Attempt: entry.GetAttempt(), EventKind: Ref(entry.GetEventKind()), CreatedAt: unixMilli(entry.GetCreatedAtMs()),
		})
	}
	return result, nil
}

func (c *Client) QueryInvocations(ctx context.Context, query InvocationQuery) (InvocationPage, error) {
	if err := c.validateRead(ctx); err != nil {
		return InvocationPage{}, err
	}
	scope, err := readScopeFromContext(ctx)
	if err != nil {
		return InvocationPage{}, err
	}
	if query.Agent != "" {
		if err := query.Agent.Validate(); err != nil {
			return InvocationPage{}, err
		}
	}
	if query.Status != "" && query.Status != DecisionAdmitted && query.Status != DecisionRejected {
		return InvocationPage{}, fmt.Errorf("%w: invalid invocation decision status", ErrInvalidArgument)
	}
	if err := validateQueryBounds(query.Range, query.Page); err != nil {
		return InvocationPage{}, err
	}
	status := kernelv2.DecisionStatus_DECISION_STATUS_UNSPECIFIED
	if query.Status == DecisionAdmitted {
		status = kernelv2.DecisionStatus_DECISION_STATUS_ADMITTED
	} else if query.Status == DecisionRejected {
		status = kernelv2.DecisionStatus_DECISION_STATUS_REJECTED
	}
	req := connect.NewRequest(&kernelv2.QueryInvocationsRequest{
		Scope: scopeToProto(scope), Agent: string(query.Agent), Status: status,
		FromMs: query.Range.From.UTC().UnixMilli(), ToMs: query.Range.To.UTC().UnixMilli(),
		PageSize: int32(query.Page.Size), PageToken: query.Page.Token,
	})
	c.authorize(req)
	response, err := c.reader.QueryInvocations(ctx, req)
	if err != nil {
		return InvocationPage{}, normalizeError(err)
	}
	if response == nil || response.Msg == nil {
		return InvocationPage{}, ErrInvalidResponse
	}
	result := InvocationPage{Invocations: make([]Invocation, 0, len(response.Msg.GetInvocations())), NextPageToken: response.Msg.GetNextPageToken()}
	if err := validateNextToken(result.NextPageToken); err != nil {
		return InvocationPage{}, err
	}
	for _, invocation := range response.Msg.GetInvocations() {
		if invocation == nil || invocation.GetId() == "" || invocation.GetStatus() == "" || invocation.GetCreatedAtMs() <= 0 || invocation.GetScope() == nil {
			return InvocationPage{}, ErrInvalidResponse
		}
		decision := decisionStatusFromProto(invocation.GetDecision())
		if decision == "" {
			return InvocationPage{}, ErrInvalidResponse
		}
		itemScope := scopeFromProto(invocation.GetScope())
		if err := itemScope.Validate(); err != nil {
			return InvocationPage{}, ErrInvalidResponse
		}
		if err := Agent(invocation.GetAgent()).Validate(); err != nil {
			return InvocationPage{}, ErrInvalidResponse
		}
		if err := Ref(invocation.GetIdempotencyKey()).Validate(); err != nil {
			return InvocationPage{}, ErrInvalidResponse
		}
		settledAt := unixMilli(invocation.GetSettledAtMs())
		createdAt := unixMilli(invocation.GetCreatedAtMs())
		if !settledAt.IsZero() && settledAt.Before(createdAt) {
			return InvocationPage{}, ErrInvalidResponse
		}
		result.Invocations = append(result.Invocations, Invocation{
			ID: invocation.GetId(), Agent: Agent(invocation.GetAgent()), Model: Ref(invocation.GetModel()), Scope: itemScope,
			Decision: decision, Status: Ref(invocation.GetStatus()), IdempotencyKey: Ref(invocation.GetIdempotencyKey()),
			CreatedAt: createdAt, SettledAt: settledAt,
		})
	}
	return result, nil
}

func (c *Client) QueryAuditEvents(ctx context.Context, query AuditQuery) (AuditPage, error) {
	if err := c.validateRead(ctx); err != nil {
		return AuditPage{}, err
	}
	if query.EventKind != "" {
		if err := query.EventKind.Validate(); err != nil {
			return AuditPage{}, fmt.Errorf("%w: audit event kind is invalid", ErrInvalidArgument)
		}
	}
	if err := validateQueryBounds(query.Range, query.Page); err != nil {
		return AuditPage{}, err
	}
	req := connect.NewRequest(&kernelv2.QueryAuditEventsRequest{
		EventKind: string(query.EventKind), FromMs: query.Range.From.UTC().UnixMilli(), ToMs: query.Range.To.UTC().UnixMilli(),
		PageSize: int32(query.Page.Size), PageToken: query.Page.Token,
	})
	c.authorize(req)
	response, err := c.reader.QueryAuditEvents(ctx, req)
	if err != nil {
		return AuditPage{}, normalizeError(err)
	}
	if response == nil || response.Msg == nil {
		return AuditPage{}, ErrInvalidResponse
	}
	result := AuditPage{Events: make([]AuditEvent, 0, len(response.Msg.GetEvents())), NextPageToken: response.Msg.GetNextPageToken()}
	if err := validateNextToken(result.NextPageToken); err != nil {
		return AuditPage{}, err
	}
	for _, event := range response.Msg.GetEvents() {
		if event == nil || event.GetId() == "" || event.GetEventKind() == "" || event.GetResourceKind() == "" || event.GetResourceId() == "" || event.GetOutcome() == "" || event.GetCreatedAtMs() <= 0 {
			return AuditPage{}, ErrInvalidResponse
		}
		result.Events = append(result.Events, AuditEvent{
			ID: event.GetId(), EventKind: Ref(event.GetEventKind()), ActorKind: Ref(event.GetActorKind()),
			ActorID: Ref(event.GetActorId()), ResourceKind: Ref(event.GetResourceKind()), ResourceID: Ref(event.GetResourceId()),
			Outcome: Ref(event.GetOutcome()), Metadata: cloneMap(event.GetMetadata()), CreatedAt: unixMilli(event.GetCreatedAtMs()),
		})
	}
	return result, nil
}

func (c *Client) validateRead(ctx context.Context) error {
	if c == nil || c.reader == nil {
		return fmt.Errorf("%w: client is not configured", ErrInvalidConfig)
	}
	if ctx == nil {
		return fmt.Errorf("%w: context is nil", ErrInvalidArgument)
	}
	return nil
}

func readScopeFromContext(ctx context.Context) (Scope, error) {
	scope, ok := ScopeFromContext(ctx)
	if !ok {
		return Scope{}, fmt.Errorf("%w: request context has no scope", ErrInvalidScope)
	}
	if err := scope.Validate(); err != nil {
		return Scope{}, err
	}
	return scope, nil
}

func validateQueryBounds(queryRange QueryRange, page Page) error {
	if err := queryRange.validate(); err != nil {
		return err
	}
	return page.validate()
}

func validateNextToken(token string) error {
	if len(token) > MaxPageToken || strings.ContainsAny(token, "\r\n \t") {
		return ErrInvalidResponse
	}
	return nil
}

func manifestFromProto(input *kernelv2.Manifest) (Manifest, error) {
	if input == nil || input.GetNamespace() == nil {
		return Manifest{}, ErrInvalidResponse
	}
	namespace := Namespace{
		Account: input.GetNamespace().GetAccount(), Application: input.GetNamespace().GetApplication(),
		Environment: input.GetNamespace().GetEnvironment(),
	}
	if err := namespace.Validate(); err != nil {
		return Manifest{}, ErrInvalidResponse
	}
	manifest := Manifest{Namespace: namespace, Routes: make([]Route, 0, len(input.GetRoutes())), Agents: make([]AgentSpec, 0, len(input.GetAgents())), Limits: make([]Limit, 0, len(input.GetLimits()))}
	for _, route := range input.GetRoutes() {
		if route == nil {
			return Manifest{}, ErrInvalidResponse
		}
		item := Route{
			Name: route.GetName(), Provider: route.GetProvider(), BaseURL: route.GetBaseUrl(), Secret: route.GetSecret(),
			AllowedModels: slices.Clone(route.GetAllowedModels()), DefaultModel: route.GetDefaultModel(), PricingRevision: route.GetPricingRevision(),
			Pricing: make([]ModelPrice, 0, len(route.GetPricing())),
		}
		for _, price := range route.GetPricing() {
			if price == nil {
				return Manifest{}, ErrInvalidResponse
			}
			item.Pricing = append(item.Pricing, ModelPrice{
				Model: Ref(price.GetModel()), InputNanosPerMillionTokens: price.GetInputNanosPerMillionTokens(),
				OutputNanosPerMillionTokens: price.GetOutputNanosPerMillionTokens(),
			})
		}
		manifest.Routes = append(manifest.Routes, item)
	}
	for _, agent := range input.GetAgents() {
		if agent == nil {
			return Manifest{}, ErrInvalidResponse
		}
		kind := AgentKind("")
		switch agent.GetKind() {
		case kernelv2.AgentKind_AGENT_KIND_LLM:
			kind = AgentLLM
		case kernelv2.AgentKind_AGENT_KIND_EMBEDDING:
			kind = AgentEmbedding
		default:
			return Manifest{}, ErrInvalidResponse
		}
		manifest.Agents = append(manifest.Agents, AgentSpec{Name: Agent(agent.GetName()), Kind: kind, Route: agent.GetRoute(), Enabled: agent.GetEnabled()})
	}
	for _, limit := range input.GetLimits() {
		if limit == nil {
			return Manifest{}, ErrInvalidResponse
		}
		window := LimitWindow("")
		switch limit.GetWindow() {
		case kernelv2.LimitWindow_LIMIT_WINDOW_ALL_TIME:
			window = WindowAllTime
		case kernelv2.LimitWindow_LIMIT_WINDOW_DAY:
			window = WindowDay
		case kernelv2.LimitWindow_LIMIT_WINDOW_MONTH:
			window = WindowMonth
		default:
			return Manifest{}, ErrInvalidResponse
		}
		selector := limit.GetSelector()
		var value LimitSelector
		if selector != nil {
			value = LimitSelector{
				Tenant: Ref(selector.GetTenant()), Actor: Ref(selector.GetActor()), BillTo: Ref(selector.GetBillTo()),
				Agent: Agent(selector.GetAgent()), Model: Ref(selector.GetModel()), Feature: Ref(selector.GetFeature()),
			}
		}
		var soft *int64
		if limit.SoftCap != nil {
			copy := limit.GetSoftCap()
			soft = &copy
		}
		manifest.Limits = append(manifest.Limits, Limit{
			Key: Ref(limit.GetKey()), Metric: Metric(limit.GetMetric()), Selector: value, Window: window,
			HardCap: limit.GetHardCap(), SoftCap: soft, Enabled: limit.GetEnabled(),
		})
	}
	if _, err := manifestToProto(manifest); err != nil {
		return Manifest{}, ErrInvalidResponse
	}
	return manifest, nil
}

func scopeToProto(scope Scope) *kernelv2.Scope {
	return &kernelv2.Scope{
		Tenant: string(scope.Tenant), Actor: string(scope.Actor), BillTo: string(scope.BillTo),
		Session: string(scope.Session), Feature: string(scope.Feature),
	}
}

func scopeFromProto(scope *kernelv2.Scope) Scope {
	if scope == nil {
		return Scope{}
	}
	return Scope{
		Tenant: Ref(scope.GetTenant()), Actor: Ref(scope.GetActor()), BillTo: Ref(scope.GetBillTo()),
		Session: Ref(scope.GetSession()), Feature: Ref(scope.GetFeature()),
	}
}

func decisionStatusFromProto(status kernelv2.DecisionStatus) DecisionStatus {
	switch status {
	case kernelv2.DecisionStatus_DECISION_STATUS_ADMITTED:
		return DecisionAdmitted
	case kernelv2.DecisionStatus_DECISION_STATUS_REJECTED:
		return DecisionRejected
	default:
		return ""
	}
}

func cloneMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
