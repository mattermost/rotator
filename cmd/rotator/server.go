package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/mux"
	"github.com/mattermost/node-rotator/api"
	"github.com/mattermost/node-rotator/model"
	"github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

var instanceID string

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Runs the node rotaror API server",
	RunE:  serverCmdF,
}

func init() {
	instanceID = model.NewID()

	serverCmd.PersistentFlags().String("listen", ":8079", "The interface and port on which to listen.")
	serverCmd.PersistentFlags().Bool("debug", false, "Whether to output debug logs.")
}

func serverCmdF(command *cobra.Command, args []string) error {
	command.SilenceUsage = true

	debug, _ := command.Flags().GetBool("debug")
	if debug {
		logger.SetLevel(logrus.DebugLevel)
	}

	logger := logger.WithField("instance", instanceID)
	logger.Info("Starting Mattermost Node Rotator Server")

	router := mux.NewRouter()

	api.Register(router, &api.Context{
		Logger: logger,
	})

	listen, _ := command.Flags().GetString("listen")
	srv := &http.Server{
		Addr:           listen,
		Handler:        router,
		ReadTimeout:    180 * time.Second,
		WriteTimeout:   180 * time.Second,
		IdleTimeout:    time.Second * 180,
		MaxHeaderBytes: 1 << 20,
		ErrorLog:       log.New(&logrusWriter{logger}, "", 0),
	}

	go func() {
		logger.WithField("addr", srv.Addr).Info("Listening")
		err := srv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Error("Failed to listen and serve")
		}
	}()

	c := make(chan os.Signal, 1)
	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/) will not be caught.
	signal.Notify(c, os.Interrupt)

	// Block until we receive our signal.
	<-c
	logger.Info("Shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	srv.Shutdown(ctx)

	return nil
}
