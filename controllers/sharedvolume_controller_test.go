package controllers

import (
	"encoding/json"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	awsefsv1alpha1 "openshift/aws-efs-operator/api/v1alpha1"
	"openshift/aws-efs-operator/pkg/fixtures"
	"openshift/aws-efs-operator/pkg/test"
	"openshift/aws-efs-operator/pkg/util"
	"runtime/debug"

	"context"
	"testing"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	k8serrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	// TODO: pkg/client/fake is deprecated, replace with pkg/envtest
	// // nolint:staticcheck
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// var fakeNS=types.NamespacedName{
// 	Name:      "bogus-name",
// 	Namespace: "bogus-namespace",
// }
// TODO: Test add()/watches somehow?

// fakeReconciler returns a SharedVolumeReconciler with a fake (as opposed to mocked)
// controller-runtime client. Use this when a test wants realistic, but good-path-only, REST client
// behavior. Use `setupMock` instead if you need to customize error conditions.
func fakeReconciler() *SharedVolumeReconciler {
	sch := scheme.Scheme
	sch.AddKnownTypes(
		awsefsv1alpha1.GroupVersion,
		&awsefsv1alpha1.SharedVolume{},
		&awsefsv1alpha1.SharedVolumeList{},
	)

	return &SharedVolumeReconciler{
		// client: fake.NewFakeClientWithScheme(sch),
		client: fake.NewClientBuilder().WithScheme(sch).Build(),
		scheme: sch,
	}
}

// mockReconciler returns a SharedVolumeReconciler with a mocked (as opposed to fake)
// controller-runtime client. The mock client itself is returned so it can be EXPECT()ed, etc.
// Use this when a fake client won't do, e.g. when you need to simulate an unexpected error.
func mockReconciler(ctrl *gomock.Controller) (*SharedVolumeReconciler, *fixtures.MockClient) {
	client := fixtures.NewMockClient(ctrl)
	rsv := &SharedVolumeReconciler{
		client: client,
		// Scheme is unused, so leave it nil
	}
	return rsv, client
}

// These save typing and allow us to abstract the Stringer interface
type svMapType map[string]*awsefsv1alpha1.SharedVolume
type pvMapType map[string]*corev1.PersistentVolume
type pvcMapType map[string]*corev1.PersistentVolumeClaim

// format takes advantage of json marshaling to produce a readable string representation of a struct.
func format(obj interface{}) string {
	p, err := json.MarshalIndent(obj, "", "\t")
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s\n", p)
}

func (m svMapType) String() string  { return format(m) }
func (m pvMapType) String() string  { return format(m) }
func (m pvcMapType) String() string { return format(m) }

// getResources returns three maps, keyed by "$namespace/$name", of the SharedVolume,
// PersistentVolume, and PersistentVolumeClaim resources found by querying the `client`.
func getResources(t *testing.T, client crclient.Client) (svMapType, pvMapType, pvcMapType) {

	svList := &awsefsv1alpha1.SharedVolumeList{}
	if err := client.List(context.TODO(), svList); err != nil {
		t.Fatal(err)
	}
	pvList := &corev1.PersistentVolumeList{}
	if err := client.List(context.TODO(), pvList); err != nil {
		t.Fatal(err)
	}
	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := client.List(context.TODO(), pvcList); err != nil {
		t.Fatal(err)
	}

	keyfunc := func(o metav1.Object) string {
		return fmt.Sprintf("%s/%s", o.GetNamespace(), o.GetName())
	}

	svMap := make(map[string]*awsefsv1alpha1.SharedVolume)
	for i := range svList.Items {
		sharedVolume := &svList.Items[i]
		svMap[keyfunc(sharedVolume)] = sharedVolume
	}
	pvMap := make(map[string]*corev1.PersistentVolume)
	for i := range pvList.Items {
		persistentVolume := &pvList.Items[i]
		pvMap[keyfunc(persistentVolume)] = persistentVolume
	}
	pvcMap := make(map[string]*corev1.PersistentVolumeClaim)
	for i := range pvcList.Items {
		persistentVolumeClaim := &pvcList.Items[i]
		pvcMap[keyfunc(persistentVolumeClaim)] = persistentVolumeClaim
	}

	return svMap, pvMap, pvcMap
}

// validateResources is for all-green scenarios. It queries the client for SharedVolume,
// PersistentVolume, and PersistentVolumeClaim resources. After any given Reconcile loop
// stabilizes, there should be an equal number of these, which we validate to be equal to
// `expectedCount`. The namespace and name of the PVs and PVCs are validated as matching
// their SharedVolumes, and each SharedVolume's Status is checked -- it should be Ready
// and refer to the corresponding PVC.
// For further inspection by the caller, we return three maps, keyed by "$namespace/$name",
// to the SharedVolume, PersistentVolume, and PVC resources.
func validateResources(
	t *testing.T, client crclient.Client, expectedCount int) (svMapType, pvMapType, pvcMapType) {

	svMap, pvMap, pvcMap := getResources(t, client)

	if len(svMap) != expectedCount || len(pvMap) != expectedCount || len(pvcMap) != expectedCount {
		t.Fatalf(
			"Didn't get the expected number of resources (%d):\nSharedVolumes: %s\nPVs: %s\nPVCs: %s\n%s",
			expectedCount, svMap, pvMap, pvcMap, debug.Stack())
	}

	// Without duplicating all the logic linking a SharedVolume to its corresponding PV and PVC
	// (that's tested elsewhere) make sure we got all the names and namespaces we expected.
	for _, sv := range svMap {
		var pvc *corev1.PersistentVolumeClaim
		var ok bool

		expectPVKey := fmt.Sprintf("/%s", pvNameForSharedVolume(sv))
		if _, ok = pvMap[expectPVKey]; !ok {
			t.Fatalf("Didn't find expected PV entry with key %s\npvMap: %s", expectPVKey, pvMap)
		}
		expectPVCKey := fmt.Sprintf("%s/%s", sv.Namespace, pvcNamespacedName(sv).Name)
		if pvc, ok = pvcMap[expectPVCKey]; !ok {
			t.Fatalf("Didn't find expected PVC entry with key %s\npvcMap: %s", expectPVCKey, pvcMap)
		}

		// Check the SharedVolume's Status
		if sv.Status.Phase != awsefsv1alpha1.SharedVolumeReady {
			t.Fatalf("Expected Ready status, but got %s", sv.Status.Phase)
		}
		if sv.Status.ClaimRef.Name != pvc.Name {
			t.Fatalf("Expected the SharedVolume's ClaimRef to point to %s but got %v",
				pvc.Name, format(sv.Status.ClaimRef))
		}
	}

	return svMap, pvMap, pvcMap
}

func makeRequest(t *testing.T, sv *awsefsv1alpha1.SharedVolume) reconcile.Request {
	nsname := crclient.ObjectKeyFromObject(sv)
	return reconcile.Request{
		NamespacedName: nsname,
	}
}

func TestReconcile(t *testing.T) {
	const (
		nsx = "namespace-x"
		nsy = "namespace-y"
		sva = "sv-a"
		svb = "sv-b"
		fs1 = "fs-000001"
		fs2 = "fs-000002"
		apd = "fsap-1111111d"
		ape = "fsap-2222222e"
	)
	var (
		sv1, sv2   *awsefsv1alpha1.SharedVolume
		svMap      svMapType
		pvMap      pvMapType
		pvcMap     pvcMapType
		req        reconcile.Request
		res        reconcile.Result
		err        error
		finalizers []string
	)
	r := fakeReconciler()

	ctx := context.TODO()
	lr := util.NewTestLogger().Logger()
	ctx = logr.NewContext(ctx, lr)

	// Verify there are no SharedVolumes, PVs, or PVCs
	validateResources(t, r.client, 0)

	// Green path: create a SharedVolume resource and reconcile; the corresponding PV and PVC
	// should be created.
	sv1 = &awsefsv1alpha1.SharedVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sva,
			Namespace: nsx,
		},
		Spec: awsefsv1alpha1.SharedVolumeSpec{
			AccessPointID: apd,
			FileSystemID:  fs1,
		},
	}
	if err = r.client.Create(ctx, sv1); err != nil {
		t.Fatal(err)
	}
	req = makeRequest(t, sv1)
	// Since the SV is new, the first reconcile loop just adds our finalizer and requeues
	if res, err = r.Reconcile(ctx, req); res != test.RequeueResult || err != nil {
		t.Fatalf("Expected requeue, no error; got\nresult: %v\nerr: %v", res, err)
	}
	// Make sure the finalizer got added
	if err = r.client.Get(ctx, req.NamespacedName, sv1); err != nil {
		t.Fatal(err)
	}
	finalizers = sv1.GetFinalizers()
	if len(finalizers) != 1 || finalizers[0] != svFinalizer {
		t.Fatalf("Didn't find our finalizer;\nExpected: {%s}\nGot:     %v", svFinalizer, finalizers)
	}
	// Our SharedVolume should be the only thing that exists
	svMap, pvMap, pvcMap = getResources(t, r.client)
	if len(svMap) != 1 || len(pvMap) != 0 || len(pvcMap) != 0 {
		t.Fatalf("Expected only our SharedVolume resource, but got\nSharedVolumes: %s\nPVs: %s\nPVCs: %s",
			svMap, pvMap, pvcMap)
	}
	// And The second time through we update the status. First make sure it's unset
	if sv1.Status.Phase != "" || sv1.Status.ClaimRef.Name != "" {
		t.Fatalf("Expected uninitialized Status, but got %v", sv1.Status)
	}
	if res, err = r.Reconcile(ctx, req); res != test.RequeueResult || err != nil {
		t.Fatalf("Expected requeue, no error; got\nresult: %v\nerr: %v", res, err)
	}
	// And now it should be Pending
	if err = r.client.Get(ctx, req.NamespacedName, sv1); err != nil {
		t.Fatal(err)
	}
	if sv1.Status.Phase != awsefsv1alpha1.SharedVolumePending || sv1.Status.ClaimRef.Name != "" {
		t.Fatalf("Expected Pending Status (no claim), but got %v", sv1.Status)
	}
	// Our SharedVolume should still be the only thing that exists
	svMap, pvMap, pvcMap = getResources(t, r.client)
	if len(svMap) != 1 || len(pvMap) != 0 || len(pvcMap) != 0 {
		t.Fatalf("Expected only our SharedVolume resource, but got\nSharedVolumes: %s\nPVs: %s\nPVCs: %s",
			svMap, pvMap, pvcMap)
	}
	// Finally, this Reconcile should
	// - Create the PV and PVC
	// - Mark the status Ready with the reference to the PVC
	// - Not requeue
	if res, err = r.Reconcile(ctx, req); res != test.NullResult || err != nil {
		t.Fatalf("Expected no requeue, no error; got\nresult: %v\nerr: %v", res, err)
	}
	validateResources(t, r.client, 1)
	// Doing it again should be a no-op
	if res, err = r.Reconcile(ctx, req); res != test.NullResult || err != nil {
		t.Fatalf("Expected no requeue, no error; got\nresult: %v\nerr: %v", res, err)
	}
	validateResources(t, r.client, 1)

	// Let's create another in a different namespace but with the same access point
	sv2 = &awsefsv1alpha1.SharedVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svb,
			Namespace: nsy,
		},
		Spec: awsefsv1alpha1.SharedVolumeSpec{
			AccessPointID: apd,
			FileSystemID:  fs1,
		},
	}
	if err = r.client.Create(ctx, sv2); err != nil {
		t.Fatalf("Error creating SharedVolume: %v", err)
	}
	req = makeRequest(t, sv2)
	if res, err = r.Reconcile(ctx, req); res != test.RequeueResult || err != nil {
		t.Fatalf("Expected requeue, no error; got\nresult: %v\nerr: %v", res, err)
	}
	if res, err = r.Reconcile(ctx, req); res != test.RequeueResult || err != nil {
		t.Fatalf("Expected requeue, no error; got\nresult: %v\nerr: %v", res, err)
	}
	if res, err = r.Reconcile(ctx, req); res != test.NullResult || err != nil {
		t.Fatalf("Expected no requeue, no error; got\nresult: %v\nerr: %v", res, err)
	}
	svMap, _, _ = validateResources(t, r.client, 2)

	// Test the `uneditSharedVolume` path: If we change sv2's FSID and APID, Reconcile ought to revert them.
	// Make sure we're using the sv2 from the server
	sv2 = svMap[fmt.Sprintf("%s/%s", nsy, svb)]
	sv2.Spec.AccessPointID = ape
	sv2.Spec.FileSystemID = fs2
	if err = r.client.Update(ctx, sv2); err != nil {
		t.Fatal(err)
	}
	// This should ask to requeue so the next run through can take a greener path
	if res, err = r.Reconcile(ctx, req); res != test.RequeueResult || err != nil {
		t.Fatalf("Expected requeue, no error; got\nresult: %v\nerr: %v", res, err)
	}
	// There should (still) be two of each resource, but let's check the SV by hand
	svMap, _, _ = validateResources(t, r.client, 2)
	sv2 = svMap[fmt.Sprintf("%s/%s", nsy, svb)]
	if sv2.Spec.AccessPointID != apd {
		t.Fatalf("Expected access point ID to be reverted to %s, but got %s", apd, sv2.Spec.AccessPointID)
	}
	if sv2.Spec.FileSystemID != fs1 {
		t.Fatalf("Expected file system ID to be reverted to %s, but got %s", fs1, sv2.Spec.FileSystemID)
	}
	// And we should be back to gold
	_, pvMap, _ = validateResources(t, r.client, 2)

	// Let's do that again with a "legacy" PV -- one with the access point in the MountOptions.
	sv2.Spec.AccessPointID = ape
	sv2.Spec.FileSystemID = fs2
	if err = r.client.Update(ctx, sv2); err != nil {
		t.Fatal(err)
	}
	pvname := fmt.Sprintf("/%s", pvNameForSharedVolume(sv2))
	pv := pvMap[pvname]
	pv.Spec.CSI.VolumeHandle = fs1
	pv.Spec.MountOptions = []string{
		"tls",
		fmt.Sprintf("accesspoint=%s", apd),
	}
	if err = r.client.Update(ctx, pv); err != nil {
		t.Fatal(err)
	}
	// This should ask to requeue so the next run through can take a greener path
	if res, err = r.Reconcile(ctx, req); res != test.RequeueResult || err != nil {
		t.Fatalf("Expected requeue, no error; got\nresult: %v\nerr: %v", res, err)
	}
	// There should (still) be two of each resource, but let's check the SV by hand
	svMap, _, _ = validateResources(t, r.client, 2)
	sv2 = svMap[fmt.Sprintf("%s/%s", nsy, svb)]
	if sv2.Spec.AccessPointID != apd {
		t.Fatalf("Expected access point ID to be reverted to %s, but got %s", apd, sv2.Spec.AccessPointID)
	}
	if sv2.Spec.FileSystemID != fs1 {
		t.Fatalf("Expected file system ID to be reverted to %s, but got %s", fs1, sv2.Spec.FileSystemID)
	}
	// And we should be back to gold
	_, pvMap, pvcMap = validateResources(t, r.client, 2)

	// Make sure a deleted PV/PVC is restored.
	pvcnsname := pvcNamespacedName(sv2)
	if err = r.client.Delete(ctx, pvcMap[fmt.Sprintf("%s/%s", pvcnsname.Namespace, pvcnsname.Name)]); err != nil {
		t.Fatal(err)
	}
	if err = r.client.Delete(ctx, pvMap[pvname]); err != nil {
		t.Fatal(err)
	}
	if res, err = r.Reconcile(ctx, req); res != test.NullResult || err != nil {
		t.Fatalf("Expected no requeue, no error; got\nresult: %v\nerr: %v", res, err)
	}

	// validateResources proves the PV and PVC came back.
	svMap, pvMap, _ = validateResources(t, r.client, 2)

	// The PV came back but we need to update the local cached version of the PV since its at
	// revision 2 and the server version after the delete and Ensure is now at revision 1
	pv = pvMap[pvname]

	// Hit some uneditSharedVolume corner cases. These will panic in uneditSharedVolume, which is
	// recovered and spoofed as a non-error on the theory that the main Reconcile should overwrite
	// the PV. But it won't do that, because PVs can't be edited (which means we shouldn't get here
	// in the first place).

	recoverPV := func() {
		if err := r.client.Delete(ctx, pvMap[pvname]); err != nil {
			t.Fatal(err)
		}
		delete(pvBySharedVolume, svKey(svMap[fmt.Sprintf("%s/%s", nsy, svb)]))
		if res, err = r.Reconcile(ctx, req); res != test.NullResult || err != nil {
			t.Fatalf("Expected no requeue, no error; got\nresult: %v\nerr: %v", res, err)
		}
		// validateResources proves the PV came back.
		_, pvMap, _ = validateResources(t, r.client, 2)
		pv = pvMap[pvname]
	}

	// 1) Trigger a "real" panic where uneditSharedVolume tries to dereference CSI, which is nil.
	pv.Spec.CSI = nil
	if err = r.client.Update(ctx, pv); err != nil {
		t.Fatal(err)
	}
	if res, err = r.Reconcile(ctx, req); res != test.NullResult || err != nil {
		t.Fatalf("Expected no requeue, no error; got\nresult: %v\nerr: %v", res, err)
	}
	_, pvMap, _ = validateResources(t, r.client, 2)
	pv = pvMap[pvname]
	// The PV didn't change
	if pv.Spec.CSI != nil {
		t.Fatalf("Expected PV not to be restored, but got %v", format(pv))
	}
	recoverPV()

	// 2) VolumeHandle is empty
	pv.Spec.CSI.VolumeHandle = ""
	if err = r.client.Update(ctx, pv); err != nil {
		t.Fatal(err)
	}
	if res, err = r.Reconcile(ctx, req); res != test.NullResult || err != nil {
		t.Fatalf("Expected no requeue, no error; got\nresult: %v\nerr: %v", res, err)
	}
	_, pvMap, _ = validateResources(t, r.client, 2)
	pv = pvMap[pvname]
	// The PV didn't change
	if pv.Spec.CSI.VolumeHandle != "" {
		t.Fatalf("Expected PV's VolumeHandle not to be restored, but got %v", format(pv))
	}
	recoverPV()

	// 3) VolumeHandle is downright malformed
	bogusVolHandle := fmt.Sprintf("%s:%s", fs1, apd)
	pv.Spec.CSI.VolumeHandle = bogusVolHandle
	if err = r.client.Update(ctx, pv); err != nil {
		t.Fatal(err)
	}
	if res, err = r.Reconcile(ctx, req); res != test.NullResult || err != nil {
		t.Fatalf("Expected no requeue, no error; got\nresult: %v\nerr: %v", res, err)
	}
	_, pvMap, _ = validateResources(t, r.client, 2)
	pv = pvMap[pvname]
	if pv.Spec.CSI.VolumeHandle != bogusVolHandle {
		t.Fatalf("Expected PV's VolumeHandle not to be restored, but got %v", format(pv))
	}
	recoverPV()

	// 4) APID is missing from the MountOptions. To trigger this, we have to force the old style
	//    VolumeHandle.
	pv.Spec.CSI.VolumeHandle = fs1
	pv.Spec.MountOptions = []string{}
	if err = r.client.Update(ctx, pv); err != nil {
		t.Fatal(err)
	}
	if res, err = r.Reconcile(ctx, req); res != test.NullResult || err != nil {
		t.Fatalf("Expected no requeue, no error; got\nresult: %v\nerr: %v", res, err)
	}
	svMap, pvMap, _ = validateResources(t, r.client, 2)
	pv = pvMap[pvname]
	if pv.Spec.CSI.VolumeHandle != fs1 || len(pv.Spec.MountOptions) != 0 {
		t.Fatalf("Expected PV not to be restored, but got %v", format(pv))
	}
	recoverPV()

	// Test the delete path.
	// This doesn't actually need a real timestamp
	delTime := metav1.Now()
	sv2 = svMap[fmt.Sprintf("%s/%s", nsy, svb)]
	sv2.DeletionTimestamp = &delTime
	if err = r.client.Update(ctx, sv2); err != nil {
		t.Fatal(err)
	}
	if res, err = r.Reconcile(ctx, req); res != test.NullResult || err != nil {
		t.Fatalf("Expected no requeue, no error; got\nresult: %v\nerr: %v", res, err)
	}
	// The PV and PVC should be gone, but the SV is still there
	svMap, pvMap, pvcMap = getResources(t, r.client)
	if len(svMap) != 1 || len(pvMap) != 1 || len(pvcMap) != 1 {
		t.Fatalf("Expected one SharedVolume resources and on PV & PVC, but got\nSharedVolumes: %s\nPVs: %s\nPVCs: %s",
			svMap, pvMap, pvcMap)
	}

	// Another reconcile at this stage should be a no-op
	if res, err = r.Reconcile(ctx, req); res != test.NullResult || err != nil {
		t.Fatalf("Expected no requeue, no error; got\nresult: %v\nerr: %v", res, err)
	}
	svMap, pvMap, pvcMap = getResources(t, r.client)
	if len(svMap) != 1 || len(pvMap) != 1 || len(pvcMap) != 1 {
		t.Fatalf("Expected one SharedVolume resources and on PV & PVC, but got\nSharedVolumes: %s\nPVs: %s\nPVCs: %s",
			svMap, pvMap, pvcMap)
	}
	//lint not used
	// sv2 = svMap[fmt.Sprintf("%s/%s", nsy, svb)]

	validateResources(t, r.client, 1)
	// This reconcile ought to hit our "deleted out of band" path, which is a no-op.
	if res, err = r.Reconcile(ctx, req); res != test.NullResult || err != nil {
		t.Fatalf("Expected no requeue, no error; got\nresult: %v\nerr: %v", res, err)
	}
	validateResources(t, r.client, 1)
}

