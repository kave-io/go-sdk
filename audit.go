package kave

import (
	"context"
	"iter"

	"connectrpc.com/connect"
)

// QueryAudits yields audit log entries matching the filter, paging transparently.
func (c *Client) QueryAudits(ctx context.Context, filter AuditFilter) iter.Seq2[*AuditLog, error] {
	return func(yield func(*AuditLog, error) bool) {
		req := filter.request()
		if req.Limit == 0 {
			req.Limit = defaultPageSize
		}
		for {
			resp, err := c.audit.QueryAudits(ctx, connect.NewRequest(req))
			if err != nil {
				yield(nil, wrapError(err))
				return
			}
			for _, item := range resp.Msg.GetEntries() {
				if !yield(toAuditLog(item), nil) {
					return
				}
			}
			if req.PageToken = resp.Msg.GetNextPageToken(); req.PageToken == "" {
				return
			}
		}
	}
}

// AppendAudit appends an audit log entry and returns the stored record.
func (c *Client) AppendAudit(ctx context.Context, in AuditEntryInput) (*AuditLog, error) {
	req, err := in.request()
	if err != nil {
		return nil, err
	}
	resp, err := c.audit.AppendAudit(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, wrapError(err)
	}
	return toAuditLog(resp.Msg), nil
}
