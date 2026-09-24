package rr

import (
	"fmt"
	"net/netip"
	"os"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/ze"
)

// appendOriginatorIDJSON renders ORIGINATOR_ID as the address it is.
//
// Without a formatter the generic arm names an attribute `attr-<code>` and
// prints its hex (appendAttributeJSON, component/bgp/format/text_json.go), so
// RFC 4456's ORIGINATOR_ID reached every JSON reader as `"attr-9": "0a000001"`.
// That is a value nothing can act on without knowing the wire format, and it is
// this plugin's own attribute: RFC 4456 Section 8 defines it, and bgp-rr is the
// plugin that sets it.
//
// ExaBGP names it `originator-id` and prints the address, which is what the
// bridge translates ze's name to; that translation has nothing to work with
// while the value is hex.
func appendOriginatorIDJSON(buf []byte, attr attribute.Attribute) []byte {
	// The VALUE type, not a pointer: knownAttrParsers stores what
	// ParseOriginatorID returns (attribute/wire.go, simple.go), and that is an
	// OriginatorID. Asserting the pointer failed silently and the generic arm
	// printed attr-9 hex, which is the shape this formatter exists to replace.
	id, ok := attr.(attribute.OriginatorID)
	if !ok {
		return nil
	}
	addr := netip.Addr(id)
	if !addr.IsValid() {
		return nil
	}
	buf = append(buf, '"')
	buf = addr.AppendTo(buf)
	return append(buf, '"')
}

// appendClusterListJSON renders CLUSTER_LIST as the addresses it holds.
//
// The same gap ORIGINATOR_ID had, and the same plugin owns it: RFC 4456 Section
// 8 defines both, and without a formatter the generic arm printed
// `"attr-10": "03030303c0a8c901"`. ExaBGP writes the list of addresses, which
// is also the only form a reader can compare against a router id.
func appendClusterListJSON(buf []byte, attr attribute.Attribute) []byte {
	list, ok := attr.(attribute.ClusterList)
	if !ok {
		return nil
	}
	buf = append(buf, '[')
	for index, id := range list {
		if index > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, '"')
		buf = netip.AddrFrom4([4]byte{byte(id >> 24), byte(id >> 16), byte(id >> 8), byte(id)}).AppendTo(buf)
		buf = append(buf, '"')
	}
	return append(buf, ']')
}

func init() {
	attribute.RegisterJSONFormatter(attribute.AttrOriginatorID, "originator-id", appendOriginatorIDJSON)
	attribute.RegisterJSONFormatter(attribute.AttrClusterList, "cluster-list", appendClusterListJSON)

	reg := registry.Registration{
		Name:         "bgp-rr",
		Description:  "Route Reflector",
		RFCs:         []string{"4456"},
		Dependencies: []string{"bgp-adj-rib-in", "bgp-rib"},
		// The reflector owns peer-up replay for peers whose state it receives.
		// Share the RS claim so Adj-RIB-In stands down before the first peer
		// establishes; UnheldRoles restores self-replay for other peers.
		Claims: []string{"bgp-peer-up-replay"},
		// The peer-up replay reflects the stored adj-rib-in into the client that
		// establishes, which is that client's initial routing update, and
		// signalSessionReady reports when it is out, after this plugin's own
		// End-of-RIB (rr.go).
		SignalsSessionReady: true,
		RunEngine:           runRouteReflector,
		Commands:            commandDecls(),
		ConfigureEventBus: func(bus ze.EventBus) {
			validationBus.Store(&bus)
		},
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			setLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "rr: registration failed: %v\n", err)
		os.Exit(1)
	}
}
