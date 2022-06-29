package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/mattermost/rotator/model"
	rotator "github.com/mattermost/rotator/rotator"
)

// Register registers the API endpoints on the given router.
func Register(rootRouter *mux.Router, context *Context) {
	apiRouter := rootRouter.PathPrefix("/api").Subrouter()

	initCluster(apiRouter, context)
}

// initCluster registers RDS cluster endpoints on the given router.
func initCluster(apiRouter *mux.Router, context *Context) {
	addContext := func(handler contextHandlerFunc) *contextHandler {
		return newContextHandler(context, handler)
	}

	clustersRouter := apiRouter.PathPrefix("/rotate").Subrouter()
	clustersRouter.Handle("", addContext(handleRotateCluster)).Methods("POST")

	nodeRouter := apiRouter.PathPrefix("/drain").Subrouter()
	nodeRouter.Handle("", addContext(handleDrainNode)).Methods("POST")

}

// handleRotateCluster responds to POST /api/rotate, beginning the process of rotating a k8s cluster.
// sample body:
// {
//     "clusterID": "12345678",
//     "maxScaling": 2,
//     "rotateMasters":  true,
//     "rotateWorkers": true,
//     "maxDrainRetries": 10,
//     "EvictGracePeriod": 60,
//     "WaitBetweenRotations": 60,
//     "WaitBetweenDrains": 60,
// }
func handleRotateCluster(c *Context, w http.ResponseWriter, r *http.Request) {

	rotateClusterRequest, err := model.NewRotateClusterRequestFromReader(r.Body)
	if err != nil {
		c.Logger.WithError(err).Error("failed to decode request")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	cluster := model.Cluster{
		ClusterID:               rotateClusterRequest.ClusterID,
		MaxScaling:              rotateClusterRequest.MaxScaling,
		RotateMasters:           rotateClusterRequest.RotateMasters,
		RotateWorkers:           rotateClusterRequest.RotateWorkers,
		MaxDrainRetries:         rotateClusterRequest.MaxDrainRetries,
		EvictGracePeriod:        rotateClusterRequest.EvictGracePeriod,
		WaitBetweenRotations:    rotateClusterRequest.WaitBetweenRotations,
		WaitBetweenDrains:       rotateClusterRequest.WaitBetweenDrains,
		WaitBetweenPodEvictions: rotateClusterRequest.WaitBetweenPodEvictions,
	}

	rotatorMetada := rotator.RotatorMetadata{}

	go rotator.InitRotateCluster(&cluster, &rotatorMetada, c.Logger.WithField("cluster", cluster.ClusterID))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	outputJSON(c, w, cluster)
}

func handleDrainNode(c *Context, w http.ResponseWriter, r *http.Request) {

	drainNodeRequest, err := model.NewDrainNodeRequestFromReader(r.Body)
	if err != nil {
		c.Logger.WithError(err).Error("failed to decode request")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	node := model.NodeDrain{
		NodeName:                drainNodeRequest.NodeName,
		GracePeriod:             drainNodeRequest.GracePeriod,
		MaxDrainRetries:         drainNodeRequest.MaxDrainRetries,
		WaitBetweenPodEvictions: drainNodeRequest.WaitBetweenPodEvictions,
		DetachNode:              drainNodeRequest.DetachNode,
		TerminateNode:           drainNodeRequest.TerminateNode,
		ClusterID:               drainNodeRequest.ClusterID,
	}

	go rotator.InitDrainNode(&node, c.Logger.WithField("node", node.NodeName))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	outputJSON(c, w, node)
}
