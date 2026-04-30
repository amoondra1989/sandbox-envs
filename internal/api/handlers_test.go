package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amoondra1989/sandbox-envs/internal/sandbox"
	"github.com/amoondra1989/sandbox-envs/pkg/sandboxclient"
)

type stubBackend struct {
	createID   string
	createErr  error
	destroyErr error
	execRes    *sandboxclient.ExecResult
	execErr    error
	readData   []byte
	readErr    error
	writeErr   error
	healthErr  error
}

func (s *stubBackend) Create(ctx context.Context, opts sandboxclient.CreateOptions) (string, error) {
	if s.createErr != nil {
		return "", s.createErr
	}
	if s.createID != "" {
		return s.createID, nil
	}
	return "abc123", nil
}

func (s *stubBackend) Destroy(ctx context.Context, sandboxID string) error {
	return s.destroyErr
}

func (s *stubBackend) Exec(ctx context.Context, sandboxID string, command string, opts sandboxclient.ExecOptions) (*sandboxclient.ExecResult, error) {
	if s.execErr != nil {
		return nil, s.execErr
	}
	if s.execRes != nil {
		return s.execRes, nil
	}
	return &sandboxclient.ExecResult{ExitCode: 0, Stdout: []byte("ok")}, nil
}

func (s *stubBackend) ReadFile(ctx context.Context, sandboxID string, path string) ([]byte, error) {
	if s.readErr != nil {
		return nil, s.readErr
	}
	return s.readData, nil
}

func (s *stubBackend) WriteFile(ctx context.Context, sandboxID string, path string, content []byte) error {
	return s.writeErr
}

func (s *stubBackend) HealthCheck(ctx context.Context, sandboxID string) error {
	return s.healthErr
}

func TestHandleCreateDecodeEmptyBody(t *testing.T) {
	h := NewServer(&stubBackend{})
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/sandboxes", "application/json", bytes.NewReader(nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestHandleExecRequiresCommand(t *testing.T) {
	h := NewServer(&stubBackend{})
	srv := httptest.NewServer(h)
	defer srv.Close()

	body := `{"command":"","opts":{}}`
	resp, err := http.Post(srv.URL+"/v1/sandboxes/x/exec", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestHandleDestroyNotFound(t *testing.T) {
	h := NewServer(&stubBackend{destroyErr: sandbox.ErrNotFound})
	req := httptest.NewRequest(http.MethodDelete, "/v1/sandboxes/nope", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestHandleReadFileMissingPath(t *testing.T) {
	h := NewServer(&stubBackend{})
	req := httptest.NewRequest(http.MethodGet, "/v1/sandboxes/x/file", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestHandleExecSuccessJSON(t *testing.T) {
	h := NewServer(&stubBackend{execRes: &sandboxclient.ExecResult{ExitCode: 3, Stdout: []byte("a"), Stderr: []byte("b")}})
	req := httptest.NewRequest(http.MethodPost, "/v1/sandboxes/sid/exec", strings.NewReader(`{"command":"echo hi","opts":{"timeout_seconds":1}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var got sandboxclient.ExecResult
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.ExitCode != 3 || string(got.Stdout) != "a" {
		t.Fatalf("got %+v", got)
	}
}

func TestHandleWriteFileOctetStream(t *testing.T) {
	h := NewServer(&stubBackend{})
	req := httptest.NewRequest(http.MethodPut, "/v1/sandboxes/x/file?path=/tmp/f", bytes.NewReader([]byte{1, 2, 3}))
	req.Header.Set("Content-Type", "application/octet-stream")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleReadFileStreamsBytes(t *testing.T) {
	h := NewServer(&stubBackend{readData: []byte{9, 9}})
	req := httptest.NewRequest(http.MethodGet, "/v1/sandboxes/x/file?path=/tmp/a", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
	b, _ := io.ReadAll(rec.Body)
	if string(b) != "\t\t" {
		t.Fatalf("got %v", b)
	}
}

func TestHandleHealthServiceUnavailable(t *testing.T) {
	h := NewServer(&stubBackend{healthErr: errors.New("down")})
	req := httptest.NewRequest(http.MethodGet, "/v1/sandboxes/x/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code=%d", rec.Code)
	}
}
