package rotator

import (
	"context"

	"github.com/aws/aws-sdk-go/service/autoscaling"
	awsTools "github.com/mattermost/node-rotator/aws"
	k8sTools "github.com/mattermost/node-rotator/k8s"
	"github.com/mattermost/node-rotator/model"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func newNodes(allNodes, oldNodes []string) []string {
	var newNodes []string
	for _, node := range allNodes {
		for _, oldNode := range oldNodes {
			if node != oldNode {
				newNodes = append(newNodes, node)
			}
		}
	}
	return newNodes
}

func (autoscalingGroup *AutoscalingGroup) SetObject(asg *autoscaling.Group) error {
	autoscalingGroup.Name = *asg.AutoScalingGroupName
	autoscalingGroup.DesiredCapacity = *asg.DesiredCapacity
	nodeHostNames, err := awsTools.GetNodeHostnames(asg.Instances)
	if err != nil {
		return errors.Wrap(err, "Failed to get asg instance node names and set asg object")
	}
	autoscalingGroup.Nodes = nodeHostNames
	return nil
}

func (autoscalingGroup *AutoscalingGroup) popNodes(popNodes []string) {
	var updatedList []string
	for _, node := range autoscalingGroup.Nodes {
		nodeFound := false
		for _, popNode := range popNodes {
			if popNode == node {
				nodeFound = true
				break
			}
		}
		if !nodeFound {
			updatedList = append(updatedList, node)
		}
	}
	autoscalingGroup.Nodes = updatedList
	return
}

func (autoscalingGroup *AutoscalingGroup) popNode(i int) {
	copy(autoscalingGroup.Nodes[i:], autoscalingGroup.Nodes[i+1:])                  // Shift a[i+1:] left one index.
	autoscalingGroup.Nodes[len(autoscalingGroup.Nodes)-1] = ""                      // Erase last element (write zero value).
	autoscalingGroup.Nodes = autoscalingGroup.Nodes[:len(autoscalingGroup.Nodes)-1] // Truncate slice.
}

func (autoscalingGroup *AutoscalingGroup) RemoveDeletedNode(nodeToDelete string) {
	for i, node := range autoscalingGroup.Nodes {
		if node == nodeToDelete {
			autoscalingGroup.Nodes = append(autoscalingGroup.Nodes[:i], autoscalingGroup.Nodes[i+1:]...)
		}
	}
}

func DrainNodes(nodesToDrain []string, attempts, gracePeriod int, clientset *kubernetes.Clientset) error {
	ctx := context.TODO()

	drainOptions := &DrainOptions{
		DeleteLocalData:    true,
		IgnoreDaemonsets:   true,
		Timeout:            600,
		GracePeriodSeconds: gracePeriod,
	}

	logger.Infof("Draining %d nodes", len(nodesToDrain))

	for _, nodeToDrain := range nodesToDrain {
		logger.Infof("Draining node %s", nodeToDrain)
		node, err := clientset.CoreV1().Nodes().Get(ctx, nodeToDrain, metav1.GetOptions{})
		if err != nil {
			return errors.Wrapf(err, "Failed to get node %s", nodeToDrain)
		}
		err = Drain(clientset, []*corev1.Node{node}, drainOptions)
		if err != nil {
			if attempts--; attempts > 0 {
				logger.Infof("Node %s drain failed, retrying...", nodeToDrain)
				DrainNodes([]string{nodeToDrain}, attempts-1, gracePeriod, clientset)
			} else {
				return errors.Wrapf(err, "Failed to drain node %s", nodeToDrain)
			}
		}
		logger.Infof("Node %s drained successfully", nodeToDrain)
	}

	return nil
}

func getk8sClientset(cluster *model.Cluster) (*kubernetes.Clientset, error) {
	if cluster.ClientSet != nil {
		return cluster.ClientSet, nil
	}

	clientSet, err := k8sTools.GetClientset()
	if err != nil {
		return nil, err
	}
	return clientSet, nil

}
