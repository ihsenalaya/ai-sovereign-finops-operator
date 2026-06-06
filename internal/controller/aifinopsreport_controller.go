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
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
	"github.com/imperium/ai-sovereign-finops-operator/internal/collectors"
	"github.com/imperium/ai-sovereign-finops-operator/internal/costengine"
	"github.com/imperium/ai-sovereign-finops-operator/internal/metrics"
	"github.com/imperium/ai-sovereign-finops-operator/internal/reporting"
	"github.com/imperium/ai-sovereign-finops-operator/internal/sovereigntyengine"
)

// AIFinOpsReportReconciler reconciles a AIFinOpsReport object.
type AIFinOpsReportReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=aiops.imperium.io,resources=aifinopsreports,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=aiops.imperium.io,resources=aifinopsreports/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=aiops.imperium.io,resources=aifinopsreports/finalizers,verbs=update
//+kubebuilder:rbac:groups=aiops.imperium.io,resources=aigateways;aimodels;aiproviders,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch

// periodWindow maps a report period to a collection window.
func periodWindow(period string) time.Duration {
	switch period {
	case "daily":
		return 24 * time.Hour
	case "weekly":
		return 7 * 24 * time.Hour
	default:
		return 30 * 24 * time.Hour
	}
}

// Reconcile collects usage, computes the cost breakdown and writes the report
// results to .status. The MVP is read-only and re-runs periodically.
func (r *AIFinOpsReportReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var report aiopsv1alpha1.AIFinOpsReport
	if err := r.Get(ctx, req.NamespacedName, &report); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	report.Status.ObservedGeneration = report.Generation

	// Resolve the optional gateway to pick the telemetry collector.
	var gw *aiopsv1alpha1.AIGateway
	if report.Spec.GatewayRef != "" {
		var g aiopsv1alpha1.AIGateway
		if err := r.Get(ctx, types.NamespacedName{Namespace: report.Namespace, Name: report.Spec.GatewayRef}, &g); err != nil {
			if !apierrors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
		} else {
			gw = &g
		}
	}

	cat, err := loadCatalog(ctx, r.Client, report.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}

	collector := collectorFor(gw)
	samples, err := collector.Collect(ctx, periodWindow(report.Spec.Target.Period))
	if err != nil {
		meta.SetStatusCondition(&report.Status.Conditions,
			readyFalse(report.Generation, aiopsv1alpha1.ReasonReconcileError,
				fmt.Sprintf("telemetry collection failed (%s): %v", collector.Name(), err)))
		_ = r.Status().Update(ctx, &report)
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}
	samples = filterByNamespace(samples, report.Spec.Target.Namespace)

	breakdown := costengine.Compute(samples, cat.priceBook())
	r.applyCostToStatus(&report, breakdown)
	r.applySovereigntyToStatus(ctx, &report, cat, samples)

	if err := r.writeReportConfigMap(ctx, &report, breakdown, collector.Name()); err != nil {
		logger.Error(err, "failed to write report ConfigMap")
	}

	now := metav1.Now()
	report.Status.GeneratedAt = &now
	meta.SetStatusCondition(&report.Status.Conditions, readyCondition(
		report.Generation, metav1.ConditionTrue, aiopsv1alpha1.ReasonReportGenerated,
		fmt.Sprintf("Report generated from %d usage sample(s) via %s collector", len(samples), collector.Name())))

	if err := r.Status().Update(ctx, &report); err != nil {
		return ctrl.Result{}, err
	}
	if r.Recorder != nil {
		r.Recorder.Eventf(&report, "Normal", "ReportGenerated",
			"Total cost %.2f %s over %d requests", breakdown.Total.CostTotal, breakdown.Currency, breakdown.Total.Requests)
	}
	logger.V(1).Info("generated AIFinOpsReport",
		"totalCost", breakdown.Total.CostTotal, "samples", len(samples))
	// Refresh periodically so the report tracks ongoing usage.
	return ctrl.Result{RequeueAfter: 10 * time.Minute}, nil
}

