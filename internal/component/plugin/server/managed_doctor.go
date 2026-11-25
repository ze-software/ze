// Design: docs/architecture/fleet-config.md -- what the managed config server tells doctor
// Related: managed_serve.go -- ManagedServer, whose Start, handleConn and handleRequest record here
// Related: ../doctor/check_managed_runtime.go -- the runtime checks that read ManagedDoctorSnapshot

package server

import (
	"crypto/tls"
	"errors"
	"net"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// managedCodeCollision is the code of the collision the local
// hub-managed-listener check (internal/component/plugin/doctor) declares and
// reports from the config; Start logs the same fault under it.
const managedCodeCollision = "doctor-hub-managed-collision"

// managedReportSource is the report-bus source of a managed server event.
const managedReportSource = "managed-server"

// notifyDropped logs and raises a config-changed notification a connected
// client was not given, naming the client and the cause. The client keeps
// running its old config until it fetches again, which it does on connect.
func notifyDropped(client, cause string) {
	var message, fix textbuf.Buffer
	message.Str("managed client ").Quoted(client).Str(" was not told its config changed: ").Str(cause).
		Str(", so it runs its previous config until it fetches again")
	fix.Str("restart ze on managed client ").Str(client).Str(": a client fetches its config each time it connects")
	managedLogger().Warn("config-changed notify dropped",
		"code", CodeDoctorManagedClientNotifyDropped, "name", client, "cause", cause)
	report.RaiseErrorFix(managedReportSource, CodeDoctorManagedClientNotifyDropped, client,
		message.String(), fix.String())
}

// certRejectionAlerts lists the six RFC 8446 / IANA TLS alerts a peer sends
// when it refuses the hub's certificate (42 bad_certificate, 43
// unsupported_certificate, 44 certificate_revoked, 45 certificate_expired, 46
// certificate_unknown, 48 unknown_ca).
var certRejectionAlerts = []tls.AlertError{
	tls.AlertError(42),
	tls.AlertError(43),
	tls.AlertError(44),
	tls.AlertError(45),
	tls.AlertError(46),
	tls.AlertError(48),
}

// isCertRejectionAlert reports whether err is a remote TLS alert indicating
// that the peer rejected the hub's certificate. In Go's crypto/tls, every fatal
// remote alert arrives as *net.OpError with Op == "remote error" whose Err is
// an unexported alert whose Error() matches tls.AlertError(code).Error().
// Non-certificate fatal alerts (such as handshake_failure or protocol_version)
// arrive with the same Op and MUST return false.
func isCertRejectionAlert(err error) bool {
	opErr, ok := errors.AsType[*net.OpError](err)
	if !ok {
		return false
	}
	if opErr.Op != "remote error" {
		return false
	}
	if opErr.Err == nil {
		return false
	}
	if alertErr, ok := errors.AsType[tls.AlertError](opErr.Err); ok {
		return slices.Contains(certRejectionAlerts, alertErr)
	}
	errText := opErr.Err.Error()
	for _, alert := range certRejectionAlerts {
		if errText == alert.Error() {
			return true
		}
	}
	return false
}

// remoteHostOf strips the TCP port from remote, falling back to remote itself
// if it is already a bare address.
func remoteHostOf(remote string) string {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return remote
	}
	return host
}

// authRefused records and logs a managed connection whose auth was refused,
// so `show doctor` names the name it claimed (never the token it sent). A
// claimed client that lookup knows, which is one the accepting listener's
// server block declares, is recorded under its name with
// ManagedRejectBadToken; any other claim goes to the bounded unknown record,
// a client declared only under another server block included.
// A connection that failed before it named a client is recorded per remote
// address without its port, split between certificate-rejection TLS alerts
// (doctor-managed-certificate-refused) and all other pre-identification
// failures (doctor-managed-client-rejected).
func (s *ManagedServer) authRefused(remote, listenerAddr string, err error, lookup func(name string) (string, bool)) {
	refusal, named := errors.AsType[*pluginipc.AuthRefusal](err)
	if !named {
		host := remoteHostOf(remote)
		now := s.now()
		if isCertRejectionAlert(err) {
			s.doctor.recordCertRefused(host, listenerAddr, err.Error(), now)
			managedLogger().Warn("managed connection refused hub certificate",
				"code", CodeDoctorManagedCertificateRefused, "remote", remote, "listener", listenerAddr, "error", err)
			return
		}
		s.doctor.recordUnidentified(host, listenerAddr, err.Error(), now)
		managedLogger().Warn("managed connection refused before it named a client",
			"code", CodeDoctorManagedClientRejected, "remote", remote, "listener", listenerAddr, "error", err)
		return
	}
	claimed := refusal.ShownClaim()
	if _, known := lookup(refusal.Claimed); known {
		s.recordDeclaredRejection(refusal.Claimed, ManagedRejectBadToken, refusal.Error())
	} else {
		s.doctor.recordUnknown(claimed, refusal.Error())
	}
	managedLogger().Warn("managed client rejected", "code", CodeDoctorManagedClientRejected,
		"name", claimed, "remote", remote, "error", err)
}

