// Design: docs/architecture/plugin/plugin-system.md -- isolated registration values and test restoration.
// Related: runner.go -- temporary connection wrapping for real transport fixtures.

package registry

import (
	"net"
	"reflect"
	"slices"
	"testing"
)

type runnerTestConn struct{ net.Conn }

// TestWrapRunnerForTestPreservesRegistry proves that a transport fixture can
// wrap the real runner without changing registration metadata, saved readers,
// setup records or unrelated registrations. A successful restore is one-shot.
func TestWrapRunnerForTestPreservesRegistry(t *testing.T) {
	snapshot := Snapshot()
	t.Cleanup(func() { Restore(snapshot) })
	reg := validReg("test-runner-wrap")
	reg.Families = []string{"ipv4/test-runner-wrap"}
	reg.RunEngine = func(conn net.Conn) int {
		if _, wrapped := conn.(*runnerTestConn); wrapped {
			return 17
		}
		return 11
	}
	if err := Register(reg); err != nil {
		t.Fatal(err)
	}
	RecordSetup(reg.Name, SetupSucceeded, "retained setup result")
	original := Lookup(reg.Name)
	before := Snapshot()
	wantSetup := SetupResults()
	restore, err := WrapRunnerForTest(reg.Name, func(conn net.Conn) net.Conn {
		return &runnerTestConn{Conn: conn}
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := Lookup(reg.Name).RunEngine(nil); got != 17 {
		t.Fatalf("fresh runner = %d, want wrapped result 17", got)
	}
	if got := original.RunEngine(nil); got != 11 {
		t.Fatalf("saved reader runner = %d, want original result 11", got)
	}
	if got := before.plugins[reg.Name].RunEngine(nil); got != 11 {
		t.Fatalf("saved snapshot runner = %d, want original result 11", got)
	}
	found := false
	for _, registered := range All() {
		if registered.Name != reg.Name {
			continue
		}
		found = true
		if got := registered.RunEngine(nil); got != 17 {
			t.Fatalf("All runner = %d, want wrapped result 17", got)
		}
		metadata, want := *registered, *original
		metadata.RunEngine, want.RunEngine = nil, nil
		metadata.CLIHandler, want.CLIHandler = nil, nil
		if !reflect.DeepEqual(metadata, want) {
			t.Fatal("connection wrapper changed registration metadata")
		}
	}
	if !found {
		t.Fatal("wrapped registration disappeared from All")
	}
	if got := PluginForFamily(reg.Families[0]); got != reg.Name {
		t.Fatalf("family owner = %q, want %q", got, reg.Name)
	}
	if got := SetupResults(); !slices.Equal(got, wantSetup) {
		t.Fatalf("setup results changed: got %+v, want %+v", got, wantSetup)
	}
	if err := Register(validReg("test-runner-unrelated")); err != nil {
		t.Fatal(err)
	}
	if err := restore(); err != nil {
		t.Fatal(err)
	}
	if got := Lookup(reg.Name).RunEngine(nil); got != 11 {
		t.Fatalf("restored runner = %d, want original result 11", got)
	}
	if !Has("test-runner-unrelated") {
		t.Fatal("runner restoration removed an unrelated registration")
	}
	if err := restore(); err == nil {
		t.Fatal("second restoration was accepted")
	}
	Restore(before)
	if got := Lookup(reg.Name).RunEngine(nil); got != 11 {
		t.Fatalf("snapshot runner = %d, want original result 11", got)
	}
}

// TestWrapRunnerForTestRefusesInvalidInput proves that an absent runner and
// a missing wrapper are refused without changing a registered engine.
func TestWrapRunnerForTestRefusesInvalidInput(t *testing.T) {
	snapshot := Snapshot()
	t.Cleanup(func() { Restore(snapshot) })
	if err := Register(validReg("test-runner-refusal")); err != nil {
		t.Fatal(err)
	}
	wrap := func(conn net.Conn) net.Conn { return &runnerTestConn{Conn: conn} }
	if restore, err := WrapRunnerForTest("test-runner-absent", wrap); err == nil {
		t.Fatal("unknown registration was accepted")
	} else if restore != nil {
		t.Fatal("unknown registration returned a restoration")
	}
	if restore, err := WrapRunnerForTest("test-runner-refusal", nil); err == nil {
		t.Fatal("nil wrapper was accepted")
	} else if restore != nil {
		t.Fatal("nil wrapper returned a restoration")
	}
	if got := Lookup("test-runner-refusal").RunEngine(nil); got != 0 {
		t.Fatalf("refused replacement changed runner result to %d", got)
	}
}

// TestWrapRunnerForTestKeepsLaterRegistration proves that a restoration cannot
// overwrite a later declaration update to the same registration.
func TestWrapRunnerForTestKeepsLaterRegistration(t *testing.T) {
	snapshot := Snapshot()
	t.Cleanup(func() { Restore(snapshot) })
	if err := Register(validReg("test-runner-later")); err != nil {
		t.Fatal(err)
	}
	restore, err := WrapRunnerForTest("test-runner-later", func(conn net.Conn) net.Conn {
		return &runnerTestConn{Conn: conn}
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := AddConfigRead("test-runner-later", "test-runner-later-root"); err != nil {
		t.Fatal(err)
	}
	if err := restore(); err == nil {
		t.Fatal("restoration overwrote a later declaration update")
	}
	if !slices.Contains(Lookup("test-runner-later").ConfigReads, "test-runner-later-root") {
		t.Fatal("restoration lost the later declaration update")
	}
}
