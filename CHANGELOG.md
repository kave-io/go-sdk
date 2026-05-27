# Go SDK Changelog

## 0.2.0

- Added SDK-native spec models and request conversion layer for control, runtime,
  and bootstrap flows.
- Split high-level behaviors into focused files (`bootstrap.go`, `control.go`,
  `runtime.go`, `models.go`, `inputs.go`, `requests.go`, `convert.go`).
- Expanded tests for specs conversion, runtime/control paths, and retry/error
  handling.
- Updated examples and quickstart usage around spec-first APIs.

## 0.1.0

- Module path finalized as `github.com/kave-io/kave/sdk/go` for the standalone Go SDK repository.
- `replace` directive dropped; `github.com/kave-io/kave/proto/gen` is pinned to v0.1.0.
- Default server address changed from `:8080` to `:18080`.
- `errors.go`: added `IsAlreadyExists`, `IsPermissionDenied`, `IsUnauthenticated`,
  `IsInvalidArgument`, `IsUnavailable`, `IsCanceled`, `IsDeadlineExceeded`.
  Moved `IsNotFound` from `highlevel.go`.
- `retry.go`: added `RetryPolicy`, `DefaultRetryPolicy`, `WithRetry`.
- `iterators.go`: added channel-based `Iterate*` helpers for all List* RPCs.
- `highlevel.go`: added `EnsureCredential`, `WithSpan`.
- `contract_test.go`: added contract test suite (build tag: `contracts`).
