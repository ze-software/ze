// Design: docs/architecture/testing/interop.md -- ordinary native transit verdicts.
// Related: check_best_route_transit.go -- real source/receiver orchestration.
package bgp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"
)

func requireBestRouteTransitInputs(output string, source, backup netip.Addr) error {
	var document struct {
		Peers map[string][]struct {
			Key string `json:"key"`
		} `json:"adj-rib-in"`
	}
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		return err
	}
	for _, peer := range []netip.Addr{source, backup} {
		found := false
		for _, route := range document.Peers[peer.String()] {
			if route.Key == "ipv4/unicast:"+bestRouteTransitSubject {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("original native %s has no retained transit candidate", peer)
		}
	}
	return nil
}

func requireBestRouteTransitSelected(output string, source netip.Addr) error {
	var paths []struct {
		Prefix string `json:"prefix"`
		Peer   string `json:"best-peer"`
	}
	if err := decodeZePipedRows(output, zeBestPathRowsKey, zePipePrefix, &paths); err != nil {
		return err
	}
	if len(paths) != 1 {
		return errors.New("denied native prefix has no unique elected Ze route")
	}
	if paths[0].Prefix != bestRouteTransitDenied {
		return errors.New("denied-route answer from Ze names another prefix")
	}
	peer, err := netip.ParseAddr(paths[0].Peer)
	if err != nil {
		return err
	}
	if peer != source {
		return errors.New("denied native prefix elected another source on Ze")
	}
	return nil
}

func requireBestRouteTransitFRR(reading bestRouteTransitSnapshot, path string, nextHop netip.Addr) error {
	for _, question := range []struct {
		prefix  string
		present bool
	}{
		{bestRouteTransitSubject, path != ""}, {bestRouteTransitControl, true}, {bestRouteTransitDenied, false},
	} {
		present, measured := jsonHasRoute(reading.table, question.prefix)
		if !measured {
			return errors.New("FRR transit table is unmeasured")
		}
		if present != question.present {
			return fmt.Errorf("FRR transit table %s present=%t, want %t", question.prefix, present, question.present)
		}
	}
	if err := requireBestRouteTransitFRRRoute(reading.control, bestRouteTransitControl, "65000 65001", nextHop); err != nil {
		return err
	}
	if path != "" {
		return requireBestRouteTransitFRRRoute(reading.subject, bestRouteTransitSubject, path, nextHop)
	}
	return nil
}

func requireBestRouteTransitFRRRoute(output, prefix, path string, nextHop netip.Addr) error {
	var document struct {
		Prefix string `json:"prefix"`
		Paths  []struct {
			Valid *bool `json:"valid"`
		} `json:"paths"`
	}
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		return err
	}
	if document.Prefix != prefix {
		return fmt.Errorf("FRR answered prefix %q, want %s", document.Prefix, prefix)
	}
	if len(document.Paths) != 1 {
		return errors.New("FRR transit route has no unique received path")
	}
	if document.Paths[0].Valid == nil {
		return errors.New("FRR transit route omits native validity")
	}
	if !*document.Paths[0].Valid {
		return errors.New("FRR transit route is not valid")
	}
	if err := requireFRRASPath(output, path); err != nil {
		return err
	}
	hops, err := parseFRRNextHops(output)
	if err != nil {
		return err
	}
	return requireSoleNextHop(hops, nextHop)
}

func requireBestRouteTransitBIRD(reading bestRouteTransitSnapshot, path string) error {
	prefixes := bestRouteTransitPrefixFields(reading.table)
	for _, question := range []struct {
		prefix  string
		present bool
	}{
		{bestRouteTransitSubject, path != ""}, {bestRouteTransitControl, true}, {bestRouteTransitDenied, false},
	} {
		prefix, err := netip.ParsePrefix(question.prefix)
		if err != nil {
			return err
		}
		if prefixes[prefix] != question.present {
			return fmt.Errorf("BIRD transit table %s present=%t, want %t", prefix, prefixes[prefix], question.present)
		}
	}
	if err := requireBestRouteTransitBIRDRoute(reading.control, bestRouteTransitControl, "65000 65001"); err != nil {
		return err
	}
	if path != "" {
		return requireBestRouteTransitBIRDRoute(reading.subject, bestRouteTransitSubject, path)
	}
	return nil
}

func requireBestRouteTransitBIRDRoute(output, prefix, path string) error {
	if err := requireBIRDASPath(output, prefix, path); err != nil {
		return err
	}
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[0] != prefix {
			continue
		}
		if fields[1] != "unicast" {
			return errors.New("BIRD transit route is not reachable unicast")
		}
		if !slices.Contains(fields, "*") {
			return errors.New("BIRD transit route is not selected")
		}
		return nil
	}
	return errors.New("BIRD transit answer has no exact native route line")
}
