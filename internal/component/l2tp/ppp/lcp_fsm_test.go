package ppp

import (
	"reflect"
	"testing"
)

// fsmCase is one row of the RFC 1661 §4.1 transition table.
type fsmCase struct {
	name        string
	state       LCPState
	event       LCPEvent
	wantState   LCPState
	wantActions []LCPAction
}

// runFSMCases walks each table entry, invokes LCPDoTransition, and
// compares the resulting state and actions.
func runFSMCases(t *testing.T, cases []fsmCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := LCPDoTransition(tc.state, tc.event)
			if got.NewState != tc.wantState {
				t.Errorf("state %s + event %d: got state %s, want %s",
					tc.state, tc.event, got.NewState, tc.wantState)
			}
			// nil and empty slice are equivalent for our purposes.
			gotActs := got.Actions
			if gotActs == nil {
				gotActs = []LCPAction{}
			}
			wantActs := tc.wantActions
			if wantActs == nil {
				wantActs = []LCPAction{}
			}
			if !reflect.DeepEqual(gotActs, wantActs) {
				t.Errorf("state %s + event %d: got actions %v, want %v",
					tc.state, tc.event, gotActs, wantActs)
			}
		})
	}
}

// VALIDATES: Happy path from Initial through Opened per RFC 1661 §4.1
//
//	for the LNS role (we receive an Up first, then an Open).
//
// PREVENTS: regressions in the canonical bring-up path.
func TestLCPFSMHappyPath(t *testing.T) {
	runFSMCases(t, []fsmCase{
		{"initial+up -> closed", LCPStateInitial, LCPEventUp, LCPStateClosed, nil},
		{"closed+open -> req-sent (irc, scr)", LCPStateClosed, LCPEventOpen, LCPStateReqSent, []LCPAction{LCPActIRC, LCPActSCR}},
		{"req-sent+rcr+ -> ack-sent (sca)", LCPStateReqSent, LCPEventRCRPlus, LCPStateAckSent, []LCPAction{LCPActSCA}},
		{"ack-sent+rca -> opened (irc, tlu)", LCPStateAckSent, LCPEventRCA, LCPStateOpened, []LCPAction{LCPActIRC, LCPActTLU}},
	})
}

// VALIDATES: Alternate happy path where RCA comes before RCR+
//
//	(Req-Sent -> Ack-Rcvd -> Opened).
func TestLCPFSMHappyPathAckFirst(t *testing.T) {
	runFSMCases(t, []fsmCase{
		{"req-sent+rca -> ack-rcvd (irc)", LCPStateReqSent, LCPEventRCA, LCPStateAckRcvd, []LCPAction{LCPActIRC}},
		{"ack-rcvd+rcr+ -> opened (sca, tlu)", LCPStateAckRcvd, LCPEventRCRPlus, LCPStateOpened, []LCPAction{LCPActSCA, LCPActTLU}},
	})
}

// VALIDATES: Retransmit handling per RFC 1661 §4.1: TO+ in ReqSent
//
//	re-sends Configure-Request; TO- terminates.
func TestLCPFSMRetransmit(t *testing.T) {
	runFSMCases(t, []fsmCase{
		{"req-sent+to+ -> req-sent (scr)", LCPStateReqSent, LCPEventTOPlus, LCPStateReqSent, []LCPAction{LCPActSCR}},
		{"ack-sent+to+ -> ack-sent (scr)", LCPStateAckSent, LCPEventTOPlus, LCPStateAckSent, []LCPAction{LCPActSCR}},
		{"ack-rcvd+to+ -> req-sent (scr)", LCPStateAckRcvd, LCPEventTOPlus, LCPStateReqSent, []LCPAction{LCPActSCR}},
		{"req-sent+to- -> stopped (tlf)", LCPStateReqSent, LCPEventTOMinus, LCPStateStopped, []LCPAction{LCPActTLF}},
		{"closing+to+ -> closing (str)", LCPStateClosing, LCPEventTOPlus, LCPStateClosing, []LCPAction{LCPActSTR}},
		{"closing+to- -> closed (tlf)", LCPStateClosing, LCPEventTOMinus, LCPStateClosed, []LCPAction{LCPActTLF}},
	})
}

