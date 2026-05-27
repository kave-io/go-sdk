package kave

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"connectrpc.com/connect"
	controlv1 "github.com/kave-io/kave/proto/gen/kave/control/v1"
	controlv1connect "github.com/kave-io/kave/proto/gen/kave/control/v1/controlv1connect"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestInputConversionsValidateRequiredFields(t *testing.T) {
	if _, err := (OrganizationInput{Name: "Acme"}).request(); !IsInvalidArgument(err) {
		t.Fatalf("OrganizationInput missing slug error = %v, want invalid argument", err)
	}

	budget := MonthlyBudget("agent_1", AmountUSD("50"))
	soft := AmountUSD("40")
	budget.SoftCap = &soft
	req, err := budget.request()
	if err != nil {
		t.Fatalf("MonthlyBudget request: %v", err)
	}
	if req.GetAgentId() != "agent_1" || req.GetHardCap().GetDecimal() != "50" || req.GetSoftCap().GetDecimal() != "40" {
		t.Fatalf("budget request = %+v", req)
	}
	if req.GetPeriod() != controlv1.BudgetPeriod_BUDGET_PERIOD_MONTHLY {
		t.Fatalf("period = %v, want monthly", req.GetPeriod())
	}

	roleReq, err := (RoleInput{Name: "ai-admin", Permissions: []string{"agents:read", "agents:write"}}).request()
	if err != nil {
		t.Fatalf("RoleInput request: %v", err)
	}
	if roleReq.GetName() != "ai-admin" || len(roleReq.GetPermissions()) != 2 {
		t.Fatalf("role request = %+v", roleReq)
	}

	bindingReq, err := (BindingInput{Role: "role_1", Subject: "user:123", Scope: "project:abc:*"}).request()
	if err != nil {
		t.Fatalf("BindingInput request: %v", err)
	}
	if bindingReq.GetRoleId() != "role_1" || bindingReq.GetSubject() != "user:123" {
		t.Fatalf("binding request = %+v", bindingReq)
	}
}

func TestNewFromConfigRejectsNegativeTimeout(t *testing.T) {
	_, err := NewFromConfig(ClientConfig{Timeout: -1})
	if !IsInvalidArgument(err) {
		t.Fatalf("NewFromConfig error = %v, want invalid argument", err)
	}
}

func TestBootstrapEnsuresResourcesInDependencyOrder(t *testing.T) {
	mux := http.NewServeMux()
	svc := newBootstrapControlService()
	path, handler := controlv1connect.NewControlPlaneServiceHandler(svc)
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	defer server.Close()

	client := New(WithAddr(server.URL), WithRetry(NoRetry))
	spec := BootstrapInput{
		Organization: OrganizationInput{Name: "Simorq", Slug: "simorq"},
		Project:      ProjectInput{Name: "simorq", Slug: "simorq"},
		Environments: []EnvironmentInput{Development()},
		Policies: []PolicyInput{
			EnforcePolicy("development", "default-policy"),
		},
		Agents: []AgentInput{
			{Env: "development", Name: "clinic-assistant", Policy: "default-policy"},
		},
		Budgets: []BudgetInput{
			MonthlyBudget("clinic-assistant", AmountUSD("50")),
		},
	}

	result, err := client.Bootstrap(context.Background(), spec)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	if result.Organization.ID == "" || result.Project.ID == "" {
		t.Fatal("Bootstrap returned empty organization or project IDs")
	}
	agentPolicy := result.Agents["clinic-assistant"].PolicyID
	if agentPolicy == nil || *agentPolicy != result.Policies["default-policy"].ID {
		t.Fatalf("agent policy = %v, want %q", agentPolicy, result.Policies["default-policy"].ID)
	}
	if result.Budgets[result.Agents["clinic-assistant"].ID].HardCap.Decimal != "50" {
		t.Fatalf("budget was not indexed by agent id")
	}

	_, err = client.Bootstrap(context.Background(), spec)
	if err != nil {
		t.Fatalf("second Bootstrap: %v", err)
	}
	if svc.createCounts["org"] != 1 || svc.createCounts["project"] != 1 || svc.createCounts["env"] != 1 || svc.createCounts["agent"] != 1 || svc.createCounts["policy"] != 1 {
		t.Fatalf("create counts = %#v, want one create per ensured resource", svc.createCounts)
	}
}

