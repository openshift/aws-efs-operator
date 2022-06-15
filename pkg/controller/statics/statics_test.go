package statics

import (
	"fmt"
	"openshift/aws-efs-operator/pkg/fixtures"
	"openshift/aws-efs-operator/pkg/util"
	"testing"

	"github.com/golang/mock/gomock"
	securityv1 "github.com/openshift/api/security/v1"
	"golang.org/x/net/context"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	// TODO: pkg/client/fake is deprecated, replace with pkg/envtest
	// nolint:staticcheck
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func TestEnsureStatics(t *testing.T) {
	// Future-proof this test against new statics being added.
	checkNumStatics(t)

	logger := logf.Log.Logger
	ctx := context.TODO()
	var statics map[string]runtime.Object

	// OpenShift types need to be registered explicitly
	scheme.Scheme.AddKnownTypes(securityv1.SchemeGroupVersion, &securityv1.SecurityContextConstraints{})

	// Bootstrap: no resources exist yet
	mockClient := fake.NewFakeClientWithScheme(scheme.Scheme)

	logger.Info("==> Phase: Bootstrap")

	if err := EnsureStatics(logger, mockClient); err != nil {
		t.Fatalf("EnsureStatics (bootstrap) failed with %v", err)
	}

	// Make sure all of our static resources got created.
	checkStatics(t, mockClient)

	logger.Info("<== Phase: Bootstrap")

	logger.Info("==> Phase: Steady state (if everything is as it should be, EnsureStatics should be effectively a no-op.)")

	if err := EnsureStatics(logger, mockClient); err != nil {
		t.Fatalf("EnsureStatics (steady state) failed with %v", err)
	}
	statics = checkStatics(t, mockClient)

	logger.Info("<== Phase: Steady state")

	// Now mess with the resources in various ways

	// Unlabel the ServiceAccount
	ns := statics["ServiceAccount"].(*corev1.ServiceAccount)
	ns.SetLabels(map[string]string{})
	if err := mockClient.Update(ctx, ns); err != nil {
		t.Fatalf("Failed to update ServiceAccount: %v", err)
	}

	// Twiddle some permissions in the SCC
	scc := statics["SecurityContextConstraints"].(*securityv1.SecurityContextConstraints)
	for _, bp := range []*bool{
		&scc.AllowHostDirVolumePlugin,
		&scc.AllowHostDirVolumePlugin,
		&scc.AllowHostIPC,
		&scc.AllowHostNetwork,
		&scc.AllowHostPID,
		&scc.AllowHostPorts,
		&scc.AllowPrivilegedContainer,
	} {
		*bp = false
	}
	scc.ReadOnlyRootFilesystem = true
	if err := mockClient.Update(ctx, scc); err != nil {
		t.Fatalf("Failed to update SecurityContextConstraints: %v", err)
	}

	// Remove a whole container from the DaemonSet
	ds := statics["DaemonSet"].(*appsv1.DaemonSet)
	ds.Spec.Template.Spec.Containers = ds.Spec.Template.Spec.Containers[1:]
	if err := mockClient.Update(ctx, ds); err != nil {
		t.Fatalf("Failed to update DaemonSet: %v", err)
	}

	// Delete the StorageClass
	sc := statics["StorageClass"].(*storagev1.StorageClass)
	if err := mockClient.Delete(ctx, sc); err != nil {
		t.Fatalf("Failed to delete StorageClass: %v", err)
	}

	// Having made a righteous mess, prove EnsureStatics fixes it.
	logger.Info("==> Phase: Recover")

	if err := EnsureStatics(logger, mockClient); err != nil {
		t.Fatalf("EnsureStatics (recover) failed with %v", err)
	}
	checkStatics(t, mockClient)

	logger.Info("<== Phase: Recover")
}

// TestEnsureStaticsError tests the error path of EnsureStatics
func TestEnsureStaticsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	log := fixtures.NewMockLogger(ctrl)
	client := fixtures.NewMockClient(ctrl)

	// Not realistic, we're just contriving a way to make Ensure fail
	theError := fixtures.AlreadyExists

	// We don't care about the calls, really, but we have to register them or gomock gets upset
	client.EXPECT().
		Get(gomock.Any(), gomock.Any(), gomock.Any()).
		Times(expectedNumStatics).
		Return(theError)
	log.EXPECT().
		Error(theError, "Failed to retrieve.", "resource", gomock.Any()).
		Times(expectedNumStatics)

	err := EnsureStatics(log, client)
	if err == nil {
		t.Fatal("Expected EnsureStatics to fail hard.")
	}
	// Check the error count in the string. It should fail for all of the statics
	expected := fmt.Sprintf("Encountered %d error(s) ensuring statics", expectedNumStatics)
	if err.Error() != expected {
		t.Fatalf("Unexpected error message.\nExpected: %s\nGot:      %s", expected, err.Error())
	}
}

