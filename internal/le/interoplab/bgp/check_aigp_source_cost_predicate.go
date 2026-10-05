// Design: docs/architecture/testing/interop.md -- exact foreign AIGP observations.
// FRR 10.3.1 bgpd/bgp_route.c: route_vty_out_detail emits aigpMetric, peer and nexthops.
// GoBGP v3.31.0 pkg/packet/bgp/bgp.go: PathAttributeAigp.String renders decoded TLVs.
package bgp

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	aigpDirectPrefix   = "10.10.2.0/24"
	aigpRecoveryPrefix = "10.10.3.0/24"
	aigpFencePrefix    = "10.10.4.0/24"
)

// requireFRRAIGPRoute binds the metric to one exact prefix, source and next hop.
// A missing AIGP attribute is not metric zero and cannot satisfy this proof.
func requireFRRAIGPRoute(output, prefix, zeAddress string, metric uint64) error {
	var route struct {
		Prefix string `json:"prefix"`
		Paths  []struct {
			Metric *uint64 `json:"aigpMetric"`
			ASPath struct {
				String string `json:"string"`
			} `json:"aspath"`
			Peer struct {
				ID string `json:"peerId"`
			} `json:"peer"`
			NextHops []struct {
				IP string `json:"ip"`
			} `json:"nexthops"`
		} `json:"paths"`
	}
	if err := json.Unmarshal([]byte(output), &route); err != nil {
		return fmt.Errorf("decode FRR AIGP route: %w", err)
	}
	if route.Prefix != prefix {
		return fmt.Errorf("FRR returned prefix %q, want %s", route.Prefix, prefix)
	}
	if len(route.Paths) != 1 {
		return fmt.Errorf("FRR holds %d paths for %s, want one", len(route.Paths), prefix)
	}
	path := &route.Paths[0]
	if path.Metric == nil {
		return errors.New("FRR route has no decoded AIGP metric")
	}
	if *path.Metric != metric {
		return fmt.Errorf("FRR decoded AIGP%d, want %d", *path.Metric, metric)
	}
	if path.ASPath.String != "65001 65004" {
		return fmt.Errorf("FRR AS path %q, want 65001 65004", path.ASPath.String)
	}
	if path.Peer.ID != zeAddress {
		return fmt.Errorf("FRR source %q, want %s", path.Peer.ID, zeAddress)
	}
	if len(path.NextHops) != 1 {
		return errors.New("FRR route has no single next hop")
	}
	if path.NextHops[0].IP != zeAddress {
		return fmt.Errorf("FRR next hop %q, want self %s", path.NextHops[0].IP, zeAddress)
	}
	return nil
}

// requireFRRAIGPInventory refuses an unanswered/empty table. The later source
// sentinel and both original routes establish a usable query and a FIFO fence.
func requireFRRAIGPInventory(output string, recovered bool) error {
	var table struct {
		Routes map[string][]json.RawMessage `json:"routes"`
	}
	if err := json.Unmarshal([]byte(output), &table); err != nil {
		return fmt.Errorf("decode FRR inventory: %w", err)
	}
	for _, prefix := range []string{injectPrefixFirst, injectPrefixSecond, aigpDirectPrefix, aigpFencePrefix} {
		if len(table.Routes[prefix]) != 1 {
			return fmt.Errorf("FRR inventory lacks the single control route %s", prefix)
		}
	}
	paths, present := table.Routes[aigpRecoveryPrefix]
	if recovered {
		if len(paths) != 1 {
			return errors.New("FRR inventory lacks the recovered AIGP route")
		}
		return nil
	}
	if present {
		return errors.New("FRR still inventories the cost-withheld route")
	}
	return nil
}

// requireGoBGPAIGPControl reuses the existing one-line route identity parser.
// The delimited full attribute distinguishes 100 from 1000 and duplicate TLVs.
func requireGoBGPAIGPControl(output, sourceHop, thirdPartyHop string) error {
	for _, route := range []struct{ prefix, hop string }{
		{aigpDirectPrefix, sourceHop}, {aigpRecoveryPrefix, thirdPartyHop},
	} {
		if err := requireGoBGPSourceBlock(output, route.prefix, "65004", route.hop); err != nil {
			return err
		}
		decoded, err := gobgpRouteFor(output, route.prefix)
		if err != nil {
			return err
		}
		line := strings.Join(decoded.fields, " ")
		if strings.Count(line, "{Aigp:") != 1 {
			return fmt.Errorf("GoBGP route %s has no single AIGP attribute", route.prefix)
		}
		if !strings.Contains(line, "{Aigp: [{Metric: 100}]}") {
			return fmt.Errorf("GoBGP route %s changed received AIGP100: %s", route.prefix, line)
		}
	}
	return nil
}

