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

// Continuous Synthetic Quality Probes.
//
// This is an ADDITIVE layer on top of the existing one-shot evaluation flow.
// When spec.continuousProbes.enabled is true, the reconciler periodically replays
// a SUBSET of the golden dataset against the source and candidate models through
// the real gateway, using ephemeral Jobs scheduled by RequeueAfter (NO Kubernetes
// CronJob). Each run writes its evidence to its own ConfigMap (rotated by
// retainRuns); the scheduler aggregates recent runs over a sliding window and
// publishes a synthetic verdict under status.continuousProbes.
//
// These are SYNTHETIC tests against known references — never scoring of real user
// traffic. Reliability and Latency continue to come from real telemetry.

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
	"github.com/imperium/ai-sovereign-finops-operator/internal/metrics"
	"github.com/imperium/ai-sovereign-finops-operator/internal/qualityengine"
)

const (
	probeTypeLabel     = "aiops.imperium.io/probe-type"
	probeTypeSynthetic = "synthetic-continuous"
	probeRunIDLabel    = "aiops.imperium.io/run-id"

	defaultProbeInterval      = time.Hour
	defaultProbeSlidingWindow = 24 * time.Hour
	defaultProbeRetainRuns    = 20
	minProbeRequeue           = 15 * time.Second
)

// reconcileContinuousProbes runs the synthetic probe scheduler. It returns a
// requeue hint (0 means "no override", keep the default cadence). It never breaks
// the gate's existing behaviour: when continuousProbes is absent or disabled it is
// a no-op beyond clearing status.continuousProbes.enabled.
func (r *AIQualityGateReconciler) reconcileContinuousProbes(
	ctx context.Context,
	gate *aiopsv1alpha1.AIQualityGate,
	prompts []goldenPrompt,
	sourceObs, candidateObs modelObservation,
) (time.Duration, error) {
	logger := log.FromContext(ctx)
	cp := gate.Spec.ContinuousProbes

	if cp == nil || !cp.Enabled {
		if gate.Status.ContinuousProbes != nil {
			gate.Status.ContinuousProbes.Enabled = false
			meta.RemoveStatusCondition(&gate.Status.Conditions, "ContinuousProbesReady")
		}
		return 0, nil
	}

	st := gate.Status.ContinuousProbes
	if st == nil {
		st = &aiopsv1alpha1.AIQualityContinuousProbesStatus{}
	}
	st.Enabled = true
	st.ObservedGeneration = gate.Generation
	interval := parseProbeDuration(cp.Interval, defaultProbeInterval)
	window := parseProbeDuration(cp.SlidingWindow, defaultProbeSlidingWindow)
	st.Window = window.String()
	now := time.Now()
	if st.NextRunAt == nil {
		t := metav1.NewTime(now)
		st.NextRunAt = &t
	}

	active, newestFinished, err := r.listProbeJobs(ctx, gate)
	if err != nil {
		return 0, err
	}

	// A probe is running: do not schedule another (maxConcurrency is 1).
	if active != nil {
		st.RunInProgress = true
		gate.Status.ContinuousProbes = st
		return minProbeRequeue, nil
	}
	st.RunInProgress = false

	// Process a finished run we have not recorded yet.
	newRunCompleted := false
	if newestFinished != nil && newestFinished.Name != st.LastRunName {
		r.processFinishedProbe(ctx, gate, st, newestFinished, now, interval, len(prompts), cp)
		newRunCompleted = true
		if err := r.rotateProbeEvidence(ctx, gate, defaultedRetainRuns(cp.RetainRuns)); err != nil {
			logger.V(1).Info("probe evidence rotation failed", "error", err.Error())
		}
	}

	// Aggregate recent runs over the sliding window into a synthetic verdict.
	r.aggregateProbeWindow(ctx, gate, st, prompts, sourceObs, candidateObs, window)

	// Freshness + anti-flapping (counters advance only on a newly completed run).
	staleAfter := probeStaleAfter(cp.DecisionPolicy)
	prevFresh := st.EvidenceFresh
	st.EvidenceFresh = probeEvidenceFresh(st, now, staleAfter)
	if prevFresh && !st.EvidenceFresh {
		r.event(gate, corev1.EventTypeWarning, "ContinuousProbeEvidenceStale",
			"continuous probe evidence is now stale (older than %s)", staleAfter.String())
	}
	metrics.QualityProbeEvidenceFresh.WithLabelValues(gate.Namespace, gate.Name).Set(boolToMetric(st.EvidenceFresh))
	if newRunCompleted {
		r.advanceAntiFlapping(gate, st, cp.DecisionPolicy)
	}

	// Schedule the next run if due and the dataset is available.
	requeue := minProbeRequeue
	if len(prompts) == 0 {
		// Nothing to replay yet; check back soon.
		gate.Status.ContinuousProbes = st
		r.setProbeCondition(gate, st)
		return minProbeRequeue, nil
	}
	if !now.Before(st.NextRunAt.Time) {
		if err := r.scheduleProbeRun(ctx, gate, st, prompts, cp, now); err != nil {
			return 0, err
		}
		st.RunInProgress = true
		requeue = minProbeRequeue
	} else {
		requeue = time.Until(st.NextRunAt.Time)
		if requeue < minProbeRequeue {
			requeue = minProbeRequeue
		}
	}

	gate.Status.ContinuousProbes = st
	r.setProbeCondition(gate, st)
	return requeue, nil
}

