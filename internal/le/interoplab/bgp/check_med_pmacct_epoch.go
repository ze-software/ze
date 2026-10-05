// Design: docs/architecture/testing/interop.md -- foreign collector epoch evidence.
// Related: check_med_pmacct.go -- conditional-MED input and selection assertions.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const medWholeSetAdjIn = "Adj-Rib-In Pre-Policy"

// medWholeSetCandidate names independent facts the collector must observe.
// The router is the BMP sender; the peer and Router ID belong to the BGP source.
type medWholeSetCandidate struct {
	router   netip.Addr
	peer     netip.Addr
	routerID netip.Addr
	asn      uint32
	med      uint32
}

func newMEDWholeSetCandidate(network [4]byte, host byte, asn uint32, identifier byte, med uint32) medWholeSetCandidate {
	return medWholeSetCandidate{
		router:   netip.AddrFrom4([4]byte{network[0], network[1], network[2], 2}),
		peer:     netip.AddrFrom4([4]byte{network[0], network[1], network[2], host}),
		routerID: netip.AddrFrom4([4]byte{198, 51, 100, identifier}),
		asn:      asn,
		med:      med,
	}
}

func waitMEDWholeSetInput(ctx context.Context, lab interoplab.CheckerLab, want *medWholeSetCandidate) error {
	var description textbuf.Buffer
	var mismatch error
	last, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 60 * time.Second, Interval: 2 * time.Second,
		Description: description.Str("pmacct route and Peer Up epoch for ").Addr(want.peer).String(),
	}, func(probeCtx context.Context) (string, error) {
		// The scenario produces fewer than 256 rows. Missing an older Peer Up
		// fails closed rather than attributing a route to an unknown epoch.
		return lab.Query(probeCtx, peerPMACCT, []string{"tail", "-n", "256", pmacctMsgLogPath}, nil)
	}, func(output string) bool {
		mismatch = requireMEDWholeSetInput(output, want)
		return mismatch == nil
	})
	if err != nil {
		if mismatch != nil {
			err = fmt.Errorf("%w: %v", err, mismatch)
		}
	}
	return withLastOutput(err, last)
}

// medWholeSetCollectorRow is pmacct's decoded output, not a synthetic UPDATE.
// Peer Up owns bgp_id; Route Monitoring owns the path attributes. Pointer fields
// distinguish an absent metric/sequence/port from an explicitly decoded zero.
type medWholeSetCollectorRow struct {
	Sequence   *uint64         `json:"seq"`
	Event      string          `json:"event_type"`
	Message    string          `json:"bmp_msg_type"`
	Router     netip.Addr      `json:"bmp_router"`
	RouterPort *uint16         `json:"bmp_router_port"`
	Peer       netip.Addr      `json:"peer_ip"`
	ASN        *uint32         `json:"peer_asn"`
	RouterID   netip.Addr      `json:"bgp_id"`
	RIB        string          `json:"bmp_rib_type"`
	LogType    string          `json:"log_type"`
	Prefix     netip.Prefix    `json:"ip_prefix"`
	ASPath     string          `json:"as_path"`
	NextHop    netip.Addr      `json:"bgp_nexthop"`
	Origin     *uint8          `json:"origin"`
	MED        *uint32         `json:"med"`
	LocalPref  json.RawMessage `json:"local_pref"`
	AIGP       json.RawMessage `json:"aigp"`
}

