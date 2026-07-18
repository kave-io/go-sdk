package kave

// This file defines the SDK's transport-neutral enums. Each is a string type so
// values are stable, self-documenting, and JSON-friendly. The empty string is
// the unspecified/zero value. Mapping to and from proto enums lives in convert.go.

// PlanTier is an organization's subscription tier.
type PlanTier string

const (
	PlanTierFree       PlanTier = "free"
	PlanTierTeam       PlanTier = "team"
	PlanTierEnterprise PlanTier = "enterprise"
)

// EnvironmentType classifies an environment.
type EnvironmentType string

const (
	EnvironmentTypeDev     EnvironmentType = "dev"
	EnvironmentTypeStaging EnvironmentType = "staging"
	EnvironmentTypeProd    EnvironmentType = "prod"
	EnvironmentTypeCustom  EnvironmentType = "custom"
)

// TrustMode controls how strictly an environment treats observed actions.
type TrustMode string

const (
	TrustModeStrict     TrustMode = "strict"
	TrustModePermissive TrustMode = "permissive"
)

// AgentStatus is the lifecycle state of an agent.
type AgentStatus string

const (
	AgentStatusActive   AgentStatus = "active"
	AgentStatusDisabled AgentStatus = "disabled"
)

// PolicyMode controls whether a policy enforces or only shadows.
type PolicyMode string

const (
	PolicyModeEnforce PolicyMode = "enforce"
	PolicyModeShadow  PolicyMode = "shadow"
)

// PolicyStatus is the lifecycle state of a policy.
type PolicyStatus string

const (
	PolicyStatusActive   PolicyStatus = "active"
	PolicyStatusArchived PolicyStatus = "archived"
)

// BudgetPeriod is the window a budget cap applies over.
type BudgetPeriod string

const (
	BudgetPeriodRun     BudgetPeriod = "run"
	BudgetPeriodDaily   BudgetPeriod = "daily"
	BudgetPeriodMonthly BudgetPeriod = "monthly"
)

// BudgetBehavior is what happens when a budget cap is exceeded.
type BudgetBehavior string

const (
	BudgetBehaviorBlock BudgetBehavior = "block"
	BudgetBehaviorWarn  BudgetBehavior = "warn"
)

// CredentialSource is where a connector credential's secret comes from.
type CredentialSource string

const (
	CredentialSourceEnv         CredentialSource = "env"
	CredentialSourceEncrypted   CredentialSource = "encrypted"
	CredentialSourceVaultRef    CredentialSource = "vault_ref"
	CredentialSourceOAuth       CredentialSource = "oauth"
	CredentialSourceSTS         CredentialSource = "sts"
	CredentialSourcePassthrough CredentialSource = "passthrough"
)

// CredentialStatus is the lifecycle state of a connector credential.
type CredentialStatus string

const (
	CredentialStatusActive          CredentialStatus = "active"
	CredentialStatusRevoked         CredentialStatus = "revoked"
	CredentialStatusExpired         CredentialStatus = "expired"
	CredentialStatusPendingRotation CredentialStatus = "pending_rotation"
)

// RunStatus is the lifecycle state of a run.
type RunStatus string

const (
	RunStatusActive    RunStatus = "active"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCancelled RunStatus = "cancelled"
	RunStatusTimedOut  RunStatus = "timed_out"
	RunStatusBlocked   RunStatus = "blocked"
)

// ActionType classifies an action.
type ActionType string

const (
	ActionTypeLLM       ActionType = "llm"
	ActionTypeTool      ActionType = "tool"
	ActionTypeRetrieval ActionType = "retrieval"
	ActionTypeMutation  ActionType = "mutation"
	ActionTypeAPI       ActionType = "api"
)

// ActionStatus is the lifecycle state of an action.
type ActionStatus string

const (
	ActionStatusPending   ActionStatus = "pending"
	ActionStatusRunning   ActionStatus = "running"
	ActionStatusCompleted ActionStatus = "completed"
	ActionStatusFailed    ActionStatus = "failed"
	ActionStatusBlocked   ActionStatus = "blocked"
	ActionStatusRetrying  ActionStatus = "retrying"
)

// ActionSource indicates whether an action was intercepted or only observed.
type ActionSource string

const (
	ActionSourceIntercepted ActionSource = "intercepted"
	ActionSourceObserved    ActionSource = "observed"
)

// SpanKind classifies a span.
type SpanKind string

const (
	SpanKindAction         SpanKind = "action"
	SpanKindObservedAction SpanKind = "observed_action"
	SpanKindImport         SpanKind = "import"
)

// SpanSource indicates how a span was captured.
type SpanSource string

const (
	SpanSourceIntercept  SpanSource = "intercept"
	SpanSourceReport     SpanSource = "report"
	SpanSourceOTelImport SpanSource = "otel_import"
)

// TriggerType is what initiated a run.
type TriggerType string

const (
	TriggerTypeAPI      TriggerType = "api"
	TriggerTypeSchedule TriggerType = "schedule"
	TriggerTypeWebhook  TriggerType = "webhook"
	TriggerTypeManual   TriggerType = "manual"
)

// AuditActorType is the kind of actor that produced an audit log entry.
type AuditActorType string

const (
	AuditActorTypeUser   AuditActorType = "user"
	AuditActorTypeAPIKey AuditActorType = "api_key"
	AuditActorTypeSystem AuditActorType = "system"
)
