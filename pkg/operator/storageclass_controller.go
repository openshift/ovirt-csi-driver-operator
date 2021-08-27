package operator

import (
	"context"
	"fmt"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	"github.com/ovirt/csi-driver-operator/internal/ovirt"
	ovirtsdk "github.com/ovirt/go-ovirt"
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
	ovirtClient               *ovirt.Client
	nodeName                  string
}

func NewOvirtStrogeClassController(operatorClient v1helpers.OperatorClient,
	kubeClient kubernetes.Interface,
	kubeInformersForNamespace v1helpers.KubeInformersForNamespaces,
	ovirtClient *ovirt.Client,
	nodeName string,
	eventRecorder events.Recorder) factory.Controller {
	c := &OvirtStrogeClassController{
		operatorClient: operatorClient,
		kubeClient:     kubeClient,
		eventRecorder:  eventRecorder,
		ovirtClient:    ovirtClient,
		nodeName:       nodeName,
	}
	return factory.New().WithSync(c.sync).WithSyncDegradedOnError(operatorClient).WithInformers(
		operatorClient.Informer(),
		kubeInformersForNamespace.InformersFor("").Storage().V1().StorageClasses().Informer(),
	).ToController("OvirtStorageClassController", eventRecorder)
}

func (c *OvirtStrogeClassController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	sdName, err := c.getStorageDomain(ctx)
	if err != nil {
		klog.Errorf(fmt.Sprintf("Failed to get Storage Domain name: %v", err))
		return err
	}

	storageClass := generateStorageClass(sdName)
	existingStorageClass, err := c.kubeClient.StorageV1().StorageClasses().Get(ctx, storageClass.Name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Errorf(fmt.Sprintf("Failed to issue get request for storage class %s, error: %v", storageClass.Name, err))
			return err
		}
	} else {
		klog.Info(fmt.Sprintf("Storage Class %s already exists", existingStorageClass.Name))
		storageClass = existingStorageClass
	}

	_, _, err = resourceapply.ApplyStorageClass(ctx, c.kubeClient.StorageV1(), c.eventRecorder, storageClass)
	if err != nil {
		klog.Errorf(fmt.Sprintf("Failed to apply storage class: %v", err))
		return err
	}

	return nil
}

func (c *OvirtStrogeClassController) getStorageDomain(ctx context.Context) (string, error) {
	get, err := c.kubeClient.CoreV1().Nodes().Get(ctx, c.nodeName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf(fmt.Sprintf("Failed to get node: %v", err))
		return "", err
	}
	nodeID := get.Status.NodeInfo.SystemUUID

	conn, err := c.ovirtClient.GetConnection()
	if err != nil {
		klog.Errorf(fmt.Sprintf("Connection to ovirt failed: %v", err))
		return "", err
	}

	vmService := conn.SystemService().VmsService().VmService(nodeID)
	attachments, err := vmService.DiskAttachmentsService().List().Send()
	if err != nil {
		klog.Errorf(fmt.Sprintf("Failed to fetch attachments: %v", err))
		return "", err
	}

	for _, attachment := range attachments.MustAttachments().Slice() {
		if attachment.MustBootable() {
			d, err := conn.FollowLink(attachment.MustDisk())
			if err != nil {
				klog.Errorf("Failed to follow disk: %v", err)
				return "", err
			}

			disk, ok := d.(*ovirtsdk.Disk)
			klog.Info(fmt.Sprintf("Extracting Storage Domain from disk: %s", disk.MustId()))

			if !ok {
				klog.Errorf(fmt.Sprintf("Failed to fetch disk: %v", err))
				return "", err
			}

			s, err := conn.FollowLink(disk.MustStorageDomains().Slice()[0])
			if err != nil {
				klog.Errorf("Failed to follow Storage Domain: %v", err)
				return "", err
			}
			sd, ok := s.(*ovirtsdk.StorageDomain)

			klog.Info(fmt.Sprintf("Fetched Storage Domain %s", sd.MustName()))
			if !ok {
				klog.Errorf(fmt.Sprintf("Failed to fetch Storage Domain: %v", err))
				return "", err
			}

			return sd.MustName(), nil
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
