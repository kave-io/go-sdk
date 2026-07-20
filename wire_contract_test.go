package kave

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// This catches accidental rewrites of the protobuf package name while the
// generated Connect client is copied into the standalone SDK module.
func TestGeneratedClientUsesV2KernelProcedure(t *testing.T) {
	t.Parallel()
	paths := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths <- r.URL.Path
		http.NotFound(w, r)
	}))
	defer server.Close()

	client, err := Open(Config{URL: server.URL, ServiceKey: "kv2_A1b2C3d4E5f6G7h8I9j0K1l2.AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = client.Apply(ctx, Manifest{Namespace: Namespace{
		Account: "account/acme", Application: "simorq", Environment: "test",
	}}, Once("wire/contract"))

	select {
	case path := <-paths:
		if path != "/kave.kernel.v2.KernelService/Apply" {
			t.Fatalf("generated Apply procedure = %q", path)
		}
	case <-ctx.Done():
		t.Fatalf("generated client did not reach server: %v", ctx.Err())
	}
}
