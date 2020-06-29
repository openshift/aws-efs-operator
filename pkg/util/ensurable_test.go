package util

import (
	"context"
	"testing"

	fx "openshift/aws-efs-operator/pkg/fixtures"

	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

var todo context.Context = context.TODO()
var nsname types.NamespacedName = types.NamespacedName{}

type mocks struct {
	ensurable           *EnsurableImpl
	log                 *fx.MockLogger
	client              *fx.MockClient
	getTypeAndServerObj runtime.Object
	getterAndCachedObj  runtime.Object
}

func mkMocks(ctrl *gomock.Controller) mocks {
	// Instances of the underlying resource
	o1 := &corev1.Pod{}
	o2 := &corev1.Pod{}
	ensurable := EnsurableImpl{
		ObjType:        o1,
		NamespacedName: nsname,
		// Test should set DefinitionGetter and/or EqualFunc as necessary.
		// By not setting one of them, we're asserting it won't be called, since
		// doing so would attempt to dereference a nil function pointer.
	}
	return mocks{
		ensurable:           &ensurable,
		log:                 fx.NewMockLogger(ctrl),
		client:              fx.NewMockClient(ctrl),
		getTypeAndServerObj: o1,
		getterAndCachedObj:  o2,
	}
}

// TestEnsureNotFoundCreateError tests the path where our resource doesn't exist on the server,
// so we try to create it, but the creation errors.
func TestEnsureNotFoundCreateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mkMocks(ctrl)
	m.ensurable.Definition = m.getterAndCachedObj

	gomock.InOrder(
		// Get called with the ObjType
		m.client.EXPECT().Get(todo, nsname, m.getTypeAndServerObj).Return(fx.NotFound),
		m.log.EXPECT().Info("Creating.", "resource", nsname),
		// Create called with the return from DefinitionGetter
		m.client.EXPECT().Create(todo, m.getterAndCachedObj).Return(fx.AlreadyExists),
		m.log.EXPECT().Error(fx.AlreadyExists, "Failed to create", "resource", nsname),
	)

	if err := m.ensurable.Ensure(m.log, m.client); err != fx.AlreadyExists {
		t.Errorf("Ensure(): expected error AlreadyExists, got %v", err)
	}
}

// TestEnsureNotFoundCreateSuccess tests the bootstrap green path where we successfully create the resource.
func TestEnsureNotFoundCreateSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mkMocks(ctrl)
	m.ensurable.Definition = m.getterAndCachedObj

	gomock.InOrder(
		m.client.EXPECT().Get(todo, nsname, m.getTypeAndServerObj).Return(fx.NotFound),
		m.log.EXPECT().Info("Creating.", "resource", nsname),
		m.client.EXPECT().Create(todo, m.getterAndCachedObj).Return(nil),
		m.log.EXPECT().Info("Created.", "resource", nsname),
	)

	if err := m.ensurable.Ensure(m.log, m.client); err != nil {
		t.Errorf("Ensure(): expected nil, got %v", err)
	}
}

// TestEnsureGetError tests when the initial GET fails with an unhandled (non-404) error.
func TestEnsureGetError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mkMocks(ctrl)

	gomock.InOrder(
		m.client.EXPECT().Get(todo, nsname, m.getTypeAndServerObj).Return(fx.AlreadyExists),
		m.log.EXPECT().Error(fx.AlreadyExists, "Failed to retrieve.", "resource", nsname),
	)

	if err := m.ensurable.Ensure(m.log, m.client); err != fx.AlreadyExists {
		t.Errorf("Ensure(): expected error AlreadyExists, got %v", err)
	}
}

// TestEnsureExistsNoUpdate tests the green path where the resource already exists
// and doesn't need an update.
func TestEnsureExistsNoUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mkMocks(ctrl)
	m.ensurable.latestVersion = m.getterAndCachedObj
	// Test the equal() path where versions are equal. This causes equal() to return true,
	// triggering "no updated needed".
	// By not defining EqualFunc, we prove that it doesn't get called.
	m.getterAndCachedObj.(metav1.Object).SetResourceVersion("abc")
	m.getTypeAndServerObj.(metav1.Object).SetResourceVersion("abc")
	// Change the server object (unrealistically) so we can prove it's the one that gets cached
	m.getTypeAndServerObj.(*corev1.Pod).Spec.HostNetwork = true

	gomock.InOrder(
		m.client.EXPECT().Get(todo, nsname, m.getTypeAndServerObj).Return(nil),
		m.log.EXPECT().Info("Found. Checking whether update is needed.", "resource", nsname),
		m.log.EXPECT().Info("No update needed."),
	)

	if err := m.ensurable.Ensure(m.log, m.client); err != nil {
		t.Errorf("Ensure(): expected nil, got %v", err)
	}
	// The latestVersion got overwritten, but with the same value
	if diff := cmp.Diff(m.ensurable.latestVersion, m.getTypeAndServerObj); diff != "" {
		t.Fatalf("Bogus latestVersion:\n%s", diff)
	}
}

