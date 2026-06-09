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
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
	"github.com/imperium/ai-sovereign-finops-operator/internal/budgetengine"
	"github.com/imperium/ai-sovereign-finops-operator/internal/costengine"
	"github.com/imperium/ai-sovereign-finops-operator/internal/metrics"
)

// budgetEnforcementFinalizer lets a deleted budget policy prune exactly its own
// enforcement series (same mechanism as the sovereignty policy finalizer).
const budgetEnforcementFinalizer = "aiops.imperium.io/budget-enforcement-metrics"

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

	// Deletion: prune this policy's enforcement series, then drop the finalizer.
	if !policy.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&policy, budgetEnforcementFinalizer) {
			enforcementMetrics.forget(policy.UID)
			controllerutil.RemoveFinalizer(&policy, budgetEnforcementFinalizer)
			if err := r.Update(ctx, &policy); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}
	if controllerutil.AddFinalizer(&policy, budgetEnforcementFinalizer) {
		if err := r.Update(ctx, &policy); err != nil {
			return ctrl.Result{}, err
		}
	}

	policy.Status.ObservedGeneration = policy.Generation

	cat, err := loadCatalog(ctx, r.Client, policy.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}
	collector, err := collectorFor(r.Client, policy.Namespace, firstGateway(ctx, r.Client, policy.Namespace))
	if err != nil {
		meta.SetStatusCondition(&policy.Status.Conditions,
			readyFalse(policy.Generation, aiopsv1alpha1.ReasonNoTelemetry, err.Error()))
		_ = r.Status().Update(ctx, &policy)
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}
	samples, err := collector.Collect(ctx, periodWindow(policy.Spec.Period))
	if err != nil {
		meta.SetStatusCondition(&policy.Status.Conditions,
			readyFalse(policy.Generation, aiopsv1alpha1.ReasonReconcileError, err.Error()))
		_ = r.Status().Update(ctx, &policy)
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}
	samples = filterByBudgetTarget(samples, policy.Spec.Target)
	breakdown := costengine.Compute(samples, cat.priceBook())

	// Forecast the full month from the observed spend (run-rate) and evaluate the
	// forecast against the monthly budget, so the phase answers "at this rate, will
	// we exceed the budget this month?".
	observed := breakdown.Total.CostTotal
	factor := monthlyFactor(policy.Spec.Period)
	projected := observed * factor

	result := budgetengine.Evaluate(
		projected,
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
	policy.Status.CurrentSpendEUR = moneyQuantityPtr(observed)
	policy.Status.ProjectedMonthlySpendEUR = moneyQuantityPtr(projected)
	msg := result.Message
	if factor != 1.0 {
		msg = fmt.Sprintf("projected %.2f / %.2f EUR monthly (%d%%) — %s [observed %.4f over %s window]",
			projected, policy.Spec.BudgetEUR.AsApproximateFloat64(), result.UsagePercent, result.Phase, observed, policy.Spec.Period)
	}
	meta.SetStatusCondition(&policy.Status.Conditions, readyTrue(policy.Generation, msg))

	metrics.BudgetUsagePercent.WithLabelValues(policy.Namespace, policy.Name).Set(float64(result.UsagePercent))

	// Enforcement signal: surface each tripped action on the shared EnforcementActions
	// vector so one dashboard/alert sees both sovereignty AND budget control decisions.
	// Budget actions are ADVISORY (this reconciler never blocks traffic — see doc above),
	// so actuated="false": honest about the fact that the operator recommends but does
	// not yet actuate budget actions at the gateway. mode carries the budget phase.
	// retire() prunes series this policy emitted before (e.g. when it drops back under a
	// threshold) without Reset()-ing the shared vector. FallbackModelRef, when set, is
	// surfaced as the recommended target via the BudgetThreshold event below.
	var enfSeries [][]string
	for _, action := range result.TriggeredActions {
		metrics.EnforcementActions.
			WithLabelValues(policy.Name, policy.Spec.Target.Namespace, policy.Spec.Target.Application, result.Phase, action, "false").
			Set(1)
		enfSeries = append(enfSeries,
			[]string{policy.Name, policy.Spec.Target.Namespace, policy.Spec.Target.Application, result.Phase, action, "false"})
	}
	enforcementMetrics.retire(policy.UID, enfSeries)

	if err := r.Status().Update(ctx, &policy); err != nil {
		return ctrl.Result{}, err
	}
	if r.Recorder != nil && len(result.TriggeredActions) > 0 {
		msg := fmt.Sprintf("%s — recommended actions: %s", result.Phase, strings.Join(result.TriggeredActions, ", "))
		if policy.Spec.FallbackModelRef != "" && result.Phase == budgetengine.PhaseExceeded {
			msg += fmt.Sprintf("; consider rerouting to cheaper fallback model %q", policy.Spec.FallbackModelRef)
		}
		r.Recorder.Eventf(&policy, "Warning", "BudgetThreshold", "%s", msg)
	}
	logger.V(1).Info("reconciled AIBudgetPolicy", "phase", result.Phase, "usagePercent", result.UsagePercent)
	return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AIBudgetPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&aiopsv1alpha1.AIBudgetPolicy{}).
		Complete(r)
}
