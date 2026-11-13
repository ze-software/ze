// Design: docs/functional-tests.md -- fixture inspection is independent of guest user accounts.

package runner

import "testing"

func TestNetnsFixtureSetcapCarriesAccountIndependentInspection(t *testing.T) {
	daemon, err := NetnsDaemonSetcap()
	if err != nil {
		t.Fatal(err)
	}
	const production = "cap_net_admin,cap_net_raw,cap_net_bind_service,cap_bpf+ep"
	if daemon != production {
		t.Fatalf("inspection changed the daemon grant: %q", daemon)
	}
	got, err := NetnsFixtureSetcap()
	if err != nil {
		t.Fatal(err)
	}
	const want = "cap_net_admin,cap_net_raw,cap_net_bind_service,cap_bpf,cap_kill,cap_sys_ptrace+ep"
	if got != want {
		t.Fatalf("fixture inspection grant=%q, want %q", got, want)
	}
}
