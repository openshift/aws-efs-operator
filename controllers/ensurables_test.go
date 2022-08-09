package controllers

// Test cases for the PV and PVC Ensurables

import (
	"fmt"
	awsefsv1alpha1 "openshift/aws-efs-operator/api/v1alpha1"
	util "openshift/aws-efs-operator/pkg/util"

	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	fakeFSID      = "fs-0123cdef"
	fakeAPID      = "fsap-0123456789abcdef"
	fakeNamespace = "project1"
	fakeSVName    = "my-shared-volume"
)

var sharedVolume = awsefsv1alpha1.SharedVolume{
	ObjectMeta: metav1.ObjectMeta{
		Name:      fakeSVName,
		Namespace: fakeNamespace,
	},
	Spec: awsefsv1alpha1.SharedVolumeSpec{
		AccessPointID: fakeAPID,
		FileSystemID:  fakeFSID,
	},
}

//// validateSharedVolumeOwner makes sure that `toSharedVolume` on `def` returns a `Request` that points
//// to `sharedVolume`, proving that `def` was created using `setSharedVolumeOwner`, and that worked.
//func validateSharedVolumeOwner(t *testing.T, def runtime.Object, sharedVolume *awsefsv1alpha1.SharedVolume) {
//	// To run toSharedVolume, we have to create a MapObject
//	mo := handler.MapObject{
//		Meta:   def.(metav1.Object),
//		Object: def,
//	}
//	rqList := toSharedVolume(mo)
//	if len(rqList) != 1 {
//		t.Fatalf("Expected one Request, got %d: %v", len(rqList), rqList)
//	}
//	if rqList[0].Name != fakeSVName || rqList[0].Namespace != fakeNamespace {
//		t.Fatalf("Expected request for %s/%s but got %v", fakeNamespace, fakeSVName, rqList[0])
//	}
//}
//
//func TestPVEnsurable(t *testing.T) {
//	pve := pvEnsurable(&sharedVolume)
//
//	// Validate pvDefinition() which also hits pvNamespacedName()
//	actualDef := pve.(*util.EnsurableImpl).Definition
//	expectedDef := pve.GetType()
//	test.LoadYAML(t, expectedDef, "persistentvolume.yaml")
//	test.DoDiff(t, expectedDef, actualDef, false)
//
//	// Verify setSharedVolumeOwner worked by making the round trip to toSharedVolume
//	validateSharedVolumeOwner(t, pve.(*util.EnsurableImpl).Definition, &sharedVolume)
//
//	// Validate EqualFunc
//	equal := pve.(*util.EnsurableImpl).EqualFunc
//	// They should be "equal" to start off
//	if !equal(actualDef, expectedDef) {
//		t.Fatalf("Expected defs to be equal:\n%v\n%v", actualDef, expectedDef)
//	}
//	// Muck with something we don't care about (EqualFunc shouldn't check ResourceVersion)
//	actualDef.(metav1.Object).SetResourceVersion("abc")
//	if !equal(actualDef, expectedDef) {
//		t.Fatalf("Expected defs to be equal:\n%v\n%v", actualDef, expectedDef)
//	}
//	// Now muck with something we care about. Since we're using AlwaysEqual (see NOTE on
//	// pvEnsurable's EqualFunc), it will evaluate equal anyway.
//	actualDef.(*corev1.PersistentVolume).Spec.AccessModes[0] = corev1.ReadOnlyMany
//	if !equal(actualDef, expectedDef) {
//		t.Fatalf("Expected defs to evaluate equal (even though they're not):\n%v\n%v",
//			format(actualDef), format(expectedDef))
//	}
//}
//
//func TestPVCEnsurable(t *testing.T) {
//	pvce := pvcEnsurable(&sharedVolume)
//
//	// Validate pvDefinition() which also hits pvNamespacedName()
//	actualDef := pvce.(*util.EnsurableImpl).Definition
//	expectedDef := pvce.GetType()
//	test.LoadYAML(t, expectedDef, "pvc.yaml")
//	test.DoDiff(t, expectedDef, actualDef, false)
//
//	// Verify setSharedVolumeOwner worked by making the round trip to toSharedVolume
//	validateSharedVolumeOwner(t, pvce.(*util.EnsurableImpl).Definition, &sharedVolume)
//
//	// Validate EqualFunc
//	equal := pvce.(*util.EnsurableImpl).EqualFunc
//	// They should be "equal" to start off
//	if !equal(actualDef, expectedDef) {
//		t.Fatalf("Expected defs to be equal:\n%v\n%v", actualDef, expectedDef)
//	}
//	// Muck with something we don't care about (EqualFunc shouldn't check ResourceVersion)
//	actualDef.(metav1.Object).SetResourceVersion("abc")
//	if !equal(actualDef, expectedDef) {
//		t.Fatalf("Expected defs to be equal:\n%v\n%v", actualDef, expectedDef)
//	}
//	// Now muck with something we care about
//	actualDef.(*corev1.PersistentVolumeClaim).Spec.AccessModes[0] = corev1.ReadOnlyMany
//	if equal(actualDef, expectedDef) {
//		t.Fatalf("Expected defs not to be equal:\n%v\n%v", actualDef, expectedDef)
//	}
//}

// TestCache is because I originally had a bug in how I was keying the caches. Makes sure that
// we don't collide if we have SharedVolumes with the same name in different namespaces.
func TestCache(t *testing.T) {
	fsid2 := "fsid2"
	apid2 := "apid2"
	ns2 := "project2"
	// Make a different SharedVolume with the same name in a different namespace.
	sv2 := awsefsv1alpha1.SharedVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fakeSVName,
			Namespace: ns2,
		},
		Spec: awsefsv1alpha1.SharedVolumeSpec{
			AccessPointID: apid2,
			FileSystemID:  fsid2,
		},
	}

	// PVs
	pv1 := pvEnsurable(&sharedVolume).(*util.EnsurableImpl).Definition.(*corev1.PersistentVolume)
	expVolumeHandle1 := fmt.Sprintf("%s::%s", fakeFSID, fakeAPID)
	if pv1.Spec.CSI.VolumeHandle != expVolumeHandle1 {
		t.Fatalf("Expected PV ensurable to correspond to\nSharedVolume %v\nbut got\nPV %v",
			format(sharedVolume), format(pv1))
	}
	expVolumeHandle2 := fmt.Sprintf("%s::%s", fsid2, apid2)
	pv2 := pvEnsurable(&sv2).(*util.EnsurableImpl).Definition.(*corev1.PersistentVolume)
	if pv2.Spec.CSI.VolumeHandle != expVolumeHandle2 {
		t.Fatalf("Expected PV ensurable to correspond to\nSharedVolume %v\nbut got\nPV %v",
			format(sv2), format(pv2))
	}

	// Same thing with PVCs
	pvc1 := pvcEnsurable(&sharedVolume).(*util.EnsurableImpl).Definition.(*corev1.PersistentVolumeClaim)
	if pvc1.Spec.VolumeName != "pv-project1-my-shared-volume" {
		t.Fatalf("Expected PVC ensurable to correspond to\nSharedVolume %v\nbut got\nPVC %v",
			format(sharedVolume), format(pvc1))
	}
	pvc2 := pvcEnsurable(&sv2).(*util.EnsurableImpl).Definition.(*corev1.PersistentVolumeClaim)
	if pvc2.Spec.VolumeName != "pv-project2-my-shared-volume" {
		t.Fatalf("Expected PVC ensurable to correspond to\nSharedVolume %v\nbut got\nPVC %v",
			format(sv2), format(pvc2))
	}
}