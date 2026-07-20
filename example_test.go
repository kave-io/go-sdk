package kave_test

import (
	"context"
	"errors"
	"net/http"
	"strings"

	kave "github.com/kave-io/go-sdk/v2"
)

func Example() {
	client, err := kave.OpenFromEnv()
	if err != nil {
		return
	}

	ctx := kave.WithScope(context.Background(), kave.Scope{
		Tenant:  kave.Ref("clinic/01JABC"),
		Actor:   kave.Ref("user/01JXYZ"),
		BillTo:  kave.Ref("clinic/01JABC"),
		Session: kave.Ref("run/01JRUN"),
	})
	ctx = kave.WithInvocation(ctx, kave.Once("run/01JRUN/answer/1"))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.openai.com/v1/responses",
		strings.NewReader(`{"model":"gpt-5","input":"hello"}`),
	)
	if err != nil {
		return
	}

	resp, err := client.HTTPClient(kave.Agent("clinic-assistant")).Do(req)
	if err == nil {
		_ = resp.Body.Close()
	}
}

func ExampleClient_Consume() {
	client, err := kave.OpenFromEnv()
	if err != nil {
		return
	}
	ctx := kave.WithScope(context.Background(), kave.Scope{
		Tenant: "clinic/01JABC", BillTo: "clinic/01JABC",
		Session: "run/01JRUN", Feature: "ai_actions",
	})
	decision, err := client.Consume(
		ctx, kave.Agent("clinic-assistant"), kave.Metric("ai_actions"), 1, kave.Once("run/01JRUN"),
	)
	if errors.Is(err, kave.ErrLimitExceeded) {
		return
	}
	_ = decision
}
