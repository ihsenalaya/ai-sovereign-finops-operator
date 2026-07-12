package controller

import (
	"context"
	"testing"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
	"github.com/imperium/ai-sovereign-finops-operator/internal/govar"
)

func TestAIAdmissionApprovalControllerBoundaries(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		mutate     func(*aiopsv1alpha1.AIAdmissionApproval, *aiopsv1alpha1.AIAdmissionApprovalDecision, *[]client.Object)
		wantPhase  aiopsv1alpha1.AIAdmissionApprovalPhase
		reconciles int
	}{
		{name: "missing decision remains pending", mutate: func(_ *aiopsv1alpha1.AIAdmissionApproval, _ *aiopsv1alpha1.AIAdmissionApprovalDecision, objects *[]client.Object) {
			*objects = (*objects)[:len(*objects)-1]
		}, wantPhase: aiopsv1alpha1.AIAdmissionApprovalPhasePending, reconciles: 1},
		{name: "cross proposal UID is rejected", mutate: func(_ *aiopsv1alpha1.AIAdmissionApproval, decision *aiopsv1alpha1.AIAdmissionApprovalDecision, _ *[]client.Object) {
			decision.Spec.ProposalUID = "other-tenant-proposal"
		}, wantPhase: aiopsv1alpha1.AIAdmissionApprovalPhaseFailed, reconciles: 1},
		{name: "altered request digest is rejected", mutate: func(_ *aiopsv1alpha1.AIAdmissionApproval, decision *aiopsv1alpha1.AIAdmissionApprovalDecision, _ *[]client.Object) {
			decision.Spec.RequestDigest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}, wantPhase: aiopsv1alpha1.AIAdmissionApprovalPhaseFailed, reconciles: 1},
		{name: "stale policy generation is rejected", mutate: func(proposal *aiopsv1alpha1.AIAdmissionApproval, _ *aiopsv1alpha1.AIAdmissionApprovalDecision, _ *[]client.Object) {
			proposal.Spec.Request.BudgetPolicy.Generation++
			proposal.Spec.Request.RequestDigest = proposal.Spec.Request.ComputeDigest()
		}, wantPhase: aiopsv1alpha1.AIAdmissionApprovalPhaseFailed, reconciles: 1},
		{name: "expired proposal is rejected", mutate: func(proposal *aiopsv1alpha1.AIAdmissionApproval, _ *aiopsv1alpha1.AIAdmissionApprovalDecision, _ *[]client.Object) {
			proposal.Spec.Request.ExpiresAt = metav1.NewTime(now.Add(-time.Second))
			proposal.Spec.Request.RequestDigest = proposal.Spec.Request.ComputeDigest()
		}, wantPhase: aiopsv1alpha1.AIAdmissionApprovalPhaseExpired, reconciles: 1},
		{name: "approved proposal becomes consumed after lease", wantPhase: aiopsv1alpha1.AIAdmissionApprovalPhaseConsumed, reconciles: 2},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			proposal, decision, objects := approvalFixture(now)
			if test.mutate != nil {
				test.mutate(proposal, decision, &objects)
				decision.Spec.RequestDigest = decisionDigestUnlessDeliberatelyAltered(test.name, decision.Spec.RequestDigest, proposal.Spec.Request.RequestDigest)
			}
			c := fakeclient.NewClientBuilder().WithScheme(approvalTestScheme(t)).WithStatusSubresource(&aiopsv1alpha1.AIAdmissionApproval{}).WithObjects(objects...).Build()
			r := &AIAdmissionApprovalReconciler{Client: c, Now: func() time.Time { return now }}
			key := client.ObjectKeyFromObject(proposal)
			if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: key}); err != nil {
				t.Fatal(err)
			}
			if test.reconciles == 2 {
				var approved aiopsv1alpha1.AIAdmissionApproval
				if err := c.Get(context.Background(), key, &approved); err != nil {
					t.Fatal(err)
				}
				if approved.Status.Phase != aiopsv1alpha1.AIAdmissionApprovalPhaseApproved {
					t.Fatalf("first phase=%s", approved.Status.Phase)
				}
				holder := approved.Spec.Request.RequestDigest
				acquire := metav1.NewMicroTime(now.Add(time.Second))
				lease := &coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{Name: aiopsv1alpha1.AdmissionApprovalConsumptionName(approved.UID), Namespace: approved.Namespace,
					OwnerReferences: []metav1.OwnerReference{{APIVersion: aiopsv1alpha1.GroupVersion.String(), Kind: "AIAdmissionApproval", Name: approved.Name, UID: approved.UID}}},
					Spec: coordinationv1.LeaseSpec{HolderIdentity: &holder, AcquireTime: &acquire}}
				if err := c.Create(context.Background(), lease); err != nil {
					t.Fatal(err)
				}
				// Simulate controller recovery after the proposal has expired. A Lease
				// acquired while valid remains definitive consumption evidence.
				r.Now = func() time.Time { return now.Add(2 * time.Hour) }
				if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: key}); err != nil {
					t.Fatal(err)
				}
			}
			var got aiopsv1alpha1.AIAdmissionApproval
			if err := c.Get(context.Background(), key, &got); err != nil {
				t.Fatal(err)
			}
			if got.Status.Phase != test.wantPhase {
				t.Fatalf("phase=%s want=%s message=%s", got.Status.Phase, test.wantPhase, got.Status.Message)
			}
			if test.wantPhase == aiopsv1alpha1.AIAdmissionApprovalPhaseConsumed && (got.Status.ConsumedAt == nil || got.Status.ConsumedByRequestID != got.Spec.Request.RequestID) {
				t.Fatalf("consumption evidence=%+v", got.Status)
			}
		})
	}
}

