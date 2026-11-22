// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- complete-path failure findings
// Related: doctor_runtime.go -- the engine runtime doctor callback
// Related: apply.go -- configuration apply and the TUN outcome it records

package engine

import (
	"errors"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

// espFormRetainedFailureDP keeps the receiver's last open failure across applies, as
// espFormReceiver.fastPathState does until a later open succeeds. Its counter read may
// fail independently of that retained record.
type espFormRetainedFailureDP struct {
	espFormCatchAllDP
	readErr error
}

func (d *espFormRetainedFailureDP) ESPFormFastPathState() (dataplane.ESPFormFastPathState, error) {
	return d.state, d.readErr
}

// TestDoctorESPFormPeerRemovalHidesRetainedFailure drives configuration apply and the
// engine's runtime doctor dispatch with a receiver whose last open failed.
//
// VALIDATES: a configured peer reports the retained refusal even before any session or
// Child SA starts; removing all peers silences the TUN finding without deleting the
// receiver's evidence; adding a peer reports the same failure again. An unread counter
// remains its own warning throughout. The retained vpn ipsec unmatched bypass keeps the
// vpn root configured, so the engine is not unloaded when its last peer is removed.
// PREVENTS: a stale TUN error after the last peer is removed, and suppressing an idle
// configured peer's real refusal or an unrelated counter-read warning.
// MUTATION: report the retained receiver failure regardless of the applied peer set:
// the removed-peer step still reports doctor-ipsec-esp-form-tun.
func TestDoctorESPFormPeerRemovalHidesRetainedFailure(t *testing.T) {
	for _, tc := range []struct {
		name    string
		readErr error
		warning string
	}{
		{name: "idle receiver"},
		{
			name:    "unread counters",
			readErr: errors.New("test: fast-path counters cannot be read"),
			warning: "warning:" + dataplane.CodeDoctorIPsecESPFormFastPath,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			refusal := espFormTUNTestRefusal()
			failed := &dataplane.ESPFormCompletePathFailure{
				Err: refusal,
				At:  time.Date(2026, 11, 15, 19, 0, 0, 0, time.UTC),
			}
			dp := &espFormRetainedFailureDP{
				state: dataplane.ESPFormFastPathState{
					Status:              dataplane.ESPFormFastPathIdle,
					CompletePathFailure: failed,
				},
				readErr: tc.readErr,
			}
			loadESPFormBackend(t, dp)
			state := applyTestState(t)
			calls := stubESPFormTUN(t, nil)
			stubESPFormFastPathReady(t, dataplane.ESPFormFastPathFaultUnspecified, nil)
			configured := testIPsecConfig(applyTestPeer("127.0.0.1"))
			wantFailure := "error:" + dataplane.CodeDoctorIPsecESPFormTUN
			if tc.warning != "" {
				wantFailure = tc.warning + "," + wantFailure
			}

			for _, step := range []struct {
				name  string
				phase applyPhase
				peers bool
				want  string
			}{
				{name: "configured before sessions start", phase: applyStartup, peers: true, want: wantFailure},
				{name: "all peers removed", phase: applyReload, want: tc.warning},
				{name: "peer configured again", phase: applyStartup, peers: true, want: wantFailure},
			} {
				cfg := testIPsecConfig()
				if step.peers {
					cfg = configured
				}
				if err := state.applyConfig(cfg, step.phase); err != nil {
					t.Fatalf("%s: applyConfig = %v", step.name, err)
				}
				if len(state.peers) != 0 {
					t.Fatalf("%s: started %d sessions, want none", step.name, len(state.peers))
				}
				diags, err := state.answerDoctorCheck(espFormFastPathCheckName)
				if err != nil {
					t.Fatalf("%s: answerDoctorCheck = %v", step.name, err)
				}
				if got := diagnosticCodes(diags); got != step.want {
					t.Errorf("%s: diagnostics = %q, want %q (%v)", step.name, got, step.want, diags)
				}
				if dp.state.CompletePathFailure != failed {
					t.Fatalf("%s: the receiver's failure evidence was cleared", step.name)
				}
			}
			if *calls != 2 {
				t.Fatalf("TUN probe ran %d times, want only the two configured applies", *calls)
			}
		})
	}
}
