// Design: docs/guide/redistribution.md -- sources and destinations
//
// VALIDATES: the IS-IS plugin declares the "isis" redistribution destination
// and the isis root it needs from init(), before any engine starts.
package isis

import (
	"testing"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	isisredistribute "github.com/ze-software/ze/internal/plugins/isis/redistribute"
)

// TestISISDeclaresItsRedistributeDestination proves the declaration the config
// doors read exists once the package is linked, with no engine running.
//
// VALIDATES: LookupDestination("isis") answers the isis config root, so
// config.RefuseRedistributeRules refuses `destination isis` with no isis block.
// PREVENTS: the destination being judged from the consumer, which registers
// only in OnStarted and is absent at load, validate and commit.
func TestISISDeclaresItsRedistributeDestination(t *testing.T) {
	dest, ok := configredist.LookupDestination(isisredistribute.ConsumerName)
	if !ok {
		t.Fatalf("destination %q is not declared at init", isisredistribute.ConsumerName)
	}
	if dest.ConfigRoot != configRoot {
		t.Fatalf("destination %q declares root %q, want %q", dest.Name, dest.ConfigRoot, configRoot)
	}
}
