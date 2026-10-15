// Design: docs/architecture/static-routes.md -- a route's table resolves against the commit's own routing-table section
//
// The goal is that one commit can add a routing table and a static route in
// it. Every participant verifies before any applies, so the routing-table
// plugin has not installed the new table when static verifies the route; the
// route must resolve against the candidate routing-table section. The method
// runs the static verifier over the sections a commit delivers while the
// installed registry holds no named table.
package static

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/routingtable"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

const routeInTableLNS = `{"static":{"table":{"lns":{"route":{"0.0.0.0/0":{"next":{"hop":{"10.0.0.1":{}}}}}}}}}` //nolint:lll // JSON test fixture

// TestStaticVerifyResolvesCandidateTable proves R-9: a route in a table the
// same commit defines is accepted and carries that table's ID, while the
// installed registry, which knows no table, is ignored.
// VALIDATES: spec-plugin-reload-applies-or-refuses R-9.
// PREVENTS: a commit adding a table and a route in it being refused because
// static read the registry the routing-table plugin installs only at apply.
// MUTATION: resolve against routingtable.GetRegistry() in
// verifyCandidateStatic and the route is refused as "not found".
// Not parallel: the registry is package state.
func TestStaticVerifyResolvesCandidateTable(t *testing.T) {
	routingtable.SetRegistry(nil)
	t.Cleanup(func() { routingtable.SetRegistry(nil) })
	sections := []sdk.ConfigSection{
		{Root: pluginName, Data: routeInTableLNS},
		{Root: routingtable.ConfigRoot, Data: `{"routing-table":{"table":{"lns":{"id":"100"}}}}`},
	}
	routes, found, err := verifyCandidateStatic(sections)
	if err != nil {
		t.Fatalf("verify refused a route in a table the commit defines: %v", err)
	}
	if !found {
		t.Fatal("verify reported no static section")
	}
	if len(routes) != 1 || routes[0].Table != 100 {
		t.Fatalf("routes = %+v, want one route in table 100", routes)
	}
}

// TestStaticVerifyRefusesTableTheCandidateLacks proves the other half: a
// table the installed registry knows but the candidate no longer defines is
// refused, so a commit that deletes a table still in use cannot pass.
// VALIDATES: spec-plugin-reload-applies-or-refuses R-9.
// PREVENTS: a route resolving against a table the commit removes.
func TestStaticVerifyRefusesTableTheCandidateLacks(t *testing.T) {
	routingtable.SetRegistry(routingtable.New(map[string]uint32{"lns": 100}))
	t.Cleanup(func() { routingtable.SetRegistry(nil) })
	sections := []sdk.ConfigSection{{Root: pluginName, Data: routeInTableLNS}}
	_, _, err := verifyCandidateStatic(sections)
	if err == nil || !strings.Contains(err.Error(), `routing-table "lns": not found`) {
		t.Fatalf("verify error = %v, want the lns table refused", err)
	}
	// The refusal names the route and both fixes (review N-1).
	for _, want := range []string{"static route ", `Fix: remove the route from static table "lns", or keep routing-table "lns"`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("verify error %q does not carry %q", err, want)
		}
	}
}
