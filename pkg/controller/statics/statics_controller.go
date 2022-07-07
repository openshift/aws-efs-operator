package statics

/**
"Statics" are the static (unchanging) resources associated with the CSI driver.
This controller knows how to bootstrap these objects and watch them for changes
(though that should never happen).
*/

import (
	"context"
	"openshift/aws-efs-operator/pkg/util"
	"time"

	"github.com/go-logr/logr"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const svCRDName = "sharedvolumes.aws-efs.managed.openshift.io"

var log = logf.Log.WithName("controller_statics")

// Add creates a new Statics Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileStatics{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("statics-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Add watches for each of our static resources.
	for _, t := range staticResources {
		err = c.Watch(&source.Kind{Type: t.GetType()}, &handler.EnqueueRequestForObject{}, util.ICarePredicate)
		if err != nil {
			return err
		}
	}

	return nil
}

// blank assignment to verify that ReconcileStatics implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileStatics{}

// ReconcileStatics is a reconcile.Reconciler providing access to a Client and Scheme
type ReconcileStatics struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for static objects and makes changes based on the state read
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileStatics) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)

	var (
		crd *apiextensions.CustomResourceDefinition
		err error
	)

	// We got this far, so it's a type we're watching that also passed our filter. That means it
	// ought to be one of our statics.
	s := findStatic(request.NamespacedName)
	if s == nil {
		// This should really never happen.
		reqLogger.Error(nil, "Got an unexpected reconcile request.", "request", request)
		// Don't requeue this one, either explicitly (Requeue=true) or implicitly (by returning an error)
		return reconcile.Result{}, nil
	}

	// Find the SharedVolume CRD, which will be set up to "own" the statics. This is the mechanism
	// by which the statics get cleaned up when the operator is uninstalled.
	// TODO(efried): Except for the SCC, which for some reason seems to ignore the OwnerReferences
	// and not get deleted. This may be an upstream bug.
	// See https://github.com/openshift/aws-efs-operator/issues/23
	if crd, err = discoverCRD(r.client); err != nil {
		if errors.IsNotFound(err) {
			// TODO(efried): Delete when https://github.com/openshift/aws-efs-operator/issues/23 is resolved.
			deleteSCC(reqLogger, r.client)
			reqLogger.Info("SharedVolume CRD has already been deleted. Skipping reconcile, awaiting demise.")
			return reconcile.Result{}, nil
		}
		reqLogger.Error(err, "Couldn't retrieve SharedVolume CRD")
		// Not sure under what circumstances this could happen, but... requeue after one second
		return reconcile.Result{Requeue: true, RequeueAfter: time.Millisecond * 1000}, nil
	}
	// If the CRD is being deleted, it means we're shutting down. Bail out to avoid thrashing (the
	// deletion of the CRD triggers deletion of the statics, which we would otherwise try to
	// restore below).
	if crd.GetDeletionTimestamp() != nil {
		// TODO(efried): Delete when https://github.com/openshift/aws-efs-operator/issues/23 is resolved.
		deleteSCC(reqLogger, r.client)
		reqLogger.Info("The SharedVolume CRD is being deleted, which means we're shutting down. Skipping reconcile.")
		return reconcile.Result{}, nil
	}
	reqLogger.Info("Reconciling.", "request", request)

	// Make sure the static is "owned" by the CRD.
	// We need to do this here because we can't count on the CRD existing during static setup.
	s.SetOwner(util.AsOwner(crd))

	if err := s.Ensure(reqLogger, r.client); err != nil {
		// TODO: Max retries so we don't get in a hard loop when the failure is something incurable?
		return reconcile.Result{Requeue: true}, err
	}
	return reconcile.Result{}, nil
}

// discoverCRD finds our SharedVolume CustomResourceDefinition.
func discoverCRD(client crclient.Client) (*apiextensions.CustomResourceDefinition, error) {
	crd := &apiextensions.CustomResourceDefinition{}
	nsn := types.NamespacedName{
		Name: svCRDName,
	}
	if err := client.Get(context.TODO(), nsn, crd); err != nil {
		return nil, err
	}
	return crd, nil
}

// deleteSCC deletes the SecurityContextConstraints static.
// TODO(efried): This is a *workaround* for https://github.com/openshift/aws-efs-operator/issues/23
// It should be deleted when that issue is resolved (upstream, or here in some better way).
func deleteSCC(logger logr.Logger, client crclient.Client) {
	logger.Info("Manually deleting SecurityContextConstraints. See https://github.com/openshift/aws-efs-operator/issues/23")
	scce := findStatic(types.NamespacedName{Name: sccName})
	// Delete() does the logging. We're ignoring any errors.
	_ = scce.Delete(logger, client)
}
