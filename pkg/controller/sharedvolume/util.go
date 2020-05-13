package sharedvolume

import (
	efscsiv1alpha1 "2uasimojo/efs-csi-operator/pkg/apis/efscsi/v1alpha1"
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"
)

// This is used for PV and PVC definitions. Therein, it is required by schema, but ignored by the
// CSI driver. So we're making it the maximum size of an EFS volume per
// https://github.com/awsdocs/amazon-efs-user-guide/blob/c5e95349e7a5c458f0b5a4a90c61011c243c2fb4/doc_source/limits.md#limits-for-amazon-efs-file-systems
// so that anyone looking at it at least has some idea.
var efsSize = resource.MustParse("47.9Ti")

func svKey(sv *efscsiv1alpha1.SharedVolume) string {
	return fmt.Sprintf("%s %s", sv.Namespace, sv.Name)
}