# Go SDK Changelog

## 0.1.0

- Module path finalized as `github.com/kave-io/go-sdk` for the standalone Go SDK repository.
- `replace` directive dropped; `github.com/kave-io/kave/proto/gen` is pinned to v0.1.0.
- Default server address changed from `:8080` to `:18080`.
- `errors.go`: added `IsAlreadyExists`, `IsPermissionDenied`, `IsUnauthenticated`,
  `IsInvalidArgument`, `IsUnavailable`, `IsCanceled`, `IsDeadlineExceeded`.
  Moved `IsNotFound` from `highlevel.go`.
- `retry.go`: added `RetryPolicy`, `DefaultRetryPolicy`, `WithRetry`.
- `iterators.go`: added channel-based `Iterate*` helpers for all List* RPCs.
- `highlevel.go`: added `EnsureCredential`, `WithSpan`.
- `contract_test.go`: added contract test suite (build tag: `contracts`).