// listProbeJobs returns the active probe Job (if any) and the newest finished one.
func (r *AIQualityGateReconciler) listProbeJobs(ctx context.Context, gate *aiopsv1alpha1.AIQualityGate) (*batchv1.Job, *batchv1.Job, error) {
	var jobs batchv1.JobList
	if err := r.List(ctx, &jobs, client.InNamespace(gate.Namespace), client.MatchingLabels{
		qualityGateLabel: gate.Name,
		probeTypeLabel:   probeTypeSynthetic,
	}); err != nil {
		return nil, nil, fmt.Errorf("list probe jobs for %s/%s: %w", gate.Namespace, gate.Name, err)
	}
	var active *batchv1.Job
	var newestFinished *batchv1.Job
	for i := range jobs.Items {
		job := &jobs.Items[i]
		finished := job.Status.Succeeded > 0 || job.Status.Failed > 0
		if !finished {
			active = job
			continue
		}
		if newestFinished == nil || job.CreationTimestamp.After(newestFinished.CreationTimestamp.Time) {
			newestFinished = job
		}
	}
	return active, newestFinished, nil
}

// processFinishedProbe records the outcome of one finished probe Job and, on
// success, persists its evidence to a per-run ConfigMap.
func (r *AIQualityGateReconciler) processFinishedProbe(
	ctx context.Context,
	gate *aiopsv1alpha1.AIQualityGate,
	st *aiopsv1alpha1.AIQualityContinuousProbesStatus,
	job *batchv1.Job,
	now time.Time,
	interval time.Duration,
	promptCount int,
	cp *aiopsv1alpha1.AIQualityContinuousProbesSpec,
) {
	logger := log.FromContext(ctx)
	runID := job.Labels[probeRunIDLabel]
	labels := metricsLabels(gate)

	finishedAt := now
	st.LastRunName = job.Name
	st.LastRunAt = &metav1.Time{Time: finishedAt}
	st.NextRunAt = &metav1.Time{Time: finishedAt.Add(interval)}
	st.RunInProgress = false

	// Advance the round-robin cursor for the next run.
	if promptCount > 0 {
		size := probeSampleSize(cp.SampleSize, promptCount)
		st.LastPromptIndex = int32((int(st.LastPromptIndex) + size) % promptCount)
	}

	if job.Status.Failed > 0 {
		st.LastRunStatus = "Failed"
		r.event(gate, corev1.EventTypeWarning, "ContinuousProbeFailed", "synthetic quality probe %q failed", job.Name)
		metrics.QualityProbeFailuresTotal.WithLabelValues(labels...).Inc()
		return
	}

	raw, err := r.evaluationJobEvidence(ctx, gate.Namespace, job.Name)
	if err != nil {
		st.LastRunStatus = "Failed"
		metrics.QualityProbeFailuresTotal.WithLabelValues(labels...).Inc()
		logger.V(1).Info("probe evidence unreadable", "job", job.Name, "error", err.Error())
		return
	}
	if err := validateEvaluationEvidence(raw); err != nil {
		st.LastRunStatus = "Failed"
		metrics.QualityProbeFailuresTotal.WithLabelValues(labels...).Inc()
		logger.V(1).Info("probe evidence invalid", "job", job.Name, "error", err.Error())
		return
	}
	if err := r.writeProbeEvidence(ctx, gate, runID, raw, job); err != nil {
		logger.V(1).Info("write probe evidence failed", "job", job.Name, "error", err.Error())
	}

	st.LastRunStatus = "Succeeded"
	r.event(gate, corev1.EventTypeNormal, "ContinuousProbeSucceeded", "synthetic quality probe %q completed", job.Name)
	metrics.QualityProbeRunsTotal.WithLabelValues(labels...).Inc()
	metrics.QualityProbeLastSuccessTimestamp.WithLabelValues(gate.Namespace, gate.Name).Set(float64(finishedAt.Unix()))
	if !job.CreationTimestamp.IsZero() {
		metrics.QualityProbeDurationSeconds.WithLabelValues(gate.Namespace, gate.Name).Set(finishedAt.Sub(job.CreationTimestamp.Time).Seconds())
	}
}

