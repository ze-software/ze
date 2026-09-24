// VALIDATES: `le build host-driver` registers its command and refuses an
// argument before any compiler runs.
// PREVENTS: a stray word after the command name being read as a request to
// build, when the old `build-artifacts host` verb became its own command.

package buildhostdriver

import (
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func TestHostDriverRegistersAndRefusesAnArgument(t *testing.T) {
	if !registry.HasLocal(leroot.CommandPath(name)) {
		t.Fatalf("importing buildhostdriver did not register %q", name)
	}
	payload, code := Answer([]string{"amd64"})
	if code == 0 {
		t.Fatal("an argument after `build host-driver` was accepted")
	}
	if payload != nil {
		t.Fatalf("refusal payload = %#v, want nil", payload)
	}
}
