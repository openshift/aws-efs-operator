package statics

/**
 * Ensurable impls for resources that are one-per-cluster (even if namespace-scoped) and should never change.
 * These resources are instances of a concrete implementation of Ensurable.
 */

import (
	util "2uasimojo/efs-csi-operator/pkg/util"
	"fmt"
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
	"k8s.io/apimachinery/pkg/util/intstr"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// CSIDriverName is used by `PersistentVolume`s
	CSIDriverName      = "efs.csi.aws.com"
	daemonSetName      = "efs-csi-node"
	namespaceName      = "openshift-efs-csi"
	sccName            = "efs-csi-scc"
	serviceAccountName = "efs-csi-sa"
	// StorageClassName is used by `PersistentVolume`s
	StorageClassName = "efs-sc"
)

const (
	// TODO(efried): Pin this to a release.
	//   See https://github.com/kubernetes-sigs/aws-efs-csi-driver/issues/152
	driverImage        = "registry.hub.docker.com/amazon/aws-efs-csi-driver:latest"
	registrarImage     = "quay.io/k8scsi/csi-node-driver-registrar:v1.1.0"
	livenessProbeImage = "quay.io/k8scsi/livenessprobe:v1.1.0"
)

// statics lists the resources the `statics-controller` will create and watch.
// The order is significant: when bootstrapping, the controller will create the resources
// in this order.
var staticResources []util.Ensurable = []util.Ensurable{
	// Namespace
	util.EnsurableImpl{
		ObjType:        &corev1.Namespace{},
		NamespacedName: types.NamespacedName{Name: namespaceName},
		Definition:     getNamespace(),
		EqualFunc:      alwaysEqual,
	},
	// ServiceAccount
	util.EnsurableImpl{
		ObjType:        &corev1.ServiceAccount{},
		NamespacedName: types.NamespacedName{Name: serviceAccountName, Namespace: namespaceName},
		Definition:     getServiceAccount(),
		EqualFunc:      alwaysEqual,
	},
	// SecurityContextConstraints
	util.EnsurableImpl{
		ObjType:        &securityv1.SecurityContextConstraints{},
		NamespacedName: types.NamespacedName{Name: sccName},
		Definition:     getSecurityContextConstraints(),
		// SCC has no Spec; the meat is at the top level
		EqualFunc:      equalOtherThanMeta,
	},
	// DaemonSet
	util.EnsurableImpl{
		ObjType:        &appsv1.DaemonSet{},
		NamespacedName: types.NamespacedName{Name: daemonSetName, Namespace: namespaceName},
		Definition:     getDaemonSet(),
		EqualFunc:      daemonSetEqual,
	},
	// CSIDriver
	util.EnsurableImpl{
		ObjType:        &storagev1beta1.CSIDriver{},
		NamespacedName: types.NamespacedName{Name: CSIDriverName},
		Definition:     getCSIDriver(),
		EqualFunc:      csiDriverEqual,
	},
	// StorageClass
	util.EnsurableImpl{
		ObjType:        &storagev1.StorageClass{},
		NamespacedName: types.NamespacedName{Name: StorageClassName},
		Definition:     getStorageClass(),
		// StorageClass has no Spec; the meat is at the top level
		EqualFunc:      equalOtherThanMeta,
	},
}

// For quick lookups in the reconciler
var staticResourceMap map[types.NamespacedName]util.Ensurable = map[types.NamespacedName]util.Ensurable{}

func init() {
	for _, s := range staticResources {
		staticResourceMap[s.GetNamespacedName()] = s
	}
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

// getNamespace in which the DaemonSet will run.
func getNamespace() runtime.Object {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceName,
		},
	}
}

// getServiceAccount tying the DaemonSet to its SecurityContextConstraints and Namespace.
func getServiceAccount() runtime.Object {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: namespaceName,
		},
	}
}

// getStorageClass for the CSIDriver.
func getStorageClass() runtime.Object {
	delete := corev1.PersistentVolumeReclaimDelete
	immediate := storagev1.VolumeBindingImmediate
	return &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: StorageClassName,
		},
		Provisioner:       CSIDriverName,
		ReclaimPolicy:     &delete,
		VolumeBindingMode: &immediate,
	}
}

// getCSIDriver resource itself.
func getCSIDriver() runtime.Object {
	falseVal := false
	return &storagev1beta1.CSIDriver{
		ObjectMeta: metav1.ObjectMeta{
			Name: CSIDriverName,
		},
		Spec: storagev1beta1.CSIDriverSpec{
			AttachRequired: &falseVal,
			PodInfoOnMount: &falseVal,
			VolumeLifecycleModes: []storagev1beta1.VolumeLifecycleMode{
				storagev1beta1.VolumeLifecyclePersistent,
			},
		},
	}
}
func csiDriverEqual(local, server runtime.Object) bool {
	return reflect.DeepEqual(
		local.(*storagev1beta1.CSIDriver).Spec,
		server.(*storagev1beta1.CSIDriver).Spec,
	)
}

