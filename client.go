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
	if o.token != "" {
		httpClient = &tokenClient{inner: o.httpClient, token: o.token}
	}

	connectOpts := append([]connect.ClientOption{
		connect.WithCompressMinBytes(1024),
	}, o.connectOpts...)

	return &Client{
		Control: controlv1connect.NewControlPlaneServiceClient(httpClient, o.baseURL, connectOpts...),
		Runtime: runtimev1connect.NewRuntimeServiceClient(httpClient, o.baseURL, connectOpts...),
		Audit:   auditv1connect.NewAuditServiceClient(httpClient, o.baseURL, connectOpts...),
	}
}

// tokenClient injects an Authorization header on every request.
type tokenClient struct {
	inner connect.HTTPClient
	token string
}

func (t *tokenClient) Do(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+t.token)
	return t.inner.Do(req)
}
