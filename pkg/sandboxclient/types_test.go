package sandboxclient

import "testing"

func TestCreateOptionsMountContainerSocketDefault(t *testing.T) {
	if !(CreateOptions{}.MountContainerSocketEnabled()) {
		t.Fatal("expected default true")
	}
	if (CreateOptions{MountContainerSocket: Bool(false)}).MountContainerSocketEnabled() {
		t.Fatal("expected false when set")
	}
}
