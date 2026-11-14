// Design: docs/architecture/testing/interop.md -- concurrent state validation.
// Related: check_special.go -- specialCheckers registry entry.
// Related: check_rfc.go -- parseFRRSessionGeneration, FRR's established count this checker waits on.
// Related: check_linklocal_multihop.go -- withFRRLogTail, the FRR log a failure carries.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/netip"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// rapidReconnectScenario proves that ze's initial sync belongs to the session
// it started for: FRR clears the session three times, each as soon as it
// reports the new session Established, and on the fourth session FRR holds
// exactly the prefixes ze originates, each once, and reports End-of-RIB
// received for every family the scenario negotiates.
const rapidReconnectScenario = "bgp-rapid-reconnect-initial-sync-frr"

const (
	// rapidReconnectClears is how many sessions FRR clears. The session after
	// the last clear is the one judged.
	rapidReconnectClears = 3
	// rapidReconnectPoll is the interval of the wait for each new session.
	// The clear follows the probe that saw the session Established, so a
	// short interval lands the clear inside the initial sync, whose peer-up
	// barrier wait lasts up to 2 s.
	rapidReconnectPoll = 100 * time.Millisecond
	// rapidReconnectSessionWait covers FRR's 1 s start timer after a clear,
	// its 2 s connect timer (frr.conf), ze's own redial and the OPEN exchange.
	rapidReconnectSessionWait = 90 * time.Second
	// rapidReconnectSyncWait covers ze's initial update on the judged
	// session, which carries five prefixes and two End-of-RIB markers.
	rapidReconnectSyncWait = 60 * time.Second
	// rapidReconnectEvidenceBound bounds the read of FRR's log a failure
	// carries. It runs after a wait may have ended ctx, so it gets its own.
	rapidReconnectEvidenceBound = 20 * time.Second
)

// rapidReconnectFamily is one family ze negotiates with FRR in this scenario,
// with the prefixes ze.conf originates in it.
type rapidReconnectFamily struct {
	// key is FRR's JSON name for the family, get_afi_safi_str(afi, safi,
	// true) in FRR 10.7.1: the key of addressFamilyInfo and endOfRibRecv.
	key string
	// words are the AFI and SAFI words of FRR's show command.
	words    string
	prefixes []string
}

// rapidReconnectFamilies are the families and prefixes of the scenario's
// ze.conf. A prefix here that ze.conf does not originate fails the scenario,
// and so does one ze.conf originates that is missing here.
var rapidReconnectFamilies = []rapidReconnectFamily{
	{key: "ipv4Unicast", words: frrIPv4Unicast, prefixes: []string{"10.83.1.0/24", "10.83.2.0/24", "10.83.3.0/24"}},
	{key: "ipv6Unicast", words: frrFamilyIPv6Unicast, prefixes: []string{"2001:db8:83:1::/64", "2001:db8:83:2::/64"}},
}

