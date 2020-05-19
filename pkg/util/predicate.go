package util

/**
This module encapsulates logic to identify resources this operator owns. This is because we
can't always rely on the actual "owner" field, particularly for resources owned by a
controller at large rather than another resource; and we can't rely on namespacing
because not all resources we care about are namespaced.

Keeping this abstracted lets us change the logic easily in the future if we need to.
*/

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// The current implementation uses a specific metadata.label key/value to indicate ownership.
const (
	// labelKey is the ObjectMeta.Label key for objects this operator controls
	labelKey = "openshift.io/aws-efs-operator-owned"
	// labelValue is the ObjectMeta.Label value for objects this operator controls
	labelValue = "true"
)

var log = logf.Log.WithName("controller_utils")

// ICarePredicate provides a Watch filter that only passes objects we care about.
// Use this when we can't match on owner, either because there is no runtime.Object owner
// (cluster-level resources that are "owned" by the controller) or because the owning and
// owned objects are in different namespaces.
var ICarePredicate = predicate.Funcs{
	CreateFunc: func(e event.CreateEvent) bool { return passes(e.Object, e.Meta) },
	DeleteFunc: func(e event.DeleteEvent) bool { return passes(e.Object, e.Meta) },
	// UpdateFunc passes if *either* the new or old object is one we care about.
	UpdateFunc: func(e event.UpdateEvent) bool {
		return passes(e.ObjectOld, e.MetaOld) || passes(e.ObjectNew, e.MetaNew)
	},
	GenericFunc: func(e event.GenericEvent) bool { return passes(e.Object, e.Meta) },
}

// DoICare answers whether our object will trigger our watcher.
func DoICare(obj runtime.Object) bool {
	metaObj := obj.(metav1.Object)
	l, ok := metaObj.GetLabels()[labelKey]
	if !ok {
		return false
	}
	return l == labelValue
}

// MakeMeCare is the write side of the DoICare: it modifies obj
// such that an event on it will make ICarePredicate pass.
func MakeMeCare(obj runtime.Object) {
	metaObj := obj.(metav1.Object)
	if metaObj.GetLabels() == nil {
		metaObj.SetLabels(make(map[string]string))
	}
	metaObj.GetLabels()[labelKey] = labelValue
}

func passes(obj runtime.Object, meta metav1.Object) bool {
	if obj == nil {
		log.Error(nil, "No object for event!")
		return false
	}
	if meta == nil {
		log.Error(nil, "No metadata for event object!")
		return false
	}
	return DoICare(obj)
}