// TestReconcileUnexpected makes sure the reconciler doesn't freak out if it gets a request for a
// nonexistent SharedVolume. This shouldn't really happen (except in the case of deletions) but
// it's possible to contrive by e.g. building a PV or PVC with our special labels.
func TestReconcileUnexpected(t *testing.T) {
	r := fakeReconciler()
	ctx := context.TODO()
	lr := util.NewTestLogger().Logger()
	ctx = logr.NewContext(ctx, lr)

	rq := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "bogus-name",
			Namespace: "bogus-namespace",
		},
	}

	if res, err := r.Reconcile(ctx, rq); res != test.NullResult || err != nil {
		t.Fatalf("Expected no requeue, no error; got\nresult: %v\nerr: %v", res, err)
	}
	// Nothing should have been created.
	validateResources(t, r.client, 0)
}

// TestReconcileGetError hits the path where the initial GET fails for non-404 reasons
func TestReconcileGetError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	r, client := mockReconciler(ctrl)

	ctx := context.TODO()
	lr := util.NewTestLogger().Logger()
	ctx = logr.NewContext(ctx, lr)

	nsname := types.NamespacedName{
		Namespace: "ns",
		Name:      "name",
	}
	req := reconcile.Request{
		NamespacedName: nsname,
	}
	// Not realistic, we're just contriving a way to make Get fail
	alreadyExists := k8serrs.NewAlreadyExists(schema.GroupResource{}, nsname.Name)
	// theError := fixtures.AlreadyExists

	// We don't especially care about the call args; they're validated in other tests
	client.EXPECT().Get(gomock.Any(), nsname, gomock.Any()).Return(alreadyExists)

	if res, err := r.Reconcile(ctx, req); res != test.NullResult || err != alreadyExists {
		t.Fatalf("Expected no requeue and error %v; got\nresult: %v\nerr: %v", alreadyExists, res, err)
	}
}

