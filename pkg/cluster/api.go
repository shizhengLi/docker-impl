package cluster

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type APIServer struct {
	manager *ClusterManager
	server  *http.Server
	router  *mux.Router
}

type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func NewAPIServer(manager *ClusterManager) *APIServer {
	return &APIServer{
		manager: manager,
		router:  mux.NewRouter(),
	}
}

func (api *APIServer) Start() error {
	api.setupRoutes()

	addr := fmt.Sprintf("%s:%d", api.manager.Config.AdvertiseAddr, api.manager.Config.AdvertisePort)

	api.server = &http.Server{
		Addr:         addr,
		Handler:      api.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	logrus.Infof("Starting API server on %s", addr)

	go func() {
		if err := api.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Errorf("API server error: %v", err)
		}
	}()

	return nil
}

func (api *APIServer) Stop() error {
	if api.server != nil {
		return api.server.Close()
	}
	return nil
}

func (api *APIServer) setupRoutes() {
	// Cluster management
	api.router.HandleFunc("/cluster/info", api.handleClusterInfo).Methods("GET")
	api.router.HandleFunc("/cluster/join", api.handleClusterJoin).Methods("POST")
	api.router.HandleFunc("/cluster/leave", api.handleClusterLeave).Methods("POST")
	api.router.HandleFunc("/cluster/status", api.handleClusterStatus).Methods("GET")

	// Node management
	api.router.HandleFunc("/nodes", api.handleListNodes).Methods("GET")
	api.router.HandleFunc("/nodes", api.handleRegisterNode).Methods("POST")
	api.router.HandleFunc("/nodes/{nodeID}", api.handleGetNode).Methods("GET")
	api.router.HandleFunc("/nodes/{nodeID}", api.handleUpdateNode).Methods("PUT")
	api.router.HandleFunc("/nodes/{nodeID}", api.handleDeleteNode).Methods("DELETE")
	api.router.HandleFunc("/nodes/{nodeID}/drain", api.handleDrainNode).Methods("POST")
	api.router.HandleFunc("/nodes/{nodeID}/activate", api.handleActivateNode).Methods("POST")

	// Task management
	api.router.HandleFunc("/tasks", api.handleListTasks).Methods("GET")
	api.router.HandleFunc("/tasks", api.handleCreateTask).Methods("POST")
	api.router.HandleFunc("/tasks/{taskID}", api.handleGetTask).Methods("GET")
	api.router.HandleFunc("/tasks/{taskID}", api.handleUpdateTask).Methods("PUT")
	api.router.HandleFunc("/tasks/{taskID}", api.handleDeleteTask).Methods("DELETE")
	api.router.HandleFunc("/tasks/{taskID}/start", api.handleStartTask).Methods("POST")
	api.router.HandleFunc("/tasks/{taskID}/stop", api.handleStopTask).Methods("POST")
	api.router.HandleFunc("/tasks/{taskID}/restart", api.handleRestartTask).Methods("POST")

	// Service management (placeholder for future)
	api.router.HandleFunc("/services", api.handleListServices).Methods("GET")
	api.router.HandleFunc("/services", api.handleCreateService).Methods("POST")

	// Health check
	api.router.HandleFunc("/health", api.handleHealthCheck).Methods("GET")

	// Middleware
	api.router.Use(api.loggingMiddleware)
	api.router.Use(api.authMiddleware)
}

func (api *APIServer) handleClusterInfo(w http.ResponseWriter, r *http.Request) {
	info := api.manager.GetClusterInfo()
	api.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    info,
	})
}

func (api *APIServer) handleClusterJoin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		JoinAddr  string `json:"join_addr"`
		JoinToken string `json:"join_token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := api.manager.JoinCluster(req.JoinAddr, req.JoinToken); err != nil {
		api.writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	api.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Message: "Successfully joined cluster",
	})
}

func (api *APIServer) handleClusterLeave(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Force bool `json:"force"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := api.manager.LeaveCluster(req.Force); err != nil {
		api.writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	api.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Message: "Successfully left cluster",
	})
}

func (api *APIServer) handleClusterStatus(w http.ResponseWriter, r *http.Request) {
	status := api.manager.GetStatus()
	api.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    status,
	})
}

func (api *APIServer) handleListNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := api.manager.NodeManager.ListNodes()
	if err != nil {
		api.writeErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	api.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    nodes,
	})
}

func (api *APIServer) handleRegisterNode(w http.ResponseWriter, r *http.Request) {
	var node Node
	if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
		api.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := api.manager.NodeManager.RegisterNode(&node); err != nil {
		api.writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	api.writeJSONResponse(w, http.StatusCreated, APIResponse{
		Success: true,
		Message: "Node registered successfully",
		Data:    node,
	})
}

func (api *APIServer) handleGetNode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	nodeID := vars["nodeID"]

	node, err := api.manager.NodeManager.GetNode(nodeID)
	if err != nil {
		api.writeErrorResponse(w, http.StatusNotFound, err.Error())
		return
	}

	api.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    node,
	})
}

