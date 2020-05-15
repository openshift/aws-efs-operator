package sharedvolume

import (
	awsefsv1alpha1 "openshift/aws-efs-operator/pkg/apis/awsefs/v1alpha1"
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"
)

// This is used for PV and PVC definitions. Therein, it is required by schema and validated
// against quota, but ignored by the EFS CSI driver because the backing file system is... elastic.
// Unfortunately, this is likely to be misleading to a human looking at the PV/PVC.
var efsSize = resource.MustParse("1Gi")

func svKey(sv *awsefsv1alpha1.SharedVolume) string {
	return fmt.Sprintf("%s %s", sv.Namespace, sv.Name)
}