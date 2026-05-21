# Go SDK Changelog

## 0.1.0

- Module renamed to `github.com/kave-io/kave/sdk/go` (monorepo; no separate repo).
- `replace` directive dropped; `go.work` workspace used instead.
- Default server address changed from `:8080` to `:18080`.
- `errors.go`: added `IsAlreadyExists`, `IsPermissionDenied`, `IsUnauthenticated`,
  `IsInvalidArgument`, `IsUnavailable`, `IsCanceled`, `IsDeadlineExceeded`.
  Moved `IsNotFound` from `highlevel.go`.
- `retry.go`: added `RetryPolicy`, `DefaultRetryPolicy`, `WithRetry`.
- `iterators.go`: added channel-based `Iterate*` helpers for all List* RPCs.
- `highlevel.go`: added `EnsureCredential`, `WithSpan`.
- `contract_test.go`: added contract test suite (build tag: `contracts`).
