package rotator

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/pkg/errors"
)

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

func DetachNodes(decrement bool, nodesToDetach []string, autoscalingGroupName string) error {
	asgSvc := autoscaling.New(session.New())

	for _, node := range nodesToDetach {
		instanceID, err := GetInstanceID(node)
		if err != nil {
			return errors.Wrapf(err, "Failed to detach and delete node %s", node)
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

func TerminateNodes(nodesToTerminate []string) error {
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
			for _, tag := range asg.Tags {
				if *tag.Key == "KubernetesCluster" && *tag.Value == fmt.Sprintf("%s-kops.k8s.local", clusterID) || *tag.Key == "Name" && strings.Contains(*tag.Value, clusterID) {
					autoscalingGroups = append(autoscalingGroups, asg)
					break
				}
			}
		}

		if resp.NextToken == nil || *resp.NextToken == "" {
			break
		}
		nextToken = resp.NextToken
	}

	return autoscalingGroups, nil
}

func (autoscalingGroup *AutoscalingGroup) AutoScalingGroupReady() (*autoscaling.Group, error) {
	svc := autoscaling.New(session.New())
	timeout := 300
	logger.Infof("Waiting up to %d seconds for load balancer %s to become ready...", timeout, autoscalingGroup.Name)

	timer := time.NewTimer(time.Duration(timeout) * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			return nil, errors.New("timed out waiting for load balancer to become ready")
		default:
			resp, err := svc.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
				AutoScalingGroupNames: []*string{
					aws.String(autoscalingGroup.Name),
				},
			})
			if err != nil {
				return nil, errors.Wrapf(err, "Failed to describe the autoscaling group %s", autoscalingGroup.Name)
			}

			if int64(len(resp.AutoScalingGroups[0].Instances)) == autoscalingGroup.DesiredCapacity {
				return resp.AutoScalingGroups[0], nil
			}

			logger.Info("AutoscalingGroup not updated with new instances, waiting...")
			time.Sleep(5 * time.Second)
		}
	}
}
