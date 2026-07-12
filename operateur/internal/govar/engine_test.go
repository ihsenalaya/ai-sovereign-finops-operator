package govar

import (
	"sync"
	"testing"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testTenant   = "tenant-a"
	testWorkload = "workload-uid-a"
)

func TestEngineReserveDispatchSettleCorrectAndFinalize(t *testing.T) {
	engine := NewEngine()
	admit := mustAdmit(t, engine, "r1", testTenant, testWorkload, defaultCandidates())
	if admit.ReservedCostMicros != 3_600 {
		t.Fatalf("reserved micros = %d, want 3600", admit.ReservedCostMicros)
	}
	assertLiability(t, engine.Liability(testTenant), 0, 3_600, 99_996_400, 1)

	res, code, err := engine.Dispatch(dispatchRequest("r1", "d1", admit.ProviderAttemptID, DispatchClaimed, testTenant, testWorkload))
	if err != nil || code != ReasonDispatchClaimed || res.State != StateDispatchPending {
		t.Fatalf("claim = (%+v,%s,%v)", res, code, err)
	}
	res, code, err = engine.Dispatch(dispatchRequest("r1", "d2", admit.ProviderAttemptID, DispatchDelivered, testTenant, testWorkload))
	if err != nil || code != ReasonDispatchDelivered || res.State != StateDispatched {
		t.Fatalf("delivery = (%+v,%s,%v)", res, code, err)
	}

	res, code, err = engine.Settle(settleRequest("r1", "s1", 2_000, 1, false, testTenant, testWorkload))
	if err != nil || code != ReasonProvisionalSettlement || res.ResidualHoldMicros != 1_600 {
		t.Fatalf("provisional settle = (%+v,%s,%v)", res, code, err)
	}
	assertLiability(t, engine.Liability(testTenant), 2_000, 1_600, 99_996_400, 1)

	res, code, err = engine.Settle(settleRequest("r1", "s2", 2_500, 2, false, testTenant, testWorkload))
	if err != nil || code != ReasonCorrection || res.ResidualHoldMicros != 1_100 {
		t.Fatalf("correction = (%+v,%s,%v)", res, code, err)
	}
	assertLiability(t, engine.Liability(testTenant), 2_500, 1_100, 99_996_400, 1)

	res, code, err = engine.Settle(settleRequest("r1", "s3", 2_500, 2, true, testTenant, testWorkload))
	if err != nil || code != ReasonFinalized || !res.Finalized || res.ResidualHoldMicros != 0 {
		t.Fatalf("finality = (%+v,%s,%v)", res, code, err)
	}
	assertLiability(t, engine.Liability(testTenant), 2_500, 0, 99_997_500, 0)

	_, code, err = engine.Settle(settleRequest("r1", "s3", 2_500, 2, true, testTenant, testWorkload))
	if err != nil || code != ReasonSettlementDuplicate {
		t.Fatalf("duplicate settlement = (%s,%v)", code, err)
	}
	assertLiability(t, engine.Liability(testTenant), 2_500, 0, 99_997_500, 0)
}

func TestEnginePendingCancellationAtomicallyBlocksDispatch(t *testing.T) {
	engine := NewEngine()
	admit := mustAdmit(t, engine, "cancel-pending", testTenant, testWorkload, defaultCandidates())
	res, code, err := engine.Cancel(cancelRequest("cancel-pending", "c1", false, testTenant, testWorkload))
	if err != nil || code != ReasonCanceled || res.OutboxState != OutboxCanceled || res.State != StateCanceledUnbilled {
		t.Fatalf("cancel = (%+v,%s,%v)", res, code, err)
	}
	assertLiability(t, engine.Liability(testTenant), 0, 0, 100_000_000, 0)
	_, code, err = engine.Dispatch(dispatchRequest("cancel-pending", "d-after-cancel", admit.ProviderAttemptID, DispatchClaimed, testTenant, testWorkload))
	if err == nil || code != ReasonInvalidTransition {
		t.Fatalf("dispatch after cancel = (%s,%v), want invalid transition", code, err)
	}
}

