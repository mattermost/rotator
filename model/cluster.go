package model

import (
	"encoding/json"
	"io"

	"k8s.io/client-go/kubernetes"

	cloudModel "github.com/mattermost/mattermost-cloud/model"
)

// Cluster represents a K8s cluster.
type Cluster struct {
	ClusterID            string
	MaxScaling           int64
	RotateMasters        bool
	RotateWorkers        bool
	MaxDrainRetries      int64
	EvictGracePeriod     int64
	WaitBetweenRotations int64
	Store                string
	ClientSet            *kubernetes.Clientset
	Cluster              cloudModel.Cluster
}

// ClusterFromReader decodes a json-encoded cluster from the given io.Reader.
func ClusterFromReader(reader io.Reader) (*Cluster, error) {
	cluster := Cluster{}
	decoder := json.NewDecoder(reader)
	err := decoder.Decode(&cluster)
	if err != nil && err != io.EOF {
		return nil, err
	}

	return &cluster, nil
}
