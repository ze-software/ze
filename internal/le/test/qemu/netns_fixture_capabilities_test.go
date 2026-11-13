// Design: docs/architecture/testing/qemu-integration.md -- separate daemon and fixture grants.

package testqemu

import (
	"strings"
	"testing"
)

func TestNamespacePreparationKeepsFixtureProcessControlOffDaemons(t *testing.T) {
	run := vmFixture(t)
	rec := &recorder{}
	run.Run = rec.run
	run.Execute()
	if len(rec.calls) == 0 {
		t.Fatal("namespace preparation executed no command")
	}
	first := strings.Join(rec.calls[0], " ")
	const fixtureGrant = "cap_net_admin,cap_net_raw,cap_net_bind_service,cap_bpf,cap_kill,cap_sys_ptrace+ep"
	for _, name := range []string{zeName, zeStrippedName} {
		target := netnsCapDir + "/" + name
		if !strings.Contains(first, setcapCommand+" "+wantNetnsGrant+" "+target+";") {
			t.Fatalf("production daemon %s lost its exact grant:\n%s", name, first)
		}
		if strings.Contains(first, setcapCommand+" "+fixtureGrant+" "+target+";") {
			t.Fatalf("production daemon %s received fixture-only process control:\n%s", name, first)
		}
		for _, command := range strings.Split(first, ";") {
			fields := strings.Fields(command)
			if len(fields) != 3 {
				continue
			}
			if fields[0] != setcapCommand {
				continue
			}
			if fields[2] != target {
				continue
			}
			if strings.Contains(fields[1], "cap_kill") {
				t.Fatalf("production daemon %s received fixture process control: %q", name, fields[1])
			}
			if strings.Contains(fields[1], "cap_sys_ptrace") {
				t.Fatalf("production daemon %s received fixture inspection: %q", name, fields[1])
			}
		}
	}
	target := netnsCapDir + "/" + leName
	if !strings.Contains(first, setcapCommand+" "+fixtureGrant+" "+target+";") {
		t.Fatalf("fixture copy cannot stop its privileged child:\n%s", first)
	}
}