// Test_static_GetType makes sure Ensurable.GetType() returns the right type for each of our statics.
func Test_static_GetType(t *testing.T) {
	// Future-proof this test against new statics being added.
	checkNumStatics(t)

	var nsn types.NamespacedName

	// ServiceAccount
	nsn = types.NamespacedName{Name: serviceAccountName, Namespace: namespaceName}
	if _, ok := findStatic(nsn).GetType().(*corev1.ServiceAccount); !ok {
		t.Fatal("GetType() returned the wrong type for ServiceAccount static resource.")
	}
	// SecurityContextConstraints
	nsn = types.NamespacedName{Name: sccName}
	if _, ok := findStatic(nsn).GetType().(*securityv1.SecurityContextConstraints); !ok {
		t.Fatal("GetType() returned the wrong type for SecurityContextConstraints static resource.")
	}
	// DaemonSet
	nsn = types.NamespacedName{Name: daemonSetName, Namespace: namespaceName}
	if _, ok := findStatic(nsn).GetType().(*appsv1.DaemonSet); !ok {
		t.Fatal("GetType() returned the wrong type for DaemonSet static resource.")
	}
	// CSIDriver
	nsn = types.NamespacedName{Name: CSIDriverName}
	if _, ok := findStatic(nsn).GetType().(*storagev1.CSIDriver); !ok {
		t.Fatal("GetType() returned the wrong type for CSIDriver static resource.")
	}
	// StorageClass
	nsn = types.NamespacedName{Name: StorageClassName}
	if _, ok := findStatic(nsn).GetType().(*storagev1.StorageClass); !ok {
		t.Fatal("GetType() returned the wrong type for StorageClass static resource.")
	}
}

// Test_AlwaysEqual should really live in ensurable_test.go (TODO) but that will entail changing
// the "Things that are super different" test case in some nontrivial way, since `staticResources`
// isn't exported.
func Test_AlwaysEqual(t *testing.T) {
	// This is kind of silly, but...
	type args struct {
		local  runtime.Object
		server runtime.Object
	}
	tests := []struct {
		name string
		args args
	}{
		{"nils", args{nil, nil}},
		{"empty pods", args{&corev1.Pod{}, &corev1.Pod{}}},
		{"Things that are super different", args{
			staticResources[0].(*util.EnsurableImpl).Definition,
			staticResources[4].(*util.EnsurableImpl).Definition,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := util.AlwaysEqual(tt.args.local, tt.args.server); !got {
				t.Errorf("AlwaysEqual() = %v, want true", got)
			}
		})
	}
}

func Test_storageClassEqual(t *testing.T) {
	sc1 := staticResourceMap[StorageClassName].(*util.EnsurableImpl).Definition.(*storagev1.StorageClass)
	sc2 := sc1.DeepCopy()

	if !util.EqualOtherThanMeta(sc1, sc1) {
		t.Error("Expected object to compare equal to itself.")
	}

	if !util.EqualOtherThanMeta(sc1, sc2) {
		t.Errorf("Getter should always return objects that compare equal.\n%v\n%v", sc1, sc2)
	}

	// Mucking with metadata shouldn't affect equality
	sc2.ObjectMeta.SelfLink = "/foo/bar/baz"
	if !util.EqualOtherThanMeta(sc1, sc2) {
		t.Errorf("Metadata should not affect equality.\n%v\n%v", sc1, sc2)
	}

	// But these fields should
	sc2.Provisioner = "foo"
	if util.EqualOtherThanMeta(sc1, sc2) {
		t.Errorf("Change of Provisioner should make these unequal.\n%v\n%v", sc1, sc2)
	}
	// reset
	sc2.Provisioner = sc1.Provisioner

	recycle := corev1.PersistentVolumeReclaimRecycle
	sc2.ReclaimPolicy = &recycle
	if util.EqualOtherThanMeta(sc1, sc2) {
		t.Errorf("Change of ReclaimPolicy should make these unequal.\n%v\n%v", sc1, sc2)
	}
	// reset
	sc2.ReclaimPolicy = sc1.ReclaimPolicy

	wf1c := storagev1.VolumeBindingWaitForFirstConsumer
	sc2.VolumeBindingMode = &wf1c
	if util.EqualOtherThanMeta(sc1, sc2) {
		t.Errorf("Change of VolumeBindingMode should make these unequal.\n%v\n%v", sc1, sc2)
	}
}

