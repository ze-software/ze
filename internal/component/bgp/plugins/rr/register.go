package rr

import (
	"fmt"
	"net/netip"
	"os"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/slogutil"
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

func init() {
	attribute.RegisterJSONFormatter(attribute.AttrOriginatorID, "originator-id", appendOriginatorIDJSON)

	reg := registry.Registration{
		Name:         "bgp-rr",
		Description:  "Route Reflector",
		RFCs:         []string{"4456"},
		Dependencies: []string{"bgp-adj-rib-in"},
		// The peer-up replay reflects the stored adj-rib-in into the client that
		// establishes, which is that client's initial routing update, and
		// signalSessionReady reports when it is out, after this plugin's own
		// End-of-RIB (rr.go).
		SignalsSessionReady: true,
		RunEngine:           runRouteReflector,
		Commands:            commandDecls(),
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
