package kave

import (
	"context"
	"iter"

	"connectrpc.com/connect"
	commonv1 "github.com/kave-io/kave/proto/gen/kave/common/v1"
	controlv1 "github.com/kave-io/kave/proto/gen/kave/control/v1"
	"google.golang.org/protobuf/types/known/emptypb"
)

const defaultPageSize int32 = 100

// EnsureOrganization returns the organization matching the input's slug or name,
// creating it if absent.
func (c *Client) EnsureOrganization(ctx context.Context, in OrganizationInput) (*Organization, error) {
	req, err := in.request()
	if err != nil {
		return nil, err
	}
	orgs, err := c.listAllOrganizations(ctx)
	if err != nil {
		return nil, err
	}
	for _, org := range orgs {
		if org.GetSlug() == req.GetSlug() || org.GetName() == req.GetName() {
			return toOrganization(org), nil
		}
	}
	resp, err := c.control.CreateOrganization(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, wrapError(err)
	}
	return toOrganization(resp.Msg), nil
}

// EnsureProject returns the project matching the input's slug or name within the
// org, creating it if absent.
func (c *Client) EnsureProject(ctx context.Context, in ProjectInput) (*Project, error) {
	req, err := in.request()
	if err != nil {
		return nil, err
	}
	projects, err := c.listAllProjects(ctx, req.GetOrgId())
	if err != nil {
		return nil, err
	}
	for _, p := range projects {
		if p.GetSlug() == req.GetSlug() || p.GetName() == req.GetName() {
			return toProject(p), nil
		}
	}
	resp, err := c.control.CreateProject(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, wrapError(err)
	}
	return toProject(resp.Msg), nil
}

// EnsureEnvironment returns the environment matching the input's slug or name
// within the project, creating it if absent.
func (c *Client) EnsureEnvironment(ctx context.Context, in EnvironmentInput) (*Environment, error) {
	req, err := in.request()
	if err != nil {
		return nil, err
	}
	envs, err := c.listAllEnvironments(ctx, req.GetProjectId())
	if err != nil {
		return nil, err
	}
	for _, env := range envs {
		if env.GetSlug() == req.GetSlug() || env.GetName() == req.GetName() {
			return toEnvironment(env), nil
		}
	}
	resp, err := c.control.CreateEnvironment(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, wrapError(err)
	}
	return toEnvironment(resp.Msg), nil
}

// EnsureAgent returns the agent matching the input's name within the
// environment, creating it if absent.
func (c *Client) EnsureAgent(ctx context.Context, in AgentInput) (*Agent, error) {
	req, err := in.request()
	if err != nil {
		return nil, err
	}
	agents, err := c.listAllAgents(ctx, req.GetEnvId())
	if err != nil {
		return nil, err
	}
	for _, a := range agents {
		if a.GetName() == req.GetName() {
			return toAgent(a), nil
		}
	}
	resp, err := c.control.CreateAgent(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, wrapError(err)
	}
	return toAgent(resp.Msg), nil
}

// EnsurePolicy returns the policy matching the input's name within the
// environment, creating it if absent.
func (c *Client) EnsurePolicy(ctx context.Context, in PolicyInput) (*Policy, error) {
	req, err := in.request()
	if err != nil {
		return nil, err
	}
	policies, err := c.listAllPolicies(ctx, req.GetEnvId())
	if err != nil {
		return nil, err
	}
	for _, p := range policies {
		if p.GetName() == req.GetName() {
			return toPolicy(p), nil
		}
	}
	resp, err := c.control.CreatePolicy(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, wrapError(err)
	}
	return toPolicy(resp.Msg), nil
}

