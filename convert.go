package kave

import (
	"time"

	auditv1 "github.com/kave-io/kave/proto/gen/kave/audit/v1"
	commonv1 "github.com/kave-io/kave/proto/gen/kave/common/v1"
	controlv1 "github.com/kave-io/kave/proto/gen/kave/control/v1"
	runtimev1 "github.com/kave-io/kave/proto/gen/kave/runtime/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

// This file is the SOLE bridge between proto and the SDK's public types (aside
// from the transport handles in client.go). Public methods accept Inputs and
// return models; everything proto-shaped is converted here.

// --- scalar helpers ---

func msToTime(ms int64) time.Time {
	if ms == 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}

func msToTimePtr(ms *int64) *time.Time {
	if ms == nil {
		return nil
	}
	t := time.UnixMilli(*ms)
	return &t
}

func timeToMs(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

func timeToMsPtr(t *time.Time) *int64 {
	if t == nil || t.IsZero() {
		return nil
	}
	ms := t.UnixMilli()
	return &ms
}

func amountToMoneyPtr(a *commonv1.Amount) *MoneyAmount {
	if a == nil {
		return nil
	}
	return &MoneyAmount{Currency: a.GetCurrency(), Decimal: a.GetDecimal()}
}

func amountToMoney(a *commonv1.Amount) MoneyAmount {
	if a == nil {
		return MoneyAmount{}
	}
	return MoneyAmount{Currency: a.GetCurrency(), Decimal: a.GetDecimal()}
}

func amountMap(m map[string]*commonv1.Amount) map[string]MoneyAmount {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]MoneyAmount, len(m))
	for k, v := range m {
		out[k] = amountToMoney(v)
	}
	return out
}

func structToMap(s *structpb.Struct) map[string]any {
	if s == nil {
		return nil
	}
	return s.AsMap()
}

func mapToStruct(m map[string]any) *structpb.Struct {
	if len(m) == 0 {
		return nil
	}
	s, err := structpb.NewStruct(m)
	if err != nil {
		return nil
	}
	return s
}

// --- enums: proto -> SDK ---

func planTierTo(p controlv1.PlanTier) PlanTier {
	switch p {
	case controlv1.PlanTier_PLAN_TIER_FREE:
		return PlanTierFree
	case controlv1.PlanTier_PLAN_TIER_TEAM:
		return PlanTierTeam
	case controlv1.PlanTier_PLAN_TIER_ENTERPRISE:
		return PlanTierEnterprise
	default:
		return ""
	}
}

func environmentTypeTo(t controlv1.EnvironmentType) EnvironmentType {
	switch t {
	case controlv1.EnvironmentType_ENVIRONMENT_TYPE_DEV:
		return EnvironmentTypeDev
	case controlv1.EnvironmentType_ENVIRONMENT_TYPE_STAGING:
		return EnvironmentTypeStaging
	case controlv1.EnvironmentType_ENVIRONMENT_TYPE_PROD:
		return EnvironmentTypeProd
	case controlv1.EnvironmentType_ENVIRONMENT_TYPE_CUSTOM:
		return EnvironmentTypeCustom
	default:
		return ""
	}
}

func environmentTypeFrom(t EnvironmentType) controlv1.EnvironmentType {
	switch t {
	case EnvironmentTypeDev:
		return controlv1.EnvironmentType_ENVIRONMENT_TYPE_DEV
	case EnvironmentTypeStaging:
		return controlv1.EnvironmentType_ENVIRONMENT_TYPE_STAGING
	case EnvironmentTypeProd:
		return controlv1.EnvironmentType_ENVIRONMENT_TYPE_PROD
	case EnvironmentTypeCustom:
		return controlv1.EnvironmentType_ENVIRONMENT_TYPE_CUSTOM
	default:
		return controlv1.EnvironmentType_ENVIRONMENT_TYPE_UNSPECIFIED
	}
}

func trustModeTo(t controlv1.TrustMode) TrustMode {
	switch t {
	case controlv1.TrustMode_TRUST_MODE_STRICT:
		return TrustModeStrict
	case controlv1.TrustMode_TRUST_MODE_PERMISSIVE:
		return TrustModePermissive
	default:
		return ""
	}
}

func agentStatusTo(s controlv1.AgentStatus) AgentStatus {
	switch s {
	case controlv1.AgentStatus_AGENT_STATUS_ACTIVE:
		return AgentStatusActive
	case controlv1.AgentStatus_AGENT_STATUS_DISABLED:
		return AgentStatusDisabled
	default:
		return ""
	}
}

func policyModeTo(m controlv1.PolicyMode) PolicyMode {
	switch m {
	case controlv1.PolicyMode_POLICY_MODE_ENFORCE:
		return PolicyModeEnforce
	case controlv1.PolicyMode_POLICY_MODE_SHADOW:
		return PolicyModeShadow
	default:
		return ""
	}
}

