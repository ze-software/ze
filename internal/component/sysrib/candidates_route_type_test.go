// Design: docs/guide/route-policy.md -- show rib candidates, the hidden paths and the chain that hid each
// Related: candidates.go -- candidatePathOf.
// Related: candidates_test.go -- candidatesThroughAuthority, candidateAnswer.

package sysrib

import (
	"context"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// Goal: `show rib candidates` names an OSPF path's type with the word the
// `route-type` match condition takes, so an operator can write the condition
// from what the command shows, and a path that states no type carries no
// route-type field. Method: an OSPF external type 2 path and a static path on
// two prefixes, read through routeViewClient.Candidates and the daemon's
// readCandidates.
func TestShowRIBCandidatesNamesOSPFRouteType(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	ospf := redistevents.RegisterProtocol("ospf")
	static := redistevents.RegisterProtocol("static")
	external := netip.MustParsePrefix("10.80.0.0/24")
	plain := netip.MustParsePrefix("10.81.0.0/24")
	loc := locrib.NewRIB()
	loc.Insert(family.IPv4Unicast, external, locrib.Path{Source: ospf, NextHop: netip.MustParseAddr("192.0.2.80"), OSPFRouteType: ribpolicy.OSPFExternal2})
	loc.Insert(family.IPv4Unicast, plain, locrib.Path{Source: static, NextHop: netip.MustParseAddr("192.0.2.81")})

	client, _ := candidatesThroughAuthority(t, ctx, loc)
	defer closeRuntimeFeedClient(t, ctx, client)

	data, err := client.Candidates(ctx, nil)
	if err != nil {
		t.Fatalf("show rib candidates: %v", err)
	}
	entries, raw := candidateAnswer(t, data)
	if len(entries) != 2 {
		t.Fatalf("answer lists %d prefixes, want 2: %+v", len(entries), entries)
	}
	if entries[0].Prefix != external || len(entries[0].Paths) != 1 {
		t.Fatalf("first entry %+v, want %s with one path", entries[0], external)
	}
	if got := entries[0].Paths[0].RouteType; got != "external-2" {
		t.Fatalf("OSPF path route-type = %q, want external-2", got)
	}
	if entries[1].Prefix != plain || len(entries[1].Paths) != 1 {
		t.Fatalf("second entry %+v, want %s with one path", entries[1], plain)
	}
	if got := entries[1].Paths[0].RouteType; got != "" {
		t.Fatalf("static path route-type = %q, want none", got)
	}
	// One route-type key in the whole answer: the OSPF row carries it under its
	// kebab-case name, and the static row carries none.
	if n := strings.Count(string(raw), `"route-type":`); n != 1 {
		t.Fatalf("JSON %s carries %d route-type fields, want 1", raw, n)
	}
	if !strings.Contains(string(raw), `"route-type":"external-2"`) {
		t.Fatalf("JSON %s lacks the kebab-case route-type field", raw)
	}
}
