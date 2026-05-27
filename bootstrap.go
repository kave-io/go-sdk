package kave

import (
	"context"
	"fmt"
)

// BootstrapInput describes a startup provisioning pass. Environments, policies,
// agents, budgets, tokens, and bindings may reference earlier resources by
// name/slug; the IDs are resolved as the pass runs.
type BootstrapInput struct {
	Organization OrganizationInput
	Project      ProjectInput
	Environments []EnvironmentInput
	Policies     []PolicyInput
	Agents       []AgentInput
	Budgets      []BudgetInput
	Tokens       []TokenInput
	Roles        []RoleInput
	Bindings     []BindingInput
}

// BootstrapResult contains resources produced by Bootstrap, keyed by id/name/slug.
type BootstrapResult struct {
	Organization *Organization
	Project      *Project
	Environments map[string]*Environment
	Policies     map[string]*Policy
	Agents       map[string]*Agent
	Budgets      map[string]*Budget
	Tokens       map[string]*IssuedToken
	Roles        map[string]*Role
	Bindings     map[string]*Binding
}

// Bootstrap ensures resources in dependency order: organization, project,
// environments, policies, agents, budgets, tokens, roles, then bindings.
func (c *Client) Bootstrap(ctx context.Context, in BootstrapInput) (*BootstrapResult, error) {
	org, err := c.EnsureOrganization(ctx, in.Organization)
	if err != nil {
		return nil, fmt.Errorf("ensure organization: %w", err)
	}

	projectInput := in.Project
	if projectInput.OrgID == "" {
		projectInput.OrgID = org.ID
	}
	project, err := c.EnsureProject(ctx, projectInput)
	if err != nil {
		return nil, fmt.Errorf("ensure project: %w", err)
	}

	result := &BootstrapResult{
		Organization: org,
		Project:      project,
		Environments: map[string]*Environment{},
		Policies:     map[string]*Policy{},
		Agents:       map[string]*Agent{},
		Budgets:      map[string]*Budget{},
		Tokens:       map[string]*IssuedToken{},
		Roles:        map[string]*Role{},
		Bindings:     map[string]*Binding{},
	}

	for _, envInput := range in.Environments {
		if envInput.ProjectID == "" {
			envInput.ProjectID = project.ID
		}
		env, err := c.EnsureEnvironment(ctx, envInput)
		if err != nil {
			return nil, fmt.Errorf("ensure environment %q: %w", envInput.Name, err)
		}
		result.Environments[env.ID] = env
		result.Environments[env.Name] = env
		result.Environments[env.Slug] = env
	}

	for _, policyInput := range in.Policies {
		if policyInput.EnvID == "" {
			env, ok := result.Environments[policyInput.Env]
			if !ok {
				return nil, invalidArgument(fmt.Sprintf("policy %q references unknown environment %q", policyInput.Name, policyInput.Env))
			}
			policyInput.EnvID = env.ID
		}
		policy, err := c.EnsurePolicy(ctx, policyInput)
		if err != nil {
			return nil, fmt.Errorf("ensure policy %q: %w", policyInput.Name, err)
		}
		result.Policies[policy.ID] = policy
		result.Policies[policy.Name] = policy
	}

	for _, agentInput := range in.Agents {
		if agentInput.EnvID == "" {
			env, ok := result.Environments[agentInput.Env]
			if !ok {
				return nil, invalidArgument(fmt.Sprintf("agent %q references unknown environment %q", agentInput.Name, agentInput.Env))
			}
			agentInput.EnvID = env.ID
		}
		if agentInput.PolicyID == "" && agentInput.Policy != "" {
			policy, ok := result.Policies[agentInput.Policy]
			if !ok {
				return nil, invalidArgument(fmt.Sprintf("agent %q references unknown policy %q", agentInput.Name, agentInput.Policy))
			}
			agentInput.PolicyID = policy.ID
		}
		agent, err := c.EnsureAgent(ctx, agentInput)
		if err != nil {
			return nil, fmt.Errorf("ensure agent %q: %w", agentInput.Name, err)
		}
		result.Agents[agent.ID] = agent
		result.Agents[agent.Name] = agent
	}

	for _, budgetInput := range in.Budgets {
		if budgetInput.AgentID == "" {
			agent, ok := result.Agents[budgetInput.Agent]
			if !ok {
				return nil, invalidArgument(fmt.Sprintf("budget references unknown agent %q", budgetInput.Agent))
			}
			budgetInput.AgentID = agent.ID
		}
		budget, err := c.EnsureBudget(ctx, budgetInput)
		if err != nil {
			return nil, fmt.Errorf("ensure budget for %q: %w", budgetInput.Agent, err)
		}
		result.Budgets[budget.AgentID] = budget
	}

	for _, tokenInput := range in.Tokens {
		if tokenInput.AgentID == "" {
			agent, ok := result.Agents[tokenInput.Agent]
			if !ok {
				return nil, invalidArgument(fmt.Sprintf("token %q references unknown agent %q", tokenInput.Name, tokenInput.Agent))
			}
			tokenInput.AgentID = agent.ID
		}
		token, err := c.CreateAgentToken(ctx, tokenInput)
		if err != nil {
			return nil, fmt.Errorf("create token %q: %w", tokenInput.Name, err)
		}
		result.Tokens[tokenInput.Name] = token
	}

	for _, roleInput := range in.Roles {
		role, err := c.EnsureRole(ctx, roleInput)
		if err != nil {
			return nil, fmt.Errorf("ensure role %q: %w", roleInput.Name, err)
		}
		result.Roles[role.ID] = role
		result.Roles[role.Name] = role
	}

	for _, bindingInput := range in.Bindings {
		if bindingInput.RoleID == "" {
			role, ok := result.Roles[bindingInput.Role]
			if !ok {
				return nil, invalidArgument(fmt.Sprintf("binding for %q references unknown role %q", bindingInput.Subject, bindingInput.Role))
			}
			bindingInput.RoleID = role.ID
		}
		binding, err := c.EnsureBinding(ctx, bindingInput)
		if err != nil {
			return nil, fmt.Errorf("ensure binding for %q: %w", bindingInput.Subject, err)
		}
		result.Bindings[binding.ID] = binding
		result.Bindings[binding.Subject+" "+binding.Scope] = binding
	}

	return result, nil
}
