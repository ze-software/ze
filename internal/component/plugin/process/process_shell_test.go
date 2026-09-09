package process

import (
	"errors"
	"strings"
	"testing"
)

// VALIDATES: a live external plugin start that failed because the host carries
// no shell reports the shell, not the plugin. The operator repairs the
// dependency the config depends on rather than reading the plugin's own code.
//
// The shell answer is a parameter of the function under test, so this test
// states it: no test can take /bin/sh away from the host it runs on.
func TestStartFailureNamesTheAbsentShell(t *testing.T) {
	shellErr := errors.New("the shell /bin/sh that starts an external plugin is absent: stat /bin/sh: no such file or directory")

	reported := startFailure(errors.New("fork/exec /bin/sh: no such file or directory"), shellErr)

	if !strings.Contains(reported.Error(), "/bin/sh") {
		t.Errorf("the error must name the absent shell, and it says %q", reported.Error())
	}
	if !errors.Is(reported, shellErr) {
		t.Errorf("the error must be the shell answer, and it says %q", reported.Error())
	}
}

// VALIDATES: a start that failed for its own reason keeps that reason, so the
// shell branch cannot swallow a fault the shell did not cause.
func TestStartFailureKeepsTheStartErrorWhenTheShellIsThere(t *testing.T) {
	startErr := errors.New("permission denied")

	reported := startFailure(startErr, nil)

	if !errors.Is(reported, startErr) {
		t.Errorf("the start error must be reported, and it says %q", reported.Error())
	}
}
