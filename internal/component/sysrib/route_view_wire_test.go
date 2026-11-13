// Design: docs/architecture/rib/unified-locrib.md -- routing-view input guards.
// Related: register_route_view.go -- the actual owner-registered raw handlers.

package sysrib

import (
	"context"
	"encoding/json"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Missing or null read frontiers cannot acquire the meaning of explicit zero.
// Each invalid raw call precedes a valid zero-valued call to the real handler;
// the admitted prefix proves that refusal did not consume authoritative state.
func TestRouteViewRawReadRequiresPresence(t *testing.T) {
	for _, test := range []struct{ name, field, form string }{
		{"missing-cursor", "cursor", "missing"},
		{"null-cursor", "cursor", "null"},
		{"missing-wait", "wait-ms", "missing"},
		{"null-wait", "wait-ms", "null"},
		{"unknown-identity", "claimed-owner", "unknown"},
	} {
		t.Run(test.name, func(t *testing.T) {
			rib := locrib.NewRIB()
			prefix := netip.MustParsePrefix("192.0.2.0/24")
			rib.Insert(family.IPv4Unicast, prefix, locrib.Path{Source: redistevents.RegisterProtocol("static")})
			owner := routeViewOwner()
			opened, err := routeViews.open(context.Background(), owner, rib, rpc.RouteViewOpenInput{})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := routeViews.cleanup(owner); err != nil {
					t.Error(err)
				}
			})
			control, err := json.Marshal(rpc.RouteViewReadInput{Session: opened.Session, Cursor: 0, Limit: 1, WaitMillis: 0})
			if err != nil {
				t.Fatal(err)
			}
			input := strings.Replace(string(control), `,"`+test.field+`":0`, "", 1)
			if test.form == "null" {
				input = strings.Replace(string(control), `"`+test.field+`":0`, `"`+test.field+`":null`, 1)
			}
			if test.form == "unknown" {
				input = strings.TrimSuffix(string(control), "}") + `,"claimed-owner":"rib"}`
			}
			if _, err := routeViewReadHandler(nil, context.Background(), owner, json.RawMessage(input)); err == nil {
				t.Errorf("raw handler accepted %s", test.name)
			}
			answer, err := routeViewReadHandler(nil, context.Background(), owner, control)
			if err != nil {
				t.Fatalf("invalid input consumed healthy zero-cursor control: %v", err)
			}
			page, ok := answer.(rpc.RouteViewReadOutput)
			if !ok {
				t.Fatalf("raw read returned %T", answer)
			}
			if !page.Complete || len(page.Entries) != 1 || page.Entries[0].Prefix != prefix {
				t.Fatalf("raw read lost admitted control: %+v", page)
			}
		})
	}
}
