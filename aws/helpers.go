package aws

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// GetNodeHostnames returns the hostnames of the autoscaling group nodes.
func GetNodeHostnames(autoscalingGroupNodes []*autoscaling.Instance) ([]string, error) {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	svc := ec2.New(sess)
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

// GetInstanceID returns the instance ID of a node.
func GetInstanceID(nodeName string, logger *logrus.Entry) (string, error) {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	svc := ec2.New(sess)
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

	if resp.Reservations == nil {
		logger.Warnf("Instance %s not found, assuming that instance was already deleted", nodeName)
		return "", nil
	}

	return *resp.Reservations[0].Instances[0].InstanceId, nil
}

// DetachNodes detaches nodes from an autoscaling group.
func DetachNodes(decrement bool, nodesToDetach []string, autoscalingGroupName string, logger *logrus.Entry) error {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	asgSvc := autoscaling.New(sess)

	for _, node := range nodesToDetach {
		instanceID, err := GetInstanceID(node, logger)
		if err != nil {
			return errors.Wrapf(err, "Failed to detach node %s", node)
		}

		if instanceID == "" {
			logger.Infof("Instance %s does not exist. No detachment required", node)
			return nil
		}

		nodeInGroup, err := nodeInAutoscalingGroup(autoscalingGroupName, instanceID)
		if err != nil {
			return errors.Wrapf(err, "Failed to check if instance is member of the ASG")
		}
		if nodeInGroup {
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

	}

	return nil
}

// TerminateNodes terminates a slice of nodes.
func TerminateNodes(nodesToTerminate []string, logger *logrus.Entry) error {
	logger.Infof("Terminating %d nodes", len(nodesToTerminate))
	for _, node := range nodesToTerminate {
		var instanceID string
		var err error
		logger.Infof(node)
		if matchesPatternPrivateDNS(node) {
			instanceID, err = GetInstanceID(node, logger)
		} else if matchesPatternID(node) {
			instanceID = node
		} else {
			instanceID = ""
			logger.Infof("Node %s is not a valid input", node)
		}

		if err != nil {
			return errors.Wrapf(err, "Failed to detach and delete node %s", node)
		}

		if instanceID == "" {
			logger.Infof("Instance %s does not exist. No termination required", node)
			return nil
		}

		logger.Infof("Terminating instance %s", instanceID)
		sess := session.Must(session.NewSessionWithOptions(session.Options{
			SharedConfigState: session.SharedConfigEnable,
		}))
		ec2Svc := ec2.New(sess)

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

// GetAutoscalingGroups gets all the autoscaling groups that their names contain the cluster ID passed.
func GetAutoscalingGroups(clusterID string) ([]*autoscaling.Group, error) {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	svc := autoscaling.New(sess)
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

// AutoScalingGroupReady gets an AutoscalingGroup object and checks that autoscaling group is in ready state.
func AutoScalingGroupReady(autoscalingGroupName string, desiredCapacity int, logger *logrus.Entry) (*autoscaling.Group, error) {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	svc := autoscaling.New(sess)
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

			if len(resp.AutoScalingGroups[0].Instances) == desiredCapacity {
				return resp.AutoScalingGroups[0], nil
			}

			logger.Info("AutoscalingGroup not updated with new instances, waiting...")
			time.Sleep(5 * time.Second)
		}
	}
}

func NodeInAutoscalingGroup(autoscalingGroupName, instanceID string) (bool, error) {
	return nodeInAutoscalingGroup(autoscalingGroupName, instanceID)
}

// nodeInAutoscalingGroup checks if an instance is member of an Autoscaling Group.
func nodeInAutoscalingGroup(autoscalingGroupName, instanceID string) (bool, error) {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	svc := autoscaling.New(sess)
	resp, err := svc.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []*string{
			aws.String(autoscalingGroupName),
		},
	})
	if err != nil {
		return false, err
	}

	for _, instance := range resp.AutoScalingGroups[0].Instances {
		if *instance.InstanceId == instanceID {
			return true, nil
		}
	}
	return false, nil
}

func GetInstanceIDByPrivateIP(privateIP string) (string, error) {
	// Create a new AWS session
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	// Create an EC2 service client
	ec2Svc := ec2.New(sess)

	// Describe instances with the given private IP
	input := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("private-ip-address"),
				Values: []*string{aws.String(privateIP)},
			},
		},
	}

	result, err := ec2Svc.DescribeInstances(input)
	if err != nil {
		return "", fmt.Errorf("failed to describe instances: %v", err)
	}

	// Check if any instances were found
	if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
		return "", fmt.Errorf("no instances found with the provided private IP: %s", privateIP)
	}

	// Retrieve the instance ID
	instanceID := aws.StringValue(result.Reservations[0].Instances[0].InstanceId)

	return instanceID, nil
}

func ExtractPrivateIP(input string) (string, error) {
	regexPattern := `ip-(\d{1,3}-\d{1,3}-\d{1,3}-\d{1,3})\.ec2\.internal`
	regex := regexp.MustCompile(regexPattern)
	matches := regex.FindStringSubmatch(input)

	if len(matches) != 2 {
		return "", fmt.Errorf("failed to extract private IP from input string")
	}

	privateIP := strings.ReplaceAll(matches[1], "-", ".")
	return privateIP, nil
}

func matchesPatternPrivateDNS(input string) bool {
	// Define the regular expression pattern
	pattern := `^ip-\d{2,3}-\d{1,3}-\d{1,3}-\d{1,3}\.ec2\.internal$`

	// Compile the regular expression
	re := regexp.MustCompile(pattern)

	// Use the MatchString method to check if the input matches the pattern
	return re.MatchString(input)
}

func matchesPatternID(input string) bool {
	// Define the regular expression pattern for ID pattern
	pattern := `^i-[0-9a-f]{8,17}$`

	// Compile the regular expression
	re := regexp.MustCompile(pattern)

	// Use the MatchString method to check if the input matches the pattern
	return re.MatchString(input)
}
