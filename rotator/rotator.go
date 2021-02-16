package rotator

import (
	"strings"
	"time"

	"github.com/mattermost/node-rotator/model"
	"github.com/pkg/errors"
)

type AutoscalingGroup struct {
	Name            string
	DesiredCapacity int64
	Nodes           []string
}

// InitRotateCluster is used to call the RotateCluster function.
func InitRotateCluster(cluster *model.Cluster) {
	err := RotateCluster(cluster)
	if err != nil {
		logger.WithError(err).Error("Failed to rotate the cluster nodes")
	}
}

// RotateCluster is used to rotate the Cluster nodes.
func RotateCluster(cluster *model.Cluster) error {
	autoscalingGroups, err := GetAutoscalingGroups(cluster.ClusterID)
	if err != nil {
		return err
	}
	logger.Infof("Cluster with cluster ID %s is consisted of %d Autoscaling Groups", cluster.ClusterID, len(autoscalingGroups))

	for _, asg := range autoscalingGroups {
		autoscalingGroup := AutoscalingGroup{}
		err := autoscalingGroup.setObject(asg)
		if err != nil {
			return err
		}

		if strings.Contains(autoscalingGroup.Name, "master") && cluster.RotateMasters {
			logger.Infof("The autoscaling group %s has %d instance(s)", autoscalingGroup.Name, autoscalingGroup.DesiredCapacity)

			err = autoscalingGroup.masterNodeRotation(cluster)
			if err != nil {
				return err
			}

			logger.Infof("Checking that all %d nodes are running...", autoscalingGroup.DesiredCapacity)
			err = autoscalingGroup.finalCheck()
			if err != nil {
				return err
			}

			logger.Infof("ASG %s rotated successfully.", autoscalingGroup.Name)
		} else if !strings.Contains(autoscalingGroup.Name, "master") && cluster.RotateWorkers {
			logger.Infof("The autoscaling group %s has %d instance(s)", autoscalingGroup.Name, autoscalingGroup.DesiredCapacity)

			err = autoscalingGroup.workerNodeRotation(cluster)
			if err != nil {
				return err
			}

			logger.Infof("Checking that all %d nodes are running...", autoscalingGroup.DesiredCapacity)
			err = autoscalingGroup.finalCheck()
			if err != nil {
				return err
			}

			logger.Infof("ASG %s rotated successfully.", autoscalingGroup.Name)
		}
	}
	logger.Info("All ASGs rotated successfully")
	return nil
}

func (autoscalingGroup *AutoscalingGroup) finalCheck() error {
	asg, err := autoscalingGroup.AutoScalingGroupReady()
	if err != nil {
		return errors.Wrap(err, "Failed to get AutoscalingGroup ready")
	}

	asgNodes, err := GetNodeHostnames(asg.Instances)
	if err != nil {
		return errors.Wrap(err, "Failed to get node hostnames")
	}

	err = NodesReady(asgNodes)
	if err != nil {
		return errors.Wrap(err, "Failed to get cluster nodes ready")
	}

	return nil
}

func (autoscalingGroup *AutoscalingGroup) masterNodeRotation(cluster *model.Cluster) error {
	for len(autoscalingGroup.Nodes) > 0 {
		logger.Infof("The number of nodes in the ASG to be rotated is %d", len(autoscalingGroup.Nodes))

		nodesToRotate := []string{autoscalingGroup.Nodes[0]}

		err := DrainNodes(nodesToRotate, 10, int(cluster.EvictGracePeriod))
		if err != nil {
			return err
		}

		err = DetachNodes(false, nodesToRotate, autoscalingGroup.Name)
		if err != nil {
			return err
		}

		err = TerminateNodes(nodesToRotate)
		if err != nil {
			return err
		}

		logger.Info("Sleeping 60 seconds for autoscaling group to balance...")
		time.Sleep(60 * time.Second)

		autoscalingGroupReady, err := autoscalingGroup.AutoScalingGroupReady()
		if err != nil {
			return err
		}

		nodeHostnames, err := GetNodeHostnames(autoscalingGroupReady.Instances)
		if err != nil {
			return err
		}

		newNodes := autoscalingGroup.newNodes(nodeHostnames)
		if err != nil {
			return err
		}

		err = NodesReady(newNodes)
		if err != nil {
			return err
		}

		logger.Info("Removing nodes from rotation list")
		autoscalingGroup.PopNodes(nodesToRotate)

	}
	return nil
}

func (autoscalingGroup *AutoscalingGroup) workerNodeRotation(cluster *model.Cluster) error {
	for len(autoscalingGroup.Nodes) > 0 {
		logger.Infof("The number of nodes in the ASG to be rotated is %d", len(autoscalingGroup.Nodes))

		var nodesToRotate []string

		if len(autoscalingGroup.Nodes) < int(cluster.MaxScaling) {
			nodesToRotate = autoscalingGroup.Nodes[:len(autoscalingGroup.Nodes)]
		} else {
			nodesToRotate = autoscalingGroup.Nodes[:int(cluster.MaxScaling)]
		}

		err := DetachNodes(false, nodesToRotate, autoscalingGroup.Name)
		if err != nil {
			return err
		}

		logger.Info("Sleeping 60 seconds for autoscaling group to balance...")
		time.Sleep(60 * time.Second)

		autoscalingGroupReady, err := autoscalingGroup.AutoScalingGroupReady()
		if err != nil {
			return err
		}

		nodeHostnames, err := GetNodeHostnames(autoscalingGroupReady.Instances)
		if err != nil {
			return err
		}

		newNodes := autoscalingGroup.newNodes(nodeHostnames)
		if err != nil {
			return err
		}

		err = NodesReady(newNodes)
		if err != nil {
			return err
		}

		err = DrainNodes(nodesToRotate, 10, int(cluster.EvictGracePeriod))
		if err != nil {
			return err
		}

		err = TerminateNodes(nodesToRotate)
		if err != nil {
			return err
		}

		err = DeleteClusterNodes(nodesToRotate)
		if err != nil {
			return err
		}

		logger.Info("Removing nodes from rotation list")
		autoscalingGroup.PopNodes(nodesToRotate)

		if len(autoscalingGroup.Nodes) > 0 {
			logger.Infof("Waiting for %d seconds before next node rotation", cluster.WaitBetweenRotations)
			time.Sleep(time.Duration(cluster.WaitBetweenRotations) * time.Second)
		}
	}

	return nil
}
