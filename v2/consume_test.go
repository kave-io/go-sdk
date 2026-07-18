package kave

import (
	"context"
	"errors"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	connect "connectrpc.com/connect"
	kernelv2 "github.com/kave-io/kave/sdk/go/v2/internal/gen"
	"github.com/kave-io/kave/sdk/go/v2/internal/gen/kernelv2connect"
	"google.golang.org/protobuf/proto"
)

type consumeTestKernel struct {
	kernelv2connect.UnimplementedKernelServiceHandler
	consume func(context.Context, *connect.Request[kernelv2.ConsumeRequest]) (*connect.Response[kernelv2.ConsumeResponse], error)
}

func (server *consumeTestKernel) Consume(ctx context.Context, request *connect.Request[kernelv2.ConsumeRequest]) (*connect.Response[kernelv2.ConsumeResponse], error) {
	return server.consume(ctx, request)
}

func openConsumeTestClient(t *testing.T, consume func(context.Context, *connect.Request[kernelv2.ConsumeRequest]) (*connect.Response[kernelv2.ConsumeResponse], error)) *Client {
	t.Helper()
	_, handler := kernelv2connect.NewKernelServiceHandler(&consumeTestKernel{consume: consume})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return openTestClient(t, server.URL, nil)
}

func TestConsumeSendsExactScopedRequestAndMapsDecision(t *testing.T) {
	t.Parallel()

	wantScope := validScope("consume")
	client := openConsumeTestClient(t, func(_ context.Context, request *connect.Request[kernelv2.ConsumeRequest]) (*connect.Response[kernelv2.ConsumeResponse], error) {
		if got := request.Header().Get("Authorization"); got != "Bearer kv2_A1b2C3d4E5f6G7h8I9j0K1l2.AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" {
			t.Errorf("Authorization = %q", got)
		}
		if got := request.Header().Get("Kave-Account"); got != "" {
			t.Errorf("caller-controlled namespace header was sent: %q", got)
		}
		want := &kernelv2.ConsumeRequest{
			Agent: "clinic-assistant", Model: "gpt-5", Metric: "ai_actions", Units: 1, IdempotencyKey: "run/consume",
			Scope: &kernelv2.Scope{
				Tenant: "clinic/consume", Actor: "user/consume", BillTo: "clinic/consume",
				Session: "run/consume", Feature: "ai_actions",
			},
		}
		if !proto.Equal(request.Msg, want) {
			t.Errorf("request = %+v, want %+v", request.Msg, want)
		}
		return connect.NewResponse(&kernelv2.ConsumeResponse{
			InvocationId: "ivk_01", Status: kernelv2.DecisionStatus_DECISION_STATUS_ADMITTED, Replayed: true,
			Warnings: []*kernelv2.LimitWarning{{LimitId: "lim_01", LimitKey: "clinic-soft", Used: 8, SoftCap: 8, ResetAtMs: 1234}},
		}), nil
	})

	decision, err := client.Consume(WithScope(context.Background(), wantScope), Agent("clinic-assistant"), Metric("ai_actions"), 1, Once("run/consume"), ForModel("gpt-5"))
	if err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	wantDecision := Decision{
		InvocationID: "ivk_01", Status: DecisionAdmitted, Replayed: true,
		Warnings:   []LimitWarning{{LimitID: "lim_01", LimitKey: "clinic-soft", Used: 8, SoftCap: 8, ResetAtMS: 1234}},
		Violations: []LimitViolation{},
	}
	if !reflect.DeepEqual(decision, wantDecision) {
		t.Fatalf("decision = %+v, want %+v", decision, wantDecision)
	}
}

