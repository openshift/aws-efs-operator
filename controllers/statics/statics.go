package statics

//go:generate go-bindata -nocompress -nometadata -pkg statics -o zz_generated_defs.go defs/

/**
 * `EnsurableImpl`s for resources that are one-per-cluster (even if namespace-scoped) and should never change.
 */

import (
	"fmt"
	"openshift/aws-efs-operator/pkg/util"
	"path/filepath"
	"reflect"

	"github.com/go-logr/logr"
	securityv1 "github.com/openshift/api/security/v1"
	"openshift/aws-efs-operator/pkg/k8sutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
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

	saDef := &corev1.ServiceAccount{}
	loadDefTemplate(saDef, "serviceaccount.yaml")
	// ServiceAccount is namespaced
	saDef.SetNamespace(namespaceName)
	serviceAccountName = saDef.Name

	sccDef := &securityv1.SecurityContextConstraints{}
	loadDefTemplate(sccDef, "scc.yaml")
	// Add the service account user
	saUser := fmt.Sprintf("system:serviceaccount:%s:%s", saDef.Namespace, saDef.Name)
	sccDef.Users = append(sccDef.Users, saUser)
	sccName = sccDef.Name

	dsDef := &appsv1.DaemonSet{}
	loadDefTemplate(dsDef, "daemonset.yaml")
	// DaemonSet is namespaced
	dsDef.SetNamespace(namespaceName)
	daemonSetName = dsDef.Name

	csiDef := &storagev1.CSIDriver{}
	loadDefTemplate(csiDef, "csidriver.yaml")
	CSIDriverName = csiDef.Name

	scDef := &storagev1.StorageClass{}
	loadDefTemplate(scDef, "storageclass.yaml")
	StorageClassName = scDef.Name

	// NOTE(efried): We can't SetOwner() yet because we don't have the CRD at this stage.
	staticResources = []util.Ensurable{
		&util.EnsurableImpl{
			ObjType:        &corev1.ServiceAccount{},
			NamespacedName: getNSName(saDef),
			Definition:     saDef,
			EqualFunc:      util.AlwaysEqual,
		},
		&util.EnsurableImpl{
			ObjType:        &securityv1.SecurityContextConstraints{},
			NamespacedName: getNSName(sccDef),
			Definition:     sccDef,
			// SCC has no Spec; the meat is at the top level
			EqualFunc: util.EqualOtherThanMeta,
		},
		&util.EnsurableImpl{
			ObjType:        &appsv1.DaemonSet{},
			NamespacedName: getNSName(dsDef),
			Definition:     dsDef,
			EqualFunc:      daemonSetEqual,
		},
		&util.EnsurableImpl{
			ObjType:        &storagev1.CSIDriver{},
			NamespacedName: getNSName(csiDef),
			Definition:     csiDef,
			EqualFunc:      csiDriverEqual,
		},
		&util.EnsurableImpl{
			ObjType:        &storagev1.StorageClass{},
			NamespacedName: getNSName(scDef),
			Definition:     scDef,
			// StorageClass has no Spec; the meat is at the top level
			EqualFunc: util.EqualOtherThanMeta,
		},
	}

	// Populate our lookup map
	for _, s := range staticResources {
		staticResourceMap[s.GetNamespacedName().Name] = s
	}
}

func loadDefTemplate(receiver crclient.Object, defFile string) {
	if err := yaml.Unmarshal(MustAsset(filepath.Join("defs", defFile)), receiver); err != nil {
		panic(fmt.Sprintf("Couldn't load %s: %s", defFile, err.Error()))
	}
}

func getNSName(definition crclient.Object) types.NamespacedName {
	/*remove err , new version returns ObjectKey{Namespace: obj.GetNamespace(), Name: obj.GetName()}*/
	nsname := crclient.ObjectKeyFromObject(definition)
	
	if len(nsname.Name) == 0 {
		panic(fmt.Sprintf("Couldn't extract NamespacedName from definition: %s", "Namespace not extracted"))
	}
	return nsname
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
		return
	}

	glog.Info("Not running in a cluster; discovering namespace from config")
	// TODO(efried): Is there a better / more accepted / canonical way to do this?
	apiConfig, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		panic(fmt.Sprintf("Couldn't discover cluster config: %s", err.Error()))
	}
	clientConfig := clientcmd.NewDefaultClientConfig(*apiConfig, &clientcmd.ConfigOverrides{})
	namespaceName, _, err = clientConfig.Namespace()
	if err != nil {
		panic(fmt.Sprintf("Couldn't get namespace from cluster config: %s", err.Error()))
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
		return fmt.Errorf("encountered %d error(s) ensuring statics", errcount)
	}
	return nil
}

func csiDriverEqual(local, server crclient.Object) bool {
	return reflect.DeepEqual(
		local.(*storagev1.CSIDriver).Spec,
		server.(*storagev1.CSIDriver).Spec,
	)
}

func daemonSetEqual(local, server crclient.Object) bool {
	// TODO: k8s updates fields in the Spec :(
	return reflect.DeepEqual(
		local.(*appsv1.DaemonSet).Spec,
		server.(*appsv1.DaemonSet).Spec)
}
