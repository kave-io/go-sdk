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

// ClientConfig is the SDK-native constructor config for applications that do
// not need to assemble Option values manually.
type ClientConfig struct {
	Addr           string
	Token          string
	UserAgent      string
	Timeout        time.Duration
	TLS            *tls.Config
	HTTPClient     *http.Client
	UseGRPC        bool
	Retry          *RetryPolicy
	Logger         *slog.Logger
	Tracer         trace.Tracer
	ConnectOptions []connect.ClientOption
}

// NewFromConfig creates a Client from a plain Go config struct.
func NewFromConfig(cfg ClientConfig) (*Client, error) {
	var opts []Option
	if cfg.Addr != "" {
		opts = append(opts, WithAddr(cfg.Addr))
	}
	if cfg.Token != "" {
		opts = append(opts, WithToken(cfg.Token))
	}
	if cfg.UserAgent != "" {
		opts = append(opts, WithUserAgent(cfg.UserAgent))
	}
	if cfg.Timeout < 0 {
		return nil, invalidArgument("timeout must be non-negative")
	}
	if cfg.Timeout > 0 {
		opts = append(opts, WithTimeout(cfg.Timeout))
	}
	if cfg.TLS != nil {
		opts = append(opts, WithTLS(cfg.TLS))
	}
	if cfg.HTTPClient != nil {
		opts = append(opts, withHTTPClient(cfg.HTTPClient))
	}
	if cfg.UseGRPC {
		opts = append(opts, WithGRPC())
	}
	if cfg.Retry != nil {
		opts = append(opts, WithRetry(*cfg.Retry))
	}
	if cfg.Logger != nil {
		opts = append(opts, WithLogger(cfg.Logger))
	}
	if cfg.Tracer != nil {
		opts = append(opts, WithTracer(cfg.Tracer))
	}
	for _, opt := range cfg.ConnectOptions {
		opts = append(opts, WithConnectOption(opt))
	}
	return New(opts...), nil
}

func withHTTPClient(client *http.Client) Option {
	return func(o *options) { o.httpClient = client }
}
