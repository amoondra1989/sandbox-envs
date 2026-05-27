package sandboxclient

import (
	"testing"
)

func TestClientEndpointJoin(t *testing.T) {
	c := Client{BaseURL: "http://127.0.0.1:8088"}
	ep, err := c.endpoint("v1", "sandboxes", "abc", "exec")
	if err != nil {
		t.Fatal(err)
	}
	if ep != "http://127.0.0.1:8088/v1/sandboxes/abc/exec" {
		t.Fatalf("got %q", ep)
	}
}
