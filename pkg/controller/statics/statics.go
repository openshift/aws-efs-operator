package statics

//go:generate go-bindata -nocompress -pkg statics -o defs.go defs/

/**
 * `EnsurableImpl`s for resources that are one-per-cluster (even if namespace-scoped) and should never change.
 */

import (
	"2uasimojo/efs-csi-operator/pkg/util"
	"fmt"
	"path/filepath"
	"reflect"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	securityv1 "github.com/openshift/api/security/v1"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	storagev1beta1 "k8s.io/api/storage/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

var (
	// These names get assigned by calls to makeEnsurable()

	// CSIDriverName is used by `PersistentVolume`s
	CSIDriverName string

	// StorageClassName is used by `PersistentVolume`s
	StorageClassName string

	daemonSetName      string
	namespaceName      string
	sccName            string
	serviceAccountName string

	// staticResources lists the resources the operator will create, and watch via the statics-controller.
	// The order is significant: when bootstrapping, the operator will create the resources in this order.
	// This is populated by `initStatics()`.
	staticResources []util.Ensurable

	// staticResourceMap is keyed by each Ensurable's resource's name. It's used for quick lookups in
	// the reconciler.
	// This is populated by `initStatics()`.
	// (It's a bit brittle that this is keyed by name; it would break if we needed two resources of the
	// same name in different namespaces. But we really shouldn't do that.)
	staticResourceMap = make(map[string]util.Ensurable)

	// Global logger used for init()
	glog = logf.Log.WithName("statics bootstrap")
)

// Bootstrap the `staticResources` and `staticResourceMap`.
func init() {
	// First discover the namespace we're running in. This is where we'll run the driver.
	// We'll need this value to set up the DaemonSet and its ServiceAccount, whose definition
	// files don't specify a namespace.
	discoverNamespace()

	// Build up our static Ensurables
	staticResources = []util.Ensurable{
		makeEnsurable(
			&corev1.ServiceAccount{},
			"serviceaccount.yaml",
			namespaceName,
			&serviceAccountName,
			alwaysEqual,
		),
		makeEnsurable(
			&securityv1.SecurityContextConstraints{},
			"scc.yaml",
			"", // SCC is cluster scoped
			&sccName,
			// SCC has no Spec; the meat is at the top level
			equalOtherThanMeta,
		),
		makeEnsurable(
			&appsv1.DaemonSet{},
			"daemonset.yaml",
			namespaceName,
			&daemonSetName,
			daemonSetEqual,
		),
		makeEnsurable(
			&storagev1beta1.CSIDriver{},
			"csidriver.yaml",
			"", // CSIDriver is cluster scoped
			&CSIDriverName,
			csiDriverEqual,
		),
		makeEnsurable(
			&storagev1.StorageClass{},
			"storageclass.yaml",
			"", // StorageClass is cluster scoped
			&StorageClassName,
			// StorageClass has no Spec; the meat is at the top level
			equalOtherThanMeta,
		),
	}
}

// makeEnsurable is a helper that bootstraps an Ensurable(Impl) instance in staticResources and
// staticResourceMap by loading its definition from the bindata in defs/*.yaml. At the same
// time, it populates the *Name string, which keys into staticResourceMap.
func makeEnsurable(
	objType runtime.Object,
	defFile string,
	namespace string,
	name *string,
	equalFunc func(local, server runtime.Object) bool) util.Ensurable {

	// Make a new copy for the definition
	definition := objType.DeepCopyObject()
	if err := yaml.Unmarshal(MustAsset(filepath.Join("defs", defFile)), definition); err != nil {
		panic("Couldn't load " + defFile + ": " + err.Error())
	}

	// Our defs don't have a namespace yet
	definition.(metav1.Object).SetNamespace(namespace)

	// Discover the NamespacedName from that definition
	nsname, err := crclient.ObjectKeyFromObject(definition)
	if err != nil {
		panic("Couldn't extract NamespacedName from definition for " + defFile + ": " + err.Error())
	}

	// Set our all-important global name variable
	*name = nsname.Name

	// Build up the object
	ensurable := &util.EnsurableImpl{
		ObjType:        objType,
		NamespacedName: nsname,
		Definition:     definition,
		EqualFunc:      equalFunc,
	}
	// Stuff it in the lookup map
	staticResourceMap[nsname.Name] = ensurable

	return ensurable
}

// discoverNamespace discovers the namespace we're running in and sets the global `namespaceName`
// variable.
// If not running in a cluster, we have to do this via kubeconfig.
// (Note that we don't use WATCH_NAMESPACE, which should always be ''.)
func discoverNamespace() {
	defer func() {
		if r := recover(); r != nil {
			glog.Error(fmt.Errorf("%v", r), "Couldn't detect namespace. Assuming we're running in a test environment.")
			// Use a namespace name that's deliberately bogus in a real cluster, to make sure this
			// still breaks if it slips through in a non-testing environment.
			namespaceName = "__TEST_NAMESPACE__"
		}
	}()

	var err error
	namespaceName, err = k8sutil.GetOperatorNamespace()
	if err == nil {
		glog.Info("Running in a cluster; discovered namespace.", "namespace", namespaceName)
	}

	glog.Info("Not running in a cluster; discovering namespace from config")
	// TODO(efried): Is there a better / more accepted / canonical way to do this?
	apiConfig, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		panic("Couldn't discover cluster config: " + err.Error())
	}
	clientConfig := clientcmd.NewDefaultClientConfig(*apiConfig, &clientcmd.ConfigOverrides{})
	namespaceName, _, err = clientConfig.Namespace()
	if err != nil {
		panic("Couldn't get namespace from cluster config: " + err.Error())
	}
	glog.Info("Discovered namespace from config.", "namespace", namespaceName)
}

// findStatic finds a static resource based on its NamespacedName, returning `nil` if not found.
// This really just exists in case we want to change how staticResourceMap is indexed at some point.
func findStatic(nsname types.NamespacedName) util.Ensurable {
	s, ok := staticResourceMap[nsname.Name]
	if ok {
		return s
	}
	return nil
}

// EnsureStatics creates and/or updates all the staticResources
func EnsureStatics(log logr.Logger, client crclient.Client) error {
	errcount := 0
	for _, s := range staticResources {
		if err := s.Ensure(log, client); err != nil {
			// Ensure already logged, just keep track of how many errors we saw
			errcount++
		}
	}
	if errcount != 0 {
		return fmt.Errorf("Encountered %d error(s) ensuring statics", errcount)
	}
	return nil
}

// alwaysEqual is a convenience implementation of static.equalFunc for objects that can't change
// (in any significant way)
func alwaysEqual(local, server runtime.Object) bool {
	return true
}

// equalOtherThanMeta is a DeepEquals that ignores ObjectMeta and TypeMeta.
// Use when a DeepEqual on Spec won't work, e.g. when the meat of the object is at the top level
// and/or there _is_ no Spec.
func equalOtherThanMeta(local, server runtime.Object) bool {
	return cmp.Equal(local, server, cmpopts.IgnoreTypes(metav1.ObjectMeta{}, metav1.TypeMeta{}))
}

func csiDriverEqual(local, server runtime.Object) bool {
	return reflect.DeepEqual(
		local.(*storagev1beta1.CSIDriver).Spec,
		server.(*storagev1beta1.CSIDriver).Spec,
	)
}

func daemonSetEqual(local, server runtime.Object) bool {
	// TODO: k8s updates fields in the Spec :(
	return reflect.DeepEqual(
		local.(*appsv1.DaemonSet).Spec,
		server.(*appsv1.DaemonSet).Spec)
}
