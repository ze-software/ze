// Design: docs/guide/graceful-restart.md -- original-physical GR commands.
// Related: rib_source_mutation.go -- exact entry selection and output joins.
package rib

import (
	"context"
	"encoding/hex"
	"errors"
	"slices"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// ribPhysicalSource is an original received connection. Zero is invalid; only
// the command parser constructs one. Its immutable identifiers narrow existing
// rights and are never a peer selector or a replacement source of authority.
type ribPhysicalSource struct {
	owner      uint64
	connection uint64
}

func parseRIBPhysicalSource(args []string) (ribPhysicalSource, []string, error) {
	if len(args) < 4 {
		return ribPhysicalSource{}, nil, sourceCommandRefused("Both original owner and connection are required")
	}
	if args[0] != "owner" {
		return ribPhysicalSource{}, nil, sourceCommandRefused("The original owner keyword is missing")
	}
	if args[2] != "connection" {
		return ribPhysicalSource{}, nil, sourceCommandRefused("The original connection keyword is missing")
	}
	owner, err := strconv.ParseUint(args[1], 10, 64)
	if err != nil {
		return ribPhysicalSource{}, nil, sourceCommandRefused("The original owner is not an unsigned integer")
	}
	if owner == 0 {
		return ribPhysicalSource{}, nil, sourceCommandRefused("The original owner must be nonzero")
	}
	connection, err := strconv.ParseUint(args[3], 10, 64)
	if err != nil {
		return ribPhysicalSource{}, nil, sourceCommandRefused("The original connection is not an unsigned integer")
	}
	if connection == 0 {
		return ribPhysicalSource{}, nil, sourceCommandRefused("The original connection must be nonzero")
	}
	return ribPhysicalSource{owner: owner, connection: connection}, args[4:], nil
}

func sourceCommandRefused(problem string) error {
	return opererr.New(problem, "No other peer or physical connection may complete this GR operation", "ze request reload")
}

const (
	cmdRIBSourceRetainRoutes        = "request bgp rib source retain-routes"
	cmdRIBSourceReleaseRoutes       = "request bgp rib source release-routes"
	cmdRIBSourceMarkStale           = "request bgp rib source mark-stale"
	cmdRIBSourcePurgeStale          = "request bgp rib source purge-stale"
	cmdRIBSourceAttachCommunity     = "request bgp rib source attach-community"
	cmdRIBSourceDeleteWithCommunity = "request bgp rib source delete-with-community"

	helpRIBSourceRetainRoutes        = "Retain one original received connection"
	helpRIBSourceReleaseRoutes       = "Release one original retained connection"
	helpRIBSourceMarkStale           = "Mark one original connection stale"
	helpRIBSourcePurgeStale          = "Purge stale routes of one original connection"
	helpRIBSourceAttachCommunity     = "Decorate stale routes of one original connection"
	helpRIBSourceDeleteWithCommunity = "Remove matching stale routes of one original connection"
)

type ribSourceCommandDefinition struct {
	name    string
	help    string
	handler CommandHandler
}

// This one declaration feeds both actual dispatch and both startup inventories.
func ribSourceCommands() []ribSourceCommandDefinition {
	return []ribSourceCommandDefinition{
		{cmdRIBSourceRetainRoutes, helpRIBSourceRetainRoutes, (*RIBManager).retainPhysicalSourceCommand},
		{cmdRIBSourceReleaseRoutes, helpRIBSourceReleaseRoutes, (*RIBManager).releasePhysicalSourceCommand},
		{cmdRIBSourceMarkStale, helpRIBSourceMarkStale, (*RIBManager).markPhysicalSourceCommand},
		{cmdRIBSourcePurgeStale, helpRIBSourcePurgeStale, (*RIBManager).purgePhysicalSourceCommand},
		{cmdRIBSourceAttachCommunity, helpRIBSourceAttachCommunity, (*RIBManager).attachPhysicalSourceCommunityCommand},
		{cmdRIBSourceDeleteWithCommunity, helpRIBSourceDeleteWithCommunity, (*RIBManager).deletePhysicalSourceCommunityCommand},
	}
}

func registerSourceCommands() {
	for _, definition := range ribSourceCommands() {
		if err := registerCommand(definition.name, definition.help, definition.handler); err != nil {
			panic("BUG: original-source RIB command registered twice: " + err.Error())
		}
	}
}

func sourceCommandDecls() []sdk.CommandDecl {
	return []sdk.CommandDecl{
		{Name: cmdRIBSourceRetainRoutes, ShortHelp: helpRIBSourceRetainRoutes, Hidden: true, Shape: shapeMap},
		{Name: cmdRIBSourceReleaseRoutes, ShortHelp: helpRIBSourceReleaseRoutes, Hidden: true, Shape: shapeMap},
		{Name: cmdRIBSourceMarkStale, ShortHelp: helpRIBSourceMarkStale, Hidden: true, Shape: shapeMap},
		{Name: cmdRIBSourcePurgeStale, ShortHelp: helpRIBSourcePurgeStale, Hidden: true, Shape: shapeMap},
		{Name: cmdRIBSourceAttachCommunity, ShortHelp: helpRIBSourceAttachCommunity, Hidden: true, Shape: shapeMap},
		{Name: cmdRIBSourceDeleteWithCommunity, ShortHelp: helpRIBSourceDeleteWithCommunity, Hidden: true, Shape: shapeMap},
	}
}

func physicalSourceAnswer(source ribPhysicalSource, key string, count int) (string, any, error) {
	return statusDone, map[string]any{"peer-owner": strconv.FormatUint(source.owner, 10), "connection-id": strconv.FormatUint(source.connection, 10), key: count}, nil
}

// retainPhysicalSourceCommand prepares only the captured source before DOWN.
// RFC 4724 Section 4.2: "When the Receiving Speaker detects termination of the
// TCP session for a BGP session with a peer that has advertised the Graceful
// Restart Capability, it MUST retain the routes received from the peer for all
// the address families that were previously received in the Graceful Restart
// Capability and MUST mark them as stale routing information."
func (r *RIBManager) retainPhysicalSourceCommand(_ string, args []string) (string, any, error) {
	source, rest, err := parseRIBPhysicalSource(args)
	if err != nil {
		return statusError, nil, err
	}
	if len(rest) < 3 {
		return statusError, nil, sourceCommandRefused("Retention requires on-down and a nonempty family set")
	}
	if rest[0] != "on-down" {
		return statusError, nil, sourceCommandRefused("The source retention handoff must name on-down")
	}
	if rest[1] != "families" {
		return statusError, nil, sourceCommandRefused("The source retention handoff must name its families")
	}
	families := make([]family.Family, 0, len(rest)-2)
	for _, name := range rest[2:] {
		selected, known := family.LookupFamily(name)
		if !known {
			return statusError, nil, sourceCommandRefused("The retained address family is not registered: " + name)
		}
		families = append(families, selected)
	}
	ctx, cancel := context.WithTimeout(context.Background(), rpc.CancellableRequestTimeout)
	defer cancel()
	if err := r.sourceLeases.retainPhysical(ctx, source); err != nil {
		return statusError, nil, err
	}
	r.peerMu.Lock()
	state, err := r.retainPhysicalStateLocked(source)
	if err != nil {
		r.peerMu.Unlock()
		return statusError, nil, err
	}
	earlier := slices.Clone(state.earlier)
	r.retainedPeers[source.owner] = true
	removed := r.removePhysicalEntriesLocked(source, func(fam family.Family) bool {
		for _, kept := range families {
			if kept == fam {
				return false
			}
		}
		return true
	}, func(*storage.RouteEntry) bool { return true })
	r.peerMu.Unlock()
	// The explicit on-down handoff leaves nonretained output to the forwarding
	// DOWN owner; the current keys are still reconciled for ordinary selection.
	r.reconcileSourceKeys(removed.keys)
	r.settleSourceLeases(source.owner)
	return retainedSourceAnswer(source, earlier)
}

// answerKeyEarlierConnections names, in the retain-routes answer, the earlier
// connections of this peer whose routes stay retained beside the named one.
// The GR plugin reads it to address each of them at expiry and End-of-RIB.
const answerKeyEarlierConnections = "earlier-connections"

// retainedSourceAnswer always carries the earlier-connection list, empty for a
// first restart, so a reader tells "none" from an answer it could not read.
// The ids are decimal strings, as connection-id is, to keep the full 64 bits.
func retainedSourceAnswer(source ribPhysicalSource, earlier []uint64) (string, any, error) {
	connections := make([]string, 0, len(earlier))
	for _, connection := range earlier {
		connections = append(connections, textbuf.StringUint(connection))
	}
	return statusDone, map[string]any{
		"peer-owner":                textbuf.StringUint(source.owner),
		"connection-id":             textbuf.StringUint(source.connection),
		"retained-peers":            1,
		answerKeyEarlierConnections: connections,
	}, nil
}

func (r *RIBManager) releasePhysicalSourceCommand(_ string, args []string) (string, any, error) {
	source, rest, err := parseRIBPhysicalSource(args)
	if err != nil {
		return statusError, nil, err
	}
	if len(rest) != 0 {
		return statusError, nil, sourceCommandRefused("Source release has an unbound argument")
	}
	count, err := r.removePhysicalSource(source, nil, func(*storage.RouteEntry) bool { return true }, nil, true)
	if err != nil {
		return statusError, nil, err
	}
	return physicalSourceAnswer(source, "released-routes", count)
}

// markPhysicalSourceCommand keeps the existing active GR state as the timer's
// identity; a reconnect never changes the original scope on that state.
// RFC 4724 Section 4.2: "When the Receiving Speaker detects termination of the
// TCP session for a BGP session with a peer that has advertised the Graceful
// Restart Capability, it MUST retain the routes received from the peer for all
// the address families that were previously received in the Graceful Restart
// Capability and MUST mark them as stale routing information."
func (r *RIBManager) markPhysicalSourceCommand(_ string, args []string) (string, any, error) {
	source, rest, err := parseRIBPhysicalSource(args)
	if err != nil {
		return statusError, nil, err
	}
	if len(rest) < 2 {
		return statusError, nil, sourceCommandRefused("Source stale marking requires restart-time")
	}
	if rest[0] != "restart-time" {
		return statusError, nil, sourceCommandRefused("The restart-time keyword is missing")
	}
	restart, err := strconv.ParseUint(rest[1], 10, 16)
	if err != nil {
		return statusError, nil, sourceCommandRefused("The restart time must be an unsigned 16-bit number of seconds")
	}
	rest = rest[2:]
	level := uint8(1)
	if len(rest) > 0 {
		if rest[0] == "stale-level" {
			if len(rest) < 2 {
				return statusError, nil, sourceCommandRefused("The stale level has no value")
			}
			value, parseErr := strconv.ParseUint(rest[1], 10, 8)
			if parseErr != nil {
				return statusError, nil, sourceCommandRefused("The stale level must be an unsigned octet")
			}
			if value == 0 {
				return statusError, nil, sourceCommandRefused("A stale level must be nonzero")
			}
			level = uint8(value)
			rest = rest[2:]
		}
	}
	selected, err := parsePhysicalSourceFamily(rest)
	if err != nil {
		return statusError, nil, err
	}
	r.peerMu.Lock()
	state := r.grState[source.owner]
	if !source.ownsState(state) {
		r.peerMu.Unlock()
		return statusError, nil, sourceCommandRefused("The original connection has no prepared retention state")
	}
	if r.stopping {
		r.peerMu.Unlock()
		return statusError, nil, sourceCommandRefused("The RIB is stopping")
	}
	marked := 0
	if routes := r.bgpPeers[source.owner]; routes != nil {
		for _, fam := range physicalSourceFamilies(routes, selected) {
			routes.ModifyFamilyAll(fam, func(entry *storage.RouteEntry) {
				if !source.matchesReceived(entry) {
					return
				}
				entry.StaleLevel = level
				marked++
			})
		}
	}
	for _, families := range r.ribOut {
		for fam, routes := range families {
			if selected != nil {
				if *selected != fam {
					continue
				}
			}
			for key, entry := range routes {
				if !source.matchesSent(&entry) {
					continue
				}
				entry.StaleLevel = level
				routes[key] = entry
			}
		}
	}
	r.stopGRExpiryLocked(state)
	now := time.Now()
	// Marking replaces the timer's identity, never the cycle's membership.
	replacement := &peerGRState{source: &source, earlier: state.earlier, StaleAt: now, RestartTime: uint16(restart), ExpiresAt: now.Add(time.Duration(restart) * time.Second)}
	r.grState[source.owner] = replacement
	if restart > 0 {
		r.startGRExpiryLocked(replacement, time.Duration(restart)*time.Second+grTimerMargin, func() { r.autoExpireStale(source.owner, replacement) })
	}
	r.peerMu.Unlock()
	// RFC 9494 Section 4.3: selection reads the newly marked retained entries.
	selectedFamily := family.Family{}
	if selected != nil {
		selectedFamily = *selected
	}
	r.reelectAdvertiseStale(source.owner, selectedFamily)
	r.reconcileFlowSpecs()
	return physicalSourceAnswer(source, "marked", marked)
}

func (r *RIBManager) purgePhysicalSourceCommand(_ string, args []string) (string, any, error) {
	source, rest, err := parseRIBPhysicalSource(args)
	if err != nil {
		return statusError, nil, err
	}
	selected, err := parsePhysicalSourceFamily(rest)
	if err != nil {
		return statusError, nil, err
	}
	// RFC 4724 Section 4.2: purge only stale original received routes.
	count, err := r.removePhysicalSource(source, selected, func(entry *storage.RouteEntry) bool { return entry.StaleLevel > storage.StaleLevelFresh }, nil, false)
	if err != nil {
		return statusError, nil, err
	}
	return physicalSourceAnswer(source, "purged", count)
}

func (r *RIBManager) attachPhysicalSourceCommunityCommand(_ string, args []string) (string, any, error) {
	source, rest, err := parseRIBPhysicalSource(args)
	if err != nil {
		return statusError, nil, err
	}
	selected, community, err := parsePhysicalSourceCommunity(rest)
	if err != nil {
		return statusError, nil, err
	}
	r.peerMu.Lock()
	if !r.physicalSourceKnownLocked(source) {
		r.peerMu.Unlock()
		return statusError, nil, sourceCommandRefused("The original received connection is not retained")
	}
	if r.stopping {
		r.peerMu.Unlock()
		return statusError, nil, sourceCommandRefused("The RIB is stopping")
	}
	attached := 0
	var refused error
	if routes := r.bgpPeers[source.owner]; routes != nil {
		routes.ModifyFamilyAll(selected, func(entry *storage.RouteEntry) {
			if !source.matchesReceived(entry) {
				return
			}
			if entry.StaleLevel == storage.StaleLevelFresh {
				return
			}
			if !r.attachCommunity(entry, community[:]) {
				if refused == nil {
					refused = sourceCommandRefused("An original received community attribute could not be updated")
				}
				return
			}
			entry.StaleLevel = storage.DepreferenceThreshold
			attached++
		})
	}
	// The sent inventory carries its own original physical provenance.
	for _, families := range r.ribOut {
		for key, entry := range families[selected] {
			if !source.matchesSent(&entry) {
				continue
			}
			if entry.StaleLevel == storage.StaleLevelFresh {
				continue
			}
			if !attachSentCommunity(&entry, community[:]) {
				if refused == nil {
					refused = sourceCommandRefused("An original sent community attribute could not be updated")
				}
				continue
			}
			entry.StaleLevel = storage.DepreferenceThreshold
			families[selected][key] = entry
		}
	}
	r.peerMu.Unlock()
	// RFC 9494 Section 4.3.
	r.reelectAdvertiseStale(source.owner, selected)
	r.reconcileFlowSpecs()
	if refused != nil {
		return statusError, nil, refused
	}
	return physicalSourceAnswer(source, "attached", attached)
}

func (r *RIBManager) deletePhysicalSourceCommunityCommand(_ string, args []string) (string, any, error) {
	source, rest, err := parseRIBPhysicalSource(args)
	if err != nil {
		return statusError, nil, err
	}
	selected, community, err := parsePhysicalSourceCommunity(rest)
	if err != nil {
		return statusError, nil, err
	}
	var unavailable error
	count, err := r.removePhysicalSource(source, &selected, func(entry *storage.RouteEntry) bool {
		if entry.StaleLevel == storage.StaleLevelFresh {
			return false
		}
		bundle := entry.GetBundle()
		if !bundle.HasCommunities() {
			return false
		}
		data, lookupErr := pool.Communities.Get(bundle.Communities)
		if lookupErr != nil {
			if unavailable == nil {
				unavailable = opererr.Wrap(lookupErr, "An original community attribute could not be read", "The cleanup cannot decide whether this route carries the requested community", "ze request reload")
			}
			return false
		}
		return containsCommunity(data, community[:])
	}, nil, false)
	if refused := errors.Join(err, unavailable); refused != nil {
		return statusError, nil, refused
	}
	return physicalSourceAnswer(source, "deleted", count)
}

func parsePhysicalSourceFamily(args []string) (*family.Family, error) {
	if len(args) == 0 {
		return nil, nil
	}
	if len(args) != 2 {
		return nil, sourceCommandRefused("The source family selection has an unbound argument")
	}
	if args[0] != "family" {
		return nil, sourceCommandRefused("The source family keyword is missing")
	}
	selected, known := family.LookupFamily(args[1])
	if !known {
		return nil, sourceCommandRefused("The source address family is not registered: " + args[1])
	}
	return &selected, nil
}

func parsePhysicalSourceCommunity(args []string) (family.Family, [4]byte, error) {
	var community [4]byte
	if len(args) != 4 {
		return family.Family{}, community, sourceCommandRefused("Source community operations require family and community")
	}
	selected, err := parsePhysicalSourceFamily(args[:2])
	if err != nil {
		return family.Family{}, community, err
	}
	if args[2] != "community" {
		return family.Family{}, community, sourceCommandRefused("The source community keyword is missing")
	}
	raw, err := hex.DecodeString(args[3])
	if err != nil {
		return family.Family{}, community, sourceCommandRefused("A community must contain exactly eight hexadecimal digits")
	}
	if len(raw) != len(community) {
		return family.Family{}, community, sourceCommandRefused("A community must contain exactly eight hexadecimal digits")
	}
	copy(community[:], raw)
	return *selected, community, nil
}
