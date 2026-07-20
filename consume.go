package kave

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	connect "connectrpc.com/connect"
	kernelv2 "github.com/kave-io/go-sdk/v2/internal/gen"
	"github.com/kave-io/go-sdk/v2/internal/gen/kernelv2connect"
)

// Idempotency identifies one logical consumption. Construct it with Once.
// Reusing the same key with different input is rejected by Kave.
type Idempotency struct {
	key Ref
}

// Once makes an exact quota consumption idempotent. A Temporal or application
// retry with the same key returns the original decision without consuming twice.
func Once(key string) Idempotency {
	return Idempotency{key: Ref(key)}
}

// ConsumeOption changes an optional exact quota dimension.
type ConsumeOption interface {
	applyConsume(*consumeOptions)
}

type consumeOption func(*consumeOptions)

func (option consumeOption) applyConsume(options *consumeOptions) {
	option(options)
}

type consumeOptions struct {
	model Ref
}

// ForModel attaches an optional model dimension to exact quota matching.
func ForModel(model string) ConsumeOption {
	return consumeOption(func(options *consumeOptions) {
		options.model = Ref(model)
	})
}

// DecisionStatus is the outcome of an exact quota admission decision.
type DecisionStatus string

const (
	DecisionAdmitted DecisionStatus = "admitted"
	DecisionRejected DecisionStatus = "rejected"
)

// LimitWarning reports a matching soft cap reached by an admitted consumption.
type LimitWarning struct {
	LimitID   string
	LimitKey  Ref
	Used      int64
	SoftCap   int64
	ResetAtMS int64
}

// LimitViolation reports a matching hard cap that rejected consumption.
type LimitViolation struct {
	LimitID   string
	LimitKey  Ref
	Metric    Metric
	Used      int64
	Requested int64
	HardCap   int64
	ResetAtMS int64
}

// Decision is the immutable result of an exact quota consumption.
type Decision struct {
	InvocationID string
	Status       DecisionStatus
	Replayed     bool
	Warnings     []LimitWarning
	Violations   []LimitViolation
}

// LimitExceededError contains the structured rejected decision returned by
// Kave. It matches ErrLimitExceeded with errors.Is and this type with errors.As.
type LimitExceededError struct {
	Decision Decision
	cause    error
}

func (e *LimitExceededError) Error() string {
	if e == nil || len(e.Decision.Violations) == 0 {
		return ErrLimitExceeded.Error()
	}
	violation := e.Decision.Violations[0]
	return fmt.Sprintf("%s: %s (%d used + %d requested > %d)", ErrLimitExceeded, violation.LimitKey, violation.Used, violation.Requested, violation.HardCap)
}

// Is makes errors.Is(err, ErrLimitExceeded) work while Unwrap preserves the
// transport-neutral protocol failure returned by Kave.
func (e *LimitExceededError) Is(target error) bool {
	return target == ErrLimitExceeded
}

func (e *LimitExceededError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

type kernelConsumer interface {
	Consume(context.Context, *connect.Request[kernelv2.ConsumeRequest]) (*connect.Response[kernelv2.ConsumeResponse], error)
}

func newKernelConsumer(client *Client) kernelConsumer {
	httpClient := client.baseClient
	httpClient.Jar = nil
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return ErrRedirectNotAllowed
	}
	return kernelv2connect.NewKernelServiceClient(&httpClient, client.endpoint.String())
}

// Consume atomically consumes units of metric for the scope in ctx and agent.
// The Once value is mandatory, making retry safety explicit at the call site.
// Consume performs no automatic retries.
func (c *Client) Consume(ctx context.Context, agent Agent, metric Metric, units int64, once Idempotency, options ...ConsumeOption) (Decision, error) {
	if c == nil || c.consumer == nil {
		return Decision{}, fmt.Errorf("%w: client is not configured", ErrInvalidConsume)
	}
	if ctx == nil {
		return Decision{}, fmt.Errorf("%w: context is nil", ErrInvalidConsume)
	}
	if err := agent.Validate(); err != nil {
		return Decision{}, fmt.Errorf("%w: %w", ErrInvalidConsume, err)
	}
	if err := metric.Validate(); err != nil {
		return Decision{}, fmt.Errorf("%w: %w", ErrInvalidConsume, err)
	}
	if units <= 0 {
		return Decision{}, fmt.Errorf("%w: units must be greater than zero", ErrInvalidConsume)
	}
	if err := once.key.Validate(); err != nil {
		return Decision{}, fmt.Errorf("%w: idempotency key is invalid: %w", ErrInvalidConsume, err)
	}
	scope, ok := ScopeFromContext(ctx)
	if !ok {
		return Decision{}, fmt.Errorf("%w: request context has no scope", ErrInvalidScope)
	}
	if err := scope.Validate(); err != nil {
		return Decision{}, err
	}

	settings := consumeOptions{}
	for _, option := range options {
		if option == nil {
			return Decision{}, fmt.Errorf("%w: option is nil", ErrInvalidConsume)
		}
		option.applyConsume(&settings)
	}
	if settings.model != "" {
		if err := settings.model.Validate(); err != nil {
			return Decision{}, fmt.Errorf("%w: model is invalid: %w", ErrInvalidConsume, err)
		}
	}

	request := connect.NewRequest(&kernelv2.ConsumeRequest{
		Agent: string(agent),
		Model: string(settings.model),
		Scope: &kernelv2.Scope{
			Tenant:  string(scope.Tenant),
			Actor:   string(scope.Actor),
			BillTo:  string(scope.BillTo),
			Session: string(scope.Session),
			Feature: string(scope.Feature),
		},
		Metric:         string(metric),
		Units:          units,
		IdempotencyKey: string(once.key),
	})
	request.Header().Set("Authorization", "Bearer "+c.serviceKey)

	response, err := c.consumer.Consume(ctx, request)
	if err != nil {
		if connect.CodeOf(err) == connect.CodeResourceExhausted {
			if decision, ok := rejectedDecisionFromError(err); ok {
				return decision, &LimitExceededError{Decision: decision, cause: normalizeError(err)}
			}
		}
		return Decision{}, normalizeError(err)
	}
	if response == nil || response.Msg == nil {
		return Decision{}, ErrInvalidResponse
	}

	decision, err := decisionFromProto(response.Msg)
	if err != nil {
		return Decision{}, err
	}
	switch decision.Status {
	case DecisionAdmitted:
		return decision, nil
	case DecisionRejected:
		return decision, &LimitExceededError{Decision: decision}
	default:
		return Decision{}, fmt.Errorf("%w: unknown decision status", ErrInvalidResponse)
	}
}