// writeProbeEvidence persists one run's evidence to its own ConfigMap, owned by the
// gate and labelled so the scheduler can aggregate and rotate it.
func (r *AIQualityGateReconciler) writeProbeEvidence(ctx context.Context, gate *aiopsv1alpha1.AIQualityGate, runID string, raw []byte, job *batchv1.Job) error {
	name := probeEvidenceName(gate, runID)
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: gate.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":   "ai-sovereign-finops-operator",
				qualityGateLabel:           gate.Name,
				qualityEvidenceSourceLabel: qualityEvidenceSourceJob,
				qualityEvaluatorLabel:      "true",
				probeTypeLabel:             probeTypeSynthetic,
				probeRunIDLabel:            runID,
			},
			Annotations: map[string]string{
				"aiops.imperium.io/run-timestamp":   time.Now().UTC().Format(time.RFC3339),
				"aiops.imperium.io/source-model":    gate.Spec.SourceModel,
				"aiops.imperium.io/candidate-model": gate.Spec.CandidateModel,
				"aiops.imperium.io/prompt-ids":      job.Annotations["aiops.imperium.io/prompt-ids"],
			},
		},
		Data: map[string]string{"results.yaml": string(raw)},
	}
	if err := ctrl.SetControllerReference(gate, &cm, r.Scheme); err != nil {
		return fmt.Errorf("set owner on probe evidence ConfigMap: %w", err)
	}
	if err := r.Create(ctx, &cm); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create probe evidence ConfigMap %s/%s: %w", gate.Namespace, name, err)
	}
	return nil
}

// rotateProbeEvidence keeps only the newest retainRuns per-run ConfigMaps.
func (r *AIQualityGateReconciler) rotateProbeEvidence(ctx context.Context, gate *aiopsv1alpha1.AIQualityGate, retain int) error {
	cms, err := r.listProbeEvidence(ctx, gate)
	if err != nil {
		return err
	}
	if len(cms) <= retain {
		return nil
	}
	sort.Slice(cms, func(i, j int) bool {
		return cms[i].CreationTimestamp.Before(&cms[j].CreationTimestamp)
	})
	for i := 0; i < len(cms)-retain; i++ {
		if err := r.Delete(ctx, &cms[i]); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("rotate probe evidence %s/%s: %w", cms[i].Namespace, cms[i].Name, err)
		}
	}
	return nil
}

func (r *AIQualityGateReconciler) listProbeEvidence(ctx context.Context, gate *aiopsv1alpha1.AIQualityGate) ([]corev1.ConfigMap, error) {
	var cms corev1.ConfigMapList
	if err := r.List(ctx, &cms, client.InNamespace(gate.Namespace), client.MatchingLabels{
		qualityGateLabel: gate.Name,
		probeTypeLabel:   probeTypeSynthetic,
	}); err != nil {
		return nil, fmt.Errorf("list probe evidence for %s/%s: %w", gate.Namespace, gate.Name, err)
	}
	return cms.Items, nil
}

