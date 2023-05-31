package operator

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift/library-go/pkg/operator/csi/csidrivercontrollerservicecontroller"
	"github.com/openshift/library-go/pkg/operator/csi/csidrivernodeservicecontroller"
	ovirtclient "github.com/ovirt/go-ovirt-client/v2"

	"github.com/ovirt/csi-driver-operator/assets"
	"github.com/ovirt/csi-driver-operator/internal/ovirt"

	opv1 "github.com/openshift/api/operator/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	configinformers "github.com/openshift/client-go/config/informers/externalversions"
	opclient "github.com/openshift/client-go/operator/clientset/versioned"
	opinformers "github.com/openshift/client-go/operator/informers/externalversions"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/csi/csicontrollerset"
	goc "github.com/openshift/library-go/pkg/operator/genericoperatorclient"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	"k8s.io/client-go/dynamic"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

const (
	// Operand and operator run in the same namespace
	defaultNamespace   = "openshift-cluster-csi-drivers"
	operatorName       = "ovirt-csi-driver-operator"
	operandName        = "ovirt-csi-driver"
	instanceName       = "csi.ovirt.org"
	trustedCAConfigMap = "ovirt-csi-driver-trusted-ca-bundle"
	secretName         = "ovirt-credentials"
	resync             = 20 * time.Minute
)

type CSIOperator struct {
	ovirtClient ovirtclient.Client
	nodeName    *string
}

func NewCSIOperator(nodeName *string) (*CSIOperator, error) {
	client, err := ovirt.NewClient()
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	return &CSIOperator{
		ovirtClient: client,
		nodeName:    nodeName,
	}, nil
}

func (o *CSIOperator) getConnection() (ovirtclient.Client, error) {
	var err error
	if o.ovirtClient == nil || o.ovirtClient.Test() != nil {
		o.ovirtClient, err = ovirt.NewClient()
	}

	return o.ovirtClient, err
}