func policyModeFrom(m PolicyMode) controlv1.PolicyMode {
	switch m {
	case PolicyModeEnforce:
		return controlv1.PolicyMode_POLICY_MODE_ENFORCE
	case PolicyModeShadow:
		return controlv1.PolicyMode_POLICY_MODE_SHADOW
	default:
		return controlv1.PolicyMode_POLICY_MODE_UNSPECIFIED
	}
}

func policyStatusTo(s controlv1.PolicyStatus) PolicyStatus {
	switch s {
	case controlv1.PolicyStatus_POLICY_STATUS_ACTIVE:
		return PolicyStatusActive
	case controlv1.PolicyStatus_POLICY_STATUS_ARCHIVED:
		return PolicyStatusArchived
	default:
		return ""
	}
}

func budgetPeriodTo(p controlv1.BudgetPeriod) BudgetPeriod {
	switch p {
	case controlv1.BudgetPeriod_BUDGET_PERIOD_RUN:
		return BudgetPeriodRun
	case controlv1.BudgetPeriod_BUDGET_PERIOD_DAILY:
		return BudgetPeriodDaily
	case controlv1.BudgetPeriod_BUDGET_PERIOD_MONTHLY:
		return BudgetPeriodMonthly
	default:
		return ""
	}
}

func budgetPeriodFrom(p BudgetPeriod) controlv1.BudgetPeriod {
	switch p {
	case BudgetPeriodRun:
		return controlv1.BudgetPeriod_BUDGET_PERIOD_RUN
	case BudgetPeriodDaily:
		return controlv1.BudgetPeriod_BUDGET_PERIOD_DAILY
	case BudgetPeriodMonthly:
		return controlv1.BudgetPeriod_BUDGET_PERIOD_MONTHLY
	default:
		return controlv1.BudgetPeriod_BUDGET_PERIOD_UNSPECIFIED
	}
}

func budgetBehaviorTo(b controlv1.BudgetBehavior) BudgetBehavior {
	switch b {
	case controlv1.BudgetBehavior_BUDGET_BEHAVIOR_BLOCK:
		return BudgetBehaviorBlock
	case controlv1.BudgetBehavior_BUDGET_BEHAVIOR_WARN:
		return BudgetBehaviorWarn
	default:
		return ""
	}
}

func credentialSourceTo(s controlv1.CredentialSource) CredentialSource {
	switch s {
	case controlv1.CredentialSource_CREDENTIAL_SOURCE_ENCRYPTED:
		return CredentialSourceEncrypted
	case controlv1.CredentialSource_CREDENTIAL_SOURCE_VAULT_REF:
		return CredentialSourceVaultRef
	case controlv1.CredentialSource_CREDENTIAL_SOURCE_OAUTH:
		return CredentialSourceOAuth
	case controlv1.CredentialSource_CREDENTIAL_SOURCE_STS:
		return CredentialSourceSTS
	case controlv1.CredentialSource_CREDENTIAL_SOURCE_PASSTHROUGH:
		return CredentialSourcePassthrough
	default:
		return ""
	}
}

func credentialStatusTo(s controlv1.CredentialStatus) CredentialStatus {
	switch s {
	case controlv1.CredentialStatus_CREDENTIAL_STATUS_ACTIVE:
		return CredentialStatusActive
	case controlv1.CredentialStatus_CREDENTIAL_STATUS_REVOKED:
		return CredentialStatusRevoked
	case controlv1.CredentialStatus_CREDENTIAL_STATUS_EXPIRED:
		return CredentialStatusExpired
	case controlv1.CredentialStatus_CREDENTIAL_STATUS_PENDING_ROTATION:
		return CredentialStatusPendingRotation
	default:
		return ""
	}
}

func runStatusTo(s runtimev1.RunStatus) RunStatus {
	switch s {
	case runtimev1.RunStatus_RUN_STATUS_ACTIVE:
		return RunStatusActive
	case runtimev1.RunStatus_RUN_STATUS_COMPLETED:
		return RunStatusCompleted
	case runtimev1.RunStatus_RUN_STATUS_FAILED:
		return RunStatusFailed
	case runtimev1.RunStatus_RUN_STATUS_CANCELLED:
		return RunStatusCancelled
	case runtimev1.RunStatus_RUN_STATUS_TIMED_OUT:
		return RunStatusTimedOut
	case runtimev1.RunStatus_RUN_STATUS_BLOCKED:
		return RunStatusBlocked
	default:
		return ""
	}
}

