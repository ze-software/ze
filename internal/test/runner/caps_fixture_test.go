// Design: docs/functional-tests.md -- fixture process control in isolated tests.

package runner

import "testing"

func TestNetnsFixtureSetcapAddsOnlyProcessControl(t *testing.T) {
	got, err := NetnsFixtureSetcap()
	if err != nil {
		t.Fatal(err)
	}
	const want = "cap_net_admin,cap_net_raw,cap_net_bind_service,cap_bpf,cap_kill,cap_sys_ptrace+ep"
	if got != want {
		t.Fatalf("fixture grant=%q, want %q", got, want)
	}
}

func TestNetnsFixtureSetcapKeepsTheDaemonGrant(t *testing.T) {
	if _, err := NetnsFixtureSetcap(); err != nil {
		t.Fatal(err)
	}
	got, err := NetnsDaemonSetcap()
	if err != nil {
		t.Fatal(err)
	}
	const want = "cap_net_admin,cap_net_raw,cap_net_bind_service,cap_bpf+ep"
	if got != want {
		t.Fatalf("daemon grant changed to fixture privileges: %q", got)
	}
}
