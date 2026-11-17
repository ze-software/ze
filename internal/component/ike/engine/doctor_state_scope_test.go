// Design: docs/architecture/doctor-and-health-checks.md -- runtime kernel dependencies.
// Related: doctor_runtime.go -- the engine's Stage 1 declaration and callback dispatch.
// Related: dataplane/xfrm_state_scope_linux.go -- the fresh complete capability check.

package engine

import (
	"errors"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// doctorStateScopeDP exposes only the backend's read-only capability check. No
// test creates a Child SA or publishes any state to manufacture the finding.
type doctorStateScopeDP struct {
	catchAllDP
	err   error
	reads int
}

func (*doctorStateScopeDP) SAStateScopeSupported() bool { return true }

func (d *doctorStateScopeDP) CheckSAStateScope() error {
	d.reads++
	return d.err
}

// doctorStateScopeFailure supplies the same operator-error boundary as XFRM's
// existing gate. These tests judge forwarding that boundary, not receipt parsing.
func doctorStateScopeFailure(cause error) *opererr.Error {
	return opererr.Wrap(errors.Join(dataplane.ErrSAOptionRefused, cause),
		"Ze cannot verify kernel support for the IPsec SA's complete traffic selector scope",
		"the SA needs verified state, ICMP quote and fragment checks before any state can be installed; rebuild and boot Ze's native runtime kernel",
		"  ./le build host-driver\n  ./ze-host appliance kernel --target runtime --arch "+runtime.GOARCH)
}

// doctorStateScopeEngine holds the applied configuration with no running peers,
// as the real engine does before OnAllPluginsReady starts its first session.
func doctorStateScopeEngine(configured bool) *ikeEngineState {
	state := &ikeEngineState{}
	if configured {
		state.applied = testIPsecConfig(applyTestPeer("127.0.0.1"))
	}
	return state
}

// TestDoctorStateScopeRegistered requires the actual runtime declaration that
// makes the daemon ask the engine's own process, including when it runs forked.
func TestDoctorStateScopeRegistered(t *testing.T) {
	const name = "ipsec-state-scope"
	count := 0
	for _, decl := range engineDoctorCheckDecls() {
		if decl.Name != name {
			continue
		}
		count++
		if decl.Phase != rpc.DoctorPhaseRuntime {
			t.Errorf("phase = %q, want engine runtime", decl.Phase)
		}
		if !slices.Equal(decl.Codes, []string{"doctor-ipsec-state-scope"}) {
			t.Errorf("codes = %v, want the complete-scope dependency code", decl.Codes)
		}
	}
	if count != 1 {
		t.Fatalf("Stage 1 declares %d scope checks, want exactly one", count)
	}
	for _, check := range registry.PluginDoctorChecks() {
		if check.PluginName == "ike" {
			if check.Name == name {
				t.Fatal("scope check is also declared in the registry process")
			}
		}
	}
}

// TestDoctorStateScopeBeforeAnyChild reports partial, absent and unreadable
// capability receipts through the existing dispatch before any Child attempt.
func TestDoctorStateScopeBeforeAnyChild(t *testing.T) {
	for _, cause := range []string{
		"kernel scope features 0x7 do not provide all required features 0xf",
		"the kernel returned no capability receipt",
		"read GETDEFAULT: operation not permitted",
	} {
		t.Run(cause, func(t *testing.T) {
			failure := doctorStateScopeFailure(errors.New(cause))
			dp := &doctorStateScopeDP{err: failure}
			loadESPFormBackend(t, dp)
			state := doctorStateScopeEngine(true)
			findings, err := state.answerDoctorCheck("ipsec-state-scope")
			if err != nil {
				t.Fatalf("registered scope dependency cannot be checked before a Child exists: %v", err)
			}
			if len(findings) != 1 {
				t.Fatalf("findings = %+v, want one scope dependency error", findings)
			}
			finding := findings[0]
			if finding.Code != "doctor-ipsec-state-scope" {
				t.Errorf("code = %q, want scope dependency", finding.Code)
			}
			if finding.Severity != "error" {
				t.Errorf("severity = %q, want error", finding.Severity)
			}
			for _, text := range []string{failure.Problem(), failure.Why(), cause} {
				if !strings.Contains(finding.Message, text) {
					t.Errorf("message = %q, missing canonical cause or explanation %q", finding.Message, text)
				}
			}
			if finding.Fix != failure.Fix() {
				t.Errorf("fix = %q, want canonical kernel repair %q", finding.Fix, failure.Fix())
			}
			if dp.reads != 1 {
				t.Errorf("capability reads = %d, want one fresh read", dp.reads)
			}
			if len(state.peers) != 0 {
				t.Fatal("the check required a running peer to expose the dependency")
			}
		})
	}
}

// TestDoctorStateScopeFreshRead makes the same loaded backend recover and fail
// again, so neither a retained error nor a cached success can answer show doctor.
func TestDoctorStateScopeFreshRead(t *testing.T) {
	dp := &doctorStateScopeDP{}
	loadESPFormBackend(t, dp)
	state := doctorStateScopeEngine(true)
	for _, step := range []struct {
		name string
		err  error
		want int
	}{
		{"partial 0x7", doctorStateScopeFailure(errors.New("kernel scope features 0x7 lack fragment gathering")), 1},
		{"complete 0xf", nil, 0},
		{"receipt unreadable again", doctorStateScopeFailure(errors.New("GETDEFAULT receipt unreadable")), 1},
	} {
		dp.err = step.err
		findings, err := state.answerDoctorCheck("ipsec-state-scope")
		if err != nil {
			t.Fatalf("%s: dispatch failed: %v", step.name, err)
		}
		if len(findings) != step.want {
			t.Errorf("%s: findings = %+v, want %d", step.name, findings, step.want)
		}
	}
	if dp.reads != 3 {
		t.Errorf("capability reads = %d, want one per doctor request", dp.reads)
	}
}

// TestDoctorStateScopeInactiveAndUndeclaredSilent preserves configurations with
// no peers and backends that have not declared this XFRM runtime dependency.
func TestDoctorStateScopeInactiveAndUndeclaredSilent(t *testing.T) {
	for _, configured := range []bool{false, true} {
		for _, declared := range []bool{false, true} {
			if configured {
				if declared {
					continue // The active, declared failure is the positive test above.
				}
			}
			t.Run(strconv.FormatBool(configured)+"/"+strconv.FormatBool(declared), func(t *testing.T) {
				dp := &doctorStateScopeDP{err: doctorStateScopeFailure(errors.New("must remain unread"))}
				var backend dataplane.Dataplane = &catchAllDP{}
				if declared {
					backend = dp
				}
				loadESPFormBackend(t, backend)
				state := doctorStateScopeEngine(configured)
				findings, err := state.answerDoctorCheck("ipsec-state-scope")
				if err != nil {
					t.Fatalf("registered check cannot answer the inactive control: %v", err)
				}
				if len(findings) != 0 {
					t.Errorf("inactive or undeclared backend produced %+v", findings)
				}
				if dp.reads != 0 {
					t.Errorf("inactive dependency was probed %d times", dp.reads)
				}
			})
		}
	}
}

// TestDoctorStateScopeAppliedWithoutPeersSilent distinguishes an applied empty
// peer set from an engine that has not received any configuration yet.
func TestDoctorStateScopeAppliedWithoutPeersSilent(t *testing.T) {
	dp := &doctorStateScopeDP{err: doctorStateScopeFailure(errors.New("must remain unread"))}
	loadESPFormBackend(t, dp)
	state := &ikeEngineState{applied: testIPsecConfig()}
	findings, err := state.answerDoctorCheck("ipsec-state-scope")
	if err != nil {
		t.Fatalf("registered check cannot answer the no-peers control: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("no peers produced %+v", findings)
	}
	if dp.reads != 0 {
		t.Errorf("unused dependency was probed %d times", dp.reads)
	}
}
