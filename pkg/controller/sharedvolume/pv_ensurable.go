package sharedvolume

// Ensurable impl for PersistentVolume

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

// Cache of PV Ensurables by SharedVolume namespace and name
var pvBySharedVolume = make(map[string]util.Ensurable)

func pvEnsurable(sharedVolume *efscsiv1alpha1.SharedVolume) util.Ensurable {
	key := svKey(sharedVolume)
	if _, ok := pvBySharedVolume[key]; !ok {
		pvBySharedVolume[key] = &util.EnsurableImpl{
			ObjType:        &corev1.PersistentVolume{},
			NamespacedName: pvNamespacedName(sharedVolume),
			Definition:     pvDefinition(sharedVolume),
			EqualFunc: func(local, server runtime.Object) bool {
				return reflect.DeepEqual(
					local.(*corev1.PersistentVolume).Spec,
					server.(*corev1.PersistentVolume).Spec)
			},
		}
	}
	return pvBySharedVolume[key]
}

func pvNamespacedName(sharedVol *efscsiv1alpha1.SharedVolume) types.NamespacedName {
	return types.NamespacedName{
		Name: pvNameForSharedVolume(sharedVol),
		// PVs are not namespaced
	}
}

func pvNameForSharedVolume(sharedVolume *efscsiv1alpha1.SharedVolume) string {
	// Name the PV after the SharedVolume so it's easy to spot visually.
	return fmt.Sprintf("pv-%s-%s", sharedVolume.Namespace, sharedVolume.Name)
}

func pvDefinition(sharedVolume *efscsiv1alpha1.SharedVolume) *corev1.PersistentVolume {
	filesystem := corev1.PersistentVolumeFilesystem
	return &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: pvNameForSharedVolume(sharedVolume),
		},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{
				// NOTE: This is ignored by the CSI driver, but a value is required to create a PV.
				corev1.ResourceStorage: efsSize,
			},
			VolumeMode:                    &filesystem,
			AccessModes:                   []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			StorageClassName:              statics.StorageClassName,
			MountOptions: []string{
				"tls",
				fmt.Sprintf("accesspoint=%s", sharedVolume.Spec.AccessPointID),
			},
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:       statics.CSIDriverName,
					VolumeHandle: sharedVolume.Spec.FileSystemID,
				},
			},
		},
	}
}