// TestUneditGetError tests the `uneditSharedVolume` code path where `Get`ting the PV fails for
// non-404 reasons.
func TestUneditGetError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	r, client := mockReconciler(ctrl)

	ctx := context.TODO()
	lr := util.NewTestLogger().Logger()
	ctx = logr.NewContext(ctx, lr)

	sv := &awsefsv1alpha1.SharedVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
		},
	}
	svNSName := crclient.ObjectKeyFromObject(sv)

	// The expected NamespacedName for the PV we'll try to retrieve. Hardcoded to avoid SHT.
	pvname := "pv-bar-foo"
	pvNSName := types.NamespacedName{
		Name: pvname,
	}
	alreadyExists := k8serrs.NewAlreadyExists(schema.GroupResource{}, sv.Name)

	gomock.InOrder(
		client.EXPECT().Get(gomock.Any(), svNSName, &awsefsv1alpha1.SharedVolume{}).Do(
			// The Get() call populates the SharedVolume object
			func(ctx context.Context, key crclient.ObjectKey, obj runtime.Object) {
				*obj.(*awsefsv1alpha1.SharedVolume) = *sv
			},
		),
		client.EXPECT().Get(gomock.Any(), pvNSName, &corev1.PersistentVolume{}).Return(alreadyExists),
	)

	if res, err := r.Reconcile(ctx, makeRequest(t, sv)); res != test.NullResult || err != alreadyExists {
		t.Fatalf("Expected no requeue and an error, but got\nresult: %v\nerr: %v", res, err)
	}
}