func (api *APIServer) handleUpdateNode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	nodeID := vars["nodeID"]

	var updates Node
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		api.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Update node resources if provided
	if updates.Resources.CPU > 0 {
		if err := api.manager.NodeManager.UpdateNodeResources(nodeID, updates.Resources); err != nil {
			api.writeErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	api.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Message: "Node updated successfully",
	})
}

func (api *APIServer) handleDeleteNode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	nodeID := vars["nodeID"]

	if err := api.manager.NodeManager.UnregisterNode(nodeID); err != nil {
		api.writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	api.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Message: "Node deleted successfully",
	})
}

func (api *APIServer) handleDrainNode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	nodeID := vars["nodeID"]

	if err := api.manager.NodeManager.DrainNode(nodeID); err != nil {
		api.writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	api.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Message: "Node drained successfully",
	})
}

func (api *APIServer) handleActivateNode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	nodeID := vars["nodeID"]

	if err := api.manager.NodeManager.ActivateNode(nodeID); err != nil {
		api.writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	api.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Message: "Node activated successfully",
	})
}

func (api *APIServer) handleListTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := api.manager.TaskManager.ListTasks()
	if err != nil {
		api.writeErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	api.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    tasks,
	})
}

func (api *APIServer) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var task Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		api.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if task.ID == "" {
		task.ID = generateTaskID()
	}

	if err := api.manager.TaskManager.CreateTask(&task); err != nil {
		api.writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	api.writeJSONResponse(w, http.StatusCreated, APIResponse{
		Success: true,
		Message: "Task created successfully",
		Data:    task,
	})
}

func (api *APIServer) handleGetTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["taskID"]

	task, err := api.manager.TaskManager.GetTask(taskID)
	if err != nil {
		api.writeErrorResponse(w, http.StatusNotFound, err.Error())
		return
	}

	api.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    task,
	})
}

func (api *APIServer) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["taskID"]

	var updates Task
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		api.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := api.manager.TaskManager.UpdateTask(taskID, &updates); err != nil {
		api.writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	api.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Message: "Task updated successfully",
	})
}

func (api *APIServer) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["taskID"]

	if err := api.manager.TaskManager.RemoveTask(taskID); err != nil {
		api.writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	api.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Message: "Task deleted successfully",
	})
}

func (api *APIServer) handleStartTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["taskID"]

	if err := api.manager.TaskManager.StartTask(taskID); err != nil {
		api.writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	api.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Message: "Task started successfully",
	})
}

func (api *APIServer) handleStopTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["taskID"]

	if err := api.manager.TaskManager.StopTask(taskID); err != nil {
		api.writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	api.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Message: "Task stopped successfully",
	})
}

func (api *APIServer) handleRestartTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["taskID"]

	if err := api.manager.TaskManager.RestartTask(taskID); err != nil {
		api.writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	api.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Message: "Task restarted successfully",
	})
}

func (api *APIServer) handleListServices(w http.ResponseWriter, r *http.Request) {
	// Placeholder for service management
	api.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    []string{}, // Empty list for now
	})
}

func (api *APIServer) handleCreateService(w http.ResponseWriter, r *http.Request) {
	// Placeholder for service management
	api.writeErrorResponse(w, http.StatusNotImplemented, "Service management not implemented")
}

func (api *APIServer) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status": "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"version": "1.0.0",
	}

	// Add cluster status
	if api.manager.GetStatus() != nil {
		health["cluster"] = api.manager.GetStatus()
	}

	api.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    health,
	})
}

func (api *APIServer) writeJSONResponse(w http.ResponseWriter, statusCode int, response APIResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}

func (api *APIServer) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	api.writeJSONResponse(w, statusCode, APIResponse{
		Success: false,
		Error:   message,
	})
}

func (api *APIServer) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		logrus.WithFields(logrus.Fields{
			"method": r.Method,
			"path":   r.URL.Path,
			"remote": r.RemoteAddr,
		}).Info("API request")

		next.ServeHTTP(w, r)

		logrus.WithFields(logrus.Fields{
			"method": r.Method,
			"path":   r.URL.Path,
			"duration": time.Since(start),
		}).Info("API request completed")
	})
}

func (api *APIServer) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simple token-based authentication
		token := r.Header.Get("X-Cluster-Token")
		if token == "" {
			token = r.URL.Query().Get("token")
		}

		if token != api.manager.Config.JoinToken && api.manager.Config.JoinToken != "" {
			api.writeErrorResponse(w, http.StatusUnauthorized, "Invalid or missing authentication token")
			return
		}

		next.ServeHTTP(w, r)
	})
}