func runStatusFrom(s RunStatus) runtimev1.RunStatus {
	switch s {
	case RunStatusActive:
		return runtimev1.RunStatus_RUN_STATUS_ACTIVE
	case RunStatusCompleted:
		return runtimev1.RunStatus_RUN_STATUS_COMPLETED
	case RunStatusFailed:
		return runtimev1.RunStatus_RUN_STATUS_FAILED
	case RunStatusCancelled:
		return runtimev1.RunStatus_RUN_STATUS_CANCELLED
	case RunStatusTimedOut:
		return runtimev1.RunStatus_RUN_STATUS_TIMED_OUT
	case RunStatusBlocked:
		return runtimev1.RunStatus_RUN_STATUS_BLOCKED
	default:
		return runtimev1.RunStatus_RUN_STATUS_UNSPECIFIED
	}
}

func actionTypeTo(t runtimev1.ActionType) ActionType {
	switch t {
	case runtimev1.ActionType_ACTION_TYPE_LLM:
		return ActionTypeLLM
	case runtimev1.ActionType_ACTION_TYPE_TOOL:
		return ActionTypeTool
	case runtimev1.ActionType_ACTION_TYPE_RETRIEVAL:
		return ActionTypeRetrieval
	case runtimev1.ActionType_ACTION_TYPE_MUTATION:
		return ActionTypeMutation
	case runtimev1.ActionType_ACTION_TYPE_API:
		return ActionTypeAPI
	default:
		return ""
	}
}

func actionTypeFrom(t ActionType) runtimev1.ActionType {
	switch t {
	case ActionTypeLLM:
		return runtimev1.ActionType_ACTION_TYPE_LLM
	case ActionTypeTool:
		return runtimev1.ActionType_ACTION_TYPE_TOOL
	case ActionTypeRetrieval:
		return runtimev1.ActionType_ACTION_TYPE_RETRIEVAL
	case ActionTypeMutation:
		return runtimev1.ActionType_ACTION_TYPE_MUTATION
	case ActionTypeAPI:
		return runtimev1.ActionType_ACTION_TYPE_API
	default:
		return runtimev1.ActionType_ACTION_TYPE_UNSPECIFIED
	}
}

func actionStatusTo(s runtimev1.ActionStatus) ActionStatus {
	switch s {
	case runtimev1.ActionStatus_ACTION_STATUS_PENDING:
		return ActionStatusPending
	case runtimev1.ActionStatus_ACTION_STATUS_RUNNING:
		return ActionStatusRunning
	case runtimev1.ActionStatus_ACTION_STATUS_COMPLETED:
		return ActionStatusCompleted
	case runtimev1.ActionStatus_ACTION_STATUS_FAILED:
		return ActionStatusFailed
	case runtimev1.ActionStatus_ACTION_STATUS_BLOCKED:
		return ActionStatusBlocked
	case runtimev1.ActionStatus_ACTION_STATUS_RETRYING:
		return ActionStatusRetrying
	default:
		return ""
	}
}

func actionStatusFrom(s ActionStatus) runtimev1.ActionStatus {
	switch s {
	case ActionStatusPending:
		return runtimev1.ActionStatus_ACTION_STATUS_PENDING
	case ActionStatusRunning:
		return runtimev1.ActionStatus_ACTION_STATUS_RUNNING
	case ActionStatusCompleted:
		return runtimev1.ActionStatus_ACTION_STATUS_COMPLETED
	case ActionStatusFailed:
		return runtimev1.ActionStatus_ACTION_STATUS_FAILED
	case ActionStatusBlocked:
		return runtimev1.ActionStatus_ACTION_STATUS_BLOCKED
	case ActionStatusRetrying:
		return runtimev1.ActionStatus_ACTION_STATUS_RETRYING
	default:
		return runtimev1.ActionStatus_ACTION_STATUS_UNSPECIFIED
	}
}

func actionSourceTo(s runtimev1.ActionSource) ActionSource {
	switch s {
	case runtimev1.ActionSource_ACTION_SOURCE_INTERCEPTED:
		return ActionSourceIntercepted
	case runtimev1.ActionSource_ACTION_SOURCE_OBSERVED:
		return ActionSourceObserved
	default:
		return ""
	}
}

func actionSourceFrom(s ActionSource) runtimev1.ActionSource {
	switch s {
	case ActionSourceIntercepted:
		return runtimev1.ActionSource_ACTION_SOURCE_INTERCEPTED
	case ActionSourceObserved:
		return runtimev1.ActionSource_ACTION_SOURCE_OBSERVED
	default:
		return runtimev1.ActionSource_ACTION_SOURCE_UNSPECIFIED
	}
}

func spanKindTo(k runtimev1.SpanKind) SpanKind {
	switch k {
	case runtimev1.SpanKind_SPAN_KIND_ACTION:
		return SpanKindAction
	case runtimev1.SpanKind_SPAN_KIND_OBSERVED_ACTION:
		return SpanKindObservedAction
	case runtimev1.SpanKind_SPAN_KIND_IMPORT:
		return SpanKindImport
	default:
		return ""
	}
}

