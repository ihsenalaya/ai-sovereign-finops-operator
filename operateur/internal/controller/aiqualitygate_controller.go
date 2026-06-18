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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
	"github.com/imperium/ai-sovereign-finops-operator/internal/collectors"
	"github.com/imperium/ai-sovereign-finops-operator/internal/metrics"
)

// AIQualityGateReconciler reconciles an AIQualityGate object.
type AIQualityGateReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

type goldenPrompt struct {
	ID       string `json:"id"`
	Prompt   string `json:"prompt"`
	Expected struct {
		RequiredKeywords []string `json:"requiredKeywords,omitempty"`
		MaxTokens        int32    `json:"maxTokens,omitempty"`
		MustBeJSON       bool     `json:"mustBeJSON,omitempty"`
	} `json:"expected,omitempty"`
}

type qualityEvidenceSample struct {
	ID                      string `json:"id"`
	Model                   string `json:"model,omitempty"`
	SchemaValid             *bool  `json:"schemaValid,omitempty"`
	UnexpectedRefusal       *bool  `json:"unexpectedRefusal,omitempty"`
	SensitiveDataLeak       *bool  `json:"sensitiveDataLeak,omitempty"`
	RequiredKeywordsPresent *bool  `json:"requiredKeywordsPresent,omitempty"`
}

type modelObservation struct {
	model                    string
	requests                 int64
	errors                   int64
	latencyWeightedMillis    float64
	latencyWeight            float64
	latencyTelemetryObserved bool
}

//+kubebuilder:rbac:groups=aiops.imperium.io,resources=aiqualitygates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=aiops.imperium.io,resources=aiqualitygates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=aiops.imperium.io,resources=aiqualitygates/finalizers,verbs=update
//+kubebuilder:rbac:groups=aiops.imperium.io,resources=aigateways,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch

func (r *AIQualityGateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var gate aiopsv1alpha1.AIQualityGate
	if err := r.Get(ctx, req.NamespacedName, &gate); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	gate.Status.ObservedGeneration = gate.Generation
	gate.Status.Phase = aiopsv1alpha1.AIQualityGatePending
	gate.Status.Verdict = "insufficient-data"
	gate.Status.CheckedSamples = 0
	gate.Status.FailedChecks = 0
	gate.Status.FailureMessages = nil
	gate.Status.SourceObservation = nil
	gate.Status.CandidateObservation = nil
	gate.Status.CanaryStatus = canaryStatus(gate.Spec.Canary)

	var failures []string
	var pending []string
	reason := aiopsv1alpha1.ReasonReconciled

	prompts, err := r.loadGoldenDataset(ctx, gate.Namespace, gate.Spec.GoldenDatasetRef)
	if err != nil {
		reason = reasonForReadErr(err)
		pending = append(pending, err.Error())
	} else {
		gate.Status.CheckedSamples = int32(len(prompts))
		failures = append(failures, validateGoldenDataset(prompts)...)
	}

	evidenceNeeded := qualityGateNeedsEvidence(gate.Spec.RequiredChecks)
	var evidence []qualityEvidenceSample
	if evidenceNeeded {
		if gate.Spec.EvidenceRef == nil {
			pending = append(pending, "response-level checks require spec.evidenceRef with deterministic golden-run results")
		} else {
			evidence, err = r.loadEvidence(ctx, gate.Namespace, *gate.Spec.EvidenceRef)
			if err != nil {
				reason = reasonForReadErr(err)
				pending = append(pending, err.Error())
			} else {
				failures = append(failures, evaluateEvidence(prompts, evidence, gate.Spec.CandidateModel, gate.Spec.RequiredChecks)...)
			}
		}
	}

	samples, collectorName, err := r.collectSamples(ctx, &gate)
	if err != nil {
		if reason == aiopsv1alpha1.ReasonReconciled {
			reason = aiopsv1alpha1.ReasonNoTelemetry
		}
		pending = append(pending, err.Error())
	} else {
		sourceObs, sourceOK := observeModel(samples, gate.Spec.Target, gate.Spec.SourceModel)
		candidateObs, candidateOK := observeModel(samples, gate.Spec.Target, gate.Spec.CandidateModel)
		if sourceOK {
			apiObs := sourceObs.toAPI()
			gate.Status.SourceObservation = &apiObs
		}
		if candidateOK {
			apiObs := candidateObs.toAPI()
			gate.Status.CandidateObservation = &apiObs
		}
		failures, pending = appendOperationalChecks(failures, pending, sourceObs, sourceOK, candidateObs, candidateOK, gate.Spec.RequiredChecks)
		logger.V(1).Info("evaluated AIQualityGate telemetry", "collector", collectorName, "sourceObserved", sourceOK, "candidateObserved", candidateOK)
	}

	gate.Status.FailureMessages = append([]string{}, failures...)
	gate.Status.FailureMessages = append(gate.Status.FailureMessages, pending...)
	gate.Status.FailedChecks = int32(len(failures))

	switch {
	case len(failures) > 0:
		gate.Status.Phase = aiopsv1alpha1.AIQualityGateFailed
		gate.Status.Verdict = "candidate-risk"
		reason = aiopsv1alpha1.ReasonValidationFailed
	case len(pending) > 0:
		gate.Status.Phase = aiopsv1alpha1.AIQualityGatePending
		gate.Status.Verdict = "insufficient-data"
	default:
		gate.Status.Phase = aiopsv1alpha1.AIQualityGatePassed
		gate.Status.Verdict = "candidate-safe"
	}

	message := qualityGateMessage(&gate)
	condStatus := conditionStatusForGate(gate.Status.Phase)
	meta.SetStatusCondition(&gate.Status.Conditions, readyCondition(gate.Generation, condStatus, reason, message))

	metrics.QualityGatePassed.WithLabelValues(gate.Namespace, gate.Name, gate.Spec.Target.Namespace, gate.Spec.Target.Application, gate.Spec.SourceModel, gate.Spec.CandidateModel).Set(qualityGatePassedValue(gate.Status.Phase))
	metrics.QualityGateFailedChecks.WithLabelValues(gate.Namespace, gate.Name, gate.Spec.Target.Namespace, gate.Spec.Target.Application).Set(float64(gate.Status.FailedChecks))

	if err := r.Status().Update(ctx, &gate); err != nil {
		return ctrl.Result{}, err
	}
	if r.Recorder != nil {
		eventType := corev1.EventTypeNormal
		if gate.Status.Phase != aiopsv1alpha1.AIQualityGatePassed {
			eventType = corev1.EventTypeWarning
		}
		r.Recorder.Eventf(&gate, eventType, "QualityGateEvaluated", "%s", message)
	}

	return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
}

