package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
)

var _ = Describe("AIAdmissionApproval API boundaries", Ordered, func() {
	var namespace string

	BeforeAll(func() {
		namespace = fmt.Sprintf("approval-schema-%d", time.Now().UnixNano())
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})).To(Succeed())
	})

	AfterAll(func() {
		Expect(k8sClient.Delete(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})).To(Succeed())
	})

	It("enforces immutable proposal spec in the API server", func() {
		request := aiopsv1alpha1.AIAdmissionApprovalRequest{RequestID: "request-1", Namespace: namespace, TenantID: "tenant-a", WorkloadUID: "workload-a",
			BudgetPolicy:      aiopsv1alpha1.AIWorkloadBindingResolvedReference{Name: "budget", UID: types.UID("budget-uid"), Generation: 1},
			RoutingPolicy:     aiopsv1alpha1.AIWorkloadBindingResolvedReference{Name: "routing", UID: types.UID("routing-uid"), Generation: 1},
			CandidateModelRef: "model", CandidateSnapshotVersion: strings.Repeat("b", 64), RouteSnapshot: aiopsv1alpha1.GOVARRouteSnapshot{Namespace: namespace, ModelName: "model", ModelUID: "model-uid", ModelGeneration: 1, ModelResourceVersion: "m1", ProviderName: "provider", ProviderUID: "provider-uid", ProviderGeneration: 1, ProviderResourceVersion: "p1", PricingVersion: "prices-v1", PricingComplianceHash: strings.Repeat("a", 64), RouteBindingName: "primary", ProviderDeployment: "provider-model", Cluster: "backend", Authority: "backend.example", PathMode: "openai-body", SnapshotHash: strings.Repeat("b", 64)},
			MaxOutputTokens: 100, ExpiresAt: metav1.NewTime(time.Now().Add(time.Hour))}
		request.RequestDigest = request.ComputeDigest()
		proposal := &aiopsv1alpha1.AIAdmissionApproval{ObjectMeta: metav1.ObjectMeta{Name: "proposal", Namespace: namespace}, Spec: aiopsv1alpha1.AIAdmissionApprovalSpec{Request: request}}
		Expect(k8sClient.Create(context.Background(), proposal)).To(Succeed())
		proposal.Spec.Request.TenantID = "tenant-b"
		proposal.Spec.Request.RequestDigest = proposal.Spec.Request.ComputeDigest()
		Expect(k8sClient.Update(context.Background(), proposal)).NotTo(Succeed())
	})

	It("rejects a human decision whose name does not equal its proposal reference", func() {
		decision := &aiopsv1alpha1.AIAdmissionApprovalDecision{ObjectMeta: metav1.ObjectMeta{Name: "different", Namespace: namespace}, Spec: aiopsv1alpha1.AIAdmissionApprovalDecisionSpec{
			ProposalName: "proposal", ProposalUID: types.UID("proposal-uid"), RequestDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Decision: aiopsv1alpha1.AIAdmissionApprovalDecisionApproved}}
		Expect(k8sClient.Create(context.Background(), decision)).NotTo(Succeed())
	})
})
