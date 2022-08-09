package controllers

// Helpers for mapping secondary resources back to the SharedVolume that owns them.

import (
	"k8s.io/client-go/util/workqueue"
	awsefsv1alpha1 "openshift/aws-efs-operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	svOwnerNamespaceKey = "openshift.io/aws-efs-operator-shared-volume-owner-namespace"
	svOwnerNameKey      = "openshift.io/aws-efs-operator-shared-volume-owner-name"
)

var _ handler.EventHandler = &enqueueRequestForSharedVolume{}

// enqueueRequestForClusterDeployment implements the handler.EventHandler interface.
type enqueueRequestForSharedVolume struct {
	Client client.Client
}

func (e *enqueueRequestForSharedVolume) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	reqs := map[reconcile.Request]struct{}{}
	e.mapAndEnqueue(q, evt.Object, reqs)
}

func (e *enqueueRequestForSharedVolume) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	reqs := map[reconcile.Request]struct{}{}
	e.mapAndEnqueue(q, evt.ObjectOld, reqs)
	e.mapAndEnqueue(q, evt.ObjectNew, reqs)
}

func (e *enqueueRequestForSharedVolume) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	reqs := map[reconcile.Request]struct{}{}
	e.mapAndEnqueue(q, evt.Object, reqs)
}

func (e *enqueueRequestForSharedVolume) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	reqs := map[reconcile.Request]struct{}{}
	e.mapAndEnqueue(q, evt.Object, reqs)
}

func (e *enqueueRequestForSharedVolume) toRequests(obj client.Object) []reconcile.Request {
	reqs := []reconcile.Request{}

	labels := obj.GetLabels()
	svNamespace := labels[svOwnerNamespaceKey]
	svName := labels[svOwnerNameKey]
	if svNamespace == "" || svName == "" {
		log.Info("Object not owned by any SharedVolume. This is unexpected.", "object", obj)
		// But what can we do about it?
		return reqs
	}

	reqs = append(reqs, reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      svName,
			Namespace: svNamespace,
		},
	})
	return reqs
}

func (e *enqueueRequestForSharedVolume) mapAndEnqueue(q workqueue.RateLimitingInterface, obj client.Object, reqs map[reconcile.Request]struct{}) {
	for _, req := range e.toRequests(obj) {
		_, ok := reqs[req]
		if !ok {
			q.Add(req)
			// Used for de-duping requests
			reqs[req] = struct{}{}
		}
	}
}

func setSharedVolumeOwner(owned metav1.Object, owner *awsefsv1alpha1.SharedVolume) {
	// Note: Owner References would theoretically be a better fit here, but they're heavier than
	// what we need, and the existing utilities (controller-runtime/pkg/controller/controllerutil)
	// forbid ownership across namespaces, including between namespace- and cluster-scoped. KISS.
	if owned.GetLabels() == nil {
		owned.SetLabels(make(map[string]string))
	}
	labels := owned.GetLabels()
	labels[svOwnerNamespaceKey] = owner.Namespace
	labels[svOwnerNameKey] = owner.Name
}