// TestEnsureExistsUpdateError tests the path where the resource exists, needs an update,
// and the update fails.
func TestEnsureExistsUpdateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mkMocks(ctrl)
	m.ensurable.latestVersion = m.getterAndCachedObj
	// Test the equal() path where the server object isn't labeled. This causes equal() to return false,
	// triggering "needs an update".
	// By not defining EqualFunc, we prove that it doesn't get called.
	MakeMeCare(m.getterAndCachedObj)

	gomock.InOrder(
		m.client.EXPECT().Get(todo, nsname, m.getTypeAndServerObj).Return(nil),
		m.log.EXPECT().Info("Found. Checking whether update is needed.", "resource", nsname),
		m.log.EXPECT().Info("Update needed. Updating..."),
		m.log.EXPECT().V(2).Return(m.log),
		// Don't bother to check the debug message
		m.log.EXPECT().Info(gomock.Any()),
		m.client.EXPECT().Update(todo, m.getterAndCachedObj).Return(fx.NotFound),
		m.log.EXPECT().Error(fx.NotFound, "Failed to update.", "resource", nsname),
	)

	if err := m.ensurable.Ensure(m.log, m.client); err != fx.NotFound {
		t.Errorf("Ensure(): expected error NotFound, got %v", err)
	}
	// The latestVersion didn't get reset
	if m.ensurable.latestVersion != m.getterAndCachedObj {
		t.Fatalf("Bogus latestVersion.\nExpected: %v\nGot:     %v", m.getterAndCachedObj, m.ensurable.latestVersion)
	}
}

// TestEnsureExistsUpdateSuccess tests the green path where the resource exists, needs an
// update, and is updated successfully.
func TestEnsureExistsUpdateSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mkMocks(ctrl)
	m.ensurable.latestVersion = m.getterAndCachedObj
	// Test the actual EqualFunc path.
	// First we need to make sure the server obj is labeled so we get that far
	MakeMeCare(m.getTypeAndServerObj)
	// Poor man's call checker:
	equalFuncCalled := false
	m.ensurable.EqualFunc = func(local, server runtime.Object) bool {
		equalFuncCalled = true
		// Trigger "needs an update"
		return false
	}

	gomock.InOrder(
		m.client.EXPECT().Get(todo, nsname, m.getTypeAndServerObj).Return(nil),
		m.log.EXPECT().Info("Found. Checking whether update is needed.", "resource", nsname),
		m.log.EXPECT().Info("Update needed. Updating..."),
		m.log.EXPECT().V(2).Return(m.log),
		// Don't bother to check the debug message
		m.log.EXPECT().Info(gomock.Any()),
		m.client.EXPECT().Update(todo, m.getterAndCachedObj).Return(nil),
		m.log.EXPECT().Info("Updated.", "resource", nsname),
	)

	if err := m.ensurable.Ensure(m.log, m.client); err != nil {
		t.Errorf("Ensure(): expected nil, got %v", err)
	}
	// The latestVersion got overwritten, but with the same value
	if m.ensurable.latestVersion != m.getterAndCachedObj {
		t.Fatalf("Bogus latestVersion.\nExpected: %v\nGot:     %v", m.getterAndCachedObj, m.ensurable.latestVersion)
	}
	// And make sure equal() got all the way to EqualFunc
	if !equalFuncCalled {
		t.Fatal("EqualFunc wasn't called.")
	}
}

// TestDeleteAlreadyGone tests the green path where the resource was already deleted
func TestDeleteAlreadyGone(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mkMocks(ctrl)

	m.client.EXPECT().Get(todo, nsname, m.getTypeAndServerObj).Return(fx.NotFound)

	if err := m.ensurable.Delete(m.log, m.client); err != nil {
		t.Errorf("Delete(): expected nil, got %v", err)
	}
}

