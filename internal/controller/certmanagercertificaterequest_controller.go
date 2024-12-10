/*
Copyright 2024.

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
	certManagerApi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmanagerv1 "github.com/jquad-group/est-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	issueRefNameField = ".spec.issueref.name"
)

// CertManagerCertificateRequestReconciler reconciles a CertManagerCertificateRequest object
type CertManagerCertificateRequestReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=certmanager.jquad.rocks,resources=certmanagercertificaterequests,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=certmanager.jquad.rocks,resources=certmanagercertificaterequests/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=certmanager.jquad.rocks,resources=certmanagercertificaterequests/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the CertManagerCertificateRequest object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.2/pkg/reconcile
func (r *CertManagerCertificateRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	var certificateRequest certManagerApi.CertificateRequest
	if err := r.Get(ctx, req.NamespacedName, &certificateRequest); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	patch := &unstructured.Unstructured{}
	patch.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "cert-manager.io",
		Version: "v1",
		Kind:    "CertificateRequest",
	})
	patch.SetNamespace(certificateRequest.GetNamespace())
	patch.SetName(certificateRequest.GetName())
	patchOptions := &client.PatchOptions{
		FieldManager: "certificaterequest-controller",
		Force:        pointer.Bool(true),
	}

	subPatchOptions := &client.SubResourcePatchOptions{
		PatchOptions: *patchOptions,
	}

	// check if the referenced issuer in the certificate requests is ready
	var issuer certmanagerv1.EstIssuer
	if err := r.Get(ctx, types.NamespacedName{Name: certificateRequest.Spec.IssuerRef.Name, Namespace: certificateRequest.Namespace}, &issuer); err != nil {
		return ctrl.Result{}, err
	}
	if !issuer.Status.Ready {
		return ctrl.Result{}, fmt.Errorf("issuer %s is not ready", issuer.Name)
	}

	// create est order
	estOrder := certmanagerv1.EstOrder{
		ObjectMeta: metav1.ObjectMeta{
			Name:      certificateRequest.Name,
			Namespace: certificateRequest.Namespace,
		},
		Spec: certmanagerv1.EstOrderSpec{
			IssuerRef: certmanagerv1.IssuerRef{
				Kind:  certificateRequest.Spec.IssuerRef.Kind,
				Group: certificateRequest.Spec.IssuerRef.Group,
				Name:  certificateRequest.Spec.IssuerRef.Name,
			},
			Request: certificateRequest.Spec.Request,
			Renewal: false,
		},
	}

	if err := r.Patch(ctx, &estOrder, client.Apply, subPatchOptions); err != nil {
		return ctrl.Result{}, err
	}

	// set owner reference
	if err := ctrl.SetControllerReference(&certificateRequest, &estOrder, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	// Update the status of the CertificateRequest
	condition := certManagerApi.CertificateRequestCondition{
		Type:               certManagerApi.CertificateRequestConditionReady,
		Status:             "False",
		Reason:             "Pending",
		Message:            "Created new EstOrder " + estOrder.Name,
		LastTransitionTime: &metav1.Time{Time: time.Now()},
	}
	certificateRequest.Status.Conditions = append(certificateRequest.Status.Conditions, condition)

	patch.UnstructuredContent()["status"] = certificateRequest.Status
	if err := r.Status().Patch(ctx, patch, client.Apply, subPatchOptions); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CertManagerCertificateRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &certManagerApi.CertificateRequest{}, issueRefNameField, func(rawObj client.Object) []string {
		// Extract the issue ref from the CertificateRequest Spec
		certificateRequest := rawObj.(*certManagerApi.CertificateRequest)
		if certificateRequest.Spec.IssuerRef.Kind != "EstIssuer" {
			return nil
		}
		return []string{certificateRequest.Spec.IssuerRef.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&certManagerApi.CertificateRequest{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(
			&certManagerApi.CertificateRequest{},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForEstIssuer),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Complete(r)
}

func (r *CertManagerCertificateRequestReconciler) findObjectsForEstIssuer(ctx context.Context, source client.Object) []reconcile.Request {
	attachedCertificates := &certmanagerv1.EstIssuerList{}
	listOps := &client.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(issueRefNameField, source.GetName()),
		Namespace:     source.GetNamespace(),
	}
	err := r.List(context.TODO(), attachedCertificates, listOps)
	if err != nil {
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, len(attachedCertificates.Items))
	for i, item := range attachedCertificates.Items {
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      item.GetName(),
				Namespace: item.GetNamespace(),
			},
		}
	}
	return requests
}