// EnsureBudget creates or replaces an agent budget.
func (c *Client) EnsureBudget(ctx context.Context, in BudgetInput) (*Budget, error) {
	req, err := in.request()
	if err != nil {
		return nil, err
	}
	current, err := c.control.GetBudget(ctx, connect.NewRequest(&controlv1.GetBudgetRequest{AgentId: req.GetAgentId()}))
	if err == nil {
		if budgetMatches(current.Msg, req) {
			return toBudget(current.Msg), nil
		}
		if _, derr := c.control.DeleteBudget(ctx, connect.NewRequest(&controlv1.DeleteBudgetRequest{AgentId: req.GetAgentId()})); derr != nil {
			return nil, wrapError(derr)
		}
	} else if !IsNotFound(err) {
		return nil, wrapError(err)
	}

	created, err := c.control.CreateBudget(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, wrapError(err)
	}
	return toBudget(created.Msg), nil
}

func budgetMatches(existing *controlv1.Budget, desired *controlv1.CreateBudgetRequest) bool {
	if existing == nil || desired == nil {
		return false
	}
	if existing.GetPeriod() != desired.GetPeriod() {
		return false
	}
	if !amountsEqual(existing.GetHardCap(), desired.GetHardCap()) {
		return false
	}
	return amountsEqual(existing.GetSoftCap(), desired.GetSoftCap())
}

