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
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"

	estClient "github.com/globalsign/est"
	"github.com/go-logr/logr"
	certmanagerv1 "github.com/jquad-group/est-operator/api/v1"
)

// EstIssuerReconciler reconciles a EstIssuer object
type EstIssuerReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=certmanager.jquad.rocks,resources=estissuers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=certmanager.jquad.rocks,resources=estissuers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=certmanager.jquad.rocks,resources=estissuers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the EstIssuer object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.2/pkg/reconcile
func (r *EstIssuerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("estissuer", req.NamespacedName)

	// Fetch the ESTIssuer resource
	var issuer certmanagerv1.EstIssuer
	if err := r.Get(ctx, req.NamespacedName, &issuer); err != nil {
		log.Error(err, "Failed to get ESTIssuer resource")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	patch := &unstructured.Unstructured{}
	patch.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   certmanagerv1.GroupVersion.Group,
		Version: certmanagerv1.GroupVersion.Version,
		Kind:    "EstIssuer",
	})
	patch.SetNamespace(issuer.GetNamespace())
	patch.SetName(issuer.GetName())
	patchOptions := &client.PatchOptions{
		FieldManager: "estissuer-controller",
		Force:        pointer.Bool(true),
	}

	subPatchOptions := &client.SubResourcePatchOptions{
		PatchOptions: *patchOptions,
	}

	// Fetch the referenced secret
	var secret corev1.Secret
	if err := r.Get(ctx, client.ObjectKey{Namespace: issuer.Namespace, Name: issuer.Spec.AuthSecretName}, &secret); err != nil {
		log.Error(err, "Referenced secret not found")
		return ctrl.Result{}, err
	}

	// Decode CA certificate
	explicitAnchor, err := base64.StdEncoding.DecodeString(issuer.Spec.Cacert)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("Failed to decode 'cacert': %v", err)
	}
	explicitAnchorCertPool, _ := ConvertToCertPool(explicitAnchor)
	fmt.Println(explicitAnchorCertPool)

	// Fetch /cacerts endpoint
	myEstClient := estClient.Client{
		Host:                  issuer.Spec.Hostname + ":" + strconv.Itoa(issuer.Spec.Port),
		AdditionalPathSegment: issuer.Spec.Label,
		ExplicitAnchor:        explicitAnchorCertPool,
		HostHeader:            "",
		Username:              "",
		Password:              "",
		DisableKeepAlives:     false,
		InsecureSkipVerify:    false,
	}

	// get and verify ca bundle
	_, err = myEstClient.CACerts(ctx)
	if err != nil {
		issuer.Status.Ready = false
		patch.UnstructuredContent()["status"] = issuer.Status
		r.Status().Patch(ctx, patch, client.Apply, subPatchOptions)
		return ctrl.Result{}, fmt.Errorf("Failed to get or verify 'cacert': %v", err)
	}

	// Update status
	issuer.Status.Ready = true
	patch.UnstructuredContent()["status"] = issuer.Status
	if err := r.Status().Patch(ctx, patch, client.Apply, subPatchOptions); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Successfully reconciled ESTIssuer", "name", req.NamespacedName)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EstIssuerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&certmanagerv1.EstIssuer{}).
		Complete(r)
}

// ConvertToCertPool converts a PEM-encoded byte slice into an x509.CertPool
func ConvertToCertPool(pemData []byte) (*x509.CertPool, error) {
	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(pemData); !ok {
		return nil, errors.New("failed to append certificates to cert pool")
	}
	return certPool, nil
}
