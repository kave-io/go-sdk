package kave

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func openTestClient(t *testing.T, endpoint string, base *http.Client) *Client {
	t.Helper()
	cfg := validConfig(endpoint)
	cfg.HTTPClient = base
	client, err := Open(cfg)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	return client
}

func requestWithScope(t *testing.T, method, target string, scope Scope) *http.Request {
	t.Helper()
	ctx := WithInvocation(WithScope(context.Background(), scope), Once("invocation/test"))
	req, err := http.NewRequestWithContext(ctx, method, target, strings.NewReader(`{"model":"test"}`))
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}
	return req
}

func TestTransportRewritesSupportedOpenAIPaths(t *testing.T) {
	t.Parallel()

	wantPaths := make(chan string, len(supportedOpenAIPaths))
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantPaths <- r.URL.RequestURI()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer gateway.Close()

	client := openTestClient(t, gateway.URL, nil).HTTPClient(Agent("clinic-assistant"))
	tests := []struct {
		input string
		want  string
	}{
		{input: "/v1/chat/completions", want: "/v2/agents/clinic-assistant/openai/chat/completions"},
		{input: "/v1/responses", want: "/v2/agents/clinic-assistant/openai/responses"},
		{input: "/v1/embeddings", want: "/v2/agents/clinic-assistant/openai/embeddings"},
	}
	for _, test := range tests {
		req := requestWithScope(t, http.MethodPost, "https://api.openai.com"+test.input, validScope("map"))
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Do(%s) error = %v", test.input, err)
		}
		_ = resp.Body.Close()
		if got := <-wantPaths; got != test.want {
			t.Fatalf("rewritten path = %q, want %q", got, test.want)
		}
	}
}

