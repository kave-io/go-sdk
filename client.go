// Package kave provides a Go client for the Kave control plane.
// It wraps the Connect-RPC generated clients for ControlPlane, Runtime, and Audit services.
package kave

import (
	"net/http"

	"connectrpc.com/connect"
	auditv1connect "github.com/kave-io/kave/proto/gen/kave/audit/v1/auditv1connect"
	controlv1connect "github.com/kave-io/kave/proto/gen/kave/control/v1/controlv1connect"
	runtimev1connect "github.com/kave-io/kave/proto/gen/kave/runtime/v1/runtimev1connect"
)

// Client is the Kave SDK entry point. Use New to construct one.
type Client struct {
	Control controlv1connect.ControlPlaneServiceClient
	RBAC    controlv1connect.RBACServiceClient
	Runtime runtimev1connect.RuntimeServiceClient
	Audit   auditv1connect.AuditServiceClient
}

// New creates a Kave client. Defaults to http://localhost:8080 over Connect protocol.
// Use WithGRPC() for binary gRPC, WithToken() for bearer auth.
func New(opts ...Option) *Client {
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}

	var httpClient connect.HTTPClient = o.httpClient
	if o.token != "" || o.userAgent != "" {
		httpClient = &headerClient{inner: o.httpClient, token: o.token, userAgent: o.userAgent}
	}

	interceptors := []connect.Interceptor{
		retryInterceptor(o.retryPolicy),
		observabilityInterceptor(o.logger, o.tracer),
		errorInterceptor(),
	}
	connectOpts := append([]connect.ClientOption{
		connect.WithCompressMinBytes(1024),
		connect.WithInterceptors(interceptors...),
	}, o.connectOpts...)

	return &Client{
		Control: controlv1connect.NewControlPlaneServiceClient(httpClient, o.baseURL, connectOpts...),
		RBAC:    controlv1connect.NewRBACServiceClient(httpClient, o.baseURL, connectOpts...),
		Runtime: runtimev1connect.NewRuntimeServiceClient(httpClient, o.baseURL, connectOpts...),
		Audit:   auditv1connect.NewAuditServiceClient(httpClient, o.baseURL, connectOpts...),
	}
}

// headerClient injects process-wide SDK headers on every request.
type headerClient struct {
	inner     connect.HTTPClient
	token     string
	userAgent string
}

func (h *headerClient) Do(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	if h.token != "" {
		req.Header.Set("Authorization", "Bearer "+h.token)
	}
	if h.userAgent != "" {
		req.Header.Set("User-Agent", h.userAgent)
	}
	return h.inner.Do(req)
}
