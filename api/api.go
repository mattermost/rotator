package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/mattermost/node-rotator/model"
	rotator "github.com/mattermost/node-rotator/rotator"
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
}

// handleRotateCluster responds to POST /api/rotate, beginning the process of creating a new RDS Aurora cluster.
// sample body:
// {
//     "clusterID": "12345678",
//     "maxScaling": 2,
//     "rotateMasters":  true,
//     "rotateWorkers": true,
//     "maxDrainRetries": 10,
//     "EvictGracePeriod": 60,
//     "WaitBetweenRotations": 60,
// }
func handleRotateCluster(c *Context, w http.ResponseWriter, r *http.Request) {
	rotateClusterRequest, err := model.NewRotateClusterRequestFromReader(r.Body)
	if err != nil {
		c.Logger.WithError(err).Error("failed to decode request")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	cluster := model.Cluster{
		ClusterID:            rotateClusterRequest.ClusterID,
		MaxScaling:           rotateClusterRequest.MaxScaling,
		RotateMasters:        rotateClusterRequest.RotateMasters,
		RotateWorkers:        rotateClusterRequest.RotateWorkers,
		MaxDrainRetries:      rotateClusterRequest.MaxDrainRetries,
		EvictGracePeriod:     rotateClusterRequest.EvictGracePeriod,
		WaitBetweenRotations: rotateClusterRequest.WaitBetweenRotations,
	}

	go rotator.InitRotateCluster(&cluster)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	outputJSON(c, w, cluster)
}
