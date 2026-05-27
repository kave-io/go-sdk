package kave

import (
	"testing"
	"time"

	commonv1 "github.com/kave-io/kave/proto/gen/kave/common/v1"
	controlv1 "github.com/kave-io/kave/proto/gen/kave/control/v1"
	runtimev1 "github.com/kave-io/kave/proto/gen/kave/runtime/v1"
)

func TestTimeHelpersRoundTrip(t *testing.T) {
	if got := msToTime(0); !got.IsZero() {
		t.Fatalf("msToTime(0) = %v, want zero", got)
	}
	if got := timeToMs(time.Time{}); got != 0 {
		t.Fatalf("timeToMs(zero) = %d, want 0", got)
	}
	ms := int64(1748000000000)
	if got := timeToMs(msToTime(ms)); got != ms {
		t.Fatalf("round trip = %d, want %d", got, ms)
	}
	if msToTimePtr(nil) != nil || timeToMsPtr(nil) != nil {
		t.Fatal("nil pointer conversions must stay nil")
	}
}

func TestEnumRoundTrips(t *testing.T) {
	runStatuses := []RunStatus{
		RunStatusActive, RunStatusCompleted, RunStatusFailed,
		RunStatusCancelled, RunStatusTimedOut, RunStatusBlocked,
	}
	for _, s := range runStatuses {
		if got := runStatusTo(runStatusFrom(s)); got != s {
			t.Fatalf("RunStatus round trip %q -> %q", s, got)
		}
	}
	if runStatusTo(runtimev1.RunStatus_RUN_STATUS_UNSPECIFIED) != "" {
		t.Fatal("unspecified RunStatus must map to empty string")
	}
	if runStatusFrom("") != runtimev1.RunStatus_RUN_STATUS_UNSPECIFIED {
		t.Fatal("empty RunStatus must map to unspecified")
	}

	envTypes := []EnvironmentType{EnvironmentTypeDev, EnvironmentTypeStaging, EnvironmentTypeProd, EnvironmentTypeCustom}
	for _, e := range envTypes {
		if got := environmentTypeTo(environmentTypeFrom(e)); got != e {
			t.Fatalf("EnvironmentType round trip %q -> %q", e, got)
		}
	}

	actionTypes := []ActionType{ActionTypeLLM, ActionTypeTool, ActionTypeRetrieval, ActionTypeMutation, ActionTypeAPI}
	for _, a := range actionTypes {
		if got := actionTypeTo(actionTypeFrom(a)); got != a {
			t.Fatalf("ActionType round trip %q -> %q", a, got)
		}
	}
}

func TestToAgentMapsOptionalFields(t *testing.T) {
	policyID := "pol_1"
	deleted := int64(1748000000000)
	proto := &controlv1.Agent{
		Id:            "agent_1",
		EnvId:         "env_1",
		Name:          "bot",
		PolicyId:      &policyID,
		MonthlyBudget: &commonv1.Amount{Currency: "USD", Decimal: "50"},
		Status:        controlv1.AgentStatus_AGENT_STATUS_ACTIVE,
		DeletedAtMs:   &deleted,
		CreatedAtMs:   1748000000000,
	}
	agent := toAgent(proto)
	if agent.ID != "agent_1" || agent.EnvID != "env_1" || agent.Name != "bot" {
		t.Fatalf("scalar fields not mapped: %+v", agent)
	}
	if agent.PolicyID == nil || *agent.PolicyID != "pol_1" {
		t.Fatalf("PolicyID = %v, want pol_1", agent.PolicyID)
	}
	if agent.MonthlyBudget == nil || agent.MonthlyBudget.Decimal != "50" {
		t.Fatalf("MonthlyBudget = %v", agent.MonthlyBudget)
	}
	if agent.Status != AgentStatusActive {
		t.Fatalf("Status = %q, want active", agent.Status)
	}
	if agent.DeletedAt == nil {
		t.Fatal("DeletedAt should be set")
	}
	if agent.CreatedAt.IsZero() {
		t.Fatal("CreatedAt should be set")
	}
}

func TestToAgentNilSafe(t *testing.T) {
	if toAgent(nil) != nil {
		t.Fatal("toAgent(nil) must be nil")
	}
}

func TestMoneyToAmountZero(t *testing.T) {
	if moneyToAmount(MoneyAmount{}) != nil {
		t.Fatal("zero MoneyAmount must convert to nil Amount")
	}
	a := moneyToAmount(AmountUSD("12.50"))
	if a == nil || a.GetCurrency() != "USD" || a.GetDecimal() != "12.50" {
		t.Fatalf("Amount = %v", a)
	}
}
