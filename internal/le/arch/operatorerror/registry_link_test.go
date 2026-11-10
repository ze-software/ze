// VALIDATES: spec assumption A-2, that the operator boundaries found from the
// registration sites by syntax cover what the runtime registries hold: every
// builtin RPC the linked product registers is a registration the scan read,
// and the scan read at least as many config verifiers as the registry holds.
// PREVENTS: a registration shape the scan does not match, which would leave a
// whole class of handlers outside the gate with every check green.
//
// The comparison runs one way. The test links internal/component/plugin/all
// under whatever feature tags the run sets, so a gated plugin the run leaves
// out is in the scan and not in the registry; the runtime set is a subset of
// the syntax set under every tag set, never equal to it.
package archoperatorerror

import (
	pathpkg "path"
	"strings"
	"testing"

	_ "github.com/ze-software/ze/internal/component/plugin/all"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestASTBoundarySetCoversRuntimeRegistry is A-2's validation.
func TestASTBoundarySetCoversRuntimeRegistry(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("checkout root: %v", err)
	}
	inventory, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if err := checkFloor(inventory, registrationFloor); err != nil {
		t.Fatal(err)
	}

	verifiers := 0
	for _, registration := range inventory.Registrations {
		if registration.Kind == KindConfigVerifier {
			verifiers++
		}
	}

	runtimeRPCs := 0
	for _, rpc := range pluginserver.AllBuiltinRPCs() {
		if rpc.Handler == nil {
			continue
		}
		if outsideProduct(rpc.Registrar()) {
			continue
		}
		runtimeRPCs++
		if !readByScan(inventory, rpc.WireMethod, rpc.Registrar()) {
			t.Errorf("builtin RPC %s, registered by %s, is no registration the syntax scan read", rpc.WireMethod, rpc.Registrar())
		}
	}
	if runtimeRPCs == 0 {
		t.Fatal("the linked product registered no builtin RPC, so the comparison judged nothing")
	}

	runtimeVerifiers := 0
	for _, registration := range registry.All() {
		if registration.InProcessConfigVerifier != nil {
			runtimeVerifiers++
		}
	}
	if runtimeVerifiers > verifiers {
		t.Errorf("the registry holds %d in-process config verifiers, the syntax scan read %d", runtimeVerifiers, verifiers)
	}
	t.Logf("A-2: %d runtime builtin RPCs all read by the scan; %d runtime config verifiers, %d read; %d registrations, %d sites, %d unresolved",
		runtimeRPCs, runtimeVerifiers, verifiers, len(inventory.Registrations), len(inventory.Sites), len(inventory.Unresolved))
}

// readByScan reports whether a runtime builtin RPC is a registration the scan
// read: one that spells its wire method whole, or one in the registering
// package whose computed wire method starts with the literal head the scan
// read (a registration built in a loop over section names). The package is
// part of the identity, so a prefix another package registers never covers a
// registration the scan missed.
func readByScan(inventory Inventory, wireMethod, registrar string) bool {
	for _, registration := range inventory.Registrations {
		if registration.Kind != KindRPC {
			continue
		}
		if registration.WireMethod == wireMethod {
			return true
		}
		if registration.WirePrefix == "" {
			continue
		}
		if modulePath+"/"+pathpkg.Dir(registration.File) != registrar {
			continue
		}
		if strings.HasPrefix(wireMethod, registration.WirePrefix) {
			return true
		}
	}
	return false
}

// outsideProduct reports whether a registering package is tooling or a test
// fake, which the scan skips on purpose (excludedDirs).
func outsideProduct(registrar string) bool {
	for _, excluded := range excludedDirs {
		if strings.HasPrefix(registrar, modulePath+"/"+excluded+"/") {
			return true
		}
		if registrar == modulePath+"/"+excluded {
			return true
		}
	}
	return false
}