// ErrManagedNotServing is the answer when no managed config server was started
// in this process: the config declares no client entry on any hub server
// block, or startManagedServer gave up before Start. It is never an empty
// record, which would read as "every listener bound, no client refused".
var ErrManagedNotServing = errors.New("managed client state is not published: no managed config server was started in this process")

// managedDetailMax bounds the detail a rejection keeps. A config ack's error
// text comes from the client, so the hub stores no more of it than this.
const managedDetailMax = 1024

// ManagedListenerUnbound is one listen address Start could not bind, for a
// reason other than the plugin acceptor holding it.
type ManagedListenerUnbound struct {
	Addr  string // The address the server block names, as Start was given it.
	Error string // The socket or certificate failure.
	Fix   string // Certificate-specific repair; empty for socket failures.
}

// ManagedRejectCause says why a managed client was last refused. The zero
// value is not a cause.
type ManagedRejectCause uint8

const (
	// ManagedRejectUnspecified is the zero value and is never recorded.
	ManagedRejectUnspecified ManagedRejectCause = iota
	// ManagedRejectNameConnected means a second session authenticated under
	// a name that already has one, and the hub refused it.
	ManagedRejectNameConnected
	// ManagedRejectConfigRejected means the client acknowledged the pushed or
	// fetched config with an error: it refused to apply it.
	ManagedRejectConfigRejected
	// ManagedRejectUnknownName means a connection claimed a name the hub has
	// no client entry for, so no secret could authenticate it.
	ManagedRejectUnknownName
	// ManagedRejectBadToken means a connection claimed a configured client's
	// name and sent a token that is not that client's secret.
	ManagedRejectBadToken
)

// managedUnknownMax bounds the unknown names the record keeps. A name reaches
// the rejection map only once it is a configured client, which bounds that
// map; an unknown name is chosen by whoever connects, so only the most recent
// managedUnknownMax of them are kept and the oldest is forgotten first.
const managedUnknownMax = 16

// ManagedClientRejection is the last refusal recorded for one client.
type ManagedClientRejection struct {
	Client string
	Cause  ManagedRejectCause
	Detail string // The refusal's own text, bounded by managedDetailMax.
	Time   time.Time
}

// ManagedDeviceRecordFailure names a durable key and the failure reported for it.
type ManagedDeviceRecordFailure struct {
	UnknownDevice bool // A data command created a key with no corresponding device.
	Name          string
	Key           string
	Error         string
}

// managedUnidentifiedMax bounds each per-address class of connections that
// ended before naming a client.
const managedUnidentifiedMax = 16

// managedUnidentifiedKeep is how long a kept remote address stays visible
// without another failure, judged on read so no timer goroutine is needed.
const managedUnidentifiedKeep = time.Hour

// ManagedUnidentifiedFailure summarizes repeated pre-identification failures
// from one remote IP address (without port).
type ManagedUnidentifiedFailure struct {
	Addr         string
	ListenerAddr string
	FirstTime    time.Time
	LastTime     time.Time
	Count        int
	LastError    string
}

// ManagedDoctorRecord is what the managed server publishes for doctor: value
// types only, copied at each read.
type ManagedDoctorRecord struct {
	StartupFailure        *opererr.Error
	Unbound               []ManagedListenerUnbound
	Rejected              []ManagedClientRejection // Sorted by client name.
	Unreadable            []ManagedDeviceRecordFailure
	Unwritable            []ManagedDeviceRecordFailure
	CertificateName       string
	CertificateIssued     bool
	CertRefused           []ManagedUnidentifiedFailure // Sorted by Addr.
	CertRefusedForgotten  int
	Unidentified          []ManagedUnidentifiedFailure // Sorted by Addr.
	UnidentifiedForgotten int
	DevicesByAddress      map[string][]string // Sorted device names per CertRefused Addr.
}

// managedDoctorSource is the managed server doctor reads. Start publishes it
// before it binds anything, so a server whose every address failed is still
// read; Stop withdraws it.
var managedDoctorSource atomic.Pointer[ManagedServer]

// ManagedDoctorSnapshot returns the managed server's listener and client
// record, or ErrManagedNotServing when no managed server was started in this
// process. Safe for concurrent use.
func ManagedDoctorSnapshot() (ManagedDoctorRecord, error) {
	record, running := ManagedRuntimeHealth()
	if !running {
		if record.StartupFailure != nil {
			return record, record.StartupFailure
		}
		return record, ErrManagedNotServing
	}
	return record, nil
}

var managedStartupFailure atomic.Pointer[opererr.Error]

