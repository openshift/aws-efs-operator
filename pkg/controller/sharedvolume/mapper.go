package sharedvolume

// Helpers for mapping secondary resources back to the SharedVolume that owns them.

import (
	awsefsv1alpha1 "openshift/aws-efs-operator/pkg/apis/awsefs/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	svOwnerNamespaceKey = "openshift.io/aws-efs-operator-shared-volume-owner-namespace"
	svOwnerNameKey      = "openshift.io/aws-efs-operator-shared-volume-owner-name"
)

func toSharedVolume(mo handler.MapObject) []reconcile.Request {
	labels := mo.Meta.GetLabels()
	svNamespace := labels[svOwnerNamespaceKey]
	svName := labels[svOwnerNameKey]
	if svNamespace == "" || svName == "" {
		log.Info("Object not owned by any SharedVolume. This is unexpected.", "object", mo.Object)
		// But what can we do about it?
		return []reconcile.Request{}
	}

	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Namespace: svNamespace,
				Name:      svName,
			},
		},
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
