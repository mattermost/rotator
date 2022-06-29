package model

import (
	"encoding/json"
	"io"
)

// NodeDrain represents a K8s node to be drained.
type NodeDrain struct {
	NodeName                string
	GracePeriod             int
	WaitBetweenPodEvictions int
	MaxDrainRetries         int
}

// NodeFromReader decodes a json-encoded node from the given io.Reader.
func NodeDrainFromReader(reader io.Reader) (*NodeDrain, error) {
	node := NodeDrain{}
	decoder := json.NewDecoder(reader)
	err := decoder.Decode(&node)
	if err != nil && err != io.EOF {
		return nil, err
	}

	return &node, nil
}