// requireMEDWholeSetInput joins only the current peer epoch on this BMP sender
// and TCP connection. A later Peer Down, new Peer Up or BMP restart invalidates
// earlier routes. No configured Router ID is inserted into a Route Monitoring row.
func requireMEDWholeSetInput(output string, want *medWholeSetCandidate) error {
	var peerUp, route *medWholeSetCollectorRow
	prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 99, 77, 0}), 24)
	rows := 0
	for line := range strings.SplitSeq(output, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		rows++
		if rows > 256 {
			return errors.New("pmacct evidence exceeds the 256-row query bound")
		}
		var row medWholeSetCollectorRow
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return fmt.Errorf("pmacct row %d is not decoded JSON: %w", rows, err)
		}
		if row.Router != want.router {
			continue
		}
		if medWholeSetBMPBoundary(&row) {
			peerUp, route = nil, nil
			continue
		}
		if row.Peer != want.peer {
			continue
		}
		if row.Message == "peer_down" {
			peerUp, route = nil, nil
			continue
		}
		if row.RIB != medWholeSetAdjIn {
			continue
		}
		if row.Message == "peer_up" {
			peerUp, route = &row, nil
			continue
		}
		if row.Message != "route_monitor" {
			continue
		}
		if row.Prefix != prefix {
			continue
		}
		if peerUp == nil {
			continue
		}
		if err := medWholeSetSameEpoch(peerUp, &row); err != nil {
			return err
		}
		route = &row
	}
	if peerUp == nil {
		return errors.New("pmacct has no current Peer Up for the candidate")
	}
	if route == nil {
		return errors.New("pmacct has no candidate route after its current Peer Up")
	}
	return medWholeSetInputCriteria(peerUp, route, want)
}

func medWholeSetBMPBoundary(row *medWholeSetCollectorRow) bool {
	return row.Event == "log_init" || row.Event == "log_close" || row.Message == "init" || row.Message == "term"
}

func medWholeSetSameEpoch(peerUp, route *medWholeSetCollectorRow) error {
	if peerUp.RouterPort == nil {
		return errors.New("pmacct Peer Up has no BMP connection port")
	}
	if route.RouterPort == nil {
		return errors.New("pmacct route has no BMP connection port")
	}
	if *peerUp.RouterPort != *route.RouterPort {
		return errors.New("pmacct route belongs to a different BMP connection than Peer Up")
	}
	if peerUp.Sequence == nil {
		return errors.New("pmacct Peer Up has no sequence")
	}
	if route.Sequence == nil {
		return errors.New("pmacct route has no sequence")
	}
	if *route.Sequence <= *peerUp.Sequence {
		return errors.New("pmacct route does not follow its Peer Up sequence")
	}
	return nil
}

func medWholeSetInputCriteria(peerUp, route *medWholeSetCollectorRow, want *medWholeSetCandidate) error {
	if peerUp.RouterID != want.routerID {
		return fmt.Errorf("pmacct Peer Up Router ID=%s, want %s", peerUp.RouterID, want.routerID)
	}
	if peerUp.ASN == nil {
		return errors.New("pmacct Peer Up has no peer ASN")
	}
	if *peerUp.ASN != want.asn {
		return fmt.Errorf("pmacct Peer Up ASN=%d, want %d", *peerUp.ASN, want.asn)
	}
	if route.ASN == nil {
		return errors.New("pmacct route has no peer ASN")
	}
	if *route.ASN != want.asn {
		return fmt.Errorf("pmacct route ASN=%d, want %d", *route.ASN, want.asn)
	}
	if route.LogType != pmacctLogUpdate {
		return fmt.Errorf("pmacct latest candidate row is %q, not an UPDATE", route.LogType)
	}
	pathAS, err := strconv.ParseUint(route.ASPath, 10, 32)
	if err != nil {
		return fmt.Errorf("pmacct candidate AS_PATH is not one ASN: %w", err)
	}
	if pathAS != uint64(want.asn) {
		return fmt.Errorf("pmacct AS_PATH=%q, want the source ASN %d", route.ASPath, want.asn)
	}
	if route.NextHop != want.peer {
		return fmt.Errorf("pmacct NEXT_HOP=%s, want %s", route.NextHop, want.peer)
	}
	if route.Origin == nil {
		return errors.New("pmacct candidate has no ORIGIN")
	}
	if *route.Origin != 0 {
		return fmt.Errorf("pmacct ORIGIN=%d, want IGP", *route.Origin)
	}
	if route.MED == nil {
		return errors.New("pmacct candidate has no MED")
	}
	if *route.MED != want.med {
		return fmt.Errorf("pmacct MED=%d, want %d", *route.MED, want.med)
	}
	if len(route.LocalPref) != 0 {
		return errors.New("pmacct candidate carries received LOCAL_PREF")
	}
	if len(route.AIGP) != 0 {
		return errors.New("pmacct candidate carries AIGP")
	}
	return nil
}
