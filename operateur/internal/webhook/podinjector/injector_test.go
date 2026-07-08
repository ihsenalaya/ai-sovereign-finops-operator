package podinjector

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	jsonpatch "github.com/evanphx/json-patch/v5"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	nodev1 "k8s.io/api/node/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
)

func TestInjectsSidecarForAnnotatedPod(t *testing.T) {
	scheme := newScheme(t)
	h := New(fakeClient(t, scheme), scheme, StaticImageResolver("controller:test"))

	original := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "risk-assistant-abc",
			Namespace: "finance",
			Labels:    map[string]string{"app": "risk-assistant"},
			Annotations: map[string]string{
				InjectKey: "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app", Image: "curlimages/curl:latest"}},
		},
	}

	mutated := runMutation(t, h, original)
	if !hasContainer(mutated, SidecarContainerName) {
		t.Fatalf("sidecar %q not injected", SidecarContainerName)
	}
	if got := envValue(mutated.Spec.Containers[0].Env, "HTTP_PROXY"); got != ProxyURL {
		t.Fatalf("HTTP_PROXY = %q, want %q", got, ProxyURL)
	}
	if got := envValue(findContainer(t, mutated, SidecarContainerName).Env, "GREENOPS_APPLICATION"); got != "risk-assistant" {
		t.Fatalf("GREENOPS_APPLICATION = %q, want risk-assistant", got)
	}
	if mutated.Annotations[InjectedProxyKey] != "true" {
		t.Fatalf("%s annotation missing", InjectedProxyKey)
	}
}

func TestInjectsFromNamespaceLabel(t *testing.T) {
	scheme := newScheme(t)
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "legal",
			Labels: map[string]string{InjectKey: "enabled"},
		},
	}
	h := New(fakeClient(t, scheme, ns), scheme, StaticImageResolver("controller:test"))

	original := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "contract-review-",
			Namespace:    "legal",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app", Image: "busybox"}},
		},
	}

	mutated := runMutation(t, h, original)
	sidecar := findContainer(t, mutated, SidecarContainerName)
	if got := envValue(sidecar.Env, "GREENOPS_APPLICATION"); got != "contract-review" {
		t.Fatalf("GREENOPS_APPLICATION = %q, want contract-review", got)
	}
}

func TestSkipsWhenNotEnabled(t *testing.T) {
	scheme := newScheme(t)
	h := New(fakeClient(t, scheme), scheme, StaticImageResolver("controller:test"))

	original := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "plain", Namespace: "default"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "busybox"}}},
	}

	raw, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal pod: %v", err)
	}
	resp := h.Handle(context.Background(), admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
			Namespace: "default",
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Object:    runtime.RawExtension{Raw: raw},
		},
	})
	if len(resp.Patches) != 0 {
		t.Fatalf("unexpected patches for non-enabled pod: %+v", resp.Patches)
	}
}

func TestManagerPodImageResolver(t *testing.T) {
	scheme := newScheme(t)
	managerPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "manager-0", Namespace: "system"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "manager", Image: "ghcr.io/example/greenops:1.2.3"},
			},
		},
	}
	resolver := &ManagerPodImageResolver{
		Client:       fakeClient(t, scheme, managerPod),
		PodName:      "manager-0",
		PodNamespace: "system",
	}
	got, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatalf("resolve image: %v", err)
	}
	if got != "ghcr.io/example/greenops:1.2.3" {
		t.Fatalf("resolved image = %q", got)
	}
}

