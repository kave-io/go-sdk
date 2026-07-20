package kave

import (
	"context"
	"fmt"
)

const (
	maxIdentifierBytes = 128
	maxRefBytes        = 160
	maxMetricBytes     = 64
)

// Namespace identifies the account, application, and environment in which a
// service key operates. All three components are required.
type Namespace struct {
	Account     string
	Application string
	Environment string
}

// Agent names a static Kave workload definition.
type Agent string

// Ref is an opaque, pseudonymous scope reference such as "clinic/01J...".
type Ref string

// Metric names a quota or usage dimension, such as ai_actions.
type Metric string

const (
	MetricRequests     Metric = "requests"
	MetricInputTokens  Metric = "input_tokens"
	MetricOutputTokens Metric = "output_tokens"
	MetricCostNanoUSD  Metric = "cost_nano_usd"
)

// Scope describes the tenant and billing dimensions of one outbound AI call.
// Tenant and BillTo are required; the remaining dimensions are optional.
type Scope struct {
	Tenant  Ref
	Actor   Ref
	BillTo  Ref
	Session Ref
	Feature Ref
}

type scopeContextKey struct{}
type invocationContextKey struct{}

// WithScope returns a child context carrying scope for a Kave HTTP transport.
// Scope is validated when a request is sent.
func WithScope(ctx context.Context, scope Scope) context.Context {
	return context.WithValue(ctx, scopeContextKey{}, scope)
}

// ScopeFromContext returns the Kave scope stored in ctx, if any.
func ScopeFromContext(ctx context.Context) (Scope, bool) {
	scope, ok := ctx.Value(scopeContextKey{}).(Scope)
	return scope, ok
}

// WithInvocation binds one outbound provider call to a stable logical
// idempotency key. Temporal/application retries must reuse the same value so
// Kave records them as attempts beneath one invocation.
func WithInvocation(ctx context.Context, once Idempotency) context.Context {
	return context.WithValue(ctx, invocationContextKey{}, once)
}

// InvocationFromContext returns the logical invocation identity in ctx.
func InvocationFromContext(ctx context.Context) (Idempotency, bool) {
	once, ok := ctx.Value(invocationContextKey{}).(Idempotency)
	return once, ok
}

// Validate verifies every namespace component.
func (n Namespace) Validate() error {
	if err := Ref(n.Account).Validate(); err != nil {
		return fmt.Errorf("%w: account %v", ErrInvalidNamespace, err)
	}
	for _, component := range []struct {
		name  string
		value string
	}{
		{name: "application", value: n.Application},
		{name: "environment", value: n.Environment},
	} {
		if err := validateIdentifier(component.value); err != nil {
			return fmt.Errorf("%w: %s %v", ErrInvalidNamespace, component.name, err)
		}
	}
	return nil
}

// Validate verifies an agent name.
func (a Agent) Validate() error {
	if err := validateIdentifier(string(a)); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidAgent, err)
	}
	return nil
}

// Validate verifies an opaque scope reference.
func (r Ref) Validate() error {
	value := string(r)
	if value == "" {
		return fmt.Errorf("%w: value is required", ErrInvalidRef)
	}
	if len(value) > maxRefBytes {
		return fmt.Errorf("%w: value is too long", ErrInvalidRef)
	}
	if !isAlphaNumeric(value[0]) {
		return fmt.Errorf("%w: value must start with an ASCII letter or digit", ErrInvalidRef)
	}
	for i := 0; i < len(value); i++ {
		if !isRefByte(value[i]) {
			return fmt.Errorf("%w: value contains an unsupported byte", ErrInvalidRef)
		}
	}
	return nil
}

// Validate verifies a product or provider usage metric.
func (m Metric) Validate() error {
	value := string(m)
	if value == "" {
		return fmt.Errorf("%w: value is required", ErrInvalidMetric)
	}
	if len(value) > maxMetricBytes {
		return fmt.Errorf("%w: value is too long", ErrInvalidMetric)
	}
	if value[0] < 'a' || value[0] > 'z' {
		return fmt.Errorf("%w: value must start with a lowercase ASCII letter", ErrInvalidMetric)
	}
	for i := 0; i < len(value); i++ {
		if !isMetricByte(value[i]) {
			return fmt.Errorf("%w: value contains an unsupported byte", ErrInvalidMetric)
		}
	}
	return nil
}

// Validate verifies the required and optional scope dimensions.
func (s Scope) Validate() error {
	if s.Tenant == "" {
		return fmt.Errorf("%w: tenant is required", ErrInvalidScope)
	}
	if s.BillTo == "" {
		return fmt.Errorf("%w: bill-to is required", ErrInvalidScope)
	}
	for _, dimension := range []struct {
		name     string
		value    Ref
		required bool
	}{
		{name: "tenant", value: s.Tenant, required: true},
		{name: "actor", value: s.Actor},
		{name: "bill-to", value: s.BillTo, required: true},
		{name: "session", value: s.Session},
		{name: "feature", value: s.Feature},
	} {
		if dimension.value == "" && !dimension.required {
			continue
		}
		if err := dimension.value.Validate(); err != nil {
			return fmt.Errorf("%w: %s is invalid", ErrInvalidScope, dimension.name)
		}
	}
	return nil
}

func validateIdentifier(value string) error {
	if value == "" {
		return fmt.Errorf("is required")
	}
	if len(value) > maxIdentifierBytes {
		return fmt.Errorf("is too long")
	}
	if !isAlphaNumeric(value[0]) {
		return fmt.Errorf("must start with an ASCII letter or digit")
	}
	for i := 0; i < len(value); i++ {
		if !isIdentifierByte(value[i]) {
			return fmt.Errorf("contains an unsupported byte")
		}
	}
	return nil
}

func isIdentifierByte(b byte) bool {
	return isAlphaNumeric(b) || b == '-' || b == '_' || b == '.'
}

func isRefByte(b byte) bool {
	return isIdentifierByte(b) || b == '/' || b == ':' || b == '@'
}

func isMetricByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b == '.' || b == '_' || b == '-'
}

func isAlphaNumeric(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}