func TestEngineRejectsSettlementBeforeDispatchClaim(t *testing.T) {
	engine := NewEngine()
	mustAdmit(t, engine, "unclaimed", testTenant, testWorkload, defaultCandidates())
	if _, code, err := engine.Settle(settleRequest("unclaimed", "usage", 100, 1, true, testTenant, testWorkload)); err == nil || code != ReasonInvalidTransition {
		t.Fatalf("unclaimed settlement=(%s,%v)", code, err)
	}
	assertLiability(t, engine.Liability(testTenant), 0, 3_600, 99_996_400, 1)
}

func TestEngineFailsClosedOnBudgetWindowChangeWithExposure(t *testing.T) {
	engine := NewEngine()
	mustAdmit(t, engine, "window-1", testTenant, testWorkload, defaultCandidates())
	changed := defaultBudget()
	changed.Generation = 2
	changed.Status.ObservedGeneration = 2
	changed.Spec.BudgetEUR = resource.MustParse("200")
	response, err := engine.Admit(admitRequest("window-2", testTenant, testWorkload), changed, defaultRouting(), defaultCandidates())
	if err != nil || response.Decision != DecisionReject || response.ReasonCode != ReasonBudgetWindowConflict {
		t.Fatalf("changed window response=%+v err=%v", response, err)
	}
}

func TestApprovalIsPolicyDerivedNotCallerControlled(t *testing.T) {
	engine := NewEngine()
	req := admitRequest("caller-approval", testTenant, testWorkload)
	req.RequireApproval = true
	response, err := engine.Admit(req, defaultBudget(), defaultRouting(), defaultCandidates())
	if err != nil || response.Decision != DecisionAdmit {
		t.Fatalf("caller boolean controlled approval: %+v %v", response, err)
	}
	routing := defaultRouting()
	routing.Spec.Canary.Enabled = true
	response, err = engine.Admit(admitRequest("policy-approval", testTenant, testWorkload), defaultBudget(), routing, defaultCandidates())
	if err != nil || response.Decision != DecisionRequireApproval {
		t.Fatalf("policy approval not enforced: %+v %v", response, err)
	}
}

func TestEngineAmbiguousCancelRetainsLiability(t *testing.T) {
	engine := NewEngine()
	admit := mustAdmit(t, engine, "ambiguous", testTenant, testWorkload, defaultCandidates())
	_, _, err := engine.Dispatch(dispatchRequest("ambiguous", "d1", admit.ProviderAttemptID, DispatchClaimed, testTenant, testWorkload))
	if err != nil {
		t.Fatal(err)
	}
	res, code, err := engine.Cancel(cancelRequest("ambiguous", "c1", false, testTenant, testWorkload))
	if err != nil || code != ReasonCancellationUnclear || res.State != StateUnresolved {
		t.Fatalf("ambiguous cancel = (%+v,%s,%v)", res, code, err)
	}
	assertLiability(t, engine.Liability(testTenant), 0, 3_600, 99_996_400, 1)

	res, code, err = engine.Cancel(cancelRequest("ambiguous", "c2", true, testTenant, testWorkload))
	if err != nil || code != ReasonCanceled || res.State != StateFailedUnbilled {
		t.Fatalf("authoritative unbilled = (%+v,%s,%v)", res, code, err)
	}
	assertLiability(t, engine.Liability(testTenant), 0, 0, 100_000_000, 0)
}

