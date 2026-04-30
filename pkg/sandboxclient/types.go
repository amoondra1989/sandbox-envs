package sandboxclient

// CreateOptions configures sandbox creation. Image is chosen by the server (fixed image).
type CreateOptions struct{}

// ExecOptions configures synchronous exec inside a sandbox.
type ExecOptions struct {
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
	WorkingDir     string            `json:"working_dir,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
}

// ExecResult is the outcome of Exec (stdout/stderr JSON-encoded as base64).
type ExecResult struct {
	ExitCode int    `json:"exit_code"`
	Stdout   []byte `json:"stdout"`
	Stderr   []byte `json:"stderr"`
}

// CreateResponse is returned after Create.
type CreateResponse struct {
	SandboxID string `json:"sandbox_id"`
}

// ExecRequest is the JSON body for POST .../exec.
type ExecRequest struct {
	Command string      `json:"command"`
	Opts    ExecOptions `json:"opts"`
}