func spanKindFrom(k SpanKind) runtimev1.SpanKind {
	switch k {
	case SpanKindAction:
		return runtimev1.SpanKind_SPAN_KIND_ACTION
	case SpanKindObservedAction:
		return runtimev1.SpanKind_SPAN_KIND_OBSERVED_ACTION
	case SpanKindImport:
		return runtimev1.SpanKind_SPAN_KIND_IMPORT
	default:
		return runtimev1.SpanKind_SPAN_KIND_UNSPECIFIED
	}
}

func spanSourceTo(s runtimev1.SpanSource) SpanSource {
	switch s {
	case runtimev1.SpanSource_SPAN_SOURCE_INTERCEPT:
		return SpanSourceIntercept
	case runtimev1.SpanSource_SPAN_SOURCE_REPORT:
		return SpanSourceReport
	case runtimev1.SpanSource_SPAN_SOURCE_OTEL_IMPORT:
		return SpanSourceOTelImport
	default:
		return ""
	}
}

func spanSourceFrom(s SpanSource) runtimev1.SpanSource {
	switch s {
	case SpanSourceIntercept:
		return runtimev1.SpanSource_SPAN_SOURCE_INTERCEPT
	case SpanSourceReport:
		return runtimev1.SpanSource_SPAN_SOURCE_REPORT
	case SpanSourceOTelImport:
		return runtimev1.SpanSource_SPAN_SOURCE_OTEL_IMPORT
	default:
		return runtimev1.SpanSource_SPAN_SOURCE_UNSPECIFIED
	}
}

func triggerTypeTo(t runtimev1.TriggerType) TriggerType {
	switch t {
	case runtimev1.TriggerType_TRIGGER_TYPE_API:
		return TriggerTypeAPI
	case runtimev1.TriggerType_TRIGGER_TYPE_SCHEDULE:
		return TriggerTypeSchedule
	case runtimev1.TriggerType_TRIGGER_TYPE_WEBHOOK:
		return TriggerTypeWebhook
	case runtimev1.TriggerType_TRIGGER_TYPE_MANUAL:
		return TriggerTypeManual
	default:
		return ""
	}
}

func triggerTypeFrom(t TriggerType) runtimev1.TriggerType {
	switch t {
	case TriggerTypeAPI:
		return runtimev1.TriggerType_TRIGGER_TYPE_API
	case TriggerTypeSchedule:
		return runtimev1.TriggerType_TRIGGER_TYPE_SCHEDULE
	case TriggerTypeWebhook:
		return runtimev1.TriggerType_TRIGGER_TYPE_WEBHOOK
	case TriggerTypeManual:
		return runtimev1.TriggerType_TRIGGER_TYPE_MANUAL
	default:
		return runtimev1.TriggerType_TRIGGER_TYPE_UNSPECIFIED
	}
}

func auditActorTypeTo(a auditv1.AuditActorType) AuditActorType {
	switch a {
	case auditv1.AuditActorType_AUDIT_ACTOR_TYPE_USER:
		return AuditActorTypeUser
	case auditv1.AuditActorType_AUDIT_ACTOR_TYPE_API_KEY:
		return AuditActorTypeAPIKey
	case auditv1.AuditActorType_AUDIT_ACTOR_TYPE_SYSTEM:
		return AuditActorTypeSystem
	default:
		return ""
	}
}

func auditActorTypeFrom(a AuditActorType) auditv1.AuditActorType {
	switch a {
	case AuditActorTypeUser:
		return auditv1.AuditActorType_AUDIT_ACTOR_TYPE_USER
	case AuditActorTypeAPIKey:
		return auditv1.AuditActorType_AUDIT_ACTOR_TYPE_API_KEY
	case AuditActorTypeSystem:
		return auditv1.AuditActorType_AUDIT_ACTOR_TYPE_SYSTEM
	default:
		return auditv1.AuditActorType_AUDIT_ACTOR_TYPE_UNSPECIFIED
	}
}

// --- proto -> model: control ---

func toOrganization(o *controlv1.Organization) *Organization {
	if o == nil {
		return nil
	}
	return &Organization{
		ID:        o.GetId(),
		Name:      o.GetName(),
		Slug:      o.GetSlug(),
		Plan:      planTierTo(o.GetPlan()),
		CreatedAt: msToTime(o.GetCreatedAtMs()),
		UpdatedAt: msToTime(o.GetUpdatedAtMs()),
	}
}

func toProject(p *controlv1.Project) *Project {
	if p == nil {
		return nil
	}
	return &Project{
		ID:          p.GetId(),
		OrgID:       p.GetOrgId(),
		Name:        p.GetName(),
		Slug:        p.GetSlug(),
		Description: p.GetDescription(),
		CreatedAt:   msToTime(p.GetCreatedAtMs()),
		UpdatedAt:   msToTime(p.GetUpdatedAtMs()),
	}
}