func rejectedDecisionFromError(err error) (Decision, bool) {
	decision := Decision{Status: DecisionRejected}
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		return decision, false
	}
	for _, detail := range connectErr.Details() {
		value, valueErr := detail.Value()
		if valueErr != nil {
			continue
		}
		exceeded, ok := value.(*kernelv2.LimitExceededDetail)
		if !ok {
			continue
		}
		if err := Ref(exceeded.GetInvocationId()).Validate(); err != nil {
			continue
		}
		violations, err := violationsFromProto(exceeded.GetViolations())
		if err != nil || len(violations) == 0 {
			continue
		}
		decision.InvocationID = exceeded.GetInvocationId()
		decision.Violations = violations
		if len(decision.Violations) > 0 {
			return decision, true
		}
	}
	return decision, false
}

func decisionFromProto(response *kernelv2.ConsumeResponse) (Decision, error) {
	if response == nil || Ref(response.GetInvocationId()).Validate() != nil {
		return Decision{}, ErrInvalidResponse
	}
	status := DecisionStatus("")
	switch response.GetStatus() {
	case kernelv2.DecisionStatus_DECISION_STATUS_ADMITTED:
		status = DecisionAdmitted
	case kernelv2.DecisionStatus_DECISION_STATUS_REJECTED:
		status = DecisionRejected
	default:
		return Decision{}, ErrInvalidResponse
	}
	warnings := make([]LimitWarning, 0, len(response.GetWarnings()))
	for _, warning := range response.GetWarnings() {
		if warning == nil || Ref(warning.GetLimitId()).Validate() != nil || Ref(warning.GetLimitKey()).Validate() != nil ||
			warning.GetUsed() < 0 || warning.GetSoftCap() < 0 || warning.GetUsed() < warning.GetSoftCap() || warning.GetResetAtMs() <= 0 {
			return Decision{}, ErrInvalidResponse
		}
		warnings = append(warnings, LimitWarning{
			LimitID: warning.GetLimitId(), LimitKey: Ref(warning.GetLimitKey()),
			Used: warning.GetUsed(), SoftCap: warning.GetSoftCap(), ResetAtMS: warning.GetResetAtMs(),
		})
	}
	violations, err := violationsFromProto(response.GetViolations())
	if err != nil {
		return Decision{}, err
	}
	if (status == DecisionAdmitted && len(violations) != 0) || (status == DecisionRejected && len(violations) == 0) {
		return Decision{}, ErrInvalidResponse
	}
	return Decision{
		InvocationID: response.GetInvocationId(), Status: status, Replayed: response.GetReplayed(),
		Warnings: warnings, Violations: violations,
	}, nil
}

func violationsFromProto(violations []*kernelv2.LimitViolation) ([]LimitViolation, error) {
	result := make([]LimitViolation, 0, len(violations))
	for _, violation := range violations {
		if violation == nil || Ref(violation.GetLimitId()).Validate() != nil || Ref(violation.GetLimitKey()).Validate() != nil ||
			Metric(violation.GetMetric()).Validate() != nil || violation.GetUsed() < 0 || violation.GetRequested() <= 0 ||
			violation.GetHardCap() < 0 || violation.GetResetAtMs() <= 0 ||
			(violation.GetUsed() <= violation.GetHardCap() && violation.GetRequested() <= violation.GetHardCap()-violation.GetUsed()) {
			return nil, ErrInvalidResponse
		}
		result = append(result, LimitViolation{
			LimitID: violation.GetLimitId(), LimitKey: Ref(violation.GetLimitKey()), Metric: Metric(violation.GetMetric()),
			Used: violation.GetUsed(), Requested: violation.GetRequested(), HardCap: violation.GetHardCap(), ResetAtMS: violation.GetResetAtMs(),
		})
	}
	return result, nil
}
