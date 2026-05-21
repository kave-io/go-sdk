package kave

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	commonv1 "github.com/kave-io/kave/proto/gen/kave/common/v1"
	controlv1 "github.com/kave-io/kave/proto/gen/kave/control/v1"
	runtimev1 "github.com/kave-io/kave/proto/gen/kave/runtime/v1"
	"google.golang.org/protobuf/types/known/emptypb"
)

const defaultPageSize int32 = 100

func (c *Client) EnsureOrganization(ctx context.Context, req *controlv1.CreateOrganizationRequest) (*controlv1.Organization, error) {
	orgs, err := c.listAllOrganizations(ctx)
	if err != nil {
		return nil, wrapError(err)
	}
	for _, org := range orgs {
		if org.GetSlug() == req.GetSlug() || org.GetName() == req.GetName() {
			return org, nil
		}
	}
	resp, err := c.Control.CreateOrganization(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, wrapError(err)
	}
	return resp.Msg, nil
}

func (c *Client) EnsureProject(ctx context.Context, req *controlv1.CreateProjectRequest) (*controlv1.Project, error) {
	projects, err := c.listAllProjects(ctx, req.GetOrgId())
	if err != nil {
		return nil, wrapError(err)
	}
	for _, p := range projects {
		if p.GetSlug() == req.GetSlug() || p.GetName() == req.GetName() {
			return p, nil
		}
	}
	resp, err := c.Control.CreateProject(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, wrapError(err)
	}
	return resp.Msg, nil
}

func (c *Client) EnsureEnvironment(ctx context.Context, req *controlv1.CreateEnvironmentRequest) (*controlv1.Environment, error) {
	envs, err := c.listAllEnvironments(ctx, req.GetProjectId())
	if err != nil {
		return nil, wrapError(err)
	}
	for _, env := range envs {
		if env.GetSlug() == req.GetSlug() || env.GetName() == req.GetName() {
			return env, nil
		}
	}
	resp, err := c.Control.CreateEnvironment(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, wrapError(err)
	}
	return resp.Msg, nil
}

func (c *Client) EnsureAgent(ctx context.Context, req *controlv1.CreateAgentRequest) (*controlv1.Agent, error) {
	agents, err := c.listAllAgents(ctx, req.GetEnvId())
	if err != nil {
		return nil, wrapError(err)
	}
	for _, a := range agents {
		if a.GetName() == req.GetName() {
			return a, nil
		}
	}
	resp, err := c.Control.CreateAgent(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, wrapError(err)
	}
	return resp.Msg, nil
}

func (c *Client) EnsurePolicy(ctx context.Context, req *controlv1.CreatePolicyRequest) (*controlv1.PolicyRecord, error) {
	policies, err := c.listAllPolicies(ctx, req.GetEnvId())
	if err != nil {
		return nil, wrapError(err)
	}
	for _, p := range policies {
		if p.GetName() == req.GetName() {
			return p, nil
		}
	}
	resp, err := c.Control.CreatePolicy(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, wrapError(err)
	}
	return resp.Msg, nil
}

func (c *Client) CreateAgentToken(ctx context.Context, req *controlv1.CreateTokenRequest) (*controlv1.CreateTokenResponse, error) {
	resp, err := c.Control.CreateToken(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, wrapError(err)
	}
	return resp.Msg, nil
}

// EnsureBudget creates or replaces an agent budget.
func (c *Client) EnsureBudget(ctx context.Context, req *controlv1.CreateBudgetRequest) (*controlv1.Budget, error) {
	current, err := c.Control.GetBudget(ctx, connect.NewRequest(&controlv1.GetBudgetRequest{AgentId: req.GetAgentId()}))
	if err == nil {
		if budgetsEqual(current.Msg, req) {
			return current.Msg, nil
		}
		_, err = c.Control.DeleteBudget(ctx, connect.NewRequest(&controlv1.DeleteBudgetRequest{AgentId: req.GetAgentId()}))
		if err != nil {
			return nil, wrapError(err)
		}
	} else if !IsNotFound(err) {
		return nil, wrapError(err)
	}

	created, err := c.Control.CreateBudget(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, wrapError(err)
	}
	return created.Msg, nil
}

