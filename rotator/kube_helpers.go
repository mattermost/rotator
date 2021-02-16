package rotator

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func NodesReady(nodes []string) error {
	wait := 600
	logger.Infof("Waiting up to %d seconds for all nodes to become ready...", wait)
	for _, node := range nodes {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(wait)*time.Second)
		defer cancel()
		node, err := WaitForNodeRunning(ctx, node)
		if err != nil {
			return errors.Wrapf(err, "Node %s failed to get ready", node)
		}
	}
	logger.Info("All nodes in Ready state")

	return nil
}

// WaitForNodeRunning will poll a given kubernetes node at a regular interval for
// it to enter the 'Ready' state. If the node fails to become ready before
// the provided timeout then an error will be returned.
func WaitForNodeRunning(ctx context.Context, nodeName string) (*corev1.Node, error) {
	clientset, err := getClientSet()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get k8s clientset")
	}
	for {
		node, err := clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if err == nil {
			for _, condition := range node.Status.Conditions {
				if condition.Reason == "KubeletReady" && condition.Status == corev1.ConditionTrue {
					return node, nil
				} else if condition.Reason == "KubeletReady" && condition.Status == corev1.ConditionFalse {
					logger.Infof("Node %s found but not ready, waiting...", nodeName)
				}
			}
		}
		if err != nil && k8sErrors.IsNotFound(err) {
			logger.Infof("Node %s not found, waiting...", nodeName)
		}

		select {
		case <-ctx.Done():
			return nil, errors.Wrap(ctx.Err(), "timed out waiting for node to become ready")
		case <-time.After(20 * time.Second):
		}
	}
}

func DrainNodes(nodesToDrain []string, attempts, gracePeriod int) error {
	ctx := context.TODO()

	clientset, err := getClientSet()
	if err != nil {
		return errors.Wrap(err, "Failed to get k8s clientset")
	}

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
				DrainNodes([]string{nodeToDrain}, attempts-1, gracePeriod)
			} else {
				return errors.Wrapf(err, "Failed to drain node %s", nodeToDrain)
			}
		}
		logger.Infof("Node %s drained successfully", nodeToDrain)
	}

	return nil
}

func DeleteClusterNodes(nodes []string) error {
	ctx := context.TODO()
	clientset, err := getClientSet()
	if err != nil {
		return errors.Wrap(err, "Failed to get k8s clientset")
	}

	for _, node := range nodes {
		err = clientset.CoreV1().Nodes().Delete(ctx, node, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

// getClientSet gets the k8s clientset
func getClientSet() (*kubernetes.Clientset, error) {
	kubeconfig := filepath.Join(
		os.Getenv("HOME"), ".kube", "config",
	)

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return clientset, nil
}