func toEnvironment(e *controlv1.Environment) *Environment {
	if e == nil {
		return nil
	}
	return &Environment{
		ID:        e.GetId(),
		ProjectID: e.GetProjectId(),
		Name:      e.GetName(),
		Slug:      e.GetSlug(),
		Type:      environmentTypeTo(e.GetType()),
		TrustMode: trustModeTo(e.GetTrustMode()),
		CreatedAt: msToTime(e.GetCreatedAtMs()),
		UpdatedAt: msToTime(e.GetUpdatedAtMs()),
	}
}

func toAgent(a *controlv1.Agent) *Agent {
	if a == nil {
		return nil
	}
	out := &Agent{
		ID:            a.GetId(),
		ProjectID:     a.GetProjectId(),
		EnvID:         a.GetEnvId(),
		Name:          a.GetName(),
		Description:   a.GetDescription(),
		MonthlyBudget: amountToMoneyPtr(a.GetMonthlyBudget()),
		Status:        agentStatusTo(a.GetStatus()),
		Metadata:      structToMap(a.GetMetadata()),
		CreatedBy:     a.GetCreatedBy(),
		UpdatedBy:     a.GetUpdatedBy(),
		DeletedAt:     msToTimePtr(a.DeletedAtMs),
		CreatedAt:     msToTime(a.GetCreatedAtMs()),
		UpdatedAt:     msToTime(a.GetUpdatedAtMs()),
	}
	if a.PolicyId != nil {
		v := a.GetPolicyId()
		out.PolicyID = &v
	}
	return out
}

func toPolicy(p *controlv1.PolicyRecord) *Policy {
	if p == nil {
		return nil
	}
	return &Policy{
		ID:                p.GetId(),
		ProjectID:         p.GetProjectId(),
		EnvID:             p.GetEnvId(),
		Name:              p.GetName(),
		Description:       p.GetDescription(),
		AllowedTypes:      p.GetAllowedTypes(),
		AllowedConnectors: p.GetAllowedConnectors(),
		AllowedMethods:    p.GetAllowedMethods(),
		BudgetCap:         amountToMoneyPtr(p.GetBudgetCap()),
		BudgetPeriod:      budgetPeriodTo(p.GetBudgetPeriod()),
		BudgetBehavior:    budgetBehaviorTo(p.GetBudgetBehavior()),
		TraceInput:        p.GetTraceInput(),
		TraceOutput:       p.GetTraceOutput(),
		RetentionDays:     p.GetRetentionDays(),
		Config:            structToMap(p.GetConfig()),
		Version:           p.GetVersion(),
		Mode:              policyModeTo(p.GetMode()),
		Status:            policyStatusTo(p.GetStatus()),
		CreatedBy:         p.GetCreatedBy(),
		UpdatedBy:         p.GetUpdatedBy(),
		CreatedAt:         msToTime(p.GetCreatedAtMs()),
		UpdatedAt:         msToTime(p.GetUpdatedAtMs()),
	}
}

func toCredential(c *controlv1.ConnectorCredential) *Credential {
	if c == nil {
		return nil
	}
	return &Credential{
		ID:              c.GetId(),
		ProjectID:       c.GetProjectId(),
		EnvID:           c.GetEnvId(),
		ConnectorType:   c.GetConnectorType(),
		AccountID:       c.GetAccountId(),
		Label:           c.GetLabel(),
		Description:     c.GetDescription(),
		SourceType:      credentialSourceTo(c.GetSourceType()),
		EncryptedBlob:   c.GetEncryptedBlob(),
		KeyHash:         c.GetKeyHash(),
		WrappingKeyID:   c.GetWrappingKeyId(),
		SecretRef:       c.GetSecretRef(),
		SecretVersion:   c.GetSecretVersion(),
		Status:          credentialStatusTo(c.GetStatus()),
		Version:         c.GetVersion(),
		ExpiresAt:       msToTimePtr(c.ExpiresAtMs),
		RotatedAt:       msToTimePtr(c.RotatedAtMs),
		RotatedBy:       c.GetRotatedBy(),
		LastUsedAt:      msToTimePtr(c.LastUsedAtMs),
		LastValidatedAt: msToTimePtr(c.LastValidatedAtMs),
		CreatedBy:       c.GetCreatedBy(),
		CreatedAt:       msToTime(c.GetCreatedAtMs()),
		UpdatedAt:       msToTime(c.GetUpdatedAtMs()),
		RevokedAt:       msToTimePtr(c.RevokedAtMs),
		RevokedBy:       c.GetRevokedBy(),
		RevokeReason:    c.GetRevokeReason(),
	}
}

