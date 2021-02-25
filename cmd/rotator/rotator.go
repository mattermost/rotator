package main

import (
	"encoding/json"
	"os"

	"github.com/mattermost/node-rotator/model"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func init() {
	clusterCmd.PersistentFlags().String("server", "http://localhost:8079", "The Node Rotator server whose API will be queried.")

	rotatorCmd.Flags().String("cluster", "", "the cluster ID of the cluster to go through node rotation")
	rotatorCmd.Flags().Int("max-scaling", 1, "the max number of nodes rotating in parallel")
	rotatorCmd.Flags().Bool("rotate-masters", false, "if disabled, master nodes will not be rotated")
	rotatorCmd.Flags().Bool("rotate-workers", false, "if disabled, worker nodes will not be rotated")
	rotatorCmd.Flags().Int("max-drain-retries", 10, "the max number of retries when drain fails")
	rotatorCmd.Flags().Int("evict-grace-period", 60, "the pod eviction grace period")
	rotatorCmd.Flags().Int("wait-between-rotations", 60, "the time in seconds between each node rotation")
	rotatorCmd.Flags().Int("wait-between-drains", 60, "the time in seconds between each node drain")

	clusterCmd.AddCommand(rotatorCmd)
}

var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Rotate cluster nodes by the node rotator server.",
}

// TODO: Add node handling capabilities
var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Handle node changees, drain, detach, terminate, etc.",
}

var rotatorCmd = &cobra.Command{
	Use:   "rotate",
	Short: "Rotate nodes of a k8s cluster.",
	RunE: func(command *cobra.Command, args []string) error {
		command.SilenceUsage = true
		serverAddress, _ := command.Flags().GetString("server")
		client := model.NewClient(serverAddress)

		clusterID, _ := command.Flags().GetString("cluster")
		maxScaling, _ := command.Flags().GetInt("max-scaling")
		rotateMasters, _ := command.Flags().GetBool("rotate-masters")
		rotateWorkers, _ := command.Flags().GetBool("rotate-workers")
		maxDrainRetries, _ := command.Flags().GetInt("max-drain-retries")
		evictGracePeriod, _ := command.Flags().GetInt("evict-grace-period")
		waitBetweenRotations, _ := command.Flags().GetInt("wait-between-rotations")
		waitBetweenDrains, _ := command.Flags().GetInt("wait-between-drains")

		rotator, err := client.RotateCluster(&model.RotateClusterRequest{
			ClusterID:            clusterID,
			MaxScaling:           maxScaling,
			RotateMasters:        rotateMasters,
			RotateWorkers:        rotateWorkers,
			MaxDrainRetries:      maxDrainRetries,
			EvictGracePeriod:     evictGracePeriod,
			WaitBetweenRotations: waitBetweenRotations,
			WaitBetweenDrains:    waitBetweenDrains,
		})
		if err != nil {
			return errors.Wrap(err, "failed to rotate nodes of the k8s cluster")
		}
		err = printJSON(rotator)
		if err != nil {
			return err
		}

		return nil
	},
}

func printJSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "    ")
	return encoder.Encode(data)
}
