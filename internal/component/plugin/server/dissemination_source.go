// Design: docs/architecture/api/architecture.md -- exact peer source grants.
// Related: dissemination.go -- the one complete-grant evaluator.
package server

import (
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/opererr"
)

// CheckDisseminationSource judges one exact resolved peer, not an address-keyed
// daemon graph which may contain a sibling at another port. The runtime
// declaration is authoritative, including an empty declaration. The caller
// MUST authenticate and recheck the Process lifetime separately. This cold
// source-lease admission reuses the same explicit complete-grant evaluator.
func (s *Server) CheckDisseminationSource(proc *process.Process, peer DeliveryPeer, namespace, role string) error {
	if proc == nil {
		return opererr.New("Source lease has no selecting process", "The request cannot be authenticated", "ze show plugin list")
	}
	declaration := proc.Registration()
	if declaration == nil {
		return opererr.New("Selecting process "+proc.Name()+" has no runtime declaration", "No source permission can be inferred from an implementation alias", "ze request plugin restart "+proc.Name())
	}
	if err := registry.CheckDisseminationDeclarations(proc.Name(), declaration.Dissemination); err != nil {
		return err
	}
	if err := registry.CheckDisseminationEvents(proc.Name(), declaration.Dissemination); err != nil {
		return err
	}
	graph := newDeliveryGraph(namespace, []DeliveryPeer{peer})
	for _, declared := range declaration.Dissemination {
		if declared.Namespace != namespace {
			continue
		}
		if declared.Role != role {
			continue
		}
		if graph.disseminationReceive(peer.Addr, proc.Name(), declared.SourceReceive) {
			return nil
		}
		return opererr.New(fmt.Sprintf("Peer %q does not grant process %q its complete source receive set", peer.Name, proc.Name()),
			"A source receipt cannot widen a partial, implicit or another process's attachment",
			"attach process "+proc.Name()+" { receive [ "+strings.Join(declared.SourceReceive, " ")+" ]; }")
	}
	return opererr.New("Process "+proc.Name()+" declares no source role "+role,
		"The live process cannot acquire source authority from an undeclared role", "ze request plugin restart "+proc.Name())
}
