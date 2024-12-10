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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	certManagerApi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/go-logr/logr"
	certmanagerv1 "github.com/jquad-group/est-operator/api/v1"
)

// EstOrderReconciler reconciles a EstOrder object
type EstOrderReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=certmanager.jquad.rocks,resources=estorders,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=certmanager.jquad.rocks,resources=estorders/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=certmanager.jquad.rocks,resources=estorders/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the EstOrder object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.2/pkg/reconcile
func (r *EstOrderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("estorder", req.NamespacedName)

	var certificateRequest certManagerApi.CertificateRequest
	if err := r.Get(ctx, req.NamespacedName, &certificateRequest); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// check if the referenced issuer in the certificate requests is ready
	var issuer certmanagerv1.EstIssuer
	if err := r.Get(ctx, types.NamespacedName{Name: certificateRequest.Spec.IssuerRef.Name, Namespace: certificateRequest.Namespace}, &issuer); err != nil {
		return ctrl.Result{}, err
	}
	if !issuer.Status.Ready {
		return ctrl.Result{}, fmt.Errorf("issuer %s is not ready", issuer.Name)
	}

	// create EstOrder
	var estOrder certmanagerv1.EstOrder
	if err := r.Get(ctx, req.NamespacedName, &estOrder); err != nil {
		log.Error(err, "unable to fetch ESTOrder")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	/*
		issuer, err := getIssuerFromResource(r.Client, estOrder.Spec.IssuerRef, estOrder.Namespace)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to get issuer: %w", err)
		}

		secret, err := getSecretFromResource(r.Client, issuer.Spec.AuthSecretName, issuer.Namespace)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to get secret: %w", err)
		}

		certReq, err := getOwnerByKind(r.Client, &estOrder.ObjectMeta, "CertificateRequest")
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to get certificate request: %w", err)
		}

		caCert, err := base64.StdEncoding.DecodeString(issuer.Spec.Cacert)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to decode CA certificate: %w", err)
		}

		request, err := base64.StdEncoding.DecodeString(estOrder.Spec.Request)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to decode request: %w", err)
		}

		baseURL := fmt.Sprintf("https://%s:%d", issuer.Spec.Hostname, issuer.Spec.Port)
		path := filepath.Join("/.well-known/est", issuer.Spec.Label)

		var httpClient *http.Client
		if estOrder.Spec.Renewal {
			path = filepath.Join(path, "simplereenroll")
			tlsSecret, err := getSecretFromResource(r.Client, certReq.Spec.SecretRef, certReq.Namespace)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("request is renewal but TLS secret is missing: %w", err)
			}

			httpClient, err = createTLSClient(caCert, &tlsSecret)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("unable to create TLS client: %w", err)
			}
		} else {
			path = filepath.Join(path, "simpleenroll")
			username, err := base64.StdEncoding.DecodeString(string(secret.Data["username"]))
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("unable to decode username: %w", err)
			}
			password, err := base64.StdEncoding.DecodeString(string(secret.Data["password"]))
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("unable to decode password: %w", err)
			}
			httpClient = createBasicAuthClient(caCert, string(username), string(password))
		}

		reqURL := baseURL + path
		resp, err := httpClient.Post(reqURL, "application/pkcs10", strings.NewReader(string(request)))
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()

		log.Info(fmt.Sprintf("EST order response: %d %s", resp.StatusCode, resp.Status))

		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return ctrl.Result{RequeueAfter: 10 * time.Minute}, fmt.Errorf("request problem: %s", resp.Status)
		} else if resp.StatusCode >= 500 {
			return ctrl.Result{RequeueAfter: 10 * time.Minute}, fmt.Errorf("server problem: %s", resp.Status)
		} else if resp.StatusCode == http.StatusAccepted {
			retryAfter := resp.Header.Get("Retry-After")
			retryDuration, err := time.ParseDuration(retryAfter)
			if err != nil {
				retryAfterSeconds, _ := strconv.Atoi(retryAfter)
				retryDuration = time.Duration(retryAfterSeconds) * time.Second
			}
			return ctrl.Result{RequeueAfter: retryDuration}, nil
		} else if resp.StatusCode == http.StatusOK {
			// Handle successful response
			// Update status of certReq
		}

	*/
	return ctrl.Result{}, nil
}

func getIssuerFromResource(client client.Client, ref certmanagerv1.IssuerRef, namespace string) (certmanagerv1.EstIssuer, error) {
	// Implement the function to get the issuer
	return certmanagerv1.EstIssuer{}, nil
}

func getSecretFromResource(client client.Client, ref string, namespace string) (corev1.Secret, error) {
	// Implement the function to get the secret
	return corev1.Secret{}, nil
}

func getOwnerByKind(client client.Client, owner metav1.Object, kind string) (certManagerApi.CertificateRequest, error) {
	// Implement the function to get the owner by kind
	return certManagerApi.CertificateRequest{}, nil
}

func createTLSClient(caCert []byte, tlsSecret *corev1.Secret) (*http.Client, error) {
	// Implement the function to create an HTTP client with TLS config
	return nil, nil
}

func createBasicAuthClient(caCert []byte, username, password string) *http.Client {
	// Implement the function to create an HTTP client with Basic Auth
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EstOrderReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&certmanagerv1.EstOrder{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}
