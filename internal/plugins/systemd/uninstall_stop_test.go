package systemd

import (
	"errors"
	"strings"
	"testing"
)

func TestServiceUninstallStopFailurePreservesUnit(t *testing.T) {
	fake := newFakeServiceOps()
	fake.files[defaultUnitPath] = "operator unit"
	fake.runErrors["systemctl stop ze.service"] = errors.New("stop refused")
	rt, _, stderr := newTestRuntime(fake)
	if code := rt.cmdUninstall(nil); code == 0 {
		t.Fatalf("uninstall succeeded: %s", stderr)
	}
	if fake.files[defaultUnitPath] != "operator unit" {
		t.Fatal("stop failure removed unit")
	}
	for _, call := range fake.runCalls {
		if strings.Contains(call, "disable") {
			t.Fatalf("stop failure disabled service: %v", fake.runCalls)
		}
	}
}
