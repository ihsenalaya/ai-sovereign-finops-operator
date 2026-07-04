// Package scheduler implements the ai-attestation-scheduler, a custom Kubernetes
// scheduler that only binds pods to nodes that carry valid AttestationEvidence
// matching the pod's ConfidentialInferencePolicy.
//
// Architecture: this is an out-of-process custom scheduler that uses the
// Kubernetes Binding API. It does NOT replace the default scheduler — it runs
// in parallel as a second scheduler and only handles pods with
//
//	schedulerName: ai-attestation-scheduler
//
// Scheduling extension points implemented:
//   - PreFilter : load ConfidentialInferencePolicy for the pod
//   - Filter    : reject nodes without valid AttestationEvidence
//   - Score     : score nodes by evidence freshness, TEE type, cost
//   - Reserve   : create / update AIPlacementDecision
//   - Bind      : bind pod to selected node via Kubernetes Binding API
package scheduler

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
	platformcrypto "github.com/imperium/ai-sovereign-finops-operator/pkg/crypto"
	"github.com/imperium/ai-sovereign-finops-operator/pkg/token"
)

const (
	SchedulerName          = "ai-attestation-scheduler"
	SimulatedEvidenceLabel = "ai.sovereign.io/simulated-evidence"
	NodeAttestationLabel   = "ai.sovereign.io/attested"
)

// NodeCandidate holds a node with its score and evidence.
type NodeCandidate struct {
	Node     corev1.Node
	Evidence *aiopsv1alpha1.AttestationEvidence
	Score    int
	Reason   string
}

// Scheduler is the attestation-aware custom scheduler.
type Scheduler struct {
	client             client.Client
	signingKey         ed25519.PrivateKey
	pubKey             ed25519.PublicKey
	tokenTTL           time.Duration
	permitTimeout      time.Duration
	permitPollInterval time.Duration
}

// New creates a Scheduler.
func New(c client.Client, signingKey ed25519.PrivateKey, pubKey ed25519.PublicKey, tokenTTL time.Duration) *Scheduler {
	if tokenTTL <= 0 {
		tokenTTL = 5 * time.Minute
	}
	return &Scheduler{
		client:             c,
		signingKey:         signingKey,
		pubKey:             pubKey,
		tokenTTL:           tokenTTL,
		permitTimeout:      15 * time.Second,
		permitPollInterval: 250 * time.Millisecond,
	}
}

// Client returns the underlying Kubernetes client.
func (s *Scheduler) Client() client.Client { return s.client }

// SchedulePod is the main entry point: given a pending pod, find the best node
// and bind the pod to it, creating an AIPlacementDecision.
func (s *Scheduler) SchedulePod(ctx context.Context, pod *corev1.Pod) error {
	logger := log.FromContext(ctx).WithValues("pod", pod.Name, "namespace", pod.Namespace)
	logger.Info("attestation-scheduler: scheduling pod")

	// --- PreFilter: load policy ---
	policy, err := s.preFilter(ctx, pod)
	if err != nil {
		return fmt.Errorf("prefilter: %w", err)
	}

	// --- List candidate nodes ---
	var nodeList corev1.NodeList
	if err := s.client.List(ctx, &nodeList); err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}

	// --- Filter: keep only nodes with valid evidence ---
	candidates, err := s.filter(ctx, pod, policy, nodeList.Items)
	if err != nil {
		return fmt.Errorf("filter: %w", err)
	}
	if len(candidates) == 0 {
		candidates, err = s.permit(ctx, pod, policy, nodeList.Items)
		if err != nil {
			return fmt.Errorf("permit: %w", err)
		}
		if len(candidates) == 0 {
			return fmt.Errorf("no suitable node with valid AttestationEvidence for pod %s/%s", pod.Namespace, pod.Name)
		}
	}

	// --- Score: rank candidates ---
	s.score(candidates, policy)

	// Sort descending by score
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})
	selected := candidates[0]
	logger.Info("attestation-scheduler: node selected", "node", selected.Node.Name, "score", selected.Score)

	// --- Reserve: create AIPlacementDecision ---
	placementToken, tokenStr, err := s.reserve(ctx, pod, policy, selected)
	if err != nil {
		return fmt.Errorf("reserve: %w", err)
	}

	// --- Bind: bind pod to node ---
	if err := s.bind(ctx, pod, selected.Node.Name); err != nil {
		return fmt.Errorf("bind: %w", err)
	}

	logger.Info("attestation-scheduler: pod bound",
		"node", selected.Node.Name,
		"placementDecision", placementToken.Name,
		"tokenDigest", tokenStr[:min(16, len(tokenStr))],
	)
	return nil
}

