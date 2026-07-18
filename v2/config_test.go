package kave

import (
	"errors"
	"net/http"
	"testing"
	"time"
)

func validConfig(endpoint string) Config {
	return Config{
		URL:        endpoint,
		ServiceKey: "kv2_A1b2C3d4E5f6G7h8I9j0K1l2.AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
	}
}

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "missing URL", mutate: func(c *Config) { c.URL = "" }},
		{name: "URL whitespace", mutate: func(c *Config) { c.URL = " https://kave.example.test" }},
		{name: "relative URL", mutate: func(c *Config) { c.URL = "/kave" }},
		{name: "unsupported scheme", mutate: func(c *Config) { c.URL = "ftp://kave.example.test" }},
		{name: "URL credentials", mutate: func(c *Config) { c.URL = "https://user:pass@kave.example.test" }},
		{name: "URL path", mutate: func(c *Config) { c.URL = "https://kave.example.test/rpc" }},
		{name: "URL query", mutate: func(c *Config) { c.URL = "https://kave.example.test?debug=1" }},
		{name: "URL fragment", mutate: func(c *Config) { c.URL = "https://kave.example.test/#fragment" }},
		{name: "insecure remote URL", mutate: func(c *Config) { c.URL = "http://kave.example.test" }},
		{name: "missing service key", mutate: func(c *Config) { c.ServiceKey = "" }},
		{name: "service key space", mutate: func(c *Config) { c.ServiceKey = "bad key" }},
		{name: "service key newline", mutate: func(c *Config) { c.ServiceKey = "bad\r\nkey" }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			cfg := validConfig("https://kave.example.test")
			test.mutate(&cfg)
			if err := cfg.Validate(); !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("Validate() error = %v, want ErrInvalidConfig", err)
			}
			if _, err := Open(cfg); !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("Open() error = %v, want ErrInvalidConfig", err)
			}
		})
	}
}

func TestConfigAllowsSecureAndExplicitDevelopmentOrigins(t *testing.T) {
	t.Parallel()

	tests := []Config{
		validConfig("https://kave.example.test"),
		validConfig("https://kave.example.test/"),
		validConfig("http://localhost:18080"),
		validConfig("http://127.0.0.1:18080"),
		validConfig("http://[::1]:18080"),
	}
	insecure := validConfig("http://kave.internal:18080")
	insecure.AllowInsecure = true
	tests = append(tests, insecure)

	for _, cfg := range tests {
		if _, err := Open(cfg); err != nil {
			t.Errorf("Open(%q) error = %v", cfg.URL, err)
		}
	}
}

func TestOpenCopiesBaseHTTPClient(t *testing.T) {
	t.Parallel()

	base := &http.Client{Timeout: 17 * time.Second}
	cfg := validConfig("https://kave.example.test")
	cfg.HTTPClient = base
	client, err := Open(cfg)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	base.Timeout = time.Second
	got := client.HTTPClient(Agent("assistant"))
	if got.Timeout != 17*time.Second {
		t.Fatalf("HTTPClient timeout = %s, want 17s", got.Timeout)
	}
	if got == base {
		t.Fatal("HTTPClient returned the caller's mutable client")
	}
}

func TestOpenRejectsCustomTransportAfterSanitizationBoundary(t *testing.T) {
	t.Parallel()
	cfg := validConfig("https://kave.example.test")
	cfg.HTTPClient = &http.Client{Transport: http.DefaultTransport}
	if _, err := Open(cfg); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("Open() error = %v, want ErrInvalidConfig", err)
	}
}

func TestOpenFromEnv(t *testing.T) {
	t.Setenv(envURL, "https://kave.example.test")
	t.Setenv(envServiceKey, "kv2_A1b2C3d4E5f6G7h8I9j0K1l2.AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	t.Setenv(envAllowInsecure, "false")

	client, err := OpenFromEnv()
	if err != nil {
		t.Fatalf("OpenFromEnv() error = %v", err)
	}
	if client.endpoint.String() != "https://kave.example.test" {
		t.Fatalf("endpoint = %q", client.endpoint.String())
	}
	if client.serviceKey != "kv2_A1b2C3d4E5f6G7h8I9j0K1l2.AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" {
		t.Fatal("service key was not loaded")
	}
}

func TestOpenFromEnvRejectsInvalidBoolean(t *testing.T) {
	t.Setenv(envAllowInsecure, "sometimes")
	_, err := OpenFromEnv()
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("OpenFromEnv() error = %v, want ErrInvalidConfig", err)
	}
}
