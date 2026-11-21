// Design: docs/architecture/doctor-and-health-checks.md -- the runtime phase: checks only the daemon runs
// Related: doctor_ike_socket_bypass.go -- answerIKESocketBypass, ikeSocketBypassFailures and checkIKESocketBypass, under test

package engine

import (
	"cmp"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// VALIDATES: an IKE socket whose per-socket bypass the kernel refused is reported by
// ipsec-ike-socket-bypass as an error naming the socket, its role and the kernel's
// answer, with the capability-and-restart fix; a socket that carries its bypass is not.
// METHOD: judge two refused sockets, one of them NAT-T, and read each finding.
// PREVENTS: a refused bypass living only in a Warn log line, while the SPD can capture
// Ze's own IKE.
func TestIKESocketBypassRefusalIsReportedByDoctor(t *testing.T) {
	findings := checkIKESocketBypass([]ikeSocketBypassFailure{
		{address: "192.0.2.1:500", cause: "setsockopt IP_XFRM_POLICY: operation not permitted"},
		{address: "0.0.0.0:4500", natT: true, cause: "setsockopt IP_XFRM_POLICY: protocol not available"},
	})
	if len(findings) != 2 {
		t.Fatalf("the check answered %d findings, want 2: %+v", len(findings), findings)
	}
	for i, want := range []struct{ socket, address, cause string }{
		{"The IKE socket", "192.0.2.1:500", "operation not permitted"},
		{"The NAT-T IKE socket", "0.0.0.0:4500", "protocol not available"},
	} {
		f := findings[i]
		if f.Code != diagnostic.CodeDoctorIPsecIKESocketBypass || f.Severity != severityError {
			t.Fatalf("finding %d is %s/%s, want %s/%s", i, f.Severity, f.Code, severityError, diagnostic.CodeDoctorIPsecIKESocketBypass)
		}
		if !strings.HasPrefix(f.Message, want.socket+" at "+want.address) || !strings.Contains(f.Message, want.cause) {
			t.Fatalf("finding %d does not name the socket, its address and the kernel's answer: %q", i, f.Message)
		}
		if f.Fix != ikeSocketBypassFix {
			t.Fatalf("finding %d's fix is %q, want %q", i, f.Fix, ikeSocketBypassFix)
		}
	}
	if findings := checkIKESocketBypass(nil); len(findings) != 0 {
		t.Fatalf("the check reports sockets that carry their bypass: %+v", findings)
	}
}

// VALIDATES: the check reads the outcome each socket the engine holds recorded, each
// socket once: a shared NAT-T socket a pair also holds is not reported twice.
// METHOD: open a real socket and hold it as the primary pair's NAT-T member and as the
// shared NAT-T socket, then compare the check with the transport's own record. The
// record differs by platform and privilege (no kernel SPD off Linux, no CAP_NET_ADMIN
// in an unprivileged run), so the test asserts agreement, never one outcome.
// PREVENTS: the check reading a socket the engine does not hold, or missing one.
func TestIKESocketBypassReadsEverySocketOnce(t *testing.T) {
	natt, err := transport.NewNATTTransport("127.0.0.1:0", slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("open the NAT-T socket: %v", err)
	}
	state := applyTestState(t) // its cleanup closes the socket (closeAllListeners)
	state.listeners.natt = natt
	state.listeners.nattShared = true
	state.sharedNATT = natt

	failures := state.ikeSocketBypassFailures()
	refused := natt.BypassError()
	if refused == nil {
		if len(failures) != 0 {
			t.Fatalf("the check reports a socket that carries its bypass: %+v", failures)
		}
		return
	}
	if len(failures) != 1 {
		t.Fatalf("the check found %d refused sockets, want the one socket once: %+v", len(failures), failures)
	}
	if !failures[0].natT || failures[0].address != natt.LocalAddr().String() || failures[0].cause != refused.Error() {
		t.Fatalf("the check read %+v, want the NAT-T socket at %s refused with %q", failures[0], natt.LocalAddr(), refused)
	}
}

// VALIDATES: every socket the engine holds is read once and reported once, whatever
// the host: a shared NAT-T socket a pair also holds is one finding, and the primary
// IKE socket is a finding of its own.
// METHOD: hold one NAT-T socket as the primary pair's member and as the shared socket,
// beside a primary IKE socket, and read them through a record that refuses every
// socket, counting each read. TestIKESocketBypassReadsEverySocketOnce reads the real
// record, which refuses nothing on a privileged host, where the dedupe goes unjudged.
// PREVENTS: a shared socket reported twice, or one the engine holds never read.
func TestIKESocketBypassRefusedEverywhereIsReportedOncePerSocket(t *testing.T) {
	ike, err := transport.NewUDPTransport("127.0.0.1:0", slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("open the IKE socket: %v", err)
	}
	natt, err := transport.NewNATTTransport("127.0.0.1:0", slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("open the NAT-T socket: %v", err)
	}
	state := applyTestState(t) // its cleanup closes both sockets (closeAllListeners)
	state.listeners.ike = ike
	state.listeners.natt = natt
	state.listeners.nattShared = true
	state.sharedNATT = natt

	reads := make(map[*transport.UDPTransport]int)
	refusal := "setsockopt IP_XFRM_POLICY: operation not permitted"
	failures := state.ikeSocketBypassFailuresRead(func(socket *transport.UDPTransport) error {
		reads[socket]++
		return errors.New(refusal)
	})
	if len(reads) != 2 || reads[ike] != 1 || reads[natt] != 1 {
		t.Fatalf("the check read the sockets %v, want the IKE and the NAT-T socket once each", reads)
	}
	want := []ikeSocketBypassFailure{
		{address: ike.LocalAddr().String(), cause: refusal},
		{address: natt.LocalAddr().String(), natT: true, cause: refusal},
	}
	slices.SortFunc(want, func(a, b ikeSocketBypassFailure) int { return cmp.Compare(a.address, b.address) })
	if !slices.Equal(failures, want) {
		t.Fatalf("the check found %+v, want each held socket once %+v", failures, want)
	}
}
