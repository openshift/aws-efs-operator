package operator

import (
	"context"
	"fmt"
	"time"

	opv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/management"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	operatorv1helpers "github.com/openshift/library-go/pkg/operator/v1helpers"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// CSIStaticResourceController creates, manages and deletes static resources of a CSI driver, such as RBAC rules.
// It's more hardcoded variant of library-go's StaticResourceController, which does not implement removal
// of objects yet.
type CSIStaticResourceController struct {
	operatorName      string
	operatorNamespace string
	operatorClient    operatorv1helpers.OperatorClientWithFinalizers
	kubeClient        kubernetes.Interface
	eventRecorder     events.Recorder

	// Objects to sync
	csiDriver          *storagev1.CSIDriver
	nodeServiceAccount *corev1.ServiceAccount
	nodeRole           *rbacv1.ClusterRole
	nodeRoleBinding    *rbacv1.ClusterRoleBinding
}

func NewCSIStaticResourceController(
	name string,
	operatorNamespace string,
	operatorClient operatorv1helpers.OperatorClientWithFinalizers,
	kubeClient kubernetes.Interface,
	informers operatorv1helpers.KubeInformersForNamespaces,
	recorder events.Recorder,
	csiDriver *storagev1.CSIDriver,
	nodeServiceAccount *corev1.ServiceAccount,
	nodeRole *rbacv1.ClusterRole,
	nodeRoleBinding *rbacv1.ClusterRoleBinding,
) factory.Controller {
	c := &CSIStaticResourceController{
		operatorName:       name,
		operatorNamespace:  operatorNamespace,
		operatorClient:     operatorClient,
		kubeClient:         kubeClient,
		eventRecorder:      recorder,
		csiDriver:          csiDriver,
		nodeServiceAccount: nodeServiceAccount,
		nodeRole:           nodeRole,
		nodeRoleBinding:    nodeRoleBinding,
	}

	operatorInformers := []factory.Informer{
		operatorClient.Informer(),
		informers.InformersFor(operatorNamespace).Core().V1().ServiceAccounts().Informer(),
		informers.InformersFor(operatorNamespace).Storage().V1().CSIDrivers().Informer(),
		informers.InformersFor(operatorNamespace).Rbac().V1().ClusterRoles().Informer(),
		informers.InformersFor(operatorNamespace).Rbac().V1().ClusterRoleBindings().Informer(),
	}
	return factory.New().
		WithSyncDegradedOnError(operatorClient).
		WithInformers(operatorInformers...).
		WithSync(c.sync).
		ResyncEvery(time.Minute).
		ToController(name, recorder.WithComponentSuffix("csi-static-resource-controller"))
}

func (c *CSIStaticResourceController) sync(ctx context.Context, controllerContext factory.SyncContext) error {
	opSpec, opStatus, _, err := c.operatorClient.GetOperatorState()
	if err != nil {
		return err
	}

	action := management.GetSyncAction(opSpec.ManagementState)
	klog.V(4).Infof("Syncing with managementState %q: %s", opSpec.ManagementState, action)
	switch action {
	case management.SyncActionManage:
		return c.syncManaged(ctx, opSpec, opStatus, controllerContext)
	case management.SyncActionDelete:
		return c.syncRemoved(ctx, opSpec, opStatus, controllerContext)
	case management.SyncActionIgnore:
		return nil
	default:
		return fmt.Errorf("unrecognized managementState: %q: %s", opSpec.ManagementState, action)
	}
}

func (c *CSIStaticResourceController) syncManaged(ctx context.Context, opSpec *opv1.OperatorSpec, opStatus *opv1.OperatorStatus, controllerContext factory.SyncContext) error {
	err := c.operatorClient.EnsureFinalizer(c.operatorName)
	if err != nil {
		return err
	}

	_, _, err = resourceapply.ApplyCSIDriver(c.kubeClient.StorageV1(), c.eventRecorder, c.csiDriver)
	if err != nil {
		return err
	}
	_, _, err = resourceapply.ApplyClusterRole(c.kubeClient.RbacV1(), c.eventRecorder, c.nodeRole)
	if err != nil {
		return err
	}
	_, _, err = resourceapply.ApplyClusterRoleBinding(c.kubeClient.RbacV1(), c.eventRecorder, c.nodeRoleBinding)
	if err != nil {
		return err
	}
	_, _, err = resourceapply.ApplyServiceAccount(c.kubeClient.CoreV1(), c.eventRecorder, c.nodeServiceAccount)
	if err != nil {
		return err
	}

	return nil
}

func (c *CSIStaticResourceController) syncRemoved(ctx context.Context, opSpec *opv1.OperatorSpec, opStatus *opv1.OperatorStatus, controllerContext factory.SyncContext) error {
	err := c.kubeClient.StorageV1().CSIDrivers().Delete(ctx, c.csiDriver.Name, metav1.DeleteOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		klog.V(4).Infof("CSIDriver %s already removed", c.csiDriver.Name)
	}

	err = c.kubeClient.RbacV1().ClusterRoles().Delete(ctx, c.nodeRole.Name, metav1.DeleteOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		klog.V(4).Infof("ClusterRole %s already removed", c.nodeRole.Name)
	}

	err = c.kubeClient.RbacV1().ClusterRoleBindings().Delete(ctx, c.nodeRoleBinding.Name, metav1.DeleteOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		klog.V(4).Infof("ClusterRoleBinding %s already removed", c.nodeRoleBinding.Name)
	}

	err = c.kubeClient.CoreV1().ServiceAccounts(c.operatorNamespace).Delete(ctx, c.nodeServiceAccount.Name, metav1.DeleteOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		klog.V(4).Infof("ServiceAccount %s already removed", c.nodeServiceAccount.Name)
	}

	// All removed, remove the finalizer as the last step
	err = c.operatorClient.RemoveFinalizer(c.operatorName)
	if err != nil {
		return err
	}
	return nil
}