func TestTransportInjectsIdentityAndScopeWithoutMutatingRequest(t *testing.T) {
	t.Parallel()

	seen := make(chan *http.Request, 1)
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		seen <- r.Clone(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	defer gateway.Close()

	client := openTestClient(t, gateway.URL, nil).HTTPClient(Agent("clinic-assistant"))
	req := requestWithScope(t, http.MethodPost, "https://api.openai.com/v1/responses", validScope("42"))
	req.Host = "api.openai.com"
	req.Header.Set("Authorization", "Bearer provider-secret")
	req.Header.Set("Api-Key", "another-provider-secret")
	req.Header.Set("X-Goog-Api-Key", "google-provider-secret")
	req.Header.Set("Anthropic-X-Api-Key", "anthropic-provider-secret")
	req.Header.Set("Proxy-Authorization", "Basic provider")
	req.Header.Set("Cookie", "provider=session")
	req.Header.Set(HeaderTenant, "spoofed")
	req.Header.Set("Kave-Admin", "true")
	req.Header.Set("Kave-Account", "spoofed-account")
	req.Header.Set("Kave-Application", "spoofed-application")
	req.Header.Set("Kave-Environment", "spoofed-environment")
	req.Header.Set("Kave-Agent", "spoofed-agent")
	req.Header.Set("Traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	req.Header.Set("Tracestate", "vendor=value")
	req.Header.Set("Baggage", "workflow=assistant")
	req.Trailer = http.Header{"X-Provider-Credential": []string{"trailer-secret"}}
	req.TransferEncoding = []string{"chunked"}

	originalURL := req.URL.String()
	originalHost := req.Host
	originalHeader := req.Header.Clone()
	originalTrailer := req.Trailer.Clone()
	originalTransferEncoding := append([]string(nil), req.TransferEncoding...)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	_ = resp.Body.Close()
	got := <-seen

	if got.URL.Path != "/v2/agents/clinic-assistant/openai/responses" || got.URL.RawQuery != "" {
		t.Fatalf("gateway URL = %s", got.URL.String())
	}
	if got.Header.Get("Authorization") != "Bearer kv2_A1b2C3d4E5f6G7h8I9j0K1l2.AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" {
		t.Fatalf("Authorization = %q", got.Header.Get("Authorization"))
	}
	for _, header := range []string{"Api-Key", "X-Goog-Api-Key", "Anthropic-X-Api-Key", "Proxy-Authorization", "Cookie", "Kave-Admin"} {
		if value := got.Header.Get(header); value != "" {
			t.Errorf("%s was forwarded: %q", header, value)
		}
	}
	wantHeaders := map[string]string{
		HeaderTenant:     "clinic/42",
		HeaderActor:      "user/42",
		HeaderBillTo:     "clinic/42",
		HeaderSession:    "run/42",
		HeaderFeature:    "ai_actions",
		HeaderInvocation: "invocation/test",
		"Traceparent":    originalHeader.Get("Traceparent"),
	}
	for _, header := range []string{"Tracestate", "Baggage"} {
		if value := got.Header.Get(header); value != "" {
			t.Errorf("sensitive tracing header %s was forwarded: %q", header, value)
		}
	}
	if value := got.Trailer.Get("X-Provider-Credential"); value != "" {
		t.Errorf("sensitive HTTP trailer was forwarded: %q", value)
	}
	if len(got.TransferEncoding) != 0 {
		t.Errorf("caller transfer encoding was forwarded: %v", got.TransferEncoding)
	}
	for header, want := range wantHeaders {
		if value := got.Header.Get(header); value != want {
			t.Errorf("%s = %q, want %q", header, value, want)
		}
	}
	for _, header := range []string{"Kave-Account", "Kave-Application", "Kave-Environment", "Kave-Agent"} {
		if value := got.Header.Get(header); value != "" {
			t.Errorf("caller-controlled identity header %s was forwarded: %q", header, value)
		}
	}

	if req.URL.String() != originalURL {
		t.Fatalf("original URL mutated: got %q, want %q", req.URL.String(), originalURL)
	}
	if req.Host != originalHost {
		t.Fatalf("original Host mutated: got %q, want %q", req.Host, originalHost)
	}
	if !reflect.DeepEqual(req.Header, originalHeader) {
		t.Fatalf("original headers mutated:\n got: %#v\nwant: %#v", req.Header, originalHeader)
	}
	if !reflect.DeepEqual(req.Trailer, originalTrailer) || !reflect.DeepEqual(req.TransferEncoding, originalTransferEncoding) {
		t.Fatal("original trailer or transfer encoding mutated")
	}
}

func TestTransportRejectsUnscopedInvalidAndUnsupportedRequestsBeforeNetwork(t *testing.T) {
	t.Parallel()

	var hits atomic.Int64
	gateway := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		hits.Add(1)
	}))
	defer gateway.Close()
	client := openTestClient(t, gateway.URL, nil)

	tests := []struct {
		name  string
		agent Agent
		req   func(*testing.T) *http.Request
		want  error
	}{
		{
			name:  "query parameters",
			agent: "assistant",
			req: func(t *testing.T) *http.Request {
				return requestWithScope(t, http.MethodPost, "https://api.openai.com/v1/responses?api_key=secret", validScope("1"))
			},
			want: ErrUnsupportedRequest,
		},
		{
			name:  "missing scope",
			agent: "assistant",
			req: func(t *testing.T) *http.Request {
				req, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/responses", nil)
				if err != nil {
					t.Fatal(err)
				}
				return req
			},
			want: ErrInvalidScope,
		},
		{
			name:  "invalid scope",
			agent: "assistant",
			req: func(t *testing.T) *http.Request {
				return requestWithScope(t, http.MethodPost, "https://api.openai.com/v1/responses", Scope{Tenant: "clinic/1"})
			},
			want: ErrInvalidScope,
		},
		{
			name:  "missing invocation",
			agent: "assistant",
			req: func(t *testing.T) *http.Request {
				ctx := WithScope(context.Background(), validScope("1"))
				req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/responses", nil)
				if err != nil {
					t.Fatal(err)
				}
				return req
			},
			want: ErrInvalidInvocation,
		},
		{
			name:  "invalid agent",
			agent: "bad/agent",
			req: func(t *testing.T) *http.Request {
				return requestWithScope(t, http.MethodPost, "https://api.openai.com/v1/responses", validScope("1"))
			},
			want: ErrInvalidAgent,
		},
		{
			name:  "GET",
			agent: "assistant",
			req: func(t *testing.T) *http.Request {
				return requestWithScope(t, http.MethodGet, "https://api.openai.com/v1/responses", validScope("1"))
			},
			want: ErrUnsupportedRequest,
		},
		{
			name:  "unknown path",
			agent: "assistant",
			req: func(t *testing.T) *http.Request {
				return requestWithScope(t, http.MethodPost, "https://api.openai.com/v1/files", validScope("1"))
			},
			want: ErrUnsupportedRequest,
		},
		{
			name:  "encoded path",
			agent: "assistant",
			req: func(t *testing.T) *http.Request {
				return requestWithScope(t, http.MethodPost, "https://api.openai.com/v1/chat%2Fcompletions", validScope("1"))
			},
			want: ErrUnsupportedRequest,
		},
		{
			name:  "URL credentials",
			agent: "assistant",
			req: func(t *testing.T) *http.Request {
				return requestWithScope(t, http.MethodPost, "https://user:pass@api.openai.com/v1/responses", validScope("1"))
			},
			want: ErrUnsupportedRequest,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			resp, err := client.HTTPClient(test.agent).Do(test.req(t))
			if resp != nil {
				_ = resp.Body.Close()
			}
			if !errors.Is(err, test.want) {
				t.Fatalf("Do() error = %v, want %v", err, test.want)
			}
		})
	}
	t.Cleanup(func() {
		if got := hits.Load(); got != 0 {
			t.Errorf("gateway received %d rejected requests", got)
		}
	})
}

