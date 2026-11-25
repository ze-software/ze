// Design: docs/architecture/fleet-config.md -- managed listener reload.
// Design: docs/architecture/pki/tls-listeners.md -- certificate preparation and rotation.
package server

import (
	"crypto/tls"
	"errors"
	"fmt"
	"maps"
	"strings"
	"time"

	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/core/configtext"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type managedCertSource struct {
	name  string
	hosts []string
	get   func(*tls.ClientHelloInfo) (*tls.Certificate, error)
}

func cloneManagedListeners(listens []ManagedListener) []ManagedListener {
	cloned := make([]ManagedListener, 0, len(listens))
	for _, listen := range listens {
		listen.Clients = maps.Clone(listen.Clients)
		labels := make(map[string]map[string]string, len(listen.ClientLabels))
		for name, assigned := range listen.ClientLabels {
			labels[name] = maps.Clone(assigned)
		}
		listen.ClientLabels = labels
		cloned = append(cloned, listen)
	}
	return cloned
}

// ManagedListenerPlan carries the certificate resolved against the candidate
// PKI store and the declarations to install after configuration promotion.
type ManagedListenerPlan struct {
	listens     []ManagedListener
	source      *managedCertSource
	bindErr     error
	certificate string
}

// PrepareManagedListeners validates the next configuration without changing any
// active listener, including when only the material behind a name changed.
func (s *ManagedServer) PrepareManagedListeners(listens []ManagedListener, certificate string) (*ManagedListenerPlan, error) {
	s.mu.Lock()
	current, serving := s.source, len(s.listeners) != 0
	s.mu.Unlock()
	cfg := s.certConfig
	cfg.Listeners, cfg.Certificate = listens, certificate
	source, err := managedCertificate(cfg, current)
	if err != nil && certificate != "" {
		return nil, managedNamedCertificateRefusal(listens, certificate, err)
	}
	if err != nil && serving && current != nil && current.name != "" {
		return nil, managedReplacementRefusal(listens, current.name, err)
	}
	return &ManagedListenerPlan{listens: cloneManagedListeners(listens), source: source, bindErr: err, certificate: certificate}, nil
}

// managedNamedCertificateRefusal names the block that selected the certificate.
// API callers can supply a common name without a configuration block.
func managedNamedCertificateRefusal(listens []ManagedListener, certificate string, err error) *opererr.Error {
	name := configtext.QuoteToken(certificate)
	problem := "managed listener certificate " + name + " cannot be served"
	for _, listen := range listens {
		if listen.Certificate == certificate && listen.Block != "" {
			problem = "bus hub server " + configtext.QuoteToken(listen.Block) + " certificate " + name + " cannot be served"
			break
		}
	}
	return opererr.Wrap(err, problem,
		"define pki certificate "+name+" with the leaf PEM in certificate and its matching PEM key in private key, supply its chain to a configured pki ca, then commit",
		"edit pki certificate "+configtext.QuoteCommandWord(certificate))
}

// Each candidate serving block needs the same name, including newly added
// blocks. An absolute merge applies just these leaves in one editor operation.
func managedReplacementRefusal(listens []ManagedListener, certificate string, err error) *opererr.Error {
	const problem = "the managed listener cannot replace its serving certificate"
	why := "keep certificate " + configtext.QuoteToken(certificate) + " on every serving block and keep its pki certificate entry until the local authority can issue the replacement leaf"
	var lines []string
	for _, listen := range listens {
		if listen.Block == "" {
			return opererr.Wrap(err, problem, why, "edit bus hub")
		}
		lines = append(lines, "set bus hub server "+configtext.QuoteCommandWord(listen.Block)+" certificate "+configtext.QuoteCommandWord(certificate))
	}
	if len(lines) == 1 {
		return opererr.Wrap(err, problem, why, lines[0])
	}
	return opererr.Wrap(err, problem, why+". At the load prompt paste these lines, then press Ctrl-D:\n"+strings.Join(lines, "\n")+"\nThe merge preserves the rest of the candidate", "load terminal absolute merge")
}

func managedAuthorityFailure(err error) *opererr.Error {
	if errors.Is(err, errManagedAuthorityLoad) {
		return opererr.Wrap(err, "the managed listener cannot load the local certificate authority",
			"first check store integrity. A clean store can still hold an invalid certificate or mismatched key. Restore meta/ca/cert and meta/ca/key from a trusted copy of this authority before retrying. If the original pair is unavailable, contact the CA administrator to coordinate a trust change",
			"ze data check")
	}
	return opererr.Wrap(err, "the local certificate authority could not issue the managed listener certificate",
		"compare the public root with the issuer trusted by the clients. Correct the reported signing error or give it to the CA administrator, then retry issuance by restarting the hub",
		"ze show pki local-ca pem")
}

