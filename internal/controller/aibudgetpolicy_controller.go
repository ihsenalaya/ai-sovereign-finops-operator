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
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
	"github.com/imperium/ai-sovereign-finops-operator/internal/budgetengine"
	"github.com/imperium/ai-sovereign-finops-operator/internal/costengine"
	"github.com/imperium/ai-sovereign-finops-operator/internal/metrics"
)

// AIBudgetPolicyReconciler reconciles a AIBudgetPolicy object.
type AIBudgetPolicyReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=aiops.imperium.io,resources=aibudgetpolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=aiops.imperium.io,resources=aibudgetpolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=aiops.imperium.io,resources=aibudgetpolicies/finalizers,verbs=update
//+kubebuilder:rbac:groups=aiops.imperium.io,resources=aigateways;aimodels;aiproviders,verbs=get;list;watch

// Reconcile computes spend for the policy target, evaluates it against the budget
// and writes phase/usage to status. Recommendation-only: no traffic is blocked.
func (r *AIBudgetPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var policy aiopsv1alpha1.AIBudgetPolicy
	if err := r.Get(ctx, req.NamespacedName, &policy); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	policy.Status.ObservedGeneration = policy.Generation

	cat, err := loadCatalog(ctx, r.Client, policy.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}
	collector := collectorFor(firstGateway(ctx, r.Client, policy.Namespace))
	samples, err := collector.Collect(ctx, periodWindow(policy.Spec.Period))
	if err != nil {
		meta.SetStatusCondition(&policy.Status.Conditions,
			readyFalse(policy.Generation, aiopsv1alpha1.ReasonReconcileError, err.Error()))
		_ = r.Status().Update(ctx, &policy)
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}
	samples = filterByBudgetTarget(samples, policy.Spec.Target)
	breakdown := costengine.Compute(samples, cat.priceBook())

	result := budgetengine.Evaluate(
		breakdown.Total.CostTotal,
		policy.Spec.BudgetEUR.AsApproximateFloat64(),
		budgetengine.Thresholds{
			WarningPct:   policy.Spec.WarningThresholdPercent,
			CriticalPct:  policy.Spec.CriticalThresholdPercent,
			HardLimitPct: policy.Spec.HardLimitPercent,
		},
		budgetengine.Actions{
			OnWarning:   policy.Spec.Actions.OnWarning,
			OnCritical:  policy.Spec.Actions.OnCritical,
			OnHardLimit: policy.Spec.Actions.OnHardLimit,
		},
	)

	policy.Status.Phase = result.Phase
	policy.Status.UsagePercent = result.UsagePercent
	policy.Status.CurrentSpendEUR = moneyQuantityPtr(result.SpendEUR)
	meta.SetStatusCondition(&policy.Status.Conditions, readyTrue(policy.Generation, result.Message))

	metrics.BudgetUsagePercent.WithLabelValues(policy.Namespace, policy.Name).Set(float64(result.UsagePercent))

	if err := r.Status().Update(ctx, &policy); err != nil {
		return ctrl.Result{}, err
	}
	if r.Recorder != nil && len(result.TriggeredActions) > 0 {
		r.Recorder.Eventf(&policy, "Warning", "BudgetThreshold",
			"%s — recommended actions: %s", result.Phase, strings.Join(result.TriggeredActions, ", "))
	}
	logger.V(1).Info("reconciled AIBudgetPolicy", "phase", result.Phase, "usagePercent", result.UsagePercent)
	return ctrl.Result{RequeueAfter: 10 * time.Minute}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AIBudgetPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&aiopsv1alpha1.AIBudgetPolicy{}).
		Complete(r)
}
