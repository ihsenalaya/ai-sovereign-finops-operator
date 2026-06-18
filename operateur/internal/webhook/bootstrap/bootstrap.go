package bootstrap

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	admissionregv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Options defines how the operator self-registers its mutating webhook.
type Options struct {
	Client           client.Client
	Name             string
	ServiceName      string
	ServiceNamespace string
	Path             string
	CertDir          string
	FailurePolicy    admissionregv1.FailurePolicyType
}

// Ensure prepares serving certs and self-registers the mutating webhook.
func Ensure(ctx context.Context, opts Options) error {
	if opts.Client == nil {
		return fmt.Errorf("webhook bootstrap requires a client")
	}
	if opts.Name == "" {
		return fmt.Errorf("webhook bootstrap name is required")
	}
	if opts.ServiceName == "" || opts.ServiceNamespace == "" {
		return fmt.Errorf("webhook bootstrap service name/namespace are required")
	}
	if opts.Path == "" {
		opts.Path = "/mutate-v1-pod"
	}
	if opts.CertDir == "" {
		opts.CertDir = filepath.Join(os.TempDir(), "k8s-webhook-server", "serving-certs")
	}
	if opts.FailurePolicy == "" {
		opts.FailurePolicy = admissionregv1.Ignore
	}

	caPEM, certPEM, keyPEM, err := generateServingBundle(opts.ServiceName, opts.ServiceNamespace)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(opts.CertDir, 0o755); err != nil {
		return fmt.Errorf("create webhook cert dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(opts.CertDir, "tls.crt"), certPEM, 0o600); err != nil {
		return fmt.Errorf("write webhook cert: %w", err)
	}
	if err := os.WriteFile(filepath.Join(opts.CertDir, "tls.key"), keyPEM, 0o600); err != nil {
		return fmt.Errorf("write webhook key: %w", err)
	}

	cfg := &admissionregv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: opts.Name},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, opts.Client, cfg, func() error {
		path := opts.Path
		port := int32(443)
		failure := opts.FailurePolicy
		sideEffects := admissionregv1.SideEffectClassNone
		timeout := int32(5)
		matchPolicy := admissionregv1.Equivalent
		reinvocation := admissionregv1.NeverReinvocationPolicy
		cfg.Webhooks = []admissionregv1.MutatingWebhook{{
			Name:                    "sidecar-injection.aiops.imperium.io",
			AdmissionReviewVersions: []string{"v1"},
			SideEffects:             &sideEffects,
			FailurePolicy:           &failure,
			TimeoutSeconds:          &timeout,
			MatchPolicy:             &matchPolicy,
			ReinvocationPolicy:      &reinvocation,
			ClientConfig: admissionregv1.WebhookClientConfig{
				CABundle: caPEM,
				Service: &admissionregv1.ServiceReference{
					Name:      opts.ServiceName,
					Namespace: opts.ServiceNamespace,
					Path:      &path,
					Port:      &port,
				},
			},
			Rules: []admissionregv1.RuleWithOperations{{
				Operations: []admissionregv1.OperationType{admissionregv1.Create},
				Rule: admissionregv1.Rule{
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
					Resources:   []string{"pods"},
				},
			}},
		}}
		return nil
	})
	if err != nil {
		return fmt.Errorf("upsert mutating webhook configuration: %w", err)
	}
	return nil
}

func generateServingBundle(serviceName, namespace string) (caPEM, certPEM, keyPEM []byte, err error) {
	now := time.Now().UTC()

	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("generate webhook CA key: %w", err)
	}
	caSerial, err := randSerial()
	if err != nil {
		return nil, nil, nil, err
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          caSerial,
		Subject:               pkix.Name{CommonName: "greenops-webhook-ca"},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create webhook CA cert: %w", err)
	}

	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("generate webhook server key: %w", err)
	}
	serverSerial, err := randSerial()
	if err != nil {
		return nil, nil, nil, err
	}
	serverTemplate := &x509.Certificate{
		SerialNumber: serverSerial,
		Subject:      pkix.Name{CommonName: fmt.Sprintf("%s.%s.svc", serviceName, namespace)},
		NotBefore:    now.Add(-5 * time.Minute),
		NotAfter:     now.Add(90 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames: []string{
			serviceName,
			fmt.Sprintf("%s.%s", serviceName, namespace),
			fmt.Sprintf("%s.%s.svc", serviceName, namespace),
			fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace),
		},
	}
	serverDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caTemplate, &serverKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create webhook serving cert: %w", err)
	}

	caPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverDER})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serverKey)})
	return caPEM, certPEM, keyPEM, nil
}

func randSerial() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, fmt.Errorf("generate certificate serial: %w", err)
	}
	return serial, nil
}