func TestInjectsConfidentialRuntimeClassAndSchedulerInSimulatedMode(t *testing.T) {
	t.Setenv(PlatformModeEnv, PlatformModeSimulatedKind)
	scheme := newScheme(t)
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "finance",
			Labels: map[string]string{"ai.sovereign.io/sensitivity": "high"},
		},
	}
	policy := &aiopsv1alpha1.ConfidentialInferencePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "finance-conf", Namespace: "finance"},
		Spec: aiopsv1alpha1.ConfidentialInferencePolicySpec{
			Target: aiopsv1alpha1.WorkloadTarget{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"ai.sovereign.io/sensitivity": "high"}},
				WorkloadSelector:  &metav1.LabelSelector{MatchLabels: map[string]string{"app": "risk-assistant"}},
			},
			RequiredTEE:                   []string{"TDX"},
			RequireConfidentialContainers: true,
			AllowedRuntimeClasses:         []string{"kata-qemu-tdx"},
			MaxEvidenceAgeSeconds:         300,
			EnforcementMode:               aiopsv1alpha1.EnforcementModeEnforce,
		},
	}
	h := New(fakeClient(t, scheme, ns, policy), scheme, StaticImageResolver("controller:test"))

	original := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "risk-assistant",
			Namespace: "finance",
			Labels:    map[string]string{"app": "risk-assistant"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app", Image: "ghcr.io/example/app@sha256:abc"}},
		},
	}

	mutated := runMutation(t, h, original)
	if mutated.Spec.RuntimeClassName == nil || *mutated.Spec.RuntimeClassName != "simulated-kata-qemu-tdx" {
		t.Fatalf("runtimeClassName = %v", mutated.Spec.RuntimeClassName)
	}
	if mutated.Spec.SchedulerName != DefaultSchedulerName {
		t.Fatalf("schedulerName = %q", mutated.Spec.SchedulerName)
	}
	if len(mutated.Spec.SchedulingGates) != 1 || mutated.Spec.SchedulingGates[0].Name != AttestationEvidenceGate {
		t.Fatalf("unexpected scheduling gates: %+v", mutated.Spec.SchedulingGates)
	}
	if mutated.Annotations[ExpectedRuntimeAnnotation] != "kata-qemu-tdx" {
		t.Fatalf("expected runtime annotation = %q", mutated.Annotations[ExpectedRuntimeAnnotation])
	}
	if mutated.Annotations[SimulatedExecutionAnnotation] != "true" {
		t.Fatalf("simulated annotation = %q", mutated.Annotations[SimulatedExecutionAnnotation])
	}
}

func TestConfidentialAnnotationsPatchWhenSchedulingAlreadySet(t *testing.T) {
	t.Setenv(PlatformModeEnv, "aks-private")
	scheme := newScheme(t)
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "finance",
			Labels: map[string]string{"ai.sovereign.io/sensitivity": "high"},
		},
	}
	policy := &aiopsv1alpha1.ConfidentialInferencePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "finance-conf", Namespace: "finance"},
		Spec: aiopsv1alpha1.ConfidentialInferencePolicySpec{
			Target: aiopsv1alpha1.WorkloadTarget{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"ai.sovereign.io/sensitivity": "high"}},
				WorkloadSelector:  &metav1.LabelSelector{MatchLabels: map[string]string{"app": "risk-assistant"}},
			},
			RequiredTEE:                   []string{"SEV-SNP"},
			RequireConfidentialContainers: false,
			AllowedRuntimeClasses:         []string{"runc"},
			MaxEvidenceAgeSeconds:         300,
			RequireModelDigest:            true,
			EnforcementMode:               aiopsv1alpha1.EnforcementModeEnforce,
		},
	}
	h := New(fakeClient(t, scheme, ns, policy), scheme, StaticImageResolver("controller:test"))
	runtimeClass := "runc"
	original := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "risk-assistant",
			Namespace: "finance",
			Labels:    map[string]string{"app": "risk-assistant"},
			Annotations: map[string]string{
				ModelDigestAnnotation:         "sha256:model",
				AttestationEvidenceAnnotation: "evidence-node-a",
			},
		},
		Spec: corev1.PodSpec{
			SchedulerName:    DefaultSchedulerName,
			RuntimeClassName: &runtimeClass,
			Containers:       []corev1.Container{{Name: "app", Image: "registry.k8s.io/pause:3.9"}},
		},
	}

	mutated := runMutation(t, h, original)
	if mutated.Annotations[PolicyHashAnnotation] == "" {
		t.Fatalf("%s annotation missing", PolicyHashAnnotation)
	}
	if mutated.Annotations[ExpectedRuntimeAnnotation] != "runc" {
		t.Fatalf("expected runtime annotation = %q", mutated.Annotations[ExpectedRuntimeAnnotation])
	}
	if mutated.Annotations[ExpectedEvidenceAgeAnnotation] != "300" {
		t.Fatalf("expected evidence age annotation = %q", mutated.Annotations[ExpectedEvidenceAgeAnnotation])
	}
	if len(mutated.Spec.SchedulingGates) != 0 {
		t.Fatalf("evidence-bearing pod should not receive scheduling gates: %+v", mutated.Spec.SchedulingGates)
	}
}