// aigpSourceFence is a comparison value, not a raw JSON DTO. Its counters are
// admitted only by readAIGPSourceFence after presence and live-session checks.
type aigpSourceFence struct {
	updates, eor, established uint64
}

func readAIGPSourceFence(output, address string) (aigpSourceFence, error) {
	var detail struct {
		Peers map[string]struct {
			State       string  `json:"state"`
			Updates     *uint64 `json:"updates-received"`
			EOR         *uint64 `json:"eor-received"`
			Established *uint64 `json:"connections-established"`
			Dropped     *uint64 `json:"connections-dropped"`
		} `json:"peers"`
	}
	if err := json.Unmarshal([]byte(output), &detail); err != nil {
		return aigpSourceFence{}, err
	}
	peer, ok := detail.Peers[address]
	if !ok {
		return aigpSourceFence{}, errors.New("source peer detail is absent")
	}
	if !strings.EqualFold(peer.State, "established") {
		return aigpSourceFence{}, errors.New("source session is not established")
	}
	for _, counter := range []*uint64{peer.Updates, peer.EOR, peer.Established, peer.Dropped} {
		if counter == nil {
			return aigpSourceFence{}, errors.New("source session counters are incomplete")
		}
	}
	if *peer.Updates < 4 {
		return aigpSourceFence{}, errors.New("source has not completed its announcement batch")
	}
	if *peer.EOR == 0 {
		return aigpSourceFence{}, errors.New("source has not sent End-of-RIB")
	}
	if *peer.Established != 1 {
		return aigpSourceFence{}, errors.New("source did not retain its original session")
	}
	if *peer.Dropped != 0 {
		return aigpSourceFence{}, errors.New("source session dropped during the proof")
	}
	return aigpSourceFence{updates: *peer.Updates, eor: *peer.EOR, established: *peer.Established}, nil
}

// aigpRecipientFence names the established session, not merely a live receiver.
// Its zero is invalid; readFRRAIGPRecipientFence admits the vendor JSON fields.
type aigpRecipientFence struct {
	established, dropped   uint64
	localPort, foreignPort uint16
}

// FRR 10.3.1 bgp_show_peer emits these fields inside the configured-address key:
// https://github.com/FRRouting/frr/blob/frr-10.3.1/bgpd/bgp_vty.c
// The timestamp is computed from independently rounded clocks and can jitter.
// Counters plus socket ports reject reconnection without that unstable equality;
// they do not identify a daemon replacement that also reuses identical ports.
func readFRRAIGPRecipientFence(output, address string) (aigpRecipientFence, error) {
	var peers map[string]struct {
		State       string  `json:"bgpState"`
		Established *uint64 `json:"connectionsEstablished"`
		Dropped     *uint64 `json:"connectionsDropped"`
		LocalPort   *uint16 `json:"portLocal"`
		ForeignPort *uint16 `json:"portForeign"`
	}
	if err := json.Unmarshal([]byte(output), &peers); err != nil {
		return aigpRecipientFence{}, fmt.Errorf("decode FRR recipient session: %w", err)
	}
	peer, ok := peers[address]
	if !ok {
		return aigpRecipientFence{}, errors.New("FRR recipient neighbor is absent")
	}
	if peer.State != "Established" {
		return aigpRecipientFence{}, errors.New("FRR recipient session is not Established")
	}
	for _, value := range []*uint64{peer.Established, peer.Dropped} {
		if value == nil {
			return aigpRecipientFence{}, errors.New("FRR recipient session epoch is incomplete")
		}
	}
	if *peer.Established == 0 {
		return aigpRecipientFence{}, errors.New("FRR recipient has no established connection")
	}
	for _, port := range []*uint16{peer.LocalPort, peer.ForeignPort} {
		if port == nil {
			return aigpRecipientFence{}, errors.New("FRR recipient socket identity is incomplete")
		}
		if *port == 0 {
			return aigpRecipientFence{}, errors.New("FRR recipient socket port is zero")
		}
	}
	return aigpRecipientFence{
		established: *peer.Established, dropped: *peer.Dropped,
		localPort: *peer.LocalPort, foreignPort: *peer.ForeignPort,
	}, nil
}

func requireFRRAIGPRecipientFence(output, address string, want aigpRecipientFence) error {
	got, err := readFRRAIGPRecipientFence(output, address)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("FRR recipient session changed during metric recovery: before=%+v after=%+v", want, got)
	}
	return nil
}