func TestEngineRejectsCrossPrincipalMutationAndDuplicateRequest(t *testing.T) {
	engine := NewEngine()
	admit := mustAdmit(t, engine, "owned", testTenant, testWorkload, defaultCandidates())
	bad := dispatchRequest("owned", "attack", admit.ProviderAttemptID, DispatchClaimed, "tenant-b", "workload-b")
	if _, code, err := engine.Dispatch(bad); err == nil || code != ReasonPrincipalMismatch {
		t.Fatalf("cross-principal dispatch = (%s,%v)", code, err)
	}
	request := admitRequest("owned", "tenant-b", "workload-b")
	if _, err := engine.Admit(request, defaultBudget(), defaultRouting(), defaultCandidates()); err == nil {
		t.Fatal("duplicate request ID from another principal was accepted")
	}
	assertLiability(t, engine.Liability(testTenant), 0, 3_600, 99_996_400, 1)
}

func TestEngineRejectsStaleAndConflictingCorrections(t *testing.T) {
	engine := NewEngine()
	admit := mustAdmit(t, engine, "versions", testTenant, testWorkload, defaultCandidates())
	if _, _, err := engine.Dispatch(dispatchRequest("versions", "versions-claim", admit.ProviderAttemptID, DispatchClaimed, testTenant, testWorkload)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := engine.Settle(settleRequest("versions", "s1", 2_000, 2, false, testTenant, testWorkload)); err != nil {
		t.Fatal(err)
	}
	if _, code, err := engine.Settle(settleRequest("versions", "stale", 1_900, 1, false, testTenant, testWorkload)); err == nil || code != ReasonInvalidTransition {
		t.Fatalf("stale correction = (%s,%v)", code, err)
	}
	if _, code, err := engine.Settle(settleRequest("versions", "conflict", 2_100, 2, false, testTenant, testWorkload)); err == nil || code != ReasonInvalidTransition {
		t.Fatalf("conflicting correction = (%s,%v)", code, err)
	}
	assertLiability(t, engine.Liability(testTenant), 2_000, 1_600, 99_996_400, 1)
}

func TestEngineRejectsSameEventIDWithConflictingPayload(t *testing.T) {
	engine := NewEngine()
	admit := mustAdmit(t, engine, "payload", testTenant, testWorkload, defaultCandidates())
	if _, _, err := engine.Dispatch(dispatchRequest("payload", "same-event", admit.ProviderAttemptID, DispatchClaimed, testTenant, testWorkload)); err != nil {
		t.Fatal(err)
	}
	if _, code, err := engine.Dispatch(dispatchRequest("payload", "same-event", admit.ProviderAttemptID, DispatchDelivered, testTenant, testWorkload)); err == nil || code != ReasonDuplicateEvent {
		t.Fatalf("conflicting replay = (%s,%v)", code, err)
	}
}

func TestEnginePostFinalityCorrectionBecomesVisibleCarriedDebt(t *testing.T) {
	engine := NewEngine()
	admit := mustAdmit(t, engine, "debt", testTenant, testWorkload, defaultCandidates())
	if _, _, err := engine.Dispatch(dispatchRequest("debt", "debt-claim", admit.ProviderAttemptID, DispatchClaimed, testTenant, testWorkload)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := engine.Settle(settleRequest("debt", "final", 2_000, 1, true, testTenant, testWorkload)); err != nil {
		t.Fatal(err)
	}
	res, code, err := engine.Settle(settleRequest("debt", "late-correction", 2_700, 2, true, testTenant, testWorkload))
	if err != nil || code != ReasonReservationExceeded || res.ProvisionalCostMicros != 2_700 {
		t.Fatalf("post-final correction=(%+v,%s,%v)", res, code, err)
	}
	got := engine.Liability(testTenant)
	if got.SettledSpendMicros != 2_000 || got.CarriedAdjustmentMicros != 700 || got.AvailableBudgetMicros != 99_997_300 {
		t.Fatalf("post-final debt liability=%+v", got)
	}
}

func TestEnginePostFinalityDownwardCorrectionDoesNotMintCredit(t *testing.T) {
	engine := NewEngine()
	admit := mustAdmit(t, engine, "no-credit", testTenant, testWorkload, defaultCandidates())
	if _, _, err := engine.Dispatch(dispatchRequest("no-credit", "no-credit-claim", admit.ProviderAttemptID, DispatchClaimed, testTenant, testWorkload)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := engine.Settle(settleRequest("no-credit", "final", 2_000, 1, true, testTenant, testWorkload)); err != nil {
		t.Fatal(err)
	}
	if _, code, err := engine.Settle(settleRequest("no-credit", "down", 1_000, 2, true, testTenant, testWorkload)); err != nil || code != ReasonCorrection {
		t.Fatalf("downward correction=(%s,%v)", code, err)
	}
	got := engine.Liability(testTenant)
	if got.CarriedAdjustmentMicros != 0 || got.AvailableBudgetMicros != 99_998_000 {
		t.Fatalf("downward correction minted credit: %+v", got)
	}
}

func TestEngineDuplicateAdmissionFingerprint(t *testing.T) {
	engine := NewEngine()
	req := admitRequest("fingerprint", testTenant, testWorkload)
	if _, err := engine.Admit(req, defaultBudget(), defaultRouting(), defaultCandidates()); err != nil {
		t.Fatal(err)
	}
	duplicate, err := engine.Admit(req, defaultBudget(), defaultRouting(), defaultCandidates())
	if err != nil || duplicate.ReasonCode != ReasonDuplicateRequest {
		t.Fatalf("exact duplicate=(%+v,%v)", duplicate, err)
	}
	req.MaxOutputTokens++
	if _, err := engine.Admit(req, defaultBudget(), defaultRouting(), defaultCandidates()); err == nil {
		t.Fatal("conflicting duplicate admission was accepted")
	}
}

func TestEngineDeterministicCandidateTieBreak(t *testing.T) {
	engine := NewEngine()
	candidates := []Candidate{
		{ModelRef: "z-deployment", InputPriceMicrosPerMillion: 400_000, OutputPriceMicrosPerMillion: 1_600_000, PricingVersion: "v1", ContextWindow: 100_000, QualityScore: 1, VerifiedOutputCap: true, Feasible: true},
		{ModelRef: "a-deployment", InputPriceMicrosPerMillion: 400_000, OutputPriceMicrosPerMillion: 1_600_000, PricingVersion: "v1", ContextWindow: 100_000, QualityScore: 1, VerifiedOutputCap: true, Feasible: true},
	}
	resp := mustAdmit(t, engine, "tie", testTenant, testWorkload, candidates)
	if resp.SelectedDeployment != "a-deployment" {
		t.Fatalf("selected %q, want deterministic a-deployment", resp.SelectedDeployment)
	}
}

func TestEngineConcurrentReservationsAreBudgetSafe(t *testing.T) {
	engine := NewEngine()
	budget := defaultBudget()
	budget.Spec.BudgetEUR = resource.MustParse("0.018") // exactly five 3600-micro holds
	const attempts = 50
	var wg sync.WaitGroup
	responses := make(chan AdmitResponse, attempts)
	errs := make(chan error, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r, err := engine.Admit(admitRequest("concurrent-"+string(rune('A'+i)), testTenant, testWorkload), budget, defaultRouting(), defaultCandidates())
			responses <- r
			errs <- err
		}(i)
	}
	wg.Wait()
	close(responses)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent admit: %v", err)
		}
	}
	admitted := 0
	for r := range responses {
		if r.Decision == DecisionAdmit {
			admitted++
		}
	}
	if admitted != 5 {
		t.Fatalf("admitted=%d, want 5", admitted)
	}
	assertLiability(t, engine.Liability(testTenant), 0, 18_000, 0, 5)
}

