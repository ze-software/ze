package engine

import (
	"errors"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/privilege"
)

// TestDoctorESPFormCompletePathFailure drives the check over the receiver's record of its
// last failure to open the complete path, through the dataplane registry the check reads
// (dataplane.Get), with a fake backend.
//
// VALIDATES: a recorded failure is one error under doctor-ipsec-esp-form-tun whose
// message names the failure's problem, the kernel's answer and its time, says that no
// Child SA that must receive both ESP forms can be installed or moved until the path
// opens, and whose fix is the failure's own; it is reported under every fast path status
// and when the fast path's counters cannot be read; a failure that is no operator error
// keeps its text and lists the receiver's warnings; no record reports nothing.
// PREVENTS: the receiver refusing every dual-form Child SA while doctor names no cause.
// MUTATION: in checkIPsecESPFormFastPath, drop the espFormCompletePathFindings append of
// the success branch: the attached and idle cases report nothing.
// MUTATION: in espFormCompletePathFindings, answer espFormFastPathFixLog for an operator
// error too: the fix is not the failure's own.
func TestDoctorESPFormCompletePathFailure(t *testing.T) {
	refusal := opererr.Wrap(errors.New("listen ip4:50 0.0.0.0: socket: operation not permitted"),
		"Ze has no permission to open the raw ESP socket (ip4:50) that receives bare ESP",
		"a raw socket needs CAP_NET_RAW", privilege.DaemonCapabilitiesFix)
	at := time.Date(2026, 11, 12, 9, 30, 0, 0, time.UTC)
	failed := &dataplane.ESPFormCompletePathFailure{Err: refusal, At: at}
	plain := &dataplane.ESPFormCompletePathFailure{Err: errors.New("test: a defect's error"), At: at}
	consequence := "no Child SA that must receive both ESP forms can be installed, or moved onto the ESP-in-UDP form, until it opens"
	tunCode := "error:" + diagnostic.CodeDoctorIPsecESPFormTUN
	fastPath := "warning:" + diagnostic.CodeDoctorIPsecESPFormFastPath

	for _, tc := range []struct {
		name  string
		dp    *espFormFastPathDP
		want  string
		texts []string
		fix   string
	}{
		{
			name: "attached, a failure recorded",
			dp: &espFormFastPathDP{state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathAttached, FirewallReading: dataplane.ESPFormFirewallRead,
				Attached: []string{"eth0"}, CompletePathFailure: failed,
			}},
			want:  tunCode,
			texts: []string{refusal.Problem(), "operation not permitted", "2026-11-12T09:30:00Z", consequence},
			fix:   privilege.DaemonCapabilitiesFix,
		},
		{
			name: "idle, a failure recorded",
			dp: &espFormFastPathDP{state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathIdle, CompletePathFailure: failed,
			}},
			want:  tunCode,
			texts: []string{refusal.Problem(), consequence},
			fix:   privilege.DaemonCapabilitiesFix,
		},
		{
			name: "the counters cannot be read, a failure recorded",
			dp: &espFormFastPathDP{
				state:    dataplane.ESPFormFastPathState{CompletePathFailure: failed},
				stateErr: errors.New("bad file descriptor"),
			},
			want:  fastPath + "," + tunCode,
			texts: []string{refusal.Problem(), consequence},
			fix:   privilege.DaemonCapabilitiesFix,
		},
		{
			name: "idle, a failure that is no operator error",
			dp: &espFormFastPathDP{state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathIdle, CompletePathFailure: plain,
			}},
			want:  tunCode,
			texts: []string{"test: a defect's error", consequence},
			fix:   espFormFastPathFixLog,
		},
		{
			name: "idle, nothing recorded",
			dp:   &espFormFastPathDP{state: dataplane.ESPFormFastPathState{Status: dataplane.ESPFormFastPathIdle}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			loadESPFormBackend(t, tc.dp)
			diags := checkIPsecESPFormFastPath(nil)
			if got := diagnosticCodes(diags); got != tc.want {
				t.Fatalf("diagnostics = %q, want %q (%v)", got, tc.want, diags)
			}
			if tc.texts != nil {
				assertESPFormFinding(t, diags[len(diags)-1], tc.texts, tc.fix)
			}
		})
	}
}

// TestDoctorESPFormDeviceFindingsReportOneCauseOnce drives the check with a refused TUN
// record from the last apply beside the receiver's record of its last failure to open.
//
// VALIDATES: when both carry the same cause, one doctor-ipsec-esp-form-tun error is
// reported, the receiver's, which carries its time; two causes are two errors.
// PREVENTS: one host problem read as two.
// MUTATION: in espFormSameCause, answer false: the same cause is reported twice.
func TestDoctorESPFormDeviceFindingsReportOneCauseOnce(t *testing.T) {
	const (
		tunProblem    = "the device cgroup refuses Ze access to the TUN device /dev/net/tun (c 10 200)"
		tunFix        = "in Docker: docker run --device /dev/net/tun"
		socketProblem = "Ze has no permission to open the raw ESP socket (ip4:50) that receives bare ESP"
	)
	refusal := opererr.Wrap(errors.New("open /dev/net/tun: operation not permitted"), tunProblem,
		"a container started without the TUN device can hold the node but not open it", tunFix)
	other := opererr.Wrap(errors.New("listen ip4:50 0.0.0.0: socket: operation not permitted"), socketProblem,
		"a raw socket needs CAP_NET_RAW", privilege.DaemonCapabilitiesFix)
	at := time.Date(2026, 11, 12, 9, 30, 0, 0, time.UTC)
	tunCode := "error:" + diagnostic.CodeDoctorIPsecESPFormTUN

	for _, tc := range []struct {
		name    string
		failure error
		want    string
		problem string
		fix     string
	}{
		{name: "the same cause", failure: refusal, want: tunCode, problem: tunProblem, fix: tunFix},
		{name: "two causes", failure: other, want: tunCode + "," + tunCode, problem: socketProblem, fix: privilege.DaemonCapabilitiesFix},
	} {
		t.Run(tc.name, func(t *testing.T) {
			loadESPFormBackend(t, &espFormFastPathDP{state: dataplane.ESPFormFastPathState{
				Status:              dataplane.ESPFormFastPathIdle,
				CompletePathFailure: &dataplane.ESPFormCompletePathFailure{Err: tc.failure, At: at},
			}})
			diags := checkIPsecESPFormFastPath(newESPFormTUNRecord(refusal))
			if got := diagnosticCodes(diags); got != tc.want {
				t.Fatalf("diagnostics = %q, want %q (%v)", got, tc.want, diags)
			}
			assertESPFormFinding(t, diags[len(diags)-1], []string{tc.problem, "2026-11-12T09:30:00Z"}, tc.fix)
		})
	}
}
