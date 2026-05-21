package kave

import (
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Code is the SDK's transport-neutral error code.
type Code string

const (
	CodeUnknown          Code = "Unknown"
	CodeCanceled         Code = "Canceled"
	CodeInvalidArgument  Code = "InvalidArgument"
	CodeDeadlineExceeded Code = "DeadlineExceeded"
	CodeNotFound         Code = "NotFound"
	CodeAlreadyExists    Code = "AlreadyExists"
	CodePermissionDenied Code = "PermissionDenied"
	CodeUnauthenticated  Code = "Unauthenticated"
	CodeUnavailable      Code = "Unavailable"
)

// Error is returned by SDK calls instead of transport-specific status types.
type Error struct {
	Code    Code
	Message string
	cause   error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message == "" {
		return string(e.Code)
	}
	return e.Message
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func wrapError(err error) error {
	if err == nil {
		return nil
	}
	var sdkErr *Error
	if errors.As(err, &sdkErr) {
		return err
	}
	return &Error{
		Code:    codeOf(err),
		Message: err.Error(),
		cause:   err,
	}
}

func codeOf(err error) Code {
	var sdkErr *Error
	if errors.As(err, &sdkErr) {
		return sdkErr.Code
	}
	if c := connect.CodeOf(err); c != connect.CodeUnknown {
		return codeFromConnect(c)
	}
	if s, ok := status.FromError(err); ok {
		return codeFromGRPC(s.Code())
	}
	return CodeUnknown
}

func codeFromConnect(c connect.Code) Code {
	switch c {
	case connect.CodeCanceled:
		return CodeCanceled
	case connect.CodeInvalidArgument:
		return CodeInvalidArgument
	case connect.CodeDeadlineExceeded:
		return CodeDeadlineExceeded
	case connect.CodeNotFound:
		return CodeNotFound
	case connect.CodeAlreadyExists:
		return CodeAlreadyExists
	case connect.CodePermissionDenied:
		return CodePermissionDenied
	case connect.CodeUnauthenticated:
		return CodeUnauthenticated
	case connect.CodeUnavailable:
		return CodeUnavailable
	default:
		return CodeUnknown
	}
}

func codeFromGRPC(c codes.Code) Code {
	switch c {
	case codes.Canceled:
		return CodeCanceled
	case codes.InvalidArgument:
		return CodeInvalidArgument
	case codes.DeadlineExceeded:
		return CodeDeadlineExceeded
	case codes.NotFound:
		return CodeNotFound
	case codes.AlreadyExists:
		return CodeAlreadyExists
	case codes.PermissionDenied:
		return CodePermissionDenied
	case codes.Unauthenticated:
		return CodeUnauthenticated
	case codes.Unavailable:
		return CodeUnavailable
	default:
		return CodeUnknown
	}
}

func IsNotFound(err error) bool { return codeOf(err) == CodeNotFound }

func IsAlreadyExists(err error) bool { return codeOf(err) == CodeAlreadyExists }

func IsPermissionDenied(err error) bool { return codeOf(err) == CodePermissionDenied }

func IsUnauthenticated(err error) bool { return codeOf(err) == CodeUnauthenticated }

func IsInvalidArgument(err error) bool { return codeOf(err) == CodeInvalidArgument }

func IsUnavailable(err error) bool { return codeOf(err) == CodeUnavailable }

func IsCanceled(err error) bool { return codeOf(err) == CodeCanceled }

func IsDeadlineExceeded(err error) bool { return codeOf(err) == CodeDeadlineExceeded }