// aggregateProbeWindow aggregates recent per-run evidence within the sliding window
// and computes the synthetic-probe verdict and scores.
func (r *AIQualityGateReconciler) aggregateProbeWindow(
	ctx context.Context,
	gate *aiopsv1alpha1.AIQualityGate,
	st *aiopsv1alpha1.AIQualityContinuousProbesStatus,
	prompts []goldenPrompt,
	sourceObs, candidateObs modelObservation,
	window time.Duration,
) {
	cms, err := r.listProbeEvidence(ctx, gate)
	if err != nil {
		st.Verdict = qualityengine.VerdictInsufficientData
		st.FailedChecks = []string{err.Error()}
		return
	}
	cutoff := time.Now().Add(-window)
	var evidence []qualityEvidenceSample
	runs := 0
	for i := range cms {
		cm := &cms[i]
		if cm.CreationTimestamp.Time.Before(cutoff) {
			continue
		}
		raw, ok := cm.Data["results.yaml"]
		if !ok {
			continue
		}
		samples, perr := parseProbeEvidence(raw)
		if perr != nil || len(samples) == 0 {
			continue
		}
		evidence = append(evidence, samples...)
		runs++
	}
	st.RunsObserved = int32(runs)

	if runs == 0 || len(prompts) == 0 {
		st.SourceScore = 0
		st.CandidateScore = 0
		st.Verdict = qualityengine.VerdictInsufficientData
		st.FailedChecks = []string{"not enough synthetic probe samples"}
		metrics.QualityProbeVerdict.WithLabelValues(gate.Namespace, gate.Name).Set(-1)
		return
	}

	comparison := qualityengine.Evaluate(qualityengine.EvaluateInput{
		SourceSamples:      probeSamplesForModel(prompts, evidence, gate.Spec.SourceModel),
		CandidateSamples:   probeSamplesForModel(prompts, evidence, gate.Spec.CandidateModel),
		SourceTelemetry:    sourceObs.toQualityTelemetry(evidence, gate.Spec.SourceModel),
		CandidateTelemetry: candidateObs.toQualityTelemetry(evidence, gate.Spec.CandidateModel),
		Weights:            qualityEngineWeights(gate.Spec.Weights),
		MinSamples:         int(defaultedMinSamples(gate.Spec.MinSamples)),
		TolerancePoints:    float64(defaultedTolerance(gate.Spec.TolerancePoints)),
		LatencyThresholdMs: float64(gate.Spec.LatencyThresholdMs),
		JudgeEnabled:       gate.Spec.Judge.Enabled,
	})

	st.Verdict = comparison.Verdict
	if comparison.Verdict == qualityengine.VerdictInsufficientData {
		st.SourceScore = 0
		st.CandidateScore = 0
		st.FailedChecks = append([]string{}, comparison.MissingInput...)
		metrics.QualityProbeVerdict.WithLabelValues(gate.Namespace, gate.Name).Set(-1)
		return
	}

	st.SourceScore = roundQuality(comparison.Source.Overall)
	st.CandidateScore = roundQuality(comparison.Candidate.Overall)
	st.FailedChecks = nil
	if comparison.Verdict == qualityengine.VerdictCandidateRisk {
		st.FailedChecks = []string{comparison.Reason}
		metrics.QualityProbeVerdict.WithLabelValues(gate.Namespace, gate.Name).Set(0)
	} else {
		metrics.QualityProbeVerdict.WithLabelValues(gate.Namespace, gate.Name).Set(1)
	}
	emitProbeScoreMetrics(gate, "source", gate.Spec.SourceModel, comparison.Source.Breakdown)
	emitProbeScoreMetrics(gate, "candidate", gate.Spec.CandidateModel, comparison.Candidate.Breakdown)
}

