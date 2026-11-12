// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- a loaded ESP GRO module can consume bare ESP before either receive path
// Related: doctor_runtime.go -- the registered engine runtime check under test

//go:build linux

package engine

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// espFormOffloadModulesFile gives the real module-list reader one private fixture.
// An absent file tests an unreadable snapshot; an empty file tests no loaded module.
// The caller MUST NOT run concurrently with another test that changes this env value.
func espFormOffloadModulesFile(t *testing.T, contents string, absent bool) {
	t.Helper()
	const key = "ze.test.doctor.modules-file"
	before := env.Get(key)
	path := filepath.Join(t.TempDir(), "modules")
	if !absent {
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := env.Set(key, path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := env.Set(key, before); err != nil {
			t.Error(err)
		}
	})
}

// espFormOffloadDoctor asks the same registered runtime callback show doctor asks.
func espFormOffloadDoctor(t *testing.T) []rpc.DoctorCheckDiagnostic {
	t.Helper()
	state := &ikeEngineState{}
	findings, err := state.answerDoctorCheck("ipsec-esp-form-offload")
	if err != nil {
		t.Fatalf("the engine cannot answer its ESP offload check: %v", err)
	}
	return findings
}

// TestDoctorESPFormOffloadLoadedModuleWarns drives each receiver-active state through
// the runtime callback with a real /proc/modules-shaped fixture. It pins the module's
// name, the interception point, the tunnel outage and other-owner conditions of the
// stop/unload/start fix, without executing that fix or touching host modules.
func TestDoctorESPFormOffloadLoadedModuleWarns(t *testing.T) {
	espFormOffloadModulesFile(t, "esp4_offload 16384 2 - Live 0x0\n", false)
	for _, status := range []dataplane.ESPFormFastPathStatus{
		dataplane.ESPFormFastPathAttached,
		dataplane.ESPFormFastPathUnavailable,
		dataplane.ESPFormFastPathDetached,
	} {
		t.Run(status.String(), func(t *testing.T) {
			loadESPFormBackend(t, &espFormFastPathDP{state: dataplane.ESPFormFastPathState{Status: status}})
			findings := espFormOffloadDoctor(t)
			if len(findings) != 1 {
				t.Fatalf("findings = %v, want one loaded-offload warning", findings)
			}
			finding := findings[0]
			if finding.Code != "doctor-ipsec-esp-form-offload" || finding.Severity != "warning" {
				t.Errorf("finding = %+v, want the loaded-offload warning", finding)
			}
			for _, want := range []string{
				"esp4_offload", "can consume bare ESP", "before tc ingress and raw sockets",
				"stops all Ze tunnels", "another IPsec owner", "no hardware-offload request",
			} {
				if !strings.Contains(finding.Message, want) {
					t.Errorf("message = %q, want %q", finding.Message, want)
				}
			}
			const fix = "sudo systemctl stop ze && sudo modprobe -r esp4_offload && sudo systemctl start ze"
			if finding.Fix != fix {
				t.Errorf("fix = %q, want %q", finding.Fix, fix)
			}
		})
	}
}

// TestDoctorESPFormOffloadAbsentModuleIsNotInvented keeps an empty readable snapshot
// separate from unreadability, and proves a similar module name does not match ESP GRO.
func TestDoctorESPFormOffloadAbsentModuleIsNotInvented(t *testing.T) {
	loadESPFormBackend(t, &espFormFastPathDP{state: dataplane.ESPFormFastPathState{Status: dataplane.ESPFormFastPathAttached}})
	for _, contents := range []string{"", "esp4_offload_helper 16384 0 - Live 0x0\nesp6_offload 16384 0 - Live 0x0\n"} {
		t.Run(contents, func(t *testing.T) {
			espFormOffloadModulesFile(t, contents, false)
			if findings := espFormOffloadDoctor(t); len(findings) != 0 {
				t.Errorf("findings = %v for modules %q, want no loaded esp4_offload finding", findings, contents)
			}
		})
	}
}

