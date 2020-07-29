package operator

import (
	"bytes"
	"context"
	"fmt"

	"github.com/ovirt/csi-driver-operator/internal/ovirt"

	opv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/library-go/pkg/operator/csi/csicontrollerset"
	goc "github.com/openshift/library-go/pkg/operator/genericoperatorclient"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/staticresourcecontroller"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	ovirtsdk "github.com/ovirt/go-ovirt"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	dynamicclient "k8s.io/client-go/dynamic"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"k8s.io/client-go/kubernetes/scheme"
	"github.com/ovirt/csi-driver-operator/pkg/generated"
)

const (
	// Operand and operator run in the same namespace
	defaultNamespace = "openshift-cluster-csi-drivers"
	operatorName     = "ovirt-csi-driver-operator"
	operandName      = "ovirt-csi-driver"
	instanceName     = "csi.ovirt.org"
)

type CSIOperator struct {
	ovirtClient  *ovirt.Client
	nodeName     *string
	storageClass *storagev1.StorageClass
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

func (o *CSIOperator) getStorageDomain(ctx context.Context, kubeClient *kubeclient.Clientset) string {
	get, err := kubeClient.CoreV1().Nodes().Get(ctx, *o.nodeName, metav1.GetOptions{})
	if err != nil {
		klog.Fatal(err)
	}
	nodeID := get.Status.NodeInfo.SystemUUID

	conn, err := o.ovirtClient.GetConnection()
	if err != nil {
		klog.Fatal(err)
	}

	vmService := conn.SystemService().VmsService().VmService(nodeID)
	attachments, err := vmService.DiskAttachmentsService().List().Send()
	if err != nil {
		klog.Fatal(err)
	}

	for _, attachment := range attachments.MustAttachments().Slice() {
		if attachment.MustBootable() {
			d, _ := conn.FollowLink(attachment.MustDisk())
			disk, ok := d.(*ovirtsdk.Disk)
			klog.Info(fmt.Sprintf("Extracting Storage Domain from disk %s", disk.MustId()))

			if !ok {
				klog.Fatal("Could not fetch disk")
			}

			s, _ := conn.FollowLink(disk.MustStorageDomains().Slice()[0])
			sd, ok := s.(*ovirtsdk.StorageDomain)

			klog.Info(fmt.Sprintf("Fetched Storage Domain %s", sd.MustName()))
			if !ok {
				klog.Fatal("Could not fetch storage domain")
			}

			return sd.MustName()
		}
	}

	return ""
}

func (o *CSIOperator) addStorageClass(ctx context.Context, kubeClient *kubeclient.Clientset) {
	sdName := o.getStorageDomain(ctx, kubeClient)

	// Create StorageClass after the fact since we need to figure out a default Storage Domain for it
	storageClass := generateStorageClass(sdName)
	existingStorageClass, _ := kubeClient.StorageV1().StorageClasses().Get(ctx, storageClass.Name, metav1.GetOptions{})
	if existingStorageClass != nil {
		klog.Info("Storage Class already exists, removing...")
		kubeClient.StorageV1().StorageClasses().Delete(ctx, storageClass.Name, metav1.DeleteOptions{})
	}

	o.storageClass = storageClass
}

func (o *CSIOperator) RunOperator(ctx context.Context, controllerConfig *controllercmd.ControllerContext) error {
	// Create clientsets and informers
	kubeClient := kubeclient.NewForConfigOrDie(rest.AddUserAgent(controllerConfig.KubeConfig, operatorName))
	dynamicClient := dynamicclient.NewForConfigOrDie(rest.AddUserAgent(controllerConfig.KubeConfig, operatorName))
	kubeInformersForNamespaces := v1helpers.NewKubeInformersForNamespaces(kubeClient, defaultNamespace, "")
	// Create GenericOperatorclient. This is used by the library-go controllers created down below
	gvr := opv1.SchemeGroupVersion.WithResource("clustercsidrivers")
	operatorClient, dynamicInformers, err := goc.NewClusterScopedOperatorClientWithConfigName(controllerConfig.KubeConfig, gvr, instanceName)
	if err != nil {
		return err
	}

	o.addStorageClass(ctx, kubeClient)

	csiControllerSet := csicontrollerset.NewCSIControllerSet(
		operatorClient,
		controllerConfig.EventRecorder,
	).WithLogLevelController().WithManagementStateController(
		operandName,
		false,
	).WithStaticResourcesController(
		"OvirtDriverStaticResources",
		kubeClient,
		kubeInformersForNamespaces,
		generated.Asset,
		[]string{
			"csidriver.yaml",
			"controller_sa.yaml",
			"node_sa.yaml",
			"rbac/attacher_binding.yaml",
			"rbac/attacher_role.yaml",
			"rbac/controller_privileged_binding.yaml",
			"rbac/node_privileged_binding.yaml",
			"rbac/privileged_role.yaml",
			"rbac/provisioner_binding.yaml",
			"rbac/provisioner_role.yaml",
			"rbac/resizer_binding.yaml",
			"rbac/resizer_role.yaml",
			"rbac/snapshotter_binding.yaml",
			"rbac/snapshotter_role.yaml",
		},
	).WithCredentialsRequestController(
		"OvirtDriverCredentialsRequestController",
		defaultNamespace,
		generated.MustAsset,
		"credentials.yaml",
		dynamicClient,
	).WithCSIDriverController(
		"OvirtDriverController",
		instanceName,
		operandName,
		defaultNamespace,
		generated.MustAsset,
		kubeClient,
		kubeInformersForNamespaces.InformersFor(defaultNamespace),
		csicontrollerset.WithControllerService("controller.yaml"),
		csicontrollerset.WithNodeService("node.yaml"),
	)

	scController := staticresourcecontroller.NewStaticResourceController(
		"StorageClass",
		o.encoder,
		[]string{"storageclass.yaml"},
		(&resourceapply.ClientHolder{}).WithKubernetes(kubeClient),
		operatorClient,
		controllerConfig.EventRecorder,
	)

	if err != nil {
		return err
	}

	klog.Info("Starting the informers")
	go kubeInformersForNamespaces.Start(ctx.Done())
	go dynamicInformers.Start(ctx.Done())

	klog.Info("Starting controllerset")
	go csiControllerSet.Run(ctx, 1)
	go scController.Run(ctx, 1)
	<-ctx.Done()

	return fmt.Errorf("stopped")
}

func generateStorageClass(storageDomainName string) *storagev1.StorageClass {
	reclaimPolicy := corev1.PersistentVolumeReclaimDelete
	var expected = &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ovirt-csi-sc",
			Namespace: defaultNamespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "StorageClass",
			APIVersion: "storage.k8s.io/v1",
		},
		// ObjectMeta will be filled below
		Provisioner:          instanceName,
		Parameters:           map[string]string{"storageDomainName": storageDomainName, "thinProvisioning": "true"},
		ReclaimPolicy:        &reclaimPolicy,
		MountOptions:         []string{},
		AllowVolumeExpansion: boolPtr(false),
	}
	expected.Annotations = map[string]string{
		"storageclass.kubernetes.io/is-default-class": "true",
	}
	return expected
}

func boolPtr(val bool) *bool {
	return &val
}

func (o *CSIOperator) encoder(s string) ([]byte, error) {
	ser := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
	var bt []byte
	bf := bytes.NewBuffer(bt)
	ser.Encode(o.storageClass, bf)

	return bf.Bytes(), nil
}
