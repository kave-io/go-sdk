package kave

import (
	auditv1 "github.com/kave-io/kave/proto/gen/kave/audit/v1"
	commonv1 "github.com/kave-io/kave/proto/gen/kave/common/v1"
	controlv1 "github.com/kave-io/kave/proto/gen/kave/control/v1"
	runtimev1 "github.com/kave-io/kave/proto/gen/kave/runtime/v1"
)

// This file converts SDK Inputs into proto requests. Like convert.go it is part
// of the proto bridge; no other file builds proto requests.

func moneyToAmount(m MoneyAmount) *commonv1.Amount {
	if m.isZero() {
		return nil
	}
	return &commonv1.Amount{Currency: m.Currency, Decimal: m.Decimal}
}

func moneyPtrToAmount(m *MoneyAmount) *commonv1.Amount {
	if m == nil {
		return nil
	}
	return moneyToAmount(*m)
}

func ptrIfNotEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func bytesCopy(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	return append([]byte(nil), b...)
}

// --- control ---

func (in OrganizationInput) request() (*controlv1.CreateOrganizationRequest, error) {
	if in.Name == "" {
		return nil, invalidArgument("organization name is required")
	}
	if in.Slug == "" {
		return nil, invalidArgument("organization slug is required")
	}
	return &controlv1.CreateOrganizationRequest{Name: in.Name, Slug: in.Slug}, nil
}

func (in ProjectInput) request() (*controlv1.CreateProjectRequest, error) {
	if in.OrgID == "" {
		return nil, invalidArgument("project org id is required")
	}
	if in.Name == "" {
		return nil, invalidArgument("project name is required")
	}
	if in.Slug == "" {
		return nil, invalidArgument("project slug is required")
	}
	return &controlv1.CreateProjectRequest{OrgId: in.OrgID, Name: in.Name, Slug: in.Slug}, nil
}

func (in EnvironmentInput) request() (*controlv1.CreateEnvironmentRequest, error) {
	if in.ProjectID == "" {
		return nil, invalidArgument("environment project id is required")
	}
	if in.Name == "" {
		return nil, invalidArgument("environment name is required")
	}
	if in.Slug == "" {
		return nil, invalidArgument("environment slug is required")
	}
	typ := in.Type
	if typ == "" {
		typ = environmentTypeFromSlug(in.Slug)
	}
	return &controlv1.CreateEnvironmentRequest{
		ProjectId: in.ProjectID,
		Name:      in.Name,
		Slug:      in.Slug,
		Type:      environmentTypeFrom(typ),
	}, nil
}

func (in AgentInput) request() (*controlv1.CreateAgentRequest, error) {
	envID := firstNonEmpty(in.EnvID, in.Env)
	if envID == "" {
		return nil, invalidArgument("agent env id is required")
	}
	if in.Name == "" {
		return nil, invalidArgument("agent name is required")
	}
	req := &controlv1.CreateAgentRequest{EnvId: envID, Name: in.Name, Description: in.Description}
	if in.PolicyID != "" {
		req.PolicyId = ptrIfNotEmpty(in.PolicyID)
	}
	return req, nil
}

func (in PolicyInput) request() (*controlv1.CreatePolicyRequest, error) {
	envID := firstNonEmpty(in.EnvID, in.Env)
	if envID == "" {
		return nil, invalidArgument("policy env id is required")
	}
	if in.Name == "" {
		return nil, invalidArgument("policy name is required")
	}
	return &controlv1.CreatePolicyRequest{
		EnvId:       envID,
		Name:        in.Name,
		Description: in.Description,
		Mode:        policyModeFrom(in.Mode),
	}, nil
}

func (in BudgetInput) request() (*controlv1.CreateBudgetRequest, error) {
	agentID := firstNonEmpty(in.AgentID, in.Agent)
	if agentID == "" {
		return nil, invalidArgument("budget agent id is required")
	}
	if in.HardCap.isZero() {
		return nil, invalidArgument("budget hard cap is required")
	}
	period := in.Period
	if period == "" {
		period = BudgetPeriodMonthly
	}
	return &controlv1.CreateBudgetRequest{
		AgentId: agentID,
		HardCap: moneyToAmount(in.HardCap),
		SoftCap: moneyPtrToAmount(in.SoftCap),
		Period:  budgetPeriodFrom(period),
	}, nil
}

func (in CredentialInput) request() (*controlv1.CreateCredentialRequest, error) {
	if in.EnvID == "" {
		return nil, invalidArgument("credential env id is required")
	}
	if in.ConnectorType == "" {
		return nil, invalidArgument("credential connector type is required")
	}
	return &controlv1.CreateCredentialRequest{
		EnvId:         in.EnvID,
		ConnectorType: in.ConnectorType,
		Label:         in.Label,
		EncryptedBlob: bytesCopy(in.EncryptedBlob),
	}, nil
}

func (in TokenInput) request() (*controlv1.CreateTokenRequest, error) {
	agentID := firstNonEmpty(in.AgentID, in.Agent)
	if agentID == "" {
		return nil, invalidArgument("token agent id is required")
	}
	if in.Name == "" {
		return nil, invalidArgument("token name is required")
	}
	return &controlv1.CreateTokenRequest{AgentId: agentID, Name: in.Name}, nil
}