// checkRapidReconnectInitialSync drives the three clears and judges the
// fourth session.
//
// Every wait ends on FRR's state, never on elapsed time: each clear waits for
// FRR's established count (connectionsEstablished, which bgp_establish
// increments) to reach the session about to be cleared, and the judgement
// waits for End-of-RIB received in every family. FRR resets the End-of-RIB
// flags of a session when it stops (bgp_stop clears af_sflags), so the flags
// read on the fourth session were set by that session's own End-of-RIB.
//
// FRR 10.7.1 sources the judged fields name, read at the frr-10.7.1 tag:
//   - gracefulRestartInfo.endOfRibRecv.<family>: bgp_show_peer_gr_extra_info
//     (bgpd/bgp_vty.c), set only when the peer is Established and FRR received
//     ze's Graceful Restart capability (PEER_CAP_RESTART_RCV).
//   - addressFamilyInfo.<family>.acceptedPrefixCounter: bgp_show_peer_afi
//     (bgpd/bgp_vty.c), p->pcount of the family.
//   - connectionsEstablished and connectionsDropped: bgp_show_peer.
//   - routes.<prefix>[].peerId: route_vty_out (bgpd/bgp_route.c) under
//     show_ip_bgp_neighbor_routes, which lists only the neighbor's paths.
func checkRapidReconnectInitialSync(ctx context.Context, check *interoplab.CheckContext) error {
	if !check.Network.IPv4.IsValid() {
		return errors.New("the rapid reconnect scenario has no selected IPv4 network")
	}
	// Lab.Query runs the command as written, so the address is this run's
	// network rather than the 172.30.0.0/24 the scenario files spell.
	zeAddress := networkHostAddress(check.Network, zeHostNumber)
	neighborCommand := []string{cmdVtysh, "-c", "show bgp neighbor " + zeAddress + " json"}
	assertion := 0
	fail := func(cause error) error {
		evidenceCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), rapidReconnectEvidenceBound)
		defer cancel()
		return checkerFailure(evidenceCtx, check.Lab, rapidReconnectScenario, assertion, withFRRLogTail(evidenceCtx, check.Lab, cause))
	}
	probe := func(probeCtx context.Context) (string, error) {
		return check.Lab.Query(probeCtx, peerFRR, neighborCommand, nil)
	}

	for session := uint64(1); session <= rapidReconnectClears; session++ {
		assertion++
		var last string
		output, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
			Timeout:     rapidReconnectSessionWait,
			Interval:    rapidReconnectPoll,
			Description: "FRR's session with ze Established for the next clear",
		}, probe, func(output string) bool {
			last = output
			generation, err := parseFRRSessionGeneration(output, zeAddress)
			return err == nil && generation >= session
		})
		if err != nil {
			return fail(interoplab.WithSearchedOutput(fmt.Errorf("FRR's session %d with ze never reached Established: %w", session, err), last))
		}
		if err := requireRapidReconnectGeneration(output, zeAddress, session); err != nil {
			return fail(err)
		}
		assertion++
		if _, err := check.Lab.Exec(ctx, peerFRR, []string{cmdVtysh, "-c", "clear bgp " + zeAddress}, nil); err != nil {
			return fail(fmt.Errorf("clear FRR's session %d with ze: %w", session, err))
		}
	}

	assertion++
	final := uint64(rapidReconnectClears + 1)
	var last string
	pending := errors.New("no probe read FRR's neighbor JSON")
	output, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     rapidReconnectSessionWait + rapidReconnectSyncWait,
		Interval:    rapidReconnectPoll,
		Description: "FRR's last session with ze Established with End-of-RIB received in every family",
	}, probe, func(output string) bool {
		last = output
		neighbor, decodeErr := decodeRapidReconnectNeighbor(output, zeAddress)
		if decodeErr != nil {
			pending = decodeErr
			return false
		}
		pending = neighbor.syncPending(final, rapidReconnectFamilies)
		return pending == nil
	})
	if err != nil {
		cause := fmt.Errorf("FRR never reported session %d with ze Established with End-of-RIB received for every family (last reading: %w): %w", final, pending, err)
		return fail(interoplab.WithSearchedOutput(cause, last))
	}

	// One answer carries both the End-of-RIB flags and the accepted counts,
	// so the counts are the ones FRR held when it had processed every
	// End-of-RIB: FRR handles the messages of one connection in order.
	assertion++
	if err := requireRapidReconnectNeighbor(output, zeAddress, final, rapidReconnectFamilies); err != nil {
		return fail(err)
	}
	for _, family := range rapidReconnectFamilies {
		assertion++
		routes, err := check.Lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", "show bgp " + family.words + " neighbors " + zeAddress + " routes json"}, nil)
		if err != nil {
			return fail(fmt.Errorf("read FRR's %s routes from ze: %w", family.key, err))
		}
		if err := requireRapidReconnectRoutes(routes, zeAddress, family); err != nil {
			return fail(interoplab.WithSearchedOutput(err, routes))
		}
	}

	// The session is read once more after the route reads: a write from an
	// earlier session's sync that ended this session, or changed what FRR
	// holds, shows here as a new drop or a changed count.
	assertion++
	output, err = probe(ctx)
	if err != nil {
		return fail(fmt.Errorf("read FRR's session with ze after the route reads: %w", err))
	}
	if err := requireRapidReconnectNeighbor(output, zeAddress, final, rapidReconnectFamilies); err != nil {
		return fail(err)
	}
	return nil
}

// requireRapidReconnectGeneration judges FRR's established count when the
// wait for session saw it Established. A count beyond session is a session
// that ended without a clear from the checker.
func requireRapidReconnectGeneration(output, address string, session uint64) error {
	generation, err := parseFRRSessionGeneration(output, address)
	if err != nil {
		return err
	}
	if generation != session {
		return fmt.Errorf("FRR counts %d established sessions with ze before clear %d, want %d: a session ended that the checker did not clear", generation, session, session)
	}
	return nil
}

