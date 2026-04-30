package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/amoondra1989/sandbox-envs/internal/sandbox"
	"github.com/amoondra1989/sandbox-envs/pkg/sandboxclient"
)

// Server exposes sandbox operations over HTTP.
type Server struct {
	Backend sandbox.Sandbox
}

// NewServer registers REST routes on a ServeMux (Go 1.22+ patterns).
func NewServer(backend sandbox.Sandbox) http.Handler {
	s := &Server{Backend: backend}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/sandboxes", s.handleCreate)
	mux.HandleFunc("DELETE /v1/sandboxes/{id}", s.handleDestroy)
	mux.HandleFunc("POST /v1/sandboxes/{id}/exec", s.handleExec)
	mux.HandleFunc("GET /v1/sandboxes/{id}/file", s.handleReadFile)
	mux.HandleFunc("PUT /v1/sandboxes/{id}/file", s.handleWriteFile)
	mux.HandleFunc("GET /v1/sandboxes/{id}/health", s.handleHealth)
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var opts sandboxclient.CreateOptions
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&opts); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	id, err := s.Backend.Create(r.Context(), opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, sandboxclient.CreateResponse{SandboxID: id})
}

func (s *Server) handleDestroy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing sandbox id")
		return
	}
	err := s.Backend.Destroy(r.Context(), id)
	if errors.Is(err, sandbox.ErrNotFound) {
		writeError(w, http.StatusNotFound, "sandbox not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleExec(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing sandbox id")
		return
	}
	var req sandboxclient.ExecRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Command == "" {
		writeError(w, http.StatusBadRequest, "command is required")
		return
	}
	wd := sandbox.EffectiveWorkingDir(req.Opts)
	if err := sandbox.ValidateExecWorkingDirSyntax(wd); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := s.Backend.Exec(r.Context(), id, req.Command, req.Opts)
	if errors.Is(err, sandbox.ErrNotFound) {
		writeError(w, http.StatusNotFound, "sandbox not found")
		return
	}
	if errors.Is(err, sandbox.ErrInvalidExecWorkingDir) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleReadFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p := r.URL.Query().Get("path")
	if id == "" || p == "" {
		writeError(w, http.StatusBadRequest, "sandbox id and path query are required")
		return
	}
	data, err := s.Backend.ReadFile(r.Context(), id, p)
	if errors.Is(err, sandbox.ErrNotFound) {
		writeError(w, http.StatusNotFound, "sandbox not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) handleWriteFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p := r.URL.Query().Get("path")
	if id == "" || p == "" {
		writeError(w, http.StatusBadRequest, "sandbox id and path query are required")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 64<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "read body")
		return
	}
	err = s.Backend.WriteFile(r.Context(), id, p, body)
	if errors.Is(err, sandbox.ErrNotFound) {
		writeError(w, http.StatusNotFound, "sandbox not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing sandbox id")
		return
	}
	err := s.Backend.HealthCheck(r.Context(), id)
	if errors.Is(err, sandbox.ErrNotFound) {
		writeError(w, http.StatusNotFound, "sandbox not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
