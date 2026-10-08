// Design: docs/architecture/wire/nlri.md -- native inventory and command framing.

package rs

import (
	"bytes"
	"encoding/hex"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/plugins/cmd/update"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/mup" // Register native command family names.
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
)

// TestMUPInventoryCommandRoundTrip feeds actual MP_REACH/MP_UNREACH sections
// through inventory extraction, removes one route, and parses the peer-down
// command. Native identities, including ADD-PATH zero, must survive all three.
// draft-ietf-bess-mup-safi Sections 3.1.1 and 3.1.4 define the ISD/T2ST bodies.
func TestMUPInventoryCommandRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		afi    family.AFI
		first  string
		second string
	}{
		{family.AFIIPv4, "0100010c0000fde90000000118c00002", "010004110000fde90000000140c633640101020304"},
		{family.AFIIPv6, "0100010d0000fde9000000012020010db8", "0100041d0000fde900000001a020010db800000000000000000000000101020304"},
	} {
		fam := family.Family{AFI: tc.afi, SAFI: family.SAFIMUP}
		for _, addPath := range []bool{false, true} {
			t.Run(fam.String()+"/addpath="+strconv.FormatBool(addPath), func(t *testing.T) {
				routes := []string{tc.first, tc.second}
				if addPath {
					routes = []string{"00000000" + tc.first, "00000011" + tc.second, "00000011" + tc.first}
				}
				var section []byte
				for _, route := range routes {
					raw, err := hex.DecodeString(route)
					if err != nil {
						t.Fatal(err)
					}
					section = append(section, raw...)
				}
				// Parse one concatenated command token, not one convenient token per route.
				args := []string{"nlri", fam.String()}
				if addPath {
					args = append(args, "addpath")
				}
				args = append(args, "del", hex.EncodeToString(section))
				parsed, err := update.ParseUpdateWire(args, plugin.WireEncodingHex)
				if err != nil {
					t.Fatal(err)
				}
				if len(parsed.Groups) != 1 {
					t.Fatalf("concatenated command groups = %d", len(parsed.Groups))
				}
				mupAssertRoutes(t, parsed.Groups[0].Withdraw, routes, addPath)

				rs := newTestRouteServer(t)
				records := extractWireNLRIRecords(mupInventoryMessage(t, fam, addPath, false, section))
				if records == nil {
					t.Fatal("native inventory extraction returned nil")
				}
				if len(records.records) != len(routes) {
					t.Fatalf("inventory split into %d records, want %d", len(records.records), len(routes))
				}
				for i, record := range records.records {
					if record.fam != fam || record.nlriStr != routes[i] || !record.wireForm || record.addPath != addPath {
						t.Fatalf("inventory record %d = %+v, want native %s", i, record, routes[i])
					}
				}
				rs.applyNLRIRecords("10.0.0.1", records.records)
				returnNLRIRecords(records)
				first, err := hex.DecodeString(routes[0])
				if err != nil {
					t.Fatal(err)
				}
				records = extractWireNLRIRecords(mupInventoryMessage(t, fam, addPath, true, first))
				if records == nil {
					t.Fatal("native withdrawal extraction returned nil")
				}
				rs.applyNLRIRecords("10.0.0.1", records.records)
				returnNLRIRecords(records)
				entries := rs.withdrawals["10.0.0.1"]
				if len(entries) != len(routes)-1 {
					t.Fatalf("withdrawal left %d entries, want %d", len(entries), len(routes)-1)
				}
				var commands []string
				rs.updateRouteHook = func(_, command string) { commands = append(commands, command) }
				rs.sendBatchedWithdrawals("10.0.0.1", entries, 0)
				wantRoutes := slices.Clone(routes[1:])
				slices.Sort(wantRoutes)
				want := "update hex nlri " + fam.String()
				if addPath {
					want += " addpath"
				}
				want += " del " + strings.Join(wantRoutes, " del ")
				if len(commands) != 1 || commands[0] != want {
					t.Fatalf("peer-down command = %q, want %q", commands, want)
				}
				parsed, err = update.ParseUpdateWire(strings.Fields(commands[0])[2:], plugin.WireEncodingHex)
				if err != nil {
					t.Fatal(err)
				}
				if len(parsed.Groups) != 1 || parsed.Groups[0].Family != fam || len(parsed.Groups[0].Announce) != 0 {
					t.Fatalf("peer-down parsed groups = %+v", parsed.Groups)
				}
				mupAssertRoutes(t, parsed.Groups[0].Withdraw, wantRoutes, addPath)
			})
		}
	}
}

