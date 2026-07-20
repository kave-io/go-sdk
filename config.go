package kave

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	envURL           = "KAVE_URL"
	envServiceKey    = "KAVE_SERVICE_KEY"
	envAllowInsecure = "KAVE_ALLOW_INSECURE"
)

// Config configures a Kave V2 client.
//
// URL must be an absolute Kave origin without a path, query, fragment, or
// embedded credentials. Plain HTTP is accepted only for loopback origins or
// when AllowInsecure is explicitly enabled.
type Config struct {
	URL           string       `json:"-"`
	ServiceKey    string       `json:"-"`
	HTTPClient    *http.Client `json:"-"`
	AllowInsecure bool
}

// Client is a configured Kave V2 client. It is safe for concurrent use.
type Client struct {
	endpoint   url.URL
	serviceKey string
	baseClient http.Client
	consumer   kernelConsumer
	controller kernelController
	reader     kernelReader
}

func (cfg Config) String() string {
	return fmt.Sprintf("{URLSet:%t ServiceKey:[REDACTED] HTTPClient:%t AllowInsecure:%t}", cfg.URL != "", cfg.HTTPClient != nil, cfg.AllowInsecure)
}

func (cfg Config) GoString() string { return cfg.String() }

func (c *Client) String() string {
	if c == nil {
		return "<nil>"
	}
	return fmt.Sprintf("{URL:%q ServiceKey:[REDACTED]}", c.endpoint.String())
}

func (c *Client) GoString() string { return c.String() }

// Validate verifies that the configuration is complete and safe.
func (cfg Config) Validate() error {
	if _, err := cfg.validatedEndpoint(); err != nil {
		return err
	}
	return cfg.validateHTTPClient()
}

// Open constructs a Kave V2 client from an explicit configuration.
func Open(cfg Config) (*Client, error) {
	endpoint, err := cfg.validatedEndpoint()
	if err != nil {
		return nil, err
	}
	if err := cfg.validateHTTPClient(); err != nil {
		return nil, err
	}

	base := http.Client{}
	if cfg.HTTPClient != nil {
		base = *cfg.HTTPClient
	}
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok || defaultTransport == nil {
		return nil, fmt.Errorf("%w: standard HTTP transport is unavailable", ErrInvalidConfig)
	}
	// Never retain the process-global mutable transport. A private clone keeps
	// connection pooling and proxy/TLS defaults while preventing unrelated code
	// from changing the credential-bearing transport after Open returns.
	base.Transport = defaultTransport.Clone()

	client := &Client{
		endpoint:   *endpoint,
		serviceKey: cfg.ServiceKey,
		baseClient: base,
	}
	client.consumer = newKernelConsumer(client)
	client.controller = newKernelController(client)
	client.reader = newKernelReader(client)
	return client, nil
}

func (cfg Config) validateHTTPClient() error {
	if cfg.HTTPClient != nil && cfg.HTTPClient.Transport != nil {
		return fmt.Errorf("%w: custom HTTPClient transports are not accepted because they run after credential sanitization", ErrInvalidConfig)
	}
	if transport, ok := http.DefaultTransport.(*http.Transport); !ok || transport == nil {
		return fmt.Errorf("%w: standard HTTP transport is unavailable", ErrInvalidConfig)
	}
	return nil
}

// OpenFromEnv constructs a client from KAVE_URL and KAVE_SERVICE_KEY. The
// service key, rather than caller-controlled headers, fixes its namespace.
//
// KAVE_ALLOW_INSECURE may be set to a value accepted by strconv.ParseBool to
// explicitly allow a non-loopback HTTP endpoint.
func OpenFromEnv() (*Client, error) {
	allowInsecure := false
	if raw := os.Getenv(envAllowInsecure); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("%w: %s must be a boolean", ErrInvalidConfig, envAllowInsecure)
		}
		allowInsecure = parsed
	}

	return Open(Config{
		URL:           os.Getenv(envURL),
		ServiceKey:    os.Getenv(envServiceKey),
		AllowInsecure: allowInsecure,
	})
}

func (cfg Config) validatedEndpoint() (*url.URL, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("%w: URL is required", ErrInvalidConfig)
	}
	if strings.TrimSpace(cfg.URL) != cfg.URL {
		return nil, fmt.Errorf("%w: URL must not have surrounding whitespace", ErrInvalidConfig)
	}

	endpoint, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("%w: URL is malformed", ErrInvalidConfig)
	}
	if endpoint.Opaque != "" || endpoint.Host == "" {
		return nil, fmt.Errorf("%w: URL must be an absolute origin", ErrInvalidConfig)
	}
	if endpoint.Scheme != "https" && endpoint.Scheme != "http" {
		return nil, fmt.Errorf("%w: URL scheme must be https or http", ErrInvalidConfig)
	}
	if endpoint.User != nil {
		return nil, fmt.Errorf("%w: URL must not contain credentials", ErrInvalidConfig)
	}
	if endpoint.Path != "" && endpoint.Path != "/" {
		return nil, fmt.Errorf("%w: URL must not contain a path", ErrInvalidConfig)
	}
	if endpoint.RawPath != "" || endpoint.RawQuery != "" || endpoint.ForceQuery || endpoint.Fragment != "" {
		return nil, fmt.Errorf("%w: URL must not contain an escaped path, query, or fragment", ErrInvalidConfig)
	}
	if endpoint.Scheme == "http" && !cfg.AllowInsecure && !isLoopbackHost(endpoint.Hostname()) {
		return nil, fmt.Errorf("%w: plain HTTP requires loopback or AllowInsecure", ErrInvalidConfig)
	}

	if err := validateServiceKey(cfg.ServiceKey); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}
	// Store a canonical origin so request rewriting cannot inherit a slash.
	endpoint.Path = ""
	return endpoint, nil
}

func validateServiceKey(key string) error {
	_, err := parseServiceKeyMaterial(key)
	return err
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
