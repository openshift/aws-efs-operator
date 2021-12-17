package statics

import (
	"testing"

	"openshift/aws-efs-operator/pkg/test"
	"openshift/aws-efs-operator/pkg/util"

	"golang.org/x/net/context"

	securityv1 "github.com/openshift/api/security/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	expectedNumStatics = 5
)

// checkNumStatics is a helper to guard against static resources being added in the future without tests
// being updated. Use it from any test that would need to be fixed if new statics are added.
func checkNumStatics(t *testing.T) {
	if numStatics := len(staticResources); numStatics != expectedNumStatics {
		t.Fatalf("Test update needed! Expected %d static resources but found %d.",
			expectedNumStatics, numStatics)
	}
}

// checkStatics queries the client for all the known static resources, verifying that they exist
// and have the expected content. It returns a map, keyed by the short name of the resource type
// (e.g. "SecurityContextConstraints") of the runtime.Object returned by the client for each resource.
func checkStatics(t *testing.T, client crclient.Client) map[string]runtime.Object {
	ret := make(map[string]runtime.Object)
	ctx := context.TODO()

	for _, i := range []struct {
		name   string
		obj    runtime.Object
		nsname types.NamespacedName
	}{
		{
			"ServiceAccount",
			&corev1.ServiceAccount{},
			types.NamespacedName{Name: serviceAccountName, Namespace: namespaceName},
		},
		{
			"SecurityContextConstraints",
			&securityv1.SecurityContextConstraints{},
			types.NamespacedName{Name: sccName},
		},
		{
			"DaemonSet",
			&appsv1.DaemonSet{},
			types.NamespacedName{Name: daemonSetName, Namespace: namespaceName},
		},
		{
			"CSIDriver",
			&storagev1.CSIDriver{},
			types.NamespacedName{Name: CSIDriverName},
		},
		{
			"StorageClass",
			&storagev1.StorageClass{},
			types.NamespacedName{Name: StorageClassName},
		},
	} {
		if err := client.Get(ctx, i.nsname, i.obj); err != nil {
			t.Fatalf("Couldn't get %s: %v", i.name, err)
		}
		test.DoDiff(t, findStatic(i.nsname).(*util.EnsurableImpl).Definition, i.obj, true)
		ret[i.name] = i.obj
	}

	return ret
}