func (r *AIQualityGateReconciler) collectSamples(ctx context.Context, gate *aiopsv1alpha1.AIQualityGate) ([]collectors.UsageSample, string, error) {
	var gw *aiopsv1alpha1.AIGateway
	if gate.Spec.GatewayRef != "" {
		var g aiopsv1alpha1.AIGateway
		if err := r.Get(ctx, types.NamespacedName{Namespace: gate.Namespace, Name: gate.Spec.GatewayRef}, &g); err != nil {
			return nil, "", fmt.Errorf("read gateway %s/%s: %w", gate.Namespace, gate.Spec.GatewayRef, err)
		}
		gw = &g
	}
	if gw == nil {
		gw = firstGateway(ctx, r.Client, gate.Namespace)
	}
	collector, err := collectorFor(r.Client, gate.Namespace, gw)
	if err != nil {
		return nil, "", err
	}
	period := gate.Spec.Period
	if period == "" {
		period = "monthly"
	}
	samples, err := collector.Collect(ctx, periodWindow(period))
	if err != nil {
		return nil, collector.Name(), fmt.Errorf("telemetry collection failed (%s): %w", collector.Name(), err)
	}
	return samples, collector.Name(), nil
}

func (r *AIQualityGateReconciler) loadGoldenDataset(ctx context.Context, defaultNamespace string, ref aiopsv1alpha1.ConfigMapDataReference) ([]goldenPrompt, error) {
	raw, err := r.readConfigMapData(ctx, defaultNamespace, ref, []string{"prompts.yaml", "prompts.yml", "prompts.json"})
	if err != nil {
		return nil, err
	}
	var prompts []goldenPrompt
	if err := yaml.Unmarshal([]byte(raw), &prompts); err == nil && len(prompts) > 0 {
		return prompts, nil
	}
	var wrapped struct {
		Prompts []goldenPrompt `json:"prompts"`
	}
	if err := yaml.Unmarshal([]byte(raw), &wrapped); err != nil {
		return nil, fmt.Errorf("parse golden dataset %s: %w", ref.Name, err)
	}
	return wrapped.Prompts, nil
}

func (r *AIQualityGateReconciler) loadEvidence(ctx context.Context, defaultNamespace string, ref aiopsv1alpha1.ConfigMapDataReference) ([]qualityEvidenceSample, error) {
	raw, err := r.readConfigMapData(ctx, defaultNamespace, ref, []string{"results.yaml", "results.yml", "evidence.yaml", "evidence.yml", "results.json"})
	if err != nil {
		return nil, err
	}
	var samples []qualityEvidenceSample
	if err := yaml.Unmarshal([]byte(raw), &samples); err == nil && len(samples) > 0 {
		return samples, nil
	}
	var wrapped struct {
		Samples []qualityEvidenceSample `json:"samples"`
	}
	if err := yaml.Unmarshal([]byte(raw), &wrapped); err != nil {
		return nil, fmt.Errorf("parse quality evidence %s: %w", ref.Name, err)
	}
	return wrapped.Samples, nil
}

