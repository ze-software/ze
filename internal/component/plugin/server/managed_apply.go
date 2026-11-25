// Design: docs/architecture/fleet-config.md -- managed listener reload.
// Design: docs/architecture/pki/tls-listeners.md -- certificate preparation and rotation.
package server

import (
	"crypto/tls"
	"errors"
	"fmt"
	"maps"
	"time"

	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
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
	listens []ManagedListener
	source  *managedCertSource
	bindErr error
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
		var block string
		for _, l := range listens {
			if l.Certificate == certificate {
				block = l.Block
				break
			}
		}
		return nil, fmt.Errorf("bus hub server %s certificate %s: %w; define pki certificate %s with a matching key, or remove the certificate leaf", block, certificate, err, certificate)
	}
	if err != nil && serving && current != nil && current.name != "" {
		return nil, fmt.Errorf("managed listener: %w; keep certificate %s in the configuration until the authority issues again", err, current.name)
	}
	return &ManagedListenerPlan{listens: cloneManagedListeners(listens), source: source, bindErr: err}, nil
}

// ApplyManagedListeners installs the prepared configuration after promotion.
func (s *ManagedServer) ApplyManagedListeners(plan *ManagedListenerPlan) {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
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
	if plan.source != nil {
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
	active := make([]*managedBound, 0, len(plan.listens))
	for _, key := range order {
		listen := next[key]
		if bound := kept[key]; bound != nil {
			active = append(active, bound)
			continue
		}
		if plan.bindErr != nil {
			s.doctor.recordUnbound(listen.Addr, plan.bindErr)
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