func TestBootstrapReportsUnknownReferences(t *testing.T) {
	mux := http.NewServeMux()
	path, handler := controlv1connect.NewControlPlaneServiceHandler(newBootstrapControlService())
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	defer server.Close()

	client := New(WithAddr(server.URL), WithRetry(NoRetry))
	_, err := client.Bootstrap(context.Background(), BootstrapInput{
		Organization: OrganizationInput{Name: "Acme", Slug: "acme"},
		Project:      ProjectInput{Name: "acme", Slug: "acme"},
		Agents:       []AgentInput{{Env: "missing-env", Name: "bot"}},
	})
	if !IsInvalidArgument(err) {
		t.Fatalf("Bootstrap error = %v, want invalid argument", err)
	}
}

type bootstrapControlService struct {
	controlv1connect.UnimplementedControlPlaneServiceHandler

	mu           sync.Mutex
	next         int
	orgs         map[string]*controlv1.Organization
	projects     map[string]*controlv1.Project
	envs         map[string]*controlv1.Environment
	agents       map[string]*controlv1.Agent
	policies     map[string]*controlv1.PolicyRecord
	budgets      map[string]*controlv1.Budget
	createCounts map[string]int
}

func newBootstrapControlService() *bootstrapControlService {
	return &bootstrapControlService{
		orgs:         make(map[string]*controlv1.Organization),
		projects:     make(map[string]*controlv1.Project),
		envs:         make(map[string]*controlv1.Environment),
		agents:       make(map[string]*controlv1.Agent),
		policies:     make(map[string]*controlv1.PolicyRecord),
		budgets:      make(map[string]*controlv1.Budget),
		createCounts: make(map[string]int),
	}
}

func (s *bootstrapControlService) id(prefix string) string {
	s.next++
	return prefix + "_" + string(rune('a'+s.next))
}

func (s *bootstrapControlService) CreateOrganization(_ context.Context, req *connect.Request[controlv1.CreateOrganizationRequest]) (*connect.Response[controlv1.Organization], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.createCounts["org"]++
	org := &controlv1.Organization{Id: s.id("org"), Name: req.Msg.GetName(), Slug: req.Msg.GetSlug()}
	s.orgs[org.GetId()] = org
	return connect.NewResponse(org), nil
}

