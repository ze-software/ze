package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/test/runner"
)

func TestCISubcommandPrintsHeaderAndTopLevelRerunHints(t *testing.T) {
	tests := runner.NewEncodingTests(t.TempDir())
	r, err := runner.NewRunner(tests, t.TempDir())
	if err != nil {
		t.Fatalf("create runner: %v", err)
	}
	defer r.Cleanup()

	var display bytes.Buffer
	r.Display().SetOutput(&display)
	ConfigureCIRunnerOutput(r, "ui")
	if !strings.Contains(display.String(), "ui") {
		t.Fatalf("missing suite header:\n%s", display.String())
	}

	if got := runner.FormatRerunCommand("ui", []string{"7"}); got != "ze-test ui 7" {
		t.Fatalf("wrong top-level rerun hint: %s", got)
	}
	if got := runner.FormatRerunCommand("plugin", []string{"7"}); got != "ze-test bgp plugin 7" {
		t.Fatalf("wrong BGP rerun hint: %s", got)
	}
}
