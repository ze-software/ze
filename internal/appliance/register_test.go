package appliance

import (
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
)

func TestApplianceRootRegistered(t *testing.T) {
	if registry.LookupRoot("appliance") == nil {
		t.Fatal("appliance root handler not registered")
	}
}

func TestApplianceAllActionsRegistered(t *testing.T) {
	table := dispatchTable()
	want := []string{
		"init", "assemble", "build", "iso", "push",
		"config", "config-push", "passwd", "replace-cert", "rekey",
		"clone", "list", "show", "run", "unlock", "export", "import",
	}
	for _, action := range want {
		if _, ok := table[action]; !ok {
			t.Errorf("action %q not in dispatch table", action)
		}
	}
}

func TestApplianceRegistersNoDaemonCommand(t *testing.T) {
	for _, cmd := range registry.ListLocal() {
		if cmd.Path == "appliance" || len(cmd.Path) > 10 && cmd.Path[:10] == "appliance " {
			t.Errorf("appliance registered a local/daemon command: %q", cmd.Path)
		}
	}
}
