package podinjector

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	InjectKey            = "aiops.imperium.io/sidecar-injection"
	ApplicationKey       = "aiops.imperium.io/application"
	TargetHostsKey       = "aiops.imperium.io/target-hosts"
	InjectedProxyKey     = "aiops.imperium.io/sidecar-injected"
	SidecarContainerName = "greenops-header-proxy"
	ProxyURL             = "http://127.0.0.1:15088"
)

// ImageResolver resolves the image used for the injected sidecar.
type ImageResolver interface {
	Resolve(context.Context) (string, error)
}

// StaticImageResolver always returns the same image.
type StaticImageResolver string

// Resolve implements ImageResolver.
func (r StaticImageResolver) Resolve(_ context.Context) (string, error) {
	if strings.TrimSpace(string(r)) == "" {
		return "", fmt.Errorf("sidecar image is empty")
	}
	return string(r), nil
}

// ManagerPodImageResolver reuses the manager container image for injected sidecars.
type ManagerPodImageResolver struct {
	Client        client.Reader
	PodName       string
	PodNamespace  string
	ContainerName string

	mu     sync.Mutex
	cached string
}

// Resolve implements ImageResolver.
func (r *ManagerPodImageResolver) Resolve(ctx context.Context) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cached != "" {
		return r.cached, nil
	}
	if r.Client == nil {
		return "", fmt.Errorf("manager pod image resolver requires a client")
	}
	if r.PodName == "" || r.PodNamespace == "" {
		return "", fmt.Errorf("manager pod name/namespace not configured")
	}
	var pod corev1.Pod
	if err := r.Client.Get(ctx, types.NamespacedName{Name: r.PodName, Namespace: r.PodNamespace}, &pod); err != nil {
		return "", fmt.Errorf("read manager pod image: %w", err)
	}
	containerName := r.ContainerName
	if containerName == "" {
		containerName = "manager"
	}
	for _, c := range pod.Spec.Containers {
		if c.Name == containerName {
			r.cached = c.Image
			return r.cached, nil
		}
	}
	return "", fmt.Errorf("manager container %q not found in pod %s/%s", containerName, r.PodNamespace, r.PodName)
}

// Handler injects the greenops sidecar proxy into opted-in Pods.
type Handler struct {
	client        client.Reader
	decoder       *admission.Decoder
	imageResolver ImageResolver
}

// New returns a mutating Pod webhook handler.
func New(c client.Reader, scheme *runtime.Scheme, resolver ImageResolver) *Handler {
	return &Handler{
		client:        c,
		decoder:       admission.NewDecoder(scheme),
		imageResolver: resolver,
	}
}

// Handle mutates Pods that opt into sidecar injection.
func (h *Handler) Handle(ctx context.Context, req admission.Request) admission.Response {
	var pod corev1.Pod
	if err := h.decoder.Decode(req, &pod); err != nil {
		return admission.Errored(400, err)
	}

	if req.Operation != "" && req.Operation != admissionv1.Create {
		return admission.Allowed("pod sidecar injection only runs on create")
	}

	namespace := req.Namespace
	if namespace == "" {
		namespace = pod.Namespace
	}
	enabled, err := h.shouldInject(ctx, namespace, &pod)
	if err != nil {
		return admission.Errored(500, err)
	}
	if !enabled {
		return admission.Allowed("pod sidecar injection not enabled")
	}
	if hasContainer(&pod, SidecarContainerName) {
		return admission.Allowed("greenops sidecar already present")
	}
	if h.imageResolver == nil {
		return admission.Errored(500, fmt.Errorf("no sidecar image resolver configured"))
	}
	image, err := h.imageResolver.Resolve(ctx)
	if err != nil {
		return admission.Errored(500, err)
	}

	mutated := pod.DeepCopy()
	app := resolveApplication(mutated)
	targetHosts := parseCSV(mutated.Annotations[TargetHostsKey])
	mutated.Spec.Containers = append(mutated.Spec.Containers, sidecarContainer(image, app, targetHosts))
	for i := range mutated.Spec.Containers {
		if mutated.Spec.Containers[i].Name == SidecarContainerName {
			continue
		}
		injectProxyEnv(&mutated.Spec.Containers[i].Env)
	}
	if mutated.Annotations == nil {
		mutated.Annotations = map[string]string{}
	}
	mutated.Annotations[InjectedProxyKey] = "true"

	marshaled, err := json.Marshal(mutated)
	if err != nil {
		return admission.Errored(500, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaled)
}

func (h *Handler) shouldInject(ctx context.Context, namespace string, pod *corev1.Pod) (bool, error) {
	if v, ok := pod.Annotations[InjectKey]; ok {
		return isEnabled(v), nil
	}
	if h.client == nil || namespace == "" {
		return false, nil
	}
	var ns corev1.Namespace
	if err := h.client.Get(ctx, types.NamespacedName{Name: namespace}, &ns); err != nil {
		return false, fmt.Errorf("read namespace %q for sidecar injection: %w", namespace, err)
	}
	return isEnabled(ns.Labels[InjectKey]), nil
}

func sidecarContainer(image, app string, targetHosts []string) corev1.Container {
	env := []corev1.EnvVar{
		{
			Name: "GREENOPS_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"},
			},
		},
		{Name: "GREENOPS_APPLICATION", Value: app},
	}
	if len(targetHosts) > 0 {
		env = append(env, corev1.EnvVar{Name: "GREENOPS_TARGET_HOSTS", Value: strings.Join(targetHosts, ",")})
	}
	return corev1.Container{
		Name:            SidecarContainerName,
		Image:           image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"/header-proxy"},
		Args:            []string{"--listen=:15088"},
		Env:             env,
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolPtr(false),
			RunAsNonRoot:             boolPtr(true),
			ReadOnlyRootFilesystem:   boolPtr(true),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{},
			Limits:   corev1.ResourceList{},
		},
	}
}

func injectProxyEnv(envs *[]corev1.EnvVar) {
	upsertEnv(envs, corev1.EnvVar{Name: "HTTP_PROXY", Value: ProxyURL})
	upsertEnv(envs, corev1.EnvVar{Name: "http_proxy", Value: ProxyURL})
}

func upsertEnv(envs *[]corev1.EnvVar, env corev1.EnvVar) {
	for i := range *envs {
		if (*envs)[i].Name == env.Name {
			(*envs)[i] = env
			return
		}
	}
	*envs = append(*envs, env)
}

func resolveApplication(pod *corev1.Pod) string {
	if pod.Annotations != nil {
		if app := strings.TrimSpace(pod.Annotations[ApplicationKey]); app != "" {
			return app
		}
	}
	labelKeys := []string{ApplicationKey, "app.kubernetes.io/name", "app", "k8s-app"}
	for _, key := range labelKeys {
		if pod.Labels != nil {
			if app := strings.TrimSpace(pod.Labels[key]); app != "" {
				return app
			}
		}
	}
	if name := strings.TrimSuffix(strings.TrimSpace(pod.GenerateName), "-"); name != "" {
		return name
	}
	if name := strings.TrimSpace(pod.Name); name != "" {
		return name
	}
	return "unknown"
}

func parseCSV(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func hasContainer(pod *corev1.Pod, name string) bool {
	for _, c := range pod.Spec.Containers {
		if c.Name == name {
			return true
		}
	}
	return false
}

func isEnabled(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "yes", "enabled", "on":
		return true
	default:
		return false
	}
}

func boolPtr(v bool) *bool { return &v }