// TestUneditUpdateError tests the `uneditSharedVolume` code path where updating the SharedVolume fails.
func TestUneditUpdateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	r, client := mockReconciler(ctrl)

	ctx := context.TODO()
	lr := util.NewTestLogger().Logger()
	ctx = logr.NewContext(ctx, lr)

	sv := &awsefsv1alpha1.SharedVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
		},
		Spec: awsefsv1alpha1.SharedVolumeSpec{
			AccessPointID: "ap",
			FileSystemID:  "fs",
		},
	}
	svNSName := crclient.ObjectKeyFromObject(sv)

	// The PV we'll retrieve.
	pve := pvEnsurable(sv)
	pv := pve.(*util.EnsurableImpl).Definition.(*corev1.PersistentVolume)
	// Make this trigger the unedit path
	pv.Spec.CSI.VolumeHandle = "abc123::ap"

	// The version of SharedVolume we expect to be passed to Update() will have that changed FSID
	svUpdate := sv.DeepCopy()
	svUpdate.Spec.FileSystemID = "abc123"
	notFound := k8serrs.NewNotFound(schema.GroupResource{}, sv.Name)
	gomock.InOrder(
		client.EXPECT().Get(gomock.Any(), svNSName, &awsefsv1alpha1.SharedVolume{}).Do(
			// The first Get() call populates the SharedVolume object
			func(ctx context.Context, key crclient.ObjectKey, obj runtime.Object) {
				*obj.(*awsefsv1alpha1.SharedVolume) = *sv
			},
		),
		client.EXPECT().Get(gomock.Any(), pve.GetNamespacedName(), &corev1.PersistentVolume{}).Do(
			// The second Get() populates the PersistentVolume object
			func(ctx context.Context, key crclient.ObjectKey, obj runtime.Object) {
				*obj.(*corev1.PersistentVolume) = *pv
			},
		),

		client.EXPECT().Update(gomock.Any(), svUpdate).Return(notFound),
	)

	if res, err := r.Reconcile(ctx, makeRequest(t, sv)); res != test.NullResult || err != notFound {
		t.Fatalf("Expected no requeue and an error, but got\nresult: %v\nerr: %v", res, err)
	}

}

