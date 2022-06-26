// Package handlers manages the different versions of the API.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/illyasch/saga-service/pkg/business/saga"
	"github.com/illyasch/saga-service/pkg/data/database"
	"github.com/jmoiron/sqlx"
)

// APIConfig contains all the mandatory systems required by handlers.
type APIConfig struct {
	Log  *zap.SugaredLogger
	DB   *sqlx.DB
	Saga saga.Saga
}

type errorResponse struct {
	Error string `json:"error"`
}

// Router constructs a http.Handler with all application routes defined.
func (cfg APIConfig) Router() http.Handler {
	router := mux.NewRouter()
	router.HandleFunc("/start", cfg.handleStart()).Methods(http.MethodPost)
	router.HandleFunc("/readiness", cfg.handleReadiness).Methods(http.MethodGet)
	router.HandleFunc("/liveness", cfg.handleLiveness).Methods(http.MethodGet)

	return router
}

// handleStart handler starts a saga with a given id.
func (cfg APIConfig) handleStart() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var sagaUUID uuid.UUID

		sagaID := r.FormValue("saga_id")
		sagaUUID, err = uuid.Parse(sagaID)

		if err != nil {
			cfg.respond(w, http.StatusBadRequest, errorResponse{Error: "input saga id is incorrect"})
			cfg.Log.Errorw("saga", "ERROR", fmt.Errorf("validation saga id(%s): %w", sagaID, err))
			return
		}

		if err := cfg.Saga.Start(r.Context(), sagaUUID); err != nil {
			cfg.respond(w, http.StatusInternalServerError, errorResponse{
				Error: http.StatusText(http.StatusInternalServerError),
			})
			cfg.Log.Errorw("saga", "ERROR", fmt.Errorf("saga start: %w", err))
			return
		}

		cfg.respond(w, http.StatusOK, nil)
		cfg.Log.Infow("saga", "statusCode", http.StatusOK, "method", r.Method, "path", r.URL.Path, "remoteaddr", r.RemoteAddr)
	}
}

// handleReadiness checks if the database is ready and if not will return a 500 status if it's not.
func (cfg APIConfig) handleReadiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()

	status := "ok"
	statusCode := http.StatusOK
	if err := database.StatusCheck(ctx, cfg.DB); err != nil {
		status = "db not ready"
		statusCode = http.StatusInternalServerError
		cfg.Log.Errorw("readiness", "ERROR", fmt.Errorf("status check: %w", err))
	}

	data := struct {
		Status string `json:"status"`
	}{
		Status: status,
	}

	cfg.respond(w, statusCode, data)
	cfg.Log.Infow("readiness", "statusCode", statusCode, "method", r.Method, "path", r.URL.Path, "remoteaddr", r.RemoteAddr)
}

// handleLiveness returns simple status info if the service is alive. If the
// app is deployed to a Kubernetes cluster, it will also return pod, node, and
// namespace details via the Downward API. The Kubernetes environment variables
// need to be set within your Pod/Deployment manifest.
func (cfg APIConfig) handleLiveness(w http.ResponseWriter, r *http.Request) {
	host, err := os.Hostname()
	if err != nil {
		host = "unavailable"
	}

	data := struct {
		Status    string `json:"status,omitempty"`
		Build     string `json:"build,omitempty"`
		Host      string `json:"host,omitempty"`
		Pod       string `json:"pod,omitempty"`
		PodIP     string `json:"podIP,omitempty"`
		Node      string `json:"node,omitempty"`
		Namespace string `json:"namespace,omitempty"`
	}{
		Status:    "up",
		Host:      host,
		Pod:       os.Getenv("KUBERNETES_PODNAME"),
		PodIP:     os.Getenv("KUBERNETES_NAMESPACE_POD_IP"),
		Node:      os.Getenv("KUBERNETES_NODENAME"),
		Namespace: os.Getenv("KUBERNETES_NAMESPACE"),
	}

	statusCode := http.StatusOK
	cfg.respond(w, statusCode, data)
	cfg.Log.Infow("liveness", "statusCode", statusCode, "method", r.Method, "path", r.URL.Path, "remoteaddr", r.RemoteAddr)
}

func (cfg APIConfig) respond(w http.ResponseWriter, statusCode int, data any) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		cfg.Log.Errorw("respond", "ERROR", fmt.Errorf("json marshal: %w", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if _, err := w.Write(jsonData); err != nil {
		cfg.Log.Errorw("respond", "ERROR", fmt.Errorf("write output: %w", err))
		return
	}
}
