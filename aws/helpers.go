package aws

import (
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// GetNodeHostnames returns the hostnames of an autoscaling group nodes
func GetNodeHostnames(autoscalingGroupNodes []*autoscaling.Instance) ([]string, error) {
	svc := ec2.New(session.New())
	var instanceHostnames []string
	for _, node := range autoscalingGroupNodes {
		resp, err := svc.DescribeInstances(&ec2.DescribeInstancesInput{
			InstanceIds: []*string{aws.String(*node.InstanceId)},
		})
		if err != nil {
			return nil, errors.Wrap(err, "Failed to describe ec2 instance")
		}
		instanceHostnames = append(instanceHostnames, *resp.Reservations[0].Instances[0].PrivateDnsName)
	}
	return instanceHostnames, nil
}

// GetInstanceID returns the instance ID of a nodename
func GetInstanceID(nodeName string) (string, error) {
	svc := ec2.New(session.New())
	resp, err := svc.DescribeInstances(&ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("private-dns-name"),
				Values: []*string{aws.String(nodeName)},
			},
		},
	})
	if err != nil {
		return "", errors.Wrap(err, "Failed to describe ec2 instance")
	}
	return *resp.Reservations[0].Instances[0].InstanceId, nil
}

// DetachNodes detaches nodes from an autoscaling group
func DetachNodes(decrement bool, nodesToDetach []string, autoscalingGroupName string, logger *logrus.Entry) error {
	asgSvc := autoscaling.New(session.New())

	for _, node := range nodesToDetach {
		instanceID, err := GetInstanceID(node)
		if err != nil {
			return errors.Wrapf(err, "Failed to detach node %s", node)
		}

		logger.Infof("Detaching instance %s", instanceID)
		_, err = asgSvc.DetachInstances(&autoscaling.DetachInstancesInput{
			AutoScalingGroupName: aws.String(autoscalingGroupName),
			InstanceIds: []*string{
				aws.String(instanceID),
			},
			ShouldDecrementDesiredCapacity: aws.Bool(decrement),
		})
		if err != nil {
			return errors.Wrapf(err, "Failed to detach instance %s", instanceID)
		}
	}

	return nil
}

// TerminateNodes terminates list of nodes
func TerminateNodes(nodesToTerminate []string, logger *logrus.Entry) error {
	logger.Infof("Terminating %d nodes", len(nodesToTerminate))

	for _, node := range nodesToTerminate {
		instanceID, err := GetInstanceID(node)
		if err != nil {
			return errors.Wrapf(err, "Failed to detach and delete node %s", node)
		}

		logger.Infof("Terminating instance %s", instanceID)
		ec2Svc := ec2.New(session.New())

		_, err = ec2Svc.TerminateInstances(&ec2.TerminateInstancesInput{
			InstanceIds: []*string{
				aws.String(instanceID),
			},
		})
		if err != nil {
			return errors.Wrapf(err, "Failed to delete instance %s", instanceID)
		}
	}

	return nil
}

// GetAutoscalingGroups gets all the autoscaling groups that their names contain the cluster ID passed
func GetAutoscalingGroups(clusterID string) ([]*autoscaling.Group, error) {
	svc := autoscaling.New(session.New())

	var autoscalingGroups []*autoscaling.Group
	var nextToken *string
	for {
		resp, err := svc.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
			NextToken: nextToken,
		})
		if err != nil {
			return nil, errors.Wrap(err, "Failed to describe autoscaling groups")
		}
		for _, asg := range resp.AutoScalingGroups {
			if strings.Contains(*asg.AutoScalingGroupName, clusterID) {
				autoscalingGroups = append(autoscalingGroups, asg)
			}
		}

		if resp.NextToken == nil || *resp.NextToken == "" {
			break
		}
		nextToken = resp.NextToken
	}

	return autoscalingGroups, nil
}

// AutoScalingGroupReady gets an AutoscalingGroup object and checks that autoscaling group is in ready state
func AutoScalingGroupReady(autoscalingGroupName string, desiredCapacity int64, logger *logrus.Entry) (*autoscaling.Group, error) {
	svc := autoscaling.New(session.New())
	timeout := 300
	logger.Infof("Waiting up to %d seconds for autoscaling group %s to become ready...", timeout, autoscalingGroupName)

	timer := time.NewTimer(time.Duration(timeout) * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			return nil, errors.New("timed out waiting for autoscaling group to become ready")
		default:
			resp, err := svc.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
				AutoScalingGroupNames: []*string{
					aws.String(autoscalingGroupName),
				},
			})
			if err != nil {
				return nil, errors.Wrapf(err, "Failed to describe the autoscaling group %s", autoscalingGroupName)
			}

			if int64(len(resp.AutoScalingGroups[0].Instances)) == desiredCapacity {
				return resp.AutoScalingGroups[0], nil
			}

			logger.Info("AutoscalingGroup not updated with new instances, waiting...")
			time.Sleep(5 * time.Second)
		}
	}
}
