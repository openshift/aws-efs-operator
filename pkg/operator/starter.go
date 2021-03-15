package operator

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/openshift/library-go/pkg/operator/loglevel"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/staticresourcecontroller"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	opv1 "github.com/openshift/api/operator/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	configinformers "github.com/openshift/client-go/config/informers/externalversions"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/library-go/pkg/controller/manager"
	"github.com/openshift/library-go/pkg/operator/csi/csidrivernodeservicecontroller"
	goc "github.com/openshift/library-go/pkg/operator/genericoperatorclient"
	"github.com/openshift/library-go/pkg/operator/v1helpers"

	"github.com/openshift/aws-efs-csi-driver-operator/pkg/generated"
)

const (
	// Operand and operator run in the same namespace
	operatorName = "aws-efs-csi-driver-operator"
	instanceName = "efs.csi.aws.com"

	namespaceReplaceKey = "${NAMESPACE}"
)

func RunOperator(ctx context.Context, controllerConfig *controllercmd.ControllerContext) error {
	operatorNamespace := controllerConfig.OperatorNamespace

	// Create core clientset and informer
	kubeClient := kubeclient.NewForConfigOrDie(rest.AddUserAgent(controllerConfig.KubeConfig, operatorName))
	kubeInformersForNamespaces := v1helpers.NewKubeInformersForNamespaces(kubeClient, operatorNamespace, "")

	// Create config clientset and informer. This is used to get the cluster ID
	configClient := configclient.NewForConfigOrDie(rest.AddUserAgent(controllerConfig.KubeConfig, operatorName))
	configInformers := configinformers.NewSharedInformerFactory(configClient, 20*time.Minute)

	// Create GenericOperatorclient. This is used by the library-go controllers created down below
	gvr := opv1.SchemeGroupVersion.WithResource("clustercsidrivers")
	operatorClient, dynamicInformers, err := goc.NewClusterScopedOperatorClientWithConfigName(controllerConfig.KubeConfig, gvr, instanceName, goc.WithOptionalInstance())
	if err != nil {
		return err
	}

	cm := manager.NewControllerManager()

	cm = cm.WithController(
		loglevel.NewClusterOperatorLoggingController(operatorClient, controllerConfig.EventRecorder),
		1)

	// TODO: replace with manual create/update/delete
	cm = cm.WithController(
		staticresourcecontroller.NewStaticResourceController(
			"AWSEFSDriverStaticResourcesController",
			mustReplaceAsset(operatorNamespace),
			[]string{
				"csidriver.yaml",
				"node_sa.yaml",
				"rbac/node_privileged_binding.yaml",
				"rbac/privileged_role.yaml",
			},
			resourceapply.NewClientHolder().WithKubernetes(kubeClient),
			operatorClient,
			controllerConfig.EventRecorder,
		).AddKubeInformers(kubeInformersForNamespaces),
		1)

	cm = cm.WithController(
		csidrivernodeservicecontroller.NewCSIDriverNodeServiceController(
			"AWSEFSDriverNodeServiceController",
			mustReplaceNamespace(operatorNamespace, "node.yaml"),
			operatorClient,
			kubeClient,
			kubeInformersForNamespaces.InformersFor(operatorNamespace).Apps().V1().DaemonSets(),
			controllerConfig.EventRecorder,
		),
		1)

	// TODO: add SharedVolume controller

	klog.Info("Starting the informers")
	go kubeInformersForNamespaces.Start(ctx.Done())
	go dynamicInformers.Start(ctx.Done())
	go configInformers.Start(ctx.Done())

	klog.Info("Starting controllerset")
	go cm.Start(ctx)

	<-ctx.Done()

	return fmt.Errorf("stopped")
}

func mustReplaceNamespace(namespace, file string) []byte {
	content := generated.MustAsset(file)
	return bytes.Replace(content, []byte(namespaceReplaceKey), []byte(namespace), -1)
}

func mustReplaceAsset(namespace string) resourceapply.AssetFunc {
	return func(file string) ([]byte, error) {
		content, err := generated.Asset(file)
		if err != nil {
			return nil, err
		}
		return bytes.Replace(content, []byte(namespaceReplaceKey), []byte(namespace), -1), nil
	}
}
