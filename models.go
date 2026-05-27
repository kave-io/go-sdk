package kave

import "time"

// This file defines the SDK's transport-neutral result models. They contain no
// proto types: timestamps are time.Time, money is MoneyAmount, metadata is
// map[string]string, and enums are the string types defined in enums.go.
// Mapping from proto messages lives in convert.go.

// Organization is a top-level tenant.
type Organization struct {
	ID        string
	Name      string
	Slug      string
	Plan      PlanTier
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Project groups environments within an organization.
type Project struct {
	ID          string
	OrgID       string
	Name        string
	Slug        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Environment is a deployment target within a project.
type Environment struct {
	ID        string
	ProjectID string
	Name      string
	Slug      string
	Type      EnvironmentType
	TrustMode TrustMode
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Agent is a registered AI agent within an environment.
type Agent struct {
	ID            string
	ProjectID     string
	EnvID         string
	Name          string
	Description   string
	PolicyID      *string
	MonthlyBudget *MoneyAmount
	Status        AgentStatus
	Metadata      map[string]any
	CreatedBy     string
	UpdatedBy     string
	DeletedAt     *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Policy is a governance policy applied to agents in an environment.
type Policy struct {
	ID                string
	ProjectID         string
	EnvID             string
	Name              string
	Description       string
	AllowedTypes      []string
	AllowedConnectors []string
	AllowedMethods    []string
	BudgetCap         *MoneyAmount
	BudgetPeriod      BudgetPeriod
	BudgetBehavior    BudgetBehavior
	TraceInput        bool
	TraceOutput       bool
	RetentionDays     int32
	Config            map[string]any
	Version           int32
	Mode              PolicyMode
	Status            PolicyStatus
	CreatedBy         string
	UpdatedBy         string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Credential is a stored connector credential. Secret material is never
// returned in plaintext; EncryptedBlob is ciphertext.
type Credential struct {
	ID              string
	ProjectID       string
	EnvID           string
	ConnectorType   string
	AccountID       string
	Label           string
	Description     string
	SourceType      CredentialSource
	EncryptedBlob   []byte
	KeyHash         string
	WrappingKeyID   string
	SecretRef       string
	SecretVersion   string
	Status          CredentialStatus
	Version         int32
	ExpiresAt       *time.Time
	RotatedAt       *time.Time
	RotatedBy       string
	LastUsedAt      *time.Time
	LastValidatedAt *time.Time
	CreatedBy       string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	RevokedAt       *time.Time
	RevokedBy       string
	RevokeReason    string
}

// Budget is a per-agent spend cap.
type Budget struct {
	ID        string
	AgentID   string
	HardCap   MoneyAmount
	SoftCap   *MoneyAmount
	Period    BudgetPeriod
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Role is an RBAC role: a named set of permissions.
type Role struct {
	ID          string
	Name        string
	Permissions []string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Binding grants a role to a subject within a scope.
type Binding struct {
	ID        string
	RoleID    string
	Subject   string
	Scope     string
	CreatedAt time.Time
}

// AgentToken is the stored record of an issued agent token (no raw secret).
type AgentToken struct {
	ID           string
	AgentID      string
	ProjectID    string
	Name         string
	Description  string
	TokenPrefix  string
	Hash         string
	IssuedFor    string
	IssuedBy     string
	Connectors   []string
	Methods      []string
	BudgetCap    *MoneyAmount
	Scopes       []string
	NotBefore    time.Time
	ExpiresAt    *time.Time
	LastUsedAt   *time.Time
	RevokedAt    *time.Time
	RevokedBy    string
	RevokeReason string
	CreatedAt    time.Time
}

// IssuedToken is returned when creating an agent token. RawToken is shown only
// once and is never stored server-side.
type IssuedToken struct {
	Token    *AgentToken
	RawToken string
}

// Run is a single agent execution.
type Run struct {
	ID             string
	ProjectID      string
	EnvID          string
	AgentID        string
	PolicyID       *string
	Name           string
	Status         RunStatus
	BudgetCap      *MoneyAmount
	Spent          *MoneyAmount
	Metadata       map[string]any
	ErrorMessage   *string
	TriggerType    TriggerType
	TriggerID      *string
	CorrelationID  *string
	SessionID      *string
	IdempotencyKey *string
	StartedAt      time.Time
	EndedAt        *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Action is a single operation within a run.
type Action struct {
	ID            string
	RunID         string
	AgentID       string
	ProjectID     string
	EnvID         string
	ParentID      *string
	ActionType    ActionType
	Connector     string
	Method        string
	Input         []byte
	Output        []byte
	Error         *string
	StartedAt     *time.Time
	EndedAt       *time.Time
	Depth         int32
	Seq           int32
	Status        ActionStatus
	Source        ActionSource
	Metadata      map[string]any
	Attempt       int32
	MaxAttempts   int32
	RetryReason   *string
	ProviderReqID *string
	ExternalID    *string
	CreatedAt     time.Time
}

// Span is a timed unit of work, optionally carrying cost and token usage.
type Span struct {
	ID                string
	ProjectID         string
	EnvID             string
	AgentID           string
	RunID             string
	ActionID          string
	ParentID          *string
	Name              string
	Kind              SpanKind
	Source            SpanSource
	Connector         string
	StartedAt         time.Time
	EndedAt           *time.Time
	DurationMs        int64
	Input             []byte
	Output            []byte
	Attrs             []byte
	Error             *string
	InputTokens       *int32
	OutputTokens      *int32
	CacheReadTokens   *int32
	CacheWriteTokens  *int32
	ReasoningTokens   *int32
	AudioInputTokens  *int32
	AudioOutputTokens *int32
	ImageUnits        *int32
	RequestCount      *int32
	ComputeMs         *int64
	StorageBytes      *int64
	BandwidthBytes    *int64
	Model             *string
	Cost              *MoneyAmount
	PriceSnapshot     *PriceSnapshot
	TraceID           string
	RootSpanID        string
	ValidationMeta    *ValidationMeta
	CreatedAt         time.Time
}

// SpanEvent is a streamed span update.
type SpanEvent struct {
	At   time.Time
	Span *Span
}

// BudgetEntry is a recorded unit of spend.
type BudgetEntry struct {
	ID                string
	ProjectID         string
	EnvID             string
	PolicyID          string
	AgentID           string
	RunID             string
	ActionID          *string
	SpanID            *string
	Connector         string
	Model             string
	InputTokens       int32
	OutputTokens      int32
	CacheReadTokens   int32
	CacheWriteTokens  int32
	ReasoningTokens   int32
	AudioInputTokens  int32
	AudioOutputTokens int32
	ImageUnits        int32
	RequestCount      int32
	ComputeMs         int64
	StorageBytes      int64
	BandwidthBytes    int64
	Cost              MoneyAmount
	PriceSnapshot     *PriceSnapshot
	UsageDetail       map[string]any
	Metadata          map[string]any
	CreatedAt         time.Time
}

// SpendReport aggregates spend over a period, broken down by dimension.
type SpendReport struct {
	Total       MoneyAmount
	ByProject   map[string]MoneyAmount
	ByEnv       map[string]MoneyAmount
	ByPolicy    map[string]MoneyAmount
	ByAgent     map[string]MoneyAmount
	ByConnector map[string]MoneyAmount
	ByModel     map[string]MoneyAmount
	PeriodStart time.Time
	PeriodEnd   time.Time
}

// PriceBook is a versioned set of price models.
type PriceBook struct {
	Version string
	Entries []PriceModel
}

// PriceModel is a per-provider/model pricing definition.
type PriceModel struct {
	Provider              string
	Match                 string
	Source                string
	InputPerMillion       *MoneyAmount
	OutputPerMillion      *MoneyAmount
	CacheReadPerMillion   *MoneyAmount
	CacheWritePerMillion  *MoneyAmount
	ReasoningPerMillion   *MoneyAmount
	AudioInputPerMillion  *MoneyAmount
	AudioOutputPerMillion *MoneyAmount
	ImageUnitPrice        *MoneyAmount
	PerRequest            *MoneyAmount
	PerComputeMs          *MoneyAmount
	PerGBStored           *MoneyAmount
	PerGBTransferred      *MoneyAmount
	EffectiveFrom         time.Time
	EffectiveTo           *time.Time
	RevisionNote          string
	Currency              string
}

// PriceSnapshot is the resolved pricing captured against a span at cost time.
type PriceSnapshot struct {
	Version               string
	Provider              string
	Model                 string
	Match                 string
	Source                string
	InputPerMillion       *MoneyAmount
	OutputPerMillion      *MoneyAmount
	CacheReadPerMillion   *MoneyAmount
	CacheWritePerMillion  *MoneyAmount
	ReasoningPerMillion   *MoneyAmount
	AudioInputPerMillion  *MoneyAmount
	AudioOutputPerMillion *MoneyAmount
	ImageUnitPrice        *MoneyAmount
	PerRequest            *MoneyAmount
	PerComputeMs          *MoneyAmount
	PerGBStored           *MoneyAmount
	PerGBTransferred      *MoneyAmount
	ResolvedAt            time.Time
	Currency              string
}

// ValidationMeta describes the outcome of policy validation against a span.
type ValidationMeta struct {
	Valid            bool
	ViolationCount   int32
	EnforcementMode  string
	ValidatorName    string
	ValidatorVersion string
	RuleVersion      string
	DurationMs       int64
	Retryable        bool
}

// RuntimeEvent is a generic streamed runtime event with JSON-encoded data.
type RuntimeEvent struct {
	Kind string
	At   time.Time
	Data []byte
}

// TraceEvent is a streamed trace event carrying a JSON-encoded run or span.
type TraceEvent struct {
	Kind string
	At   time.Time
	Data []byte
}

// LogLine is a streamed log entry.
type LogLine struct {
	At      time.Time
	Level   string
	Message string
	Context map[string]string
}

// AuditLog is an immutable audit trail entry.
type AuditLog struct {
	ID           string
	OrgID        string
	ProjectID    *string
	EnvID        *string
	ActorID      string
	ActorType    AuditActorType
	Event        string
	ResourceType string
	ResourceID   string
	DiffBefore   []byte
	DiffAfter    []byte
	IP           *string
	CreatedAt    time.Time
	Provenance   []byte
}