// permit waits briefly for evidence to become valid when candidates are close
// to admission but not yet fresh enough or still pending verification.
func (s *Scheduler) permit(
	ctx context.Context,
	pod *corev1.Pod,
	policy *aiopsv1alpha1.ConfidentialInferencePolicy,
	nodes []corev1.Node,
) ([]NodeCandidate, error) {
	if s.permitTimeout <= 0 {
		return nil, nil
	}

	deadline := time.Now().Add(s.permitTimeout)
	var lastCandidates []NodeCandidate
	for {
		candidates, err := s.filter(ctx, pod, policy, nodes)
		if err != nil {
			return nil, err
		}
		if len(candidates) > 0 {
			return candidates, nil
		}
		lastCandidates = candidates
		if time.Now().After(deadline) {
			return lastCandidates, fmt.Errorf("timed out waiting for valid attestation evidence after %s", s.permitTimeout)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(s.permitPollInterval):
		}
	}
}

// preFilter loads the ConfidentialInferencePolicy for this pod.
// Returns nil if no policy is found (non-confidential pod — should not reach this scheduler).
func (s *Scheduler) preFilter(ctx context.Context, pod *corev1.Pod) (*aiopsv1alpha1.ConfidentialInferencePolicy, error) {
	var ns corev1.Namespace
	if err := s.client.Get(ctx, types.NamespacedName{Name: pod.Namespace}, &ns); err != nil {
		return nil, fmt.Errorf("read namespace %q: %w", pod.Namespace, err)
	}

	var list aiopsv1alpha1.ConfidentialInferencePolicyList
	if err := s.client.List(ctx, &list, client.InNamespace(pod.Namespace)); err != nil {
		return nil, fmt.Errorf("list ConfidentialInferencePolicy: %w", err)
	}

	for i := range list.Items {
		policy := &list.Items[i]
		if policyMatchesPod(policy, &ns, pod) {
			return policy, nil
		}
	}
	return nil, nil
}

// filter returns nodes that carry a valid, non-revoked AttestationEvidence.
func (s *Scheduler) filter(
	ctx context.Context,
	pod *corev1.Pod,
	policy *aiopsv1alpha1.ConfidentialInferencePolicy,
	nodes []corev1.Node,
) ([]NodeCandidate, error) {
	var candidates []NodeCandidate

	for i := range nodes {
		node := &nodes[i]

		// Skip unschedulable or tainted nodes.
		if node.Spec.Unschedulable {
			continue
		}

		ev, reason, ok := s.nodeHasValidEvidence(ctx, node, policy)
		if !ok {
			log.FromContext(ctx).V(1).Info("attestation-scheduler: node filtered", "node", node.Name, "reason", reason)
			continue
		}
		candidates = append(candidates, NodeCandidate{Node: *node, Evidence: ev})
	}
	return candidates, nil
}