// scheduleProbeRun creates one ephemeral probe Job for the round-robin subset.
func (r *AIQualityGateReconciler) scheduleProbeRun(
	ctx context.Context,
	gate *aiopsv1alpha1.AIQualityGate,
	st *aiopsv1alpha1.AIQualityContinuousProbesStatus,
	prompts []goldenPrompt,
	cp *aiopsv1alpha1.AIQualityContinuousProbesSpec,
	now time.Time,
) error {
	endpoint := strings.TrimSpace(gate.Spec.Evaluation.Endpoint)
	if endpoint == "" {
		st.FailedChecks = []string{"continuous probes require spec.evaluation.endpoint with the gateway chat/completions URL"}
		return nil
	}
	if msg, err := r.evaluationSovereigntyBlock(ctx, gate); err != nil {
		return err
	} else if msg != "" {
		st.FailedChecks = []string{msg}
		return nil
	}
	image, err := r.qualityEvaluatorImage(ctx, gate)
	if err != nil {
		return err
	}

	ids := selectProbePrompts(prompts, cp, int(st.LastPromptIndex))
	runID := now.UTC().Format("20060102-150405")
	jobName := probeJobName(gate, runID)

	job := r.buildEvaluationJob(gate, jobName, image, endpoint)
	// Restrict this run to the selected subset.
	container := &job.Spec.Template.Spec.Containers[0]
	container.Args = append(container.Args, "--prompt-ids="+strings.Join(ids, ","))
	// Mark it as a synthetic continuous probe.
	job.Labels[probeTypeLabel] = probeTypeSynthetic
	job.Labels[probeRunIDLabel] = runID
	job.Spec.Template.Labels[probeTypeLabel] = probeTypeSynthetic
	job.Spec.Template.Labels[probeRunIDLabel] = runID
	if job.Annotations == nil {
		job.Annotations = map[string]string{}
	}
	job.Annotations["aiops.imperium.io/prompt-ids"] = strings.Join(ids, ",")

	if err := ctrl.SetControllerReference(gate, job, r.Scheme); err != nil {
		return fmt.Errorf("set owner on probe job: %w", err)
	}
	if err := r.Create(ctx, job); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("create probe job %s/%s: %w", gate.Namespace, jobName, err)
	}
	st.LastScheduledAt = &metav1.Time{Time: now}
	r.event(gate, corev1.EventTypeNormal, "ContinuousProbeStarted", "synthetic quality probe %q started (prompts: %s)", jobName, strings.Join(ids, ","))
	return nil
}

func (r *AIQualityGateReconciler) setProbeCondition(gate *aiopsv1alpha1.AIQualityGate, st *aiopsv1alpha1.AIQualityContinuousProbesStatus) {
	status := metav1.ConditionTrue
	reason := "LastRunSucceeded"
	message := "Continuous synthetic quality probes are scheduled."
	switch {
	case st.LastRunStatus == "Failed":
		status = metav1.ConditionFalse
		reason = "LastRunFailed"
		message = "The last synthetic quality probe run failed; see status.continuousProbes."
	case st.LastRunName == "":
		status = metav1.ConditionFalse
		reason = "NoRunYet"
		message = "Continuous probes enabled; waiting for the first synthetic run."
	}
	meta.SetStatusCondition(&gate.Status.Conditions, metav1.Condition{
		Type:               "ContinuousProbesReady",
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: gate.Generation,
	})
}

// --- pure helpers -----------------------------------------------------------

func parseProbeDuration(v string, def time.Duration) time.Duration {
	if d, err := time.ParseDuration(strings.TrimSpace(v)); err == nil && d > 0 {
		return d
	}
	return def
}

func defaultedRetainRuns(v int32) int {
	if v <= 0 {
		return defaultProbeRetainRuns
	}
	return int(v)
}

func probeSampleSize(v int32, promptCount int) int {
	if v <= 0 || int(v) > promptCount {
		return promptCount
	}
	return int(v)
}

// selectProbePrompts returns the prompt IDs to replay for one run. round-robin
// walks the dataset from the cursor; random picks a pseudo-random subset.
func selectProbePrompts(prompts []goldenPrompt, cp *aiopsv1alpha1.AIQualityContinuousProbesSpec, cursor int) []string {
	n := len(prompts)
	if n == 0 {
		return nil
	}
	size := probeSampleSize(cp.SampleSize, n)
	ids := make([]string, 0, size)
	if strings.EqualFold(cp.Strategy, "random") {
		// Deterministic pseudo-random walk seeded by the cursor, without repeats.
		seen := map[int]bool{}
		idx := cursor % n
		for len(ids) < size {
			if !seen[idx] {
				seen[idx] = true
				ids = append(ids, prompts[idx].ID)
			}
			idx = (idx*31 + 7) % n
			if len(seen) >= n {
				break
			}
		}
		return ids
	}
	for i := 0; i < size; i++ {
		ids = append(ids, prompts[(cursor+i)%n].ID)
	}
	return ids
}

