package statics

import (
	"2uasimojo/efs-csi-operator/pkg/util"
	"2uasimojo/efs-csi-operator/pkg/fixtures"
	"context"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/go-logr/logr"
	securityv1 "github.com/openshift/api/security/v1"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var nullResult = reconcile.Result{}
var requeueResult = reconcile.Result{Requeue: true}

func setup() (logr.Logger, *ReconcileStatics) {
	logf.SetLogger(logf.ZapLogger(true))

	client := fake.NewFakeClientWithScheme(scheme.Scheme)

	return logf.Log.Logger, &ReconcileStatics{client: client, scheme: scheme.Scheme}
}

func TestReconcile(t *testing.T) {
	checkNumStatics(t)

	// OpenShift types need to be registered explicitly
	scheme.Scheme.AddKnownTypes(securityv1.SchemeGroupVersion, &securityv1.SecurityContextConstraints{})

	ctx := context.TODO()

	logger, r := setup()

	// This may be overkill, but verify that none of our statics exist
	for _, staticResource := range staticResources {
		obj := staticResource.GetType()
		nsname := staticResource.GetNamespacedName()
		if err := r.client.Get(ctx, nsname, obj); !errors.IsNotFound(err) {
			t.Fatalf("Didn't expect to find resounce %v but got:\n%v", nsname, obj)
		}
	}

	// We'll use these later
	var dsStatic util.Ensurable
	var resources map[string]runtime.Object

	// Now let's run the reconciler for each of our tracked resources.
	for _, staticResource := range staticResources {
		nsname := staticResource.GetNamespacedName()
		// Grab this for later
		if nsname.Name == daemonSetName {
			dsStatic = staticResource
		}
		logger.Info("Bootstrap: reconciling", "resource", nsname)
		res, err := r.Reconcile(reconcile.Request{NamespacedName: nsname})
		if err != nil {
			t.Fatalf("Didn't expect an error, but got %v", err)
		}
		if !reflect.DeepEqual(res, nullResult) {
			t.Fatalf("Unexpected result.\nExpected: %v\nGot:     %v", nullResult, res)
		}
	}

	// Having done that, all our statics ought to be golden.
	resources = checkStatics(t, r.client)

	// Let's twiddle our DaemonSet
	daemonSet := resources["DaemonSet"].(*appsv1.DaemonSet)
	daemonSet.Spec.Template.Spec.Containers = daemonSet.Spec.Template.Spec.Containers[:1]
	if err := r.client.Update(ctx, daemonSet); err != nil {
		t.Fatalf("Failed to update DaemonSet in test: %v", err)
	}

	// Now reconcile it
	res, err := r.Reconcile(reconcile.Request{NamespacedName: dsStatic.GetNamespacedName()})
	if err != nil {
		t.Fatalf("Didn't expect an error, but got %v", err)
	}
	if !reflect.DeepEqual(res, nullResult) {
		t.Fatalf("Unexpected result.\nExpected: %v\nGot:     %v", nullResult, res)
	}

	// And now it should be golden again. Check all the things, to make sure we didn't do something bad to them.
	resources = checkStatics(t, r.client)
}

// TestReconcileUnexpected tests the code path where a resource we don't care about somehow makes it past the filter
func TestReconcileUnexpected(t *testing.T) {
	_, r := setup()
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "foo", Name: "bar"}}
	res, err := r.Reconcile(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if !reflect.DeepEqual(res, nullResult) {
		t.Fatalf("Unexpected result.\nExpected: %v\nGot:     %v", nullResult, res)
	}
}

// TestReconcileEnsureFails tests the path where an Ensure fails during a Reconcile
func TestReconcileEnsureFails(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := fixtures.NewMockClient(ctrl)

	// Not realistic, we're just contriving a way to make Ensure fail
	theError := fixtures.AlreadyExists

	// Don't really care about the args
	client.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).MaxTimes(1).Return(theError)

	// Any resource is fine, just making sure we actually try to Ensure it
	staticResource := staticResources[3]

	r := ReconcileStatics{client: client, scheme: scheme.Scheme}

	res, err := r.Reconcile(reconcile.Request{NamespacedName: staticResource.GetNamespacedName()})

	if err == nil {
		t.Fatal("Expected an error")
	}
	if !reflect.DeepEqual(res, requeueResult) {
		t.Fatalf("Expected a requeue, got %v", res)
	}
}