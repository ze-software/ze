// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- one Child SA receives both ESP forms
// Related: doctor.go -- the sibling ike readiness checks
// Related: doctor_runtime.go -- engineDoctorChecks, where this runtime check is declared and dispatched
// RFC: rfc/short/rfc7296.md -- receive both ESP forms at any time (Section 2.23)

package engine

import (
	"errors"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/privilege"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// espFormFastPathCheckName names the check in the engine's Stage 1 registration and in
// the doctor callback's dispatch.
const espFormFastPathCheckName = "ipsec-esp-form-fast-path"

// espFormFastPathCheckDecl declares the check at the runtime phase in the engine's
// Stage 1 registration (runEngine, register.go), so show doctor asks the engine's own
// process for it, internal and forked alike. The dataplane and its dual-form receiver
// live only there: a registry.DoctorCheckDef would run in whichever process answers
// doctor, where dataplane.Get() is nil (ai/rules/plugins.md).
var espFormFastPathCheckDecl = rpc.DoctorCheckDecl{
	Name:  espFormFastPathCheckName,
	Phase: rpc.DoctorPhaseRuntime,
	Order: 742,
	Codes: []string{
		diagnostic.CodeDoctorIPsecESPFormFastPath,
		diagnostic.CodeDoctorIPsecESPFormLink,
		diagnostic.CodeDoctorIPsecESPFormTUN,
		diagnostic.CodeDoctorIPsecESPFormFirewall,
	},
}

// espFormTUNAvailable opens and closes the TUN device the complete path writes into. It
// is a variable so a test answers it without the host's /dev/net/tun.
//
// It is not a read: where /dev/net/tun is missing and the kernel allows it, it creates
// the node (c 10 200, mode 0600), exactly as the receiver's first Watch would
// (openESPFormTUNClone, dataplane/espform_tun_linux.go). So only the engine calls it, in
// its own process, the one that would create the node anyway, and only from the apply
// path (recordESPFormTUN, apply.go). The doctor check reads the outcome the apply
// recorded, because a runtime doctor check only reads.
var espFormTUNAvailable = dataplane.ESPFormTUNAvailable

// espFormTUNStatus is the outcome of the engine's last attempt to open the TUN clone
// device. The zero value names no outcome: only newESPFormTUNRecord builds a record, so
// a record holding it is a Ze defect, and the check reports it as unread.
type espFormTUNStatus uint8

const (
	espFormTUNStatusUnspecified espFormTUNStatus = iota
	// espFormTUNOpened: the device opened, so the receiver can create ze-espform.
	espFormTUNOpened
	// espFormTUNUnsupported: the platform has no TUN device for the receiver
	// (dataplane.ErrNotSupported), and no XFRM backend either, so nothing is refused.
	espFormTUNUnsupported
	// espFormTUNRefused: the device did not open, so every Child SA on UDP port 4500 is
	// refused at install.
	espFormTUNRefused
)

// espFormTUNRecord is the outcome of one attempt, with the error a refusal carries. It
// is never changed once stored (ikeEngineState.espFormTUN), so the doctor callback reads
// it with no lock.
type espFormTUNRecord struct {
	status espFormTUNStatus
	err    error
}

// newESPFormTUNRecord turns the answer of espFormTUNAvailable into a record.
func newESPFormTUNRecord(err error) *espFormTUNRecord {
	if err == nil {
		return &espFormTUNRecord{status: espFormTUNOpened}
	}
	if errors.Is(err, dataplane.ErrNotSupported) {
		return &espFormTUNRecord{status: espFormTUNUnsupported, err: err}
	}
	return &espFormTUNRecord{status: espFormTUNRefused, err: err}
}

// espFormDevice is the device the complete path writes into, named in every message
// because a firewall rule keyed on the input interface sees it. The dataplane declares
// it for Linux alone (espFormTUNName), and this check builds on every platform.
const espFormDevice = "ze-espform"

// espFormNoCause stands in a finding's message for a cause the dataplane left empty,
// which only a Ze defect produces, so the message never shows empty parentheses.
const espFormNoCause = "the dataplane gave no cause"

// Fix lines of the fast path findings. Each is the command or setting alone: the
// message says what it does.
const (
	// espFormFastPathFixCapabilities stops ze, which the install needs because it
	// refuses while the daemon holds the store, rewrites the ze unit with the
	// capabilities the current unit grants, CAP_BPF included
	// (internal/plugins/systemd/unit.go), runs systemctl daemon-reload, and starts ze
	// again, which loads the fast path again.
	espFormFastPathFixCapabilities = privilege.DaemonCapabilitiesFix
	// espFormFastPathFixRestart restarts ze, whose next dual-form Child SA follows
	// link creation again, and whose fast path reads the nftables ruleset and joins
	// the nftables monitor again. The dataplane's warnings name the same command.
	espFormFastPathFixRestart = dataplane.ESPFormFixRestart
	// espFormFastPathPeerEncap is the strongSwan connection setting that makes the peer
	// send ESP-in-UDP, the form the kernel state receives on every link. It is a step
	// taken on the peer, so the link finding carries it in its message, with the
	// command that loads it there, and never as its fix line, which holds a command
	// run on this box (espFormLinkFix).
	espFormFastPathPeerEncap = "encap = yes"
	// espFormFastPathPeerLoad is the strongSwan command that loads the peer's changed
	// connection setting.
	espFormFastPathPeerLoad = "swanctl --load-conns"
	// espFormFastPathFixLinks lists every interface, the fix line for a
	// refused link whose name cannot be pasted as one CLI word.
	espFormFastPathFixLinks = "ze show interface"
	// espFormFastPathFixLog lists the dual-form receiver's own warnings.
	espFormFastPathFixLog = dataplane.ESPFormFixLog
	// espFormFastPathFixEngineLog lists the engine's own warnings, among them the one
	// runEngine logs when the dataplane fails to load.
	espFormFastPathFixEngineLog = "ze show log recent level warn component ike"
	// espFormTUNFixLog lists the dual-form receiver's warnings, for a TUN probe error
	// that carries no fix of its own.
	espFormTUNFixLog = espFormFastPathFixLog
	// espFormSharedSPIFixPeer, followed by the peer's name, tears down that peer's IKE
	// SA and its Child SA and starts them again (handleClearIPsecSAPeer, ike/cmd/ipsec.go,
	// calling TerminatePeerSA, register.go), so the new Child SA draws a new random
	// inbound SPI (generateESPSPI, child.go). Ze has no command that rekeys one Child SA
	// alone, so clearing its peer is the command that gives it a new SPI.
	espFormSharedSPIFixPeer = dataplane.ESPFormFixClearPeer
	// espFormSharedSPIFixPeers lists every peer with its Child SA's inbound SPI, the fix
	// line for a shared SPI whose peer Ze cannot name, or whose name cannot be pasted
	// as one CLI word.
	espFormSharedSPIFixPeers = dataplane.ESPFormFixListSAs
)

// checkIPsecESPFormFastPath reports what keeps bare ESP off the fast path.
//
// RFC 7296 Section 2.23: "all devices MUST be able to receive and process both
// UDP-encapsulated ESP and non-UDP-encapsulated ESP packets at any time."
//
// A Child SA on UDP port 4500 meets that through two paths: the tc-ingress fast path
// rewrites bare ESP in the kernel on the link it arrived on, and the complete path
// re-presents what the fast path left through ze-espform, rate-bounded. The complete
// path drops what arrives above its bound, so each finding says what it costs while it
// carries the traffic. The fast path unavailable, or detached after a refused gate
// shut, is one warning, and the only one about the fast path: no program reads the
// gate, so how the firewall reads decides nothing.
// Under Attached, a fast path the firewall keeps off is one warning, a link the fast
// path does not serve is one warning each, a link subscription that ended is one
// more, and a Child SA the fast path does not serve because another on a different
// local address drew the same inbound SPI is one warning each (espFormSharedSPI). Idle,
// nothing watched, reports nothing about the fast path.
//
// The complete path needs the TUN device, and an SA that cannot receive both forms is
// refused at install, so a device Ze cannot open refuses every Child SA on UDP port 4500.
// That is one error, read from tun, the outcome of the open the last apply made while a
// peer was configured (recordESPFormTUN, apply.go): every IKE SA may float to UDP port
// 4500. The check never opens the device itself, because that open may create the node.
//
// The check runs in the engine's process (espFormFastPathCheckDecl), so the dataplane it
// reads is the one runEngine loaded.
func checkIPsecESPFormFastPath(tun *espFormTUNRecord) []rpc.DoctorCheckDiagnostic {
	dp := dataplane.Get()
	// In the engine's process a nil dataplane means dataplane.Load failed (runEngine logs
	// it), so no Child SA installs and nothing can say which path would carry bare ESP.
	if dp == nil {
		return []rpc.DoctorCheckDiagnostic{espFormFastPathUnread(
			"the IPsec dataplane did not load, so no Child SA is installed", espFormFastPathFixEngineLog)}
	}
	reporter, ok := dp.(dataplane.ESPFormFastPathReporter)
	// refuse-by-default: guards dataplane.ESPFormFastPathReporter -- a loaded backend without it (VPP) has no dual-form receiver, so it has neither a fast path nor a complete path to judge
	if !ok {
		return nil
	}
	state, err := reporter.ESPFormFastPathState()
	// A backend that answers ErrNotSupported holds no fast path to read, as the override
	// guard check treats the same answer.
	if errors.Is(err, dataplane.ErrNotSupported) {
		return nil
	}
	if err != nil {
		var why textbuf.Buffer
		unread := espFormFastPathUnread(why.Str("reading it failed: ").Err(err).String(), espFormFastPathFixLog)
		// The state is unknown, so whether an SPI is watched is too: the recorded
		// outcome alone decides. The receiver's own record is read whatever the fast
		// path's counters answered.
		return append([]rpc.DoctorCheckDiagnostic{unread}, espFormDeviceFindings(tun, false, state.CompletePathFailure)...)
	}
	out, watching := espFormFastPathFindings(&state)
	return append(out, espFormDeviceFindings(tun, watching, state.CompletePathFailure)...)
}

// espFormDeviceFindings answers the findings under doctor-ipsec-esp-form-tun: the TUN
// device's, from the outcome the last apply recorded (espFormTUNFindings), then the
// receiver's last failure to open its complete path (espFormCompletePathFindings). When
// both carry the same cause (espFormSameCause), the receiver's finding alone is reported:
// it is the later of the two, it carries its time, and two errors for one cause would read
// as two problems.
func espFormDeviceFindings(tun *espFormTUNRecord, watching bool, failure *dataplane.ESPFormCompletePathFailure) []rpc.DoctorCheckDiagnostic {
	var out []rpc.DoctorCheckDiagnostic
	if !espFormSameCause(tun, failure) {
		out = espFormTUNFindings(tun, watching)
	}
	return append(out, espFormCompletePathFindings(failure)...)
}

// espFormSameCause reports whether the TUN record the last apply left and the receiver's
// last failure to open are one refusal: the apply's probe and the receiver's open both
// build theirs from the same refusals (openESPFormTUN, ESPFormTUNAvailable), so one cause
// gives one text.
func espFormSameCause(tun *espFormTUNRecord, failure *dataplane.ESPFormCompletePathFailure) bool {
	if tun != nil && failure != nil && tun.status == espFormTUNRefused {
		return tun.err.Error() == failure.Err.Error()
	}
	return false
}

// espFormCompletePathFindings answers the finding for the receiver's last failure to open
// its complete path (dataplane.ESPFormCompletePathFailure), read from the receiver's own
// record, never by opening anything: nothing when the last open succeeded, one error
// otherwise.
//
// It is reported under doctor-ipsec-esp-form-tun, not the fast path's code, because its
// subject is the complete path, the raw ESP socket and the ze-espform device the TUN
// finding names, and its consequence is that finding's: every Child SA that must receive
// both ESP forms is refused, at install and at a MOBIKE move onto the ESP-in-UDP form. A
// fast path finding never refuses an SA; it reports what the complete path then carries.
// The operator reads the complete path as Ze's own receiver for bare ESP.
func espFormCompletePathFindings(failure *dataplane.ESPFormCompletePathFailure) []rpc.DoctorCheckDiagnostic {
	if failure == nil {
		return nil
	}
	var tb textbuf.Buffer
	tb.Str("Ze could not open its own receiver for bare ESP, its raw ESP socket and the ").Str(espFormDevice).
		Str(" interface, at its last attempt at ").Str(failure.At.UTC().Format(time.RFC3339)).Str(" (")
	fix := espFormFastPathFixLog
	if refusal, ok := errors.AsType[*opererr.Error](failure.Err); ok {
		tb.Str(refusal.Problem())
		if cause := errors.Unwrap(refusal); cause != nil {
			tb.Str(": ").Err(cause)
		}
		fix = refusal.Fix()
	} else {
		tb.Err(failure.Err)
	}
	tb.Str("), so no Child SA that must receive both ESP forms can be installed, or moved onto the " +
		"ESP-in-UDP form, until it opens. A Child SA installed before receives its bare ESP only where the " +
		"fast path rewrites it, and Ze opens the receiver again by itself, at most ").
		Int(int64(dataplane.ESPFormCompletePathReopenMax / time.Second)).
		Str(" seconds apart, while such an SA is installed.")
	return []rpc.DoctorCheckDiagnostic{{
		Code:     diagnostic.CodeDoctorIPsecESPFormTUN,
		Severity: severityError,
		Message:  tb.String(),
		Fix:      fix,
	}}
}

// espFormFastPathFindings answers the fast path's findings for state, and whether the
// receiver watches any SPI: under Attached, Unavailable and Detached it does, and
// under Idle it does not. A status nothing set is a finding, and says nothing about
// watching.
//
// Unavailable and Detached each report their own finding alone. No program is
// attached, so nothing reads the firewall gate: bare ESP takes the complete path
// whatever the firewall reading says, and a firewall finding would describe a gate
// nothing reads. It would also contradict the finding's fix, the unit and capability
// install for Unavailable and a restart for Detached, where the Moving and Rewatching
// findings say no restart is needed for the firewall reading.
func espFormFastPathFindings(state *dataplane.ESPFormFastPathState) ([]rpc.DoctorCheckDiagnostic, bool) {
	switch state.Status {
	case dataplane.ESPFormFastPathIdle:
		return nil, false
	case dataplane.ESPFormFastPathUnavailable:
		return []rpc.DoctorCheckDiagnostic{espFormFastPathUnavailable(state.Cause)}, true
	case dataplane.ESPFormFastPathDetached:
		return []rpc.DoctorCheckDiagnostic{espFormFastPathDetached(state.Cause)}, true
	case dataplane.ESPFormFastPathAttached:
		return append(espFormFirewallFindings(state), espFormFastPathAttached(state)...), true
	case dataplane.ESPFormFastPathUnspecified:
	}
	var why textbuf.Buffer
	return []rpc.DoctorCheckDiagnostic{espFormFastPathUnread(
		why.Str("it reported the status ").Str(state.Status.String()).String(), espFormFastPathFixLog)}, false
}

// espFormTUNFindings answers the TUN device's finding from tun, the outcome the last apply
// recorded, and watching, whether the receiver watches an SPI now.
//   - watching: the receiver opened the device on its first Watch (espFormReceiver,
//     dataplane/espform_linux.go), so the device could be created, whatever an earlier
//     apply recorded. Nothing. The device can still be lost later: a reader that stopped
//     and could not open again leaves an SPI watched with no device, and the receiver's
//     own record reports that case (espFormCompletePathFindings).
//   - nil: no apply attempted the open, because no peer is configured or the loaded
//     dataplane has no dual-form receiver. Nothing.
//   - opened, or a platform with no TUN support (ErrNotSupported, which has no XFRM
//     backend either): nothing.
//   - refused: one error with the cause and its fix.
//   - a record holding no outcome: the state could not be read, one warning.
func espFormTUNFindings(tun *espFormTUNRecord, watching bool) []rpc.DoctorCheckDiagnostic {
	if watching {
		return nil
	}
	// refuse-by-default: guards recordESPFormTUN -- a nil record is the no-attempt state it leaves when no peer is configured or the dataplane has no dual-form receiver, so no device is owed
	if tun == nil {
		return nil
	}
	switch tun.status {
	case espFormTUNOpened, espFormTUNUnsupported:
		return nil
	case espFormTUNRefused:
		return []rpc.DoctorCheckDiagnostic{espFormTUNUnavailable(tun.err)}
	case espFormTUNStatusUnspecified:
	}
	return []rpc.DoctorCheckDiagnostic{espFormTUNUnread()}
}

// espFormTUNUnread is the warning for a recorded outcome that names none, which only a
// Ze defect produces: the check cannot say whether Child SAs on UDP port 4500 install.
func espFormTUNUnread() rpc.DoctorCheckDiagnostic {
	return rpc.DoctorCheckDiagnostic{
		Code:     diagnostic.CodeDoctorIPsecESPFormTUN,
		Severity: severityWarning,
		Message: "Ze could not read the outcome of its last attempt to open the TUN device /dev/net/tun " +
			"(the record names no outcome, which is a Ze defect), so it cannot tell whether Child SAs on " +
			"UDP port 4500 can be installed. The warnings the command below lists name the cause.",
		Fix: espFormFastPathFixEngineLog,
	}
}

// espFormTUNUnavailable is the error for a ze-espform interface Ze cannot create, or
// could create only with settings that defeat it, as the last apply's attempt found
// (recordESPFormTUN, apply.go): the TUN clone device /dev/net/tun does not open, or, on a
// read-only /proc/sys, the interface would skip the inbound policy check
// (disable_policy) or apply strict reverse-path filtering (rp_filter), which
// dataplane.ESPFormTUNAvailable predicts from conf/default and conf/all without writing.
// The recorded error is an operator error naming the cause and the fix; one that is not
// keeps its text, and the fix lists the receiver's warnings.
func espFormTUNUnavailable(err error) rpc.DoctorCheckDiagnostic {
	var tb textbuf.Buffer
	tb.Str("Ze cannot create a usable ").Str(espFormDevice).Str(" interface for its own receiver for bare ESP to write into (")
	fix := espFormTUNFixLog
	if refusal, ok := errors.AsType[*opererr.Error](err); ok {
		tb.Str(refusal.Problem())
		if cause := errors.Unwrap(refusal); cause != nil {
			tb.Str(": ").Err(cause)
		}
		fix = refusal.Fix()
	} else {
		tb.Err(err)
	}
	tb.Str("), so every Child SA on UDP port 4500 is refused at install. Such an SA must " +
		"receive bare ESP as well as ESP-in-UDP, and the bare ESP the fast path cannot rewrite " +
		"(fragments, IPv4 options, a link it does not serve) reaches the SA only through ").
		Str(espFormDevice).Str(", so Ze installs none rather than one that receives a single ESP form.")
	return rpc.DoctorCheckDiagnostic{
		Code:     diagnostic.CodeDoctorIPsecESPFormTUN,
		Severity: severityError,
		Message:  tb.String(),
		Fix:      fix,
	}
}

// espFormFastPathAttached answers the findings of an attached fast path: one per link
// it does not serve, one when it stopped following link creation, and one per SA it
// does not serve because another SA holds its SPI in the fast path's map.
func espFormFastPathAttached(state *dataplane.ESPFormFastPathState) []rpc.DoctorCheckDiagnostic {
	var out []rpc.DoctorCheckDiagnostic
	if state.Cause != "" {
		out = append(out, espFormFastPathStoppedFollowing(state))
	}
	for i := range state.Refused {
		out = append(out, espFormLinkRefused(&state.Refused[i]))
	}
	for i := range state.SharedSPIs {
		out = append(out, espFormSharedSPI(&state.SharedSPIs[i]))
	}
	return out
}

// espFormSharedSPI is the warning for a Child SA whose bare ESP takes the complete path
// because a Child SA on another local address drew the same random inbound SPI
// (dataplane.ESPFormSharedSPI). The fix gives the unserved SA a new SPI by clearing its
// peer, which costs that peer's tunnel the time a new IKE SA takes to come up; the SA
// the fast path serves keeps working. A peer Ze cannot name, or one whose name cannot be
// pasted as one CLI word, gets the peer listing as its fix line. That listing shows each
// peer's inbound SPI but no local address, so the message says that clearing either peer
// whose Child SA carries the SPI ends the collision: the cleared peer's new Child SA
// draws a new SPI, and the receiver's Forget of its old one hands the fast path to the
// other (espFormReceiver.Forget).
func espFormSharedSPI(shared *dataplane.ESPFormSharedSPI) rpc.DoctorCheckDiagnostic {
	var tb textbuf.Buffer
	tb.Str("Two Child SAs on different local addresses drew the same random inbound SPI ").
		Uint32(shared.SPI).Str(", one on ").Str(shared.Served.String()).
		Str(" and one on ").Str(shared.Unserved.String()).
		Str(". The kernel serves both, but the IPsec ESP form fast path serves one SA per SPI, " +
			"the one on ").Str(shared.Served.String()).
		Str(", so bare ESP for the SA on ").Str(shared.Unserved.String()).
		Str(" takes the complete path. ")
	espFormCompletePathCost(&tb, "the link it arrived on")
	tb.Str(" A new SPI for the SA on ").Str(shared.Unserved.String()).Str(" ends this. ")
	peer, found := espFormSharedSPIPeer(shared.SPI, shared.Unserved)
	if found && espFormOneWord(peer) {
		tb.Str("The command below clears the SAs of peer ").Str(peer).
			Str(", which holds it, and negotiates them again, so the new Child SA draws a new SPI. " +
				"That peer's tunnel carries no traffic until its new IKE SA and Child SA are up.")
		return rpc.DoctorCheckDiagnostic{
			Code:     diagnostic.CodeDoctorIPsecESPFormFastPath,
			Severity: severityWarning,
			Message:  tb.String(),
			Fix:      espFormSharedSPIFixPeer + peer,
		}
	}
	if found {
		tb.Str("Peer ").Str(peer).Str(" holds it, but its name cannot be typed as one command word.")
	} else {
		tb.Str("Ze could not tell which peer holds it: no peer's current or pending Child SA carries " +
			"that SPI to that address, which happens while a rekey replaces it.")
	}
	tb.Str(" The command below lists each peer with its inbound SPI, but not its local address. Run ").
		Str(espFormSharedSPIFixPeer).Str("<name> for either peer whose Child SA carries SPI ").Uint32(shared.SPI).
		Str(": clearing either one ends the collision, because its new Child SA draws a new SPI and the " +
			"other SA then holds the SPI alone. The cleared peer's tunnel carries no traffic until its " +
			"new SAs are up.")
	return rpc.DoctorCheckDiagnostic{
		Code:     diagnostic.CodeDoctorIPsecESPFormFastPath,
		Severity: severityWarning,
		Message:  tb.String(),
		Fix:      espFormSharedSPIFixPeers,
	}
}

// espFormSharedSPIPeer answers the name of the peer whose current Child SA, or whose
// pending one, carries spi to local, read from the engine's active sessions
// (activePeersMap, the map TerminatePeerSA clears a peer from), and whether one does.
// The check runs in the engine's process (espFormFastPathCheckDecl), so these are the
// sessions that installed the SA.
//
// A session can hold three Child SAs: its current one (childSA), the one a parallel
// handshake installed before it supersedes the current one (pendingChild), and the one
// a rekey replaced, kept installed until the peer deletes it (supersededChild). The
// first two are guarded by ps.mu and are read here. supersededChild is owned by the
// session's maintainSA loop and guarded by no lock, so this goroutine cannot read it,
// and a replaced SA still watched until it is removed names no session.
//
// The SPI and the address are compared inside the ps.mu critical section, because a
// MOBIKE move (migrateMobikePair) rewrites a Child SA's LocalAddr under ps.mu: a
// pointer copied out under the lock and read after it races that write. The locks are
// taken in the order PeerInfoMap takes them: peersMu, then the session's own.
func espFormSharedSPIPeer(spi uint32, local netip.Addr) (string, bool) {
	want := local.Unmap()
	peersMu.RLock()
	defer peersMu.RUnlock()
	for name, ps := range activePeersMap {
		ps.mu.Lock()
		carries := espFormChildCarries(ps.childSA, spi, want) || espFormChildCarries(ps.pendingChild, spi, want)
		ps.mu.Unlock()
		if carries {
			return name, true
		}
	}
	return "", false
}

// espFormChildCarries reports whether child carries spi to want, an address already
// unmapped. A nil child carries nothing. The caller MUST hold the session's mu, which
// guards the Child SA's LocalAddr against a MOBIKE move.
func espFormChildCarries(child *ChildSA, spi uint32, want netip.Addr) bool {
	if child != nil && child.InboundSPI == spi {
		addr, ok := netip.AddrFromSlice(child.LocalAddr)
		return ok && addr.Unmap() == want
	}
	return false
}

// espFormCompletePathCost appends what the complete path costs while it carries bare
// ESP that arrived on link: the bound, and the device a firewall rule sees.
//
// The drop is the raw ESP reader's rate check (espFormLimiter.allow, read by the
// reader loop in dataplane/espform_linux.go): a datagram over the bound is skipped
// before it is written into the device, and the kernel has already refused the bare
// form for an SA that carries ESP-in-UDP, so nothing else delivers it.
func espFormCompletePathCost(tb *textbuf.Buffer, link string) {
	tb.Str("Every packet is still checked against the SA's policy, but at most ").
		Int(dataplane.ESPFormCompletePathRate).
		Str(" datagrams per second go through ").Str(espFormDevice).
		Str(": the complete path drops every datagram above that rate before XFRM sees it, " +
			"so that traffic is lost. A firewall rule keyed on the input interface sees ").Str(espFormDevice).
		Str(", not ").Str(link).Str(".")
}

// espFormFirewallFindings answers the finding of a fast path the firewall keeps off,
// or none, from how the fast path reads the ruleset (state.FirewallReading). The cause
// is text for the operator, so the reading alone chooses the fix:
//   - Unheld: no program is loaded, so there is no gate to keep shut. Nothing.
//   - Pending: the gate is shut only for the first dump after start, as it is for the
//     length of a firewall commit. Nothing.
//   - Read: a blocking site when a rule can drop ESP, otherwise nothing. A blocking
//     site is reported only from a read: without one there is no site to name.
//   - Unreadable, WatchClosed: nothing reads the ruleset again until the fast path
//     starts again, so the restart is the fix.
//   - DumpFailed: the watch still runs, so the next commit to the ruleset reads it
//     again and no restart is needed; the restart is the fix that reads it at once.
//   - Rewatching: Ze watches the ruleset again by itself, so no restart is needed, and
//     the fix lists why the watch keeps failing.
//   - Moving: a stream of commits keeps every read from completing. Nothing while it
//     is younger than dataplane.ESPFormFirewallMovingReportAfter; at that age or
//     older, one warning whose fix shows which tool commits (espFormFirewallMoving).
//   - Unspecified, or a value outside the set: only a Ze defect produces it, one warning.
func espFormFirewallFindings(state *dataplane.ESPFormFastPathState) []rpc.DoctorCheckDiagnostic {
	switch state.FirewallReading {
	case dataplane.ESPFormFirewallUnheld, dataplane.ESPFormFirewallPending:
		return nil
	case dataplane.ESPFormFirewallRead:
		if state.FirewallBlocked {
			return []rpc.DoctorCheckDiagnostic{espFormFirewallBlocked(state.FirewallSite, state.FirewallAcceptHow, state.FirewallAcceptFix)}
		}
		return nil
	case dataplane.ESPFormFirewallUnreadable, dataplane.ESPFormFirewallWatchClosed:
		return []rpc.DoctorCheckDiagnostic{espFormFirewallUnread(state.FirewallCause)}
	case dataplane.ESPFormFirewallDumpFailed:
		return []rpc.DoctorCheckDiagnostic{espFormFirewallDumpFailed(state.FirewallCause)}
	case dataplane.ESPFormFirewallRewatching:
		return []rpc.DoctorCheckDiagnostic{espFormFirewallRewatching(state.FirewallCause)}
	case dataplane.ESPFormFirewallMoving:
		return espFormFirewallMoving(state.FirewallMovingSince, espFormFirewallNow())
	case dataplane.ESPFormFirewallUnspecified:
	}
	return []rpc.DoctorCheckDiagnostic{espFormFirewallReadingUnknown(state.FirewallReading)}
}

// espFormFirewallBlocked is the warning for a fast path kept off because a firewall
// rule or policy on the input path could drop ESP. The fast path rewrites bare ESP at
// tc ingress, before netfilter, so it would hand that rule ESP-in-UDP instead and the
// rule would not see the ESP it was written for. how and fix are what the dataplane
// chose for the site's table (dataplane.ESPFormFastPathState.FirewallAcceptHow and
// FirewallAcceptFix): the nft insert for a table Ze does not own, the editor lines for
// a table of Ze's firewall config, the state a plugin's table follows
// (espFormOwnedTableAccept). A blocking site with no command is a Ze defect, so the
// fix then lists the dataplane's warnings, which name the site, rather than a guessed
// command.
func espFormFirewallBlocked(site, how, fix string) rpc.DoctorCheckDiagnostic {
	if site == "" {
		site = "the dataplane named no site"
	}
	var tb textbuf.Buffer
	tb.Str("The IPsec ESP form fast path is off because a firewall rule or policy on the input " +
		"path could drop ESP, and the fast path would rewrite bare ESP before that rule sees it. " +
		"The first site that could drop it: ").Str(site).
		Str(". So bare ESP for every Child SA on UDP port 4500 takes the complete path, which reads " +
			"it after the input path, and the firewall still decides on it. ")
	espFormCompletePathCost(&tb, "the link it arrived on")
	tb.Str(" To keep the fast path, and only if that ESP should be accepted, add a first rule " +
		"accepting protocol esp in that chain.")
	if how != "" {
		tb.Byte(' ').Str(how)
	}
	tb.Str(" Otherwise leave the firewall as it is, and accept the rate bound and the drops above it.")
	if fix == "" {
		tb.Str(" The dataplane named no command for this site, which is a Ze defect; the command " +
			"below lists its warnings, which name the site.")
		fix = espFormFastPathFixLog
	}
	return rpc.DoctorCheckDiagnostic{
		Code:     diagnostic.CodeDoctorIPsecESPFormFirewall,
		Severity: severityWarning,
		Message:  tb.String(),
		Fix:      fix,
	}
}

// espFormAcceptTerm is the term the editor lines of espFormOwnedTableAccept add.
const espFormAcceptTerm = "accept-esp"

// espFormOwnedTableAccept answers, for a blocking site in a table whose kernel name
// starts with ze_, who writes the table and what changes its rule
// (dataplane.ESPFormOwnedTableAccept, registered from register_esp_form_table.go).
//   - A table a producer reserved (firewall.TableControlOf) is that producer's: its
//     rules follow the state the reservation names, and the fix is the command that
//     shows it. The config editor cannot reach such a table, and an nft rule there
//     lasts until the producer next writes it, so neither is named.
//   - Any other ze_ table is the firewall config's, named ze_ and the config's table
//     name: the fix is the editor lines that add the accept-esp term to the chain and
//     commit it. The editor appends a new term after the chain's existing ones and has
//     no command that moves one ahead of another (a gap the config-list-entry-insert
//     spec owns), so the sentence says the term accepts ESP only where no earlier term
//     drops it, and offers no workaround.
//   - A ze_ table whose table or chain name the config cannot hold, the table named ze_
//     alone included (espFormConfigSite), is neither: the fix lists the tables Ze
//     applied.
func espFormOwnedTableAccept(_, table, chain string) (how, fix string) {
	if control, ok := firewall.TableControlOf(table); ok {
		var why textbuf.Buffer
		why.Str("Table ").Str(table).Str(" is written by Ze's ").Str(control.Plugin).
			Str(": its rules follow ").Str(control.Decides).
			Str(", and Ze rewrites the table whenever that changes, so neither an nft rule nor the " +
				"config editor changes it for long. The rule changes only with that state, which the " +
				"command below shows.")
		return why.String(), control.Command
	}
	name := firewall.StripZeTablePrefix(table)
	if !espFormConfigSite(table, name, chain) {
		var why textbuf.Buffer
		why.Str("Table ").Str(table).Str(" carries Ze's ze_ prefix, but no Ze producer reserved it and " +
			"its table or chain name is one the firewall configuration cannot hold, so it is no table " +
			"of Ze's configuration either. The command below lists the tables Ze applied.")
		return why.String(), "show firewall ruleset"
	}
	var lines textbuf.Buffer
	lines.Str("set firewall table ").Str(name).Str(" chain ").Str(chain).Str(" term ").Str(espFormAcceptTerm).
		Str(" from protocol esp\n")
	lines.Str("set firewall table ").Str(name).Str(" chain ").Str(chain).Str(" term ").Str(espFormAcceptTerm).
		Str(" then accept\n")
	lines.Str("commit now")
	var why textbuf.Buffer
	why.Str("Table ").Str(table).Str(" is the table ").Str(name).
		Str(" of Ze's firewall configuration, and Ze rewrites it at each firewall commit, so an nft " +
			"rule there would be gone after the next one. Paste the lines below into ze config edit: " +
			"they add the term ").Str(espFormAcceptTerm).Str(", which accepts ESP, to chain ").Str(chain).
		Str(" and commit it. The editor adds a new term after the chain's existing terms and has no " +
			"command that moves a term ahead of another, so the term accepts ESP only where no earlier " +
			"term of the chain drops it.")
	return why.String(), lines.String()
}

// espFormConfigSite answers whether the kernel table table, whose name with the ze_
// prefix stripped is name, and its chain chain can be a table and a chain of the
// firewall configuration, so editor lines naming them type as written.
func espFormConfigSite(table, name, chain string) bool {
	// StripZeTablePrefix leaves a table named ze_ alone unchanged, and a config table
	// named ze_ would be the kernel table ze_ze_, so no config table is this one.
	if name == table {
		return false
	}
	if !espFormConfigName(name) {
		return false
	}
	return espFormConfigName(chain)
}

// espFormConfigName answers whether name is one the firewall configuration holds for a
// table or a chain (ze-firewall-conf.yang: 1 to 255 characters, a letter or a digit
// first, then letters, digits, underscores and dashes), so an editor line naming it
// types as written.
func espFormConfigName(name string) bool {
	if name == "" {
		return false
	}
	if len(name) > 255 {
		return false
	}
	for i, r := range name {
		letterOrDigit := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if letterOrDigit {
			continue
		}
		if i > 0 && (r == '_' || r == '-') {
			continue
		}
		return false
	}
	return true
}

// espFormFirewallOffForSafety appends what a fast path kept off for want of a ruleset
// read costs: why it is off, with cause, and the path bare ESP takes instead.
func espFormFirewallOffForSafety(tb *textbuf.Buffer, cause string) {
	if cause == "" {
		cause = espFormNoCause
	}
	tb.Str("The IPsec ESP form fast path is off for safety (").Str(cause).
		Str("): Ze cannot tell whether a firewall rule on the input path could drop ESP, which " +
			"the fast path would rewrite before that rule sees it. So bare ESP for every Child SA " +
			"on UDP port 4500 takes the complete path, which reads it after the input path, and " +
			"the firewall still decides on it. ")
	espFormCompletePathCost(tb, "the link it arrived on")
}

// espFormFirewallUnread is the warning for a fast path kept off because the nftables
// reader or watch could not open when the fast path started
// (dataplane.ESPFormFirewallUnreadable), or because Close ended the watch that follows
// the ruleset's changes (dataplane.ESPFormFirewallWatchClosed): Ze cannot tell whether
// a rule could drop ESP, so it keeps the fast path off rather than bypass one, and
// nothing reads the ruleset again until the fast path starts again, which a restart
// does.
func espFormFirewallUnread(cause string) rpc.DoctorCheckDiagnostic {
	var tb textbuf.Buffer
	espFormFirewallOffForSafety(&tb, cause)
	tb.Str(" Nothing reads the ruleset again until ze restarts, or until no Child SA is served " +
		"in both ESP forms and one is installed again. Restart ze with the command below to " +
		"read the ruleset and follow its changes again. If the cause is a permission error, ze " +
		"lacks CAP_NET_ADMIN, and the daemon-capabilities finding names how to grant it. If this " +
		"finding returns, ").Str(espFormFastPathFixLog).Str(" lists why the ruleset could not be read.")
	return rpc.DoctorCheckDiagnostic{
		Code:     diagnostic.CodeDoctorIPsecESPFormFirewall,
		Severity: severityWarning,
		Message:  tb.String(),
		Fix:      espFormFastPathFixRestart,
	}
}

// espFormFirewallDumpFailed is the warning for a fast path kept off because the last
// dump of the nftables ruleset failed while the watch that follows its changes still
// runs (dataplane.ESPFormFirewallDumpFailed). The next commit to the ruleset, by Ze's
// firewall or by any other program, makes Ze read it again, and a read that finds no
// rule that can drop ESP turns the fast path back on, so no restart is needed. Nothing
// reads the ruleset before such a commit, so the restart is the fix that reads it at
// once.
func espFormFirewallDumpFailed(cause string) rpc.DoctorCheckDiagnostic {
	var tb textbuf.Buffer
	espFormFirewallOffForSafety(&tb, cause)
	tb.Str(" For the firewall reading, no restart is needed: Ze still follows the ruleset's changes, so the next commit " +
		"to it, by Ze's firewall or by any other program, reads it again, and the fast path " +
		"comes back on once a read finds no rule that can drop ESP. Nothing reads the ruleset " +
		"before that commit. To read it at once, restart ze with the command below. If this " +
		"finding returns after a commit, ").Str(espFormFastPathFixLog).
		Str(" lists why the dump failed.")
	return rpc.DoctorCheckDiagnostic{
		Code:     diagnostic.CodeDoctorIPsecESPFormFirewall,
		Severity: severityWarning,
		Message:  tb.String(),
		Fix:      espFormFastPathFixRestart,
	}
}

// espFormFirewallRewatching is the warning for a fast path kept off because the watch
// that follows the nftables ruleset's changes failed (dataplane.ESPFormFirewallRewatching).
// Ze watches the ruleset again by itself, after a wait that doubles up to
// dataplane.ESPFormFirewallRewatchMax, and a fresh read after the watch is up turns the
// fast path back on, so a restart is no fix: the fix lists the dataplane's warnings,
// which name why the watch keeps failing.
func espFormFirewallRewatching(cause string) rpc.DoctorCheckDiagnostic {
	var tb textbuf.Buffer
	espFormFirewallOffForSafety(&tb, cause)
	tb.Str(" For the firewall reading, no restart is needed: Ze watches the ruleset again by itself, trying at most ").
		Int(int64(dataplane.ESPFormFirewallRewatchMax / time.Second)).
		Str(" seconds apart, and turns the fast path back on once a fresh read of the ruleset " +
			"finds no rule that can drop ESP. If this finding stays, the command below lists why " +
			"the watch keeps failing, most often a missing CAP_NET_ADMIN capability.")
	return rpc.DoctorCheckDiagnostic{
		Code:     diagnostic.CodeDoctorIPsecESPFormFirewall,
		Severity: severityWarning,
		Message:  tb.String(),
		Fix:      espFormFastPathFixLog,
	}
}

// espFormFirewallNow is the clock espFormFirewallMoving measures a stream of commits
// against. It is a variable so a test sets the time.
var espFormFirewallNow = time.Now

// espFormFirewallMoving answers the warning for a fast path kept off because the
// nftables ruleset has kept changing since at least since
// (dataplane.ESPFormFirewallMoving), or nothing while since is younger than
// dataplane.ESPFormFirewallMovingReportAfter: a shorter stream is a burst of commits,
// which the fast path outlasts. since is when the first judgment that met the stream
// ended, not when the stream began, so the message says "since at least". Each commit
// shuts the gate, and no read of the ruleset completes while every read meets a new
// commit, so the gate stays shut until the commits pause, and then the read that
// completes decides it: it turns the fast path back on only when it finds no rule that
// can drop ESP. No restart is needed. The tool that keeps committing is what changes
// it: the fix shows each commit as it lands, and the message says to make that tool
// batch its changes, or to stop it. A zero since is a Ze defect, named as such rather
// than printed as a date.
func espFormFirewallMoving(since, now time.Time) []rpc.DoctorCheckDiagnostic {
	if !since.IsZero() && now.Sub(since) < dataplane.ESPFormFirewallMovingReportAfter {
		return nil
	}
	var tb textbuf.Buffer
	tb.Str("The nftables ruleset has kept changing since ")
	if since.IsZero() {
		tb.Str("a time the dataplane did not record (a Ze defect)")
	} else {
		tb.Str("at least ").Str(since.UTC().Format(time.RFC3339)).Str(", ").
			Int(int64(now.Sub(since) / time.Second)).Str(" seconds ago")
	}
	tb.Str(", so the IPsec ESP form fast path stays off: every read of the ruleset meets a new commit, " +
		"and the fast path must not rewrite bare ESP past a rule it has not read. So bare ESP for every " +
		"Child SA on UDP port 4500 takes the complete path, which reads it after the input path, and the " +
		"firewall still decides on it. ")
	espFormCompletePathCost(&tb, "the link it arrived on")
	tb.Str(" For the firewall reading, no restart is needed: the fast path comes back on once the commits pause long enough for " +
		"one read, and that read finds no rule that can drop ESP. Run the command below to see which " +
		"tool keeps committing, then make that tool batch its changes, or stop it. fail2ban, for " +
		"example, commits each ban on its own.")
	return []rpc.DoctorCheckDiagnostic{{
		Code:     diagnostic.CodeDoctorIPsecESPFormFirewall,
		Severity: severityWarning,
		Message:  tb.String(),
		Fix:      dataplane.ESPFormFixMoving,
	}}
}

// espFormFirewallReadingUnknown is the warning for a firewall reading the check does
// not know (dataplane.ESPFormFirewallUnspecified, or a value outside the set), which
// only a Ze defect produces: the check cannot tell whether the firewall keeps the fast
// path off, nor which fix applies.
func espFormFirewallReadingUnknown(reading dataplane.ESPFormFirewallReading) rpc.DoctorCheckDiagnostic {
	var tb textbuf.Buffer
	tb.Str("Ze could not read how the IPsec ESP form fast path reads the nftables ruleset " +
		"(it reported the firewall reading ").Str(reading.String()).
		Str(", which only a Ze defect produces), so it cannot tell whether a firewall rule keeps " +
			"the fast path off and bare ESP for Child SAs on UDP port 4500 on the rate-bounded " +
			"complete path through ").Str(espFormDevice).
		Str(". The warnings the command below lists name the cause.")
	return rpc.DoctorCheckDiagnostic{
		Code:     diagnostic.CodeDoctorIPsecESPFormFirewall,
		Severity: severityWarning,
		Message:  tb.String(),
		Fix:      espFormFastPathFixLog,
	}
}

// espFormFastPathUnavailable is the warning for a fast path that could not load.
func espFormFastPathUnavailable(cause string) rpc.DoctorCheckDiagnostic {
	if cause == "" {
		cause = espFormNoCause
	}
	var tb textbuf.Buffer
	tb.Str("The IPsec ESP form fast path is unavailable (").Str(cause).
		Str("), so bare ESP for every Child SA on UDP port 4500 is re-presented by the complete path. ")
	espFormCompletePathCost(&tb, "the link it arrived on")
	tb.Str(" Under systemd, a ze unit written by an older ze does not grant CAP_BPF: the " +
		"command below stops ze with sudo systemctl stop ze, because the install refuses while " +
		"ze runs, then rewrites the unit (add the --config <dir> the unit was first installed " +
		"with to the install), reloads systemd and starts ze again. Without systemd, start ze with CAP_BPF and CAP_NET_ADMIN. On a kernel " +
		"before Linux 6.6, which has no TCX, upgrade the kernel instead.")
	return rpc.DoctorCheckDiagnostic{
		Code:     diagnostic.CodeDoctorIPsecESPFormFastPath,
		Severity: severityWarning,
		Message:  tb.String(),
		Fix:      espFormFastPathFixCapabilities,
	}
}

// espFormFastPathDetached is the warning for a fast path that loaded and attached, and
// then detached itself from every link because the kernel refused a write that shuts
// its firewall gate (dataplane.ESPFormFastPathDetached). The program had loaded, so a
// capability or a kernel upgrade is no fix: the fast path stays detached until it
// starts again, which a restart of ze does, the command the dataplane's warning names
// too (dataplane.ESPFormFixRestart).
func espFormFastPathDetached(cause string) rpc.DoctorCheckDiagnostic {
	if cause == "" {
		cause = espFormNoCause
	}
	var tb textbuf.Buffer
	tb.Str("The IPsec ESP form fast path detached itself from every link (").Str(cause).
		Str("). Ze could not shut its firewall gate, which keeps the program from rewriting bare ESP " +
			"while Ze checks the nftables ruleset, so it removed the program rather than let it rewrite " +
			"bare ESP past a firewall rule that should see it. So bare ESP for every Child SA on UDP port " +
			"4500 takes the complete path, which reads it after the input path, and the firewall still " +
			"decides on it. ")
	espFormCompletePathCost(&tb, "the link it arrived on")
	tb.Str(" The fast path stays detached until ze restarts, or until no Child SA is served in both " +
		"ESP forms and one is installed again. Restart ze with the command below to load it again. If " +
		"this finding returns, ").Str(espFormFastPathFixLog).Str(" lists why the gate could not be shut.")
	return rpc.DoctorCheckDiagnostic{
		Code:     diagnostic.CodeDoctorIPsecESPFormFastPath,
		Severity: severityWarning,
		Message:  tb.String(),
		Fix:      espFormFastPathFixRestart,
	}
}

// espFormFastPathStoppedFollowing is the warning for a fast path whose link
// subscription ended: the links it holds stay attached, a new link is not.
func espFormFastPathStoppedFollowing(state *dataplane.ESPFormFastPathState) rpc.DoctorCheckDiagnostic {
	var tb textbuf.Buffer
	tb.Str("The IPsec ESP form fast path stopped following new links (").Str(state.Cause).
		Str("). It still rewrites bare ESP on the ").Int(int64(len(state.Attached))).
		Str(" links it is attached to; bare ESP on a link created since is re-presented by the complete path. ")
	espFormCompletePathCost(&tb, "that link")
	tb.Str(" Restart ze to follow links again.")
	return rpc.DoctorCheckDiagnostic{
		Code:     diagnostic.CodeDoctorIPsecESPFormFastPath,
		Severity: severityWarning,
		Message:  tb.String(),
		Fix:      espFormFastPathFixRestart,
	}
}

// espFormLinkRefused is the warning for one link the fast path does not serve.
func espFormLinkRefused(refusal *dataplane.ESPFormLinkRefusal) rpc.DoctorCheckDiagnostic {
	var tb textbuf.Buffer
	tb.Str("The IPsec ESP form fast path does not serve link ").Str(refusal.Link).
		Str(" (").Str(refusal.Reason).
		Str("), so bare ESP arriving on it for a Child SA on UDP port 4500 takes the complete path. ")
	espFormCompletePathCost(&tb, refusal.Link)
	tb.Str(" To keep that traffic in the kernel, set the peer to always encapsulate ESP in UDP " +
		"(on a strongSwan peer, set ").Str(espFormFastPathPeerEncap).
		Str(" in the connection and run ").Str(espFormFastPathPeerLoad).
		Str(" there), or route it over a link of a type the fast path serves.")
	fix, pastable := espFormLinkFix(refusal.Link)
	if pastable {
		tb.Str(" The command below shows the link's type and state.")
	} else {
		tb.Str(" The link's name cannot be typed as one command word, so the command below " +
			"lists every interface, so you can find it there.")
	}
	return rpc.DoctorCheckDiagnostic{
		Code:     diagnostic.CodeDoctorIPsecESPFormLink,
		Severity: severityWarning,
		Message:  tb.String(),
		Fix:      fix,
	}
}

// espFormLinkFix answers the command that shows link, whose type or attach the
// refusal names: `ze show interface name <link> detail`, and true. The kernel takes
// any byte in a link name but '/', ':' and white space (dev_valid_name,
// net/core/dev.c), so a quote, a pipe or a control character can reach here. A name
// holding anything but ASCII letters, digits, '.', '-' and '_' is not pasted into a
// command: the answer is the interface listing, and false.
func espFormLinkFix(link string) (fix string, pastable bool) {
	if !espFormOneWord(link) {
		return espFormFastPathFixLinks, false
	}
	var tb textbuf.Buffer
	tb.Str("ze show interface name ").Str(link).Str(" detail")
	return tb.String(), true
}

// espFormOneWord answers whether name can be pasted into a fix line as one CLI word: it
// is not empty and holds only the bytes espFormLinkNameByte admits.
func espFormOneWord(name string) bool {
	if name == "" {
		return false
	}
	for i := range len(name) {
		if !espFormLinkNameByte(name[i]) {
			return false
		}
	}
	return true
}

// espFormLinkNameByte answers whether c can stand in a link name pasted as one CLI
// word.
func espFormLinkNameByte(c byte) bool {
	if c >= 'a' && c <= 'z' {
		return true
	}
	if c >= 'A' && c <= 'Z' {
		return true
	}
	if c >= '0' && c <= '9' {
		return true
	}
	return c == '.' || c == '-' || c == '_'
}

// espFormFastPathUnread is the warning raised when the check could not read the fast
// path's state, so it cannot say which path carries bare ESP. fix lists the warnings
// that name the cause: the dataplane's when it answered, the engine's when it did not
// load.
func espFormFastPathUnread(why, fix string) rpc.DoctorCheckDiagnostic {
	var tb textbuf.Buffer
	tb.Str("Ze could not read the IPsec ESP form fast path's state (").Str(why).
		Str("), so it cannot tell whether bare ESP for Child SAs on UDP port 4500 is rewritten in " +
			"the kernel or re-presented through ").Str(espFormDevice).
		Str(" at a bounded rate. The warnings the command below lists name the cause.")
	return rpc.DoctorCheckDiagnostic{
		Code:     diagnostic.CodeDoctorIPsecESPFormFastPath,
		Severity: severityWarning,
		Message:  tb.String(),
		Fix:      fix,
	}
}
