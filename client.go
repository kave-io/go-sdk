// Package kave provides a self-contained Go client for the Kave control plane.
//
// The public API is transport-neutral: methods accept SDK Input types and
// return SDK models (see models.go, enums.go, inputs.go). Consumers do not need
// to import the generated proto packages. For advanced use of RPCs the typed
// surface does not cover, Client.Raw exposes the underlying Connect clients.
package kave

import (
	"net/http"

	"connectrpc.com/connect"
	auditv1connect "github.com/kave-io/kave/proto/gen/kave/audit/v1/auditv1connect"
	controlv1connect "github.com/kave-io/kave/proto/gen/kave/control/v1/controlv1connect"
	runtimev1connect "github.com/kave-io/kave/proto/gen/kave/runtime/v1/runtimev1connect"
)

// Client is the Kave SDK entry point. Use New or NewFromConfig to construct one.
type Client struct {
	control controlv1connect.ControlPlaneServiceClient
	rbac    controlv1connect.RBACServiceClient
	runtime runtimev1connect.RuntimeServiceClient
	audit   auditv1connect.AuditServiceClient
}

// RawClients exposes the underlying generated Connect clients. Use it only for
// RPCs the typed Client surface does not cover; prefer the typed methods.
type RawClients struct {
	Control controlv1connect.ControlPlaneServiceClient
	RBAC    controlv1connect.RBACServiceClient
	Runtime runtimev1connect.RuntimeServiceClient
	Audit   auditv1connect.AuditServiceClient
}

// New creates a Kave client. Defaults to http://localhost:18080 over Connect protocol.
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
		control: controlv1connect.NewControlPlaneServiceClient(httpClient, o.baseURL, connectOpts...),
		rbac:    controlv1connect.NewRBACServiceClient(httpClient, o.baseURL, connectOpts...),
		runtime: runtimev1connect.NewRuntimeServiceClient(httpClient, o.baseURL, connectOpts...),
		audit:   auditv1connect.NewAuditServiceClient(httpClient, o.baseURL, connectOpts...),
	}
}

// Raw returns the underlying Connect clients as an escape hatch.
func (c *Client) Raw() RawClients {
	return RawClients{Control: c.control, RBAC: c.rbac, Runtime: c.runtime, Audit: c.audit}
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
