package kave

import (
	"crypto/tls"
	"log/slog"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"go.opentelemetry.io/otel/trace"
)

type options struct {
	baseURL     string
	httpClient  *http.Client
	token       string
	userAgent   string
	connectOpts []connect.ClientOption
	retryPolicy RetryPolicy
	logger      *slog.Logger
	tracer      trace.Tracer
}

type Option func(*options)

func WithAddr(addr string) Option {
	return func(o *options) { o.baseURL = addr }
}

func WithToken(token string) Option {
	return func(o *options) { o.token = token }
}

func WithUserAgent(userAgent string) Option {
	return func(o *options) { o.userAgent = userAgent }
}

// WithTLS configures mutual TLS. Pass nil cfg to use system defaults.
func WithTLS(cfg *tls.Config) Option {
	return func(o *options) {
		t := http.DefaultTransport.(*http.Transport).Clone()
		t.TLSClientConfig = cfg
		o.httpClient = &http.Client{Transport: t}
	}
}

func WithTimeout(d time.Duration) Option {
	return func(o *options) {
		o.httpClient = &http.Client{Timeout: d}
	}
}

// WithGRPC switches the transport to binary gRPC (HTTP/2).
func WithGRPC() Option {
	return func(o *options) {
		o.connectOpts = append(o.connectOpts, connect.WithGRPC())
	}
}

func WithConnectOption(opt connect.ClientOption) Option {
	return func(o *options) {
		o.connectOpts = append(o.connectOpts, opt)
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(o *options) { o.logger = logger }
}

func WithTracer(tracer trace.Tracer) Option {
	return func(o *options) { o.tracer = tracer }
}

func defaultOptions() options {
	return options{
		baseURL:     "http://localhost:18080",
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		userAgent:   "kave-go-sdk/dev",
		retryPolicy: DefaultRetryPolicy,
	}
}