func (r *AIQualityGateReconciler) readConfigMapData(ctx context.Context, defaultNamespace string, ref aiopsv1alpha1.ConfigMapDataReference, defaultKeys []string) (string, error) {
	namespace := ref.Namespace
	if namespace == "" {
		namespace = defaultNamespace
	}
	var cm corev1.ConfigMap
	if err := r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: ref.Name}, &cm); err != nil {
		return "", fmt.Errorf("read ConfigMap %s/%s: %w", namespace, ref.Name, err)
	}
	if ref.Key != "" {
		if v, ok := cm.Data[ref.Key]; ok {
			return v, nil
		}
		return "", fmt.Errorf("ConfigMap %s/%s missing key %q", namespace, ref.Name, ref.Key)
	}
	for _, key := range defaultKeys {
		if v, ok := cm.Data[key]; ok {
			return v, nil
		}
	}
	return "", fmt.Errorf("ConfigMap %s/%s missing one of keys %s", namespace, ref.Name, strings.Join(defaultKeys, ", "))
}

func validateGoldenDataset(prompts []goldenPrompt) []string {
	if len(prompts) == 0 {
		return []string{"golden dataset contains no prompts"}
	}
	seen := map[string]bool{}
	var failures []string
	for i, p := range prompts {
		if strings.TrimSpace(p.ID) == "" {
			failures = append(failures, fmt.Sprintf("golden prompt at index %d is missing id", i))
		}
		if strings.TrimSpace(p.Prompt) == "" {
			failures = append(failures, fmt.Sprintf("golden prompt %q is missing prompt text", p.ID))
		}
		if p.ID != "" {
			if seen[p.ID] {
				failures = append(failures, fmt.Sprintf("golden prompt id %q is duplicated", p.ID))
			}
			seen[p.ID] = true
		}
	}
	return failures
}

func qualityGateNeedsEvidence(c aiopsv1alpha1.AIQualityRequiredChecks) bool {
	return c.SchemaValid || c.NoUnexpectedRefusal || c.NoSensitiveDataLeak || len(c.RequiredKeywords) > 0
}

func evaluateEvidence(prompts []goldenPrompt, evidence []qualityEvidenceSample, candidateModel string, checks aiopsv1alpha1.AIQualityRequiredChecks) []string {
	byID := map[string]qualityEvidenceSample{}
	for _, ev := range evidence {
		if ev.ID == "" {
			continue
		}
		if ev.Model != "" && ev.Model != candidateModel {
			continue
		}
		byID[ev.ID] = ev
	}
	var failures []string
	for _, p := range prompts {
		ev, ok := byID[p.ID]
		if !ok {
			failures = append(failures, fmt.Sprintf("missing quality evidence for prompt %q and candidate model %q", p.ID, candidateModel))
			continue
		}
		if checks.SchemaValid {
			if ev.SchemaValid == nil || !*ev.SchemaValid {
				failures = append(failures, fmt.Sprintf("schema check failed for prompt %q", p.ID))
			}
		}
		if checks.NoUnexpectedRefusal {
			if ev.UnexpectedRefusal == nil || *ev.UnexpectedRefusal {
				failures = append(failures, fmt.Sprintf("unexpected refusal check failed for prompt %q", p.ID))
			}
		}
		if checks.NoSensitiveDataLeak {
			if ev.SensitiveDataLeak == nil || *ev.SensitiveDataLeak {
				failures = append(failures, fmt.Sprintf("sensitive-data leak check failed for prompt %q", p.ID))
			}
		}
		if len(checks.RequiredKeywords) > 0 {
			if ev.RequiredKeywordsPresent == nil || !*ev.RequiredKeywordsPresent {
				failures = append(failures, fmt.Sprintf("required-keywords check failed for prompt %q", p.ID))
			}
		}
	}
	return failures
}

func observeModel(samples []collectors.UsageSample, target aiopsv1alpha1.AIQualityGateTarget, model string) (modelObservation, bool) {
	obs := modelObservation{model: model}
	budgetTarget := budgetTargetForQualityGate(target)
	for _, s := range samples {
		if s.Model != model || !budgetTargetMatchesSample(s, budgetTarget) {
			continue
		}
		obs.requests += s.Requests
		obs.errors += s.Errors
		if s.LatencyMillis > 0 && s.Requests > 0 {
			obs.latencyTelemetryObserved = true
			obs.latencyWeightedMillis += s.LatencyMillis * float64(s.Requests)
			obs.latencyWeight += float64(s.Requests)
		}
	}
	return obs, obs.requests > 0
}

