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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
)

// AIModelReconciler reconciles a AIModel object.
type AIModelReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=aiops.imperium.io,resources=aimodels,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=aiops.imperium.io,resources=aimodels/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=aiops.imperium.io,resources=aimodels/finalizers,verbs=update
//+kubebuilder:rbac:groups=aiops.imperium.io,resources=aiproviders,verbs=get;list;watch

// Reconcile validates that the referenced AIProvider exists and records readiness.
func (r *AIModelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var model aiopsv1alpha1.AIModel
	if err := r.Get(ctx, req.NamespacedName, &model); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	model.Status.ObservedGeneration = model.Generation

	var provider aiopsv1alpha1.AIProvider
	provKey := types.NamespacedName{Namespace: model.Namespace, Name: model.Spec.ProviderRef}
	if err := r.Get(ctx, provKey, &provider); err != nil {
		if apierrors.IsNotFound(err) {
			model.Status.ResolvedProvider = ""
			meta.SetStatusCondition(&model.Status.Conditions, readyFalse(
				model.Generation, aiopsv1alpha1.ReasonReferenceNotFound,
				fmt.Sprintf("AIProvider %q not found", model.Spec.ProviderRef)))
			if err := r.Status().Update(ctx, &model); err != nil {
				return ctrl.Result{}, err
			}
			// Not a hard error: the provider may be created later, which will
			// trigger a re-reconcile through the watch.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	model.Status.ResolvedProvider = provider.Spec.Type
	meta.SetStatusCondition(&model.Status.Conditions,
		readyTrue(model.Generation, "AIModel catalogued and provider resolved"))

	if err := r.Status().Update(ctx, &model); err != nil {
		return ctrl.Result{}, err
	}
	logger.V(1).Info("reconciled AIModel", "provider", provider.Spec.Type)
	return ctrl.Result{}, nil
}

// modelsForProvider maps an AIProvider event to reconcile requests for every
// AIModel in the same namespace that references it. This ensures a model that
// was created before its provider becomes Ready once the provider appears.
func (r *AIModelReconciler) modelsForProvider(ctx context.Context, obj client.Object) []reconcile.Request {
	var models aiopsv1alpha1.AIModelList
	if err := r.List(ctx, &models, client.InNamespace(obj.GetNamespace())); err != nil {
		return nil
	}
	var reqs []reconcile.Request
	for i := range models.Items {
		if models.Items[i].Spec.ProviderRef == obj.GetName() {
			reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: models.Items[i].Namespace,
				Name:      models.Items[i].Name,
			}})
		}
	}
	return reqs
}

// SetupWithManager sets up the controller with the Manager.
func (r *AIModelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&aiopsv1alpha1.AIModel{}).
		Watches(&aiopsv1alpha1.AIProvider{}, handler.EnqueueRequestsFromMapFunc(r.modelsForProvider)).
		Complete(r)
}
