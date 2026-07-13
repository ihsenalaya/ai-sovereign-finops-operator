package govar

import (
	"sync"
	"testing"
	"time"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
	"github.com/imperium/ai-sovereign-finops-operator/internal/govarpricing"
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

	next := settleRequest("r1", "s2", 2_500, 2, false, testTenant, testWorkload)
	next.PredecessorEventID = "s1"
	res, code, err = engine.Settle(next)
	if err != nil || code != ReasonCorrection || res.ResidualHoldMicros != 1_100 {
		t.Fatalf("correction = (%+v,%s,%v)", res, code, err)
	}
	assertLiability(t, engine.Liability(testTenant), 2_500, 1_100, 99_996_400, 1)

	finalReq := settleRequest("r1", "s3", 2_500, 2, true, testTenant, testWorkload)
	finalReq.PredecessorEventID = "s2"
	res, code, err = engine.Settle(finalReq)
	if err != nil || code != ReasonFinalized || !res.Finalized || res.ResidualHoldMicros != 0 {
		t.Fatalf("finality = (%+v,%s,%v)", res, code, err)
	}
	assertLiability(t, engine.Liability(testTenant), 2_500, 0, 99_997_500, 0)

	_, code, err = engine.Settle(finalReq)
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

func TestMissingComponentUsageCannotFinalizeOrReleaseItsHold(t *testing.T) {
	engine := NewEngine()
	candidate := candidateForTest("gpt-fr", "test-pricing-v1")
	candidate.PricingSnapshot.InapplicableBases = removeTestBasis(candidate.PricingSnapshot.InapplicableBases, aiopsv1alpha1.ProviderBasisCachedInputTokens)
	maximum := int64(500)
	candidate.PricingSnapshot.Charges = append(candidate.PricingSnapshot.Charges, aiopsv1alpha1.AIProviderNormalizedChargeStatus{Basis: aiopsv1alpha1.ProviderBasisCachedInputTokens, Applicability: aiopsv1alpha1.ProviderChargeProviderResponse, PriceMicrosPerUnit: 200_000, UnitDenominator: 1_000_000, SettlementUsageField: aiopsv1alpha1.ProviderBasisCachedInputTokens, MaximumQuantity: &maximum, DisjointUsage: true})
	candidate.PricingSnapshot.SnapshotSHA256 = govarpricing.SnapshotDigest(candidate.PricingSnapshot)
	admit := mustAdmit(t, engine, "missing-detail", testTenant, testWorkload, []Candidate{candidate})
	if _, _, err := engine.Dispatch(dispatchRequest("missing-detail", "claim-detail", admit.ProviderAttemptID, DispatchClaimed, testTenant, testWorkload)); err != nil {
		t.Fatal(err)
	}
	req := settleRequest("missing-detail", "settle-detail", 2_000, 1, true, testTenant, testWorkload)
	res, code, err := engine.Settle(req)
	if err != nil || code != ReasonProvisionalSettlement || res.Finalized || len(res.MissingUsageBases) != 1 || res.MissingUsageBases[0] != aiopsv1alpha1.ProviderBasisCachedInputTokens || res.ResidualHoldMicros == 0 {
		t.Fatalf("incomplete usage released hold: res=%+v code=%s err=%v", res, code, err)
	}
}

func TestSettlementUsesFrozenPricingAndFlagsPerComponentBoundViolation(t *testing.T) {
	engine := NewEngine()
	candidate := candidateForTest("gpt-fr", "test-pricing-v1")
	admit := mustAdmit(t, engine, "frozen-price", testTenant, testWorkload, []Candidate{candidate})
	candidate.PricingSnapshot.Charges[0].PriceMicrosPerUnit = 9_000_000
	candidate.PricingSnapshot.SnapshotSHA256 = govarpricing.SnapshotDigest(candidate.PricingSnapshot)
	if _, _, err := engine.Dispatch(dispatchRequest("frozen-price", "claim-frozen", admit.ProviderAttemptID, DispatchClaimed, testTenant, testWorkload)); err != nil {
		t.Fatal(err)
	}
	req := SettleRequest{RequestID: "frozen-price", SettlementID: "over-component", TenantID: testTenant, WorkloadUID: testWorkload, ProviderAttemptID: admit.ProviderAttemptID,
		Usage: []govarpricing.UsageQuantity{{Basis: aiopsv1alpha1.ProviderBasisInputTokens, Quantity: 1_001}, {Basis: aiopsv1alpha1.ProviderBasisOutputTokens, Quantity: 0}}, UsageVersion: 1,
		AuthenticatedTenantID: testTenant, AuthenticatedWorkloadUID: testWorkload}
	res, code, err := engine.Settle(req)
	if err != nil || code != ReasonReservationExceeded || !res.ComponentBoundExceeded || res.ProvisionalCostMicros != 401 || len(res.ActualComponents) != 2 {
		t.Fatalf("bound violation=(%+v,%s,%v)", res, code, err)
	}
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

func TestLedgerResponsesDoNotFabricateTraceIdentity(t *testing.T) {
	engine := NewEngine()
	admitted, err := engine.Admit(admitRequest("trace-boundary", testTenant, testWorkload), defaultBudget(), defaultRouting(), defaultCandidates())
	if err != nil || admitted.Decision != DecisionAdmit {
		t.Fatalf("admit=(%+v,%v)", admitted, err)
	}
	if admitted.TraceID != "" {
		t.Fatalf("ledger request identifier was exposed as trace identity: %q", admitted.TraceID)
	}
	rejected := decisionResponse("not-a-trace", DecisionAbstain, ReasonNoCandidate, defaultBudget(), defaultRouting())
	if rejected.TraceID != "" {
		t.Fatalf("decision fabricated trace identity: %q", rejected.TraceID)
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
	if _, _, err := engine.Settle(settleRequest("versions", "s1", 2_000, 1, false, testTenant, testWorkload)); err != nil {
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
	correction := settleRequest("debt", "late-correction", 2_700, 2, true, testTenant, testWorkload)
	correction.PredecessorEventID = "final"
	res, code, err := engine.Settle(correction)
	if err != nil || code != ReasonCorrection || res.ProvisionalCostMicros != 2_700 {
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
	down := settleRequest("no-credit", "down", 1_000, 2, true, testTenant, testWorkload)
	down.PredecessorEventID = "final"
	if _, code, err := engine.Settle(down); err != nil || code != ReasonCorrection {
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
		candidateForTest("z-deployment", "v1"),
		candidateForTest("a-deployment", "v1"),
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
		ProviderAttemptID: attemptID, RouteSnapshotHash: testRouteSnapshot("gpt-fr", "test-pricing-v1").SnapshotHash,
		Status: status, AuthenticatedTenantID: tenant, AuthenticatedWorkloadUID: workload}
}

func settleRequest(requestID, eventID string, cost MoneyMicros, version int64, final bool, tenant, workload string) SettleRequest {
	input, output := usageForTestCost(cost)
	return SettleRequest{RequestID: requestID, SettlementID: eventID, TenantID: tenant, WorkloadUID: workload,
		ProviderAttemptID: requestID + ":attempt:1",
		Usage:             []govarpricing.UsageQuantity{{Basis: aiopsv1alpha1.ProviderBasisInputTokens, Quantity: input}, {Basis: aiopsv1alpha1.ProviderBasisOutputTokens, Quantity: output}}, UsageVersion: version, Final: final,
		AuthenticatedTenantID: tenant, AuthenticatedWorkloadUID: workload}
}

func usageForTestCost(cost MoneyMicros) (int64, int64) {
	for output := int64(0); output <= 2_000; output++ {
		outputCost, _ := costFromPriceMicros(0, 1_600_000, 0, output)
		for input := int64(0); input <= 1_000; input++ {
			inputCost, _ := costFromPriceMicros(400_000, 0, input, 0)
			if inputCost+outputCost == cost {
				return input, output
			}
		}
	}
	panic("test cost cannot be represented within frozen component bounds")
}

func cancelRequest(requestID, eventID string, authoritative bool, tenant, workload string) CancelRequest {
	return CancelRequest{RequestID: requestID, EventID: eventID, TenantID: tenant, WorkloadUID: workload,
		ProviderAttemptID:     requestID + ":attempt:1",
		AuthoritativeUnbilled: authoritative, AuthenticatedTenantID: tenant, AuthenticatedWorkloadUID: workload}
}

func defaultCandidates() []Candidate {
	return []Candidate{candidateForTest("gpt-fr", "test-pricing-v1")}
}

func candidateForTest(model, pricing string) Candidate {
	snapshot := testRouteSnapshot(model, pricing)
	pricingSnapshot := testPricingSnapshot(pricing)
	return Candidate{ModelRef: model, ProviderRef: "provider-test", ProviderType: "openai", InputPriceMicrosPerMillion: 400_000, OutputPriceMicrosPerMillion: 1_600_000,
		PricingVersion: pricing, PricingSnapshot: pricingSnapshot, SnapshotVersion: snapshot.SnapshotHash, RouteSnapshot: snapshot, ContextWindow: 128_000,
		QualityScore: 0.95, VerifiedOutputCap: true, VerifiedOutputCapTokens: 16_384, CapEvidenceDigest: testSHA("cap"), Feasible: true}
}

func testPricingSnapshot(version string) govarpricing.NormalizedPricingSnapshot {
	now := metav1.NewTime(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	future := metav1.NewTime(time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC))
	s := govarpricing.NormalizedPricingSnapshot{SpecGeneration: 1, Version: version, ObservedAt: now, ValidUntil: future,
		Currency: "EUR", Completeness: aiopsv1alpha1.ProviderPricingComplete, AdapterVersion: govarpricing.CurrentAdapterVersion,
		EvidenceMode: aiopsv1alpha1.ProviderEvidenceAdminAttested, EvidenceSHA256: testSHA("pricing"), SourceVersion: "synthetic-fixture-v1",
		Charges: []aiopsv1alpha1.AIProviderNormalizedChargeStatus{
			{Basis: aiopsv1alpha1.ProviderBasisInputTokens, Applicability: aiopsv1alpha1.ProviderChargeRequestDeclared, PriceMicrosPerUnit: 400_000, UnitDenominator: 1_000_000, SettlementUsageField: aiopsv1alpha1.ProviderBasisInputTokens, RequestBoundField: "input_tokens"},
			{Basis: aiopsv1alpha1.ProviderBasisOutputTokens, Applicability: aiopsv1alpha1.ProviderChargeRequestDeclared, PriceMicrosPerUnit: 1_600_000, UnitDenominator: 1_000_000, SettlementUsageField: aiopsv1alpha1.ProviderBasisOutputTokens, RequestBoundField: "max_output_tokens"},
		},
		InapplicableBases: []aiopsv1alpha1.ProviderBillableBasis{aiopsv1alpha1.ProviderBasisCachedInputTokens, aiopsv1alpha1.ProviderBasisReasoningTokens, aiopsv1alpha1.ProviderBasisRequest, aiopsv1alpha1.ProviderBasisToolCall, aiopsv1alpha1.ProviderBasisMediaUnit, aiopsv1alpha1.ProviderBasisBillableSecond, aiopsv1alpha1.ProviderBasisCancellation, aiopsv1alpha1.ProviderBasisRetryAttempt}}
	s.SnapshotSHA256 = govarpricing.SnapshotDigest(s)
	return s
}

func testSHA(seed string) string { return eventPayloadHash("test-sha", seed) }

func removeTestBasis(in []aiopsv1alpha1.ProviderBillableBasis, target aiopsv1alpha1.ProviderBillableBasis) []aiopsv1alpha1.ProviderBillableBasis {
	var out []aiopsv1alpha1.ProviderBillableBasis
	for _, basis := range in {
		if basis != target {
			out = append(out, basis)
		}
	}
	return out
}

func testRouteSnapshot(model, pricing string) RouteSnapshot {
	snapshot := RouteSnapshot{Namespace: "finance", ModelName: model, ModelUID: "model-uid-1", ModelGeneration: 1, ModelResourceVersion: "model-rv-1",
		ProviderName: "provider-test", ProviderUID: "provider-uid-1", ProviderGeneration: 1, ProviderResourceVersion: "provider-rv-1",
		PricingVersion: pricing, PricingComplianceHash: eventPayloadHash("pricing-compliance-test"), RouteBindingName: "primary",
		ProviderDeployment: "provider-model", Cluster: "backend", Authority: "backend.example", PathMode: "openai-body"}
	snapshot.SnapshotHash = RouteSnapshotHash(snapshot)
	return snapshot
}

func defaultBudget() aiopsv1alpha1.AIBudgetPolicy {
	return aiopsv1alpha1.AIBudgetPolicy{ObjectMeta: metav1.ObjectMeta{Name: "budget"}, Spec: aiopsv1alpha1.AIBudgetPolicySpec{
		Target: aiopsv1alpha1.BudgetTarget{Namespace: "finance"}, Period: "monthly", BudgetEUR: resource.MustParse("100")},
		Status: aiopsv1alpha1.AIBudgetPolicyStatus{Conditions: []metav1.Condition{{Type: aiopsv1alpha1.ConditionReady, Status: metav1.ConditionTrue}}}}
}

func defaultRouting() aiopsv1alpha1.AIRoutingPolicy {
	now := metav1.Now()
	return aiopsv1alpha1.AIRoutingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "routing"}, Spec: aiopsv1alpha1.AIRoutingPolicySpec{
		Objective: "cost", Guardrails: aiopsv1alpha1.AIRoutingPolicyGuardrails{RequireSovereigntyCompliance: true},
		GOVAR: &aiopsv1alpha1.GOVARRoutingPolicySpec{Reservation: aiopsv1alpha1.GOVARReservationPolicy{Method: aiopsv1alpha1.GOVARReservationStrictProviderCap},
			Drift: aiopsv1alpha1.GOVARDriftPolicy{Detector: "coverage-gap", ThresholdPPB: 10_000_000, Fallback: "strict_provider_cap", RevalidationMinimumSupport: 1}}},
		Status: aiopsv1alpha1.AIRoutingPolicyStatus{LastEvaluatedAt: &now, Conditions: []metav1.Condition{{Type: aiopsv1alpha1.ConditionReady, Status: metav1.ConditionTrue}}}}
}

func typedEstimateRouting(method aiopsv1alpha1.GOVARReservationMethod, value, margin int64) aiopsv1alpha1.AIRoutingPolicy {
	routing := defaultRouting()
	routing.Spec.GOVAR.Reservation.Method = method
	switch method {
	case aiopsv1alpha1.GOVARReservationMean:
		routing.Spec.GOVAR.Reservation.MeanOutputTokens = &value
	case aiopsv1alpha1.GOVARReservationFixedMargin:
		routing.Spec.GOVAR.Reservation.MeanOutputTokens = &value
		routing.Spec.GOVAR.Reservation.MarginOutputTokens = &margin
	case aiopsv1alpha1.GOVARReservationFixedQuantile:
		routing.Spec.GOVAR.Reservation.FixedQuantileOutputTokens = &value
	}
	return routing
}

func typedAdaptiveRouting(method aiopsv1alpha1.GOVARReservationMethod, upper int64, at time.Time, candidate Candidate) aiopsv1alpha1.AIRoutingPolicy {
	routing := defaultRouting()
	routing.Generation = 7
	routing.Status.ObservedGeneration = 7
	routing.Spec.GOVAR.Reservation.Method = method
	cal := &aiopsv1alpha1.GOVARCalibrationPolicy{ArtifactRef: "artifact-v1", ArtifactSHA256: eventPayloadHash("artifact"),
		CalibrationDataRef: "calibration", CalibrationInputSHA256: eventPayloadHash("calibration-input"), MonitoringDataRef: "monitoring",
		Version: "v1", FeatureSchemaVersion: "features-v1", PriceRegimeSHA256: candidate.PricingSnapshot.SnapshotSHA256,
		CapRegimeSHA256: candidate.CapEvidenceDigest, ProducerSoftwareSHA256: eventPayloadHash("software"), CoverageTargetPPB: 990_000_000,
		MinimumSupport: 100, MaxAgeSeconds: 3600}
	routing.Spec.GOVAR.Calibration = cal
	routing.Spec.GOVAR.Drift = aiopsv1alpha1.GOVARDriftPolicy{Detector: "coverage-gap", ThresholdPPB: 10_000_000,
		Fallback: "strict_provider_cap", RevalidationMinimumSupport: 100}
	routing.Status.GOVAR = &aiopsv1alpha1.GOVARRoutingPolicyStatus{
		Calibration: &aiopsv1alpha1.GOVARCalibrationStatus{ArtifactRef: cal.ArtifactRef, Version: cal.Version,
			ArtifactSHA256: cal.ArtifactSHA256, CalibrationInputSHA256: cal.CalibrationInputSHA256,
			FeatureSchemaVersion: cal.FeatureSchemaVersion, PriceRegimeSHA256: cal.PriceRegimeSHA256,
			CapRegimeSHA256: cal.CapRegimeSHA256, ProducerSoftwareSHA256: cal.ProducerSoftwareSHA256,
			CoverageTargetPPB: cal.CoverageTargetPPB, EmpiricalCoveragePPB: 995_000_000,
			Support: 100, AdaptiveOutputTokens: upper, Valid: true,
			CalibrationWindowStart: metav1.NewTime(at.Add(-30 * time.Minute)), CalibrationWindowEnd: metav1.NewTime(at.Add(-20 * time.Minute)), ObservedAt: metav1.NewTime(at)},
		Drift: &aiopsv1alpha1.GOVARDriftStatus{Detected: false, ConservativeMode: false, Detector: "coverage-gap",
			ThresholdPPB: 10_000_000, MonitoringInputSHA256: eventPayloadHash("monitoring-input"), Support: 100,
			EmpiricalCoveragePPB: 995_000_000, MonitoringWindowStart: metav1.NewTime(at.Add(-10 * time.Minute)),
			MonitoringWindowEnd: metav1.NewTime(at.Add(-time.Minute)), ObservedAt: metav1.NewTime(at.Add(-time.Minute))}}
	return routing
}

func bindTypedCohort(routing *aiopsv1alpha1.AIRoutingPolicy, c FrozenCohort) {
	routing.Spec.GOVAR.Cohort = &aiopsv1alpha1.GOVARCohortPolicy{RegistryRef: c.RegistryDigest, Size: c.Size,
		OpportunitySetHash: c.DataHash, WeightsHash: c.ConfigHash, FrozenAt: metav1.NewTime(c.FrozenAt)}
	routing.Spec.GOVAR.Risk = &aiopsv1alpha1.GOVARRiskPolicy{TenantRiskPPB: c.TenantRiskPPB, Allocation: "fixed-weights"}
}

func assertLiability(t *testing.T, got LiabilityResponse, settled, held, available MoneyMicros, active int) {
	t.Helper()
	if got.SettledSpendMicros != settled || got.OutstandingLiabilityMicros != held || got.AvailableBudgetMicros != available || got.ActiveReservations != active {
		t.Fatalf("liability=%+v, want settled=%d held=%d available=%d active=%d", got, settled, held, available, active)
	}
}
