package operator

import (
	"context"
	"fmt"

	operatorapi "github.com/openshift/api/operator/v1"
	opinformers "github.com/openshift/client-go/operator/informers/externalversions"
	"github.com/openshift/library-go/pkg/controller/factory"
	csiscc "github.com/openshift/library-go/pkg/operator/csi/csistorageclasscontroller"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	ovirtclient "github.com/ovirt/go-ovirt-client/v2"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type OvirtStorageClassController struct {
	operatorClient            v1helpers.OperatorClient
	kubeClient                kubernetes.Interface
	kubeInformersForNamespace v1helpers.KubeInformersForNamespaces
	eventRecorder             events.Recorder
	ovirtClientFactory        func() (ovirtclient.Client, error)
	nodeName                  string
	scStateEvaluator          *csiscc.StorageClassStateEvaluator
}

func NewOvirtStorageClassController(
	operatorClient v1helpers.OperatorClient,
	kubeClient kubernetes.Interface,
	kubeInformersForNamespace v1helpers.KubeInformersForNamespaces,
	operatorInformer opinformers.SharedInformerFactory,
	ovirtClientFactory func() (ovirtclient.Client, error),
	nodeName string,
	eventRecorder events.Recorder,
) factory.Controller {
	clusterCSIDriverLister := operatorInformer.Operator().V1().ClusterCSIDrivers().Lister()
	evaluator := csiscc.NewStorageClassStateEvaluator(
		kubeClient,
		clusterCSIDriverLister,
		eventRecorder,
	)
	c := &OvirtStorageClassController{
		operatorClient:     operatorClient,
		kubeClient:         kubeClient,
		eventRecorder:      eventRecorder,
		ovirtClientFactory: ovirtClientFactory,
		nodeName:           nodeName,
		scStateEvaluator:   evaluator,
	}
	return factory.New().WithSync(c.sync).WithSyncDegradedOnError(operatorClient).WithInformers(
		operatorClient.Informer(),
		kubeInformersForNamespace.InformersFor("").Storage().V1().StorageClasses().Informer(),
		operatorInformer.Operator().V1().ClusterCSIDrivers().Informer(),
	).ToController("OvirtStorageClassController", eventRecorder)
}

func (c *OvirtStorageClassController) sync(ctx context.Context, _ factory.SyncContext) error {
	scState := c.scStateEvaluator.GetStorageClassState(instanceName)
	sdName, err := c.getStorageDomain(ctx, scState)
	if err != nil {
		klog.Errorf("failed to get Storage Domain name: %w", err)
		return err
	}

	storageClass := generateStorageClass(sdName)
	existingStorageClass, err := c.kubeClient.StorageV1().StorageClasses().Get(ctx, storageClass.Name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Errorf("failed to issue get request for storage class %s, error: %w", storageClass.Name, err)
			return err
		}
	} else {
		klog.Infof("Storage Class %s already exists", existingStorageClass.Name)
		storageClass = existingStorageClass
	}

	err = c.scStateEvaluator.ApplyStorageClass(ctx, storageClass, scState)
	if err != nil {
		klog.Errorf("failed to apply storage class: %w", err)
		return err
	}

	return nil
}

func (c *OvirtStorageClassController) getStorageDomain(ctx context.Context, scState operatorapi.StorageClassStateName) (string, error) {
	// if the SC is not managed, there is no need to get the storage domain
	if !c.scStateEvaluator.IsManaged(scState) {
		return "", nil
	}

	get, err := c.kubeClient.CoreV1().Nodes().Get(ctx, c.nodeName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("failed to get node: %w", err)
		return "", err
	}
	nodeID := get.Status.NodeInfo.SystemUUID

	ovirtClient, err := c.ovirtClientFactory()
	if err != nil {
		return "", fmt.Errorf("failed to create oVirt client (%w)", err)
	}
	attachments, err := ovirtClient.ListDiskAttachments(ovirtclient.VMID(nodeID), ovirtclient.ContextStrategy(ctx))
	if err != nil {
		klog.Errorf("failed to fetch attachments: %w", err)
		return "", err
	}

	for _, attachment := range attachments {
		if attachment.Bootable() {
			d, err := attachment.Disk(ovirtclient.ContextStrategy(ctx))
			if err != nil {
				klog.Errorf("failed to fetch disk: %w", err)
				return "", err
			}
			klog.Infof("Extracting Storage Domain from disk: %s", d.ID())
			storageDomains := d.StorageDomainIDs()
			if len(storageDomains) == 0 {
				return "", fmt.Errorf("no storage domains found on disk %s", d.ID())
			}
			storageDoamin, err := ovirtClient.GetStorageDomain(storageDomains[0])
			if err != nil {
				klog.Errorf("failed while finding storage domain by ID %s, error: %w", storageDomains[0], err)
				return "", err
			}
			return storageDoamin.Name(), nil
		}
	}

	return "", nil
}

func generateStorageClass(storageDomainName string) *storagev1.StorageClass {
	reclaimPolicy := corev1.PersistentVolumeReclaimDelete
	expected := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ovirt-csi-sc",
			Namespace: defaultNamespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "StorageClass",
			APIVersion: "storage.k8s.io/v1",
		},
		Provisioner:          instanceName,
		Parameters:           map[string]string{"storageDomainName": storageDomainName, "thinProvisioning": "true"},
		ReclaimPolicy:        &reclaimPolicy,
		MountOptions:         []string{},
		AllowVolumeExpansion: boolPtr(true),
	}

	expected.Annotations = map[string]string{
		"storageclass.kubernetes.io/is-default-class": "true",
	}

	return expected
}

func boolPtr(val bool) *bool {
	return &val
}
