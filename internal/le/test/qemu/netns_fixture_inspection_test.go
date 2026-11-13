// Design: docs/architecture/testing/qemu-integration.md -- fixture-only process inspection.

package testqemu

import (
	"strings"
	"testing"
)

func TestNamespacePreparationMakesInspectionIndependentOfAccounts(t *testing.T) {
	run := vmFixture(t)
	rec := &recorder{}
	run.Run = rec.run
	run.Execute()
	if len(rec.calls) == 0 {
		t.Fatal("namespace preparation executed no command")
	}
	first := strings.Join(rec.calls[0], " ")
	seen := make(map[string]bool)
	for _, command := range strings.Split(first, ";") {
		fields := strings.Fields(command)
		if len(fields) != 3 {
			continue
		}
		if fields[0] != setcapCommand {
			continue
		}
		for _, name := range []string{zeName, zeStrippedName} {
			if fields[2] != netnsCapDir+"/"+name {
				continue
			}
			seen[name] = true
			if fields[1] != wantNetnsGrant {
				t.Fatalf("daemon %s received fixture privileges: %q", name, fields[1])
			}
		}
	}
	for _, name := range []string{zeName, zeStrippedName} {
		if !seen[name] {
			t.Fatalf("preparation omitted daemon %s", name)
		}
	}
	const fixtureGrant = "cap_net_admin,cap_net_raw,cap_net_bind_service,cap_bpf,cap_kill,cap_sys_ptrace+ep"
	if !strings.Contains(first, setcapCommand+" "+fixtureGrant+" "+netnsCapDir+"/"+leName+";") {
		t.Fatal("prepared fixture cannot inspect a nondumpable daemon without a sudo account")
	}
}
