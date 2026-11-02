package hub

import (
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	_ "github.com/ze-software/ze/internal/component/plugin/all"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestLocalCommandRefusalReturned proves a local-data command refused on the
// daemon route reaches the caller as the response error, and that the daemon
// writes nothing on its own stdout or stderr.
//
// VALIDATES: AC-7, R-1. `show env get` with a key no variable carries is
// dispatched through the dispatcher pluginserver.NewServer builds, the way
// the daemon builds it, while the process's stdout and stderr point at
// files. The response is a failed status whose error is the Diagnostic the
// local route prints for the same arguments (asked of the handler itself,
// not written here), without its final newline; both files stay empty.
// PREVENTS: the refusal landing on the daemon's stderr while the remote
// caller gets an empty failure.
func TestLocalCommandRefusalReturned(t *testing.T) {
	const input = "show env get ze.no.such.variable.anywhere"

	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	if err != nil {
		t.Fatalf("daemon dispatcher: %v", err)
	}
	t.Cleanup(server.Stop)
	d := server.Dispatcher()

	handler, args := command.LookupLocalData(strings.Fields(input))
	if handler == nil {
		t.Fatalf("%q has no local-data handler in this binary", input)
	}
	validated, err := command.ValidateModelArgs("show env get", args, nil)
	if err != nil {
		t.Fatalf("local route arguments: %v", err)
	}
	local := handler(validated)
	if local.Payload != nil || local.Code == 0 || local.Diagnostic == "" {
		t.Fatalf("local route did not refuse %q: %+v", input, local)
	}

	stdout, stderr := redirectProcessOutput(t)
	resp, dispatchErr := d.Dispatch(&pluginserver.CommandContext{}, input)
	written := restoreProcessOutput(t, stdout, stderr)

	if dispatchErr == nil {
		t.Fatalf("dispatch of %q succeeded, want the refusal", input)
	}
	if resp == nil {
		t.Fatalf("dispatch of %q answered no response beside %v", input, dispatchErr)
	}
	// Every surface prints an error as "error: " and the text (SSH, ze cli,
	// the web terminal), so the response carries the local refusal without
	// its own "error: ": the surface then prints the local route's bytes once.
	if rendered := "error: " + resp.Error + "\n"; rendered != local.Diagnostic {
		t.Errorf("response error %q renders %q, want the local refusal %q", resp.Error, rendered, local.Diagnostic)
	}
	if written != "" {
		t.Errorf("the daemon wrote %q on its own stdout or stderr", written)
	}
}

// processOutput holds the real stdout and stderr while a test points them at
// files.
type processOutput struct {
	real *os.File
	file *os.File
}

// redirectProcessOutput points os.Stdout and os.Stderr at temporary files.
// The caller MUST call restoreProcessOutput after.
func redirectProcessOutput(t *testing.T) (stdout, stderr processOutput) {
	t.Helper()
	open := func(name string) *os.File {
		f, err := os.CreateTemp(t.TempDir(), name)
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	stdout = processOutput{real: os.Stdout, file: open("stdout")}
	stderr = processOutput{real: os.Stderr, file: open("stderr")}
	os.Stdout = stdout.file
	os.Stderr = stderr.file
	return stdout, stderr
}

// restoreProcessOutput puts the real stdout and stderr back and answers what
// was written to the files meanwhile. It MUST be called after
// redirectProcessOutput.
func restoreProcessOutput(t *testing.T, stdout, stderr processOutput) string {
	t.Helper()
	os.Stdout = stdout.real
	os.Stderr = stderr.real
	var written strings.Builder
	for _, out := range []processOutput{stdout, stderr} {
		if err := out.file.Close(); err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(out.file.Name())
		if err != nil {
			t.Fatal(err)
		}
		written.Write(content)
	}
	return written.String()
}