// probeSamplesForModel maps every aggregated evidence entry for one model to a
// qualityengine sample (one per entry, so multiple runs contribute multiple
// samples — unlike the per-prompt buildQualitySamples used by the one-shot flow).
func probeSamplesForModel(prompts []goldenPrompt, evidence []qualityEvidenceSample, model string) []qualityengine.EvidenceSample {
	byID := map[string]goldenPrompt{}
	for _, p := range prompts {
		byID[p.ID] = p
	}
	out := make([]qualityengine.EvidenceSample, 0, len(evidence))
	for _, ev := range evidence {
		if ev.Model != model || ev.ID == "" {
			continue
		}
		p := byID[ev.ID]
		expected := firstNonEmpty(ev.Reference, ev.Expected, p.Expected.Reference)
		actual := firstNonEmpty(ev.Actual, ev.Output, ev.Response)
		expectedFields := p.Expected.Fields
		if len(ev.ExpectedFields) > 0 {
			expectedFields = ev.ExpectedFields
		}
		actualFields := ev.ActualFields
		if len(actualFields) == 0 {
			actualFields = ev.Fields
		}
		out = append(out, qualityengine.EvidenceSample{
			ID:                      ev.ID,
			Expected:                expected,
			Actual:                  actual,
			MustBeJSON:              p.Expected.MustBeJSON,
			ExpectedFields:          expectedFields,
			ActualFields:            actualFields,
			SemanticScore:           ev.SemanticScore,
			JudgedScore:             ev.JudgedScore,
			RequiredKeywordsPresent: ev.RequiredKeywordsPresent,
		})
	}
	return out
}

func parseProbeEvidence(raw string) ([]qualityEvidenceSample, error) {
	var samples []qualityEvidenceSample
	if err := yaml.Unmarshal([]byte(raw), &samples); err == nil && len(samples) > 0 {
		return samples, nil
	}
	var wrapped struct {
		Samples []qualityEvidenceSample `json:"samples"`
	}
	if err := yaml.Unmarshal([]byte(raw), &wrapped); err != nil {
		return nil, err
	}
	return wrapped.Samples, nil
}

func metricsLabels(gate *aiopsv1alpha1.AIQualityGate) []string {
	return []string{gate.Namespace, gate.Name, gate.Spec.SourceModel, gate.Spec.CandidateModel}
}

// isContributeToGate reports whether probe evidence should be pooled into the
// main scoring (decisionPolicy.effect == ContributeToGate).
func isContributeToGate(gate *aiopsv1alpha1.AIQualityGate) bool {
	cp := gate.Spec.ContinuousProbes
	return cp != nil && cp.Enabled && cp.DecisionPolicy != nil && cp.DecisionPolicy.Effect == effectContributeToGate
}

// pooledProbeEvidence returns the probe evidence within the sliding window, for
// ContributeToGate pooling into the main scoring.
func (r *AIQualityGateReconciler) pooledProbeEvidence(ctx context.Context, gate *aiopsv1alpha1.AIQualityGate) []qualityEvidenceSample {
	cp := gate.Spec.ContinuousProbes
	if cp == nil {
		return nil
	}
	window := parseProbeDuration(cp.SlidingWindow, defaultProbeSlidingWindow)
	cms, err := r.listProbeEvidence(ctx, gate)
	if err != nil {
		return nil
	}
	cutoff := time.Now().Add(-window)
	var out []qualityEvidenceSample
	for i := range cms {
		cm := &cms[i]
		if cm.CreationTimestamp.Time.Before(cutoff) {
			continue
		}
		raw, ok := cm.Data["results.yaml"]
		if !ok {
			continue
		}
		samples, perr := parseProbeEvidence(raw)
		if perr != nil {
			continue
		}
		out = append(out, samples...)
	}
	return out
}

func emitProbeScoreMetrics(gate *aiopsv1alpha1.AIQualityGate, role, model string, b qualityengine.DimensionScores) {
	set := func(dimension string, value float64) {
		metrics.QualityProbeScore.WithLabelValues(gate.Namespace, gate.Name, model, role, dimension).Set(value)
	}
	set("correctness", b.Correctness)
	set("reliability", b.Reliability)
	set("latency", b.Latency)
	set("semantic", b.Semantic)
	set("judged", b.Judged)
}

func probeJobName(gate *aiopsv1alpha1.AIQualityGate, runID string) string {
	return truncatedName("quality-probe-", gate.Name, runID)
}

func probeEvidenceName(gate *aiopsv1alpha1.AIQualityGate, runID string) string {
	return truncatedName("quality-probe-", gate.Name, runID)
}

func truncatedName(prefix, name, suffix string) string {
	base := dnsLabelPart(name)
	maxBase := 63 - len(prefix) - len(suffix) - 1
	if maxBase < 1 {
		maxBase = 1
	}
	if len(base) > maxBase {
		base = base[:maxBase]
	}
	return prefix + base + "-" + suffix
}