// TODO: This is kind of goofy -- is there not a better way to define a simple Matcher inline?
type matchFinalizer struct{}

func (m matchFinalizer) String() string {
	return fmt.Sprintf("has finalizer %s", svFinalizer)
}
func (m matchFinalizer) Matches(x interface{}) bool {
	finalizers := x.(metav1.Object).GetFinalizers()
	for _, f := range finalizers {
		if f == svFinalizer {
			return true
		}
	}
	return false
}

// TestFinalizerUpdateError covers the path where we fail to update the SharedVolume with
// the finalizer.
func TestFinalizerUpdateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	r, client := mockReconciler(ctrl)

	ctx := context.TODO()
	lr := util.NewTestLogger().Logger()
	ctx = logr.NewContext(ctx, lr)

	sv := &awsefsv1alpha1.SharedVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
		},
	}

	notFound := k8serrs.NewNotFound(schema.GroupResource{}, sv.Name)
	gomock.InOrder(
		// First the reconciler gets the SharedVolume
		client.EXPECT().Get(gomock.Any(), gomock.Any(), &awsefsv1alpha1.SharedVolume{}).Return(nil),
		// uneditSharedVolume checks for the PV. We'll say it's 404 to make unedit return quick.
		client.EXPECT().Get(gomock.Any(), gomock.Any(), &corev1.PersistentVolume{}).Return(notFound),
		// Now we add the finalizer and try to update; trigger the error there.
		client.EXPECT().Update(gomock.Any(), matchFinalizer{}).Return(notFound),
	)

	if res, err := r.Reconcile(ctx, makeRequest(t, sv)); res != test.NullResult || err != notFound {
		t.Fatalf("Expected no requeue and an error, but got\nresult: %v\nerr: %v", res, err)
	}

}

