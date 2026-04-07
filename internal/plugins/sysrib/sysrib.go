// Design: docs/architecture/core-design.md -- System RIB plugin
//
// System RIB aggregates best routes from all protocol RIBs and selects
// the system-wide best per prefix by administrative distance (lower wins).
// Subscribes to rib/best-change/ prefix on the Bus, publishes sysrib/best-change.
package sysrib

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// loggerPtr is the package-level logger, disabled by default.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// busPtr stores the Bus instance.
var busPtr atomic.Pointer[ze.Bus]

func setBus(b ze.Bus) {
	if b != nil {
		busPtr.Store(&b)
	}
}

func getBus() ze.Bus {
	p := busPtr.Load()
	if p == nil {
		return nil
	}
	return *p
}

// sysribTopic is the Bus topic for system-wide best-path change events.
const sysribTopic = "sysrib/best-change"

// protocolRoute is one protocol's best route for a prefix.
type protocolRoute struct {
	protocol         string
	protocolType     string // "ebgp", "ibgp", "static", etc. for admin distance lookup
	nextHop          string
	priority         int // effective admin distance (lower wins)
	incomingPriority int // original priority from protocol RIB (before override)
	metric           uint32
}

// prefixKey identifies a unique prefix in the system RIB.
type prefixKey struct {
	family string
	prefix string
}

// sysRIB selects across protocols by admin distance.
type sysRIB struct {
	// routes[prefixKey][protocol] = protocolRoute.
	routes map[prefixKey]map[string]*protocolRoute
	// best[prefixKey] = current system best route.
	best map[prefixKey]*protocolRoute
	// adminDist maps protocol type (e.g., "ebgp", "ibgp", "static") to
	// configured admin distance. Empty when no sysrib config is present,
	// in which case incoming priorities pass through unchanged.
	adminDist map[string]int
	mu        sync.RWMutex
}

func newSysRIB() *sysRIB {
	return &sysRIB{
		routes: make(map[prefixKey]map[string]*protocolRoute),
		best:   make(map[prefixKey]*protocolRoute),
	}
}

// parseAdminDistanceConfig extracts the admin-distance map from the sysrib
// config section JSON. Returns an empty map if no admin-distance block is present.
func parseAdminDistanceConfig(jsonData string) (map[string]int, error) {
	var tree map[string]any
	if err := json.Unmarshal([]byte(jsonData), &tree); err != nil {
		return nil, fmt.Errorf("unmarshal sysrib config: %w", err)
	}

	sysribTree, ok := tree["sysrib"].(map[string]any)
	if !ok {
		return make(map[string]int), nil
	}

	adTree, ok := sysribTree["admin-distance"].(map[string]any)
	if !ok {
		return make(map[string]int), nil
	}

	result := make(map[string]int, len(adTree))
	for proto, v := range adTree {
		num, ok := v.(float64)
		if !ok {
			return nil, fmt.Errorf("admin-distance %s: expected number, got %T", proto, v)
		}
		result[proto] = int(num)
	}

	return result, nil
}

// incomingBatch is the JSON payload from protocol RIBs.
type incomingBatch struct {
	Changes []incomingChange `json:"changes"`
}

type incomingChange struct {
	Action       string `json:"action"`
	Prefix       string `json:"prefix"`
	NextHop      string `json:"next-hop"`
	Priority     int    `json:"priority"`
	Metric       uint32 `json:"metric"`
	ProtocolType string `json:"protocol-type"` // "ebgp", "ibgp", etc.
}

// outgoingChange is one entry in the sysrib/best-change payload.
type outgoingChange struct {
	Action   string `json:"action"`
	Prefix   string `json:"prefix"`
	NextHop  string `json:"next-hop,omitempty"`
	Protocol string `json:"protocol"`
}

// outgoingBatch is the JSON payload published to sysrib/best-change.
type outgoingBatch struct {
	Changes []outgoingChange `json:"changes"`
}

// effectivePriority returns the configured admin distance for a protocol type
// if one exists, otherwise returns the incoming priority unchanged.
func (s *sysRIB) effectivePriority(protocolType string, incomingPriority int) int {
	if len(s.adminDist) == 0 {
		return incomingPriority
	}
	if d, ok := s.adminDist[protocolType]; ok {
		return d
	}
	return incomingPriority
}