func (in RoleInput) request() (*controlv1.CreateRoleRequest, error) {
	if in.Name == "" {
		return nil, invalidArgument("role name is required")
	}
	return &controlv1.CreateRoleRequest{Name: in.Name, Permissions: append([]string(nil), in.Permissions...)}, nil
}

func (in BindingInput) request() (*controlv1.CreateBindingRequest, error) {
	roleID := firstNonEmpty(in.RoleID, in.Role)
	if roleID == "" {
		return nil, invalidArgument("binding role id is required")
	}
	if in.Subject == "" {
		return nil, invalidArgument("binding subject is required")
	}
	if in.Scope == "" {
		return nil, invalidArgument("binding scope is required")
	}
	return &controlv1.CreateBindingRequest{RoleId: roleID, Subject: in.Subject, Scope: in.Scope}, nil
}

// --- runtime ---

func (in RunInput) request() (*runtimev1.CreateRunRequest, error) {
	if in.ProjectID == "" {
		return nil, invalidArgument("run project id is required")
	}
	if in.EnvID == "" {
		return nil, invalidArgument("run env id is required")
	}
	if in.AgentID == "" {
		return nil, invalidArgument("run agent id is required")
	}
	req := &runtimev1.CreateRunRequest{
		ProjectId:      in.ProjectID,
		EnvId:          in.EnvID,
		AgentId:        in.AgentID,
		Name:           in.Name,
		TriggerType:    triggerTypeFrom(in.TriggerType),
		TriggerId:      ptrIfNotEmpty(in.TriggerID),
		CorrelationId:  ptrIfNotEmpty(in.CorrelationID),
		SessionId:      ptrIfNotEmpty(in.SessionID),
		IdempotencyKey: ptrIfNotEmpty(in.IdempotencyKey),
	}
	if in.PolicyID != "" {
		req.PolicyId = ptrIfNotEmpty(in.PolicyID)
	}
	return req, nil
}

func (in RunUpdateInput) request() (*runtimev1.UpdateRunRequest, error) {
	if in.ID == "" {
		return nil, invalidArgument("run id is required")
	}
	update := &runtimev1.RunUpdate{
		Spent:        moneyPtrToAmount(in.Spent),
		ErrorMessage: optStr(in.ErrorMessage),
		EndedAtMs:    timeToMsPtr(in.EndedAt),
		Metadata:     mapToStruct(in.Metadata),
	}
	if in.Status != nil {
		s := runStatusFrom(*in.Status)
		update.Status = &s
	}
	return &runtimev1.UpdateRunRequest{Id: in.ID, Update: update}, nil
}

func (in ActionInput) request() (*runtimev1.CreateActionRequest, error) {
	if in.RunID == "" {
		return nil, invalidArgument("action run id is required")
	}
	if in.AgentID == "" {
		return nil, invalidArgument("action agent id is required")
	}
	if in.ProjectID == "" {
		return nil, invalidArgument("action project id is required")
	}
	if in.EnvID == "" {
		return nil, invalidArgument("action env id is required")
	}
	if in.Connector == "" {
		return nil, invalidArgument("action connector is required")
	}
	if in.Method == "" {
		return nil, invalidArgument("action method is required")
	}
	return &runtimev1.CreateActionRequest{
		RunId:      in.RunID,
		AgentId:    in.AgentID,
		ProjectId:  in.ProjectID,
		EnvId:      in.EnvID,
		ActionType: actionTypeFrom(in.ActionType),
		Connector:  in.Connector,
		Method:     in.Method,
	}, nil
}

func (in SpanInput) request() (*runtimev1.OpenSpanRequest, error) {
	if in.Name == "" {
		return nil, invalidArgument("span name is required")
	}
	span := &runtimev1.SpanInput{
		ProjectId:   in.ProjectID,
		EnvId:       in.EnvID,
		AgentId:     in.AgentID,
		RunId:       in.RunID,
		ActionId:    in.ActionID,
		Name:        in.Name,
		Kind:        spanKindFrom(in.Kind),
		Source:      spanSourceFrom(in.Source),
		Connector:   in.Connector,
		StartedAtMs: timeToMs(in.StartedAt),
		Input:       bytesCopy(in.Input),
		TraceId:     in.TraceID,
		RootSpanId:  in.RootSpanID,
	}
	if in.ParentID != "" {
		span.ParentId = ptrIfNotEmpty(in.ParentID)
	}
	return &runtimev1.OpenSpanRequest{Span: span}, nil
}