func TestHTTPClientRefusesRedirects(t *testing.T) {
	t.Parallel()

	var redirectedHits atomic.Int64
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirectedHits.Add(1)
	}))
	defer target.Close()

	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/stolen", http.StatusTemporaryRedirect)
	}))
	defer gateway.Close()

	client := openTestClient(t, gateway.URL, nil).HTTPClient(Agent("assistant"))
	resp, err := client.Do(requestWithScope(t, http.MethodPost, "https://api.openai.com/v1/responses", validScope("redirect")))
	if resp != nil {
		_ = resp.Body.Close()
	}
	if !errors.Is(err, ErrRedirectNotAllowed) {
		t.Fatalf("Do() error = %v, want ErrRedirectNotAllowed", err)
	}
	if got := redirectedHits.Load(); got != 0 {
		t.Fatalf("redirect target received %d requests", got)
	}
}

func TestTransportDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int64
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer gateway.Close()
	client := openTestClient(t, gateway.URL, nil).HTTPClient(Agent("assistant"))
	resp, err := client.Do(requestWithScope(t, http.MethodPost, "https://api.openai.com/v1/responses", validScope("once")))
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("transport attempts = %d, want 1", got)
	}
}

func TestTransportKeepsConcurrentScopesIsolated(t *testing.T) {
	t.Parallel()

	const requests = 128
	handlerErrors := make(chan error, requests)
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.Header.Get(HeaderSession), "run/")
		checks := map[string]string{
			HeaderTenant: "clinic/" + id,
			HeaderActor:  "user/" + id,
			HeaderBillTo: "clinic/" + id,
		}
		for header, want := range checks {
			if got := r.Header.Get(header); got != want {
				handlerErrors <- fmt.Errorf("session %q: %s = %q, want %q", id, header, got, want)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer gateway.Close()

	client := openTestClient(t, gateway.URL, nil).HTTPClient(Agent("assistant"))
	requestErrors := make(chan error, requests)
	var wg sync.WaitGroup
	for i := 0; i < requests; i++ {
		id := strconv.Itoa(i)
		req := requestWithScope(t, http.MethodPost, "https://api.openai.com/v1/responses", validScope(id))
		wg.Add(1)
		go func(req *http.Request) {
			defer wg.Done()
			resp, err := client.Do(req)
			if err != nil {
				requestErrors <- err
				return
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusNoContent {
				requestErrors <- fmt.Errorf("status = %d", resp.StatusCode)
			}
		}(req)
	}
	wg.Wait()
	close(requestErrors)
	close(handlerErrors)
	for err := range requestErrors {
		t.Error(err)
	}
	for err := range handlerErrors {
		t.Error(err)
	}
}