// nodeHasValidEvidence returns the best matching AttestationEvidence for a node, or a reason for rejection.
func (s *Scheduler) nodeHasValidEvidence(
	ctx context.Context,
	node *corev1.Node,
	policy *aiopsv1alpha1.ConfidentialInferencePolicy,
) (*aiopsv1alpha1.AttestationEvidence, string, bool) {
	var evList aiopsv1alpha1.AttestationEvidenceList
	if err := s.client.List(ctx, &evList); err != nil {
		return nil, "cannot list AttestationEvidence", false
	}

	now := metav1.Now()
	for i := range evList.Items {
		ev := &evList.Items[i]
		if ev.Spec.SubjectRef.Name != node.Name {
			continue
		}
		if ev.Status.Revoked {
			return nil, fmt.Sprintf("AttestationEvidence %q is revoked", ev.Name), false
		}
		if !ev.Status.Verified {
			continue
		}
		// Check freshness
		if policy != nil && policy.Spec.MaxEvidenceAgeSeconds > 0 && ev.Status.LastVerifiedTime != nil {
			age := now.Sub(ev.Status.LastVerifiedTime.Time)
			maxAge := time.Duration(policy.Spec.MaxEvidenceAgeSeconds) * time.Second
			if age > maxAge {
				return nil, fmt.Sprintf("evidence age %v exceeds max %v", age.Round(time.Second), maxAge), false
			}
		}
		// Check TEE requirements
		if policy != nil && len(policy.Spec.RequiredTEE) > 0 {
			if !teeMatches(ev.Spec.TEE, policy.Spec.RequiredTEE) {
				return nil, fmt.Sprintf("TEE %q not in required list", ev.Spec.TEE), false
			}
		}
		return ev, "", true
	}
	return nil, "no valid AttestationEvidence for node", false
}

// score assigns a score to each candidate node. Higher is better.
func (s *Scheduler) score(candidates []NodeCandidate, policy *aiopsv1alpha1.ConfidentialInferencePolicy) {
	for i := range candidates {
		c := &candidates[i]
		score := 100

		if c.Evidence != nil {
			// Fresher evidence → higher score
			if c.Evidence.Status.LastVerifiedTime != nil && policy != nil && policy.Spec.MaxEvidenceAgeSeconds > 0 {
				age := time.Since(c.Evidence.Status.LastVerifiedTime.Time)
				maxAge := time.Duration(policy.Spec.MaxEvidenceAgeSeconds) * time.Second
				freshnessFraction := 1.0 - (float64(age) / float64(maxAge))
				if freshnessFraction < 0 {
					freshnessFraction = 0
				}
				score += int(freshnessFraction * 50)
			}

			// Prefer non-simulated evidence
			if !c.Evidence.Spec.Simulated {
				score += 20
			}

			// TDX > SEV-SNP > other
			switch strings.ToUpper(c.Evidence.Spec.TEE) {
			case "TDX":
				score += 10
			case "SEV-SNP", "SEV_SNP":
				score += 5
			}
		}

		c.Score = score
	}
}