// RecordManagedStartupFailure keeps the failure visible when the server could
// not reach Start and therefore cannot publish its own doctor record.
func RecordManagedStartupFailure(failure *opererr.Error) {
	managedStartupFailure.Store(failure)
}

// ManagedRuntimeHealth supplies the current findings even when a startup error
// prevented publication. The boolean distinguishes a running, healthy server
// from a process without a server.
func ManagedRuntimeHealth() (ManagedDoctorRecord, bool) {
	s := managedDoctorSource.Load()
	if s == nil {
		return ManagedDoctorRecord{StartupFailure: managedStartupFailure.Load()}, false
	}
	record := s.doctor.snapshotAt(s.now())
	s.appendDeviceRecordFailures(&record)
	s.appendCertificateAndAddressDevices(&record)
	return record, true
}

// appendCertificateAndAddressDevices enriches record with the currently
// presented managed certificate identity and the devices whose last session
// came from each address in record.CertRefused.
func (s *ManagedServer) appendCertificateAndAddressDevices(record *ManagedDoctorRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.source != nil {
		record.CertificateName = s.source.name
		record.CertificateIssued = s.source.name == ""
	}
	if len(record.CertRefused) == 0 {
		return
	}
	wanted := make(map[string]struct{}, len(record.CertRefused))
	for _, entry := range record.CertRefused {
		wanted[entry.Addr] = struct{}{}
	}
	byAddr := make(map[string][]string, len(wanted))
	for name, dev := range s.devices {
		if !dev.SourceAddress.IsValid() {
			continue
		}
		addr := dev.SourceAddress.String()
		if _, ok := wanted[addr]; ok {
			byAddr[addr] = append(byAddr[addr], name)
		}
	}
	for addr := range byAddr {
		slices.Sort(byAddr[addr])
	}
	if len(byAddr) > 0 {
		record.DevicesByAddress = byAddr
	}
}

// managedDoctorState is the record one ManagedServer keeps. The unbound list
// is bounded by the configured addresses, the rejection map by the configured
// client names (a name reaches it only when the lookup knows it), the unknown
// map by managedUnknownMax, and each nameless connection class by
// managedUnidentifiedMax. Safe for concurrent use.
type managedDoctorState struct {
	mu                    sync.Mutex
	unbound               []ManagedListenerUnbound
	rejected              map[string]ManagedClientRejection
	unknown               map[string]ManagedClientRejection
	certRefused           map[string]ManagedUnidentifiedFailure
	certRefusedForgotten  int
	unidentified          map[string]ManagedUnidentifiedFailure
	unidentifiedForgotten int
}

// recordUnknown records a connection that claimed a name the hub has no
// client entry for. When managedUnknownMax names are already kept and this
// one is new, the oldest is forgotten.
func (d *managedDoctorState) recordUnknown(claimed, detail string) {
	detail = boundedManagedDetail(detail)
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.unknown == nil {
		d.unknown = make(map[string]ManagedClientRejection)
	}
	if _, kept := d.unknown[claimed]; !kept && len(d.unknown) >= managedUnknownMax {
		oldest := ""
		var oldestTime time.Time
		for name, rejection := range d.unknown {
			if oldest == "" || rejection.Time.Before(oldestTime) {
				oldest, oldestTime = name, rejection.Time
			}
		}
		delete(d.unknown, oldest)
	}
	d.unknown[claimed] = ManagedClientRejection{
		Client: claimed, Cause: ManagedRejectUnknownName, Detail: detail, Time: time.Now(),
	}
}

// recordUnbound records one address Start could not bind.
func (d *managedDoctorState) recordUnbound(addr string, err error) {
	d.recordUnboundWithFix(addr, err, "")
}

func (d *managedDoctorState) recordUnboundWithFix(addr string, err error, fix string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.unbound = append(d.unbound, ManagedListenerUnbound{Addr: addr, Error: err.Error(), Fix: fix})
}

// recordDeclaredRejection serializes refusal reporting with declaration removal.
func (s *ManagedServer) recordDeclaredRejection(client string, cause ManagedRejectCause, detail string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	device := s.devices[client]
	if device != nil && device.declared {
		s.doctor.recordRejected(client, cause, detail)
	}
}

// recordRejected records the last refusal of one client, replacing any
// earlier one.
func (d *managedDoctorState) recordRejected(client string, cause ManagedRejectCause, detail string) {
	detail = boundedManagedDetail(detail)
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.rejected == nil {
		d.rejected = make(map[string]ManagedClientRejection)
	}
	d.rejected[client] = ManagedClientRejection{Client: (&pluginipc.AuthRefusal{Claimed: client}).ShownClaim(), Cause: cause, Detail: detail, Time: time.Now()}
}

// clearRejected forgets a client's refusal once it applied a config.
func (d *managedDoctorState) clearRejected(client string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.rejected, client)
}