// mupAssertRoutes checks exact native bytes and the retained ADD-PATH metadata.
func mupAssertRoutes(t *testing.T, got []nlri.NLRI, want []string, addPath bool) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("parsed routes = %d, want %d", len(got), len(want))
	}
	for i, route := range got {
		wire, ok := route.(*nlri.WireNLRI)
		if !ok {
			t.Fatalf("route %d is %T, want native WireNLRI", i, route)
		}
		if hex.EncodeToString(wire.Bytes()) != want[i] || wire.HasAddPath() != addPath {
			t.Fatalf("route %d = %x addpath=%t, want %s addpath=%t", i, wire.Bytes(), wire.HasAddPath(), want[i], addPath)
		}
	}
}

// mupInventoryMessage supplies native MP attributes and negotiated framing to
// the real wire parser. The MP next-hop is IPv6 for both MUP AFIs.
func mupInventoryMessage(t *testing.T, fam family.Family, addPath, withdraw bool, routes []byte) *bgptypes.RawMessage {
	t.Helper()
	value := []byte{0, byte(fam.AFI), byte(fam.SAFI)}
	code := byte(15)
	if !withdraw {
		code = 14
		value = append(value, 16, 0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0)
	}
	value = append(value, routes...)
	attrs := append([]byte{0x80, code, byte(len(value))}, value...)
	body := append([]byte{0, 0, 0, byte(len(attrs))}, attrs...)
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{fam: addPath}))
	if err != nil {
		t.Fatal(err)
	}
	wu := wireu.NewWireUpdate(body, ctxID)
	attrsWire, err := wu.Attrs()
	if err != nil {
		t.Fatal(err)
	}
	return &bgptypes.RawMessage{RawBytes: body, AttrsWire: attrsWire, WireUpdate: wu}
}

// TestMUPMalformedNativeConsumersRefuse prevents a truncated route from being
// fragmented into CIDR-looking inventory entries or accepted by a wire command.
func TestMUPMalformedNativeConsumersRefuse(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMUP}
	valid, err := hex.DecodeString("0100010c0000fde90000000118c00002")
	if err != nil {
		t.Fatal(err)
	}
	for _, addPath := range []bool{false, true} {
		for _, malformed := range [][]byte{{1, 0, 1}, valid[:len(valid)-1]} {
			data := slices.Clone(malformed)
			args := []string{"nlri", fam.String()}
			if addPath {
				data = append([]byte{0, 0, 0, 0}, data...)
				args = append(args, "addpath")
			}
			args = append(args, "del", hex.EncodeToString(data))
			if result, err := update.ParseUpdateWire(args, plugin.WireEncodingHex); err == nil || result != nil {
				t.Fatalf("malformed command accepted: %x, result %v error %v", data, result, err)
			}
			carrier, err := nlri.NewWireNLRI(fam, data, addPath)
			if err != nil {
				t.Fatal(err)
			}
			var scratch [nlrisplit.PrefixKeyScratchSize]byte
			if records := appendParsedRecords(nil, fam, []nlri.NLRI{carrier}, actionAdd, scratch[:]); len(records) != 0 {
				t.Fatalf("malformed carrier became inventory: %+v", records)
			}
		}
	}
	// NewWireNLRI's caller contract is one route. A double carrier must not
	// become one identity even though its two envelopes are separately valid.
	carrier, err := nlri.NewWireNLRI(fam, bytes.Repeat(valid, 2), false)
	if err != nil {
		t.Fatal(err)
	}
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	if records := appendParsedRecords(nil, fam, []nlri.NLRI{carrier}, actionAdd, scratch[:]); len(records) != 0 {
		t.Fatalf("multi-route carrier became inventory: %+v", records)
	}
}
