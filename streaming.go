package kave

import (
	"context"
	"iter"

	"connectrpc.com/connect"
	runtimev1 "github.com/kave-io/kave/proto/gen/kave/runtime/v1"
)

func (c *Client) WatchRuns(ctx context.Context, req *runtimev1.WatchRunsRequest) iter.Seq2[*runtimev1.RunRecord, error] {
	return streamSeq(ctx, req, func(ctx context.Context, r *runtimev1.WatchRunsRequest) (*connect.ServerStreamForClient[runtimev1.RunRecord], error) {
		return c.Runtime.WatchRuns(ctx, connect.NewRequest(r))
	})
}

func (c *Client) WatchEvents(ctx context.Context, req *runtimev1.WatchEventsRequest) iter.Seq2[*runtimev1.RuntimeEvent, error] {
	return streamSeq(ctx, req, func(ctx context.Context, r *runtimev1.WatchEventsRequest) (*connect.ServerStreamForClient[runtimev1.RuntimeEvent], error) {
		return c.Runtime.WatchEvents(ctx, connect.NewRequest(r))
	})
}

func (c *Client) WatchLogs(ctx context.Context, req *runtimev1.WatchLogsRequest) iter.Seq2[*runtimev1.LogLine, error] {
	return streamSeq(ctx, req, func(ctx context.Context, r *runtimev1.WatchLogsRequest) (*connect.ServerStreamForClient[runtimev1.LogLine], error) {
		return c.Runtime.WatchLogs(ctx, connect.NewRequest(r))
	})
}

func (c *Client) TailTraces(ctx context.Context, req *runtimev1.TailTracesRequest) iter.Seq2[*runtimev1.TraceEvent, error] {
	return streamSeq(ctx, req, func(ctx context.Context, r *runtimev1.TailTracesRequest) (*connect.ServerStreamForClient[runtimev1.TraceEvent], error) {
		return c.Runtime.TailTraces(ctx, connect.NewRequest(r))
	})
}

func (c *Client) StreamSpans(ctx context.Context, req *runtimev1.StreamSpansRequest) iter.Seq2[*runtimev1.SpanEvent, error] {
	return streamSeq(ctx, req, func(ctx context.Context, r *runtimev1.StreamSpansRequest) (*connect.ServerStreamForClient[runtimev1.SpanEvent], error) {
		return c.Runtime.StreamSpans(ctx, connect.NewRequest(r))
	})
}

func streamSeq[Req any, Res any](
	ctx context.Context,
	req *Req,
	open func(context.Context, *Req) (*connect.ServerStreamForClient[Res], error),
) iter.Seq2[*Res, error] {
	return func(yield func(*Res, error) bool) {
		if req == nil {
			req = new(Req)
		}
		for attempt := 0; attempt < 2; attempt++ {
			stream, err := open(ctx, req)
			if err != nil {
				if attempt == 0 && isRetriable(err) {
					continue
				}
				yield(nil, wrapError(err))
				return
			}
			for stream.Receive() {
				if !yield(stream.Msg(), nil) {
					_ = stream.Close()
					return
				}
			}
			err = stream.Err()
			if err == nil {
				return
			}
			if attempt == 0 && isRetriable(err) {
				continue
			}
			yield(nil, wrapError(err))
			return
		}
	}
}
