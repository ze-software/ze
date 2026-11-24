// Design: docs/architecture/api/architecture.md -- exact participant source grants.
// Related: dissemination.go -- the one complete-grant evaluator.
package server

import (
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// CheckDisseminationSource judges one exact resolved participant, not an
// address-keyed daemon graph which may contain a sibling at another port. The
// runtime declaration is authoritative, including an empty declaration. The
// caller MUST authenticate and recheck the Process lifetime separately.
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
	graph, err := s.buildDeliveryGraphForPeers(namespace, []DeliveryPeer{peer}, plugin.BusConfig{})
	if err != nil {
		return err
	}
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
		var problem textbuf.Buffer
		problem.Str("Participant ").Quoted(peer.Name).Str(" does not grant endpoint ").
			Quoted(proc.Name()).Str(" its complete source receive set")
		return opererr.New(
			problem.String(),
			"A source receipt cannot widen a narrowed bus filter",
			disseminationBusFixLine(proc.Name(), namespace, declared.SourceReceive, nil))
	}
	return opererr.New("Process "+proc.Name()+" declares no source role "+role,
		"The live process cannot acquire source authority from an undeclared role", "ze request plugin restart "+proc.Name())
}