func decisionDigestUnlessDeliberatelyAltered(name, current, proposal string) string {
	if name == "altered request digest is rejected" {
		return current
	}
	return proposal
}

func approvalTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	sch := runtime.NewScheme()
	if err := aiopsv1alpha1.AddToScheme(sch); err != nil {
		t.Fatal(err)
	}
	if err := coordinationv1.AddToScheme(sch); err != nil {
		t.Fatal(err)
	}
	return sch
}

func approvalFixture(now time.Time) (*aiopsv1alpha1.AIAdmissionApproval, *aiopsv1alpha1.AIAdmissionApprovalDecision, []client.Object) {
	ready := []metav1.Condition{{Type: aiopsv1alpha1.ConditionReady, Status: metav1.ConditionTrue}}
	budget := &aiopsv1alpha1.AIBudgetPolicy{ObjectMeta: metav1.ObjectMeta{Name: "budget", Namespace: "finance", UID: "budget-uid", Generation: 2}, Status: aiopsv1alpha1.AIBudgetPolicyStatus{ObservedGeneration: 2, Conditions: ready}}
	routing := &aiopsv1alpha1.AIRoutingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "routing", Namespace: "finance", UID: "routing-uid", Generation: 3}, Status: aiopsv1alpha1.AIRoutingPolicyStatus{ObservedGeneration: 3, Conditions: ready}}
	observed := metav1.NewTime(now)
	model := &aiopsv1alpha1.AIModel{ObjectMeta: metav1.ObjectMeta{Name: "model", Namespace: "finance", UID: "model-uid", Generation: 1, ResourceVersion: "11"}, Spec: aiopsv1alpha1.AIModelSpec{ProviderRef: "provider", ModelName: "model", ContextWindow: 4096, GOVAR: &aiopsv1alpha1.AIModelGOVARSpec{Routable: true, RouteBindingRef: "primary"}},
		Status: aiopsv1alpha1.AIModelStatus{ObservedGeneration: 1, LastQualityScore: 1, LastEvaluatedAt: &observed, Conditions: ready, GOVAR: &aiopsv1alpha1.AIModelGOVARStatus{VerifiedOutputCap: &aiopsv1alpha1.AIModelVerifiedOutputCapStatus{Verified: true, MaxOutputTokens: 1024, ObservedAt: observed, SourceVersion: "cap-v1"}}}}
	provider := &aiopsv1alpha1.AIProvider{ObjectMeta: metav1.ObjectMeta{Name: "provider", Namespace: "finance", UID: "provider-uid", Generation: 1, ResourceVersion: "12"},
		Spec:   aiopsv1alpha1.AIProviderSpec{Type: "openai", Pricing: aiopsv1alpha1.ProviderPricing{Currency: "EUR", InputTokenPricePerMillion: resource.MustParse("1"), OutputTokenPricePerMillion: resource.MustParse("2"), Version: "prices-v1", ObservedAt: &observed, Completeness: aiopsv1alpha1.ProviderPricingComplete}, GOVAR: &aiopsv1alpha1.AIProviderGOVARSpec{GatewayRoutes: []aiopsv1alpha1.AIProviderGatewayRouteBinding{{Name: "primary", ProviderDeployment: "provider-model", Cluster: "backend", Authority: "backend.example", PathMode: aiopsv1alpha1.GOVARRouteOpenAIBody}}}},
		Status: aiopsv1alpha1.AIProviderStatus{ObservedGeneration: 1, Conditions: ready}}
	candidate := govar.BuildCandidates(govar.RequestContext{Namespace: "finance"}, []aiopsv1alpha1.AIModel{*model}, map[string]aiopsv1alpha1.AIProvider{"provider": *provider})[0]
	request := aiopsv1alpha1.AIAdmissionApprovalRequest{RequestID: "request-1", Namespace: "finance", TenantID: "tenant-a", WorkloadUID: "workload-uid",
		BudgetPolicy:      aiopsv1alpha1.AIWorkloadBindingResolvedReference{Name: budget.Name, UID: budget.UID, Generation: budget.Generation},
		RoutingPolicy:     aiopsv1alpha1.AIWorkloadBindingResolvedReference{Name: routing.Name, UID: routing.UID, Generation: routing.Generation},
		CandidateModelRef: model.Name, CandidateSnapshotVersion: candidate.SnapshotVersion, RouteSnapshot: candidate.RouteSnapshot, InputTokens: 10, InputTokensExact: true, MaxOutputTokens: 100, ExpiresAt: metav1.NewTime(now.Add(time.Hour))}
	request.RequestDigest = request.ComputeDigest()
	proposal := &aiopsv1alpha1.AIAdmissionApproval{ObjectMeta: metav1.ObjectMeta{Name: "govar-approval-test", Namespace: "finance", UID: types.UID("proposal-uid"), Generation: 1}, Spec: aiopsv1alpha1.AIAdmissionApprovalSpec{Request: request}}
	decision := &aiopsv1alpha1.AIAdmissionApprovalDecision{ObjectMeta: metav1.ObjectMeta{Name: proposal.Name, Namespace: proposal.Namespace, UID: "decision-uid", Generation: 1}, Spec: aiopsv1alpha1.AIAdmissionApprovalDecisionSpec{
		ProposalName: proposal.Name, ProposalUID: proposal.UID, RequestDigest: request.RequestDigest, Decision: aiopsv1alpha1.AIAdmissionApprovalDecisionApproved}}
	objects := []client.Object{budget, routing, model, provider, proposal, decision}
	return proposal, decision, objects
}
