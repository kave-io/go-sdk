package kave

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	controlv1 "github.com/kave-io/kave/proto/gen/kave/control/v1"
	controlv1connect "github.com/kave-io/kave/proto/gen/kave/control/v1/controlv1connect"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestErrorPredicatesMapConnectAndGRPC(t *testing.T) {
	connectErr := wrapError(connect.NewError(connect.CodeNotFound, errors.New("missing")))
	if !IsNotFound(connectErr) {
		t.Fatalf("connect not-found was not mapped: %v", connectErr)
	}
	var sdkErr *Error
	if !errors.As(connectErr, &sdkErr) {
		t.Fatalf("wrapped error does not expose *Error")
	}
	if sdkErr.Code != CodeNotFound {
		t.Fatalf("code = %s, want %s", sdkErr.Code, CodeNotFound)
	}

	grpcErr := wrapError(status.Error(codes.PermissionDenied, "denied"))
	if !IsPermissionDenied(grpcErr) {
		t.Fatalf("grpc permission-denied was not mapped: %v", grpcErr)
	}
}

func TestDoWithRetryRetriesTransientErrors(t *testing.T) {
	attempts := 0
	got, err := doWithRetry(context.Background(), RetryPolicy{
		MaxAttempts:    3,
		Base:           time.Nanosecond,
		Cap:            time.Nanosecond,
		JitterFraction: 0,
	}, func() (string, error) {
		attempts++
		if attempts < 3 {
			return "", connect.NewError(connect.CodeUnavailable, errors.New("try again"))
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("doWithRetry returned error: %v", err)
	}
	if got != "ok" || attempts != 3 {
		t.Fatalf("got (%q, %d attempts), want (ok, 3 attempts)", got, attempts)
	}
}

func TestIterateAgentsPagesUntilNextCursorIsEmpty(t *testing.T) {
	mux := http.NewServeMux()
	path, handler := controlv1connect.NewControlPlaneServiceHandler(&iteratorControlService{})
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	defer server.Close()

	client := New(WithAddr(server.URL), WithRetry(NoRetry))
	var ids []string
	for agent, err := range client.IterateAgents(context.Background(), "env_1") {
		if err != nil {
			t.Fatalf("IterateAgents yielded error: %v", err)
		}
		ids = append(ids, agent.GetId())
	}
	if len(ids) != 3 {
		t.Fatalf("ids = %v, want three paged agents", ids)
	}
}

type iteratorControlService struct {
	controlv1connect.UnimplementedControlPlaneServiceHandler
}

func (iteratorControlService) ListAgents(_ context.Context, req *connect.Request[controlv1.ListAgentsRequest]) (*connect.Response[controlv1.ListAgentsResponse], error) {
	if req.Msg.GetCursor() == "" {
		return connect.NewResponse(&controlv1.ListAgentsResponse{
			Agents: []*controlv1.Agent{
				{Id: "agent_1", EnvId: req.Msg.GetEnvId(), Name: "one"},
				{Id: "agent_2", EnvId: req.Msg.GetEnvId(), Name: "two"},
			},
			NextCursor: "page_2",
		}), nil
	}
	return connect.NewResponse(&controlv1.ListAgentsResponse{
		Agents: []*controlv1.Agent{
			{Id: "agent_3", EnvId: req.Msg.GetEnvId(), Name: "three"},
		},
	}), nil
}