func TestConsumeRequiresValidOnceAndRejectsLocally(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	client := openConsumeTestClient(t, func(context.Context, *connect.Request[kernelv2.ConsumeRequest]) (*connect.Response[kernelv2.ConsumeResponse], error) {
		calls.Add(1)
		return connect.NewResponse(&kernelv2.ConsumeResponse{Status: kernelv2.DecisionStatus_DECISION_STATUS_ADMITTED}), nil
	})
	ctx := WithScope(context.Background(), validScope("invalid"))

	tests := []struct {
		name   string
		ctx    context.Context
		agent  Agent
		metric Metric
		units  int64
		once   Idempotency
		option ConsumeOption
		want   error
	}{
		{name: "nil context", ctx: nil, agent: "assistant", metric: "ai_actions", units: 1, once: Once("run/1"), want: ErrInvalidConsume},
		{name: "missing scope", ctx: context.Background(), agent: "assistant", metric: "ai_actions", units: 1, once: Once("run/1"), want: ErrInvalidScope},
		{name: "invalid agent", ctx: ctx, agent: "bad/agent", metric: "ai_actions", units: 1, once: Once("run/1"), want: ErrInvalidConsume},
		{name: "invalid metric", ctx: ctx, agent: "assistant", metric: "AI", units: 1, once: Once("run/1"), want: ErrInvalidConsume},
		{name: "zero units", ctx: ctx, agent: "assistant", metric: "ai_actions", units: 0, once: Once("run/1"), want: ErrInvalidConsume},
		{name: "missing once", ctx: ctx, agent: "assistant", metric: "ai_actions", units: 1, once: Idempotency{}, want: ErrInvalidConsume},
		{name: "invalid model", ctx: ctx, agent: "assistant", metric: "ai_actions", units: 1, once: Once("run/1"), option: ForModel("bad model"), want: ErrInvalidConsume},
		{name: "nil option", ctx: ctx, agent: "assistant", metric: "ai_actions", units: 1, once: Once("run/1"), option: nil, want: ErrInvalidConsume},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var options []ConsumeOption
			if test.option != nil || test.name == "nil option" {
				options = append(options, test.option)
			}
			_, err := client.Consume(test.ctx, test.agent, test.metric, test.units, test.once, options...)
			if !errors.Is(err, test.want) {
				t.Fatalf("Consume() error = %v, want %v", err, test.want)
			}
		})
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("server received %d invalid requests", got)
	}
}

func TestConsumeMapsStructuredLimitExceededError(t *testing.T) {
	t.Parallel()

	client := openConsumeTestClient(t, func(context.Context, *connect.Request[kernelv2.ConsumeRequest]) (*connect.Response[kernelv2.ConsumeResponse], error) {
		connectErr := connect.NewError(connect.CodeResourceExhausted, errors.New("quota rejected without parseable prose"))
		detail, err := connect.NewErrorDetail(&kernelv2.LimitExceededDetail{
			InvocationId: "ivk_rejected",
			Violations: []*kernelv2.LimitViolation{{
				LimitId: "lim_clinic", LimitKey: "clinic-monthly", Metric: "ai_actions",
				Used: 10, Requested: 1, HardCap: 10, ResetAtMs: 9999,
			}},
		})
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		connectErr.AddDetail(detail)
		return nil, connectErr
	})

	decision, err := client.Consume(WithScope(context.Background(), validScope("limit")), Agent("assistant"), Metric("ai_actions"), 1, Once("run/limit"))
	if !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("Consume() error = %v, want ErrLimitExceeded", err)
	}
	var exceeded *LimitExceededError
	if !errors.As(err, &exceeded) {
		t.Fatalf("Consume() error type = %T, want *LimitExceededError", err)
	}
	if !errors.Is(err, ErrResourceExhausted) {
		t.Fatalf("underlying error = %v, want ErrResourceExhausted", err)
	}
	want := Decision{
		InvocationID: "ivk_rejected", Status: DecisionRejected,
		Violations: []LimitViolation{{
			LimitID: "lim_clinic", LimitKey: "clinic-monthly", Metric: "ai_actions",
			Used: 10, Requested: 1, HardCap: 10, ResetAtMS: 9999,
		}},
	}
	if !reflect.DeepEqual(decision, want) || !reflect.DeepEqual(exceeded.Decision, want) {
		t.Fatalf("decision = %+v, error decision = %+v, want %+v", decision, exceeded.Decision, want)
	}
}

func TestConsumeDoesNotMisclassifyUnstructuredResourceExhausted(t *testing.T) {
	t.Parallel()
	client := openConsumeTestClient(t, func(context.Context, *connect.Request[kernelv2.ConsumeRequest]) (*connect.Response[kernelv2.ConsumeResponse], error) {
		return nil, connect.NewError(connect.CodeResourceExhausted, errors.New("upstream capacity"))
	})
	_, err := client.Consume(WithScope(context.Background(), validScope("capacity")), Agent("assistant"), Metric("ai_actions"), 1, Once("run/capacity"))
	if errors.Is(err, ErrLimitExceeded) || !errors.Is(err, ErrResourceExhausted) {
		t.Fatalf("Consume() error = %v, want only ErrResourceExhausted", err)
	}
}

func TestConsumeRejectsUnknownSuccessStatus(t *testing.T) {
	t.Parallel()

	client := openConsumeTestClient(t, func(context.Context, *connect.Request[kernelv2.ConsumeRequest]) (*connect.Response[kernelv2.ConsumeResponse], error) {
		return connect.NewResponse(&kernelv2.ConsumeResponse{InvocationId: "ivk_unknown"}), nil
	})
	_, err := client.Consume(WithScope(context.Background(), validScope("status")), Agent("assistant"), Metric("ai_actions"), 1, Once("run/status"))
	if !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("Consume() error = %v, want ErrInvalidResponse", err)
	}
}
