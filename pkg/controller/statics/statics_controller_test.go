package statics

import (
	"context"
	"openshift/aws-efs-operator/pkg/fixtures"
	"openshift/aws-efs-operator/pkg/test"
	"openshift/aws-efs-operator/pkg/util"
	"reflect"
	"testing"
	"time"

	"github.com/go-logr/logr"
	securityv1 "github.com/openshift/api/security/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	// TODO: pkg/client/fake is deprecated, replace with pkg/envtest
	"sigs.k8s.io/controller-runtime/pkg/client/fake" //nolint:staticcheck
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// TODO: Test add()/watches somehow?

func setup() (logr.Logger, *ReconcileStatics) {

	// OpenShift types need to be registered explicitly
	scheme.Scheme.AddKnownTypes(securityv1.SchemeGroupVersion, &securityv1.SecurityContextConstraints{})
	// And so do extensions
	scheme.Scheme.AddKnownTypes(apiextensions.SchemeGroupVersion, &apiextensions.CustomResourceDefinition{})

	client := fake.NewFakeClientWithScheme(scheme.Scheme)

	err := client.Create(context.TODO(), &apiextensions.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: svCRDName,
		},
	})
	if err != nil {
		panic(err)
	}

	return logf.Log.Logger, &ReconcileStatics{client: client, scheme: scheme.Scheme}
}

// TestStartup simulates operator startup by creating the statics before the CRD is discovered,
// then reconciling and proving that the statics are updated with the proper OwnerReferences.
func TestStartup(t *testing.T) {
	ctx := context.TODO()

	logger, r := setup()

	// This is how statics are bootstrapped on operator startup
	if err := EnsureStatics(logger, r.client); err != nil {
		t.Fatal(err)
	}

	for _, staticResource := range staticResources {
		obj := staticResource.GetType()
		nsname := staticResource.GetNamespacedName()
		// The static was created
		if err := r.client.Get(ctx, nsname, obj); err != nil {
			t.Fatal(err)
		}
		// But shouldn't have OwnerReferences set
		if orefs := obj.(metav1.Object).GetOwnerReferences(); len(orefs) != 0 {
			t.Fatalf("Expected no OwnerReferences but got %v", orefs)
		}
		// Until we reconcile
		res, err := r.Reconcile(reconcile.Request{NamespacedName: nsname})
		if err != nil {
			t.Fatalf("Didn't expect an error, but got %v", err)
		}
		if !reflect.DeepEqual(res, test.NullResult) {
			t.Fatalf("Unexpected result.\nExpected: %v\nGot:     %v", test.NullResult, res)
		}
		// And then there should be one
		if err := r.client.Get(ctx, nsname, obj); err != nil {
			t.Fatal(err)
		}
		orefs := obj.(metav1.Object).GetOwnerReferences()
		if len(orefs) != 1 {
			t.Fatalf("Expected one OwnerReference but got %v", orefs)
		}
		// Sanity check the OwnerReference
		if orefs[0].Name != svCRDName {
			t.Fatalf("Expected the OwnerReference to have Name=%q but got\n%v", svCRDName, orefs[0])
		}
	}

}