// applyCostToStatus writes the cost breakdown into the report status.
func (r *AIFinOpsReportReconciler) applyCostToStatus(report *aiopsv1alpha1.AIFinOpsReport, b costengine.Breakdown) {
	report.Status.TotalCostEUR = moneyQuantityPtr(b.Total.CostTotal)
	report.Status.TotalInputTokens = b.Total.InputTokens
	report.Status.TotalOutputTokens = b.Total.OutputTokens

	nsLabel := report.Spec.Target.Namespace
	if nsLabel == "" {
		nsLabel = report.Namespace
	}
	metrics.RequestsTotal.WithLabelValues(nsLabel).Set(float64(b.Total.Requests))
	metrics.InputTokensTotal.WithLabelValues(nsLabel).Set(float64(b.Total.InputTokens))
	metrics.OutputTokensTotal.WithLabelValues(nsLabel).Set(float64(b.Total.OutputTokens))
	metrics.CostEURTotal.WithLabelValues(nsLabel).Set(b.Total.CostTotal)

	top := costengine.TopByCost(b.ByModel, 5)
	report.Status.TopModels = make([]aiopsv1alpha1.ModelCost, 0, len(top))
	for _, li := range top {
		report.Status.TopModels = append(report.Status.TopModels, aiopsv1alpha1.ModelCost{
			Name:    li.Key,
			CostEUR: moneyQuantity(li.CostTotal),
		})
	}

	// Basic cost-driven recommendation; richer engines populate this in later sprints.
	report.Status.Recommendations = nil
	if len(top) > 0 && top[0].CostTotal > 0 {
		report.Status.Recommendations = append(report.Status.Recommendations, aiopsv1alpha1.Recommendation{
			Type: "cost-optimization",
			Message: fmt.Sprintf("Model %q drives %.2f %s; review routing to a cheaper tier for low-stakes traffic.",
				top[0].Key, top[0].CostTotal, b.Currency),
		})
	}
	for _, m := range b.UnpricedModels {
		report.Status.Recommendations = append(report.Status.Recommendations, aiopsv1alpha1.Recommendation{
			Type:    "data-quality",
			Message: fmt.Sprintf("No pricing found for model %q; create an AIModel/AIProvider to attribute its cost.", m),
		})
	}

	byType := map[string]float64{}
	for _, rec := range report.Status.Recommendations {
		byType[rec.Type]++
	}
	for tp, n := range byType {
		metrics.Recommendations.WithLabelValues(tp).Set(n)
	}
}

// applySovereigntyToStatus runs the sovereignty engine (if a policy exists in the
// namespace) and records findings on the report.
func (r *AIFinOpsReportReconciler) applySovereigntyToStatus(ctx context.Context, report *aiopsv1alpha1.AIFinOpsReport, cat catalog, samples []collectors.UsageSample) {
	policy := firstSovereigntyPolicy(ctx, r.Client, report.Namespace)
	if policy == nil {
		return
	}
	findings := sovereigntyengine.Evaluate(policyToEngine(policy.Spec), cat.providerInfos(samples))
	report.Status.SovereigntyFindings = make([]aiopsv1alpha1.SovereigntyFinding, 0, len(findings))
	for _, f := range findings {
		report.Status.SovereigntyFindings = append(report.Status.SovereigntyFindings, aiopsv1alpha1.SovereigntyFinding{
			Severity: aiopsv1alpha1.Severity(f.Severity),
			Message:  f.Message,
		})
	}
}

// writeReportConfigMap renders the report to Markdown + JSON and upserts a
// ConfigMap named "<report>-report" owned by the report (GC'd with it).
func (r *AIFinOpsReportReconciler) writeReportConfigMap(ctx context.Context, report *aiopsv1alpha1.AIFinOpsReport, b costengine.Breakdown, collectorName string) error {
	generatedAt := time.Now()
	if report.Status.GeneratedAt != nil {
		generatedAt = report.Status.GeneratedAt.Time
	}
	data := reporting.Data{
		Name:        report.Name,
		Namespace:   report.Spec.Target.Namespace,
		Period:      report.Spec.Target.Period,
		GeneratedAt: generatedAt,
		Collector:   collectorName,
		Breakdown:   b,
		Sovereignty: report.Status.SovereigntyFindings,
		Recommends:  report.Status.Recommendations,
	}
	md := reporting.RenderMarkdown(data)
	jsonBytes, err := reporting.RenderJSON(data)
	if err != nil {
		return err
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      report.Name + "-report",
			Namespace: report.Namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, cm, func() error {
		if cm.Labels == nil {
			cm.Labels = map[string]string{}
		}
		cm.Labels["app.kubernetes.io/managed-by"] = "ai-sovereign-finops-operator"
		cm.Labels["aiops.imperium.io/report"] = report.Name
		cm.Data = map[string]string{
			"report.md":   md,
			"report.json": string(jsonBytes),
		}
		return controllerutil.SetControllerReference(report, cm, r.Scheme)
	})
	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *AIFinOpsReportReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&aiopsv1alpha1.AIFinOpsReport{}).
		Complete(r)
}