func TestValidationRejectsSimulatedRuntimeClassInProduction(t *testing.T) {
	t.Setenv(PlatformModeEnv, PlatformModeProduction)
	assertValidationRejectsSimulatedRuntimeClass(t)
}

func TestValidationTreatsAKSPrivateAsProduction(t *testing.T) {
	t.Setenv(PlatformModeEnv, "aks-private")
	assertValidationRejectsSimulatedRuntimeClass(t)
}

func assertValidationRejectsSimulatedRuntimeClass(t *testing.T) {
	t.Helper()
	scheme := newScheme(t)
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "finance",
			Labels: map[string]string{"ai.sovereign.io/sensitivity": "high"},
		},
	}
	policy := &aiopsv1alpha1.ConfidentialInferencePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "finance-conf", Namespace: "finance"},
		Spec: aiopsv1alpha1.ConfidentialInferencePolicySpec{
			Target: aiopsv1alpha1.WorkloadTarget{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"ai.sovereign.io/sensitivity": "high"}},
				WorkloadSelector:  &metav1.LabelSelector{MatchLabels: map[string]string{"app": "risk-assistant"}},
			},
			RequiredTEE:                   []string{"TDX"},
			RequireConfidentialContainers: true,
			AllowedRuntimeClasses:         []string{"kata-qemu-tdx"},
			MaxEvidenceAgeSeconds:         300,
			RequireImageDigest:            true,
			RequireModelDigest:            true,
			EnforcementMode:               aiopsv1alpha1.EnforcementModeEnforce,
		},
	}
	rc := &nodev1.RuntimeClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "simulated-kata-qemu-tdx",
			Labels: map[string]string{SimulatedRuntimeClassLabel: "true"},
		},
		Handler: "runc",
	}
	h := NewValidation(fakeClient(t, scheme, ns, policy, rc), scheme)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "risk-assistant",
			Namespace:   "finance",
			Labels:      map[string]string{"app": "risk-assistant"},
			Annotations: map[string]string{ModelDigestAnnotation: "sha256:model"},
		},
		Spec: corev1.PodSpec{
			RuntimeClassName: ptr("simulated-kata-qemu-tdx"),
			Containers:       []corev1.Container{{Name: "app", Image: "ghcr.io/example/app@sha256:abc"}},
		},
	}
	raw, err := json.Marshal(pod)
	if err != nil {
		t.Fatalf("marshal pod: %v", err)
	}
	resp := h.Handle(context.Background(), admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
			Namespace: pod.Namespace,
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Object:    runtime.RawExtension{Raw: raw},
		},
	})
	if resp.Allowed {
		t.Fatal("expected validation denial in production mode")
	}
}