func (in SpanCloseInput) request() (*runtimev1.CloseSpanRequest, error) {
	if in.SpanID == "" {
		return nil, invalidArgument("span id is required")
	}
	end := &runtimev1.SpanEnd{
		EndedAtMs:    timeToMsPtr(in.EndedAt),
		DurationMs:   in.DurationMs,
		Output:       bytesCopy(in.Output),
		Attrs:        bytesCopy(in.Attrs),
		Error:        optStr(in.Error),
		InputTokens:  in.InputTokens,
		OutputTokens: in.OutputTokens,
		Model:        optStr(in.Model),
		Cost:         moneyPtrToAmount(in.Cost),
		TraceId:      in.TraceID,
		RootSpanId:   in.RootSpanID,
	}
	return &runtimev1.CloseSpanRequest{SpanId: in.SpanID, End: end}, nil
}

func (f RunFilter) request() *runtimev1.ListRunsRequest {
	filter := &runtimev1.RunFilter{
		ProjectId: f.ProjectID,
		EnvId:     f.EnvID,
		AgentId:   f.AgentID,
		Status:    runStatusFrom(f.Status),
		FromMs:    timeToMsPtr(f.From),
		ToMs:      timeToMsPtr(f.To),
	}
	return &runtimev1.ListRunsRequest{Filter: filter, Limit: f.Limit}
}

func (f ActionFilter) request() *runtimev1.ListActionsRequest {
	filter := &runtimev1.ActionFilter{
		RunId:      f.RunID,
		AgentId:    f.AgentID,
		ActionType: actionTypeFrom(f.ActionType),
		Status:     actionStatusFrom(f.Status),
		Source:     actionSourceFrom(f.Source),
	}
	return &runtimev1.ListActionsRequest{Filter: filter, Limit: f.Limit}
}

func (w RunWatch) request() *runtimev1.WatchRunsRequest {
	req := &runtimev1.WatchRunsRequest{EnvId: w.EnvID}
	if w.AgentID != "" {
		req.AgentId = ptrIfNotEmpty(w.AgentID)
	}
	for _, s := range w.Statuses {
		req.Statuses = append(req.Statuses, runStatusFrom(s))
	}
	return req
}

func (w EventWatch) request() *runtimev1.WatchEventsRequest {
	return &runtimev1.WatchEventsRequest{EnvId: w.EnvID, ProjectId: w.ProjectID, Kind: w.Kind}
}

func (w LogWatch) request() *runtimev1.WatchLogsRequest {
	return &runtimev1.WatchLogsRequest{Level: w.Level}
}

func (w TraceTail) request() *runtimev1.TailTracesRequest {
	return &runtimev1.TailTracesRequest{ProjectId: w.ProjectID, EnvId: w.EnvID, RunId: w.RunID}
}

func (w SpanStream) request() *runtimev1.StreamSpansRequest {
	return &runtimev1.StreamSpansRequest{ProjectId: w.ProjectID, EnvId: w.EnvID, RunId: w.RunID}
}

func (f SpendFilter) request() *runtimev1.GetSpendReportRequest {
	return &runtimev1.GetSpendReportRequest{Filter: &runtimev1.SpendFilter{
		ProjectId: f.ProjectID,
		EnvId:     f.EnvID,
		PolicyId:  f.PolicyID,
		AgentId:   f.AgentID,
		Connector: f.Connector,
		Model:     f.Model,
		FromMs:    timeToMsPtr(f.From),
		ToMs:      timeToMsPtr(f.To),
	}}
}

// --- audit ---

func (f AuditFilter) request() *auditv1.QueryAuditsRequest {
	filter := &auditv1.AuditFilter{
		OrgId:        f.OrgID,
		ProjectId:    f.ProjectID,
		EnvId:        f.EnvID,
		ActorId:      f.ActorID,
		ResourceType: f.ResourceType,
		ResourceId:   f.ResourceID,
		Event:        f.Event,
		FromMs:       timeToMsPtr(f.From),
		ToMs:         timeToMsPtr(f.To),
	}
	return &auditv1.QueryAuditsRequest{Filter: filter, Limit: f.Limit}
}

func (in AuditEntryInput) request() (*auditv1.AppendAuditRequest, error) {
	if in.OrgID == "" {
		return nil, invalidArgument("audit org id is required")
	}
	if in.Event == "" {
		return nil, invalidArgument("audit event is required")
	}
	entry := &auditv1.AppendAuditInput{
		OrgId:        in.OrgID,
		ProjectId:    ptrIfNotEmpty(in.ProjectID),
		EnvId:        ptrIfNotEmpty(in.EnvID),
		ActorId:      in.ActorID,
		ActorType:    auditActorTypeFrom(in.ActorType),
		Event:        in.Event,
		ResourceType: in.ResourceType,
		ResourceId:   in.ResourceID,
		DiffBefore:   bytesCopy(in.DiffBefore),
		DiffAfter:    bytesCopy(in.DiffAfter),
		Ip:           ptrIfNotEmpty(in.IP),
		Provenance:   bytesCopy(in.Provenance),
	}
	return &auditv1.AppendAuditRequest{Entry: entry}, nil
}

func environmentTypeFromSlug(slug string) EnvironmentType {
	switch slug {
	case "dev", "development":
		return EnvironmentTypeDev
	case "staging":
		return EnvironmentTypeStaging
	case "prod", "production":
		return EnvironmentTypeProd
	default:
		return EnvironmentTypeCustom
	}
}