// recordCertRefused records one certificate-rejection handshake alert from addr.
func (d *managedDoctorState) recordCertRefused(addr, listenerAddr, detail string, now time.Time) {
	detail = boundedManagedDetail(detail)
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.certRefused == nil {
		d.certRefused = make(map[string]ManagedUnidentifiedFailure)
	}
	recordUnidentifiedClass(d.certRefused, &d.certRefusedForgotten, addr, listenerAddr, detail, now)
}

// recordUnidentified records one non-certificate pre-identification failure from addr.
func (d *managedDoctorState) recordUnidentified(addr, listenerAddr, detail string, now time.Time) {
	detail = boundedManagedDetail(detail)
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.unidentified == nil {
		d.unidentified = make(map[string]ManagedUnidentifiedFailure)
	}
	recordUnidentifiedClass(d.unidentified, &d.unidentifiedForgotten, addr, listenerAddr, detail, now)
}

// recordUnidentifiedClass updates an existing entry or inserts a new one into
// class, pruning expired entries first and evicting the oldest when at
// managedUnidentifiedMax.
func recordUnidentifiedClass(class map[string]ManagedUnidentifiedFailure, forgotten *int, addr, listenerAddr, detail string, now time.Time) {
	pruneUnidentifiedClass(class, now)
	if existing, ok := class[addr]; ok {
		existing.ListenerAddr = listenerAddr
		existing.LastTime = now
		existing.Count++
		existing.LastError = detail
		class[addr] = existing
		return
	}
	if len(class) >= managedUnidentifiedMax {
		oldest := ""
		var oldestTime time.Time
		for key, entry := range class {
			if oldest == "" || entry.LastTime.Before(oldestTime) {
				oldest, oldestTime = key, entry.LastTime
			}
		}
		delete(class, oldest)
		*forgotten++
	}
	class[addr] = ManagedUnidentifiedFailure{
		Addr:         addr,
		ListenerAddr: listenerAddr,
		FirstTime:    now,
		LastTime:     now,
		Count:        1,
		LastError:    detail,
	}
}

// pruneUnidentifiedClass forgets entries that have not repeated for at least
// managedUnidentifiedKeep.
func pruneUnidentifiedClass(class map[string]ManagedUnidentifiedFailure, now time.Time) {
	for addr, entry := range class {
		if now.Sub(entry.LastTime) >= managedUnidentifiedKeep {
			delete(class, addr)
		}
	}
}

// clearAddress removes addr from both nameless-connection failure classes once
// a session from addr authenticates.
func (d *managedDoctorState) clearAddress(addr string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.certRefused, addr)
	delete(d.unidentified, addr)
}

// snapshot copies the record as of time.Now().
func (d *managedDoctorState) snapshot() ManagedDoctorRecord {
	return d.snapshotAt(time.Now())
}

// snapshotAt copies the record as of now, forgetting unidentified entries that
// have not repeated for managedUnidentifiedKeep.
func (d *managedDoctorState) snapshotAt(now time.Time) ManagedDoctorRecord {
	d.mu.Lock()
	defer d.mu.Unlock()
	pruneUnidentifiedClass(d.certRefused, now)
	pruneUnidentifiedClass(d.unidentified, now)
	record := ManagedDoctorRecord{
		Unbound:               slices.Clone(d.unbound),
		CertRefusedForgotten:  d.certRefusedForgotten,
		UnidentifiedForgotten: d.unidentifiedForgotten,
	}
	for _, rejection := range d.rejected {
		record.Rejected = append(record.Rejected, rejection)
	}
	for _, rejection := range d.unknown {
		record.Rejected = append(record.Rejected, rejection)
	}
	slices.SortFunc(record.Rejected, func(a, b ManagedClientRejection) int { return strings.Compare(a.Client, b.Client) })
	for _, entry := range d.certRefused {
		record.CertRefused = append(record.CertRefused, entry)
	}
	slices.SortFunc(record.CertRefused, func(a, b ManagedUnidentifiedFailure) int { return strings.Compare(a.Addr, b.Addr) })
	for _, entry := range d.unidentified {
		record.Unidentified = append(record.Unidentified, entry)
	}
	slices.SortFunc(record.Unidentified, func(a, b ManagedUnidentifiedFailure) int { return strings.Compare(a.Addr, b.Addr) })
	return record
}

// publishManagedDoctor makes s the managed server doctor reads.
func publishManagedDoctor(s *ManagedServer) {
	managedStartupFailure.Store(nil)
	managedDoctorSource.Store(s)
}

// withdrawManagedDoctor withdraws s, unless another server was published
// after it.
func withdrawManagedDoctor(s *ManagedServer) {
	managedRegistryReaders.Lock()
	defer managedRegistryReaders.Unlock()
	managedDoctorSource.CompareAndSwap(s, nil)
}
