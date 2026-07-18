package kave

import (
	"context"
	"errors"
	"fmt"

	connect "connectrpc.com/connect"
)

var (
	// ErrInvalidConfig means the client configuration is incomplete or unsafe.
	ErrInvalidConfig = errors.New("kave: invalid config")
	// ErrInvalidNamespace means a namespace component is empty or unsafe.
	ErrInvalidNamespace = errors.New("kave: invalid namespace")
	// ErrInvalidAgent means an agent name is empty or unsafe.
	ErrInvalidAgent = errors.New("kave: invalid agent")
	// ErrInvalidRef means an opaque scope reference is empty or unsafe.
	ErrInvalidRef = errors.New("kave: invalid reference")
	// ErrInvalidScope means a request has no valid tenant and billing scope.
	ErrInvalidScope = errors.New("kave: invalid scope")
	// ErrInvalidMetric means a quota metric is empty or malformed.
	ErrInvalidMetric = errors.New("kave: invalid metric")
	// ErrInvalidConsume means an exact quota consumption request is incomplete.
	ErrInvalidConsume = errors.New("kave: invalid consume request")
	// ErrInvalidInvocation means a provider call has no stable retry identity.
	ErrInvalidInvocation = errors.New("kave: invalid invocation")
	// ErrInvalidResponse means Kave returned a response the SDK cannot safely interpret.
	ErrInvalidResponse = errors.New("kave: invalid response")
	// ErrLimitExceeded means at least one matching hard limit rejected consumption.
	ErrLimitExceeded = errors.New("kave: limit exceeded")
	// ErrUnsupportedRequest means a request is not an allowed OpenAI-compatible call.
	ErrUnsupportedRequest = errors.New("kave: unsupported request")
	// ErrRedirectNotAllowed is returned instead of following a gateway redirect.
	ErrRedirectNotAllowed = errors.New("kave: redirects are not allowed")

	ErrUnauthenticated     = errors.New("kave: unauthenticated")
	ErrPermissionDenied    = errors.New("kave: permission denied")
	ErrInvalidArgument     = errors.New("kave: invalid argument")
	ErrNotFound            = errors.New("kave: not found")
	ErrIdempotencyConflict = errors.New("kave: idempotency conflict")
	ErrRevisionConflict    = errors.New("kave: revision conflict")
	ErrFailedPrecondition  = errors.New("kave: failed precondition")
	ErrResourceExhausted   = errors.New("kave: resource exhausted")
	ErrUnavailable         = errors.New("kave: unavailable")
	ErrCanceled            = errors.New("kave: canceled")
	ErrDeadlineExceeded    = errors.New("kave: deadline exceeded")
	ErrInternal            = errors.New("kave: internal error")
)

// Error is Kave's transport-neutral protocol error. It deliberately does not
// unwrap to Connect, so callers can depend on stable sentinels and predicates
// without leaking a wire implementation into application code.
type Error struct {
	Kind    error
	Message string
}

func (e *Error) Error() string {
	if e == nil || e.Kind == nil {
		return "kave: request failed"
	}
	if e.Message == "" {
		return e.Kind.Error()
	}
	return fmt.Sprintf("%s: %s", e.Kind, e.Message)
}

func (e *Error) Is(target error) bool { return e != nil && target == e.Kind }

func normalizeError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return &Error{Kind: ErrCanceled, Message: "request canceled"}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &Error{Kind: ErrDeadlineExceeded, Message: "deadline exceeded"}
	}
	kind := ErrInternal
	switch connect.CodeOf(err) {
	case connect.CodeUnauthenticated:
		kind = ErrUnauthenticated
	case connect.CodePermissionDenied:
		kind = ErrPermissionDenied
	case connect.CodeInvalidArgument:
		kind = ErrInvalidArgument
	case connect.CodeNotFound:
		kind = ErrNotFound
	case connect.CodeAlreadyExists:
		kind = ErrIdempotencyConflict
	case connect.CodeAborted:
		kind = ErrRevisionConflict
	case connect.CodeFailedPrecondition:
		kind = ErrFailedPrecondition
	case connect.CodeResourceExhausted:
		kind = ErrResourceExhausted
	case connect.CodeUnavailable:
		kind = ErrUnavailable
	case connect.CodeCanceled:
		kind = ErrCanceled
	case connect.CodeDeadlineExceeded:
		kind = ErrDeadlineExceeded
	case connect.CodeInternal, connect.CodeUnknown:
		kind = ErrInternal
	}
	return &Error{Kind: kind, Message: err.Error()}
}

func IsNotFound(err error) bool         { return errors.Is(err, ErrNotFound) }
func IsPermissionDenied(err error) bool { return errors.Is(err, ErrPermissionDenied) }
func IsUnauthenticated(err error) bool  { return errors.Is(err, ErrUnauthenticated) }
func IsInvalidArgument(err error) bool  { return errors.Is(err, ErrInvalidArgument) }
func IsUnavailable(err error) bool      { return errors.Is(err, ErrUnavailable) }
func IsCanceled(err error) bool         { return errors.Is(err, ErrCanceled) }
func IsDeadlineExceeded(err error) bool { return errors.Is(err, ErrDeadlineExceeded) }
