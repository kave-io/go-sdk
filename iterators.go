package kave

import (
	"context"
	"iter"

	"connectrpc.com/connect"
	controlv1 "github.com/kave-io/kave/proto/gen/kave/control/v1"
	runtimev1 "github.com/kave-io/kave/proto/gen/kave/runtime/v1"
)

func (c *Client) IterateOrganizations(ctx context.Context) iter.Seq2[*controlv1.Organization, error] {
	return func(yield func(*controlv1.Organization, error) bool) {
		var cursor string
		for {
			resp, err := c.Control.ListOrganizations(ctx, connect.NewRequest(&controlv1.ListOrganizationsRequest{
				Limit:  defaultPageSize,
				Cursor: cursor,
			}))
			if err != nil {
				yield(nil, wrapError(err))
				return
			}
			for _, item := range resp.Msg.GetOrganizations() {
				if !yield(item, nil) {
					return
				}
			}
			cursor = resp.Msg.GetNextCursor()
			if cursor == "" {
				return
			}
		}
	}
}

func (c *Client) IterateProjects(ctx context.Context, orgID string) iter.Seq2[*controlv1.Project, error] {
	return func(yield func(*controlv1.Project, error) bool) {
		var cursor string
		for {
			resp, err := c.Control.ListProjects(ctx, connect.NewRequest(&controlv1.ListProjectsRequest{
				OrgId:  orgID,
				Limit:  defaultPageSize,
				Cursor: cursor,
			}))
			if err != nil {
				yield(nil, wrapError(err))
				return
			}
			for _, item := range resp.Msg.GetProjects() {
				if !yield(item, nil) {
					return
				}
			}
			cursor = resp.Msg.GetNextCursor()
			if cursor == "" {
				return
			}
		}
	}
}

func (c *Client) IterateEnvironments(ctx context.Context, projectID string) iter.Seq2[*controlv1.Environment, error] {
	return func(yield func(*controlv1.Environment, error) bool) {
		var cursor string
		for {
			resp, err := c.Control.ListEnvironments(ctx, connect.NewRequest(&controlv1.ListEnvironmentsRequest{
				ProjectId: projectID,
				Limit:     defaultPageSize,
				Cursor:    cursor,
			}))
			if err != nil {
				yield(nil, wrapError(err))
				return
			}
			for _, item := range resp.Msg.GetEnvironments() {
				if !yield(item, nil) {
					return
				}
			}
			cursor = resp.Msg.GetNextCursor()
			if cursor == "" {
				return
			}
		}
	}
}

func (c *Client) IterateAgents(ctx context.Context, envID string) iter.Seq2[*controlv1.Agent, error] {
	return func(yield func(*controlv1.Agent, error) bool) {
		var cursor string
		for {
			resp, err := c.Control.ListAgents(ctx, connect.NewRequest(&controlv1.ListAgentsRequest{
				EnvId:  envID,
				Limit:  defaultPageSize,
				Cursor: cursor,
			}))
			if err != nil {
				yield(nil, wrapError(err))
				return
			}
			for _, item := range resp.Msg.GetAgents() {
				if !yield(item, nil) {
					return
				}
			}
			cursor = resp.Msg.GetNextCursor()
			if cursor == "" {
				return
			}
		}
	}
}

func (c *Client) IteratePolicies(ctx context.Context, envID string) iter.Seq2[*controlv1.PolicyRecord, error] {
	return func(yield func(*controlv1.PolicyRecord, error) bool) {
		var cursor string
		for {
			resp, err := c.Control.ListPolicies(ctx, connect.NewRequest(&controlv1.ListPoliciesRequest{
				EnvId:  envID,
				Limit:  defaultPageSize,
				Cursor: cursor,
			}))
			if err != nil {
				yield(nil, wrapError(err))
				return
			}
			for _, item := range resp.Msg.GetPolicies() {
				if !yield(item, nil) {
					return
				}
			}
			cursor = resp.Msg.GetNextCursor()
			if cursor == "" {
				return
			}
		}
	}
}

func (c *Client) IterateRuns(ctx context.Context, req *runtimev1.ListRunsRequest) iter.Seq2[*runtimev1.RunRecord, error] {
	return func(yield func(*runtimev1.RunRecord, error) bool) {
		request := *req
		if request.Limit == 0 {
			request.Limit = defaultPageSize
		}
		for {
			resp, err := c.Runtime.ListRuns(ctx, connect.NewRequest(&request))
			if err != nil {
				yield(nil, wrapError(err))
				return
			}
			for _, item := range resp.Msg.GetRuns() {
				if !yield(item, nil) {
					return
				}
			}
			request.Cursor = resp.Msg.GetNextCursor()
			if request.Cursor == "" {
				return
			}
		}
	}
}

func (c *Client) IterateActions(ctx context.Context, req *runtimev1.ListActionsRequest) iter.Seq2[*runtimev1.ActionRecord, error] {
	return func(yield func(*runtimev1.ActionRecord, error) bool) {
		request := *req
		if request.Limit == 0 {
			request.Limit = defaultPageSize
		}
		for {
			resp, err := c.Runtime.ListActions(ctx, connect.NewRequest(&request))
			if err != nil {
				yield(nil, wrapError(err))
				return
			}
			for _, item := range resp.Msg.GetActions() {
				if !yield(item, nil) {
					return
				}
			}
			request.Cursor = resp.Msg.GetNextCursor()
			if request.Cursor == "" {
				return
			}
		}
	}
}