func toBudget(b *controlv1.Budget) *Budget {
	if b == nil {
		return nil
	}
	return &Budget{
		ID:        b.GetId(),
		AgentID:   b.GetAgentId(),
		HardCap:   amountToMoney(b.GetHardCap()),
		SoftCap:   amountToMoneyPtr(b.GetSoftCap()),
		Period:    budgetPeriodTo(b.GetPeriod()),
		CreatedAt: msToTime(b.GetCreatedAtMs()),
		UpdatedAt: msToTime(b.GetUpdatedAtMs()),
	}
}

func toRole(r *controlv1.Role) *Role {
	if r == nil {
		return nil
	}
	return &Role{
		ID:          r.GetId(),
		Name:        r.GetName(),
		Permissions: r.GetPermissions(),
		CreatedAt:   msToTime(r.GetCreatedAt()),
		UpdatedAt:   msToTime(r.GetUpdatedAt()),
	}
}

func toBinding(b *controlv1.Binding) *Binding {
	if b == nil {
		return nil
	}
	return &Binding{
		ID:        b.GetId(),
		RoleID:    b.GetRoleId(),
		Subject:   b.GetSubject(),
		Scope:     b.GetScope(),
		CreatedAt: msToTime(b.GetCreatedAt()),
	}
}

func toAgentToken(t *controlv1.AgentToken) *AgentToken {
	if t == nil {
		return nil
	}
	return &AgentToken{
		ID:           t.GetId(),
		AgentID:      t.GetAgentId(),
		ProjectID:    t.GetProjectId(),
		Name:         t.GetName(),
		Description:  t.GetDescription(),
		TokenPrefix:  t.GetTokenPrefix(),
		Hash:         t.GetHash(),
		IssuedFor:    t.GetIssuedFor(),
		IssuedBy:     t.GetIssuedBy(),
		Connectors:   t.GetConnectors(),
		Methods:      t.GetMethods(),
		BudgetCap:    amountToMoneyPtr(t.GetBudgetCap()),
		Scopes:       t.GetScopes(),
		NotBefore:    msToTime(t.GetNotBeforeMs()),
		ExpiresAt:    msToTimePtr(t.ExpiresAtMs),
		LastUsedAt:   msToTimePtr(t.LastUsedAtMs),
		RevokedAt:    msToTimePtr(t.RevokedAtMs),
		RevokedBy:    t.GetRevokedBy(),
		RevokeReason: t.GetRevokeReason(),
		CreatedAt:    msToTime(t.GetCreatedAtMs()),
	}
}

func toIssuedToken(r *controlv1.CreateTokenResponse) *IssuedToken {
	if r == nil {
		return nil
	}
	return &IssuedToken{
		Token:    toAgentToken(r.GetToken()),
		RawToken: r.GetRawToken(),
	}
}

// --- proto -> model: runtime ---

func toRun(r *runtimev1.RunRecord) *Run {
	if r == nil {
		return nil
	}
	return &Run{
		ID:             r.GetId(),
		ProjectID:      r.GetProjectId(),
		EnvID:          r.GetEnvId(),
		AgentID:        r.GetAgentId(),
		PolicyID:       optStr(r.PolicyId),
		Name:           r.GetName(),
		Status:         runStatusTo(r.GetStatus()),
		BudgetCap:      amountToMoneyPtr(r.GetBudgetCap()),
		Spent:          amountToMoneyPtr(r.GetSpent()),
		Metadata:       structToMap(r.GetMetadata()),
		ErrorMessage:   optStr(r.ErrorMessage),
		TriggerType:    triggerTypeTo(r.GetTriggerType()),
		TriggerID:      optStr(r.TriggerId),
		CorrelationID:  optStr(r.CorrelationId),
		SessionID:      optStr(r.SessionId),
		IdempotencyKey: optStr(r.IdempotencyKey),
		StartedAt:      msToTime(r.GetStartedAtMs()),
		EndedAt:        msToTimePtr(r.EndedAtMs),
		CreatedAt:      msToTime(r.GetCreatedAtMs()),
		UpdatedAt:      msToTime(r.GetUpdatedAtMs()),
	}
}

