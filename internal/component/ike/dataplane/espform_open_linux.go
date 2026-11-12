// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- both ESP forms on one inbound state: the complete path re-presents on a policy-checked device
// RFC: rfc/short/rfc7296.md -- receive both ESP forms at any time (Section 2.23)
// Overview: espform_linux.go -- the dual-form receiver whose complete path this opens and reopens
// Related: espform_tun_linux.go -- the ze-espform device the open creates

//go:build linux

package dataplane

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/privilege"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// espFormSocketWhy is why every refusal of the reader socket matters. The SA is refused
// because of what the host lacks, never because of what the operator configured.
const espFormSocketWhy = "Ze reads, through a raw ESP socket, the bare ESP the kernel refuses to a Child SA " +
	"that must receive both ESP forms. Without it, Ze cannot receive bare ESP on such a Child SA " +
	"(RFC 7296 Section 2.23), so it refuses the SA at install and at a MOBIKE move. This is not a " +
	"configuration error"

// The fixes of the reader socket's refusals, besides the capability install
// (privilege.DaemonCapabilitiesFix) and the dataplane's warnings (ESPFormFixLog).
const (
	// espFormFixSecurityModule lists this boot's denials by SELinux (an "avc:" record)
	// and AppArmor (apparmor="DENIED"). Both are audit records: where auditd does not run
	// the kernel logs them, and where it does, journald reads the audit stream too on a
	// standard systemd host, so the journal holds them either way, where ausearch needs
	// the audit package and dmesg misses what auditd took.
	espFormFixSecurityModule = `sudo journalctl -b --grep 'avc:|apparmor="DENIED"'`
	// espFormFixFileLimit rewrites the ze unit, whose LimitNOFILE is 65536
	// (internal/plugins/systemd/unit.go), and starts ze again under it: a ze started
	// another way runs with the soft limit it inherited, 1024 by default.
	espFormFixFileLimit = privilege.DaemonCapabilitiesFix
	// espFormFixFileTable raises the host's table of open files to the value systemd
	// sets at boot, the largest the kernel takes.
	espFormFixFileTable = "sudo sysctl -w fs.file-max=9223372036854775807"
	// espFormFixMemory shows the host's free memory, which the kernel was short of.
	espFormFixMemory = "free -m"
)

// The subjects of the reader socket's refusals, each a verb phrase after "Ze ... to".
const (
	espFormSocketSubject = "open the raw ESP socket (" + espFormListenNetwork + ") that receives bare ESP"
	espFormBypassSubject = "exempt its raw ESP socket from the inbound IPsec policy check"
)

// espFormSocketRefusal maps the error of the reader socket's open. EPERM is a process
// without CAP_NET_RAW, which a raw socket needs (inet_create), and the unit install grants
// it; EACCES is a security module that refuses the socket (security_socket_create), which
// no capability changes. Any other answer is the host's limit or a cause Ze does not
// know (espFormOpenFailure). ListenPacket's error already names the network and the call,
// so it is the cause unchanged.
func espFormSocketRefusal(err error) error {
	if errors.Is(err, unix.EPERM) {
		return opererr.Wrap(err,
			"Ze has no permission to "+espFormSocketSubject,
			"a raw socket needs CAP_NET_RAW. "+espFormSocketWhy,
			privilege.DaemonCapabilitiesFix)
	}
	if errors.Is(err, unix.EACCES) {
		return espFormSecurityModuleRefusal(err, espFormSocketSubject, espFormSocketWhy)
	}
	return espFormOpenFailure(err, espFormSocketSubject, espFormSocketWhy)
}