// getSecurityContextConstraints giving the DaemonSet the necessary privileges.
func getSecurityContextConstraints() runtime.Object {
	trueVal := true
	return &securityv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name: sccName,
			Annotations: map[string]string{
				"kubernetes.io/description": "Highly privileged SCC for the EFS CSI driver DaemonSet.",
			},
		},
		AllowHostDirVolumePlugin: true,
		AllowHostIPC:             true,
		AllowHostNetwork:         true,
		AllowHostPID:             true,
		AllowHostPorts:           true,
		AllowPrivilegedContainer: true,
		AllowPrivilegeEscalation: &trueVal,
		AllowedCapabilities:      []corev1.Capability{"*"},
		AllowedUnsafeSysctls:     []string{"*"},
		FSGroup: securityv1.FSGroupStrategyOptions{
			Type: securityv1.FSGroupStrategyRunAsAny,
		},
		Groups: []string{
			"system:cluster-admins",
			"system:nodes",
			"system:masters",
		},
		ReadOnlyRootFilesystem: false,
		RunAsUser: securityv1.RunAsUserStrategyOptions{
			Type: securityv1.RunAsUserStrategyRunAsAny,
		},
		SELinuxContext: securityv1.SELinuxContextStrategyOptions{
			Type: securityv1.SELinuxStrategyRunAsAny,
		},
		SeccompProfiles: []string{"*"},
		SupplementalGroups: securityv1.SupplementalGroupsStrategyOptions{
			Type: securityv1.SupplementalGroupsStrategyRunAsAny,
		},
		Users: []string{
			"system:admin",
			fmt.Sprintf("system:serviceaccount:%s:%s", namespaceName, serviceAccountName),
		},
		Volumes: []securityv1.FSType{"*"},
	}
}

// getDaemonSet that runs the driver image and provisions the volume mounts.
func getDaemonSet() runtime.Object {
	labels := map[string]string{"app": daemonSetName}
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      daemonSetName,
			Namespace: namespaceName,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: serviceAccountName,
					NodeSelector: map[string]string{
						"node-role.kubernetes.io/worker": "",
					},
					HostNetwork: true,
					Tolerations: []corev1.Toleration{
						{
							Operator: "Exists",
						},
					},
					Containers: []corev1.Container{
						getPluginContainer(),
						getRegistrarContainer(),
						getLivenessProbeContainer(),
					},
					Volumes: []corev1.Volume{
						getVolume("kubelet-dir", "/var/lib/kubelet", "Directory"),
						getVolume("registration-dir", "/var/lib/kubelet/plugins_registry/", "Directory"),
						getVolume("plugin-dir", fmt.Sprintf("/var/lib/kubelet/plugins/%s/", CSIDriverName), "DirectoryOrCreate"),
						getVolume("efs-state-dir", "/var/run/efs", "DirectoryOrCreate"),
					},
				},
			},
		},
	}
}
func daemonSetEqual(local, server runtime.Object) bool {
	// TODO: k8s updates fields in the Spec :(
	return reflect.DeepEqual(
		local.(*appsv1.DaemonSet).Spec,
		server.(*appsv1.DaemonSet).Spec)
}

func getVolume(name string, path string, hpType string) corev1.Volume {
	hpt := corev1.HostPathType(hpType)
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: path,
				Type: &hpt,
			},
		},
	}
}

func getPluginContainer() corev1.Container {
	trueVal := true
	bidirectional := corev1.MountPropagationBidirectional
	return corev1.Container{
		Name: "efs-plugin",
		SecurityContext: &corev1.SecurityContext{
			Privileged: &trueVal,
		},
		Image:           driverImage,
		ImagePullPolicy: corev1.PullAlways,
		Args: []string{
			"--endpoint=$(CSI_ENDPOINT)",
			"--logtostderr",
			"--v=5",
		},
		Env: []corev1.EnvVar{
			{
				Name:  "CSI_ENDPOINT",
				Value: "unix:/csi/csi.sock",
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:             "kubelet-dir",
				MountPath:        "/var/lib/kubelet",
				MountPropagation: &bidirectional,
			},
			{
				Name:      "plugin-dir",
				MountPath: "/csi",
			},
			{
				Name:      "efs-state-dir",
				MountPath: "/var/run/efs",
			},
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "healthz",
				ContainerPort: 9809,
				HostPort:      9809,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		LivenessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromString("healthz"),
				},
			},
			InitialDelaySeconds: 10,
			TimeoutSeconds:      3,
			PeriodSeconds:       2,
			FailureThreshold:    5,
		},
	}
}

func getRegistrarContainer() corev1.Container {
	return corev1.Container{
		Name:            "csi-driver-registrar",
		Image:           registrarImage,
		ImagePullPolicy: corev1.PullAlways,
		Args: []string{
			"--csi-address=$(ADDRESS)",
			"--kubelet-registration-path=$(DRIVER_REG_SOCK_PATH)",
			"--v=5",
		},
		Env: []corev1.EnvVar{
			{
				Name:  "ADDRESS",
				Value: "/csi/csi.sock",
			},
			{
				Name:  "DRIVER_REG_SOCK_PATH",
				Value: fmt.Sprintf("/var/lib/kubelet/plugins/%s/csi.sock", CSIDriverName),
			},
			{
				Name: "KUBE_NODE_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "spec.nodeName",
					},
				},
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "plugin-dir",
				MountPath: "/csi",
			},
			{
				Name:      "registration-dir",
				MountPath: "/registration",
			},
		},
	}
}

func getLivenessProbeContainer() corev1.Container {
	return corev1.Container{
		Name:            "liveness-probe",
		Image:           livenessProbeImage,
		ImagePullPolicy: corev1.PullAlways,
		Args: []string{
			"--csi-address=/csi/csi.sock",
			"--health-port=9809",
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "plugin-dir",
				MountPath: "/csi",
			},
		},
	}
}
