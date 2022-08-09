package controllers

// Ensurable impl for PersistentVolumeClaim

import (
	awsefsv1alpha1 "openshift/aws-efs-operator/api/v1alpha1"
	"openshift/aws-efs-operator/controllers/statics"
	util "openshift/aws-efs-operator/pkg/util"

	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Cache of PVC Ensurables by SharedVolume namespace and name
var pvcBySharedVolume = make(map[string]util.Ensurable)

func pvcEnsurable(sharedVolume *awsefsv1alpha1.SharedVolume) util.Ensurable {
	key := svKey(sharedVolume)
	if _, ok := pvcBySharedVolume[key]; !ok {
		pvcBySharedVolume[key] = &util.EnsurableImpl{
			ObjType:        &corev1.PersistentVolumeClaim{},
			NamespacedName: pvcNamespacedName(sharedVolume),
			Definition:     pvcDefinition(sharedVolume),
			// PVCs are (almost*) immutable once created, so doing an equals check is probably
			// always a no-op, and we could use AlwaysEqual here instead. But it's harmless for
			// now, so leave it.
			// * except for the size, if supported, which in the case of EFS it's not,
			//   and wouldn't make sense anyway, because elastic.
			EqualFunc: func(local, server crclient.Object) bool {
				return reflect.DeepEqual(
					local.(*corev1.PersistentVolumeClaim).Spec,
					server.(*corev1.PersistentVolumeClaim).Spec)
			},
		}
	}
	return pvcBySharedVolume[key]
}

func pvcNamespacedName(sharedVolume *awsefsv1alpha1.SharedVolume) types.NamespacedName {
	return types.NamespacedName{
		// Name the PVC after the SharedVolume so it's easy to spot visually.
		Name:      fmt.Sprintf("pvc-%s", sharedVolume.Name),
		Namespace: sharedVolume.Namespace,
	}
}

func pvcDefinition(sharedVolume *awsefsv1alpha1.SharedVolume) *corev1.PersistentVolumeClaim {
	nsname := pvcNamespacedName(sharedVolume)
	scname := statics.StorageClassName
	filesystem := corev1.PersistentVolumeFilesystem
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsname.Name,
			Namespace: nsname.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					// NOTE: This is ignored by the CSI driver, but a value is required to create a PVC.
					// To make matters worse, the number is validated against quota.
					corev1.ResourceStorage: efsSize,
				},
			},
			StorageClassName: &scname,
			VolumeMode:       &filesystem,
			VolumeName:       pvNameForSharedVolume(sharedVolume),
		},
	}
	setSharedVolumeOwner(pvc, sharedVolume)
	return pvc
}
