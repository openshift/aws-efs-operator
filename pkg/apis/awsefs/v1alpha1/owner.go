package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateOwnerReference constructs an OwnerReference from this SharedVolume, for use in the list of
// OwnerReferences of another object.
func (sv *SharedVolume) CreateOwnerReference() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: sv.APIVersion,
		Kind:       sv.Kind,
		Name:       sv.Name,
		UID:        sv.UID,
	}
}