func TestReconcile(t *testing.T) {
	checkNumStatics(t)

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
	var (
		dsStatic  util.Ensurable
		resources map[string]runtime.Object
	)

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
		if !reflect.DeepEqual(res, test.NullResult) {
			t.Fatalf("Unexpected result.\nExpected: %v\nGot:     %v", test.NullResult, res)
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
	if !reflect.DeepEqual(res, test.NullResult) {
		t.Fatalf("Unexpected result.\nExpected: %v\nGot:     %v", test.NullResult, res)
	}

	// And now it should be golden again. Check all the things, to make sure we didn't do something bad to them.
	checkStatics(t, r.client)
}

// TestReconcileCRDVariants tests the code paths where the CRD is in other-than-green states.
func TestReconcileCRDVariants(t *testing.T) {
	_, r := setup()

	var (
		crd *apiextensions.CustomResourceDefinition
		err error
		ctx = context.TODO()
	)

	// Restores the state where a) our statics exist, b) the CRD is green.
	reset := func() {
		if err = r.client.Delete(ctx, crd); err != nil && !errors.IsNotFound(err) {
			t.Log(err)
		}

		err = r.client.Create(ctx, &apiextensions.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: svCRDName,
			},
		})
		if err != nil {
			t.Log(err)
		}
		for _, staticResource := range staticResources {
			nsname := staticResource.GetNamespacedName()
			if _, err := r.Reconcile(reconcile.Request{NamespacedName: nsname}); err != nil {
				t.Fatal(err)
			}

			obj := staticResource.GetType()
			if err := r.client.Get(ctx, nsname, obj); err != nil {
				t.Fatal(err)
			}

		}
	}

	// Makes sure the statics are in the expected state
	// TODO(efried): The `expectSCC` param exists because of the workaround for
	// https://github.com/openshift/aws-efs-operator/issues/23 and should be deleted when that is
	// resolved.
	check := func(expectSCC bool) {
		for _, staticResource := range staticResources {
			nsname := staticResource.GetNamespacedName()
			obj := staticResource.GetType()
			err = r.client.Get(ctx, nsname, obj)
			// TODO(efried): Delete this conditional when https://github.com/openshift/aws-efs-operator/issues/23 is resolved.
			if !expectSCC && nsname.Name == sccName {
				if !errors.IsNotFound(err) {
					t.Fatalf("Expected SCC to be gone, but err was %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		}
	}

	// Setup
	reset()
	if crd, err = discoverCRD(r.client); err != nil {
		t.Fatal(err)
	}

	// 1) We catch the CRD while it's being deleted
	now := metav1.Now()
	crd.SetDeletionTimestamp(&now)
	if err := r.client.Update(ctx, crd); err != nil {
		t.Fatal(err)
	}

	// Overkill, but prove this behaves the same for any static
	for _, staticResource := range staticResources {
		nsname := staticResource.GetNamespacedName()
		res, err := r.Reconcile(reconcile.Request{NamespacedName: nsname})
		if err != nil {
			t.Fatalf("Expected no error reconciling %v but got %v", nsname, err)
		}
		if !reflect.DeepEqual(res, test.NullResult) {
			t.Fatalf("Unexpected result.\nExpected: %v\nGot:     %v", test.NullResult, res)
		}
	}
	// SCC manually deleted
	check(false)

	reset()

	// 2) The CRD has already been deleted
	if err := r.client.Delete(ctx, crd); err != nil {
		t.Fatal(err)
	}

	// Same again
	for _, staticResource := range staticResources {
		nsname := staticResource.GetNamespacedName()
		res, err := r.Reconcile(reconcile.Request{NamespacedName: nsname})
		if err != nil {
			t.Fatalf("Expected no error reconciling %v but got %v", nsname, err)
		}
		if !reflect.DeepEqual(res, test.NullResult) {
			t.Fatalf("Unexpected result.\nExpected: %v\nGot:     %v", test.NullResult, res)
		}
	}
	// SCC manually deleted
	check(false)

	reset()

	// 3) Error retrieving the CRD
	realclient := r.client
	fcwce := &test.FakeClientWithCustomErrors{
		Client:      realclient,
		GetBehavior: make([]error, len(staticResources)),
	}
	r.client = fcwce
	for i, staticResource := range staticResources {
		// Set our fake to error on this reconcile.
		fcwce.GetBehavior[i] = fixtures.AlreadyExists
		nsname := staticResource.GetNamespacedName()
		res, err := r.Reconcile(reconcile.Request{NamespacedName: nsname})
		if err != nil {
			t.Fatalf("Expected no error reconciling %v but got %v", nsname, err)
		}
		if !res.Requeue || res.RequeueAfter != time.Millisecond*1000 {
			t.Fatalf("Unexpected result. Expected: requeue after 1s; Got: %v", res)
		}
	}
	// The error path doesn't delete the SCC
	check(true)
}

// TestReconcileUnexpected tests the code path where a resource we don't care about somehow makes it past the filter
func TestReconcileUnexpected(t *testing.T) {
	_, r := setup()
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "foo", Name: "bar"}}
	res, err := r.Reconcile(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if !reflect.DeepEqual(res, test.NullResult) {
		t.Fatalf("Unexpected result.\nExpected: %v\nGot:     %v", test.NullResult, res)
	}
}

// TestReconcileEnsureFails tests the path where an Ensure fails during a Reconcile
func TestReconcileEnsureFails(t *testing.T) {
	_, r := setup()

	fcwce := &test.FakeClientWithCustomErrors{
		Client: r.client,
		// The first GET is for the CRD. Make the second (for the ensurable) fail.
		GetBehavior: []error{nil, fixtures.AlreadyExists},
	}

	// Any resource is fine, just making sure we actually try to Ensure it
	staticResource := staticResources[3]

	rs := ReconcileStatics{client: fcwce, scheme: scheme.Scheme}

	res, err := rs.Reconcile(reconcile.Request{NamespacedName: staticResource.GetNamespacedName()})

	if err == nil {
		t.Fatal("Expected an error")
	}
	if !reflect.DeepEqual(res, test.RequeueResult) {
		t.Fatalf("Expected a requeue, got %v", res)
	}
}