// EnsureCredential returns the credential matching connector_type + label within
// the env, creating it if absent.
func (c *Client) EnsureCredential(ctx context.Context, in CredentialInput) (*Credential, error) {
	req, err := in.request()
	if err != nil {
		return nil, err
	}
	resp, err := c.control.ListCredentials(ctx, connect.NewRequest(&controlv1.ListCredentialsRequest{
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
	if creds := resp.Msg.GetCredentials(); len(creds) > 0 {
		return toCredential(creds[0]), nil
	}
	created, err := c.control.CreateCredential(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, wrapError(err)
	}
	return toCredential(created.Msg), nil
}

// CreateAgentToken issues a new agent token. The returned RawToken is shown only once.
func (c *Client) CreateAgentToken(ctx context.Context, in TokenInput) (*IssuedToken, error) {
	req, err := in.request()
	if err != nil {
		return nil, err
	}
	resp, err := c.control.CreateToken(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, wrapError(err)
	}
	return toIssuedToken(resp.Msg), nil
}

// EnsureRole returns the role matching the input's name, creating it if absent.
func (c *Client) EnsureRole(ctx context.Context, in RoleInput) (*Role, error) {
	req, err := in.request()
	if err != nil {
		return nil, err
	}
	resp, err := c.rbac.ListRoles(ctx, connect.NewRequest(&emptypb.Empty{}))
	if err != nil {
		return nil, wrapError(err)
	}
	for _, role := range resp.Msg.GetRoles() {
		if role.GetName() == req.GetName() {
			return toRole(role), nil
		}
	}
	created, err := c.rbac.CreateRole(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, wrapError(err)
	}
	return toRole(created.Msg), nil
}

// EnsureBinding returns the binding matching role + subject + scope, creating it
// if absent.
func (c *Client) EnsureBinding(ctx context.Context, in BindingInput) (*Binding, error) {
	req, err := in.request()
	if err != nil {
		return nil, err
	}
	resp, err := c.rbac.ListBindings(ctx, connect.NewRequest(&emptypb.Empty{}))
	if err != nil {
		return nil, wrapError(err)
	}
	for _, binding := range resp.Msg.GetBindings() {
		if binding.GetRoleId() == req.GetRoleId() &&
			binding.GetSubject() == req.GetSubject() &&
			binding.GetScope() == req.GetScope() {
			return toBinding(binding), nil
		}
	}
	created, err := c.rbac.CreateBinding(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, wrapError(err)
	}
	return toBinding(created.Msg), nil
}

// ListRoles returns all RBAC roles.
func (c *Client) ListRoles(ctx context.Context) ([]Role, error) {
	resp, err := c.rbac.ListRoles(ctx, connect.NewRequest(&emptypb.Empty{}))
	if err != nil {
		return nil, wrapError(err)
	}
	out := make([]Role, 0, len(resp.Msg.GetRoles()))
	for _, r := range resp.Msg.GetRoles() {
		out = append(out, *toRole(r))
	}
	return out, nil
}

// ListBindings returns all RBAC bindings.
func (c *Client) ListBindings(ctx context.Context) ([]Binding, error) {
	resp, err := c.rbac.ListBindings(ctx, connect.NewRequest(&emptypb.Empty{}))
	if err != nil {
		return nil, wrapError(err)
	}
	out := make([]Binding, 0, len(resp.Msg.GetBindings()))
	for _, b := range resp.Msg.GetBindings() {
		out = append(out, *toBinding(b))
	}
	return out, nil
}

// --- iterators ---

// IterateOrganizations yields every organization, paging transparently.
func (c *Client) IterateOrganizations(ctx context.Context) iter.Seq2[*Organization, error] {
	return func(yield func(*Organization, error) bool) {
		var cursor string
		for {
			resp, err := c.control.ListOrganizations(ctx, connect.NewRequest(&controlv1.ListOrganizationsRequest{
				Limit: defaultPageSize, Cursor: cursor,
			}))
			if err != nil {
				yield(nil, wrapError(err))
				return
			}
			for _, item := range resp.Msg.GetOrganizations() {
				if !yield(toOrganization(item), nil) {
					return
				}
			}
			if cursor = resp.Msg.GetNextCursor(); cursor == "" {
				return
			}
		}
	}
}

// IterateProjects yields every project in an organization.
func (c *Client) IterateProjects(ctx context.Context, orgID string) iter.Seq2[*Project, error] {
	return func(yield func(*Project, error) bool) {
		var cursor string
		for {
			resp, err := c.control.ListProjects(ctx, connect.NewRequest(&controlv1.ListProjectsRequest{
				OrgId: orgID, Limit: defaultPageSize, Cursor: cursor,
			}))
			if err != nil {
				yield(nil, wrapError(err))
				return
			}
			for _, item := range resp.Msg.GetProjects() {
				if !yield(toProject(item), nil) {
					return
				}
			}
			if cursor = resp.Msg.GetNextCursor(); cursor == "" {
				return
			}
		}
	}
}

// IterateEnvironments yields every environment in a project.
func (c *Client) IterateEnvironments(ctx context.Context, projectID string) iter.Seq2[*Environment, error] {
	return func(yield func(*Environment, error) bool) {
		var cursor string
		for {
			resp, err := c.control.ListEnvironments(ctx, connect.NewRequest(&controlv1.ListEnvironmentsRequest{
				ProjectId: projectID, Limit: defaultPageSize, Cursor: cursor,
			}))
			if err != nil {
				yield(nil, wrapError(err))
				return
			}
			for _, item := range resp.Msg.GetEnvironments() {
				if !yield(toEnvironment(item), nil) {
					return
				}
			}
			if cursor = resp.Msg.GetNextCursor(); cursor == "" {
				return
			}
		}
	}
}

// IterateAgents yields every agent in an environment.
func (c *Client) IterateAgents(ctx context.Context, envID string) iter.Seq2[*Agent, error] {
	return func(yield func(*Agent, error) bool) {
		var cursor string
		for {
			resp, err := c.control.ListAgents(ctx, connect.NewRequest(&controlv1.ListAgentsRequest{
				EnvId: envID, Limit: defaultPageSize, Cursor: cursor,
			}))
			if err != nil {
				yield(nil, wrapError(err))
				return
			}
			for _, item := range resp.Msg.GetAgents() {
				if !yield(toAgent(item), nil) {
					return
				}
			}
			if cursor = resp.Msg.GetNextCursor(); cursor == "" {
				return
			}
		}
	}
}

// IteratePolicies yields every policy in an environment.
func (c *Client) IteratePolicies(ctx context.Context, envID string) iter.Seq2[*Policy, error] {
	return func(yield func(*Policy, error) bool) {
		var cursor string
		for {
			resp, err := c.control.ListPolicies(ctx, connect.NewRequest(&controlv1.ListPoliciesRequest{
				EnvId: envID, Limit: defaultPageSize, Cursor: cursor,
			}))
			if err != nil {
				yield(nil, wrapError(err))
				return
			}
			for _, item := range resp.Msg.GetPolicies() {
				if !yield(toPolicy(item), nil) {
					return
				}
			}
			if cursor = resp.Msg.GetNextCursor(); cursor == "" {
				return
			}
		}
	}
}

// --- internal proto-level list helpers (used by Ensure* and Bootstrap) ---

func (c *Client) listAllOrganizations(ctx context.Context) ([]*controlv1.Organization, error) {
	var (
		cursor string
		out    []*controlv1.Organization
	)
	for {
		resp, err := c.control.ListOrganizations(ctx, connect.NewRequest(&controlv1.ListOrganizationsRequest{
			Limit: defaultPageSize, Cursor: cursor,
		}))
		if err != nil {
			return nil, wrapError(err)
		}
		out = append(out, resp.Msg.GetOrganizations()...)
		if cursor = resp.Msg.GetNextCursor(); cursor == "" {
			return out, nil
		}
	}
}

func (c *Client) listAllProjects(ctx context.Context, orgID string) ([]*controlv1.Project, error) {
	if orgID == "" {
		return nil, invalidArgument("org_id is required")
	}
	var (
		cursor string
		out    []*controlv1.Project
	)
	for {
		resp, err := c.control.ListProjects(ctx, connect.NewRequest(&controlv1.ListProjectsRequest{
			OrgId: orgID, Limit: defaultPageSize, Cursor: cursor,
		}))
		if err != nil {
			return nil, wrapError(err)
		}
		out = append(out, resp.Msg.GetProjects()...)
		if cursor = resp.Msg.GetNextCursor(); cursor == "" {
			return out, nil
		}
	}
}

func (c *Client) listAllEnvironments(ctx context.Context, projectID string) ([]*controlv1.Environment, error) {
	if projectID == "" {
		return nil, invalidArgument("project_id is required")
	}
	var (
		cursor string
		out    []*controlv1.Environment
	)
	for {
		resp, err := c.control.ListEnvironments(ctx, connect.NewRequest(&controlv1.ListEnvironmentsRequest{
			ProjectId: projectID, Limit: defaultPageSize, Cursor: cursor,
		}))
		if err != nil {
			return nil, wrapError(err)
		}
		out = append(out, resp.Msg.GetEnvironments()...)
		if cursor = resp.Msg.GetNextCursor(); cursor == "" {
			return out, nil
		}
	}
}

func (c *Client) listAllAgents(ctx context.Context, envID string) ([]*controlv1.Agent, error) {
	if envID == "" {
		return nil, invalidArgument("env_id is required")
	}
	var (
		cursor string
		out    []*controlv1.Agent
	)
	for {
		resp, err := c.control.ListAgents(ctx, connect.NewRequest(&controlv1.ListAgentsRequest{
			EnvId: envID, Limit: defaultPageSize, Cursor: cursor,
		}))
		if err != nil {
			return nil, wrapError(err)
		}
		out = append(out, resp.Msg.GetAgents()...)
		if cursor = resp.Msg.GetNextCursor(); cursor == "" {
			return out, nil
		}
	}
}

func (c *Client) listAllPolicies(ctx context.Context, envID string) ([]*controlv1.PolicyRecord, error) {
	if envID == "" {
		return nil, invalidArgument("env_id is required")
	}
	var (
		cursor string
		out    []*controlv1.PolicyRecord
	)
	for {
		resp, err := c.control.ListPolicies(ctx, connect.NewRequest(&controlv1.ListPoliciesRequest{
			EnvId: envID, Limit: defaultPageSize, Cursor: cursor,
		}))
		if err != nil {
			return nil, wrapError(err)
		}
		out = append(out, resp.Msg.GetPolicies()...)
		if cursor = resp.Msg.GetNextCursor(); cursor == "" {
			return out, nil
		}
	}
}

func amountsEqual(a, b *commonv1.Amount) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.GetCurrency() == b.GetCurrency() && a.GetDecimal() == b.GetDecimal()
}