// VALIDATES: Termination -- both peer-initiated (RTR in Opened) and
//
//	local-initiated (Close in Opened).
func TestLCPFSMTerminate(t *testing.T) {
	runFSMCases(t, []fsmCase{
		// Peer-initiated: RTR in Opened.
		{"opened+rtr -> stopping (tld, zrc, sta)", LCPStateOpened, LCPEventRTR, LCPStateStopping, []LCPAction{LCPActTLD, LCPActZRC, LCPActSTA}},
		// Local-initiated: Close in Opened.
		{"opened+close -> closing (tld, irc, str)", LCPStateOpened, LCPEventClose, LCPStateClosing, []LCPAction{LCPActTLD, LCPActIRC, LCPActSTR}},
		// Closing terminates on RTA.
		{"closing+rta -> closed (tlf)", LCPStateClosing, LCPEventRTA, LCPStateClosed, []LCPAction{LCPActTLF}},
		// Stopping terminates on RTA.
		{"stopping+rta -> stopped (tlf)", LCPStateStopping, LCPEventRTA, LCPStateStopped, []LCPAction{LCPActTLF}},
	})
}

// VALIDATES: Unknown code (RUC) triggers Send-Code-Reject in every
//
//	state where an Open exists or we're operational, per
//	RFC 1661 §4.1.
func TestLCPFSMCodeReject(t *testing.T) {
	runFSMCases(t, []fsmCase{
		{"closed+ruc -> closed (scj)", LCPStateClosed, LCPEventRUC, LCPStateClosed, []LCPAction{LCPActSCJ}},
		{"stopped+ruc -> stopped (scj)", LCPStateStopped, LCPEventRUC, LCPStateStopped, []LCPAction{LCPActSCJ}},
		{"req-sent+ruc -> req-sent (scj)", LCPStateReqSent, LCPEventRUC, LCPStateReqSent, []LCPAction{LCPActSCJ}},
		{"ack-rcvd+ruc -> ack-rcvd (scj)", LCPStateAckRcvd, LCPEventRUC, LCPStateAckRcvd, []LCPAction{LCPActSCJ}},
		{"ack-sent+ruc -> ack-sent (scj)", LCPStateAckSent, LCPEventRUC, LCPStateAckSent, []LCPAction{LCPActSCJ}},
		{"opened+ruc -> opened (scj)", LCPStateOpened, LCPEventRUC, LCPStateOpened, []LCPAction{LCPActSCJ}},
	})
}

// VALIDATES: Echo handling -- RXR (echo-request etc.) in Opened
//
//	triggers Send-Echo-Reply.
func TestLCPFSMRXRInOpened(t *testing.T) {
	runFSMCases(t, []fsmCase{
		{"opened+rxr -> opened (ser)", LCPStateOpened, LCPEventRXR, LCPStateOpened, []LCPAction{LCPActSER}},
		// In other states, RXR is silently ignored (RFC 1661 §4.1).
		{"req-sent+rxr -> req-sent (no-op)", LCPStateReqSent, LCPEventRXR, LCPStateReqSent, nil},
	})
}

// VALIDATES: Renegotiation -- RCR+ in Opened triggers TLD then a fresh
//
//	Configure-Request and Configure-Ack.
func TestLCPFSMRenegotiateInOpened(t *testing.T) {
	runFSMCases(t, []fsmCase{
		{"opened+rcr+ -> ack-sent (tld, scr, sca)", LCPStateOpened, LCPEventRCRPlus, LCPStateAckSent, []LCPAction{LCPActTLD, LCPActSCR, LCPActSCA}},
		{"opened+rcr- -> req-sent (tld, scr, scn)", LCPStateOpened, LCPEventRCRMinus, LCPStateReqSent, []LCPAction{LCPActTLD, LCPActSCR, LCPActSCN}},
	})
}

// VALIDATES: Stop name + invalid event in unrelated state is a no-op
//
//	(state unchanged, no actions).
func TestLCPFSMNoOpEvents(t *testing.T) {
	runFSMCases(t, []fsmCase{
		{"initial+rca -> initial (no-op)", LCPStateInitial, LCPEventRCA, LCPStateInitial, nil},
		{"starting+rcr+ -> starting (no-op)", LCPStateStarting, LCPEventRCRPlus, LCPStateStarting, nil},
	})
}

// VALIDATES: State and Action stringification for log output.
func TestLCPStateString(t *testing.T) {
	cases := []struct {
		s    LCPState
		want string
	}{
		{LCPStateInitial, "initial"},
		{LCPStateOpened, "opened"},
		{LCPState(99), "unknown"},
	}
	for _, tc := range cases {
		if tc.s.String() != tc.want {
			t.Errorf("LCPState(%d).String() = %q, want %q", tc.s, tc.s.String(), tc.want)
		}
	}
}

func TestLCPActionString(t *testing.T) {
	cases := []struct {
		a    LCPAction
		want string
	}{
		{LCPActTLU, "tlu"},
		{LCPActSCR, "scr"},
		{LCPActSER, "ser"},
		{LCPAction(99), "?"},
	}
	for _, tc := range cases {
		if tc.a.String() != tc.want {
			t.Errorf("LCPAction(%d).String() = %q, want %q", tc.a, tc.a.String(), tc.want)
		}
	}
}
