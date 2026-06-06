/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
)

const testNamespace = "default"

func reqFor(name string) reconcile.Request {
	return reconcile.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: testNamespace}}
}

var _ = Describe("aiops reconcilers", func() {
	ctx := context.Background()

	Context("AIProvider", func() {
		It("registers and becomes Ready", func() {
			provider := &aiopsv1alpha1.AIProvider{
				ObjectMeta: metav1.ObjectMeta{Name: "prov-ready", Namespace: testNamespace},
				Spec: aiopsv1alpha1.AIProviderSpec{
					Type:          "azure-openai",
					DataResidency: "france",
					Managed:       true,
					Pricing: aiopsv1alpha1.ProviderPricing{
						Currency:                   "EUR",
						InputTokenPricePerMillion:  resource.MustParse("2.5"),
						OutputTokenPricePerMillion: resource.MustParse("10"),
					},
				},
			}
			Expect(k8sClient.Create(ctx, provider)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, provider) })

			r := &AIProviderReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := r.Reconcile(ctx, reqFor("prov-ready"))
			Expect(err).NotTo(HaveOccurred())

			got := &aiopsv1alpha1.AIProvider{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "prov-ready", Namespace: testNamespace}, got)).To(Succeed())
			Expect(got.Status.ObservedGeneration).To(Equal(got.Generation))
			Expect(meta.IsStatusConditionTrue(got.Status.Conditions, aiopsv1alpha1.ConditionReady)).To(BeTrue())
		})
	})

	Context("AIGateway", func() {
		It("resolves governed namespaces from the selector", func() {
			gw := &aiopsv1alpha1.AIGateway{
				ObjectMeta: metav1.ObjectMeta{Name: "gw-ready", Namespace: testNamespace},
				Spec: aiopsv1alpha1.AIGatewaySpec{
					Type:     "litellm",
					Endpoint: "http://litellm.default.svc.cluster.local:4000",
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"ai-finops/enabled": "true"},
					},
					Telemetry: aiopsv1alpha1.TelemetrySpec{Mode: aiopsv1alpha1.TelemetryModeFake},
				},
			}
			Expect(k8sClient.Create(ctx, gw)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, gw) })

			r := &AIGatewayReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := r.Reconcile(ctx, reqFor("gw-ready"))
			Expect(err).NotTo(HaveOccurred())

			got := &aiopsv1alpha1.AIGateway{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "gw-ready", Namespace: testNamespace}, got)).To(Succeed())
			Expect(meta.IsStatusConditionTrue(got.Status.Conditions, aiopsv1alpha1.ConditionReady)).To(BeTrue())
			// No namespace carries the label in the test env, so the set is empty (not nil-panic).
			Expect(got.Status.GovernedNamespaces).To(BeEmpty())
		})
	})

	Context("AIModel", func() {
		It("is NotReady when the provider is missing, Ready once it exists", func() {
			model := &aiopsv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{Name: "model-x", Namespace: testNamespace},
				Spec: aiopsv1alpha1.AIModelSpec{
					ProviderRef: "prov-for-model",
					ModelName:   "mistral-small",
					Type:        "llm",
				},
			}
			Expect(k8sClient.Create(ctx, model)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, model) })

			r := &AIModelReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := r.Reconcile(ctx, reqFor("model-x"))
			Expect(err).NotTo(HaveOccurred())

			got := &aiopsv1alpha1.AIModel{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "model-x", Namespace: testNamespace}, got)).To(Succeed())
			cond := meta.FindStatusCondition(got.Status.Conditions, aiopsv1alpha1.ConditionReady)
			Expect(cond).NotTo(BeNil())
			Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).To(Equal(aiopsv1alpha1.ReasonReferenceNotFound))

			By("creating the referenced provider and reconciling again")
			provider := &aiopsv1alpha1.AIProvider{
				ObjectMeta: metav1.ObjectMeta{Name: "prov-for-model", Namespace: testNamespace},
				Spec: aiopsv1alpha1.AIProviderSpec{
					Type: "self-hosted",
					Pricing: aiopsv1alpha1.ProviderPricing{
						Currency:                   "EUR",
						InputTokenPricePerMillion:  resource.MustParse("0"),
						OutputTokenPricePerMillion: resource.MustParse("0"),
					},
				},
			}
			Expect(k8sClient.Create(ctx, provider)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, provider) })

			_, err = r.Reconcile(ctx, reqFor("model-x"))
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "model-x", Namespace: testNamespace}, got)).To(Succeed())
			Expect(meta.IsStatusConditionTrue(got.Status.Conditions, aiopsv1alpha1.ConditionReady)).To(BeTrue())
			Expect(got.Status.ResolvedProvider).To(Equal("self-hosted"))
		})
	})

	Context("AIBudgetPolicy", func() {
		It("evaluates spend against budget and becomes Ready", func() {
			policy := &aiopsv1alpha1.AIBudgetPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "budget-x", Namespace: testNamespace},
				Spec: aiopsv1alpha1.AIBudgetPolicySpec{
					Target:                   aiopsv1alpha1.BudgetTarget{Namespace: "rh"},
					Period:                   "monthly",
					BudgetEUR:                resource.MustParse("500"),
					WarningThresholdPercent:  70,
					CriticalThresholdPercent: 90,
					HardLimitPercent:         100,
				},
			}
			Expect(k8sClient.Create(ctx, policy)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, policy) })

			r := &AIBudgetPolicyReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := r.Reconcile(ctx, reqFor("budget-x"))
			Expect(err).NotTo(HaveOccurred())

			got := &aiopsv1alpha1.AIBudgetPolicy{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "budget-x", Namespace: testNamespace}, got)).To(Succeed())
			// No catalog/spend in the test env → 0% of a 500 EUR budget.
			Expect(got.Status.Phase).To(Equal("WithinBudget"))
			Expect(got.Status.UsagePercent).To(Equal(int32(0)))
			Expect(meta.IsStatusConditionTrue(got.Status.Conditions, aiopsv1alpha1.ConditionReady)).To(BeTrue())
		})
	})

	Context("AISovereigntyPolicy", func() {
		It("registers in reportOnly mode and becomes Ready", func() {
			policy := &aiopsv1alpha1.AISovereigntyPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "sov-x", Namespace: testNamespace},
				Spec: aiopsv1alpha1.AISovereigntyPolicySpec{
					DataResidency:   aiopsv1alpha1.DataResidencyRule{AllowedZones: []string{"FR", "EU"}},
					EnforcementMode: aiopsv1alpha1.EnforcementReportOnly,
				},
			}
			Expect(k8sClient.Create(ctx, policy)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, policy) })

			r := &AISovereigntyPolicyReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := r.Reconcile(ctx, reqFor("sov-x"))
			Expect(err).NotTo(HaveOccurred())

			got := &aiopsv1alpha1.AISovereigntyPolicy{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "sov-x", Namespace: testNamespace}, got)).To(Succeed())
			Expect(meta.IsStatusConditionTrue(got.Status.Conditions, aiopsv1alpha1.ConditionReady)).To(BeTrue())
		})
	})

	Context("AIBreakEvenAnalysis", func() {
		It("computes a recommendation and becomes Ready", func() {
			analysis := &aiopsv1alpha1.AIBreakEvenAnalysis{
				ObjectMeta: metav1.ObjectMeta{Name: "be-x", Namespace: testNamespace},
				Spec: aiopsv1alpha1.AIBreakEvenAnalysisSpec{
					Target:          aiopsv1alpha1.BreakEvenTarget{Namespace: "rh"},
					CurrentModelRef: "mistral-small",
					AlternativeSelfHosted: aiopsv1alpha1.SelfHostedOption{
						ModelName:         "llama-3-70b",
						Runtime:           "vllm",
						GpuCount:          2,
						MonthlyGpuCostEUR: resource.MustParse("1800"),
					},
					AnalysisWindowDays: 30,
				},
			}
			Expect(k8sClient.Create(ctx, analysis)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, analysis) })

			r := &AIBreakEvenAnalysisReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := r.Reconcile(ctx, reqFor("be-x"))
			Expect(err).NotTo(HaveOccurred())

			got := &aiopsv1alpha1.AIBreakEvenAnalysis{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "be-x", Namespace: testNamespace}, got)).To(Succeed())
			// No catalog/usage in the test env → managed cost 0 vs self-hosted > 0,
			// so self-hosting yields no saving and the engine recommends keep-managed.
			Expect(got.Status.Recommendation).To(Equal(aiopsv1alpha1.RecommendationKeepManaged))
			Expect(got.Status.SelfHostedMonthlyCostEUR).NotTo(BeNil())
			Expect(meta.IsStatusConditionTrue(got.Status.Conditions, aiopsv1alpha1.ConditionReady)).To(BeTrue())
		})
	})

	Context("AIFinOpsReport", func() {
		It("stamps GeneratedAt and becomes Ready", func() {
			report := &aiopsv1alpha1.AIFinOpsReport{
				ObjectMeta: metav1.ObjectMeta{Name: "report-x", Namespace: testNamespace},
				Spec: aiopsv1alpha1.AIFinOpsReportSpec{
					Target: aiopsv1alpha1.ReportTarget{Namespace: "rh", Period: "monthly"},
				},
			}
			Expect(k8sClient.Create(ctx, report)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, report) })

			r := &AIFinOpsReportReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := r.Reconcile(ctx, reqFor("report-x"))
			Expect(err).NotTo(HaveOccurred())

			got := &aiopsv1alpha1.AIFinOpsReport{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "report-x", Namespace: testNamespace}, got)).To(Succeed())
			Expect(got.Status.GeneratedAt).NotTo(BeNil())
			Expect(meta.IsStatusConditionTrue(got.Status.Conditions, aiopsv1alpha1.ConditionReady)).To(BeTrue())
		})
	})
})