// TestDeleteGetError tests the error path where the initial retrieval fails
func TestDeleteGetError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mkMocks(ctrl)

	gomock.InOrder(
		m.client.EXPECT().Get(todo, nsname, m.getTypeAndServerObj).Return(fx.AlreadyExists),
		m.log.EXPECT().Error(fx.AlreadyExists, "Failed to retrieve.", "resource", nsname),
	)

	if err := m.ensurable.Delete(m.log, m.client); err != fx.AlreadyExists {
		t.Errorf("Delete(): expected error %v; got %v", fx.AlreadyExists, err)
	}
}

// TestDeleteOutOfBand tests the green (well, chartreuse, I guess) path where the object exists
// when we initially retrieve it, but then gets deleted between then and when we attempt to delete.
func TestDeleteOutOfBand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mkMocks(ctrl)

	gomock.InOrder(
		m.client.EXPECT().Get(todo, nsname, m.getTypeAndServerObj).Return(nil),
		m.log.EXPECT().Info("Deleting.", "resource", nsname),
		m.client.EXPECT().Delete(todo, m.getTypeAndServerObj).Return(fx.NotFound),
	)

	if err := m.ensurable.Delete(m.log, m.client); err != nil {
		t.Errorf("Delete(): expected nil, got %v", err)
	}
}

// TestDeleteDeleteError tests the error path where our Delete call fails.
func TestDeleteDeleteError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mkMocks(ctrl)

	gomock.InOrder(
		m.client.EXPECT().Get(todo, nsname, m.getTypeAndServerObj).Return(nil),
		m.log.EXPECT().Info("Deleting.", "resource", nsname),
		m.client.EXPECT().Delete(todo, m.getTypeAndServerObj).Return(fx.AlreadyExists),
		m.log.EXPECT().Error(fx.AlreadyExists, "Failed to delete.", "resource", nsname),
	)

	if err := m.ensurable.Delete(m.log, m.client); err != fx.AlreadyExists {
		t.Errorf("Delete(): expected error %v; got %v", fx.AlreadyExists, err)
	}
}

// TestDeleteDeletes tests the green path where the resource is found and deleted successfully
func TestDeleteDeletes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mkMocks(ctrl)

	gomock.InOrder(
		m.client.EXPECT().Get(todo, nsname, m.getTypeAndServerObj).Return(nil),
		m.log.EXPECT().Info("Deleting.", "resource", nsname),
		m.client.EXPECT().Delete(todo, m.getTypeAndServerObj).Return(nil),
	)

	if err := m.ensurable.Delete(m.log, m.client); err != nil {
		t.Errorf("Delete(): expected nil, got %v", err)
	}
}

// TestGetType proves that GetType() returns a new object rather than reusing the one the
// Ensurable is initialized with.
func TestGetType(t *testing.T) {
	pp := &corev1.Pod{}
	s := EnsurableImpl{
		ObjType: pp,
	}
	if s.GetType() == pp {
		t.Fatal("Expected GetType to return a new copy of its object.")
	}
}

func TestVersionsEqual(t *testing.T) {
	noVersion := &corev1.Pod{}
	blankVersion := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "",
		},
	}
	version1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "123",
		},
	}
	version2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "456",
		},
	}

	type args struct {
		local  runtime.Object
		server runtime.Object
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"Equal", args{local: version1, server: version1}, true},
		{"Versions differ", args{local: version1, server: version2}, false},
		{"Server unset", args{local: version1, server: noVersion}, false},
		{"Local unset", args{local: noVersion, server: version1}, false},
		{"Both unset", args{local: noVersion, server: noVersion}, false},
		{"Server blank", args{local: version1, server: blankVersion}, false},
		{"Local blank", args{local: blankVersion, server: version1}, false},
		// For this one, the versions are technically equal, but since we treat the server side
		// being blank as meaning it doesn't track versions for this object, we still "fail"
		// VersionsEqual (meaning we need to look deeper to determine object "equality").
		{"Both blank", args{local: blankVersion, server: blankVersion}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := VersionsEqual(tt.args.local, tt.args.server); got != tt.want {
				t.Errorf("VersionsEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}