func (s *bootstrapControlService) ListOrganizations(context.Context, *connect.Request[controlv1.ListOrganizationsRequest]) (*connect.Response[controlv1.ListOrganizationsResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	resp := &controlv1.ListOrganizationsResponse{}
	for _, org := range s.orgs {
		resp.Organizations = append(resp.Organizations, org)
	}
	return connect.NewResponse(resp), nil
}

func (s *bootstrapControlService) CreateProject(_ context.Context, req *connect.Request[controlv1.CreateProjectRequest]) (*connect.Response[controlv1.Project], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.createCounts["project"]++
	project := &controlv1.Project{Id: s.id("project"), OrgId: req.Msg.GetOrgId(), Name: req.Msg.GetName(), Slug: req.Msg.GetSlug()}
	s.projects[project.GetId()] = project
	return connect.NewResponse(project), nil
}

func (s *bootstrapControlService) ListProjects(_ context.Context, req *connect.Request[controlv1.ListProjectsRequest]) (*connect.Response[controlv1.ListProjectsResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	resp := &controlv1.ListProjectsResponse{}
	for _, project := range s.projects {
		if project.GetOrgId() == req.Msg.GetOrgId() {
			resp.Projects = append(resp.Projects, project)
		}
	}
	return connect.NewResponse(resp), nil
}

func (s *bootstrapControlService) CreateEnvironment(_ context.Context, req *connect.Request[controlv1.CreateEnvironmentRequest]) (*connect.Response[controlv1.Environment], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.createCounts["env"]++
	env := &controlv1.Environment{Id: s.id("env"), ProjectId: req.Msg.GetProjectId(), Name: req.Msg.GetName(), Slug: req.Msg.GetSlug(), Type: req.Msg.GetType()}
	s.envs[env.GetId()] = env
	return connect.NewResponse(env), nil
}

func (s *bootstrapControlService) ListEnvironments(_ context.Context, req *connect.Request[controlv1.ListEnvironmentsRequest]) (*connect.Response[controlv1.ListEnvironmentsResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	resp := &controlv1.ListEnvironmentsResponse{}
	for _, env := range s.envs {
		if env.GetProjectId() == req.Msg.GetProjectId() {
			resp.Environments = append(resp.Environments, env)
		}
	}
	return connect.NewResponse(resp), nil
}

func (s *bootstrapControlService) CreatePolicy(_ context.Context, req *connect.Request[controlv1.CreatePolicyRequest]) (*connect.Response[controlv1.PolicyRecord], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.createCounts["policy"]++
	policy := &controlv1.PolicyRecord{Id: s.id("policy"), EnvId: req.Msg.GetEnvId(), Name: req.Msg.GetName(), Description: req.Msg.GetDescription(), Mode: req.Msg.GetMode()}
	s.policies[policy.GetId()] = policy
	return connect.NewResponse(policy), nil
}

func (s *bootstrapControlService) ListPolicies(_ context.Context, req *connect.Request[controlv1.ListPoliciesRequest]) (*connect.Response[controlv1.ListPoliciesResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	resp := &controlv1.ListPoliciesResponse{}
	for _, policy := range s.policies {
		if policy.GetEnvId() == req.Msg.GetEnvId() {
			resp.Policies = append(resp.Policies, policy)
		}
	}
	return connect.NewResponse(resp), nil
}

func (s *bootstrapControlService) CreateAgent(_ context.Context, req *connect.Request[controlv1.CreateAgentRequest]) (*connect.Response[controlv1.Agent], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.createCounts["agent"]++
	agent := &controlv1.Agent{Id: s.id("agent"), EnvId: req.Msg.GetEnvId(), Name: req.Msg.GetName(), Description: req.Msg.GetDescription(), PolicyId: req.Msg.PolicyId}
	s.agents[agent.GetId()] = agent
	return connect.NewResponse(agent), nil
}

func (s *bootstrapControlService) ListAgents(_ context.Context, req *connect.Request[controlv1.ListAgentsRequest]) (*connect.Response[controlv1.ListAgentsResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	resp := &controlv1.ListAgentsResponse{}
	for _, agent := range s.agents {
		if agent.GetEnvId() == req.Msg.GetEnvId() {
			resp.Agents = append(resp.Agents, agent)
		}
	}
	return connect.NewResponse(resp), nil
}

func (s *bootstrapControlService) GetBudget(_ context.Context, req *connect.Request[controlv1.GetBudgetRequest]) (*connect.Response[controlv1.Budget], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	budget := s.budgets[req.Msg.GetAgentId()]
	if budget == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("budget not found"))
	}
	return connect.NewResponse(budget), nil
}

func (s *bootstrapControlService) CreateBudget(_ context.Context, req *connect.Request[controlv1.CreateBudgetRequest]) (*connect.Response[controlv1.Budget], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.createCounts["budget"]++
	budget := &controlv1.Budget{Id: s.id("budget"), AgentId: req.Msg.GetAgentId(), HardCap: req.Msg.GetHardCap(), SoftCap: req.Msg.SoftCap, Period: req.Msg.GetPeriod()}
	s.budgets[budget.GetAgentId()] = budget
	return connect.NewResponse(budget), nil
}

func (s *bootstrapControlService) DeleteBudget(_ context.Context, req *connect.Request[controlv1.DeleteBudgetRequest]) (*connect.Response[emptypb.Empty], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.budgets, req.Msg.GetAgentId())
	return connect.NewResponse(&emptypb.Empty{}), nil
}