func mustAdmit(t *testing.T, engine *Engine, requestID, tenant, workload string, candidates []Candidate) AdmitResponse {
	t.Helper()
	resp, err := engine.Admit(admitRequest(requestID, tenant, workload), defaultBudget(), defaultRouting(), candidates)
	if err != nil {
		t.Fatalf("admit: %v", err)
	}
	if resp.Decision != DecisionAdmit || resp.ReasonCode != ReasonHighestUtility {
		t.Fatalf("admit response = %+v", resp)
	}
	return resp
}

func admitRequest(requestID, tenant, workload string) AdmitRequest {
	return AdmitRequest{
		RequestID: requestID, Namespace: "finance", TenantID: tenant, WorkloadUID: workload,
		AuthenticatedTenantID: tenant, AuthenticatedWorkloadUID: workload,
		AuthenticatedNamespace: "finance",
		BudgetPolicyName:       "budget", RoutingPolicyName: "routing", InputTokens: 1_000, MaxOutputTokens: 2_000,
		InputTokensExact: true,
	}
}

func dispatchRequest(requestID, eventID, attemptID string, status DispatchStatus, tenant, workload string) DispatchRequest {
	return DispatchRequest{RequestID: requestID, EventID: eventID, TenantID: tenant, WorkloadUID: workload,
		ProviderAttemptID: attemptID, Status: status, AuthenticatedTenantID: tenant, AuthenticatedWorkloadUID: workload}
}

