// Design: docs/architecture/ospf/ospf-4-component-config.md -- incomplete interface inventory.
// Related: instance.go -- topology read and per-engine warning ownership.
// Related: lsdb/origination.go -- per-area self-LSA preservation.
package ospf

import (
	"context"
	"time"

	ifcomp "github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	ospfiface "github.com/ze-software/ze/internal/plugins/ospf/iface"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const codeOSPFInterfaceAddressesUnavailable = "doctor-ospf-interface-addresses-unavailable"
const interfaceAddressCheckName = "ospf-interface-addresses"

var interfaceAddressCheckDecl = rpc.DoctorCheckDecl{
	Name:  interfaceAddressCheckName,
	Phase: rpc.DoctorPhaseRuntime,
	Codes: []string{codeOSPFInterfaceAddressesUnavailable},
}

func interfaceAddressInventoryError(name string, cause error) *opererr.Error {
	problem := textbuf.New()
	defer problem.Release()
	problem.Str("OSPF cannot read the IPv4 addresses of interface ").Str(name)
	fix := textbuf.New()
	defer fix.Release()
	fix.Str("ze show interface name ").Str(name).Str(" detail")
	return opererr.Wrap(cause, problem.String(),
		"OSPF retains the area's last complete Router and Network LSAs and retries; inspect the interface's address-read error",
		fix.String())
}

// A zero identity names interface "" in instance zero, which enrollment never
// supplies. A zero warning is a pending clear only after pending is set.
// No addresses or topology snapshots are retained in this notification state.
type interfaceAddressIdentity struct {
	name     string
	instance uint8
}

type interfaceAddressWarning struct {
	issue          *opererr.Error
	pending        bool
	deliveryLogged bool
}

// recordInterfaceAddressRead only records transitions. The maintenance worker
// publishes them, so packet-driven reads never perform or retry SDK calls.
func (e *engine) recordInterfaceAddressRead(name string, instance uint8, runtime *ospfiface.Interface, cause error) {
	// A topology-only snapshot is not an enrolled runtime. It MUST NOT enqueue
	// warnings or start maintenance that would outlive the snapshot's caller.
	if runtime == nil {
		return
	}
	e.mu.Lock()
	if e.ctx.Err() != nil {
		e.mu.Unlock()
		return
	}
	if _, enrolled := e.running[name]; !enrolled {
		e.mu.Unlock()
		return
	}
	if e.interfaces[name] != runtime {
		e.mu.Unlock()
		return
	}
	if e.cfg.InstanceID != instance {
		e.mu.Unlock()
		return
	}
	owner := e.addressIssueOwner
	key := interfaceAddressIdentity{name: name, instance: instance}
	changed := owner.queueInterfaceAddressWarning(key, cause)
	e.mu.Unlock()
	if changed {
		owner.startNeighborRetransmitLoop()
	}
}

// queueInterfaceAddressWarning changes notification state only. Enrollment
// callers hold their own engine mu through this mutation, always before the
// base owner's addressIssueMu; no caller holds two engines' mu locks.
func (e *engine) queueInterfaceAddressWarning(key interfaceAddressIdentity, cause error) bool {
	e.addressIssueMu.Lock()
	defer e.addressIssueMu.Unlock()
	warning, warned := e.addressIssues[key]
	if cause == nil {
		if !warned {
			return false
		}
		if warning.issue == nil {
			return false
		}
		// Retain the identity until clear is acknowledged, including when
		// recovery races an in-flight raise or its acknowledgement was lost.
		e.addressIssues[key] = interfaceAddressWarning{pending: true}
		return true
	}
	if warned {
		if warning.issue != nil {
			return false
		}
	}
	if e.addressIssues == nil {
		e.addressIssues = make(map[interfaceAddressIdentity]interfaceAddressWarning)
	}
	issue := interfaceAddressInventoryError(key.name, cause)
	e.addressIssues[key] = interfaceAddressWarning{issue: issue, pending: true}
	e.log.Warn(issue.Error(), "code", codeOSPFInterfaceAddressesUnavailable,
		"interface", key.name, "instance", key.instance)
	return true
}

// publishInterfaceAddressWarnings has one caller: the maintenance ticker.
// Only an unchanged successful clear removes an identity. A recovery recorded
// during a raise stays pending, so even a successful late raise is cleared.
func (e *engine) publishInterfaceAddressWarnings() {
	if e.addressIssueOwner != e {
		return
	}
	publisher, attached := e.state.(*sdk.Plugin)
	if !attached {
		return
	}
	e.addressIssueMu.Lock()
	pending := make(map[interfaceAddressIdentity]*opererr.Error)
	for key, warning := range e.addressIssues {
		if warning.pending {
			pending[key] = warning.issue
		}
	}
	e.addressIssueMu.Unlock()
	for key, issue := range pending {
		ctx, cancel := context.WithTimeout(e.ctx, time.Second)
		err := publishInterfaceAddressWarning(ctx, publisher, key, issue)
		cancel()
		e.addressIssueMu.Lock()
		warning := e.addressIssues[key]
		if warning.issue != issue {
			e.addressIssueMu.Unlock()
			if err != nil {
				e.logInterfaceAddressReportError(key, err)
			}
			continue
		}
		if err == nil {
			if issue == nil {
				delete(e.addressIssues, key)
				e.addressIssueMu.Unlock()
				continue
			}
			warning.pending = false
			e.addressIssues[key] = warning
			e.addressIssueMu.Unlock()
			continue
		}
		logFailure := !warning.deliveryLogged
		warning.deliveryLogged = true
		e.addressIssues[key] = warning
		e.addressIssueMu.Unlock()
		if logFailure {
			e.logInterfaceAddressReportError(key, err)
		}
	}
}

func publishInterfaceAddressWarning(ctx context.Context, publisher *sdk.Plugin, key interfaceAddressIdentity, issue *opererr.Error) error {
	subject := textbuf.New()
	defer subject.Release()
	subject.Str(key.name).Str(" instance ").Uint8(key.instance)
	if issue == nil {
		return publisher.ClearIssue(ctx, codeOSPFInterfaceAddressesUnavailable, subject.String())
	}
	return publisher.RaiseIssue(ctx, sdk.Issue{
		Severity: rpc.ReportWarning, Code: codeOSPFInterfaceAddressesUnavailable,
		Subject: subject.String(), Message: issue.Error(), Fix: issue.Fix(),
	})
}

func (e *engine) logInterfaceAddressReportError(key interfaceAddressIdentity, cause error) {
	problem := textbuf.New()
	defer problem.Release()
	problem.Str("OSPF cannot update the address warning for interface ").Str(key.name)
	issue := opererr.Wrap(cause, problem.String(),
		"The daemon's warning may not reflect the current address inventory; OSPF retries while running",
		"ze show log recent level warn component ospf")
	e.log.Warn(issue.Error(), "code", codeOSPFInterfaceAddressesUnavailable,
		"interface", key.name, "instance", key.instance)
}

// clearInterfaceAddressWarnings runs after worker join. Stored identities
// avoid consulting config under another lock. One independent bounded context
// permits shutdown clears after engine cancellation without an RPC under a lock.
func (e *engine) clearInterfaceAddressWarnings() {
	if e.addressIssueOwner != e {
		// The base publisher outlives a config-only instance removal. Its
		// shared identity state also reconciles a later same-instance re-add.
		e.mu.Lock()
		instance := e.cfg.InstanceID
		e.mu.Unlock()
		owner := e.addressIssueOwner
		owner.addressIssueMu.Lock()
		for key, warning := range owner.addressIssues {
			if key.instance != instance {
				continue
			}
			if warning.issue != nil {
				owner.addressIssues[key] = interfaceAddressWarning{pending: true}
			}
		}
		owner.addressIssueMu.Unlock()
		owner.startNeighborRetransmitLoop()
		return
	}
	e.addressIssueMu.Lock()
	identities := make([]interfaceAddressIdentity, 0, len(e.addressIssues))
	for key := range e.addressIssues {
		identities = append(identities, key)
		e.addressIssues[key] = interfaceAddressWarning{pending: true}
	}
	e.addressIssueMu.Unlock()
	publisher, attached := e.state.(*sdk.Plugin)
	if !attached {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(e.ctx), time.Second)
	defer cancel()
	for _, key := range identities {
		if err := publishInterfaceAddressWarning(ctx, publisher, key, nil); err != nil {
			e.logInterfaceAddressReportError(key, err)
			continue
		}
		e.addressIssueMu.Lock()
		delete(e.addressIssues, key)
		e.addressIssueMu.Unlock()
	}
}

// interfaceAddressDoctorFindings is called only by the SDK Runtime doctor
// callback. It reads enrolled running interfaces, not offline configuration,
// so a prestart check with no iface backend does not invent a runtime failure.
func (m *instanceManager) interfaceAddressDoctorFindings() []rpc.DoctorCheckDiagnostic {
	var findings []rpc.DoctorCheckDiagnostic
	for _, e := range m.snapshot() {
		if e.dispatch.codec.IsV6() {
			continue
		}
		e.mu.Lock()
		names := make([]string, 0, len(e.running))
		for name := range e.running {
			names = append(names, name)
		}
		e.mu.Unlock()
		for _, name := range names {
			if _, err := ifcomp.Addresses(name); err != nil {
				issue := interfaceAddressInventoryError(name, err)
				findings = append(findings, rpc.DoctorCheckDiagnostic{
					Code: codeOSPFInterfaceAddressesUnavailable, Severity: "warning",
					Message: issue.Error(), Fix: issue.Fix(),
				})
			}
		}
	}
	return findings
}