// TestDoctorESPFormOffloadUnreadableModulesWarns proves a missing module list is an
// unknown result with its own code and the procfs restoration command, never absence.
func TestDoctorESPFormOffloadUnreadableModulesWarns(t *testing.T) {
	loadESPFormBackend(t, &espFormFastPathDP{state: dataplane.ESPFormFastPathState{Status: dataplane.ESPFormFastPathAttached}})
	espFormOffloadModulesFile(t, "", true)
	findings := espFormOffloadDoctor(t)
	if len(findings) != 1 {
		t.Fatalf("findings = %v, want one unreadable-module-list warning", findings)
	}
	finding := findings[0]
	if finding.Code != "doctor-ipsec-esp-form-offload-unknown" || finding.Severity != "warning" {
		t.Errorf("finding = %+v, want the unknown-module-list warning", finding)
	}
	for _, want := range []string{"cannot read", "/proc/modules", "cannot tell", "built into the kernel"} {
		if !strings.Contains(finding.Message, want) {
			t.Errorf("message = %q, want %q", finding.Message, want)
		}
	}
	if finding.Fix != "sudo mount -t proc proc /proc" {
		t.Errorf("fix = %q, want the procfs restoration command", finding.Fix)
	}
}

// TestDoctorESPFormOffloadIdleReceiverIsSilent keeps both loaded and unreadable module
// snapshots silent while no dual-form SPI is watched: no such packet needs these paths.
func TestDoctorESPFormOffloadIdleReceiverIsSilent(t *testing.T) {
	loadESPFormBackend(t, &espFormFastPathDP{state: dataplane.ESPFormFastPathState{Status: dataplane.ESPFormFastPathIdle}})
	for _, absent := range []bool{false, true} {
		espFormOffloadModulesFile(t, "esp4_offload 16384 0 - Live 0x0\n", absent)
		if findings := espFormOffloadDoctor(t); len(findings) != 0 {
			t.Errorf("findings = %v while the receiver is idle, want none", findings)
		}
	}
}

// TestDoctorESPFormOffloadUnreadableReceiverWarns proves a receiver whose state cannot
// be read is unknown even when its module snapshot could otherwise look harmless.
func TestDoctorESPFormOffloadUnreadableReceiverWarns(t *testing.T) {
	loadESPFormBackend(t, &espFormFastPathDP{stateErr: errors.New("test: counters unavailable")})
	espFormOffloadModulesFile(t, "", false)
	findings := espFormOffloadDoctor(t)
	if len(findings) != 1 || findings[0].Code != "doctor-ipsec-esp-form-offload-unknown" {
		t.Fatalf("findings = %v, want one unknown receiver-state warning", findings)
	}
	if !strings.Contains(findings[0].Message, "test: counters unavailable") {
		t.Errorf("message = %q, want the unreadable receiver's cause", findings[0].Message)
	}
	if findings[0].Fix != dataplane.ESPFormFixLog {
		t.Errorf("fix = %q, want the dataplane warning log", findings[0].Fix)
	}
}

// TestDoctorESPFormOffloadRegistered pins the declaration the engine sends at Stage 1
// and both diagnostic descriptions, so a runtime warning is accepted and explained.
func TestDoctorESPFormOffloadRegistered(t *testing.T) {
	// The binary registers these after init, in zeSetup; a Go test must perform
	// the same bootstrap before it asks Lookup to resolve a declaration's codes.
	diagnostic.RegisterBuiltinCodes()

	const name = "ipsec-esp-form-offload"
	want := []string{"doctor-ipsec-esp-form-offload", "doctor-ipsec-esp-form-offload-unknown"}
	for _, decl := range engineDoctorCheckDecls() {
		if decl.Name != name {
			continue
		}
		if decl.Phase != rpc.DoctorPhaseRuntime {
			t.Errorf("phase = %q, want runtime", decl.Phase)
		}
		if !slices.Equal(decl.Codes, want) {
			t.Errorf("codes = %v, want %v", decl.Codes, want)
		}
		for _, code := range want {
			if diagnostic.Lookup(code) == nil {
				t.Errorf("code %s has no diagnostic description", code)
			}
		}
		return
	}
	t.Fatalf("the engine declares no runtime check %s", name)
}