func budgetTargetForQualityGate(target aiopsv1alpha1.AIQualityGateTarget) aiopsv1alpha1.BudgetTarget {
	return aiopsv1alpha1.BudgetTarget{
		Namespace:   target.Namespace,
		Team:        target.Team,
		Application: target.Application,
	}
}

func (o modelObservation) errorRatePercent() float64 {
	if o.requests <= 0 {
		return 0
	}
	return float64(o.errors) * 100 / float64(o.requests)
}

func (o modelObservation) latencyMillis() float64 {
	if !o.latencyTelemetryObserved || o.latencyWeight <= 0 {
		return 0
	}
	return o.latencyWeightedMillis / o.latencyWeight
}

func (o modelObservation) toAPI() aiopsv1alpha1.AIQualityModelObservation {
	return aiopsv1alpha1.AIQualityModelObservation{
		Model:                     o.model,
		Requests:                  o.requests,
		Errors:                    o.errors,
		ErrorRatePercent:          decimalString(o.errorRatePercent(), 3),
		ObservedLatencyMillis:     decimalString(o.latencyMillis(), 3),
		LatencyTelemetryAvailable: o.latencyTelemetryObserved,
	}
}

func appendOperationalChecks(failures, pending []string, source modelObservation, sourceOK bool, candidate modelObservation, candidateOK bool, checks aiopsv1alpha1.AIQualityRequiredChecks) ([]string, []string) {
	if !candidateOK {
		pending = append(pending, fmt.Sprintf("no telemetry observed for candidate model %q in target application", candidate.model))
	}
	if checks.MaxErrorRatePercent > 0 && candidateOK {
		if candidate.errorRatePercent() > float64(checks.MaxErrorRatePercent) {
			failures = append(failures, fmt.Sprintf("candidate error rate %.3f%% exceeds threshold %d%%", candidate.errorRatePercent(), checks.MaxErrorRatePercent))
		}
	}
	if checks.MaxLatencyIncreasePercent > 0 {
		if !sourceOK || !candidateOK || !source.latencyTelemetryObserved || !candidate.latencyTelemetryObserved {
			pending = append(pending, "latency regression check requires observed latency for both source and candidate models")
		} else {
			src := source.latencyMillis()
			cand := candidate.latencyMillis()
			if src <= 0 {
				pending = append(pending, "latency regression check requires positive source latency")
			} else {
				increase := (cand - src) * 100 / src
				if increase > float64(checks.MaxLatencyIncreasePercent) {
					failures = append(failures, fmt.Sprintf("candidate latency increase %.3f%% exceeds threshold %d%%", increase, checks.MaxLatencyIncreasePercent))
				}
			}
		}
	}
	if checks.MaxRetryIncreasePercent > 0 {
		pending = append(pending, "retry regression check is configured but retry telemetry is not available yet")
	}
	return failures, pending
}

func canaryStatus(c aiopsv1alpha1.AIQualityCanarySpec) string {
	if !c.Enabled {
		return "Disabled"
	}
	return "Configured"
}

func conditionStatusForGate(phase aiopsv1alpha1.AIQualityGatePhase) metav1.ConditionStatus {
	if phase == aiopsv1alpha1.AIQualityGatePassed {
		return metav1.ConditionTrue
	}
	return metav1.ConditionFalse
}

func qualityGateMessage(gate *aiopsv1alpha1.AIQualityGate) string {
	target := gate.Spec.Target.Namespace + "/" + gate.Spec.Target.Application
	switch gate.Status.Phase {
	case aiopsv1alpha1.AIQualityGatePassed:
		return fmt.Sprintf("quality gate passed for %s: candidate %q can be considered safe", target, gate.Spec.CandidateModel)
	case aiopsv1alpha1.AIQualityGateFailed:
		return fmt.Sprintf("quality gate failed for %s: %d failed check(s)", target, gate.Status.FailedChecks)
	default:
		return fmt.Sprintf("quality gate pending for %s: %d missing/insufficient signal(s)", target, len(gate.Status.FailureMessages))
	}
}

func qualityGatePassedValue(phase aiopsv1alpha1.AIQualityGatePhase) float64 {
	if phase == aiopsv1alpha1.AIQualityGatePassed {
		return 1
	}
	return 0
}

func reasonForReadErr(err error) string {
	if apierrors.IsNotFound(err) {
		return aiopsv1alpha1.ReasonReferenceNotFound
	}
	return aiopsv1alpha1.ReasonReconcileError
}

func (r *AIQualityGateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&aiopsv1alpha1.AIQualityGate{}).
		Complete(r)
}
