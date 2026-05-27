package kave

import "time"

// This file defines the SDK's transport-neutral input types. They contain no
// proto types. Conversion to proto requests lives in requests.go.

// MoneyAmount is a transport-neutral money amount with a canonical decimal string.
type MoneyAmount struct {
	Currency string
	Decimal  string
}

// Amount creates a money amount with a canonical decimal string.
func Amount(currency, decimal string) MoneyAmount {
	return MoneyAmount{Currency: currency, Decimal: decimal}
}

// AmountUSD creates a USD amount with a canonical decimal string.
func AmountUSD(decimal string) MoneyAmount {
	return Amount("USD", decimal)
}

func (a MoneyAmount) isZero() bool {
	return a.Currency == "" && a.Decimal == ""
}

// OrganizationInput describes an organization to ensure or create.
type OrganizationInput struct {
	Name string
	Slug string
}

// ProjectInput describes a project to ensure or create.
type ProjectInput struct {
	OrgID       string
	Name        string
	Slug        string
	Description string
}

// EnvironmentInput describes an environment to ensure or create.
type EnvironmentInput struct {
	ProjectID string
	Name      string
	Slug      string
	Type      EnvironmentType
}

// Development returns a conventional development environment input.
func Development() EnvironmentInput {
	return EnvironmentInput{Name: "development", Slug: "development", Type: EnvironmentTypeDev}
}

// Staging returns a conventional staging environment input.
func Staging() EnvironmentInput {
	return EnvironmentInput{Name: "staging", Slug: "staging", Type: EnvironmentTypeStaging}
}

// Production returns a conventional production environment input.
func Production() EnvironmentInput {
	return EnvironmentInput{Name: "production", Slug: "production", Type: EnvironmentTypeProd}
}

// AgentInput describes an agent to ensure or create. Env is a bootstrap-local
// reference by environment slug/name; EnvID is used for direct calls. Likewise
// Policy references a policy by name within a bootstrap, PolicyID is direct.
type AgentInput struct {
	EnvID       string
	Env         string
	Name        string
	Description string
	PolicyID    string
	Policy      string
}

// PolicyInput describes a policy to ensure or create.
type PolicyInput struct {
	EnvID       string
	Env         string
	Name        string
	Description string
	Mode        PolicyMode
}

// EnforcePolicy returns an enforcing policy input scoped to an environment reference.
func EnforcePolicy(env, name string) PolicyInput {
	return PolicyInput{Env: env, Name: name, Mode: PolicyModeEnforce}
}

// BudgetInput describes a per-agent budget to ensure.
type BudgetInput struct {
	AgentID string
	Agent   string
	HardCap MoneyAmount
	SoftCap *MoneyAmount
	Period  BudgetPeriod
}

// MonthlyBudget returns a monthly budget input for an agent reference.
func MonthlyBudget(agent string, hardCap MoneyAmount) BudgetInput {
	return BudgetInput{Agent: agent, HardCap: hardCap, Period: BudgetPeriodMonthly}
}

// CredentialInput describes a connector credential to ensure or create.
type CredentialInput struct {
	EnvID         string
	ConnectorType string
	Label         string
	EncryptedBlob []byte
}

// TokenInput describes an agent token to create.
type TokenInput struct {
	AgentID string
	Agent   string
	Name    string
}

// RoleInput describes an RBAC role to ensure by name.
type RoleInput struct {
	Name        string
	Permissions []string
}

// BindingInput describes an RBAC binding to ensure.
type BindingInput struct {
	RoleID  string
	Role    string
	Subject string
	Scope   string
}

// RunInput describes a run to create.
type RunInput struct {
	ProjectID      string
	EnvID          string
	AgentID        string
	PolicyID       string
	Name           string
	TriggerType    TriggerType
	TriggerID      string
	CorrelationID  string
	SessionID      string
	IdempotencyKey string
}

// RunUpdateInput describes a partial update to a run. Only set fields are sent.
type RunUpdateInput struct {
	ID           string
	Status       *RunStatus
	Spent        *MoneyAmount
	ErrorMessage *string
	EndedAt      *time.Time
	Metadata     map[string]any
}

// ActionInput describes an action to create.
type ActionInput struct {
	RunID      string
	AgentID    string
	ProjectID  string
	EnvID      string
	ActionType ActionType
	Connector  string
	Method     string
}

// SpanInput describes a span to open.
type SpanInput struct {
	ProjectID  string
	EnvID      string
	AgentID    string
	RunID      string
	ActionID   string
	ParentID   string
	Name       string
	Kind       SpanKind
	Source     SpanSource
	Connector  string
	StartedAt  time.Time
	Input      []byte
	TraceID    string
	RootSpanID string
}

// SpanCloseInput describes how to close an open span.
type SpanCloseInput struct {
	SpanID       string
	EndedAt      *time.Time
	DurationMs   int64
	Output       []byte
	Attrs        []byte
	Error        *string
	InputTokens  *int32
	OutputTokens *int32
	Model        *string
	Cost         *MoneyAmount
	TraceID      string
	RootSpanID   string
}

// RunFilter selects runs for listing.
type RunFilter struct {
	ProjectID string
	EnvID     string
	AgentID   string
	Status    RunStatus
	From      *time.Time
	To        *time.Time
	Limit     int32
}

// ActionFilter selects actions for listing.
type ActionFilter struct {
	RunID      string
	AgentID    string
	ActionType ActionType
	Status     ActionStatus
	Source     ActionSource
	Limit      int32
}

// SpendFilter scopes a spend report. At least one scope field should be set for
// a meaningful report.
type SpendFilter struct {
	ProjectID string
	EnvID     string
	PolicyID  string
	AgentID   string
	Connector string
	Model     string
	From      *time.Time
	To        *time.Time
}

// AuditFilter selects audit log entries for querying.
type AuditFilter struct {
	OrgID        string
	ProjectID    string
	EnvID        string
	ActorID      string
	ResourceType string
	ResourceID   string
	Event        string
	From         *time.Time
	To           *time.Time
	Limit        int32
}

// AuditEntryInput describes an audit entry to append.
type AuditEntryInput struct {
	OrgID        string
	ProjectID    string
	EnvID        string
	ActorID      string
	ActorType    AuditActorType
	Event        string
	ResourceType string
	ResourceID   string
	DiffBefore   []byte
	DiffAfter    []byte
	IP           string
	Provenance   []byte
}

// RunWatch selects a live run stream.
type RunWatch struct {
	EnvID    string
	AgentID  string
	Statuses []RunStatus
}

// EventWatch selects a live runtime event stream.
type EventWatch struct {
	EnvID     string
	ProjectID string
	Kind      string
}

// LogWatch selects a live log stream.
type LogWatch struct {
	Level string
}

// TraceTail selects a live trace event stream.
type TraceTail struct {
	ProjectID string
	EnvID     string
	RunID     string
}

// SpanStream selects a live span event stream.
type SpanStream struct {
	ProjectID string
	EnvID     string
	RunID     string
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
