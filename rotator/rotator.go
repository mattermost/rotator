package rotator

import (
	"strings"
	"time"

	awsTools "github.com/mattermost/node-rotator/aws"
	k8sTools "github.com/mattermost/node-rotator/k8s"
	"github.com/mattermost/node-rotator/model"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

type AutoscalingGroup struct {
	Name            string
	DesiredCapacity int64
	Nodes           []string
}

// InitRotateCluster is used to call the RotateCluster function.
func InitRotateCluster(cluster *model.Cluster) {
	logger := logger.WithField("cluster", cluster.ClusterID)
	err := RotateCluster(cluster, logger)
	if err != nil {
		logger.WithError(err).Error("Failed to rotate the cluster nodes")
	}
}

// RotateCluster is used to rotate the Cluster nodes.
func RotateCluster(cluster *model.Cluster, logger *logrus.Entry) error {

	clientset, err := getk8sClientset(cluster)
	if err != nil {
		return err
	}
	// if len(cluster.Cluster.RotatorMetadata.MasterGroups) > 0  || cluster.Cluster.RotatorMetadata.WorkerGroups > 0 {
	// 	for _, asg := range cluster.Cluster.RotatorMetadata.MasterGroups {
	// 		logger.Infof("The autoscaling group %s has %d instance(s)", autoscalingGroup.Name, autoscalingGroup.DesiredCapacity)

	// 		err = masterNodeRotation(cluster, &autoscalingGroup, clientset, logger)
	// 		if err != nil {
	// 			return err
	// 		}

	// 		logger.Infof("Checking that all %d nodes are running...", autoscalingGroup.DesiredCapacity)
	// 		err = finalCheck(&autoscalingGroup, clientset, logger)
	// 		if err != nil {
	// 			return err
	// 		}

	// 		logger.Infof("ASG %s rotated successfully.", autoscalingGroup.Name)
	// 	}

	// 	for _, asg := range cluster.Cluster.RotatorMetadata.WorkerGroups {
	// 		logger.Infof("The autoscaling group %s has %d instance(s)", autoscalingGroup.Name, autoscalingGroup.DesiredCapacity)

	// 		err = workerNodeRotation(cluster, &autoscalingGroup, clientset, logger)
	// 		if err != nil {
	// 			return err
	// 		}

	// 		logger.Infof("Checking that all %d nodes are running...", autoscalingGroup.DesiredCapacity)
	// 		err = finalCheck(&autoscalingGroup, clientset, logger)
	// 		if err != nil {
	// 			return err
	// 		}
	// 	}
	// }

	autoscalingGroups, err := awsTools.GetAutoscalingGroups(cluster.ClusterID)
	if err != nil {
		return err
	}
	logger.Infof("Cluster with cluster ID %s is consisted of %d Autoscaling Groups", cluster.ClusterID, len(autoscalingGroups))

	for _, asg := range autoscalingGroups {
		autoscalingGroup := AutoscalingGroup{}
		err := autoscalingGroup.SetObject(asg)
		if err != nil {
			return err
		}

		if strings.Contains(autoscalingGroup.Name, "master") && cluster.RotateMasters {
			logger.Infof("The autoscaling group %s has %d instance(s)", autoscalingGroup.Name, autoscalingGroup.DesiredCapacity)

			err = masterNodeRotation(cluster, &autoscalingGroup, clientset, logger)
			if err != nil {
				return err
			}

			logger.Infof("Checking that all %d nodes are running...", autoscalingGroup.DesiredCapacity)
			err = finalCheck(&autoscalingGroup, clientset, logger)
			if err != nil {
				return err
			}

			logger.Infof("ASG %s rotated successfully.", autoscalingGroup.Name)
		} else if !strings.Contains(autoscalingGroup.Name, "master") && cluster.RotateWorkers {
			logger.Infof("The autoscaling group %s has %d instance(s)", autoscalingGroup.Name, autoscalingGroup.DesiredCapacity)

			err = workerNodeRotation(cluster, &autoscalingGroup, clientset, logger)
			if err != nil {
				return err
			}

			logger.Infof("Checking that all %d nodes are running...", autoscalingGroup.DesiredCapacity)
			err = finalCheck(&autoscalingGroup, clientset, logger)
			if err != nil {
				return err
			}

			logger.Infof("ASG %s rotated successfully.", autoscalingGroup.Name)
		}
	}
	logger.Info("All ASGs rotated successfully")
	return nil
}

func finalCheck(autoscalingGroup *AutoscalingGroup, clientset *kubernetes.Clientset, logger *logrus.Entry) error {
	asg, err := awsTools.AutoScalingGroupReady(autoscalingGroup.Name, autoscalingGroup.DesiredCapacity, logger)
	if err != nil {
		return errors.Wrap(err, "Failed to get AutoscalingGroup ready")
	}

	asgNodes, err := awsTools.GetNodeHostnames(asg.Instances)
	if err != nil {
		return errors.Wrap(err, "Failed to get node hostnames")
	}

	err = k8sTools.NodesReady(asgNodes, clientset, logger)
	if err != nil {
		return errors.Wrap(err, "Failed to get cluster nodes ready")
	}

	return nil
}

func masterNodeRotation(cluster *model.Cluster, autoscalingGroup *AutoscalingGroup, clientset *kubernetes.Clientset, logger *logrus.Entry) error {

	for len(autoscalingGroup.Nodes) > 0 {
		logger.Infof("The number of nodes in the ASG to be rotated is %d", len(autoscalingGroup.Nodes))

		nodesToRotate := []string{autoscalingGroup.Nodes[0]}

		err := DrainNodes(nodesToRotate, 10, int(cluster.EvictGracePeriod), clientset)
		if err != nil {
			return err
		}

		err = awsTools.DetachNodes(false, nodesToRotate, autoscalingGroup.Name, logger)
		if err != nil {
			return err
		}

		err = awsTools.TerminateNodes(nodesToRotate, logger)
		if err != nil {
			return err
		}

		logger.Info("Sleeping 60 seconds for autoscaling group to balance...")
		time.Sleep(60 * time.Second)

		autoscalingGroupReady, err := awsTools.AutoScalingGroupReady(autoscalingGroup.Name, autoscalingGroup.DesiredCapacity, logger)
		if err != nil {
			return err
		}

		nodeHostnames, err := awsTools.GetNodeHostnames(autoscalingGroupReady.Instances)
		if err != nil {
			return err
		}

		newNodes := newNodes(nodeHostnames, autoscalingGroup.Nodes)
		if err != nil {
			return err
		}

		err = k8sTools.NodesReady(newNodes, clientset, logger)
		if err != nil {
			return err
		}

		logger.Info("Removing nodes from rotation list")
		autoscalingGroup.popNodes(nodesToRotate)

	}
	return nil
}

func workerNodeRotation(cluster *model.Cluster, autoscalingGroup *AutoscalingGroup, clientset *kubernetes.Clientset, logger *logrus.Entry) error {

	for len(autoscalingGroup.Nodes) > 0 {
		logger.Infof("The number of nodes in the ASG to be rotated is %d", len(autoscalingGroup.Nodes))

		var nodesToRotate []string

		if len(autoscalingGroup.Nodes) < int(cluster.MaxScaling) {
			nodesToRotate = autoscalingGroup.Nodes[:len(autoscalingGroup.Nodes)]
		} else {
			nodesToRotate = autoscalingGroup.Nodes[:int(cluster.MaxScaling)]
		}

		err := awsTools.DetachNodes(false, nodesToRotate, autoscalingGroup.Name, logger)
		if err != nil {
			return err
		}

		logger.Info("Sleeping 60 seconds for autoscaling group to balance...")
		time.Sleep(60 * time.Second)

		autoscalingGroupReady, err := awsTools.AutoScalingGroupReady(autoscalingGroup.Name, autoscalingGroup.DesiredCapacity, logger)
		if err != nil {
			return err
		}

		nodeHostnames, err := awsTools.GetNodeHostnames(autoscalingGroupReady.Instances)
		if err != nil {
			return err
		}

		newNodes := newNodes(nodeHostnames, autoscalingGroup.Nodes)
		if err != nil {
			return err
		}

		err = k8sTools.NodesReady(newNodes, clientset, logger)
		if err != nil {
			return err
		}

		err = DrainNodes(nodesToRotate, 10, int(cluster.EvictGracePeriod), clientset)
		if err != nil {
			return err
		}

		err = awsTools.TerminateNodes(nodesToRotate, logger)
		if err != nil {
			return err
		}

		err = k8sTools.DeleteClusterNodes(nodesToRotate, clientset)
		if err != nil {
			return err
		}

		logger.Info("Removing nodes from rotation list")
		autoscalingGroup.popNodes(nodesToRotate)

		if len(autoscalingGroup.Nodes) > 0 {
			logger.Infof("Waiting for %d seconds before next node rotation", cluster.WaitBetweenRotations)
			time.Sleep(time.Duration(cluster.WaitBetweenRotations) * time.Second)
		}
	}

	return nil
}
