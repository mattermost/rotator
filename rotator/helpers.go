package rotator

import (
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/pkg/errors"
)

func (autoscalingGroup *AutoscalingGroup) newNodes(allNodes []string) []string {
	var newNodes []string
	for _, node := range allNodes {
		for _, oldNode := range autoscalingGroup.Nodes {
			if node != oldNode {
				newNodes = append(newNodes, node)
			}
		}
	}
	return newNodes
}

func (autoscalingGroup *AutoscalingGroup) setObject(asg *autoscaling.Group) error {
	autoscalingGroup.Name = *asg.AutoScalingGroupName
	autoscalingGroup.DesiredCapacity = *asg.DesiredCapacity
	nodeHostNames, err := GetNodeHostnames(asg.Instances)
	if err != nil {
		return errors.Wrap(err, "Failed to get asg instance node names and set asg object")
	}
	autoscalingGroup.Nodes = nodeHostNames
	return nil
}

func (autoscalingGroup *AutoscalingGroup) PopNodes(popNodes []string) {
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
