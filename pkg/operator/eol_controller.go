package operator

import (
	"context"
	"fmt"

	operatorapi "github.com/openshift/api/operator/v1"
	opclient "github.com/openshift/client-go/operator/clientset/versioned"
	opinformers "github.com/openshift/client-go/operator/informers/externalversions"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

type OvirtEOLController struct {
	name              string
	operatorClient    v1helpers.OperatorClient
	operatorClientSet *opclient.Clientset
	eventRecorder     events.Recorder
}

func NewOvirtEOLController(
	operatorClient v1helpers.OperatorClient,
	operatorClientSet *opclient.Clientset,
	operatorInformer opinformers.SharedInformerFactory,
	eventRecorder events.Recorder,
) factory.Controller {
	c := &OvirtEOLController{
		name:              "OvirtEOLController",
		operatorClient:    operatorClient,
		operatorClientSet: operatorClientSet,
		eventRecorder:     eventRecorder,
	}
	return factory.New().WithSync(c.sync).WithInformers(
		operatorInformer.Operator().V1().ClusterCSIDrivers().Informer(),
	).ToController(c.name, c.eventRecorder)
}

func (c *OvirtEOLController) markAsNonUpgradeable(ctx context.Context, desiredCondition operatorapi.OperatorCondition) error {
	_, _, err := v1helpers.UpdateStatus(ctx, c.operatorClient, func(status *operatorapi.OperatorStatus) error {
		status.Conditions = append(status.Conditions, desiredCondition)
		return nil
	})

	return err
}

func (c *OvirtEOLController) buildNonUpgradeableCondition() operatorapi.OperatorCondition {
	return operatorapi.OperatorCondition{
		Type:               c.name + operatorapi.OperatorStatusTypeUpgradeable,
		Status:             operatorapi.ConditionFalse,
		Message:            "oVirt is no longer supported and will be removed in a future release",
		Reason:             "EOL",
		LastTransitionTime: metav1.Now(),
	}
}

func (c *OvirtEOLController) sync(ctx context.Context, _ factory.SyncContext) error {
	clusterCSIDriver, err := c.operatorClientSet.OperatorV1().ClusterCSIDrivers().Get(ctx, instanceName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		klog.Info(fmt.Sprintf("No ClusterCSIDriver '%s' found.", instanceName))
		return nil
	}

	desiredCondition := c.buildNonUpgradeableCondition()
	for _, condition := range clusterCSIDriver.Status.Conditions {
		if condition.Type == desiredCondition.Type && condition.Status == desiredCondition.Status {
			klog.Info("Operator is already flagged with Upgradeable=False. Skipping...")
			return nil
		}
	}

	klog.Info("Updating OperatorStatus.Upgradeable=False due to EOL...")
	err = c.markAsNonUpgradeable(ctx, desiredCondition)
	if err != nil {
		klog.Error(fmt.Errorf("Failed to mark oVirtCSIDriverOperator as not upgradeable: %s", err))
	} else {
		klog.Info("Updating OperatorStatus.Upgradeable=False succeeded")
	}
	return nil
}
