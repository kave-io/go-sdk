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
	URL           string
	ServiceKey    string
	HTTPClient    *http.Client
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

// Validate verifies that the configuration is complete and safe.
func (cfg Config) Validate() error {
	_, err := cfg.validatedEndpoint()
	return err
}

// Open constructs a Kave V2 client from an explicit configuration.
func Open(cfg Config) (*Client, error) {
	endpoint, err := cfg.validatedEndpoint()
	if err != nil {
		return nil, err
	}

	base := *http.DefaultClient
	if cfg.HTTPClient != nil {
		if cfg.HTTPClient.Transport != nil {
			return nil, fmt.Errorf("%w: custom HTTPClient transports are not accepted because they run after credential sanitization", ErrInvalidConfig)
		}
		base = *cfg.HTTPClient
	}

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
