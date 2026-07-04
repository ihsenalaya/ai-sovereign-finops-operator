package controller

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
)

type AttestationEvidenceReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=aiops.imperium.io,resources=attestationevidences,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=aiops.imperium.io,resources=attestationevidences/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=aiops.imperium.io,resources=attestationevidences/finalizers,verbs=update
//+kubebuilder:rbac:groups=aiops.imperium.io,resources=airevocationpolicies,verbs=get;list;watch

func (r *AttestationEvidenceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var evidence aiopsv1alpha1.AttestationEvidence
	if err := r.Get(ctx, req.NamespacedName, &evidence); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	revoked, err := r.isEvidenceRevoked(ctx, &evidence)
	if err != nil {
		return ctrl.Result{}, err
	}
	evidence.Status.ObservedGeneration = evidence.Generation
	evidence.Status.Revoked = revoked
	evidence.Status.Verified = !revoked && (evidence.Spec.Simulated || evidence.Spec.Digest != "" || len(evidence.Spec.Signatures) > 0)
	if evidence.Status.Verified {
		now := metav1.NewTime(time.Now().UTC())
		evidence.Status.LastVerifiedTime = &now
	}
	if evidence.Status.Verified {
		meta.SetStatusCondition(&evidence.Status.Conditions, readyTrue(evidence.Generation, "Attestation evidence verified"))
	} else {
		reason := aiopsv1alpha1.ReasonValidationFailed
		message := "attestation evidence is not yet verified"
		if revoked {
			message = "attestation evidence has been revoked"
		}
		meta.SetStatusCondition(&evidence.Status.Conditions, readyFalse(evidence.Generation, reason, message))
	}
	if err := r.Status().Update(ctx, &evidence); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *AttestationEvidenceReconciler) isEvidenceRevoked(ctx context.Context, evidence *aiopsv1alpha1.AttestationEvidence) (bool, error) {
	var policies aiopsv1alpha1.AIRevocationPolicyList
	if err := r.List(ctx, &policies, client.InNamespace(evidence.Namespace)); err != nil {
		return false, err
	}
	now := time.Now().UTC()
	for i := range policies.Items {
		policy := policies.Items[i]
		if policy.Spec.EvidenceRef == nil || policy.Spec.EvidenceRef.Name != evidence.Name {
			continue
		}
		if policy.Status.ExpiresAt != nil && policy.Status.ExpiresAt.Time.Before(now) {
			continue
		}
		return true, nil
	}
	return false, nil
}

func (r *AttestationEvidenceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&aiopsv1alpha1.AttestationEvidence{}).
		Complete(r)
}
