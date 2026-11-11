// Design: docs/architecture/fleet-config.md -- what the managed config server tells doctor
// Related: managed_serve.go -- ManagedServer, whose Start, handleConn and handleRequest record here
// Related: ../doctor/check_managed_runtime.go -- the runtime checks that read ManagedDoctorSnapshot

package server

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/diagnostic"
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
		"code", diagnostic.CodeDoctorManagedClientNotifyDropped, "name", client, "cause", cause)
	report.RaiseErrorFix(managedReportSource, diagnostic.CodeDoctorManagedClientNotifyDropped, client,
		message.String(), fix.String())
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
	Error string // The bind error the kernel returned, as text.
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
)

// ManagedClientRejection is the last refusal recorded for one client.
type ManagedClientRejection struct {
	Client string
	Cause  ManagedRejectCause
	Detail string // The refusal's own text, bounded by managedDetailMax.
	Time   time.Time
}

// ManagedDoctorRecord is what the managed server publishes for doctor: value
// types only, copied at each read.
type ManagedDoctorRecord struct {
	Unbound  []ManagedListenerUnbound
	Rejected []ManagedClientRejection // Sorted by client name.
}

// managedDoctorSource is the managed server doctor reads. Start publishes it
// before it binds anything, so a server whose every address failed is still
// read; Stop withdraws it.
var managedDoctorSource atomic.Pointer[ManagedServer]

// ManagedDoctorSnapshot returns the managed server's listener and client
// record, or ErrManagedNotServing when no managed server was started in this
// process. Safe for concurrent use.
func ManagedDoctorSnapshot() (ManagedDoctorRecord, error) {
	s := managedDoctorSource.Load()
	if s == nil {
		return ManagedDoctorRecord{}, ErrManagedNotServing
	}
	return s.doctor.snapshot(), nil
}

// managedDoctorState is the record one ManagedServer keeps. The unbound list
// is bounded by the configured addresses, the rejection map by the configured
// client names. Safe for concurrent use.
type managedDoctorState struct {
	mu       sync.Mutex
	unbound  []ManagedListenerUnbound
	rejected map[string]ManagedClientRejection
}

// recordUnbound records one address Start could not bind.
func (d *managedDoctorState) recordUnbound(addr string, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.unbound = append(d.unbound, ManagedListenerUnbound{Addr: addr, Error: err.Error()})
}

// recordRejected records the last refusal of one client, replacing any
// earlier one.
func (d *managedDoctorState) recordRejected(client string, cause ManagedRejectCause, detail string) {
	if len(detail) > managedDetailMax {
		detail = detail[:managedDetailMax]
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.rejected == nil {
		d.rejected = make(map[string]ManagedClientRejection)
	}
	d.rejected[client] = ManagedClientRejection{Client: client, Cause: cause, Detail: detail, Time: time.Now()}
}

// clearRejected forgets a client's refusal once it applied a config.
func (d *managedDoctorState) clearRejected(client string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.rejected, client)
}

// snapshot copies the record.
func (d *managedDoctorState) snapshot() ManagedDoctorRecord {
	d.mu.Lock()
	defer d.mu.Unlock()
	record := ManagedDoctorRecord{Unbound: slices.Clone(d.unbound)}
	for _, rejection := range d.rejected {
		record.Rejected = append(record.Rejected, rejection)
	}
	slices.SortFunc(record.Rejected, func(a, b ManagedClientRejection) int { return strings.Compare(a.Client, b.Client) })
	return record
}

// publishManagedDoctor makes s the managed server doctor reads.
func publishManagedDoctor(s *ManagedServer) {
	managedDoctorSource.Store(s)
}

// withdrawManagedDoctor withdraws s, unless another server was published
// after it.
func withdrawManagedDoctor(s *ManagedServer) {
	managedDoctorSource.CompareAndSwap(s, nil)
}
