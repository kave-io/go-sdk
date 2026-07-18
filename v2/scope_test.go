package kave

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func validScope(id string) Scope {
	return Scope{
		Tenant:  Ref("clinic/" + id),
		Actor:   Ref("user/" + id),
		BillTo:  Ref("clinic/" + id),
		Session: Ref("run/" + id),
		Feature: Ref("ai_actions"),
	}
}

func TestNamespaceAgentAndRefValidation(t *testing.T) {
	t.Parallel()

	if err := (Namespace{Account: "acct-1", Application: "app_core", Environment: "prod.eu"}).Validate(); err != nil {
		t.Fatalf("Namespace.Validate() error = %v", err)
	}
	if err := Agent("clinic-assistant").Validate(); err != nil {
		t.Fatalf("Agent.Validate() error = %v", err)
	}
	if err := Ref("clinic/01JABC:def@ghi").Validate(); err != nil {
		t.Fatalf("Ref.Validate() error = %v", err)
	}

	if err := Agent("bad/agent").Validate(); !errors.Is(err, ErrInvalidAgent) {
		t.Fatalf("Agent.Validate() error = %v, want ErrInvalidAgent", err)
	}
	if err := Ref("clinic/header\r\nspoof").Validate(); !errors.Is(err, ErrInvalidRef) {
		t.Fatalf("Ref.Validate() error = %v, want ErrInvalidRef", err)
	}
	if err := Ref(strings.Repeat("x", maxRefBytes+1)).Validate(); !errors.Is(err, ErrInvalidRef) {
		t.Fatalf("long Ref.Validate() error = %v, want ErrInvalidRef", err)
	}
}

func TestMetricValidation(t *testing.T) {
	t.Parallel()

	for _, metric := range []Metric{"ai_actions", MetricRequests, "tools.search-v2"} {
		if err := metric.Validate(); err != nil {
			t.Errorf("Metric(%q).Validate() error = %v", metric, err)
		}
	}
	for _, metric := range []Metric{"", "AI_Actions", "1request", "bad/metric", Metric(strings.Repeat("x", maxMetricBytes+1))} {
		if err := metric.Validate(); !errors.Is(err, ErrInvalidMetric) {
			t.Errorf("Metric(%q).Validate() error = %v, want ErrInvalidMetric", metric, err)
		}
	}
}

func TestScopeValidation(t *testing.T) {
	t.Parallel()

	if err := validScope("1").Validate(); err != nil {
		t.Fatalf("valid Scope.Validate() error = %v", err)
	}
	minimal := Scope{Tenant: "clinic/1", BillTo: "clinic/1"}
	if err := minimal.Validate(); err != nil {
		t.Fatalf("minimal Scope.Validate() error = %v", err)
	}

	tests := []Scope{
		{BillTo: "clinic/1"},
		{Tenant: "clinic/1"},
		{Tenant: "clinic/1", BillTo: "clinic/1", Actor: "bad actor"},
		{Tenant: "clinic/1", BillTo: "clinic/1", Session: "bad\nheader"},
	}
	for _, scope := range tests {
		if err := scope.Validate(); !errors.Is(err, ErrInvalidScope) {
			t.Errorf("Scope.Validate(%+v) error = %v, want ErrInvalidScope", scope, err)
		}
	}
}

func TestScopeContextHelpers(t *testing.T) {
	t.Parallel()

	type parentKey struct{}
	parent := context.WithValue(context.Background(), parentKey{}, "preserved")
	want := validScope("ctx")
	ctx := WithScope(parent, want)

	got, ok := ScopeFromContext(ctx)
	if !ok || got != want {
		t.Fatalf("ScopeFromContext() = (%+v, %v), want (%+v, true)", got, ok, want)
	}
	if ctx.Value(parentKey{}) != "preserved" {
		t.Fatal("WithScope did not preserve the parent context")
	}
	if _, ok := ScopeFromContext(context.Background()); ok {
		t.Fatal("ScopeFromContext unexpectedly found a scope")
	}
	once := Once("invocation/ctx")
	ctx = WithInvocation(ctx, once)
	gotOnce, ok := InvocationFromContext(ctx)
	if !ok || gotOnce != once {
		t.Fatalf("InvocationFromContext() = (%+v, %v), want (%+v, true)", gotOnce, ok, once)
	}
}
