package operator

import (
	"context"
	"fmt"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	ovirtclient "github.com/ovirt/go-ovirt-client"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type OvirtStrogeClassController struct {
	operatorClient            v1helpers.OperatorClient
	kubeClient                kubernetes.Interface
	kubeInformersForNamespace v1helpers.KubeInformersForNamespaces
	eventRecorder             events.Recorder
	ovirtClientFactory        func() (ovirtclient.Client, error)
	nodeName                  string
}

func NewOvirtStrogeClassController(
	operatorClient v1helpers.OperatorClient,
	kubeClient kubernetes.Interface,
	kubeInformersForNamespace v1helpers.KubeInformersForNamespaces,
	ovirtClientFactory func() (ovirtclient.Client, error),
	nodeName string,
	eventRecorder events.Recorder,
) factory.Controller {
	c := &OvirtStrogeClassController{
		operatorClient:     operatorClient,
		kubeClient:         kubeClient,
		eventRecorder:      eventRecorder,
		ovirtClientFactory: ovirtClientFactory,
		nodeName:           nodeName,
	}
	return factory.New().WithSync(c.sync).WithSyncDegradedOnError(operatorClient).WithInformers(
		operatorClient.Informer(),
		kubeInformersForNamespace.InformersFor("").Storage().V1().StorageClasses().Informer(),
	).ToController("OvirtStorageClassController", eventRecorder)
}

func (c *OvirtStrogeClassController) sync(ctx context.Context, _ factory.SyncContext) error {
	sdName, err := c.getStorageDomain(ctx)
	if err != nil {
		klog.Errorf("failed to get Storage Domain name: %w", err)
		return err
	}

	storageClass := generateStorageClass(sdName)
	existingStorageClass, err := c.kubeClient.StorageV1().StorageClasses().Get(ctx, storageClass.Name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Errorf(
				"failed to issue get request for storage class %s, error: %w", storageClass.Name, err)
			return err
		}
	} else {
		klog.Info("Storage Class %s already exists", existingStorageClass.Name)
		storageClass = existingStorageClass
	}

	_, _, err = resourceapply.ApplyStorageClass(ctx, c.kubeClient.StorageV1(), c.eventRecorder, storageClass)
	if err != nil {
		klog.Errorf("failed to apply storage class: %w", err)
		return err
	}

	return nil
}

func (c *OvirtStrogeClassController) getStorageDomain(ctx context.Context) (string, error) {
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
	attachments, err := ovirtClient.ListDiskAttachments(nodeID, ovirtclient.ContextStrategy(ctx))
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
			klog.Info("Extracting Storage Domain from disk: %s", d.ID())
			storageDoamin, err := ovirtClient.GetStorageDomain(d.StorageDomainID())
			if err != nil {
				klog.Errorf("failed while finding storage domain by ID %s, error: %w", d.StorageDomainID(), err)
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