// espFormBypassRefusal maps the error of the reader socket's exemption from the inbound
// policy check (IP_XFRM_POLICY). EPERM is a process without CAP_NET_ADMIN, which the
// kernel asks of a caller that sets a socket policy (xfrm_user_policy), and EACCES a
// security module that refuses the policy (security_xfrm_policy_alloc). Any other answer
// is the host's limit or a cause Ze does not know (espFormOpenFailure).
func espFormBypassRefusal(err error) error {
	cause := fmt.Errorf("setsockopt IP_XFRM_POLICY: %w", err)
	why := "without the exemption the kernel drops into the raw ESP socket every bare ESP datagram it " +
		"refused, so the socket receives nothing. " + espFormSocketWhy
	if errors.Is(err, unix.EPERM) {
		return opererr.Wrap(cause,
			"Ze has no permission to "+espFormBypassSubject,
			"setting a socket policy needs CAP_NET_ADMIN; "+why,
			privilege.DaemonCapabilitiesFix)
	}
	if errors.Is(err, unix.EACCES) {
		return espFormSecurityModuleRefusal(cause, espFormBypassSubject, why)
	}
	return espFormOpenFailure(cause, espFormBypassSubject, why)
}

// espFormSecurityModuleRefusal is the refusal for an EACCES of the reader socket or its
// exemption: a Linux security module's policy refused ze, which the capability install
// does not change, so the fix lists the denials the kernel recorded.
func espFormSecurityModuleRefusal(cause error, subject, why string) error {
	return opererr.Wrap(cause,
		"a Linux security module (SELinux or AppArmor) does not allow Ze to "+subject,
		"the kernel answered EACCES, which a security module's policy gives, not a missing capability, "+
			"so reinstalling the ze unit does not fix it: the policy must allow it for ze. The command "+
			"below lists the denials it recorded. "+why,
		espFormFixSecurityModule)
}

// espFormOpenFailure is the refusal for an answer of the reader socket or its exemption
// that is no refused privilege: the process's limit of open files (EMFILE), the host's
// (ENFILE), memory the kernel was short of (ENOMEM, ENOBUFS), and any other cause, which
// Ze does not know and which it tries again by itself while an SA needs the socket.
func espFormOpenFailure(cause error, subject, why string) error {
	retry := ". Ze opens it again at the next install of such a Child SA, and while one is installed " +
		"by itself, at most " + espFormReopenBound() + " apart"
	if errors.Is(cause, unix.EMFILE) {
		return opererr.Wrap(cause,
			"Ze has reached its limit of open files, so it cannot "+subject,
			"the ze unit Ze installs raises the limit to 65536 (LimitNOFILE), and the command below "+
				"rewrites the unit and starts ze again under it. "+why+retry,
			espFormFixFileLimit)
	}
	if errors.Is(cause, unix.ENFILE) {
		return opererr.Wrap(cause,
			"the host has reached its limit of open files (fs.file-max), so Ze cannot "+subject,
			"the limit is for every process on the host, and the command below raises it to the value "+
				"systemd sets at boot. "+why+retry,
			espFormFixFileTable)
	}
	if errors.Is(cause, unix.ENOMEM) {
		return espFormMemoryRefusal(cause, subject, why+retry)
	}
	if errors.Is(cause, unix.ENOBUFS) {
		return espFormMemoryRefusal(cause, subject, why+retry)
	}
	return opererr.Wrap(cause,
		"Ze cannot "+subject,
		why+". Ze does not know this cause"+retry+"; the warnings the command below lists show each attempt",
		ESPFormFixLog)
}

// espFormMemoryRefusal is the refusal for an answer the kernel gives when it is short of
// memory.
func espFormMemoryRefusal(cause error, subject, why string) error {
	return opererr.Wrap(cause,
		"the kernel is short of memory, so Ze cannot "+subject,
		"the command below shows the host's free memory; free some. "+why,
		espFormFixMemory)
}

// espFormReopenBound is espFormReopenWaitMax in words, for a message that names the
// longest wait between two reopens of the receiver.
func espFormReopenBound() string {
	var tb textbuf.Buffer
	return tb.Int(int64(espFormReopenWaitMax / time.Second)).Str(" seconds").String()
}

