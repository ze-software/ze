// Design: docs/architecture/api/architecture.md -- complete explicit transit grants.
// Related: dissemination.go -- the same immutable graph resolves effective owners.
package server

import (
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// ValidateDisseminationBindings refuses partial explicit transit requests.
// Source and destination grants MUST belong to one configured process. Derived
// bindings are deliberately ignored: they request no dissemination role.
// The graph and declarations MUST remain immutable during this call.
func (g *DeliveryGraph) ValidateDisseminationBindings(processes []DisseminationProcess) error {
	for _, process := range processes {
		if err := registry.CheckDisseminationDeclarations(process.Name, process.Declarations); err != nil {
			return err
		}
		if err := registry.CheckDisseminationEvents(process.Name, process.Declarations); err != nil {
			return err
		}
		for _, declaration := range process.Declarations {
			if declaration.Namespace != g.namespace {
				continue
			}
			if err := g.validateDisseminationProcess(process.Name, &declaration); err != nil {
				return err
			}
			for _, peer := range g.order {
				if _, err := g.ResolveDisseminationOwner(peer, declaration.Role, processes); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (g *DeliveryGraph) validateDisseminationProcess(process string, declaration *rpc.DisseminationDecl) error {
	for _, peer := range g.order {
		attachment := g.disseminationAttachment(peer, process)
		if attachment == nil {
			continue
		}
		if !attachment.explicit {
			continue
		}
		name := peer
		if g.peers[peer].name != "" {
			name = g.peers[peer].name
		}
		destination := false
		for _, required := range declaration.DestinationSend {
			destination = destination || g.MaySend(peer, process, required)
		}
		if destination {
			line := disseminationAttachLine(process, declaration.DestinationReceive, declaration.DestinationSend)
			if !g.disseminationDestination(peer, process, declaration) {
				return fmt.Errorf("peer %q process %q has incomplete destination grants; complete with `%s`", name, process, line)
			}
			if !g.disseminationSource(process, declaration) {
				source := disseminationAttachLine(process, declaration.SourceReceive, nil)
				return fmt.Errorf("peer %q destination process %q has no complete source on this same instance; attach a source peer with `%s`", name, process, source)
			}
			continue
		}
		if !g.disseminationReceive(peer, process, declaration.SourceReceive) {
			line := disseminationAttachLine(process, declaration.SourceReceive, nil)
			return fmt.Errorf("peer %q process %q has incomplete source grants; complete with `%s`", name, process, line)
		}
	}
	return nil
}

func disseminationAttachLine(process string, receive, send []string) string {
	line := "attach process " + process + " { receive [ " + strings.Join(receive, " ") + " ];"
	if len(send) > 0 {
		line += " send [ " + strings.Join(send, " ") + " ];"
	}
	return line + " }"
}

// NewDisseminationGraph builds the immutable graph used by offline config
// validation as well as runtime delivery. It grants nothing on its own.
func NewDisseminationGraph(namespace string, peers []DeliveryPeer) *DeliveryGraph {
	return newDeliveryGraph(namespace, peers)
}