// processEvent handles a batch of protocol RIB changes from the Bus.
// Returns outgoing changes to publish (caller publishes after processing).
func (s *sysRIB) processEvent(event ze.Event) []outgoingChange {
	proto := event.Metadata["protocol"]
	fam := event.Metadata["family"]
	if proto == "" || fam == "" {
		logger().Warn("sysrib: event missing protocol or family metadata")
		return nil
	}

	var batch incomingBatch
	if err := json.Unmarshal(event.Payload, &batch); err != nil {
		logger().Warn("sysrib: failed to unmarshal batch", "error", err)
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var outChanges []outgoingChange

	for _, c := range batch.Changes {
		if c.Prefix == "" {
			logger().Warn("sysrib: skipping change with empty prefix")
			continue
		}
		if c.Action != "add" && c.Action != "update" && c.Action != "withdraw" {
			logger().Warn("sysrib: unrecognized action", "action", c.Action, "prefix", c.Prefix)
			continue
		}

		key := prefixKey{family: fam, prefix: c.Prefix}

		if c.Action == "add" || c.Action == "update" {
			// Use per-change protocol type for admin distance override.
			// Falls back to batch-level protocol if per-change type is absent.
			protoType := c.ProtocolType
			if protoType == "" {
				protoType = proto
			}
			priority := s.effectivePriority(protoType, c.Priority)

			if s.routes[key] == nil {
				s.routes[key] = make(map[string]*protocolRoute)
			}
			s.routes[key][proto] = &protocolRoute{
				protocol:         proto,
				protocolType:     protoType,
				nextHop:          c.NextHop,
				priority:         priority,
				incomingPriority: c.Priority,
				metric:           c.Metric,
			}
		} else if c.Action == "withdraw" && s.routes[key] != nil {
			delete(s.routes[key], proto)
			if len(s.routes[key]) == 0 {
				delete(s.routes, key)
			}
		}

		if change := s.recomputeBest(key); change != nil {
			outChanges = append(outChanges, *change)
		}
	}

	return outChanges
}

// reapplyAdminDistances recalculates effective priorities for all stored routes
// using the current adminDist map, then recomputes best for each prefix.
// Returns outgoing changes grouped by family. Caller MUST NOT hold s.mu.
func (s *sysRIB) reapplyAdminDistances() map[string][]outgoingChange {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Recalculate effective priority for every stored route.
	for _, protocols := range s.routes {
		for _, route := range protocols {
			route.priority = s.effectivePriority(route.protocolType, route.incomingPriority)
		}
	}

	// Recompute best for all prefixes; collect changes by family.
	changesByFamily := make(map[string][]outgoingChange)
	for key := range s.routes {
		if change := s.recomputeBest(key); change != nil {
			changesByFamily[key.family] = append(changesByFamily[key.family], *change)
		}
	}

	return changesByFamily
}

// recomputeBest selects the system-wide best route for a prefix.
// Returns an outgoing change if the system best changed, nil otherwise.
// Caller MUST hold s.mu.
func (s *sysRIB) recomputeBest(key prefixKey) *outgoingChange {
	protocols := s.routes[key]
	prev := s.best[key]

	if len(protocols) == 0 {
		if prev != nil {
			delete(s.best, key)
			return &outgoingChange{
				Action: "withdraw",
				Prefix: key.prefix,
			}
		}
		return nil
	}

	// Select lowest priority (admin distance). Deterministic tiebreak by protocol name.
	var winner *protocolRoute
	for _, route := range protocols {
		if winner == nil || route.priority < winner.priority ||
			(route.priority == winner.priority && route.protocol < winner.protocol) {
			winner = route
		}
	}

	if prev == nil {
		s.best[key] = winner
		return &outgoingChange{
			Action:   "add",
			Prefix:   key.prefix,
			NextHop:  winner.nextHop,
			Protocol: winner.protocol,
		}
	}

	if prev.protocol == winner.protocol && prev.nextHop == winner.nextHop &&
		prev.priority == winner.priority && prev.metric == winner.metric {
		// Update the pointer so s.best[key] tracks the current route object
		// even when the values are unchanged (the old struct may be stale).
		s.best[key] = winner
		return nil
	}

	s.best[key] = winner
	return &outgoingChange{
		Action:   "update",
		Prefix:   key.prefix,
		NextHop:  winner.nextHop,
		Protocol: winner.protocol,
	}
}

// publishChanges marshals outgoing changes and publishes to the Bus.
func publishChanges(changes []outgoingChange, family string) {
	bus := getBus()
	if bus == nil {
		return
	}

	batch := outgoingBatch{Changes: changes}
	payload, err := json.Marshal(batch)
	if err != nil {
		logger().Warn("sysrib: marshal failed", "error", err)
		return
	}

	metadata := map[string]string{
		"family": family,
	}
	bus.Publish(sysribTopic, payload, metadata)
}

// replayBest publishes the current system best table as batch events.
// Used for full-table replay when a downstream subscriber requests it.
func (s *sysRIB) replayBest() {
	bus := getBus()
	if bus == nil {
		return
	}

	s.mu.RLock()
	changesByFamily := make(map[string][]outgoingChange)
	for key, route := range s.best {
		changesByFamily[key.family] = append(changesByFamily[key.family], outgoingChange{
			Action:   "add",
			Prefix:   key.prefix,
			NextHop:  route.nextHop,
			Protocol: route.protocol,
		})
	}
	s.mu.RUnlock()

	for famName, changes := range changesByFamily {
		batch := outgoingBatch{Changes: changes}
		payload, err := json.Marshal(batch)
		if err != nil {
			logger().Warn("sysrib: replay marshal failed", "error", err)
			continue
		}
		metadata := map[string]string{
			"family": famName,
			"replay": "true",
		}
		bus.Publish(sysribTopic, payload, metadata)
	}

	logger().Info("sysrib: replay published", "families", len(changesByFamily))
}

// sysribReplayConsumer implements ze.Consumer for the sysrib/replay-request topic.
type sysribReplayConsumer struct {
	sysrib *sysRIB
}

// Deliver triggers a full system RIB replay.
func (c *sysribReplayConsumer) Deliver(_ []ze.Event) error {
	c.sysrib.replayBest()
	return nil
}

// busConsumer implements ze.Consumer for Bus subscription.
type busConsumer struct {
	sysrib *sysRIB
}

// Deliver processes a batch of Bus events.
func (c *busConsumer) Deliver(events []ze.Event) error {
	for _, event := range events {
		fam := event.Metadata["family"]
		changes := c.sysrib.processEvent(event)
		if len(changes) > 0 {
			publishChanges(changes, fam)
		}
	}
	return nil
}

// run subscribes to protocol RIB events and blocks until ctx is canceled.
func (s *sysRIB) run(ctx context.Context) {
	bus := getBus()
	if bus == nil {
		logger().Warn("sysrib: no bus configured")
		return
	}

	sub, err := bus.Subscribe("rib/best-change/", nil, &busConsumer{sysrib: s})
	if err != nil {
		logger().Error("sysrib: subscribe failed", "error", err)
		return
	}
	defer bus.Unsubscribe(sub)

	// Subscribe to replay requests from downstream consumers (e.g., fib-kernel).
	replaySub, err := bus.Subscribe("sysrib/replay-request", nil, &sysribReplayConsumer{sysrib: s})
	if err != nil {
		logger().Warn("sysrib: replay request subscribe failed", "error", err)
	} else {
		defer bus.Unsubscribe(replaySub)
	}

	// Request full-table replay from protocol RIBs so we populate
	// even if they started before us.
	bus.Publish("rib/replay-request", nil, nil)

	logger().Info("sysrib: running")
	<-ctx.Done()
	logger().Info("sysrib: stopped")
}

// showRIB returns the current system RIB state as JSON.
func (s *sysRIB) showRIB() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type entry struct {
		Prefix   string `json:"prefix"`
		Family   string `json:"family"`
		NextHop  string `json:"next-hop"`
		Protocol string `json:"protocol"`
		Priority int    `json:"priority"`
	}

	entries := make([]entry, 0, len(s.best))
	for key, route := range s.best {
		entries = append(entries, entry{
			Prefix:   key.prefix,
			Family:   key.family,
			NextHop:  route.nextHop,
			Protocol: route.protocol,
			Priority: route.priority,
		})
	}

	data, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