// ApplyManagedListeners installs the prepared configuration after promotion.
func (s *ManagedServer) ApplyManagedListeners(plan *ManagedListenerPlan) {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if plan == nil {
		return // No managed declarations were prepared by the caller.
	}
	if s.ctx == nil {
		return // Start has not yet taken ownership of the listeners.
	}
	if s.ctx.Err() != nil {
		return // Shutdown owns the listeners.
	}
	// Named configuration blocks keep their own sockets across reordering. For
	// anonymous API declarations at one address, their order distinguishes them.
	order := make([]managedListenerKey, 0, len(plan.listens))
	next := make(map[managedListenerKey]ManagedListener, len(plan.listens))
	anonymous := make(map[string]int)
	labels := make(map[string]map[string]string)
	for _, listen := range plan.listens {
		key := managedListenerKey{addr: listen.Addr, block: listen.Block}
		if listen.Block == "" {
			key.occurrence = anonymous[listen.Addr]
			anonymous[listen.Addr]++
		}
		order = append(order, key)
		next[key] = listen
		for name := range listen.Clients {
			labels[name] = listen.ClientLabels[name]
		}
	}

	s.mu.Lock()
	if plan.bindErr == nil {
		s.source = plan.source
	}
	var retired []*managedBound
	kept := make(map[managedListenerKey]*managedBound)
	for _, bound := range s.listeners {
		listen, ok := next[bound.key]
		if ok {
			kept[bound.key] = bound
		} else {
			retired = append(retired, bound)
		}
		bound.clients = listen.Clients
	}
	type staleSession struct {
		conn *rpc.MuxConn
		done <-chan struct{}
	}
	var sessions []staleSession
	for name, d := range s.devices {
		if d.bound != nil && d.bound.clients[name] != d.secret {
			sessions = append(sessions, staleSession{d.conn, d.sessionDone})
		}
		if _, declared := labels[name]; !declared && d.declared {
			d.declared, d.labels = false, nil
			s.doctor.clearRejected(name)
			d.RemovedAt = s.now().UTC()
			s.dirtyDeviceLocked(name)
		}
	}
	for name, assigned := range labels {
		d := s.devices[name]
		if d == nil {
			d = &managedDevice{ManagedDeviceRecord: ManagedDeviceRecord{Name: name}}
			s.devices[name] = d
		}
		d.declared, d.labels = true, maps.Clone(assigned)
		if !d.RemovedAt.IsZero() {
			d.RemovedAt = time.Time{}
			s.dirtyDeviceLocked(name)
		}
	}
	s.mu.Unlock()

	for _, bound := range retired {
		_ = bound.listener.Close()
	}
	for _, session := range sessions {
		_ = session.conn.Close()
		<-session.done
	}
	s.doctor.mu.Lock()
	s.doctor.unbound = nil
	s.doctor.mu.Unlock()
	bindFailure := plan.bindErr
	var certFix string
	if bindFailure != nil {
		var problem *opererr.Error
		if plan.certificate == "" {
			problem = managedAuthorityFailure(bindFailure)
		} else {
			problem = managedNamedCertificateRefusal(plan.listens, plan.certificate, bindFailure)
		}
		bindFailure = fmt.Errorf("%s: %w", problem.Why(), bindFailure)
		certFix = problem.Fix()
	}
	active := make([]*managedBound, 0, len(plan.listens))
	for _, key := range order {
		listen := next[key]
		if bound := kept[key]; bound != nil {
			active = append(active, bound)
			continue
		}
		if bindFailure != nil {
			s.doctor.recordUnboundWithFix(listen.Addr, bindFailure, certFix)
			managedLogger().Error("managed listener disabled: its certificate could not be prepared",
				"code", CodeDoctorManagedListenerUnbound, "addr", listen.Addr, "error", bindFailure, "fix", certFix)
			continue
		}
		lns, err := pluginipc.StartListeners([]string{listen.Addr}, s.getCertificate)
		if err != nil {
			s.bindFailed(listen.Addr, err)
			continue
		}
		for _, ln := range lns {
			bound := &managedBound{key: key, listener: ln, clients: listen.Clients}
			active = append(active, bound)
			s.wg.Go(func() { s.acceptLoop(bound) })
		}
	}
	s.mu.Lock()
	s.listeners = active
	s.mu.Unlock()
}

func (s *ManagedServer) getCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	s.mu.Lock()
	source := s.source
	s.mu.Unlock()
	if source == nil {
		return nil, errors.New("managed listener has no certificate")
	}
	return source.get(hello)
}
