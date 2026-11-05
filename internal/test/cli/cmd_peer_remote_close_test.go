// Design: docs/architecture/testing/ci-format.md -- remote_close peer option

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/test/peer"
)

func TestPeerRemoteCloseOptionReachesConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "peer.ci")
	if err := os.WriteFile(path, []byte("option=remote_close:value=true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	config, ok := zeTestParsePeerFlags([]string{path})
	if !ok {
		t.Fatal("peer command rejected remote_close")
	}
	if config == nil {
		t.Fatal("peer command returned no config")
	}
	if !config.RemoteClose {
		t.Fatal("peer command dropped remote_close")
	}
	if _, err := peer.New(config); err != nil {
		t.Fatal(err)
	}
}
