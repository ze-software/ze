// Design: docs/architecture/testing/ci-format.md -- compiled Go fixtures
// Related: routing_fixture.go -- registers this scenario and holds the row helpers

package fixture

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// isisRedistPrefixTLV is the TLV 135 entry body for 10.78.2.0/24 with no sub-TLV
// and the up/down bit clear (RFC 5305 Section 4): the control octet 0x18 carries
// prefix length 24, and the three significant octets follow.
const isisRedistPrefixTLV = "180a4e02"

// isisRedistNoBGPScenario has the synthetic fakeredist producer announce
// 10.78.2.0/24, then waits until this node's own LSP advertises it, in a config
// whose `redistribute { destination isis { import fakeredist } }` stands with no
// `bgp` block. It reads the state the redistribution produces, the LSDB, rather
// than a log line. It polls because the producer, the orchestrator and the IS-IS
// consumer start in an order nothing fixes: the prefix arrives through the first
// batch or the late-consumer replay, at a time no observer can predict.
func isisRedistNoBGPScenario(ctx context.Context, plugin *sdk.Plugin) error {
	if _, err := routingValue(ctx, plugin, "request fakeredist emit add ipv4/unicast 10.78.2.0/24"); err != nil {
		return err
	}
	var rows []map[string]any
	var dispatchErr error
	if Poll(ctx, 40, 250*time.Millisecond, func() bool {
		rows, dispatchErr = routingRows(ctx, plugin, "show isis database detail", false)
		return dispatchErr == nil && isisOwnLSPCarries(rows, 135, isisRedistPrefixTLV)
	}) {
		fmt.Fprintln(os.Stderr, "OK: own LSP advertises the redistributed 10.78.2.0/24 in TLV 135 with no bgp block")
		return nil
	}
	if dispatchErr != nil {
		return dispatchErr
	}
	return fmt.Errorf("show isis database detail: no own LSP carries 10.78.2.0/24 in TLV 135 (want %q in its value): %#v",
		isisRedistPrefixTLV, rows)
}

// isisOwnLSPCarries reports whether an own LSP row holds a TLV of tlvType whose
// hex value contains body.
func isisOwnLSPCarries(rows []map[string]any, tlvType float64, body string) bool {
	for _, row := range rows {
		if !routingBool(row, "own") {
			continue
		}
		tlvs, _ := row["tlvs"].([]any)
		for _, raw := range tlvs {
			tlv, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if routingNumber(tlv, "type") != tlvType {
				continue
			}
			if strings.Contains(routingString(tlv, "value"), body) {
				return true
			}
		}
	}
	return false
}
