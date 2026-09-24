// Design: docs/architecture/wire/nlri-bgpls.md -- native snapshot replacement

package ls_export

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/ls"
	"github.com/ze-software/ze/internal/core/linkstateevents"
)

type exportDomainKey struct {
	source string
	domain linkstateevents.Domain
}

type exportDomain struct {
	generation uint64
	routes     map[string]exportedRoute
}

// The worker mutates sent until shutdown joins it; cleanup then owns that state.
// A peer-up event replaces the session pointer, never a map being reconciled.
type exportPeer struct {
	sent     map[string]exportedRoute
	refresh  uint64 // Protected by topologyExporter.mu.
	replayed uint64 // Written only by the worker after a successful replay.
}

type exportDispatcher interface {
	UpdateRouteWithMeta(context.Context, string, string, map[string]any) (uint32, uint32, error)
}

// topologyExporter stores one encoded replacement per domain and one successful
// advertisement set per attached peer. The wake channel coalesces changes; it
// never drops the latest desired database. Worker and cleanup sends are serialized.
type topologyExporter struct {
	mu         sync.Mutex
	domains    map[exportDomainKey]exportDomain
	peers      map[string]*exportPeer
	config     exportConfig
	ready      bool
	request    bool
	wake       chan struct{}
	dispatch   exportDispatcher
	workerMu   sync.Mutex
	workerStop context.CancelFunc
	workerDone <-chan struct{}
}

func newTopologyExporter(dispatch exportDispatcher) *topologyExporter {
	return &topologyExporter{domains: make(map[exportDomainKey]exportDomain), peers: make(map[string]*exportPeer),
		wake: make(chan struct{}, 1), dispatch: dispatch}
}

func (e *topologyExporter) signal() {
	select {
	case e.wake <- struct{}{}:
	default:
	}
}

func (e *topologyExporter) replace(source string, snapshot *linkstateevents.Snapshot) error {
	if snapshot == nil {
		return errors.New("nil native topology snapshot")
	}
	if len(snapshot.Nodes)+len(snapshot.Links)+len(snapshot.Prefixes)+len(snapshot.SIDs) > linkstateevents.RouteMax {
		return errors.New("native topology snapshot exceeds route limit")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.config.enabled {
		return nil
	}
	key := exportDomainKey{source: source, domain: snapshot.Domain}
	if previous, exists := e.domains[key]; exists && snapshot.Generation < previous.generation {
		return nil
	}
	view := *snapshot
	mapping := key
	mapping.domain.Identifier = 0
	if identifier, configured := e.config.domains[mapping]; configured {
		view.Domain.Identifier = identifier
	}
	routes, err := encodeTopology(&view)
	if err != nil {
		return err
	}
	total := len(routes)
	for domain, state := range e.domains {
		if domain == key {
			continue
		}
		total += len(state.routes)
		for identity, route := range routes {
			if previous, exists := state.routes[identity]; exists {
				if !bytes.Equal(previous.attributes, route.attributes) {
					return fmt.Errorf("native domains disagree on BGP-LS NLRI %x", route.nlri)
				}
			}
		}
	}
	if total > linkstateevents.RouteMax {
		return errors.New("native topology export exceeds route limit")
	}
	// Keep an empty generation tombstone until disable so a late snapshot cannot
	// resurrect a removed domain. Producers serialize generation across teardown.
	e.domains[key] = exportDomain{generation: snapshot.Generation, routes: routes}
	e.signal()
	return nil
}

func (e *topologyExporter) configure(config exportConfig) {
	e.mu.Lock()
	e.config = config
	clear(e.domains)
	e.request = config.enabled
	e.mu.Unlock()
	e.signal()
}

func (e *topologyExporter) peerState(peer string, up bool) {
	e.mu.Lock()
	if up {
		// Each up event starts a new session, which has no advertisements yet.
		e.peers[peer] = &exportPeer{sent: make(map[string]exportedRoute)}
	} else {
		delete(e.peers, peer)
	}
	e.mu.Unlock()
	e.signal()
}

func (e *topologyExporter) peerRefresh(peer string) {
	e.mu.Lock()
	if session := e.peers[peer]; session != nil {
		session.refresh++
	}
	e.mu.Unlock()
	e.signal()
}

func (e *topologyExporter) start(request func() error) error {
	e.mu.Lock()
	e.ready = true
	e.request = false
	e.mu.Unlock()
	if err := request(); err != nil {
		return err
	}
	e.signal()
	return nil
}

// resume starts at most one worker, including after a refused removal rolls
// back or the retained process receives a replacement configuration.
func (e *topologyExporter) resume(ctx context.Context, config exportConfig, request func() error) {
	e.workerMu.Lock()
	defer e.workerMu.Unlock()
	e.configure(config)
	if e.workerStop != nil {
		return
	}
	worker, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	e.workerStop, e.workerDone = stop, done
	// stopWorker MUST join this goroutine before the owning plugin returns.
	go func() {
		defer close(done)
		e.run(worker, request)
	}()
}

func (e *topologyExporter) stopWorker() {
	e.workerMu.Lock()
	defer e.workerMu.Unlock()
	e.stopWorkerLocked()
}

func (e *topologyExporter) stopWorkerLocked() {
	if e.workerStop == nil {
		return
	}
	e.workerStop()
	<-e.workerDone
	e.workerStop, e.workerDone = nil, nil
}

// shutdown retains failed withdrawals for another bye attempt. It leaves
// intake disabled until configuration rollback/re-enable explicitly resumes.
func (e *topologyExporter) shutdown(ctx context.Context) error {
	e.workerMu.Lock()
	defer e.workerMu.Unlock()
	e.stopWorkerLocked()
	e.configure(exportConfig{})
	return e.reconcile(ctx)
}

func (e *topologyExporter) run(ctx context.Context, request func() error) {
	// The exporter worker runs until its owning plugin cancels ctx.
	for {
		select {
		case <-ctx.Done():
			return
		case <-e.wake:
			e.mu.Lock()
			ready, replay := e.ready, e.request
			if ready {
				e.request = false
			}
			e.mu.Unlock()
			if !ready {
				continue
			}
			if replay {
				if err := request(); err != nil {
					exportLogger.Error("native topology snapshot request failed", "error", err)
				}
			}
			if err := e.reconcile(ctx); err != nil {
				exportLogger.Error("native topology advertisement failed", "error", err)
			}
		}
	}
}

func (e *topologyExporter) reconcile(ctx context.Context) error {
	// Copy immutable desired routes under the source lock, then release it
	// before any engine call. Producers can publish while a peer is slow.
	e.mu.Lock()
	desired := make(map[string]exportedRoute)
	for _, domain := range e.domains {
		maps.Copy(desired, domain.routes)
	}
	peers := make([]string, 0, len(e.peers))
	sessions := make(map[string]*exportPeer, len(e.peers))
	refreshes := make(map[string]uint64, len(e.peers))
	for peer, session := range e.peers {
		peers = append(peers, peer)
		sessions[peer] = session
		refreshes[peer] = session.refresh
	}
	e.mu.Unlock()
	slices.Sort(peers)
	var failures []error
	deadline, bounded := ctx.Deadline()
	for i, peer := range peers {
		session := sessions[peer]
		replay := refreshes[peer] != session.replayed
		peerContext := ctx
		var cancel context.CancelFunc
		if bounded {
			// Reserve time for later collectors during bounded cleanup instead
			// of spending the entire shutdown grace on the first stalled peer.
			peerContext, cancel = context.WithTimeout(ctx, time.Until(deadline)/time.Duration(len(peers)-i))
		}
		err := e.reconcilePeer(peerContext, peer, session, desired, replay)
		if cancel != nil {
			cancel()
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("BGP-LS peer %s: %w", peer, err))
			continue
		}
		session.replayed = refreshes[peer]
	}
	return errors.Join(failures...)
}

