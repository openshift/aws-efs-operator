package sharedvolume

// Ensurable impl for PersistentVolume

import (
	awsefsv1alpha1 "openshift/aws-efs-operator/pkg/apis/awsefs/v1alpha1"
	"openshift/aws-efs-operator/pkg/controller/statics"
	util "openshift/aws-efs-operator/pkg/util"

	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Cache of PV Ensurables by SharedVolume namespace and name
var pvBySharedVolume = make(map[string]util.Ensurable)

func pvEnsurable(sharedVolume *awsefsv1alpha1.SharedVolume) util.Ensurable {
	key := svKey(sharedVolume)
	if _, ok := pvBySharedVolume[key]; !ok {
		pvBySharedVolume[key] = &util.EnsurableImpl{
			ObjType:        &corev1.PersistentVolume{},
			NamespacedName: pvNamespacedName(sharedVolume),
			Definition:     pvDefinition(sharedVolume),
			// NOTE: PVs are immutable once created, so theoretically we should never encounter an
			// event that requires an actual update. And if we did, we wouldn't be able to update
			// anyway, so pretend the change didn't happen.
			// The exception is an upgrade like the one from 0.0.2, where the shape of a SV-backed
			// PV changed, meaning that if the operator notices an old-style PV, it will try to
			// "fix" it. Which won't work. Spoofing "always equal" will avoid that.
			EqualFunc: util.AlwaysEqual,
		}
	}
	return pvBySharedVolume[key]
}

func pvNamespacedName(sharedVol *awsefsv1alpha1.SharedVolume) types.NamespacedName {
	return types.NamespacedName{
		Name: pvNameForSharedVolume(sharedVol),
		// PVs are not namespaced
	}
}

func pvNameForSharedVolume(sharedVolume *awsefsv1alpha1.SharedVolume) string {
	// Name the PV after the SharedVolume so it's easy to spot visually.
	return fmt.Sprintf("pv-%s-%s", sharedVolume.Namespace, sharedVolume.Name)
}

func pvDefinition(sharedVolume *awsefsv1alpha1.SharedVolume) *corev1.PersistentVolume {
	filesystem := corev1.PersistentVolumeFilesystem
	volumeHandle := fmt.Sprintf("%s::%s", sharedVolume.Spec.FileSystemID, sharedVolume.Spec.AccessPointID)
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: pvNameForSharedVolume(sharedVolume),
		},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{
				// NOTE: This is ignored by the CSI driver, but a value is required to create a PV.
				// To make matters worse, the number is validated against quota.
				corev1.ResourceStorage: efsSize,
			},
			VolumeMode:                    &filesystem,
			AccessModes:                   []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			StorageClassName:              statics.StorageClassName,
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:       statics.CSIDriverName,
					VolumeHandle: volumeHandle,
				},
			},
		},
	}
	setSharedVolumeOwner(pv, sharedVolume)
	return pv
}