func budgetsEqual(existing *controlv1.Budget, desired *controlv1.CreateBudgetRequest) bool {
	if existing == nil || desired == nil {
		return false
	}
	if existing.GetPeriod() != desired.GetPeriod() {
		return false
	}
	if !amountEqual(existing.GetHardCap(), desired.GetHardCap()) {
		return false
	}
	return amountEqual(existing.GetSoftCap(), desired.GetSoftCap())
}

func amountEqual(a, b *commonv1.Amount) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.GetCurrency() == b.GetCurrency() && a.GetDecimal() == b.GetDecimal()
}

// EnsureCredential looks up a credential by connector_type + label within the env;
// creates it if absent.
func (c *Client) EnsureCredential(ctx context.Context, req *controlv1.CreateCredentialRequest) (*controlv1.ConnectorCredential, error) {
	resp, err := c.Control.ListCredentials(ctx, connect.NewRequest(&controlv1.ListCredentialsRequest{
		Filter: &controlv1.CredentialFilter{
			EnvId:         req.GetEnvId(),
			ConnectorType: req.GetConnectorType(),
			Label:         req.GetLabel(),
		},
		Limit: 1,
	}))
	if err != nil {
		return nil, wrapError(err)
	}
	if len(resp.Msg.GetCredentials()) > 0 {
		return resp.Msg.GetCredentials()[0], nil
	}
	created, err := c.Control.CreateCredential(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, wrapError(err)
	}
	return created.Msg, nil
}

func (c *Client) EnsureRBACBinding(ctx context.Context, req *controlv1.CreateBindingRequest) (*controlv1.Binding, error) {
	resp, err := c.RBAC.ListBindings(ctx, connect.NewRequest(&emptypb.Empty{}))
	if err != nil {
		return nil, wrapError(err)
	}
	for _, binding := range resp.Msg.GetBindings() {
		if binding.GetRoleId() == req.GetRoleId() &&
			binding.GetSubject() == req.GetSubject() &&
			binding.GetScope() == req.GetScope() {
			return binding, nil
		}
	}
	created, err := c.RBAC.CreateBinding(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, wrapError(err)
	}
	return created.Msg, nil
}

// WithSpan opens a span, runs fn, then closes the span — recording any error.
func (c *Client) WithSpan(ctx context.Context, req *runtimev1.OpenSpanRequest, fn func(context.Context, *runtimev1.SpanRow) error) (err error) {
	span, err := c.OpenSpan(ctx, req)
	if err != nil {
		return wrapError(err)
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			_, _ = c.CloseSpan(ctx, &runtimev1.CloseSpanRequest{
				SpanId: span.GetId(),
				End: &runtimev1.SpanEnd{
					Error: stringPtr("panic"),
				},
			})
			panic(recovered)
		}
	}()
	fnErr := fn(ctx, span)
	end := &runtimev1.SpanEnd{}
	if fnErr != nil {
		errMsg := fnErr.Error()
		end.Error = &errMsg
	}
	_, closeErr := c.CloseSpan(ctx, &runtimev1.CloseSpanRequest{
		SpanId: span.GetId(),
		End:    end,
	})
	if fnErr != nil {
		return fnErr
	}
	return wrapError(closeErr)
}

func (c *Client) CreateRun(ctx context.Context, req *runtimev1.CreateRunRequest) (*runtimev1.RunRecord, error) {
	resp, err := c.Runtime.CreateRun(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, wrapError(err)
	}
	return resp.Msg, nil
}

func (c *Client) UpdateRun(ctx context.Context, req *runtimev1.UpdateRunRequest) (*runtimev1.RunRecord, error) {
	resp, err := c.Runtime.UpdateRun(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, wrapError(err)
	}
	return resp.Msg, nil
}

func (c *Client) CreateAction(ctx context.Context, req *runtimev1.CreateActionRequest) (*runtimev1.ActionRecord, error) {
	resp, err := c.Runtime.CreateAction(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, wrapError(err)
	}
	return resp.Msg, nil
}

