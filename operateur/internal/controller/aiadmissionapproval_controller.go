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

	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
	"github.com/imperium/ai-sovereign-finops-operator/internal/govar"
)

// AIAdmissionApprovalReconciler is the sole writer of verified approval and
// consumption status. Proposal and human-decision principals cannot write it.
type AIAdmissionApprovalReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Now    func() time.Time
}

//+kubebuilder:rbac:groups=aiops.imperium.io,resources=aiadmissionapprovals,verbs=get;list;watch
//+kubebuilder:rbac:groups=aiops.imperium.io,resources=aiadmissionapprovals/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=aiops.imperium.io,resources=aiadmissionapprovaldecisions,verbs=get;list;watch
//+kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch

func (r *AIAdmissionApprovalReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var proposal aiopsv1alpha1.AIAdmissionApproval
	if err := r.Get(ctx, req.NamespacedName, &proposal); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	now := time.Now().UTC()
	if r.Now != nil {
		now = r.Now().UTC()
	}
	request := &proposal.Spec.Request
	proposal.Status.ObservedGeneration = proposal.Generation
	proposal.Status.RequestDigest = request.RequestDigest

	if request.Namespace != proposal.Namespace || request.RequestDigest == "" || request.RequestDigest != request.ComputeDigest() {
		r.setApprovalPhase(&proposal, aiopsv1alpha1.AIAdmissionApprovalPhaseFailed, "immutable proposal namespace or digest is invalid")
		return ctrl.Result{}, r.Status().Update(ctx, &proposal)
	}
	var decision aiopsv1alpha1.AIAdmissionApprovalDecision
	err := r.Get(ctx, req.NamespacedName, &decision)
	if apierrors.IsNotFound(err) {
		if !now.Before(request.ExpiresAt.Time) {
			r.setApprovalPhase(&proposal, aiopsv1alpha1.AIAdmissionApprovalPhaseExpired, "request-level approval expired without a human decision")
			return ctrl.Result{}, r.Status().Update(ctx, &proposal)
		}
		r.setApprovalPhase(&proposal, aiopsv1alpha1.AIAdmissionApprovalPhasePending, "waiting for separately authorized human decision")
		return ctrl.Result{RequeueAfter: durationUntil(now, request.ExpiresAt.Time)}, r.Status().Update(ctx, &proposal)
	}
	if err != nil {
		return ctrl.Result{}, err
	}
	if decision.Name != proposal.Name || decision.Namespace != proposal.Namespace ||
		decision.Spec.ProposalName != proposal.Name || decision.Spec.ProposalUID != proposal.UID ||
		decision.Spec.RequestDigest != request.RequestDigest {
		r.setApprovalPhase(&proposal, aiopsv1alpha1.AIAdmissionApprovalPhaseFailed, "human decision does not bind the exact proposal UID and digest")
		return ctrl.Result{}, r.Status().Update(ctx, &proposal)
	}
	proposal.Status.DecisionResourceUID = decision.UID
	proposal.Status.DecisionResourceGeneration = decision.Generation
	if decision.Spec.Decision == aiopsv1alpha1.AIAdmissionApprovalDecisionRejected {
		r.setApprovalPhase(&proposal, aiopsv1alpha1.AIAdmissionApprovalPhaseRejected, "request-level admission rejected by reviewer")
		return ctrl.Result{}, r.Status().Update(ctx, &proposal)
	}
	if decision.Spec.Decision != aiopsv1alpha1.AIAdmissionApprovalDecisionApproved {
		r.setApprovalPhase(&proposal, aiopsv1alpha1.AIAdmissionApprovalPhaseFailed, "human decision value is invalid")
		return ctrl.Result{}, r.Status().Update(ctx, &proposal)
	}

	var lease coordinationv1.Lease
	leaseKey := client.ObjectKey{Namespace: proposal.Namespace, Name: aiopsv1alpha1.AdmissionApprovalConsumptionName(proposal.UID)}
	err = r.Get(ctx, leaseKey, &lease)
	if err == nil {
		if proposal.Status.Phase == aiopsv1alpha1.AIAdmissionApprovalPhaseApproved && proposal.Status.ApprovedAt != nil {
			if !validApprovalConsumptionLease(&lease, &proposal) {
				r.setApprovalPhase(&proposal, aiopsv1alpha1.AIAdmissionApprovalPhaseFailed, "consumption Lease does not bind the approved proposal UID and digest")
				return ctrl.Result{}, r.Status().Update(ctx, &proposal)
			}
			consumedAt := metav1.NewTime(now)
			if lease.Spec.AcquireTime != nil {
				consumedAt = metav1.NewTime(lease.Spec.AcquireTime.Time)
			}
			proposal.Status.ConsumedAt = &consumedAt
			proposal.Status.ConsumedByRequestID = request.RequestID
			r.setApprovalPhase(&proposal, aiopsv1alpha1.AIAdmissionApprovalPhaseConsumed, "one-use approval was atomically consumed")
			return ctrl.Result{}, r.Status().Update(ctx, &proposal)
		}
	} else if !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}
	if !now.Before(request.ExpiresAt.Time) {
		r.setApprovalPhase(&proposal, aiopsv1alpha1.AIAdmissionApprovalPhaseExpired, "request-level approval expired before consumption")
		return ctrl.Result{}, r.Status().Update(ctx, &proposal)
	}
	if err := r.validateApprovalReferences(ctx, proposal.Namespace, request); err != nil {
		r.setApprovalPhase(&proposal, aiopsv1alpha1.AIAdmissionApprovalPhaseFailed, err.Error())
		return ctrl.Result{}, r.Status().Update(ctx, &proposal)
	}
	if proposal.Status.ApprovedAt == nil {
		approvedAt := metav1.NewTime(now)
		proposal.Status.ApprovedAt = &approvedAt
	}
	r.setApprovalPhase(&proposal, aiopsv1alpha1.AIAdmissionApprovalPhaseApproved, "one immutable request is approved and awaiting one-use consumption")
	return ctrl.Result{RequeueAfter: durationUntil(now, request.ExpiresAt.Time)}, r.Status().Update(ctx, &proposal)
}

