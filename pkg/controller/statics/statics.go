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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	storagev1beta1 "k8s.io/api/storage/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// These names get assigned by calls to mkEnsurable()

// CSIDriverName is used by `PersistentVolume`s
var CSIDriverName string

// StorageClassName is used by `PersistentVolume`s
var StorageClassName string

var daemonSetName string
var namespaceName string
var sccName string
var serviceAccountName string

// staticResources lists the resources the operator will create, and watch via the statics-controller.
// The order is significant: when bootstrapping, the operator will create the resources in this order.
var staticResources = []util.Ensurable{
	mkEnsurable(
		&corev1.Namespace{},
		"namespace.yaml",
		&namespaceName,
		alwaysEqual,
	),
	mkEnsurable(
		&corev1.ServiceAccount{},
		"serviceaccount.yaml",
		&serviceAccountName,
		alwaysEqual,
	),
	mkEnsurable(
		&securityv1.SecurityContextConstraints{},
		"scc.yaml",
		&sccName,
		// SCC has no Spec; the meat is at the top level
		equalOtherThanMeta,
	),
	mkEnsurable(
		&appsv1.DaemonSet{},
		"daemonset.yaml",
		&daemonSetName,
		daemonSetEqual,
	),
	mkEnsurable(
		&storagev1beta1.CSIDriver{},
		"csidriver.yaml",
		&CSIDriverName,
		csiDriverEqual,
	),
	mkEnsurable(
		&storagev1.StorageClass{},
		"storageclass.yaml",
		&StorageClassName,
		// StorageClass has no Spec; the meat is at the top level
		equalOtherThanMeta,
	),
}

// staticResourceMap is keyed by each Ensurable's resource's name. It's used for quick lookups in
// the reconciler.
// (It's a bit brittle that this is keyed by name; it would break if we needed two resources of the
// same name in different namespaces. But we really shouldn't do that.)
var staticResourceMap = make(map[string]util.Ensurable)

// mkEnsurable is a helper that bootstraps the Ensurable(Impl) instances in staticResources and
// staticResourceMap by loading their definitions from the bindata in defs/*.yaml. At the same
// time, it populates the *Name strings, which are the keys into staticResourceMap. Some of those
// names are also exported because they're used in the `PersistentVolume`s we create.
func mkEnsurable(
	objType runtime.Object,
	defFile string,
	name *string,
	equalFunc func(local, server runtime.Object) bool) util.Ensurable {

	// Make a new copy for the definition
	definition := objType.DeepCopyObject()
	if err := yaml.Unmarshal(MustAsset(filepath.Join("defs", defFile)), definition); err != nil {
		panic("Couldn't load " + defFile + ": " + err.Error())
	}

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
