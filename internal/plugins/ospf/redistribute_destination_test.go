// Design: docs/guide/redistribution.md -- sources and destinations
//
// VALIDATES: the OSPF plugin declares the "ospf" redistribution destination
// and the ospf root it needs from init(), before any engine starts.
package ospf

import (
	"testing"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	ospfredistribute "github.com/ze-software/ze/internal/plugins/ospf/redistribute"
)

// TestOSPFDeclaresItsRedistributeDestination proves the declaration the config
// doors read exists once the package is linked, with no engine running.
//
// VALIDATES: LookupDestination("ospf") answers the ospf config root, so
// config.RefuseRedistributeRules refuses `destination ospf` with no ospf block.
// PREVENTS: the destination being judged from the consumer, which registers
// only in OnStarted and is absent at load, validate and commit.
func TestOSPFDeclaresItsRedistributeDestination(t *testing.T) {
	dest, ok := configredist.LookupDestination(ospfredistribute.ConsumerName)
	if !ok {
		t.Fatalf("destination %q is not declared at init", ospfredistribute.ConsumerName)
	}
	if dest.ConfigRoot != Namespace {
		t.Fatalf("destination %q declares root %q, want %q", dest.Name, dest.ConfigRoot, Namespace)
	}
}