func durationUntil(now, deadline time.Time) time.Duration {
	d := deadline.Sub(now)
	if d <= 0 {
		return time.Millisecond
	}
	return d
}

func validApprovalConsumptionLease(lease *coordinationv1.Lease, proposal *aiopsv1alpha1.AIAdmissionApproval) bool {
	if lease.Spec.HolderIdentity == nil || *lease.Spec.HolderIdentity != proposal.Spec.Request.RequestDigest ||
		lease.Spec.AcquireTime == nil || !lease.Spec.AcquireTime.Time.Before(proposal.Spec.Request.ExpiresAt.Time) {
		return false
	}
	for _, owner := range lease.OwnerReferences {
		if owner.APIVersion == aiopsv1alpha1.GroupVersion.String() && owner.Kind == "AIAdmissionApproval" &&
			owner.Name == proposal.Name && owner.UID == proposal.UID {
			return true
		}
	}
	return false
}

func (r *AIAdmissionApprovalReconciler) validateApprovalReferences(ctx context.Context, namespace string, request *aiopsv1alpha1.AIAdmissionApprovalRequest) error {
	var budget aiopsv1alpha1.AIBudgetPolicy
	if err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: request.BudgetPolicy.Name}, &budget); err != nil || budget.UID != request.BudgetPolicy.UID || budget.Generation != request.BudgetPolicy.Generation ||
		budget.Status.ObservedGeneration != budget.Generation || !apimeta.IsStatusConditionTrue(budget.Status.Conditions, aiopsv1alpha1.ConditionReady) {
		return fmt.Errorf("approved budget policy identity is stale or not Ready")
	}
	var routing aiopsv1alpha1.AIRoutingPolicy
	if err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: request.RoutingPolicy.Name}, &routing); err != nil || routing.UID != request.RoutingPolicy.UID || routing.Generation != request.RoutingPolicy.Generation ||
		routing.Status.ObservedGeneration != routing.Generation || !apimeta.IsStatusConditionTrue(routing.Status.Conditions, aiopsv1alpha1.ConditionReady) {
		return fmt.Errorf("approved routing policy identity is stale or not Ready")
	}
	var model aiopsv1alpha1.AIModel
	if err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: request.CandidateModelRef}, &model); err != nil {
		return fmt.Errorf("approved model route is stale or not routable")
	}
	var provider aiopsv1alpha1.AIProvider
	if err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: model.Spec.ProviderRef}, &provider); err != nil {
		return fmt.Errorf("approved candidate snapshot is stale")
	}
	candidates := govar.BuildCandidates(govar.RequestContext{Namespace: request.Namespace, Team: request.Team, Application: request.Application, SensitiveData: request.SensitiveData, AllowedZones: request.AllowedZones}, []aiopsv1alpha1.AIModel{model}, map[string]aiopsv1alpha1.AIProvider{provider.Name: provider})
	if len(candidates) != 1 || !candidates[0].Feasible || candidates[0].SnapshotVersion != request.CandidateSnapshotVersion || request.CandidateSnapshotVersion != request.RouteSnapshot.SnapshotHash || candidates[0].RouteSnapshot != request.RouteSnapshot {
		return fmt.Errorf("approved provider-owned route snapshot is stale or invalid")
	}
	return nil
}

func (r *AIAdmissionApprovalReconciler) setApprovalPhase(proposal *aiopsv1alpha1.AIAdmissionApproval, phase aiopsv1alpha1.AIAdmissionApprovalPhase, message string) {
	proposal.Status.Phase = phase
	proposal.Status.Message = message
	conditionStatus := metav1.ConditionTrue
	if phase == aiopsv1alpha1.AIAdmissionApprovalPhaseFailed || phase == aiopsv1alpha1.AIAdmissionApprovalPhaseExpired || phase == aiopsv1alpha1.AIAdmissionApprovalPhaseRejected {
		conditionStatus = metav1.ConditionFalse
	}
	reason := aiopsv1alpha1.ReasonReconciled
	if phase == aiopsv1alpha1.AIAdmissionApprovalPhaseFailed {
		reason = aiopsv1alpha1.ReasonReconcileError
	}
	apimeta.SetStatusCondition(&proposal.Status.Conditions, readyCondition(proposal.Generation, conditionStatus, reason, message))
}

func (r *AIAdmissionApprovalReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&aiopsv1alpha1.AIAdmissionApproval{}).
		Watches(&aiopsv1alpha1.AIAdmissionApprovalDecision{}, &handler.EnqueueRequestForObject{}).
		Watches(&coordinationv1.Lease{}, handler.EnqueueRequestsFromMapFunc(func(_ context.Context, object client.Object) []ctrl.Request {
			for _, owner := range object.GetOwnerReferences() {
				if owner.APIVersion == aiopsv1alpha1.GroupVersion.String() && owner.Kind == "AIAdmissionApproval" {
					return []ctrl.Request{{NamespacedName: client.ObjectKey{Namespace: object.GetNamespace(), Name: owner.Name}}}
				}
			}
			return nil
		})).
		Complete(r)
}
