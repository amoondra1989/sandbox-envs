package sandboxclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
)

// Client talks to the sandbox REST API (stdlib HTTP only).
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func (c *Client) base() (*url.URL, error) {
	raw := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if raw == "" {
		return nil, fmt.Errorf("sandboxclient: BaseURL is empty")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (c *Client) endpoint(parts ...string) (string, error) {
	if _, err := c.base(); err != nil {
		return "", err
	}
	if len(parts) == 0 {
		return strings.TrimRight(strings.TrimSpace(c.BaseURL), "/"), nil
	}
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	suffix := path.Join(parts...)
	return base + "/" + suffix, nil
}

// Create provisions a new sandbox.
func (c *Client) Create(ctx context.Context, opts CreateOptions) (string, error) {
	ep, err := c.endpoint("v1", "sandboxes")
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(opts)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("sandboxclient: create: %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	var out CreateResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return "", err
	}
	if out.SandboxID == "" {
		return "", fmt.Errorf("sandboxclient: empty sandbox_id")
	}
	return out.SandboxID, nil
}

// Destroy deletes a sandbox.
func (c *Client) Destroy(ctx context.Context, sandboxID string) error {
	ep, err := c.endpoint("v1", "sandboxes", sandboxID)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, ep, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sandboxclient: destroy: %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return nil
}

// Exec runs a command synchronously inside the sandbox.
func (c *Client) Exec(ctx context.Context, sandboxID string, command string, opts ExecOptions) (*ExecResult, error) {
	ep, err := c.endpoint("v1", "sandboxes", sandboxID, "exec")
	if err != nil {
		return nil, err
	}
	payload := ExecRequest{Command: command, Opts: opts}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sandboxclient: exec: %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	var out ExecResult
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ReadFile returns file contents from the sandbox.
func (c *Client) ReadFile(ctx context.Context, sandboxID string, filePath string) ([]byte, error) {
	ep, err := c.endpoint("v1", "sandboxes", sandboxID, "file")
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(ep)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("path", filePath)
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sandboxclient: read_file: %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return b, nil
}

// WriteFile writes content to a path inside the sandbox.
func (c *Client) WriteFile(ctx context.Context, sandboxID string, filePath string, content []byte) error {
	ep, err := c.endpoint("v1", "sandboxes", sandboxID, "file")
	if err != nil {
		return err
	}
	u, err := url.Parse(ep)
	if err != nil {
		return err
	}
	q := u.Query()
	q.Set("path", filePath)
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u.String(), bytes.NewReader(content))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sandboxclient: write_file: %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return nil
}

// HealthCheck verifies the sandbox is reachable.
func (c *Client) HealthCheck(ctx context.Context, sandboxID string) error {
	ep, err := c.endpoint("v1", "sandboxes", sandboxID, "health")
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ep, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("sandboxclient: health: %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return nil
}
