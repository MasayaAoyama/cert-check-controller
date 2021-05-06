/*
Copyright 2021 amsy810.

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

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/cri-api/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	srev1beta1 "github.com/MasayaAoyama/cert-check-controller/api/v1beta1"
	"github.com/MasayaAoyama/cert-check-controller/controllers/utils"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/adam-lavrik/go-imath/ix"
)

var (
	NumExpired = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "is_expired_certificate",
			Help: "expired certificate",
		},
		[]string{"certcheck", "certificate"},
	)
)

// CertCheckReconciler reconciles a CertCheck object
type CertCheckReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=sre.amsy810.dev,resources=certchecks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sre.amsy810.dev,resources=certchecks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *CertCheckReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("certcheck", req.NamespacedName)

	instance := &srev1beta1.CertCheck{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		r.Log.Info("error is occured", "msg", err.Error())
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		r.Log.Info("error is not notfound", "msg", err.Error())
		// TODO
		// errors.IsNotFound() is not work. maybe version problem?
		return ctrl.Result{}, nil
		// return ctrl.Result{}, err
	}
	r.Log.Info("got certcheck", "name", instance.Name, "namespace", instance.Namespace)

	sList := &v1.SecretList{}
	r.List(ctx, sList, &client.ListOptions{
		Namespace: instance.Namespace,
		Raw: &metav1.ListOptions{
			LabelSelector: metav1.FormatLabelSelector(instance.Spec.Selector),
		},
	})

	exps := []int{}
	certStatuses := []srev1beta1.Certificate{}

	for _, sec := range sList.Items {
		r.Log.Info("got selected Secret by CertCheck", "secret_name", sec.Name, "secret_namespace", sec.Namespace, "name", instance.Name, "namespace", instance.Namespace)
		if sec.Type == v1.SecretTypeTLS {
			notBefore, notAfter, err := utils.GetExpirationDate(&sec)
			if err != nil {
				r.Log.Error(err, "error is occured", "msg", err.Error())
				return ctrl.Result{}, err
			}
			r.Log.Info("got secret expiration date", "secret_name", sec.Name, "secret_namespace", sec.Namespace, "not_before", notBefore, "not_after", notAfter)
			remaining := int(notAfter.Sub(time.Now()).Hours() / 24)
			th := remaining - instance.Spec.Threshold

			isActive := true
			if remaining < 0 {
				r.Recorder.Eventf(&sec, v1.EventTypeWarning, "Expired", "TLS Secret %s/%s is expired at %s", sec.Namespace, sec.Name, notAfter)
				isActive = false
				NumExpired.With(
					prometheus.Labels{
						"certcheck":   instance.Name,
						"certificate": sec.Name,
					}).Set(1)
			} else if th < 0 {
				r.Recorder.Eventf(&sec, v1.EventTypeWarning, "WillBeExpired", "TLS Secret %s/%s will be expired at %s", sec.Namespace, sec.Name, notAfter)
				NumExpired.With(
					prometheus.Labels{
						"certcheck":   instance.Name,
						"certificate": sec.Name,
					}).Set(0.5)
			} else {
				exps = append(exps, remaining)
				NumExpired.With(
					prometheus.Labels{
						"certcheck":   instance.Name,
						"certificate": sec.Name,
					}).Set(0)
			}

			certStatuses = append(certStatuses, srev1beta1.Certificate{
				Name:      sec.Name,
				NotBefore: notBefore,
				NotAfter:  notAfter,
				Active:    isActive,
			})

			sec.Annotations["certcheck.amsy.dev/active"] = fmt.Sprintf("%t", isActive)
			sec.Annotations["certcheck.amsy.dev/notBefore"] = notBefore.String()
			sec.Annotations["certcheck.amsy.dev/notAfter"] = notAfter.String()
			r.Update(context.TODO(), &sec)
		}
	}

	instance.Status = srev1beta1.CertCheckStatus{
		TargetCertsCount: len(certStatuses),
		Certificates:     certStatuses,
	}
	r.Status().Update(context.TODO(), instance)

	if len(certStatuses) == 0 {
		r.Log.Info("no target is found", "name", instance.Name, "namespace", instance.Namespace)
		return ctrl.Result{}, err
	}

	min := ix.MinSlice(exps)

	// waiting time
	waiting := time.Duration(time.Hour * 24 * time.Duration(min-instance.Spec.Threshold))
	r.Log.Info("Requeuing for waiting expiration", "min", min, "threshold", instance.Spec.Threshold, "waiting_time", waiting)

	// TODO
	// re-sync period is 10 hours
	// maybe requeuing over 10 hours is pointless
	return ctrl.Result{
		RequeueAfter: waiting,
	}, nil
}

func (r *CertCheckReconciler) SetupWithManager(mgr ctrl.Manager) error {
	secretMapper := handler.ToRequestsFunc(
		func(a handler.MapObject) []reconcile.Request {
			sec := a.Object.(*v1.Secret)
			ret := []reconcile.Request{}
			r.Log.Info("got secret", "name", sec.Name, "namespace", sec.Namespace)

			ccList := &srev1beta1.CertCheckList{}

			err := r.Client.List(context.TODO(), ccList)
			if err != nil {
				return ret
			}

			for _, cc := range ccList.Items {
				// TODO: queing only for related cert check
				// if cc.Spec.Selector == sec.ObjectMeta.Labels {
				ret = append(ret, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      cc.GetName(),
						Namespace: cc.GetNamespace(),
					}})
				// }
			}

			return ret
		})

	return ctrl.NewControllerManagedBy(mgr).
		For(&srev1beta1.CertCheck{}).
		Watches(
			&source.Kind{Type: &v1.Secret{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: secretMapper,
			},
		).
		Complete(r)
}
