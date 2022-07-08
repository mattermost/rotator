package model

import (
	"encoding/json"
	"io"

	"github.com/pkg/errors"
)

// DrainNodeRequest specifies the parameters for a new cluster node drain.
type DrainNodeRequest struct {
	NodeName                string `json:"nodeName,omitempty"`
	GracePeriod             int    `json:"gracePeriod,omitempty"`
	MaxDrainRetries         int    `json:"maxDrainRetries,omitempty"`
	WaitBetweenPodEvictions int    `json:"waitBetweenPodEvictions,omitempty"`
	DetachNode              bool   `json:"detachNode,omitempty"`
	TerminateNode           bool   `json:"terminateNode,omitempty"`
	ClusterID               string `json:"clusterID,omitempty"`
}

// NewDrainNodeRequestFromReader decodes the request and returns after validation and setting the defaults.
func NewDrainNodeRequestFromReader(reader io.Reader) (*DrainNodeRequest, error) {
	var drainNodeRequest DrainNodeRequest
	err := json.NewDecoder(reader).Decode(&drainNodeRequest)
	if err != nil && err != io.EOF {
		return nil, errors.Wrap(err, "failed to decode node drain request")
	}

	err = drainNodeRequest.Validate()
	if err != nil {
		return nil, errors.Wrap(err, "drain cluster request failed validation")
	}
	drainNodeRequest.SetDefaults()

	return &drainNodeRequest, nil
}

// Validate validates the values of a node drain request.
func (request *DrainNodeRequest) Validate() error {
	if request.NodeName == "" {
		return errors.New("Node name cannot be empty")
	}
	return nil
}

// SetDefaults sets the default values for a node drain request.
func (request *DrainNodeRequest) SetDefaults() {}