// openESPFormPaths opens the reader socket, exempts it from the inbound policy check, and
// opens the ze-espform device. Either failing fails the open, with nothing left open:
// the caller refuses the SA rather than install one whose bare form nothing serves.
// Every failure the host can cause is an operator error with its own fix
// (espFormSocketRefusal, espFormBypassRefusal, openESPFormTUN), which the refusal of the
// SA and the doctor carry.
func openESPFormPaths() (espFormPaths, error) {
	// (*net.ListenConfig).ListenPacket rather than net.ListenPacket: the package
	// form takes no context and cannot be canceled, which `noctx` refuses. The
	// socket is opened once at receiver start and lives until Close, so there is no
	// deadline to carry and Background is the honest context here.
	var lc net.ListenConfig
	packetConn, err := lc.ListenPacket(context.Background(), espFormListenNetwork, "0.0.0.0")
	if err != nil {
		return espFormPaths{}, espFormSocketRefusal(err)
	}
	// An "ip4:" network always answers an IP socket (net.ListenConfig.ListenPacket
	// builds an *IPConn for it), so another answer is a defect no host can produce;
	// the reader could not read the IPv4 header the received TOS is in.
	conn, ok := packetConn.(*net.IPConn)
	if !ok {
		panic("BUG: the ESP form reader opened " + espFormListenNetwork + " and got no IP socket")
	}
	// Before the reader runs, and reported rather than swallowed. Without the
	// exemption the reader blocks forever on a socket the kernel drops into, and
	// the SA receives one ESP form only (ai/rules/evidence.md).
	if err := espFormBypassInboundPolicy(conn); err != nil {
		conn.Close() //nolint:errcheck // the descriptor is released whatever close answers; the refusal is the error reported
		return espFormPaths{}, err
	}
	tun, err := openESPFormTUN()
	if err != nil {
		conn.Close() //nolint:errcheck // the descriptor is released whatever close answers; the refusal is the error reported
		return espFormPaths{}, err
	}
	return espFormPaths{conn: conn, tun: tun}, nil
}

// read runs the reader on one open of the paths, and closes life.dead when the socket
// failed while the receiver still wanted the reader, which wakes its revive worker. It
// takes no lock, which is what lets closeReaderLocked wait for it under r.mu. The caller
// MUST have called r.wg.Add(1); read reports its own completion.
func (r *espFormReceiver) read(conn espFormPacketReader, inj espFormInjector, life *espFormReaderLife) {
	defer r.wg.Done()

	err := r.run(conn, inj)
	if espFormStopped(life.stop) {
		return
	}
	life.err = err
	close(life.dead)
}