// reserve creates or updates the AIPlacementDecision and mints the placement token.
func (s *Scheduler) reserve(
	ctx context.Context,
	pod *corev1.Pod,
	policy *aiopsv1alpha1.ConfidentialInferencePolicy,
	selected NodeCandidate,
) (*aiopsv1alpha1.AIPlacementDecision, string, error) {
	podSpecHash, err := platformcrypto.CanonicalSHA256Hex(pod.Spec)
	if err != nil {
		return nil, "", fmt.Errorf("hash pod spec: %w", err)
	}

	var policyHash, evidenceHash string
	if policy != nil {
		policyHash, err = platformcrypto.CanonicalSHA256Hex(policy.Spec)
		if err != nil {
			return nil, "", fmt.Errorf("hash policy spec: %w", err)
		}
	}
	if selected.Evidence != nil {
		evidenceHash, err = evidenceHashForPlacement(selected.Evidence)
		if err != nil {
			return nil, "", fmt.Errorf("hash evidence: %w", err)
		}
	}

	runtimeClass := ""
	if pod.Spec.RuntimeClassName != nil {
		runtimeClass = *pod.Spec.RuntimeClassName
	}

	imageDigest := ""
	if len(pod.Spec.Containers) > 0 {
		imageDigest = pod.Spec.Containers[0].Image
	}
	modelDigest := pod.Annotations["ai.sovereign.io/model-digest"]

	// Mint placement token (no-op if no signing key)
	tokenStr := ""
	if s.signingKey != nil {
		tok, err := token.Mint(s.signingKey,
			string(pod.UID), podSpecHash,
			imageDigest, modelDigest,
			selected.Node.Name, runtimeClass,
			evidenceHash, policyHash,
			s.tokenTTL,
			token.MintOptions{},
		)
		if err != nil {
			return nil, "", fmt.Errorf("mint placement token: %w", err)
		}
		encoded, err := token.Encode(tok)
		if err != nil {
			return nil, "", fmt.Errorf("encode placement token: %w", err)
		}
		tokenStr = encoded
	}

	tokenDigest := platformcrypto.SHA256Hex([]byte(tokenStr))

	decisionName := fmt.Sprintf("%s-%s", pod.Name, pod.Namespace)
	if len(decisionName) > 63 {
		decisionName = decisionName[:63]
	}

	decision := &aiopsv1alpha1.AIPlacementDecision{
		ObjectMeta: metav1.ObjectMeta{
			Name:      decisionName,
			Namespace: pod.Namespace,
			Labels: map[string]string{
				"ai.sovereign.io/pod-name": pod.Name,
			},
		},
	}

	decision, _, err = createOrUpdatePlacementDecision(ctx, s.client, decision, func(decision *aiopsv1alpha1.AIPlacementDecision) {
		decision.Spec.TargetRef = aiopsv1alpha1.ObjectReference{Name: pod.Name, Namespace: pod.Namespace}
		if policy != nil {
			decision.Spec.PolicyRef = aiopsv1alpha1.ObjectReference{Name: policy.Name, Namespace: policy.Namespace}
		}
		if selected.Evidence != nil {
			ref := aiopsv1alpha1.ObjectReference{Name: selected.Evidence.Name, Namespace: selected.Evidence.Namespace}
			decision.Spec.EvidenceRef = &ref
		}
		decision.Spec.SchedulerName = SchedulerName
		decision.Status.Decision = "allow"
		decision.Status.NodeName = selected.Node.Name
		decision.Status.PlacementTokenDigest = tokenDigest
		decision.Status.Simulated = selected.Evidence != nil && selected.Evidence.Spec.Simulated
	})
	if err != nil {
		return nil, "", fmt.Errorf("upsert AIPlacementDecision: %w", err)
	}

	if err := s.preBind(ctx, pod, decision, selected); err != nil {
		return nil, "", fmt.Errorf("prebind: %w", err)
	}

	return decision, tokenStr, nil
}

// bind binds the pod to the target node using the Kubernetes Binding API.
func (s *Scheduler) bind(ctx context.Context, pod *corev1.Pod, nodeName string) error {
	binding := &corev1.Binding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		},
		Target: corev1.ObjectReference{
			Kind: "Node",
			Name: nodeName,
		},
	}
	return s.client.Create(ctx, binding)
}

// --- helpers ---

func policyMatchesPod(policy *aiopsv1alpha1.ConfidentialInferencePolicy, ns *corev1.Namespace, pod *corev1.Pod) bool {
	if policy.Spec.Target.NamespaceSelector != nil {
		sel, err := metav1.LabelSelectorAsSelector(policy.Spec.Target.NamespaceSelector)
		if err != nil || !sel.Matches(labels.Set(ns.Labels)) {
			return false
		}
	}
	if policy.Spec.Target.WorkloadSelector != nil {
		sel, err := metav1.LabelSelectorAsSelector(policy.Spec.Target.WorkloadSelector)
		if err != nil || !sel.Matches(labels.Set(pod.Labels)) {
			return false
		}
	}
	return true
}

func teeMatches(nodeTEE string, required []string) bool {
	nodeTEENorm := strings.ToUpper(strings.TrimSpace(nodeTEE))
	for _, req := range required {
		if strings.ToUpper(strings.TrimSpace(req)) == nodeTEENorm {
			return true
		}
	}
	return false
}