func TestValidationRejectsConfidentialGPUUntilEvidenceExists(t *testing.T) {
	t.Setenv(PlatformModeEnv, PlatformModeProduction)
	scheme := newScheme(t)
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "finance",
			Labels: map[string]string{"ai.sovereign.io/sensitivity": "high"},
		},
	}
	policy := &aiopsv1alpha1.ConfidentialInferencePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "gpu-conf", Namespace: "finance"},
		Spec: aiopsv1alpha1.ConfidentialInferencePolicySpec{
			Target: aiopsv1alpha1.WorkloadTarget{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"ai.sovereign.io/sensitivity": "high"}},
				WorkloadSelector:  &metav1.LabelSelector{MatchLabels: map[string]string{"app": "gpu-risk"}},
			},
			RequiredTEE:                   []string{"SEV-SNP"},
			RequireConfidentialContainers: true,
			AllowedRuntimeClasses:         []string{"kata-qemu-snp"},
			RequireConfidentialGPU:        true,
			GPU: &aiopsv1alpha1.ConfidentialGPURequirements{
				Vendor:      "nvidia",
				DeviceClass: "confidential",
			},
			MaxEvidenceAgeSeconds: 300,
			RequireModelDigest:    true,
			EnforcementMode:       aiopsv1alpha1.EnforcementModeEnforce,
		},
	}
	h := NewValidation(fakeClient(t, scheme, ns, policy), scheme)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "gpu-risk",
			Namespace:   "finance",
			Labels:      map[string]string{"app": "gpu-risk"},
			Annotations: map[string]string{ModelDigestAnnotation: "sha256:model"},
		},
		Spec: corev1.PodSpec{
			RuntimeClassName: ptr("kata-qemu-snp"),
			Containers:       []corev1.Container{{Name: "app", Image: "ghcr.io/example/app@sha256:abc"}},
		},
	}
	raw, err := json.Marshal(pod)
	if err != nil {
		t.Fatalf("marshal pod: %v", err)
	}
	resp := h.Handle(context.Background(), admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
			Namespace: pod.Namespace,
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Object:    runtime.RawExtension{Raw: raw},
		},
	})
	if resp.Allowed {
		t.Fatal("expected confidential GPU workload to fail closed")
	}
	if resp.Result == nil || !strings.Contains(resp.Result.Message, "confidential GPU attestation is not implemented") {
		t.Fatalf("unexpected denial message: %+v", resp.Result)
	}
}

func runMutation(t *testing.T, h *Handler, pod *corev1.Pod) *corev1.Pod {
	t.Helper()
	raw, err := json.Marshal(pod)
	if err != nil {
		t.Fatalf("marshal pod: %v", err)
	}
	resp := h.Handle(context.Background(), admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
			Namespace: pod.Namespace,
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Object:    runtime.RawExtension{Raw: raw},
		},
	})
	if !resp.Allowed {
		t.Fatalf("admission denied: %+v", resp.Result)
	}
	if len(resp.Patches) == 0 {
		t.Fatal("expected patches, got none")
	}
	patchBytes, err := json.Marshal(resp.Patches)
	if err != nil {
		t.Fatalf("marshal patches: %v", err)
	}
	patch, err := jsonpatch.DecodePatch(patchBytes)
	if err != nil {
		t.Fatalf("decode patches: %v", err)
	}
	mutatedRaw, err := patch.Apply(raw)
	if err != nil {
		t.Fatalf("apply patches: %v", err)
	}
	var mutated corev1.Pod
	if err := json.Unmarshal(mutatedRaw, &mutated); err != nil {
		t.Fatalf("unmarshal mutated pod: %v", err)
	}
	return &mutated
}

func fakeClient(t *testing.T, scheme *runtime.Scheme, objs ...client.Object) client.Client {
	t.Helper()
	return fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 to scheme: %v", err)
	}
	if err := nodev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add nodev1 to scheme: %v", err)
	}
	if err := aiopsv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add aiopsv1alpha1 to scheme: %v", err)
	}
	return scheme
}

func findContainer(t *testing.T, pod *corev1.Pod, name string) corev1.Container {
	t.Helper()
	for _, c := range pod.Spec.Containers {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("container %q not found", name)
	return corev1.Container{}
}

func envValue(envs []corev1.EnvVar, name string) string {
	for _, env := range envs {
		if env.Name == name {
			return env.Value
		}
	}
	return ""
}

func ptr[T any](v T) *T { return &v }
