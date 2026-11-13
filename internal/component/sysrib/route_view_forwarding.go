// Design: docs/architecture/rib/unified-locrib.md -- authoritative selected-route consumption.
// Related: sysrib.go -- common forwarding, resolution and ECMP evaluation.

package sysrib

import (
	"slices"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

type routeViewSelectedChange interface {
	selectedKey() prefixKey
}

type routeViewSelectedAdd struct {
	key   prefixKey
	route protocolRoute
}

type routeViewSelectedRemove struct{ key prefixKey }

func (change routeViewSelectedAdd) selectedKey() prefixKey    { return change.key } //nolint:gocritic // hugeParam: a value variant; a pointer variant would admit a typed nil
func (change routeViewSelectedRemove) selectedKey() prefixKey { return change.key }

// applySelected consumes the authority's one selected value for a prefix.
// The caller MUST own its slices and apply the complete reader batch first.
// The change is read and never retained.
func (s *sysRIB) applySelected(change *locrib.Change) ([]outgoingChange, error) {
	prepared, err := prepareRouteViewSelected(change)
	if err != nil {
		return nil, err
	}
	batches, err := s.applySelectedBatch([]routeViewSelectedChange{prepared})
	return batches[change.Family], err
}

// prepareRouteViewSelected copies all retained metadata while the authority or
// projection callback still owns its borrowed path. No forward handle escapes.
func prepareRouteViewSelected(change *locrib.Change) (routeViewSelectedChange, error) {
	return prepareRouteViewSelectedMetadata(change, nil)
}

// prepareRouteViewSelectedMetadata reads change and members and retains
// neither: every slice the result keeps is cloned.
func prepareRouteViewSelectedMetadata(change *locrib.Change, members []projectionMemberMetadata) (routeViewSelectedChange, error) {
	key := prefixKey{family: change.Family, prefix: change.Prefix}
	if !key.prefix.IsValid() || key.prefix != key.prefix.Masked() || key.family == (family.Family{}) {
		return nil, routeViewRefusal("The selected routing change has no valid prefix or family", "No forwarding state was changed", "ze request reload")
	}
	switch change.Kind {
	case locrib.ChangeRemove:
		return routeViewSelectedRemove{key: key}, nil
	case locrib.ChangeAdd, locrib.ChangeUpdate:
		path := &change.Best
		protocol := redistevents.ProtocolName(path.Source)
		if protocol == "" {
			return nil, routeViewRefusal("The selected routing path has no registered source", "A consumer cannot invent protocol ownership", "ze request reload")
		}
		protocolType := protocol
		if path.IsBGP {
			if path.IsEBGP {
				protocolType = "ebgp"
			} else {
				protocolType = "ibgp"
			}
		}
		metadata := slices.Clone(members)
		for i := range members {
			metadata[i] = projectionMemberMetadata{source: members[i].source, labels: slices.Clone(members[i].labels), sid: members[i].sid}
		}
		return routeViewSelectedAdd{key: key, route: protocolRoute{
			protocol: protocol, protocolType: protocolType, osInstalled: protocolInstalledByOS(protocol),
			nextHop: path.NextHop, nextHopInterface: path.Interface, nextHopWeight: path.Weight, nextHopOnLink: path.OnLink,
			priority: int(path.AdminDistance),
			metric:   path.Metric, labels: slices.Clone(path.Labels), srv6SID: path.SRv6SID, routeType: path.RouteType,
			backupNextHop: path.BackupNextHop, backupLabels: slices.Clone(path.BackupRepairLabels),
			backupInterface: path.BackupInterface, backupOnLink: path.BackupOnLink, ecmpNextHops: slices.Clone(change.ECMP), ecmpMetadata: metadata,
		}}, nil
	case locrib.ChangeUnspecified:
		return nil, routeViewRefusal("The selected routing change has no action", "No forwarding state was changed", "ze request reload")
	default:
		return nil, routeViewRefusal("The selected routing change has an unknown action", "No forwarding state was changed", "ze request reload")
	}
}

// applySelectedBatch installs every selected cache value before forwarding
// evaluation. The caller MUST first update the complete LPM reader batch.
// The returned changes are owned values; publication happens off these locks.
func (s *sysRIB) applySelectedBatch(changes []routeViewSelectedChange) (map[family.Family][]outgoingChange, error) {
	for _, change := range changes {
		switch change.(type) {
		case routeViewSelectedAdd, routeViewSelectedRemove:
		default:
			return nil, routeViewRefusal("The selected routing batch contains an unprepared value", "No part of the batch was applied", "ze request reload")
		}
	}
	// Empty selected pages are idle polls, not received routing changes.
	if len(changes) != 0 {
		if current := sysribMetricsPtr.Load(); current != nil {
			current.eventsReceived.Inc()
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := make([]prefixKey, 0, len(changes))
	seen := make(map[prefixKey]struct{}, len(changes))
	for _, change := range changes {
		key := change.selectedKey()
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			keys = append(keys, key)
		}
		s.untrackNextHopGroup(key)
		switch value := change.(type) {
		case routeViewSelectedAdd:
			route := value.route
			s.routes[key] = map[string]*protocolRoute{route.protocol: &route}
		case routeViewSelectedRemove:
			delete(s.routes, key)
		}
	}
	batches := make(map[family.Family][]outgoingChange)
	for _, key := range keys {
		var selected *protocolRoute
		// Every changed cache slot contains exactly the one authority value.
		// Reading that value is not selection across candidate protocols.
		for _, route := range s.routes[key] {
			selected = route
			break
		}
		if output := s.recomputeForwarding(key, selected); output != nil {
			batches[key.family] = append(batches[key.family], *output)
		}
	}
	if current := sysribMetricsPtr.Load(); current != nil {
		for _, batch := range batches {
			for i := range batch {
				if counter := current.routeChanges[batch[i].Action]; counter != nil {
					counter.Inc()
				}
			}
		}
		current.routesBest.Set(float64(len(s.best)))
	}
	return batches, nil
}