// frrRapidReconnectNeighbor is the part of FRR's `show bgp neighbor <address>
// json` answer this checker judges. Counters are pointers, so a field FRR
// did not print is refused rather than read as zero.
type frrRapidReconnectNeighbor struct {
	State           string  `json:"bgpState"`
	Established     *uint64 `json:"connectionsEstablished"`
	Dropped         *uint64 `json:"connectionsDropped"`
	GracefulRestart *struct {
		EndOfRIBReceived map[string]bool `json:"endOfRibRecv"`
	} `json:"gracefulRestartInfo"`
	Families map[string]struct {
		Accepted *uint64 `json:"acceptedPrefixCounter"`
	} `json:"addressFamilyInfo"`
}

// decodeRapidReconnectNeighbor decodes FRR's answer, which holds one object
// per neighbor keyed by its address, and answers ze's.
func decodeRapidReconnectNeighbor(output, address string) (frrRapidReconnectNeighbor, error) {
	var neighbors map[string]frrRapidReconnectNeighbor
	if err := json.Unmarshal([]byte(output), &neighbors); err != nil {
		return frrRapidReconnectNeighbor{}, fmt.Errorf("decode FRR's neighbor JSON: %w", err)
	}
	neighbor, found := neighbors[address]
	if !found {
		return frrRapidReconnectNeighbor{}, fmt.Errorf("FRR's neighbor JSON holds no neighbor %s", address)
	}
	return neighbor, nil
}

// syncPending answers what the neighbor still lacks before the wait hands
// its answer to requireRapidReconnectNeighbor: Established on session final
// or a later one, with End-of-RIB received in every family. A nil answer is
// that point.
func (n frrRapidReconnectNeighbor) syncPending(final uint64, families []rapidReconnectFamily) error {
	if n.State != stateEstablished {
		return fmt.Errorf("FRR's session with ze is %q", n.State)
	}
	if n.Established == nil {
		return errors.New("FRR printed no connectionsEstablished")
	}
	if *n.Established < final {
		return fmt.Errorf("FRR counts %d established sessions, want %d", *n.Established, final)
	}
	if n.GracefulRestart == nil {
		return errors.New("FRR printed no gracefulRestartInfo")
	}
	for _, family := range families {
		if !n.GracefulRestart.EndOfRIBReceived[family.key] {
			return fmt.Errorf("FRR has no End-of-RIB from ze for %s yet", family.key)
		}
	}
	return nil
}

// requireRapidReconnectNeighbor judges FRR's view of the session after the
// clears: Established on session final, after exactly the checker's clears,
// with End-of-RIB received in exactly the scenario's families and, in each,
// as many accepted prefixes as ze.conf originates.
func requireRapidReconnectNeighbor(output, address string, final uint64, families []rapidReconnectFamily) error {
	neighbor, err := decodeRapidReconnectNeighbor(output, address)
	if err != nil {
		return err
	}
	if neighbor.State != stateEstablished {
		return fmt.Errorf("FRR's session with ze is %q, want %s", neighbor.State, stateEstablished)
	}
	if neighbor.Established == nil {
		return errors.New("FRR's neighbor JSON for ze carries no connectionsEstablished, so the number of sessions is unknown")
	}
	if neighbor.Dropped == nil {
		return errors.New("FRR's neighbor JSON for ze carries no connectionsDropped, so the number of sessions is unknown")
	}
	if *neighbor.Established != final {
		return fmt.Errorf("FRR counts %d established sessions with ze, want %d: a session ended that the checker did not clear", *neighbor.Established, final)
	}
	if *neighbor.Dropped != final-1 {
		return fmt.Errorf("FRR counts %d dropped sessions with ze, want %d, one for each clear: a session ended that the checker did not clear", *neighbor.Dropped, final-1)
	}
	if neighbor.GracefulRestart == nil {
		return errors.New("FRR's neighbor JSON for ze carries no gracefulRestartInfo, so End-of-RIB received cannot be read")
	}
	if len(neighbor.GracefulRestart.EndOfRIBReceived) == 0 {
		return errors.New("FRR reports End-of-RIB received from ze in no family on the last session. FRR prints the flags only for a peer whose Graceful Restart capability it received (bgp_show_peer_gr_extra_info), so either ze sent no End-of-RIB or ze.conf no longer advertises `capability { graceful-restart { ... } }`")
	}
	wanted := make(map[string]bool, len(families))
	for _, family := range families {
		wanted[family.key] = true
		if !neighbor.GracefulRestart.EndOfRIBReceived[family.key] {
			return fmt.Errorf("FRR reports no End-of-RIB received from ze for %s on the last session: ze never closed that family's initial update on the session that stayed", family.key)
		}
		counters, found := neighbor.Families[family.key]
		if !found {
			return fmt.Errorf("FRR's neighbor JSON for ze carries no addressFamilyInfo for %s", family.key)
		}
		if counters.Accepted == nil {
			return fmt.Errorf("FRR's neighbor JSON for ze carries no acceptedPrefixCounter for %s", family.key)
		}
		if *counters.Accepted != uint64(len(family.prefixes)) {
			return fmt.Errorf("FRR accepted %d %s prefixes from ze once it had ze's End-of-RIB for the family, want the %d ze.conf originates: the End-of-RIB reached FRR before the routes it closes, or a route was lost across the clears", *counters.Accepted, family.key, len(family.prefixes))
		}
	}
	for _, key := range slices.Sorted(maps.Keys(neighbor.GracefulRestart.EndOfRIBReceived)) {
		if !wanted[key] {
			return fmt.Errorf("FRR reports End-of-RIB received from ze for %s, a family the scenario does not list: add it and its prefixes to rapidReconnectFamilies", key)
		}
	}
	for _, key := range slices.Sorted(maps.Keys(neighbor.Families)) {
		if !wanted[key] {
			return fmt.Errorf("FRR negotiates %s with ze, a family the scenario does not list: add it and its prefixes to rapidReconnectFamilies", key)
		}
	}
	return nil
}

