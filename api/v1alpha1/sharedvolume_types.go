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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SharedVolumeSpec defines the desired state of SharedVolume
type SharedVolumeSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The ID of the EFS volume, e.g. `fs-0123cdef`. Required. Immutable.
	// +kubebuilder:validation:Pattern=^fs-[0-9a-f]+$
	FileSystemID string `json:"fileSystemID"`
	// The ID of an EFS volume access point, e.g. `fsap-0123456789abcdef`.
	// The EFS volume will be mounted to the specified access point.
	// Required. Immutable.
	// +kubebuilder:validation:Pattern=^fsap-[0-9a-f]+$
	AccessPointID string `json:"accessPointID"`
}

// SharedVolumePhase are possible values for `SharedVolumeStatus.Phase`
type SharedVolumePhase string

const (
	// SharedVolumePending indicates that we've noticed the SharedVolume and are working on
	// creating its associated resources.
	SharedVolumePending SharedVolumePhase = "Pending"
	// SharedVolumeReady means we've created the resources associated with the SharedVolume,
	// successfully as far as we can tell. Importantly, this does not imply that the
	// PersistentVolumeClaim is bound, etc. It's up to the consumer to check that.
	SharedVolumeReady SharedVolumePhase = "Ready"
	// SharedVolumeDeleting means we've noticed a deletion timestamp and have started to finalize;
	// that is, delete the associated resources. There is no phase indicating that we've finished
	// doing that; we expect the SharedVolume to disappear (be garbage collected) shortly.
	SharedVolumeDeleting SharedVolumePhase = "Deleting"
	// SharedVolumeFailed means something went wrong. We'll do our best to populate
	// SharedVolume.Message with something useful, but more information should be available by
	// inspecting the associated PersistentVolume(Claim) resources.
	SharedVolumeFailed SharedVolumePhase = "Failed"
)

// SharedVolumeStatus defines the observed state of SharedVolume
type SharedVolumeStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// ClaimRef refers to the PersistentVolumeClaim bound to a PersistentVolume representing the
	// file system access point, both of which are created at the behest of this SharedVolume.
	ClaimRef corev1.TypedLocalObjectReference `json:"claimRef,omitempty"`
	// Phase indicates the state of the PersistentVolume and PersistentVolumeClaim artifacts
	// associated with this SharedVolume. See SharedVolumePhase consts for possible values.
	Phase SharedVolumePhase `json:"phase,omitempty"`
	// Message is a human-readable string, usually describing what went wrong when `Phase` is `SharedVolumeFailed`.
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SharedVolume is the Schema for the sharedvolumes API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=sharedvolumes,shortName=sv,scope=Namespaced
// +kubebuilder:printcolumn:name="File System",type=string,JSONPath=`.spec.fileSystemID`
// +kubebuilder:printcolumn:name="Access Point",type=string,JSONPath=`.spec.accessPointID`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Claim",type=string,JSONPath=`.status.claimRef.name`
// +kubebuilder:printcolumn:name="Message",type=string,JSONPath=`.status.message`
type SharedVolume struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SharedVolumeSpec   `json:"spec,omitempty"`
	Status SharedVolumeStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SharedVolumeList contains a list of SharedVolume
type SharedVolumeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SharedVolume `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SharedVolume{}, &SharedVolumeList{})
}
