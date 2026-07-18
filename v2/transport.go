package kave

import (
	"fmt"
	"net/http"
	"net/url"
)

const (
	HeaderTenant  = "Kave-Tenant"
	HeaderActor   = "Kave-Actor"
	HeaderBillTo  = "Kave-Bill-To"
	HeaderSession = "Kave-Session"
	HeaderFeature = "Kave-Feature"
	// HeaderInvocation is an idempotency identity, not a tenant scope. It is
	// generated only from WithInvocation context and never trusted from callers.
	HeaderInvocation = "X-Kave-Invocation-Key"
)

var supportedOpenAIPaths = map[string]string{
	"/v1/chat/completions": "chat/completions",
	"/v1/responses":        "responses",
	"/v1/embeddings":       "embeddings",
}

// HTTPClient returns an HTTP client scoped to agent. It preserves the base
// client's timeout and transport, disables cookie state, and refuses every
// redirect. Invalid agents are reported when a request is sent.
func (c *Client) HTTPClient(agent Agent) *http.Client {
	client := c.baseClient
	client.Transport = c.RoundTripper(agent)
	client.Jar = nil
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return ErrRedirectNotAllowed
	}
	return &client
}

// RoundTripper returns a transport that sends supported OpenAI-compatible
// requests through Kave for agent. It performs no retries.
func (c *Client) RoundTripper(agent Agent) http.RoundTripper {
	next := c.baseClient.Transport
	if next == nil {
		next = http.DefaultTransport
	}
	return &scopedTransport{
		endpoint:   c.endpoint,
		serviceKey: c.serviceKey,
		agent:      agent,
		agentErr:   agent.Validate(),
		next:       next,
	}
}

type scopedTransport struct {
	endpoint   url.URL
	serviceKey string
	agent      Agent
	agentErr   error
	next       http.RoundTripper
}

func (t *scopedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("%w: request is nil", ErrUnsupportedRequest)
	}
	if t.agentErr != nil {
		return nil, t.agentErr
	}

	scope, ok := ScopeFromContext(req.Context())
	if !ok {
		return nil, fmt.Errorf("%w: request context has no scope", ErrInvalidScope)
	}
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	once, ok := InvocationFromContext(req.Context())
	if !ok {
		return nil, fmt.Errorf("%w: request context has no logical invocation", ErrInvalidInvocation)
	}
	if err := once.key.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInvocation, err)
	}

	suffix, err := openAISuffix(req)
	if err != nil {
		return nil, err
	}

	out := req.Clone(req.Context())
	target := t.endpoint
	target.Path = "/v2/agents/" + url.PathEscape(string(t.agent)) + "/openai/" + suffix
	target.RawPath = ""
	target.RawQuery = ""
	target.ForceQuery = false
	target.Fragment = ""
	out.URL = &target
	out.Host = ""
	out.RequestURI = ""
	// HTTP trailers are a second header channel. Clear them together with the
	// caller's transfer-encoding hints so provider credentials cannot bypass the
	// explicit outbound header allowlist.
	out.Trailer = nil
	out.TransferEncoding = nil
	out.Header = allowedOutboundHeaders(req.Header)
	out.Header.Set("Authorization", "Bearer "+t.serviceKey)
	out.Header.Set(HeaderTenant, string(scope.Tenant))
	out.Header.Set(HeaderBillTo, string(scope.BillTo))
	setOptionalHeader(out.Header, HeaderActor, scope.Actor)
	setOptionalHeader(out.Header, HeaderSession, scope.Session)
	setOptionalHeader(out.Header, HeaderFeature, scope.Feature)
	out.Header.Set(HeaderInvocation, string(once.key))

	return t.next.RoundTrip(out)
}

func openAISuffix(req *http.Request) (string, error) {
	if req.Method != http.MethodPost {
		return "", fmt.Errorf("%w: only POST is allowed", ErrUnsupportedRequest)
	}
	if req.URL == nil || req.URL.Opaque != "" || req.URL.Host == "" {
		return "", fmt.Errorf("%w: URL must be absolute", ErrUnsupportedRequest)
	}
	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return "", fmt.Errorf("%w: URL scheme must be http or https", ErrUnsupportedRequest)
	}
	if req.URL.User != nil || req.URL.RawPath != "" || req.URL.RawQuery != "" || req.URL.ForceQuery || req.URL.Fragment != "" {
		return "", fmt.Errorf("%w: URL contains unsupported components", ErrUnsupportedRequest)
	}
	suffix, ok := supportedOpenAIPaths[req.URL.Path]
	if !ok {
		return "", fmt.Errorf("%w: OpenAI-compatible path is not allowed", ErrUnsupportedRequest)
	}
	return suffix, nil
}

func allowedOutboundHeaders(source http.Header) http.Header {
	header := make(http.Header, 4)
	for _, name := range []string{"Content-Type", "Accept", "Traceparent"} {
		for _, value := range source.Values(name) {
			header.Add(name, value)
		}
	}
	return header
}

func setOptionalHeader(header http.Header, name string, value Ref) {
	if value != "" {
		header.Set(name, string(value))
	}
}
