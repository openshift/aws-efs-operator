package sharedvolume

import (
	efscsiv1alpha1 "2uasimojo/efs-csi-operator/pkg/apis/efscsi/v1alpha1"
	"fmt"
)

func svKey(sv *efscsiv1alpha1.SharedVolume) string {
	return fmt.Sprintf("%s %s", sv.Namespace, sv.Name)
}