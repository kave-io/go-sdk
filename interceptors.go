package kave

import (
	"context"
	"log/slog"
	"path"
	"strings"
	"time"

	"connectrpc.com/connect"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func errorInterceptor() connect.Interceptor {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			resp, err := next(ctx, req)
			return resp, wrapError(err)
		}
	})
}

func retryInterceptor(policy RetryPolicy) connect.Interceptor {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if !isIdempotentProcedure(req.Spec().Procedure) {
				resp, err := next(ctx, req)
				return resp, err
			}
			return doWithRetry(ctx, policy, func() (connect.AnyResponse, error) {
				return next(ctx, req)
			})
		}
	})
}

func observabilityInterceptor(logger *slog.Logger, tracer trace.Tracer) connect.Interceptor {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			rpc := methodName(req.Spec().Procedure)
			if tracer != nil {
				var span trace.Span
				ctx, span = tracer.Start(ctx, "kave."+rpc)
				defer span.End()
				propagation.TraceContext{}.Inject(ctx, propagation.HeaderCarrier(req.Header()))
			}

			start := time.Now()
			resp, err := next(ctx, req)
			if logger != nil {
				logger.DebugContext(ctx, "kave rpc",
					"rpc", rpc,
					"latency_ms", time.Since(start).Milliseconds(),
					"code", string(codeOf(err)),
				)
			}
			return resp, err
		}
	})
}

func isIdempotentProcedure(procedure string) bool {
	name := methodName(procedure)
	return strings.HasPrefix(name, "List") ||
		strings.HasPrefix(name, "Get") ||
		strings.HasPrefix(name, "Watch")
}

func methodName(procedure string) string {
	if procedure == "" {
		return "unknown"
	}
	return path.Base(procedure)
}
