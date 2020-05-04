package sharedvolume

// Test cases for the PV and PVC Ensurables

import (
	efscsiv1alpha1 "2uasimojo/efs-csi-operator/pkg/apis/efscsi/v1alpha1"
	"2uasimojo/efs-csi-operator/pkg/test"
	util "2uasimojo/efs-csi-operator/pkg/util"

	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	fakeFSID = "fs-484648c8"
	fakeAPID = "fsap-097bd0daaba932e64"
	fakeNamespace = "project1"
)

var sharedVolume = efscsiv1alpha1.SharedVolume{
	ObjectMeta: metav1.ObjectMeta{
		Name: "my-shared-volume",
		Namespace: fakeNamespace,
	},
	Spec: efscsiv1alpha1.SharedVolumeSpec{
		AccessPointID: fakeAPID,
		FileSystemID: fakeFSID,
	},
}

func TestPVEnsurable(t *testing.T) {
	pve := pvEnsurable(&sharedVolume)

	// Validate pvDefinition() which also hits pvNamespacedName()
	actualDef := pve.(*util.EnsurableImpl).Definition
	expectedDef := pve.GetType()
	test.LoadYAML(t, expectedDef, "persistentvolume.yaml")
	test.DoDiff(t, expectedDef, actualDef, false)

	// Validate EqualFunc
	equal := pve.(*util.EnsurableImpl).EqualFunc
	// They should be "equal" to start off
	if !equal(actualDef, expectedDef) {
		t.Fatalf("Expected defs to be equal:\n%v\n%v", actualDef, expectedDef)
	}
	// Muck with something we don't care about (EqualFunc shouldn't check ResourceVersion)
	actualDef.(metav1.Object).SetResourceVersion("abc")
	if !equal(actualDef, expectedDef) {
		t.Fatalf("Expected defs to be equal:\n%v\n%v", actualDef, expectedDef)
	}
	// Now muck with something we care about
	actualDef.(*corev1.PersistentVolume).Spec.AccessModes[0] = corev1.ReadOnlyMany
	if equal(actualDef, expectedDef) {
		t.Fatalf("Expected defs not to be equal:\n%v\n%v", actualDef, expectedDef)
	}
}

func TestPVCEnsurable(t *testing.T) {
	pvce := pvcEnsurable(&sharedVolume)

	// Validate pvDefinition() which also hits pvNamespacedName()
	actualDef := pvce.(*util.EnsurableImpl).Definition
	expectedDef := pvce.GetType()
	test.LoadYAML(t, expectedDef, "pvc.yaml")
	test.DoDiff(t, expectedDef, actualDef, false)

	// Validate EqualFunc
	equal := pvce.(*util.EnsurableImpl).EqualFunc
	// They should be "equal" to start off
	if !equal(actualDef, expectedDef) {
		t.Fatalf("Expected defs to be equal:\n%v\n%v", actualDef, expectedDef)
	}
	// Muck with something we don't care about (EqualFunc shouldn't check ResourceVersion)
	actualDef.(metav1.Object).SetResourceVersion("abc")
	if !equal(actualDef, expectedDef) {
		t.Fatalf("Expected defs to be equal:\n%v\n%v", actualDef, expectedDef)
	}
	// Now muck with something we care about
	actualDef.(*corev1.PersistentVolumeClaim).Spec.AccessModes[0] = corev1.ReadOnlyMany
	if equal(actualDef, expectedDef) {
		t.Fatalf("Expected defs not to be equal:\n%v\n%v", actualDef, expectedDef)
	}
}
