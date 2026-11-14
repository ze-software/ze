// Design: docs/architecture/testing/interop.md -- the Extended Prefix/Link scenario.
// Related: register_ospf_ext_prefix_link.go -- its registration.
// RFC: rfc/short/rfc7684.md Sections 2 and 3.
// FRR: https://github.com/FRRouting/frr/blob/frr-10.7.1/ospfd/ospf_opaque.c
// ospf_opaque_lsa_originate_schedule decides when FRR's Extended LSAs exist.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	ospfExtPrefixLinkScenario = "ospf-ext-prefix-link-frr"
	ospfExtPrefixLinkWait     = 60 * time.Second
)

// ospfExtAdvertiser is the part of one decoded Extended Prefix or Extended Link
// Opaque LSA in Ze's `show ospf database opaque-area` the checker judges: the
// router that originated it.
type ospfExtAdvertiser struct {
	Router netip.Addr `json:"advertising-router"`
}

// checkOSPFExtPrefixLink proves both directions of the RFC 7684 exchange with FRR.
// Assertion 1 is the adjacency. Assertion 2 is the receive side: Ze's LSDB holds an
// Extended Prefix LSA and an Extended Link LSA that FRR advertised, decoded, beside
// Ze's own. Assertion 3 is the send side: FRR's opaque-area database names Ze.
// Assertion 4 is that the adjacency survived the exchange.
func checkOSPFExtPrefixLink(ctx context.Context, check *interoplab.CheckContext) error {
	if !check.Network.IPv4.IsValid() {
		return errors.New("assertion 1: the Extended Prefix/Link lab has no selected IPv4 network")
	}
	neighbor := []string{cmdVtysh, "-c", frrShowOSPFNeighbor}
	if err := waitContains(ctx, check.Lab, peerFRR, neighbor, 90*time.Second, ospfStateFull); err != nil {
		return fmt.Errorf("assertion 1: FRR never reached Full with Ze: %w", err)
	}
	zeRouter, err := netip.ParseAddr(networkHostAddress(check.Network, zeHostNumber))
	if err != nil {
		return fmt.Errorf("assertion 2: Ze lab address: %w", err)
	}
	frrRouter, err := netip.ParseAddr(networkHostAddress(check.Network, frrHostNumber))
	if err != nil {
		return fmt.Errorf("assertion 2: FRR lab address: %w", err)
	}
	if err := waitOSPFExtFromFRR(ctx, check.Lab, frrRouter, zeRouter); err != nil {
		return fmt.Errorf("assertion 2: %w", err)
	}
	opaque := []string{cmdVtysh, "-c", frrShowOSPFDatabaseOpaqueArea}
	if err := waitContains(ctx, check.Lab, peerFRR, opaque, ospfExtPrefixLinkWait, zeRouter.String()); err != nil {
		return fmt.Errorf("assertion 3: FRR's opaque-area database never named Ze %s: %w", zeRouter, err)
	}
	if err := delayRequireContains(ctx, check.Lab, peerFRR, neighbor, 5*time.Second, ospfStateFull); err != nil {
		return fmt.Errorf("assertion 4: the adjacency did not stay Full: %w", err)
	}
	return nil
}

// waitOSPFExtFromFRR waits rather than reading once at Full: FRR schedules its own
// opaque LSAs only once an opaque-capable neighbor appears, each type
// `min_ls_interval` after the previous (`ospf_opaque.c`,
// `ospf_opaque_lsa_originate_schedule`), so at the instant FRR shows Full its
// Extended Prefix and Extended Link LSAs may not exist yet.
func waitOSPFExtFromFRR(ctx context.Context, lab interoplab.CheckerLab, frrRouter, zeRouter netip.Addr) error {
	command := zeCommand("show ospf database opaque-area")
	var last string
	var verdict error
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: ospfExtPrefixLinkWait, Interval: 2 * time.Second, Description: "FRR's Extended Prefix and Extended Link LSAs in Ze's LSDB",
	}, func(ctx context.Context) (string, error) {
		output, err := lab.Query(ctx, "ze", command, queryEnvironment("ze", command))
		last = output
		return output, err
	}, func(output string) bool {
		verdict = ospfExtPrefixLinkVerdict(output, frrRouter, zeRouter)
		return verdict == nil
	})
	if err != nil {
		return fmt.Errorf("the Ze LSDB never held both Extended LSA types from FRR %s: %w (last verdict: %s); last reply: %s", frrRouter, err, ospfSRAnycastVerdictText(verdict), last)
	}
	return nil
}

// ospfExtPrefixLinkVerdict judges Ze's `show ospf database opaque-area` answer. Each
// decoded row carries the router that advertised it, so the LSA type is bound to its
// originator: Ze's own Extended LSAs cannot stand for FRR's, and an FRR opaque LSA of
// another type, listed in the generic LSDB rows, cannot stand for either Extended
// type. Ze's own rows are required too, so Ze still originates both types.
func ospfExtPrefixLinkVerdict(output string, frrRouter, zeRouter netip.Addr) error {
	var views []struct {
		ExtendedPrefix []ospfExtAdvertiser `json:"extended-prefix"`
		ExtendedLink   []ospfExtAdvertiser `json:"extended-link"`
	}
	if err := json.Unmarshal([]byte(output), &views); err != nil {
		return fmt.Errorf("decode Ze's opaque-area database: %w", err)
	}
	var prefixFrom, linkFrom []netip.Addr
	for _, view := range views {
		for _, lsa := range view.ExtendedPrefix {
			prefixFrom = append(prefixFrom, lsa.Router)
		}
		for _, lsa := range view.ExtendedLink {
			linkFrom = append(linkFrom, lsa.Router)
		}
	}
	for _, want := range []struct {
		kind   string
		router netip.Addr
		from   []netip.Addr
	}{
		{"Extended Prefix", frrRouter, prefixFrom},
		{"Extended Link", frrRouter, linkFrom},
		{"Extended Prefix", zeRouter, prefixFrom},
		{"Extended Link", zeRouter, linkFrom},
	} {
		if !slices.Contains(want.from, want.router) {
			return fmt.Errorf("no decoded %s LSA advertised by %s; Ze holds that type from %v", want.kind, want.router, want.from)
		}
	}
	return nil
}
