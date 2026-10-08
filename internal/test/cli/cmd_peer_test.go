// Design: docs/architecture/testing/ci-format.md — test runner CLI

package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/test/peer"
)

// VALIDATES: option=await_eor:value=true declared in a .ci peer block reaches
// the running peer's Config.AwaitEOR, so the batch actually waits for ze's
// End-of-RIB before handing connections to their scripts.
// PREVENTS: the option being parsed and then dropped by the command-line merge.
// It was dropped for exactly that reason, and because a dropped option is
// silent, every conn_map test that opted in never waited: the source's UPDATE
// raced bgp-rs's peer-up registration, landed at or below the receiver's cut,
// and reached the receiver through the announce-only Adj-RIB-In replay with its
// withdrawal missing (test/plugin/rfc7606-relay-one-field.ci).
func TestPeerAwaitEOROptionReachesConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "peer.ci")
	body := "option=conn_map:value=remote-ip\n" +
		"option=tcp_connections:value=2\n" +
		"option=await_eor:value=true\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write peer block: %v", err)
	}

	_, fileConfig, err := peer.LoadExpectFile(path)
	if err != nil {
		t.Fatalf("LoadExpectFile: %v", err)
	}
	if !fileConfig.AwaitEOR {
		t.Fatal("parser did not set AwaitEOR from option=await_eor:value=true")
	}

	config := &peer.Config{}
	zeTestMergePeerFileConfig(config, fileConfig)
	if !config.AwaitEOR {
		t.Fatal("merge dropped AwaitEOR: option=await_eor is accepted but has no effect")
	}
	if config.ConnMap != "remote-ip" || config.TCPConnections != 2 {
		t.Fatalf("merge dropped a sibling option: conn-map=%q tcp-connections=%d",
			config.ConnMap, config.TCPConnections)
	}

	// Through the real entry point, so the merge being CALLED is covered too.
	// Testing the merge function alone leaves the same hole one level up: a
	// correct, complete merge that nothing invokes drops every option exactly as
	// the original bug did (ai/rules/evidence.md, "drive the guard from
	// the entry point").
	full, ok := zeTestParsePeerFlags([]string{path})
	if !ok || full == nil {
		t.Fatal("zeTestParsePeerFlags rejected a peer block it should accept")
	}
	if !full.AwaitEOR {
		t.Fatal("the peer built from the command line does not carry await_eor: " +
			"the .ci option is parsed and then never reaches the running peer")
	}
	if full.ConnMap != "remote-ip" || full.TCPConnections != 2 {
		t.Fatalf("the peer built from the command line dropped sibling options: "+
			"conn-map=%q tcp-connections=%d", full.ConnMap, full.TCPConnections)
	}
}

// VALIDATES: every peer.Config field the .ci peer block can set is folded by
// zeTestMergePeerFileConfig.
// PREVENTS: the next field being added to the parser and forgotten in the
// merge. Derived from the struct by reflection rather than a hand-written list,
// so a new field fails this test until it is either folded or listed below as
// deliberately not file-sourced. Fail-closed: an unknown field counts as
// must-be-folded.
func TestPeerFileConfigMergeIsComplete(t *testing.T) {
	// Fields that never come from the .ci peer block. Each is set by a
	// command-line flag or by the runner, so the merge must NOT carry it.
	notFileSourced := map[string]string{
		"Port":   "--port flag",
		"Mode":   "--mode flag",
		"Decode": "--decode flag",
		"Dial":   "--dial flag",
		"Inject": "--inject-* flags",
		"TTL":    "--ttl flag (GTSM peers: `le test peer --port $PORT --ttl 255`)",
		"Output": "set by the peer runtime, not by config",
		"Expect": "assigned directly from LoadExpectFile's first return value",
	}

	fileConfig := &peer.Config{}
	fv := reflect.ValueOf(fileConfig).Elem()
	for i := range fv.NumField() {
		name := fv.Type().Field(i).Name
		if _, skip := notFileSourced[name]; skip {
			continue
		}
		setNonZero(t, fv.Field(i), name)
	}

	config := &peer.Config{}
	zeTestMergePeerFileConfig(config, fileConfig)

	cv := reflect.ValueOf(config).Elem()
	for i := range cv.NumField() {
		name := cv.Type().Field(i).Name
		if _, skip := notFileSourced[name]; skip {
			continue
		}
		if cv.Field(i).IsZero() {
			t.Errorf("zeTestMergePeerFileConfig drops field %s: a .ci option that sets it "+
				"is accepted and then silently ignored. Fold it in the merge, or add it to "+
				"notFileSourced with the flag that owns it", name)
		}
	}
}

// setNonZero gives a field a value distinguishable from its zero value, so the
// merge check above can tell "carried over" from "left untouched".
func setNonZero(t *testing.T, f reflect.Value, name string) {
	t.Helper()
	//exhaustive:ignore // Seed only peer.Config field kinds; fail the test for unsupported kinds.
	switch f.Kind() {
	case reflect.Bool:
		f.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		f.SetInt(1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		f.SetUint(1)
	case reflect.String:
		f.SetString("x")
	case reflect.Slice:
		f.Set(reflect.MakeSlice(f.Type(), 1, 1))
	case reflect.Pointer:
		f.Set(reflect.New(f.Type().Elem()))
	default:
		t.Fatalf("field %s has kind %s, which setNonZero cannot populate; extend it so this "+
			"test keeps covering every field", name, f.Kind())
	}
}

// TestPeerInjectPerUpdateAndVaryReachSpec checks that --inject-per-update and
// --inject-vary-attrs reach the running peer's InjectSpec, and that a negative
// cap is refused. Method: parse the flag lists a stress scenario passes.
// VALIDATES: the profile scenario's per-UPDATE input reaches the injector.
// PREVENTS: a flag parsed and dropped, which packs the stream into a few
// UPDATEs and leaves the per-UPDATE paths out of the profile.
func TestPeerInjectPerUpdateAndVaryReachSpec(t *testing.T) {
	base := []string{
		"--mode", "inject", "--dial", "172.31.0.2:179",
		"--inject-prefix", "10.0.0.0/24", "--inject-count", "10",
		"--inject-nexthop", "172.31.0.3", "--inject-asn", "65100",
	}
	config, ok := zeTestParsePeerFlags(append(slices.Clone(base), "--inject-per-update", "1", "--inject-vary-attrs"))
	if config == nil || !ok {
		t.Fatal("zeTestParsePeerFlags rejected a valid inject flag list")
	}
	if config.Inject.PerUpdate != 1 {
		t.Errorf("PerUpdate: got %d, want 1", config.Inject.PerUpdate)
	}
	if !config.Inject.VaryAttrs {
		t.Error("VaryAttrs: got false, want true")
	}

	config, _ = zeTestParsePeerFlags(slices.Clone(base))
	if config == nil {
		t.Fatal("zeTestParsePeerFlags rejected the flag list without the optional flags")
	}
	if config.Inject.PerUpdate != 0 || config.Inject.VaryAttrs {
		t.Errorf("defaults: got PerUpdate=%d VaryAttrs=%v, want 0 and false", config.Inject.PerUpdate, config.Inject.VaryAttrs)
	}

	if config, _ := zeTestParsePeerFlags(append(slices.Clone(base), "--inject-per-update", "-1")); config != nil {
		t.Error("a negative --inject-per-update was accepted")
	}
}
