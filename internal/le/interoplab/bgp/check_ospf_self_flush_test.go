package bgp

import (
	"slices"
	"strings"
	"testing"
)

// TestOSPFSelfFlushRouteAbsenceVerdict drives the self-flush scenario's route-table
// absence through absenceVerdict, with the needle and the proof token read from the
// scenario's own operation, so a drift of either constant, or of the operation, breaks
// it. FRR's `show ip route ospf json` answers "{}" for an empty OSPF table, which is the
// absence the scenario waits for; an answer keyed by the default prefix is the route
// still present; an empty answer is a query that measured nothing.
func TestOSPFSelfFlushRouteAbsenceVerdict(t *testing.T) {
	index := slices.IndexFunc(ospfSelfLSAFlushOperations, func(op operation) bool {
		return op.kind == opWaitAbsent && slices.Equal(op.command, ospfSelfFlushFRRRoutes)
	})
	if index < 0 {
		t.Fatal("the self-flush scenario has no route-table absence wait")
	}
	op := ospfSelfLSAFlushOperations[index]
	if !slices.Equal(op.absent, []string{ospfSelfFlushDefaultPrefix}) {
		t.Fatalf("the route-table absence looks for %q, want the default prefix key %q", op.absent, ospfSelfFlushDefaultPrefix)
	}
	if !slices.Equal(op.proof, []string{ospfSelfFlushFRRRouteJSONClose}) {
		t.Fatalf("the route-table absence proves its run with %q, want %q", op.proof, ospfSelfFlushFRRRouteJSONClose)
	}

	if err := absenceVerdict("{}\n", op.absent, op.proof); err != nil {
		t.Fatalf("FRR's empty OSPF route table was not read as the default's absence: %v", err)
	}

	const withDefault = `{"0.0.0.0/0":[{"prefix":"0.0.0.0/0","prefixLen":0,"protocol":"ospf","selected":true,"installed":true,` +
		`"nexthops":[{"ip":"172.30.0.2","afi":"ipv4","interfaceName":"eth0","active":true}]}]}` + "\n"
	err := absenceVerdict(withDefault, op.absent, op.proof)
	if err == nil {
		t.Fatal("an OSPF route table holding Ze's default was read as its absence")
	}
	if !strings.Contains(err.Error(), "unexpectedly contains") {
		t.Fatalf("the refusal does not name the default still present: %v", err)
	}

	err = absenceVerdict("", op.absent, op.proof)
	if err == nil {
		t.Fatal("an empty answer was read as the default's absence")
	}
	if !strings.Contains(err.Error(), "was not exercised") {
		t.Fatalf("the refusal of an empty answer does not say the absence was not measured: %v", err)
	}
}
