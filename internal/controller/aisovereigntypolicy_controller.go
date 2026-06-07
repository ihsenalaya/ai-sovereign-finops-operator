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

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
	"github.com/imperium/ai-sovereign-finops-operator/internal/metrics"
	"github.com/imperium/ai-sovereign-finops-operator/internal/sovereigntyengine"
)

// AISovereigntyPolicyReconciler reconciles a AISovereigntyPolicy object.
type AISovereigntyPolicyReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=aiops.imperium.io,resources=aisovereigntypolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=aiops.imperium.io,resources=aisovereigntypolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=aiops.imperium.io,resources=aisovereigntypolicies/finalizers,verbs=update
//+kubebuilder:rbac:groups=aiops.imperium.io,resources=aigateways;aimodels;aiproviders,verbs=get;list;watch

// policyToEngine maps the CRD spec to the pure engine policy.
func policyToEngine(p aiopsv1alpha1.AISovereigntyPolicySpec) sovereigntyengine.Policy {
	return sovereigntyengine.Policy{
		AllowedZones:             p.DataResidency.AllowedZones,
		ForbiddenZones:           p.DataResidency.ForbiddenZones,
		ExternalProvidersAllowed: p.SensitiveData.ExternalProvidersAllowed,
		RequireAnonymization:     p.SensitiveData.RequireAnonymization,
	}
}

// Reconcile evaluates providers in use against the policy and records findings.
// reportOnly: nothing is ever blocked.
func (r *AISovereigntyPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var policy aiopsv1alpha1.AISovereigntyPolicy
	if err := r.Get(ctx, req.NamespacedName, &policy); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	policy.Status.ObservedGeneration = policy.Generation

	cat, err := loadCatalog(ctx, r.Client, policy.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}
	collector := collectorFor(r.Client, policy.Namespace, firstGateway(ctx, r.Client, policy.Namespace))
	samples, err := collector.Collect(ctx, 30*24*time.Hour)
	if err != nil {
		meta.SetStatusCondition(&policy.Status.Conditions,
			readyFalse(policy.Generation, aiopsv1alpha1.ReasonReconcileError, err.Error()))
		_ = r.Status().Update(ctx, &policy)
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	findings := sovereigntyengine.EvaluateFlows(policyToEngine(policy.Spec), cat.flows(samples))
	counts := sovereigntyengine.CountBySeverity(findings)

	policy.Status.FindingsCount = int32(len(findings))
	meta.SetStatusCondition(&policy.Status.Conditions, readyTrue(policy.Generation,
		fmt.Sprintf("Evaluated (%s): %d finding(s) — %d critical, %d warning",
			policy.Spec.EnforcementMode, len(findings), counts[sovereigntyengine.SeverityCritical], counts[sovereigntyengine.SeverityWarning])))

	// Flow-aware metric: findings per namespace/application/severity.
	type appSev struct{ ns, app, sev string }
	perApp := map[appSev]int{}
	for _, f := range findings {
		perApp[appSev{f.Namespace, f.Application, f.Severity}]++
	}
	for k, n := range perApp {
		metrics.SovereigntyFindings.WithLabelValues(k.ns, k.app, policy.Name, k.sev).Set(float64(n))
	}

	if err := r.Status().Update(ctx, &policy); err != nil {
		return ctrl.Result{}, err
	}
	if r.Recorder != nil && counts[sovereigntyengine.SeverityCritical] > 0 {
		r.Recorder.Eventf(&policy, "Warning", "SovereigntyViolation",
			"%d critical sovereignty finding(s) detected", counts[sovereigntyengine.SeverityCritical])
	}
	logger.V(1).Info("reconciled AISovereigntyPolicy", "findings", len(findings))
	return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AISovereigntyPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&aiopsv1alpha1.AISovereigntyPolicy{}).
		Complete(r)
}