func (o *CSIOperator) RunOperator(ctx context.Context, controllerConfig *controllercmd.ControllerContext) error {
	// Create clientsets and informers
	kubeClient := kubeclient.NewForConfigOrDie(rest.AddUserAgent(controllerConfig.KubeConfig, operatorName))
	kubeInformersForNamespaces := v1helpers.NewKubeInformersForNamespaces(kubeClient, defaultNamespace, "")
	nodeInformer := kubeInformersForNamespaces.InformersFor("").Core().V1().Nodes()
	configMapInformer := kubeInformersForNamespaces.InformersFor(defaultNamespace).Core().V1().ConfigMaps()
	secretInformer := kubeInformersForNamespaces.InformersFor(defaultNamespace).Core().V1().Secrets()
	// Create GenericOperatorclient. This is used by the library-go controllers created down below
	gvr := opv1.GroupVersion.WithResource("clustercsidrivers")
	operatorClient, dynamicInformers, err := goc.NewClusterScopedOperatorClientWithConfigName(controllerConfig.KubeConfig, gvr, instanceName)
	if err != nil {
		return err
	}
	// Create config clientset and informer. This is used to get the HTTP proxy setting
	configClient := configclient.NewForConfigOrDie(rest.AddUserAgent(controllerConfig.KubeConfig, operatorName))
	configInformers := configinformers.NewSharedInformerFactory(configClient, resync)

	// operator.openshift.io client, used for ClusterCSIDriver
	operatorClientSet := opclient.NewForConfigOrDie(rest.AddUserAgent(controllerConfig.KubeConfig, operatorName))
	operatorInformers := opinformers.NewSharedInformerFactory(operatorClientSet, resync)

	dynamicClient, err := dynamic.NewForConfig(controllerConfig.KubeConfig)
	if err != nil {
		return err
	}

	csiControllerSet := csicontrollerset.NewCSIControllerSet(
		operatorClient,
		controllerConfig.EventRecorder,
	).WithLogLevelController().WithManagementStateController(
		operandName,
		false,
	).WithStaticResourcesController(
		"OvirtDriverStaticResources",
		kubeClient,
		dynamicClient,
		kubeInformersForNamespaces,
		assets.ReadFile,
		[]string{
			"controller_sa.yaml",
			"node_sa.yaml",
			"cabundle_cm.yaml",
			"rbac/attacher_role.yaml",
			"rbac/privileged_role.yaml",
			"rbac/provisioner_role.yaml",
			"rbac/resizer_role.yaml",
			"rbac/snapshotter_role.yaml",
			"rbac/kube_rbac_proxy_role.yaml",
			"rbac/prometheus_role.yaml",
			"rbac/attacher_binding.yaml",
			"rbac/node_privileged_binding.yaml",
			"rbac/controller_privileged_binding.yaml",
			"rbac/provisioner_binding.yaml",
			"rbac/resizer_binding.yaml",
			"rbac/snapshotter_binding.yaml",
			"rbac/kube_rbac_proxy_binding.yaml",
			"rbac/prometheus_rolebinding.yaml",
			"controller_pdb.yaml",
			"service.yaml",
			"csidriver.yaml",
		},
	).WithCSIConfigObserverController(
		"OvirtDriverCSIConfigObserverController",
		configInformers,
	).WithCSIDriverControllerService(
		"OvirtDriverControllerServiceController",
		assets.ReadFile,
		"controller.yaml",
		kubeClient,
		kubeInformersForNamespaces.InformersFor(defaultNamespace),
		configInformers,
		[]factory.Informer{
			nodeInformer.Informer(),
			configMapInformer.Informer(),
			secretInformer.Informer(),
		},
		csidrivercontrollerservicecontroller.WithSecretHashAnnotationHook(defaultNamespace, secretName, secretInformer),
		csidrivercontrollerservicecontroller.WithObservedProxyDeploymentHook(),
		csidrivercontrollerservicecontroller.WithReplicasHook(nodeInformer.Lister()),
		csidrivercontrollerservicecontroller.WithCABundleDeploymentHook(
			defaultNamespace,
			trustedCAConfigMap,
			configMapInformer,
		),
	).WithCSIDriverNodeService(
		"OvirtDriverNodeServiceController",
		assets.ReadFile,
		"node.yaml",
		kubeClient,
		kubeInformersForNamespaces.InformersFor(defaultNamespace),
		[]factory.Informer{
			configMapInformer.Informer(),
			secretInformer.Informer(),
		},
		csidrivernodeservicecontroller.WithSecretHashAnnotationHook(defaultNamespace, secretName, secretInformer),
		csidrivernodeservicecontroller.WithObservedProxyDaemonSetHook(),
		csidrivernodeservicecontroller.WithCABundleDaemonSetHook(
			defaultNamespace,
			trustedCAConfigMap,
			configMapInformer,
		),
	).WithServiceMonitorController(
		"OvirtDriverServiceMonitorController",
		dynamicClient,
		assets.ReadFile,
		"servicemonitor.yaml",
	)

	scController := NewOvirtStorageClassController(
		operatorClient,
		kubeClient,
		kubeInformersForNamespaces,
		operatorInformers,
		o.getConnection,
		*o.nodeName,
		controllerConfig.EventRecorder,
	)

	eolController := NewOvirtEOLController(
		operatorClient,
		operatorClientSet,
		operatorInformers,
		controllerConfig.EventRecorder,
	)

	klog.Info("Starting the informers")
	go kubeInformersForNamespaces.Start(ctx.Done())
	go dynamicInformers.Start(ctx.Done())
	go configInformers.Start(ctx.Done())
	go operatorInformers.Start(ctx.Done())

	klog.Info("Starting controllerset")
	go csiControllerSet.Run(ctx, 1)
	go scController.Run(ctx, 1)
	go eolController.Run(ctx, 1)

	<-ctx.Done()

	return fmt.Errorf("stopped")
}
