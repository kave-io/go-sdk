package kave

import (
	"context"
	"iter"

	"connectrpc.com/connect"
	runtimev1 "github.com/kave-io/kave/proto/gen/kave/runtime/v1"
)

// CreateRun starts a new run.
func (c *Client) CreateRun(ctx context.Context, in RunInput) (*Run, error) {
	req, err := in.request()
	if err != nil {
		return nil, err
	}
	resp, err := c.runtime.CreateRun(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, wrapError(err)
	}
	return toRun(resp.Msg), nil
}

// UpdateRun applies a partial update to a run.
func (c *Client) UpdateRun(ctx context.Context, in RunUpdateInput) (*Run, error) {
	req, err := in.request()
	if err != nil {
		return nil, err
	}
	resp, err := c.runtime.UpdateRun(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, wrapError(err)
	}
	return toRun(resp.Msg), nil
}

// CreateAction records an action within a run.
func (c *Client) CreateAction(ctx context.Context, in ActionInput) (*Action, error) {
	req, err := in.request()
	if err != nil {
		return nil, err
	}
	resp, err := c.runtime.CreateAction(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, wrapError(err)
	}
	return toAction(resp.Msg), nil
}

// OpenSpan opens a span.
func (c *Client) OpenSpan(ctx context.Context, in SpanInput) (*Span, error) {
	req, err := in.request()
	if err != nil {
		return nil, err
	}
	resp, err := c.runtime.OpenSpan(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, wrapError(err)
	}
	return toSpan(resp.Msg), nil
}

// CloseSpan closes an open span.
func (c *Client) CloseSpan(ctx context.Context, in SpanCloseInput) (*Span, error) {
	req, err := in.request()
	if err != nil {
		return nil, err
	}
	resp, err := c.runtime.CloseSpan(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, wrapError(err)
	}
	return toSpan(resp.Msg), nil
}

// WithSpan opens a span, runs fn, then closes the span, recording any error.
func (c *Client) WithSpan(ctx context.Context, in SpanInput, fn func(context.Context, *Span) error) (err error) {
	span, err := c.OpenSpan(ctx, in)
	if err != nil {
		return err
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			panicMsg := "panic"
			_, _ = c.CloseSpan(ctx, SpanCloseInput{SpanID: span.ID, Error: &panicMsg})
			panic(recovered)
		}
	}()
	fnErr := fn(ctx, span)
	close := SpanCloseInput{SpanID: span.ID}
	if fnErr != nil {
		msg := fnErr.Error()
		close.Error = &msg
	}
	_, closeErr := c.CloseSpan(ctx, close)
	if fnErr != nil {
		return fnErr
	}
	return closeErr
}

// IterateRuns yields runs matching the filter, paging transparently.
func (c *Client) IterateRuns(ctx context.Context, filter RunFilter) iter.Seq2[*Run, error] {
	return func(yield func(*Run, error) bool) {
		req := filter.request()
		if req.Limit == 0 {
			req.Limit = defaultPageSize
		}
		for {
			resp, err := c.runtime.ListRuns(ctx, connect.NewRequest(req))
			if err != nil {
				yield(nil, wrapError(err))
				return
			}
			for _, item := range resp.Msg.GetRuns() {
				if !yield(toRun(item), nil) {
					return
				}
			}
			if req.Cursor = resp.Msg.GetNextCursor(); req.Cursor == "" {
				return
			}
		}
	}
}

// IterateActions yields actions matching the filter, paging transparently.
func (c *Client) IterateActions(ctx context.Context, filter ActionFilter) iter.Seq2[*Action, error] {
	return func(yield func(*Action, error) bool) {
		req := filter.request()
		if req.Limit == 0 {
			req.Limit = defaultPageSize
		}
		for {
			resp, err := c.runtime.ListActions(ctx, connect.NewRequest(req))
			if err != nil {
				yield(nil, wrapError(err))
				return
			}
			for _, item := range resp.Msg.GetActions() {
				if !yield(toAction(item), nil) {
					return
				}
			}
			if req.Cursor = resp.Msg.GetNextCursor(); req.Cursor == "" {
				return
			}
		}
	}
}

// GetSpendReport returns aggregated spend for the given filter.
func (c *Client) GetSpendReport(ctx context.Context, filter SpendFilter) (*SpendReport, error) {
	resp, err := c.runtime.GetSpendReport(ctx, connect.NewRequest(filter.request()))
	if err != nil {
		return nil, wrapError(err)
	}
	return toSpendReport(resp.Msg), nil
}

// GetPriceBook returns the current price book.
func (c *Client) GetPriceBook(ctx context.Context) (*PriceBook, error) {
	resp, err := c.runtime.GetPriceBook(ctx, connect.NewRequest(&runtimev1.GetPriceBookRequest{}))
	if err != nil {
		return nil, wrapError(err)
	}
	return toPriceBook(resp.Msg), nil
}

// --- streaming ---

// WatchRuns streams run records as they change.
func (c *Client) WatchRuns(ctx context.Context, w RunWatch) iter.Seq2[*Run, error] {
	return streamSeq(ctx, w.request(), func(ctx context.Context, r *runtimev1.WatchRunsRequest) (*connect.ServerStreamForClient[runtimev1.RunRecord], error) {
		return c.runtime.WatchRuns(ctx, connect.NewRequest(r))
	}, toRun)
}

// WatchEvents streams runtime events.
func (c *Client) WatchEvents(ctx context.Context, w EventWatch) iter.Seq2[*RuntimeEvent, error] {
	return streamSeq(ctx, w.request(), func(ctx context.Context, r *runtimev1.WatchEventsRequest) (*connect.ServerStreamForClient[runtimev1.RuntimeEvent], error) {
		return c.runtime.WatchEvents(ctx, connect.NewRequest(r))
	}, toRuntimeEvent)
}

// WatchLogs streams log lines.
func (c *Client) WatchLogs(ctx context.Context, w LogWatch) iter.Seq2[*LogLine, error] {
	return streamSeq(ctx, w.request(), func(ctx context.Context, r *runtimev1.WatchLogsRequest) (*connect.ServerStreamForClient[runtimev1.LogLine], error) {
		return c.runtime.WatchLogs(ctx, connect.NewRequest(r))
	}, toLogLine)
}

// TailTraces streams trace events for a trace tail.
func (c *Client) TailTraces(ctx context.Context, w TraceTail) iter.Seq2[*TraceEvent, error] {
	return streamSeq(ctx, w.request(), func(ctx context.Context, r *runtimev1.TailTracesRequest) (*connect.ServerStreamForClient[runtimev1.TraceEvent], error) {
		return c.runtime.TailTraces(ctx, connect.NewRequest(r))
	}, toTraceEvent)
}

// StreamSpans streams span events.
func (c *Client) StreamSpans(ctx context.Context, w SpanStream) iter.Seq2[*SpanEvent, error] {
	return streamSeq(ctx, w.request(), func(ctx context.Context, r *runtimev1.StreamSpansRequest) (*connect.ServerStreamForClient[runtimev1.SpanEvent], error) {
		return c.runtime.StreamSpans(ctx, connect.NewRequest(r))
	}, toSpanEvent)
}

// streamSeq opens a server stream, retries once on a retriable open/receive
// error, and yields each message converted to its SDK model.
func streamSeq[Req any, Res any, Out any](
	ctx context.Context,
	req *Req,
	open func(context.Context, *Req) (*connect.ServerStreamForClient[Res], error),
	conv func(*Res) *Out,
) iter.Seq2[*Out, error] {
	return func(yield func(*Out, error) bool) {
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
				if !yield(conv(stream.Msg()), nil) {
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