func (c *Client) OpenSpan(ctx context.Context, req *runtimev1.OpenSpanRequest) (*runtimev1.SpanRow, error) {
	resp, err := c.Runtime.OpenSpan(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, wrapError(err)
	}
	return resp.Msg, nil
}

func (c *Client) CloseSpan(ctx context.Context, req *runtimev1.CloseSpanRequest) (*runtimev1.SpanRow, error) {
	resp, err := c.Runtime.CloseSpan(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, wrapError(err)
	}
	return resp.Msg, nil
}

func (c *Client) listAllOrganizations(ctx context.Context) ([]*controlv1.Organization, error) {
	var (
		cursor string
		out    []*controlv1.Organization
	)
	for {
		resp, err := c.Control.ListOrganizations(ctx, connect.NewRequest(&controlv1.ListOrganizationsRequest{
			Limit:  defaultPageSize,
			Cursor: cursor,
		}))
		if err != nil {
			return nil, wrapError(err)
		}
		out = append(out, resp.Msg.GetOrganizations()...)
		cursor = resp.Msg.GetNextCursor()
		if cursor == "" {
			return out, nil
		}
	}
}

func (c *Client) listAllProjects(ctx context.Context, orgID string) ([]*controlv1.Project, error) {
	if orgID == "" {
		return nil, errors.New("org_id is required")
	}
	var (
		cursor string
		out    []*controlv1.Project
	)
	for {
		resp, err := c.Control.ListProjects(ctx, connect.NewRequest(&controlv1.ListProjectsRequest{
			OrgId:  orgID,
			Limit:  defaultPageSize,
			Cursor: cursor,
		}))
		if err != nil {
			return nil, wrapError(err)
		}
		out = append(out, resp.Msg.GetProjects()...)
		cursor = resp.Msg.GetNextCursor()
		if cursor == "" {
			return out, nil
		}
	}
}

func (c *Client) listAllEnvironments(ctx context.Context, projectID string) ([]*controlv1.Environment, error) {
	if projectID == "" {
		return nil, errors.New("project_id is required")
	}
	var (
		cursor string
		out    []*controlv1.Environment
	)
	for {
		resp, err := c.Control.ListEnvironments(ctx, connect.NewRequest(&controlv1.ListEnvironmentsRequest{
			ProjectId: projectID,
			Limit:     defaultPageSize,
			Cursor:    cursor,
		}))
		if err != nil {
			return nil, wrapError(err)
		}
		out = append(out, resp.Msg.GetEnvironments()...)
		cursor = resp.Msg.GetNextCursor()
		if cursor == "" {
			return out, nil
		}
	}
}

func (c *Client) listAllAgents(ctx context.Context, envID string) ([]*controlv1.Agent, error) {
	if envID == "" {
		return nil, errors.New("env_id is required")
	}
	var (
		cursor string
		out    []*controlv1.Agent
	)
	for {
		resp, err := c.Control.ListAgents(ctx, connect.NewRequest(&controlv1.ListAgentsRequest{
			EnvId:  envID,
			Limit:  defaultPageSize,
			Cursor: cursor,
		}))
		if err != nil {
			return nil, wrapError(err)
		}
		out = append(out, resp.Msg.GetAgents()...)
		cursor = resp.Msg.GetNextCursor()
		if cursor == "" {
			return out, nil
		}
	}
}

func (c *Client) listAllPolicies(ctx context.Context, envID string) ([]*controlv1.PolicyRecord, error) {
	if envID == "" {
		return nil, errors.New("env_id is required")
	}
	var (
		cursor string
		out    []*controlv1.PolicyRecord
	)
	for {
		resp, err := c.Control.ListPolicies(ctx, connect.NewRequest(&controlv1.ListPoliciesRequest{
			EnvId:  envID,
			Limit:  defaultPageSize,
			Cursor: cursor,
		}))
		if err != nil {
			return nil, wrapError(err)
		}
		out = append(out, resp.Msg.GetPolicies()...)
		cursor = resp.Msg.GetNextCursor()
		if cursor == "" {
			return out, nil
		}
	}
}

func stringPtr(v string) *string {
	return &v
}
