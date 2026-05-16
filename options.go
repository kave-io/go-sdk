package kave

import (
	"crypto/tls"
	"net/http"
	"time"

	"connectrpc.com/connect"
)

type options struct {
	baseURL     string
	httpClient  *http.Client
	token       string
	connectOpts []connect.ClientOption
}

type Option func(*options)

func WithAddr(addr string) Option {
	return func(o *options) { o.baseURL = addr }
}

func WithToken(token string) Option {
	return func(o *options) { o.token = token }
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

func defaultOptions() options {
	return options{
		baseURL:    "http://localhost:8080",
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}
