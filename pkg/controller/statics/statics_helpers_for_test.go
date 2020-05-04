package statics

import (
	"testing"

	"2uasimojo/efs-csi-operator/pkg/test"
	util "2uasimojo/efs-csi-operator/pkg/util"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"golang.org/x/net/context"

	securityv1 "github.com/openshift/api/security/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	storagev1beta1 "k8s.io/api/storage/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	expectedNumStatics = 6
)

// Common options we're passing into cmp.Diff.
var diffOpts cmp.Options

func init() {
	diffOpts = cmp.Options{
		// We want to ignore TypeMeta in all cases, because it's a given of the type itself.
		cmpopts.IgnoreTypes(metav1.TypeMeta{}),
		// We ignore the ResourceVersion because it gets set by the server and is unpredictable/opaque.
		// We ignore labels *in cmp.Diff* because sometimes we're checking a virgin resource definition
		// from a getter (label validation is done separately).
		cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion", "Labels"),
	}
}

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

	ns := &corev1.Namespace{}
	if err := client.Get(ctx, types.NamespacedName{Name: namespaceName}, ns); err != nil {
		t.Fatalf("Couldn't get Namespace: %v", err)
	}
	validateNamespace(t, ns, true)
	ret["Namespace"] = ns

	sa := &corev1.ServiceAccount{}
	if err := client.Get(ctx, types.NamespacedName{Name: serviceAccountName, Namespace: namespaceName}, sa); err != nil {
		t.Fatalf("Couldn't get ServiceAccount: %v", err)
	}
	validateServiceAccount(t, sa, true)
	ret["ServiceAccount"] = sa

	scc := &securityv1.SecurityContextConstraints{}
	if err := client.Get(ctx, types.NamespacedName{Name: sccName}, scc); err != nil {
		t.Fatalf("Couldn't get SecurityContextConstraints: %v", err)
	}
	validateSecurityContextConstraints(t, scc, true)
	ret["SecurityContextConstraints"] = scc

	ds := &appsv1.DaemonSet{}
	if err := client.Get(ctx, types.NamespacedName{Name: daemonSetName, Namespace: namespaceName}, ds); err != nil {
		t.Fatalf("Couldn't get DaemonSet: %v", err)
	}
	validateDaemonSet(t, ds, true)
	ret["DaemonSet"] = ds

	cd := &storagev1beta1.CSIDriver{}
	if err := client.Get(ctx, types.NamespacedName{Name: CSIDriverName}, cd); err != nil {
		t.Fatalf("Couldn't get CSIDriver: %v", err)
	}
	validateCSIDriver(t, cd, true)
	ret["CSIDriver"] = cd

	sc := &storagev1.StorageClass{}
	if err := client.Get(ctx, types.NamespacedName{Name: StorageClassName}, sc); err != nil {
		t.Fatalf("Couldn't get StorageClass: %v", err)
	}
	validateStorageClass(t, sc, true)
	ret["StorageClass"] = sc

	return ret
}

func doDiff(t *testing.T, expected, actual runtime.Object, expectLabel bool) {
	diff := cmp.Diff(expected, actual, diffOpts...)
	if diff != "" {
		t.Fatal("Objects differ: -expected, +actual\n", diff)
	}
	if doICare := util.DoICare(actual); expectLabel != doICare {
		t.Fatalf("expectLabel was %v but DoICare returned %v", expectLabel, doICare)
	}
}
func validateNamespace(t *testing.T, actual *corev1.Namespace, expectLabel bool) {
	expected := &corev1.Namespace{}
	test.LoadYAML(t, expected, "namespace.yaml")
	doDiff(t, expected, actual, expectLabel)
}

func validateServiceAccount(t *testing.T, actual *corev1.ServiceAccount, expectLabel bool) {
	expected := &corev1.ServiceAccount{}
	test.LoadYAML(t, expected, "serviceaccount.yaml")
	doDiff(t, expected, actual, expectLabel)
}

func validateStorageClass(t *testing.T, actual *storagev1.StorageClass, expectLabel bool) {
	expected := &storagev1.StorageClass{}
	test.LoadYAML(t, expected, "storageclass.yaml")
	doDiff(t, expected, actual, expectLabel)
}

func validateCSIDriver(t *testing.T, actual *storagev1beta1.CSIDriver, expectLabel bool) {
	expected := &storagev1beta1.CSIDriver{}
	test.LoadYAML(t, expected, "csidriver.yaml")
	doDiff(t, expected, actual, expectLabel)
}

func validateSecurityContextConstraints(t *testing.T, actual *securityv1.SecurityContextConstraints, expectLabel bool) {
	expected := &securityv1.SecurityContextConstraints{}
	test.LoadYAML(t, expected, "scc.yaml")
	doDiff(t, expected, actual, expectLabel)
}

func validateDaemonSet(t *testing.T, actual *appsv1.DaemonSet, expectLabel bool) {
	expected := &appsv1.DaemonSet{}
	test.LoadYAML(t, expected, "daemonset.yaml")
	doDiff(t, expected, actual, expectLabel)
}
