package sharedvolume

// Ensurable impl for PersistentVolumeClaim

import (
	efscsiv1alpha1 "2uasimojo/efs-csi-operator/pkg/apis/efscsi/v1alpha1"
	"2uasimojo/efs-csi-operator/pkg/controller/statics"
	util "2uasimojo/efs-csi-operator/pkg/util"

	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

// Cache of PVC Ensurables by SharedVolume namespace and name
var pvcBySharedVolume = make(map[string]util.Ensurable)

func pvcEnsurable(sharedVolume *efscsiv1alpha1.SharedVolume) util.Ensurable {
	key := svKey(sharedVolume)
	if _, ok := pvcBySharedVolume[key]; !ok {
		pvcBySharedVolume[key] = &util.EnsurableImpl{
			ObjType:        &corev1.PersistentVolumeClaim{},
			NamespacedName: pvcNamespacedName(sharedVolume),
			Definition:     pvcDefinition(sharedVolume),
			EqualFunc: func(local, server runtime.Object) bool {
				return reflect.DeepEqual(
					local.(*corev1.PersistentVolumeClaim).Spec,
					server.(*corev1.PersistentVolumeClaim).Spec)
			},
		}
	}
	return pvcBySharedVolume[key]
}

func pvcNamespacedName(sharedVolume *efscsiv1alpha1.SharedVolume) types.NamespacedName {
	return types.NamespacedName{
		// Name the PVC after the SharedVolume so it's easy to spot visually.
		Name:      fmt.Sprintf("pvc-%s", sharedVolume.Name),
		Namespace: sharedVolume.Namespace,
	}
}

func pvcDefinition(sharedVolume *efscsiv1alpha1.SharedVolume) *corev1.PersistentVolumeClaim {
	nsname := pvcNamespacedName(sharedVolume)
	scname := statics.StorageClassName
	filesystem := corev1.PersistentVolumeFilesystem
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsname.Name,
			Namespace: nsname.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					// NOTE: This is ignored by the CSI driver, but a value is required to create a PVC.
					corev1.ResourceStorage: efsSize,
				},
			},
			StorageClassName: &scname,
			VolumeMode:       &filesystem,
			VolumeName:       pvNameForSharedVolume(sharedVolume),
		},
	}
}