func toAction(a *runtimev1.ActionRecord) *Action {
	if a == nil {
		return nil
	}
	return &Action{
		ID:            a.GetId(),
		RunID:         a.GetRunId(),
		AgentID:       a.GetAgentId(),
		ProjectID:     a.GetProjectId(),
		EnvID:         a.GetEnvId(),
		ParentID:      optStr(a.ParentId),
		ActionType:    actionTypeTo(a.GetActionType()),
		Connector:     a.GetConnector(),
		Method:        a.GetMethod(),
		Input:         a.GetInput(),
		Output:        a.GetOutput(),
		Error:         optStr(a.Error),
		StartedAt:     msToTimePtr(a.StartedAtMs),
		EndedAt:       msToTimePtr(a.EndedAtMs),
		Depth:         a.GetDepth(),
		Seq:           a.GetSeq(),
		Status:        actionStatusTo(a.GetStatus()),
		Source:        actionSourceTo(a.GetSource()),
		Metadata:      structToMap(a.GetMetadata()),
		Attempt:       a.GetAttempt(),
		MaxAttempts:   a.GetMaxAttempts(),
		RetryReason:   optStr(a.RetryReason),
		ProviderReqID: optStr(a.ProviderReqId),
		ExternalID:    optStr(a.ExternalId),
		CreatedAt:     msToTime(a.GetCreatedAtMs()),
	}
}

func toSpan(s *runtimev1.SpanRow) *Span {
	if s == nil {
		return nil
	}
	return &Span{
		ID:                s.GetId(),
		ProjectID:         s.GetProjectId(),
		EnvID:             s.GetEnvId(),
		AgentID:           s.GetAgentId(),
		RunID:             s.GetRunId(),
		ActionID:          s.GetActionId(),
		ParentID:          optStr(s.ParentId),
		Name:              s.GetName(),
		Kind:              spanKindTo(s.GetKind()),
		Source:            spanSourceTo(s.GetSource()),
		Connector:         s.GetConnector(),
		StartedAt:         msToTime(s.GetStartedAtMs()),
		EndedAt:           msToTimePtr(s.EndedAtMs),
		DurationMs:        s.GetDurationMs(),
		Input:             s.GetInput(),
		Output:            s.GetOutput(),
		Attrs:             s.GetAttrs(),
		Error:             optStr(s.Error),
		InputTokens:       s.InputTokens,
		OutputTokens:      s.OutputTokens,
		CacheReadTokens:   s.CacheReadTokens,
		CacheWriteTokens:  s.CacheWriteTokens,
		ReasoningTokens:   s.ReasoningTokens,
		AudioInputTokens:  s.AudioInputTokens,
		AudioOutputTokens: s.AudioOutputTokens,
		ImageUnits:        s.ImageUnits,
		RequestCount:      s.RequestCount,
		ComputeMs:         s.ComputeMs,
		StorageBytes:      s.StorageBytes,
		BandwidthBytes:    s.BandwidthBytes,
		Model:             optStr(s.Model),
		Cost:              amountToMoneyPtr(s.GetCost()),
		PriceSnapshot:     toPriceSnapshot(s.GetPriceSnapshot()),
		TraceID:           s.GetTraceId(),
		RootSpanID:        s.GetRootSpanId(),
		ValidationMeta:    toValidationMeta(s.GetValidationMeta()),
		CreatedAt:         msToTime(s.GetCreatedAtMs()),
	}
}

func toSpanEvent(e *runtimev1.SpanEvent) *SpanEvent {
	if e == nil {
		return nil
	}
	return &SpanEvent{
		At:   msToTime(e.GetAt()),
		Span: toSpan(e.GetSpan()),
	}
}

func toPriceSnapshot(p *runtimev1.PriceSnapshot) *PriceSnapshot {
	if p == nil {
		return nil
	}
	return &PriceSnapshot{
		Version:               p.GetVersion(),
		Provider:              p.GetProvider(),
		Model:                 p.GetModel(),
		Match:                 p.GetMatch(),
		Source:                p.GetSource(),
		InputPerMillion:       amountToMoneyPtr(p.GetInputPerMillion()),
		OutputPerMillion:      amountToMoneyPtr(p.GetOutputPerMillion()),
		CacheReadPerMillion:   amountToMoneyPtr(p.GetCacheReadPerMillion()),
		CacheWritePerMillion:  amountToMoneyPtr(p.GetCacheWritePerMillion()),
		ReasoningPerMillion:   amountToMoneyPtr(p.GetReasoningPerMillion()),
		AudioInputPerMillion:  amountToMoneyPtr(p.GetAudioInputPerMillion()),
		AudioOutputPerMillion: amountToMoneyPtr(p.GetAudioOutputPerMillion()),
		ImageUnitPrice:        amountToMoneyPtr(p.GetImageUnitPrice()),
		PerRequest:            amountToMoneyPtr(p.GetPerRequest()),
		PerComputeMs:          amountToMoneyPtr(p.GetPerComputeMs()),
		PerGBStored:           amountToMoneyPtr(p.GetPerGbStored()),
		PerGBTransferred:      amountToMoneyPtr(p.GetPerGbTransferred()),
		ResolvedAt:            msToTime(p.GetResolvedAtMs()),
		Currency:              p.GetCurrency(),
	}
}

