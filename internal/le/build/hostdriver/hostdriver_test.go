// VALIDATES: `le build host-driver` registers its command and refuses an
// argument before any compiler runs.
// PREVENTS: a stray word after the command name being read as a request to
// build, when the old `build-artifacts host` verb became its own command.

package buildhostdriver

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	buildcompile "github.com/ze-software/ze/internal/le/build/compile"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func TestHostDriverRegistersAndRefusesAnArgument(t *testing.T) {
	if !command.HasLocal(leroot.CommandPath(name)) {
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

// VALIDATES: the runtime-kernel build every le refusal prints resolves: its
// first line is this registered verb, and its second runs the binary that verb
// writes, built with ze_setup, the tag that carries `appliance`.
// PREVENTS: a refusal naming `./ze appliance kernel`, which the ./ze wrapper
// (ze_core, ze_distro) answers with "unknown command: appliance".
func TestRuntimeKernelBuildResolvesToRegisteredCommands(t *testing.T) {
	lines := strings.Split(RuntimeKernelBuild("arm64"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want two command lines, got %q", lines)
	}
	verb, found := strings.CutPrefix(strings.TrimSpace(lines[0]), "./le ")
	if !found || !command.HasLocal(leroot.CommandPath(verb)) {
		t.Errorf("first line %q is not a registered le verb", lines[0])
	}
	if strings.TrimSpace(lines[1]) != "./"+buildcompile.HostBinary+" appliance kernel --target runtime --arch arm64" {
		t.Errorf("second line %q does not run the host driver's kernel build", lines[1])
	}
	if !strings.Contains(buildcompile.HostTags, "ze_setup") {
		t.Errorf("ze-host tags %q lack ze_setup, which carries `appliance`", buildcompile.HostTags)
	}
}