func (e *topologyExporter) reconcilePeer(ctx context.Context, peer string, session *exportPeer, desired map[string]exportedRoute, replay bool) error {
	sent := session.sent
	keys := make([]string, 0, len(sent))
	for key := range sent {
		if _, exists := desired[key]; !exists {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	slices.Reverse(keys)
	// Keep the old advertisement set across refresh so source removals still
	// withdraw Links before Nodes, before any replacement is announced.
	for _, key := range keys {
		if !e.currentSession(peer, session) {
			return nil
		}
		if err := e.send(ctx, peer, sent[key], true, false); err != nil {
			return err
		}
		delete(sent, key)
	}
	keys = keys[:0]
	for key, route := range desired {
		if previous, exists := sent[key]; exists {
			if !replay {
				if bytes.Equal(previous.attributes, route.attributes) {
					continue
				}
			}
		}
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		if !e.currentSession(peer, session) {
			return nil
		}
		if err := e.send(ctx, peer, desired[key], false, replay); err != nil {
			return err
		}
		sent[key] = desired[key]
	}
	return nil
}

func (e *topologyExporter) currentSession(peer string, session *exportPeer) bool {
	e.mu.Lock()
	current := e.peers[peer] == session
	e.mu.Unlock()
	return current
}

func (e *topologyExporter) send(ctx context.Context, peer string, route exportedRoute, withdraw, replay bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	command := "update hex "
	if !withdraw {
		// ORIGIN IGP and empty AS_PATH are locally originated. The ordinary
		// reactor send path applies per-session AS handling and MP_REACH framing.
		attrs := make([]byte, 11+len(route.attributes))
		copy(attrs, []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x90, 29, 0, 0})
		binary.BigEndian.PutUint16(attrs[9:11], uint16(len(route.attributes)))
		copy(attrs[11:], route.attributes)
		command += "attr set " + hex.EncodeToString(attrs) + " nhop set self "
	}
	command += "nlri " + ls.BGPLSFamily.String()
	if withdraw {
		command += " del "
	} else {
		command += " add "
	}
	command += hex.EncodeToString(route.nlri)
	call, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var meta map[string]any
	if replay {
		// A collector refresh must bypass the engine's Adj-RIB-Out deduplication.
		meta = map[string]any{"replay": true}
	}
	announced, withdrawn, err := e.dispatch.UpdateRouteWithMeta(call, peer, command, meta)
	if err != nil {
		return err
	}
	if withdraw {
		if withdrawn != 1 {
			return errors.New("BGP-LS withdrawal was not accepted")
		}
		return nil
	}
	if announced != 1 {
		return errors.New("BGP-LS announcement was not accepted")
	}
	return nil
}