// frrNeighborRoutes is FRR's answer to `show bgp <afi> <safi> neighbors
// <address> routes json`: the table header and one array of paths per prefix.
// Routes is nil only when FRR printed no table.
type frrNeighborRoutes struct {
	Routes map[string][]struct {
		PeerID string `json:"peerId"`
	} `json:"routes"`
}

// requireRapidReconnectRoutes judges FRR's routes from ze in one family: each
// prefix ze.conf originates is held by exactly one path, that path is ze's,
// and no other prefix from ze is held.
func requireRapidReconnectRoutes(output, address string, family rapidReconnectFamily) error {
	var table frrNeighborRoutes
	if err := json.Unmarshal([]byte(output), &table); err != nil {
		return fmt.Errorf("decode FRR's %s routes from ze: %w", family.key, err)
	}
	if table.Routes == nil {
		return fmt.Errorf("FRR printed no %s route table for ze, so the routes it holds are unknown", family.key)
	}
	// held maps each prefix FRR holds from ze to its path count and to the
	// text FRR printed for it, so a report names the prefix as FRR wrote it.
	held := make(map[netip.Prefix]int, len(table.Routes))
	printed := make(map[netip.Prefix]string, len(table.Routes))
	for _, key := range slices.Sorted(maps.Keys(table.Routes)) {
		prefix, err := netip.ParsePrefix(key)
		if err != nil {
			return fmt.Errorf("FRR's %s routes from ze hold %q, which is not a prefix: %w", family.key, key, err)
		}
		for _, path := range table.Routes[key] {
			if path.PeerID != address {
				return fmt.Errorf("FRR lists a path for %s from %q among ze's %s routes, want only paths from %s", key, path.PeerID, family.key, address)
			}
		}
		held[prefix] = len(table.Routes[key])
		printed[prefix] = key
	}
	for _, text := range family.prefixes {
		prefix, err := netip.ParsePrefix(text)
		if err != nil {
			return fmt.Errorf("the scenario's %s prefix %q is not a prefix: %w", family.key, text, err)
		}
		paths, found := held[prefix]
		if !found {
			return fmt.Errorf("FRR holds no route for %s from ze on the last session, although ze.conf originates it and FRR has ze's End-of-RIB for %s", text, family.key)
		}
		if paths != 1 {
			return fmt.Errorf("FRR holds %d paths for %s from ze, want exactly one", paths, text)
		}
		delete(held, prefix)
	}
	if len(held) > 0 {
		extra := make([]string, 0, len(held))
		for prefix := range held {
			extra = append(extra, printed[prefix])
		}
		slices.Sort(extra)
		return fmt.Errorf("FRR holds %v from ze in %s, which ze.conf does not originate", extra, family.key)
	}
	return nil
}
