/*
Copyright 2022.

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
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"strings"

	awsefsv1alpha1 "openshift/aws-efs-operator/api/v1alpha1"
	"openshift/aws-efs-operator/pkg/util"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	// TODO: Is there a lib const for this somewhere?
	pvcKind     = "PersistentVolumeClaim"
	svFinalizer = "finalizer.awsefs.managed.openshift.io"
)

var log = logf.Log.WithName("controller_sharedvolume")

// SharedVolumeReconciler reconciles a SharedVolume object
type SharedVolumeReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=aws-efs.managed.openshift.io,resources=sharedvolumes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=aws-efs.managed.openshift.io,resources=sharedvolumes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=aws-efs.managed.openshift.io,resources=sharedvolumes/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SharedVolume object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *SharedVolumeReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	log, errx := logr.FromContext(ctx)

	if errx != nil {
		panic(errx)
	}

	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling SharedVolume")

	// Fetch the SharedVolume instance
	sharedVolume := &awsefsv1alpha1.SharedVolume{}
	if err := r.client.Get(ctx, request.NamespacedName, sharedVolume); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("SharedVolume was deleted out-of-band.")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Failed to retrieve.")
		return reconcile.Result{}, err
	}

	// Deleting?
	if sharedVolume.GetDeletionTimestamp() != nil {
		err := r.markStatus(reqLogger, sharedVolume, awsefsv1alpha1.SharedVolumeDeleting, "")
		if err != nil {
			reqLogger.Error(err, "Error updating SharedVolume status")
		}
		return reconcile.Result{}, r.handleDelete(reqLogger, sharedVolume)
	}

	// Try to detect whether the SharedVolume got updated bogusly, and revert it.
	if updated, err := r.uneditSharedVolume(reqLogger, sharedVolume); err != nil {
		// If that didn't work, we really don't want to try to reconcile the PV/PVC.
		// uneditSharedVolume() logs
		return reconcile.Result{}, err
	} else if updated {
		// If we pushed a change, it's going to trigger another reconcile.
		// Let that happen and do the rest of our work then.
		// Make sure it happens by forcing a requeue -- the controller is supposed to collapse duplicates.
		return reconcile.Result{Requeue: true}, nil
	}

	/////
	// CREATE/UPDATE
	/////

	// Make sure our finalizer is registered before we start doing things that will need it
	if updated, err := r.ensureFinalizer(reqLogger, sharedVolume); err != nil {
		// If that didn't work, don't continue; requeue and let the next iteration try to fix things.
		return reconcile.Result{}, err
	} else if updated {
		// If we pushed a change, it's going to trigger another reconcile.
		// Let that happen and do the rest of our work then.
		// Make sure it happens by forcing a requeue -- the controller is supposed to collapse duplicates.
		return reconcile.Result{Requeue: true}, nil
	}

	// If we never set the status, it means this SharedVolume is new, and we'll be creating the
	// associated resources.
	if sharedVolume.Status.Phase == "" {
		err := r.markStatus(reqLogger, sharedVolume, awsefsv1alpha1.SharedVolumePending, "")
		// Whether this worked or not (err could be nil), requeue and let the next Reconcile do the rest.
		return reconcile.Result{Requeue: true}, err
	}
	// Otherwise, don't try to maintain any kind of in-flight status while we check and reconcile
	// the PV/PVC. Whatever state was set before is fine until we have something new to report.
	// TODO: Unless it was "Deleting". Could that even happen?

	// TODO: If either the PV or PVC gets munged, the other ends up in a bad/unusable state.
	//       We probably just want to delete and recreate both

	// The sub-resources we're going to be managing
	pve := pvEnsurable(sharedVolume)
	pvce := pvcEnsurable(sharedVolume)

	reqLogger.Info("Reconciling PersistentVolume", "Name", pve.GetNamespacedName().Name)
	if err := pve.Ensure(reqLogger, r.client); err != nil {
		// Mark Error status. This is best-effort (ignore any errors), since it's happening within
		// an error path whose behavior we don't want to disrupt.
		// Note that we don't clear Status.ClaimRef: if it's set, it might help track
		// down the cause of the error.
		_ = r.markStatus(reqLogger, sharedVolume, awsefsv1alpha1.SharedVolumeFailed, err.Error())
		return reconcile.Result{}, err
	}

	pvcnsname := pvce.GetNamespacedName()
	reqLogger.Info("Reconciling PersistentVolumeClaim", "NamespacedName", pvcnsname)
	if err := pvce.Ensure(reqLogger, r.client); err != nil {
		// Mark Error status. This is best-effort (ignore any errors), since it's happening within
		// an error path whose behavior we don't want to disrupt.
		// Note that we don't clear Status.ClaimRef: if it's set, it might help track
		// down the cause of the error.
		_ = r.markStatus(reqLogger, sharedVolume, awsefsv1alpha1.SharedVolumeFailed, err.Error())
		return reconcile.Result{}, err
	}

	// If we got this far, the PV/PVC are good (as far as we can tell).
	return reconcile.Result{}, r.markReady(reqLogger, sharedVolume, pvcnsname)
}

// SetupWithManager sets up the controller with the Manager.
func (r *SharedVolumeReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&awsefsv1alpha1.SharedVolume{}).
		Watches(&source.Kind{Type: &corev1.PersistentVolume{}}, &enqueueRequestForSharedVolume{
			Client: mgr.GetClient(),
		}).
		Watches(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, &enqueueRequestForSharedVolume{
			Client: mgr.GetClient(),
		}).
		Complete(r)

	//// Watch PVs that trigger our predicate, and map them to the SharedVolume that owns them.
	//// Note that we can't use owner references because PVs aren't namespaced.
	//err = c.Watch(
	//	&source.Kind{Type: &corev1.PersistentVolume{}},
	//	&handler.EnqueueRequestsFromMapFunc{ToRequests: handler.ToRequestsFunc(toSharedVolume)},
	//	util.ICarePredicate)
	//if err != nil {
	//	return err
	//}
	//
	//// Watch PVCs that trigger our predicate, and map them to the SharedVolume that owns them.
	//// (Could have done this with owner references, but prefer being consistent with the way
	//// we're handling PersistentVolumes.)
	//err = c.Watch(
	//	&source.Kind{Type: &corev1.PersistentVolumeClaim{}},
	//	&handler.EnqueueRequestsFromMapFunc{ToRequests: handler.ToRequestsFunc(toSharedVolume)},
	//	util.ICarePredicate)
	//if err != nil {
	//	return err
	//}
	//
	//return nil
}

// ensureFinalizer makes sure the `sharedVolume` has our finalizer registered.
// The `bool` return indicates whether an update was pushed to the server.
func (r *SharedVolumeReconciler) ensureFinalizer(logger logr.Logger, sharedVolume *awsefsv1alpha1.SharedVolume) (bool, error) {
	if util.StringInSlice(svFinalizer, sharedVolume.GetFinalizers()) {
		return false, nil
	}
	logger.Info("Registering finalizer")
	controllerutil.AddFinalizer(sharedVolume, svFinalizer)
	
	
	ctx := logr.NewContext(context.TODO(), logger)
	
	
	if err := r.client.Update(ctx, sharedVolume); err != nil {
		logger.Error(err, "Failed to register finalizer")
		return false, err
	}
	return true, nil
}

func (r *SharedVolumeReconciler) handleDelete(logger logr.Logger, sharedVolume *awsefsv1alpha1.SharedVolume) error {
	if !util.StringInSlice(svFinalizer, sharedVolume.GetFinalizers()) {
		// Nothing to do
		return nil
	}
	logger.Info("SharedVolume marked for deletion. Finalizing...")

	// Note that Delete only cares about the NamespacedName of each Ensurable. This matters
	// because it's theoretically possible that the guts of the PV and/or PVC are out of sync
	// with what's in the SharedVolume. We specifically don't care if that's true because it's
	// all going away.
	// TODO(efried): Move cache cleaning logic into pv[c]_ensurable.go... somehow.
	// Order matters here. Delete the PVC first...
	e := pvcEnsurable(sharedVolume)
	k := svKey(sharedVolume)
	defer delete(pvcBySharedVolume, k)
	if err := e.Delete(logger, r.client); err != nil {
		// Delete did the logging
		return err
	}
	// ...then the PV
	e = pvEnsurable(sharedVolume)
	defer delete(pvBySharedVolume, k)
	if err := e.Delete(logger, r.client); err != nil {
		// Delete did the logging
		return err
	}
	ctx := logr.NewContext(context.TODO(), logger)
	// We're done. Remove our finalizer and let the SharedVolume deletion proceed.
	controllerutil.RemoveFinalizer(sharedVolume, svFinalizer)
	if err := r.client.Update(ctx, sharedVolume); err != nil {
		logger.Error(err, "Failed to remove finalizer")
		return err
	}
	return nil
}

// markStatus tries to update the SharedVolume's Status.Phase if not already `phase`, and the
// Message likewise, returning any error from the update. Don't use this for the Ready phase -- use
// markReady instead, because that knows how to handle the PVC bit. Also note that clearing the
// message is an important part of marking status, so pass in "" if that's what you mean to do.
func (r *SharedVolumeReconciler) markStatus(
	logger logr.Logger, sharedVolume *awsefsv1alpha1.SharedVolume,
	phase awsefsv1alpha1.SharedVolumePhase, message string) error {

	updateRequired := false
	if sharedVolume.Status.Phase != phase {
		sharedVolume.Status.Phase = phase
		updateRequired = true
	}
	if sharedVolume.Status.Message != message {
		sharedVolume.Status.Message = message
		updateRequired = true
	}
	if !updateRequired {
		// No update necessary. Short out.
		return nil
	}
	return r.updateStatus(logger, sharedVolume)
}

// markReady tries to update the SharedVolume's Status.ClaimRef per `pvcnsname` and
// its Phase as Ready, returning an error if the update fails. This only attempts the
// update if necessary, so as not to trigger an unnecessary Reconcile.
func (r *SharedVolumeReconciler) markReady(
	logger logr.Logger, sharedVolume *awsefsv1alpha1.SharedVolume, pvcnsname types.NamespacedName) error {

	// Only update the SharedVolume if necessary. Otherwise this could trigger another reconcile
	// and get us in a tight loop.
	// TODO: Better way to construct/populate this TypedLocalObjectReference? Looking for something
	// like ObjectRefFromObject()
	updateNeeded := false
	if sharedVolume.Status.Phase != awsefsv1alpha1.SharedVolumeReady {
		sharedVolume.Status.Phase = awsefsv1alpha1.SharedVolumeReady
		updateNeeded = true
	}
	if sharedVolume.Status.ClaimRef.Name != pvcnsname.Name {
		sharedVolume.Status.ClaimRef.Name = pvcnsname.Name
		updateNeeded = true
	}
	if updateNeeded {
		return r.updateStatus(logger, sharedVolume)
	}
	return nil
}

func (r *SharedVolumeReconciler) updateStatus(logger logr.Logger, sharedVolume *awsefsv1alpha1.SharedVolume) error {
	logger.Info("Updating SharedVolume status", "status", sharedVolume.Status)
	// TODO: I shouldn't have to set this, since PVC is in core.
	apiGroup := ""
	sharedVolume.Status.ClaimRef.APIGroup = &apiGroup
	sharedVolume.Status.ClaimRef.Kind = pvcKind
	ctx := logr.NewContext(context.TODO(), logger)
	if err := r.client.Status().Update(ctx, sharedVolume); err != nil {
		logger.Error(err, "Failed to update SharedVolume status")
		return err
	}
	return nil
}

// uneditSharedVolume is a poor man's enforcement of immutability of a SharedVolume. It looks for
// the PV associated with the SharedVolume. If found, peels out the FS and AP IDs. If they differ
// from what's in the SharedVolume, it means the SharedVolume was "edited", in which case we
// restore the original values, editing the `sharedVolume` parameter in place and pushing the
// change back to the server.
// The `bool` return indicates whether an update was pushed successfully.
// NOTE: Cases where we can't glean the FSID/APID are treated in a way that may not be intuitive.
// This can happen when:
// - The SharedVolume is fresh and the PV hasn't been created yet. In this case, we want the caller
//   to proceed to "reconcile" (create) the PV.
// - The PV was deleted. Ditto.
// - The PV was edited in some way that makes it impossible to grab the FSID and/or APID. In this
//   case, we assume that the SharedVolume was not edited simultaneously, and represents what we
//   want the PV to look like. So we treat this as a non-update, non-error so the caller can,
//   again, reconcile the PV and get it back to a good state.
// TODO: We may want to make this return in such a way that we can distinguish the "PV was edited"
// scenario, because in that case it's unlikely that restoring it by itself will even work. In
// that case we should either delete the PV/PVC pair and start over, or mark the SharedVolume as
// Failed and refuse to continue reconciling it, requiring it to be deleted and recreated.
func (r *SharedVolumeReconciler) uneditSharedVolume(
	logger logr.Logger, sharedVolume *awsefsv1alpha1.SharedVolume) (updated bool, err error) {

	updated = false
	err = nil
	pv := &corev1.PersistentVolume{}
	// Take advantage of the predictable naming convention to look for our PV
	pvname := pvNameForSharedVolume(sharedVolume)
	nsname := types.NamespacedName{
		Name: pvname,
	}
	ctx := logr.NewContext(context.TODO(), logger)

	if err = r.client.Get(ctx, nsname, pv); err != nil {
		if errors.IsNotFound(err) {
			// We haven't created this PV yet. One way or another, this means we need to trust that
			// the SharedVolume is copacetic.
			err = nil
			return
		}
		// Some other error. This ain't good.
		logger.Error(err, "Failed to retrieve the associated PV", "PV name", pvname)
		return
	}

	// Things could get squirrelly here, e.g. if the PV has been changed in ways that leave us
	// trying to access nil pointers. Safeguard against that.
	defer func() {
		if r := recover(); r != nil {
			logger.Error(fmt.Errorf("%v", r), "Couldn't glean File System or Access Point ID from the SharedVolume")
			// Return no-update, no-error so this PV gets restored
			err = nil
		}
	}()

	// This is just used for logging
	svname := fmt.Sprintf("%s/%s", sharedVolume.Namespace, sharedVolume.Name)

	// We found the corresponding PV. Peel the FS and AP IDs out of it.
	volHandle := pv.Spec.PersistentVolumeSource.CSI.VolumeHandle
	if volHandle == "" {
		// Let's funnel this into our recover() since it's the same class of error as e.g. nil
		// pointer dereference. This will make it easier to handle those errors differently if
		// we decide to do that in the future.
		panic(fmt.Sprintf("PersistentVolume %s for SharedVolume %s has no VolumeHandle", pvname, svname))
	}
	var apid string
	// Discover the access point. We'll tolerate either the old style with the access point in the
	// MountOptions (before [1]), or the new style where the VolumeHandle is colon-delimited
	// and includes the access point as the third field.
	// [1] https://github.com/openshift/aws-efs-operator/pull/17/commits/bfcfcda1158510a28cc253a76c74fd03edd20a4f#diff-b7b6189fad2ed163b0a2ff5f7f22ad50L73-L81
	tokens := strings.SplitN(volHandle, ":", 3)
	fsid := tokens[0]
	if len(tokens) == 1 {
		// Access point is in MountOptions
		for _, opt := range pv.Spec.MountOptions {
			tokens := strings.SplitN(opt, "=", 2)
			if len(tokens) == 2 && tokens[0] == "accesspoint" {
				apid = tokens[1]
			}
		}
	} else if len(tokens) == 3 {
		apid = tokens[2]
	} else {
		panic(fmt.Sprintf("Couldn't parse VolumeHandle %q", volHandle))
	}
	if apid == "" {
		// Ditto above
		panic(fmt.Sprintf("Couldn't find Access Point ID in PersistentVolume %s for SharedVolume %s", pvname, svname))
	}

	// Now make sure the SharedVolume is right
	updateNeeded := false
	if sharedVolume.Spec.FileSystemID != fsid {
		logger.Info("SharedVolume has an unexpected FileSystemID",
			"SharedVolume", svname, "Found FSID", sharedVolume.Spec.FileSystemID, "Expected FSID", fsid)
		sharedVolume.Spec.FileSystemID = fsid
		updateNeeded = true
	}
	if sharedVolume.Spec.AccessPointID != apid {
		logger.Info("SharedVolume has an unexpected AccessPointID",
			"SharedVolume", svname, "Found APID", sharedVolume.Spec.AccessPointID, "Expected APID", apid)
		sharedVolume.Spec.AccessPointID = apid
		updateNeeded = true
	}
	if !updateNeeded {
		err = nil
		return
	}

	logger.Info("Detected changes to SharedVolume. Don't do that. "+
		"If you need to attach to a different file system or access point, "+
		"delete the SharedVolume and create a new one. Reverting...", "SharedVolume", sharedVolume)
	if err = r.client.Update(ctx, sharedVolume); err != nil {
		logger.Error(err, "Failed to revert changes to SharedVolume")
		return
	}

	// That worked. We're done. Tell the caller we pushed an update.
	updated = true
	return
}