// revive is the revive worker of one reader. It waits for the reader to stop on a failed
// socket, logs it, hands the stop to the engine (reportReaderStopped), and opens the
// paths again, at most espFormReopenWaitMax apart on the receiver's backoff
// (reopenWaitLocked, reviveLocked), until an open succeeds (the new reader has its own
// worker) or the receiver retires the reader (stopLocked, or an open from a Watch or
// acquire that replaced it). The SAs it was serving are in the SAD, and their bare ESP is
// owed at any time, so it does not wait for a new Watch. The caller MUST have called
// r.revivers.Add(1); Close waits for it.
//
// RFC 7296 Section 2.23: "all devices MUST be able to receive and process both UDP-encapsulated ESP and non-UDP-encapsulated ESP packets at any time."
func (r *espFormReceiver) revive(life *espFormReaderLife) {
	defer r.revivers.Done()

	select {
	case <-life.stop:
		return
	case <-life.dead:
	}
	r.logger().Warn("esp-form: Ze's own receiver for bare ESP stopped reading its raw ESP socket, so the bare "+
		"ESP of every Child SA that receives both ESP forms is received only where the fast path rewrites it "+
		"until Ze opens the receiver again; Ze opens it again by itself",
		"error", life.err, "code", diagnostic.CodeDoctorIPsecESPFormReaderStopped)
	r.reportReaderStopped(life)

	r.mu.Lock()
	wait := r.reopenWaitLocked(life)
	r.mu.Unlock()
	failing := false
	for {
		timer := time.NewTimer(wait)
		select {
		case <-life.stop:
			timer.Stop()
			return
		case <-timer.C:
		}
		r.mu.Lock()
		retry, err := r.reviveLocked(life)
		wait = r.reopenWait
		r.mu.Unlock()
		if !retry {
			if failing {
				r.logger().Info("esp-form: Ze's own receiver for bare ESP opened again, so bare ESP is received again")
			}
			return
		}
		if !failing {
			r.logger().LogAttrs(context.Background(), slog.LevelWarn,
				"esp-form: Ze cannot open its own receiver for bare ESP again, and tries again by itself at most "+
					espFormReopenBound()+" apart; until it opens, no Child SA that must receive both ESP forms "+
					"can be installed or moved onto the ESP-in-UDP form",
				espFormFailureAttrs(err, slog.String("code", diagnostic.CodeDoctorIPsecESPFormTUN))...)
		}
		failing = true
	}
}

// reportReaderStopped hands the stop of life's reader to the readerStopped seam when a
// test set one, and otherwise to the sink the engine registered
// (RegisterESPFormReaderStopped), which raises it on the report bus. With neither, a
// backend no engine runs, the Warn revive logs is the stop's only record.
func (r *espFormReceiver) reportReaderStopped(life *espFormReaderLife) {
	stopped := ESPFormReaderStopped{Err: life.err, At: time.Now()}
	if r.readerStopped != nil {
		r.readerStopped(stopped)
		return
	}
	if sink := espFormReaderStoppedSink.Load(); sink != nil {
		(*sink)(stopped)
	}
}

// reopenWaitLocked answers the wait before the first reopen after life's reader stopped:
// the receiver's backoff, which the stop of a reader that served espFormReopenResetAfter
// first resets to zero. A reader that dies right after each open is therefore reopened
// further apart each time, up to espFormReopenWaitMax, rather than in a tight loop.
// r.mu must be held.
func (r *espFormReceiver) reopenWaitLocked(life *espFormReaderLife) time.Duration {
	if time.Since(life.opened) >= espFormReopenResetAfter {
		r.reopenWait = 0
	}
	return r.reopenWait
}

// reviveLocked makes one attempt to open the paths for the stopped reader life, and
// answers whether to try again, with the open's error when it does. Before the attempt it
// doubles the receiver's backoff (reopenWait, from reopenMin up to espFormReopenWaitMax),
// whatever the attempt answers: only a reader that served long enough resets it
// (reopenWaitLocked). It answers false, and changes nothing, when life is no longer the
// receiver's reader: the paths were stopped, or a Watch or acquire opened new ones.
// r.mu must be held.
func (r *espFormReceiver) reviveLocked(life *espFormReaderLife) (bool, error) {
	if r.life != life {
		return false, nil
	}
	r.reopenWait = min(max(2*r.reopenWait, r.reopenMin), espFormReopenWaitMax)
	if err := r.reopenReaderLocked(); err != nil {
		return true, err
	}
	return false, nil
}

// espFormFailureAttrs answers the log attributes of an open failure, followed by attrs,
// which carry the doctor code at the call: the operator error's problem and its own
// attributes (why, fix and cause) when it is one, and the error alone when it is not.
func espFormFailureAttrs(err error, attrs ...slog.Attr) []slog.Attr {
	if refusal, ok := errors.AsType[*opererr.Error](err); ok {
		return refusal.LogAttrs(append([]slog.Attr{slog.String("problem", refusal.Problem())}, attrs...)...)
	}
	return append([]slog.Attr{slog.Any("error", err)}, attrs...)
}