func toPriceModel(p *runtimev1.PriceModel) PriceModel {
	return PriceModel{
		Provider:              p.GetProvider(),
		Match:                 p.GetMatch(),
		Source:                p.GetSource(),
		InputPerMillion:       amountToMoneyPtr(p.GetInputPerMillion()),
		OutputPerMillion:      amountToMoneyPtr(p.GetOutputPerMillion()),
		CacheReadPerMillion:   amountToMoneyPtr(p.GetCacheReadPerMillion()),
		CacheWritePerMillion:  amountToMoneyPtr(p.GetCacheWritePerMillion()),
		ReasoningPerMillion:   amountToMoneyPtr(p.GetReasoningPerMillion()),
		AudioInputPerMillion:  amountToMoneyPtr(p.GetAudioInputPerMillion()),
		AudioOutputPerMillion: amountToMoneyPtr(p.GetAudioOutputPerMillion()),
		ImageUnitPrice:        amountToMoneyPtr(p.GetImageUnitPrice()),
		PerRequest:            amountToMoneyPtr(p.GetPerRequest()),
		PerComputeMs:          amountToMoneyPtr(p.GetPerComputeMs()),
		PerGBStored:           amountToMoneyPtr(p.GetPerGbStored()),
		PerGBTransferred:      amountToMoneyPtr(p.GetPerGbTransferred()),
		EffectiveFrom:         msToTime(p.GetEffectiveFromMs()),
		EffectiveTo:           msToTimePtr(p.EffectiveToMs),
		RevisionNote:          p.GetRevisionNote(),
		Currency:              p.GetCurrency(),
	}
}

func toPriceBook(b *runtimev1.PriceBook) *PriceBook {
	if b == nil {
		return nil
	}
	out := &PriceBook{Version: b.GetVersion()}
	for _, e := range b.GetEntries() {
		out.Entries = append(out.Entries, toPriceModel(e))
	}
	return out
}

func toValidationMeta(v *runtimev1.ValidationMeta) *ValidationMeta {
	if v == nil {
		return nil
	}
	return &ValidationMeta{
		Valid:            v.GetValid(),
		ViolationCount:   v.GetViolationCount(),
		EnforcementMode:  v.GetEnforcementMode(),
		ValidatorName:    v.GetValidatorName(),
		ValidatorVersion: v.GetValidatorVersion(),
		RuleVersion:      v.GetRuleVersion(),
		DurationMs:       v.GetDurationMs(),
		Retryable:        v.GetRetryable(),
	}
}

func toSpendReport(r *runtimev1.SpendReport) *SpendReport {
	if r == nil {
		return nil
	}
	return &SpendReport{
		Total:       amountToMoney(r.GetTotal()),
		ByProject:   amountMap(r.GetByProject()),
		ByEnv:       amountMap(r.GetByEnv()),
		ByPolicy:    amountMap(r.GetByPolicy()),
		ByAgent:     amountMap(r.GetByAgent()),
		ByConnector: amountMap(r.GetByConnector()),
		ByModel:     amountMap(r.GetByModel()),
		PeriodStart: msToTime(r.GetPeriodStartMs()),
		PeriodEnd:   msToTime(r.GetPeriodEndMs()),
	}
}

func toRuntimeEvent(e *runtimev1.RuntimeEvent) *RuntimeEvent {
	if e == nil {
		return nil
	}
	return &RuntimeEvent{Kind: e.GetKind(), At: msToTime(e.GetAt()), Data: e.GetData()}
}

func toTraceEvent(e *runtimev1.TraceEvent) *TraceEvent {
	if e == nil {
		return nil
	}
	return &TraceEvent{Kind: e.GetKind(), At: msToTime(e.GetAt()), Data: e.GetData()}
}

func toLogLine(l *runtimev1.LogLine) *LogLine {
	if l == nil {
		return nil
	}
	return &LogLine{At: msToTime(l.GetAt()), Level: l.GetLevel(), Message: l.GetMessage(), Context: l.GetContext()}
}

// --- proto -> model: audit ---

func toAuditLog(a *auditv1.AuditLog) *AuditLog {
	if a == nil {
		return nil
	}
	return &AuditLog{
		ID:           a.GetId(),
		OrgID:        a.GetOrgId(),
		ProjectID:    optStr(a.ProjectId),
		EnvID:        optStr(a.EnvId),
		ActorID:      a.GetActorId(),
		ActorType:    auditActorTypeTo(a.GetActorType()),
		Event:        a.GetEvent(),
		ResourceType: a.GetResourceType(),
		ResourceID:   a.GetResourceId(),
		DiffBefore:   a.GetDiffBefore(),
		DiffAfter:    a.GetDiffAfter(),
		IP:           optStr(a.Ip),
		CreatedAt:    msToTime(a.GetCreatedAtMs()),
		Provenance:   a.GetProvenance(),
	}
}

// optStr copies an optional proto string field into a fresh pointer.
func optStr(p *string) *string {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
