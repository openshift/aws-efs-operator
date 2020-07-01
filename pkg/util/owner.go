package util

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// AsOwner constructs an OwnerReference from the provided obj.
func AsOwner(obj runtime.Object) *metav1.OwnerReference {
	apiVersion, kind := obj.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	mobj := obj.(metav1.Object)
	return &metav1.OwnerReference{
		APIVersion: apiVersion,
		Kind:       kind,
		Name:       mobj.GetName(),
		UID:        mobj.GetUID(),
	}
}