func Test_csiDriverEqual(t *testing.T) {
	cd1 := staticResourceMap[CSIDriverName].(*util.EnsurableImpl).Definition.(*storagev1.CSIDriver)
	cd2 := cd1.DeepCopy()

	if !csiDriverEqual(cd1, cd1) {
		t.Error("Expected object to compare equal to itself.")
	}

	if !csiDriverEqual(cd1, cd2) {
		t.Errorf("Getter should always return objects that compare equal.\n%v\n%v", cd1, cd2)
	}

	// Mucking with metadata shouldn't affect equality
	cd2.ObjectMeta.SelfLink = "/foo/bar/baz"
	if !csiDriverEqual(cd1, cd2) {
		t.Errorf("Metadata should not affect equality.\n%v\n%v", cd1, cd2)
	}

	// But anything in the Spec should
	trueVal := true
	cd2.Spec.AttachRequired = &trueVal
	if csiDriverEqual(cd1, cd2) {
		t.Errorf("Change in Spec should make these unequal.\n%v\n%v", cd1, cd2)
	}
}

func Test_securityContextConstraintsEqual(t *testing.T) {
	scc1 := staticResourceMap[sccName].(*util.EnsurableImpl).Definition.(*securityv1.SecurityContextConstraints)
	scc2 := scc1.DeepCopy()

	if !util.EqualOtherThanMeta(scc1, scc1) {
		t.Error("Expected object to compare equal to itself.")
	}

	if !util.EqualOtherThanMeta(scc1, scc2) {
		t.Errorf("Getter should always return objects that compare equal.\n%v\n%v", scc1, scc2)
	}

	// Mucking with metadata shouldn't affect equality
	scc2.ObjectMeta.SelfLink = "/foo/bar/baz"
	if !util.EqualOtherThanMeta(scc1, scc2) {
		t.Errorf("Metadata should not affect equality.\n%v\n%v", scc1, scc2)
	}

	// Pick a few fields to test
	scc2.AllowHostIPC = false
	if util.EqualOtherThanMeta(scc1, scc2) {
		t.Errorf("Changing AllowHostIPC should make these unequal.\n%v\n%v", scc1, scc2)
	}
	scc2.AllowHostIPC = true

	scc2.RunAsUser.Type = securityv1.RunAsUserStrategyMustRunAs
	if util.EqualOtherThanMeta(scc1, scc2) {
		t.Errorf("Changing RunAsUser.Type should make these unequal.\n%v\n%v", scc1, scc2)
	}
	scc2.RunAsUser.Type = securityv1.RunAsUserStrategyRunAsAny

	scc2.Users = append(scc2.Users, "foo")
	if util.EqualOtherThanMeta(scc1, scc2) {
		t.Errorf("Changing Users should make these unequal.\n%v\n%v", scc1, scc2)
	}
}

func Test_daemonSetEqual(t *testing.T) {
	ds1 := staticResourceMap[daemonSetName].(*util.EnsurableImpl).Definition.(*appsv1.DaemonSet)
	ds2 := ds1.DeepCopy()

	if !daemonSetEqual(ds1, ds1) {
		t.Error("Expected object to compare equal to itself.")
	}

	if !daemonSetEqual(ds1, ds2) {
		t.Errorf("Getter should always return objects that compare equal.\n%v\n%v", ds1, ds2)
	}

	// Mucking with metadata shouldn't affect equality
	ds2.ObjectMeta.SelfLink = "/foo/bar/baz"
	if !daemonSetEqual(ds1, ds2) {
		t.Errorf("Metadata should not affect equality.\n%v\n%v", ds1, ds2)
	}

	// But anything in the Spec should
	ds2.Spec.Template.Spec.Containers[0].ImagePullPolicy = corev1.PullIfNotPresent
	if daemonSetEqual(ds1, ds2) {
		t.Errorf("Change in Spec should make these unequal.\n%v\n%v", ds1, ds2)
	}
}