// hijackEnsurable makes it so that the next time the ensurable corresponding to the resource type
// `rtype` (which should be an instance of either *PersistentVolume or *PersistentVolumeClaim) for
// the SharedVolume `sv` is accessed, `ensurable` is returned instead of whatever you would
// otherwise expect. This should be used (sparingly - it's a hack) to "mock" the behavior of a PV
// or PVC Ensurable in a test flow that is otherwise out of our control, like an end-to-end
// Reconcile with a fake (as opposed to mocked) client.
func hijackEnsurable(rtype runtime.Object, sv *awsefsv1alpha1.SharedVolume, ensurable util.Ensurable) {
	// Replace the value in the global cache.
	// TODO: This sucks, and has the potential to blow up if tests run in parallel. They don't
	// at the time of this writing, but...
	key := svKey(sv)
	switch rtype.(type) {
	case *corev1.PersistentVolume:
		pvBySharedVolume[key] = ensurable
	case *corev1.PersistentVolumeClaim:
		pvcBySharedVolume[key] = ensurable
	default:
		panic(fmt.Sprintf("rtype argument must be an instance of *PersistentVolume or *PersistentVolumeClaim; got %T", rtype))
	}
}

// TestEnsureFails covers the code paths where `Ensure`ing the PV or PVC fails. The
// SharedVolume gets Failed Status.Phase with a Message corresponding to the error from Ensure.
func TestEnsureFails(t *testing.T) {
	// Mock controller for *just* the Ensurable, not the client or logger.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	r := fakeReconciler()

	ctx := context.TODO()
	lr := util.NewTestLogger().Logger()
	ctx = logr.NewContext(ctx, lr)

	sv := &awsefsv1alpha1.SharedVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sv",
			Namespace: "proj1",
		},
	}
	if err := r.client.Create(ctx, sv); err != nil {
		t.Fatal(err)
	}

	req := makeRequest(t, sv)

	if res, err := r.Reconcile(ctx, req); res != test.RequeueResult || err != nil {
		t.Fatalf("Expected requeue result, no error, but got\nresult: %v\nerr: %v", res, err)
	}
	if res, err := r.Reconcile(ctx, req); res != test.RequeueResult || err != nil {
		t.Fatalf("Expected requeue result, no error, but got\nresult: %v\nerr: %v", res, err)
	}

	svMap, pvMap, pvcMap := getResources(t, r.client)
	if len(svMap) != 1 || len(pvMap) != 0 || len(pvcMap) != 0 {
		t.Errorf("Expected only our SharedVolume resource, but got\nSharedVolumes: %s\nPVs: %s\nPVCs: %s",
			svMap, pvMap, pvcMap)
	}
	// Sanity-check the initial SV Status
	sv = svMap["proj1/sv"]
	if sv.Status.Phase != awsefsv1alpha1.SharedVolumePending || sv.Status.Message != "" {
		t.Errorf("Expected Pending Phase and no Message but got %v", sv)
	}

	// Now subsequent Reconcile()s get to the ensuring; so set up our failures

	// "Mock" the PV Ensurable
	mockPVEnsurable := fixtures.NewMockEnsurable(ctrl)
	hijackEnsurable(&corev1.PersistentVolume{}, sv, mockPVEnsurable)
	// "Mock" the PVC Ensurable
	mockPVCEnsurable := fixtures.NewMockEnsurable(ctrl)
	hijackEnsurable(&corev1.PersistentVolumeClaim{}, sv, mockPVCEnsurable)
	notFound := k8serrs.NewNotFound(schema.GroupResource{}, sv.Name)
	alreadyExists := k8serrs.NewAlreadyExists(schema.GroupResource{}, sv.Name)
	// We'll do two runs through Reconcile()...
	gomock.InOrder(
		// On the first run, we'll make the PV's Ensure fail
		mockPVEnsurable.EXPECT().GetNamespacedName().Return(types.NamespacedName{}),
		mockPVEnsurable.EXPECT().Ensure(gomock.Any(), gomock.Any()).Return(notFound),
		// On the second run, make it pass so we get to the PVC's Ensure
		mockPVEnsurable.EXPECT().GetNamespacedName().Return(types.NamespacedName{}),
		mockPVEnsurable.EXPECT().Ensure(gomock.Any(), gomock.Any()).Return(nil),
		// Make PVC's Ensure fail. (Use a different error so we can distinguish.)
		mockPVCEnsurable.EXPECT().GetNamespacedName().Return(types.NamespacedName{}),
		mockPVCEnsurable.EXPECT().Ensure(gomock.Any(), gomock.Any()).Times(1).Return(alreadyExists),
	)

	// Do the first run. The NotFound error bubbles up from the PV's Ensure().
	if res, err := r.Reconcile(ctx, req); res != test.NullResult || err != notFound {
		t.Errorf("Expected no requeue and a error, got\nresult: %v\nerr: %v", res, err)
	}
	// That should have caused Reconcile to set the SharedVolume's Status to Failed
	svMap, pvMap, pvcMap = getResources(t, r.client)
	if len(svMap) != 1 || len(pvMap) != 0 || len(pvcMap) != 0 {
		t.Errorf("Expected only our SharedVolume resource, but got\nSharedVolumes: %s\nPVs: %s\nPVCs: %s",
			svMap, pvMap, pvcMap)
	}
	sv = svMap["proj1/sv"]
	if sv.Status.Phase != awsefsv1alpha1.SharedVolumeFailed || sv.Status.Message != notFound.Error() {
		t.Errorf("Expected Failed Phase and NotFound Message but got %v", sv)
	}
	// alreadyExists := k8serrs.NewAlreadyExists(schema.GroupResource{},sv.Name)
	if res, err := r.Reconcile(ctx, req); res != test.NullResult || err != alreadyExists {
		t.Errorf("Expected no requeue and a error, got\nresult: %v\nerr: %v", res, err)
	}
	// Note that the PV (and PVC) still hasn't been created because we mocked the guts out of its Ensure
	svMap, pvMap, pvcMap = getResources(t, r.client)
	if len(svMap) != 1 || len(pvMap) != 0 || len(pvcMap) != 0 {
		t.Errorf("Expected only our SharedVolume resource, but got\nSharedVolumes: %s\nPVs: %s\nPVCs: %s",
			svMap, pvMap, pvcMap)
	}
	// The second failure should have updated the status message to the other error
	sv = svMap["proj1/sv"]
	if sv.Status.Phase != awsefsv1alpha1.SharedVolumeFailed || sv.Status.Message != alreadyExists.Error() {
		t.Errorf("Expected Failed Phase and NotFound Message but got %v", sv)
	}
}