func settleRequest(requestID, eventID string, cost MoneyMicros, version int64, final bool, tenant, workload string) SettleRequest {
	return SettleRequest{RequestID: requestID, SettlementID: eventID, TenantID: tenant, WorkloadUID: workload,
		ActualCostMicros: cost, UsageVersion: version, Final: final,
		AuthenticatedTenantID: tenant, AuthenticatedWorkloadUID: workload}
}

func cancelRequest(requestID, eventID string, authoritative bool, tenant, workload string) CancelRequest {
	return CancelRequest{RequestID: requestID, EventID: eventID, TenantID: tenant, WorkloadUID: workload,
		AuthoritativeUnbilled: authoritative, AuthenticatedTenantID: tenant, AuthenticatedWorkloadUID: workload}
}

func defaultCandidates() []Candidate {
	return []Candidate{{ModelRef: "gpt-fr", InputPriceMicrosPerMillion: 400_000, OutputPriceMicrosPerMillion: 1_600_000,
		PricingVersion: "test-pricing-v1", SnapshotVersion: "test-snapshot-v1", ContextWindow: 128_000,
		QualityScore: 0.95, VerifiedOutputCap: true, Feasible: true}}
}

func defaultBudget() aiopsv1alpha1.AIBudgetPolicy {
	return aiopsv1alpha1.AIBudgetPolicy{ObjectMeta: metav1.ObjectMeta{Name: "budget"}, Spec: aiopsv1alpha1.AIBudgetPolicySpec{
		Target: aiopsv1alpha1.BudgetTarget{Namespace: "finance"}, Period: "monthly", BudgetEUR: resource.MustParse("100")},
		Status: aiopsv1alpha1.AIBudgetPolicyStatus{Conditions: []metav1.Condition{{Type: aiopsv1alpha1.ConditionReady, Status: metav1.ConditionTrue}}}}
}

func defaultRouting() aiopsv1alpha1.AIRoutingPolicy {
	now := metav1.Now()
	return aiopsv1alpha1.AIRoutingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "routing", Annotations: map[string]string{AnnotationReservationMethod: "strict_provider_cap"}}, Spec: aiopsv1alpha1.AIRoutingPolicySpec{
		Objective: "cost", Guardrails: aiopsv1alpha1.AIRoutingPolicyGuardrails{RequireSovereigntyCompliance: true}},
		Status: aiopsv1alpha1.AIRoutingPolicyStatus{LastEvaluatedAt: &now, Conditions: []metav1.Condition{{Type: aiopsv1alpha1.ConditionReady, Status: metav1.ConditionTrue}}}}
}

func assertLiability(t *testing.T, got LiabilityResponse, settled, held, available MoneyMicros, active int) {
	t.Helper()
	if got.SettledSpendMicros != settled || got.OutstandingLiabilityMicros != held || got.AvailableBudgetMicros != available || got.ActiveReservations != active {
		t.Fatalf("liability=%+v, want settled=%d held=%d available=%d active=%d", got, settled, held, available, active)
	}
}