func createOrUpdatePlacementDecision(
	ctx context.Context,
	c client.Client,
	decision *aiopsv1alpha1.AIPlacementDecision,
	mutate func(*aiopsv1alpha1.AIPlacementDecision),
) (*aiopsv1alpha1.AIPlacementDecision, string, error) {
	var existing aiopsv1alpha1.AIPlacementDecision
	err := c.Get(ctx, types.NamespacedName{Name: decision.Name, Namespace: decision.Namespace}, &existing)
	if apierrors.IsNotFound(err) {
		mutate(decision)
		if err := c.Create(ctx, decision); err != nil {
			return nil, "", err
		}
		return decision, "created", nil
	}
	if err != nil {
		return nil, "", err
	}
	mutate(&existing)
	if err := c.Update(ctx, &existing); err != nil {
		return nil, "", err
	}
	return &existing, "updated", nil
}

func (s *Scheduler) preBind(
	ctx context.Context,
	pod *corev1.Pod,
	decision *aiopsv1alpha1.AIPlacementDecision,
	selected NodeCandidate,
) error {
	if decision == nil {
		return fmt.Errorf("placement decision is nil")
	}

	var latestPod corev1.Pod
	if err := s.client.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, &latestPod); err != nil {
		return fmt.Errorf("read pod before bind: %w", err)
	}

	if decision.Spec.PolicyRef.Name != "" {
		var policy aiopsv1alpha1.ConfidentialInferencePolicy
		if err := s.client.Get(ctx, types.NamespacedName{
			Name:      decision.Spec.PolicyRef.Name,
			Namespace: decision.Spec.PolicyRef.Namespace,
		}, &policy); err != nil {
			return fmt.Errorf("read policy before bind: %w", err)
		}
		expectedPolicyHash := strings.TrimSpace(latestPod.Annotations["ai.sovereign.io/policy-hash"])
		if expectedPolicyHash != "" {
			currentPolicyHash, err := platformcrypto.CanonicalSHA256Hex(policy.Spec)
			if err != nil {
				return fmt.Errorf("hash policy before bind: %w", err)
			}
			if currentPolicyHash != expectedPolicyHash {
				return fmt.Errorf("policy hash mismatch before bind")
			}
		}
	}

	if decision.Spec.EvidenceRef == nil {
		return nil
	}

	var evidence aiopsv1alpha1.AttestationEvidence
	if err := s.client.Get(ctx, types.NamespacedName{
		Name:      decision.Spec.EvidenceRef.Name,
		Namespace: decision.Spec.EvidenceRef.Namespace,
	}, &evidence); err != nil {
		return fmt.Errorf("read evidence before bind: %w", err)
	}
	if evidence.Status.Revoked {
		return fmt.Errorf("evidence revoked before bind")
	}

	evidenceHash, err := evidenceHashForPlacement(&evidence)
	if err != nil {
		return fmt.Errorf("hash evidence before bind: %w", err)
	}

	if selected.Evidence != nil {
		selectedHash, err := evidenceHashForPlacement(selected.Evidence)
		if err != nil {
			return fmt.Errorf("hash selected evidence before bind: %w", err)
		}
		if evidenceHash != selectedHash {
			return fmt.Errorf("evidence hash mismatch before bind")
		}
	}

	maxAge := time.Duration(0)
	if ageRaw := strings.TrimSpace(latestPod.Annotations["ai.sovereign.io/expected-evidence-age-seconds"]); ageRaw != "" {
		var secs int
		if _, err := fmt.Sscan(ageRaw, &secs); err == nil && secs > 0 {
			maxAge = time.Duration(secs) * time.Second
		}
	}
	if maxAge > 0 && evidence.Status.LastVerifiedTime != nil && time.Since(evidence.Status.LastVerifiedTime.Time) > maxAge {
		return fmt.Errorf("evidence expired before bind")
	}
	return nil
}

func evidenceHashForPlacement(evidence *aiopsv1alpha1.AttestationEvidence) (string, error) {
	if evidence == nil {
		return "", nil
	}
	payload := struct {
		Spec   aiopsv1alpha1.AttestationEvidenceSpec   `json:"spec"`
		Status aiopsv1alpha1.AttestationEvidenceStatus `json:"status"`
	}{
		Spec:   evidence.Spec,
		Status: evidence.Status,
	}
	return platformcrypto.CanonicalSHA256Hex(payload)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