// TestHandleDeleteFails hits unusual failure paths in `handleDelete`
func TestHandleDeleteFails(t *testing.T) {
	// Make sure the caches are cleared from other tests
	pvBySharedVolume = make(map[string]util.Ensurable)
	pvcBySharedVolume = make(map[string]util.Ensurable)

	r := fakeReconciler()
	// We'll use this later to wrap the fake client to make it error where we want it
	realFakeClient := r.client

	ctx := context.TODO()
	lr := util.NewTestLogger().Logger()
	ctx = logr.NewContext(ctx, lr)

	sv := &awsefsv1alpha1.SharedVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sv",
			Namespace: "proj1",
		},
		Spec: awsefsv1alpha1.SharedVolumeSpec{
			AccessPointID: "fs-abc123abc123",
			FileSystemID:  "fs-123abc",
		},
	}

	if err := r.client.Create(ctx, sv); err != nil {
		t.Fatal(err)
	}

	// It takes three Reconciles to get to steady state. This sequence is validated thoroughly in
	// TestReconcile, so just rough it up here.
	req := makeRequest(t, sv)
	if res, err := r.Reconcile(ctx, req); res != test.RequeueResult || err != nil {
		t.Errorf("Expected requeue and no error, got\nresult: %v\nerr: %v", res, err)
	}
	if res, err := r.Reconcile(ctx, req); res != test.RequeueResult || err != nil {
		t.Errorf("Expected requeue and no error, got\nresult: %v\nerr: %v", res, err)
	}
	if res, err := r.Reconcile(ctx, req); res != test.NullResult || err != nil {
		t.Errorf("Expected no requeue and a error, got\nresult: %v\nerr: %v", res, err)
	}
	// This proves our SV/PV/PVC are all present and accounted for
	svMap, _, _ := validateResources(t, r.client, 1)
	sv = svMap["proj1/sv"]

	// Let's also make sure the caches are warm
	svk := svKey(sv)
	if _, ok := pvcBySharedVolume[svk]; !ok {
		t.Fatal("Expected the PVC cache to be warm")
	}
	if _, ok := pvBySharedVolume[svk]; !ok {
		t.Fatal("Expected the PV cache to be warm")
	}

	// Now mark the SV for deletion to trigger the handleDelete path
	delTime := metav1.Now()
	sv.DeletionTimestamp = &delTime
	if err := r.client.Update(ctx, sv); err != nil {
		t.Fatal(err)
	}
	alreadyExists := k8serrs.NewAlreadyExists(schema.GroupResource{}, sv.Name)

	// 1) Make the PVC Ensurable's Delete fail
	r.client = &test.FakeClientWithCustomErrors{
		Client: realFakeClient,
		DeleteBehavior: []error{
			// This has to be an error other than NotFound to make EnsurableImpl.Delete return it.
			// Beyond that, it doesn't much matter.
			alreadyExists,
		},
	}
	if res, err := r.Reconcile(ctx, req); res != test.NullResult || err != alreadyExists {
		t.Fatalf("Expected null result, AlreadyExists error, but got\nresult: %v\nerr: %v", res, err)
	}
	// The PVC should be gone from the cache, but the PV should not
	if _, ok := pvcBySharedVolume[svk]; ok {
		t.Fatal("Expected the PVC cache to be clear")
	}
	if _, ok := pvBySharedVolume[svk]; !ok {
		t.Fatal("Expected the PV cache to be warm")
	}
	// All three resources should still be there
	svMap, pvMap, pvcMap := getResources(t, r.client)
	if len(svMap) != 1 || len(pvMap) != 1 || len(pvcMap) != 1 {
		t.Fatalf("Expected one each of SVs, PVs, and PVCs, but got:\nSVs: %s\nPVs: %s\nPVCs: %s", svMap, pvMap, pvcMap)
	}
	sv = svMap["proj1/sv"]
	// The finalizer should still be there
	if len(sv.GetFinalizers()) != 1 {
		t.Fatalf("Expected 1 finalizer but found %v", sv.GetFinalizers())
	}

	// 2) Make the PV Ensurable's Delete fail (after the PVC Ensurable's Delete succeeds)
	r.client = &test.FakeClientWithCustomErrors{
		Client: realFakeClient,
		DeleteBehavior: []error{
			// The first Delete is the PVC's
			nil,
			// The second is the PV's
			alreadyExists,
		},
	}
	if res, err := r.Reconcile(ctx, req); res != test.NullResult || err != alreadyExists {
		t.Fatalf("Expected null result, AlreadyExists error, but got\nresult: %v\nerr: %v", res, err)
	}
	// Both the PVC and the PV should be gone from the cache
	if _, ok := pvcBySharedVolume[svk]; ok {
		t.Fatal("Expected the PVC cache to be clear")
	}
	if _, ok := pvBySharedVolume[svk]; ok {
		t.Fatal("Expected the PV cache to be clear")
	}
	// But only the PVC got deleted
	svMap, pvMap, pvcMap = getResources(t, r.client)
	if len(svMap) != 1 || len(pvMap) != 1 || len(pvcMap) != 0 {
		t.Fatalf("Expected one SVs and PV and no PVCs, but got:\nSVs: %s\nPVs: %s\nPVCs: %s", svMap, pvMap, pvcMap)
	}
	sv = svMap["proj1/sv"]
	// And the finalizer should still be there
	if len(sv.GetFinalizers()) != 1 {
		t.Fatalf("Expected 1 finalizer but found %v", sv.GetFinalizers())
	}

	// 3) Make the SV's Update (to clear the finalizer) fail.
	//    Note that when we start this sub-case, the PVC is already gone, so this also exercises
	//    the idempotency of that deletion.
	r.client = &test.FakeClientWithCustomErrors{
		Client: realFakeClient,
		UpdateBehavior: []error{
			// The SV's Update() to clear the finalizer is the first (and only) Update().
			// There's a .Status().Update() before it, but that's different.
			alreadyExists,
		},
	}
	if res, err := r.Reconcile(ctx, req); res != test.NullResult || err != alreadyExists {
		t.Fatalf("Expected null result, AlreadyExists error, but got\nresult: %v\nerr: %v", res, err)
	}
	// Both the PVC and the PV should be gone from the cache
	if _, ok := pvcBySharedVolume[svk]; ok {
		t.Fatal("Expected the PVC cache to be clear")
	}
	if _, ok := pvBySharedVolume[svk]; ok {
		t.Fatal("Expected the PV cache to be clear")
	}
	// And both got deleted
	svMap, pvMap, pvcMap = getResources(t, r.client)
	if len(svMap) != 1 || len(pvMap) != 0 || len(pvcMap) != 0 {
		t.Fatalf("Expected one SVs and no PVs or PVCs, but got:\nSVs: %s\nPVs: %s\nPVCs: %s", svMap, pvMap, pvcMap)
	}
	sv = svMap["proj1/sv"]
	// And the finalizer should still be there
	if len(sv.GetFinalizers()) != 1 {
		t.Fatalf("Expected 1 finalizer but found %v", sv.GetFinalizers())
	}

	// 4) Finally, let everything run through.
	//    We start off in a messy state where the PV and PVC are already gone, so this isn't
	//    *exactly* a green path. More... chartreuse.
	r.client = realFakeClient
	if res, err := r.Reconcile(ctx, req); res != test.NullResult || err != nil {
		t.Fatalf("Expected null result, no error, but got\nresult: %v\nerr: %v", res, err)
	}
	// Both the PVC and the PV should be gone from the cache
	if _, ok := pvcBySharedVolume[svk]; ok {
		t.Fatal("Expected the PVC cache to be clear")
	}
	if _, ok := pvBySharedVolume[svk]; ok {
		t.Fatal("Expected the PV cache to be clear")
	}
	// And all got deleted
	svMap, pvMap, pvcMap = getResources(t, r.client)
	if len(svMap) != 0 || len(pvMap) != 0 || len(pvcMap) != 0 {
		t.Fatalf("Expected one SVs and no PVs or PVCs, but got:\nSVs: %s\nPVs: %s\nPVCs: %s", svMap, pvMap, pvcMap)
	}
	//lint not used
	// sv = svMap["proj1/sv"]
}

// TestUpdateStatusFail covers the `updateStatus` path where the Update fails.
func TestUpdateStatusFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	r, client := mockReconciler(ctrl)
	// ctx := context.TODO()
	tl := util.NewTestLogger()
	logger := tl.Logger()
	// ctx = logr.NewContext(ctx, logger)

	sv := &awsefsv1alpha1.SharedVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "sv",
			Namespace:  "proj1",
			Finalizers: []string{svFinalizer},
		},
		Status: awsefsv1alpha1.SharedVolumeStatus{
			Phase: awsefsv1alpha1.SharedVolumePending,
		},
	}
	alreadyExists := k8serrs.NewAlreadyExists(schema.GroupResource{}, sv.Name)
	gomock.InOrder(
		//logger.EXPECT().Info("Updating SharedVolume status", "status", sv.Status),
		client.EXPECT().Status().Return(client),
		client.EXPECT().Update(gomock.Any(), sv).Return(alreadyExists),
		//logger.EXPECT().Error(fixtures.AlreadyExists, "Failed to update SharedVolume status"),
	)
	if err := r.updateStatus(logger, sv); err != alreadyExists {
		t.Fatalf("Expected AlreadyExists but got %v", err)
	}
	assert.Contains(t, tl.Messages(), "Failed to update SharedVolume status")
}
