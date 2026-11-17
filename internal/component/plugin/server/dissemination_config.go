// Design: docs/architecture/api/process-protocol.md -- candidate Stage 1 grants.
// Related: startup_removal.go -- the exact Configure lifetime owns this graph.
package server

import (
	"fmt"
	"slices"
	"sync"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/opererr"
)

// DisseminationConfigResolver projects one namespace's resolved attachments
// from a whole candidate tree. It MUST apply nothing or retain the tree. The
// returned values are borrowed only until the caller builds its owned graph.
type DisseminationConfigResolver func(map[string]any) ([]DeliveryPeer, error)

var disseminationConfigResolvers struct {
	sync.RWMutex
	byNamespace map[string]DisseminationConfigResolver
}

// RegisterDisseminationConfigResolver installs the namespace owner's pure
// config projection. Call from init, before startup; safe for concurrent reads.
// Infrastructure never parses a protocol's config or imports its component.
func RegisterDisseminationConfigResolver(namespace string, resolve DisseminationConfigResolver) error {
	if namespace == "" {
		return fmt.Errorf("dissemination config resolver has no namespace")
	}
	if resolve == nil {
		return fmt.Errorf("dissemination config resolver %q is nil", namespace)
	}
	disseminationConfigResolvers.Lock()
	defer disseminationConfigResolvers.Unlock()
	if disseminationConfigResolvers.byNamespace[namespace] != nil {
		return fmt.Errorf("dissemination config resolver %q is already registered", namespace)
	}
	if disseminationConfigResolvers.byNamespace == nil {
		disseminationConfigResolvers.byNamespace = make(map[string]DisseminationConfigResolver)
	}
	disseminationConfigResolvers.byNamespace[namespace] = resolve
	return nil
}

// candidateDisseminationGraphs runs before removalMu is acquired. A Configure
// scope owns one immutable graph per namespace, shared by its Stage 1 readers.
func candidateDisseminationGraphs(tree map[string]any) ([]*DeliveryGraph, error) {
	disseminationConfigResolvers.RLock()
	resolvers := make(map[string]DisseminationConfigResolver, len(disseminationConfigResolvers.byNamespace))
	namespaces := make([]string, 0, len(disseminationConfigResolvers.byNamespace))
	for namespace, resolve := range disseminationConfigResolvers.byNamespace {
		resolvers[namespace] = resolve
		namespaces = append(namespaces, namespace)
	}
	disseminationConfigResolvers.RUnlock()
	slices.Sort(namespaces)
	graphs := make([]*DeliveryGraph, 0, len(namespaces))
	for _, namespace := range namespaces {
		peers, err := resolvers[namespace](tree)
		if err != nil {
			return nil, err
		}
		graphs = append(graphs, newDeliveryGraph(namespace, peers))
	}
	return graphs, nil
}

// validateDisseminationForProcess uses the map owned by this exact Configure
// attempt, never an active graph with different grants. A process outside a
// Configure attempt is judged against the active graph, as a runtime restart is.
func (s *Server) validateDisseminationForProcess(proc *process.Process) error {
	selected, scoped := s.pluginConfigureConfig(proc)
	if !scoped {
		return s.ValidateDisseminationBindings()
	}
	if selected.tree == nil {
		return s.ValidateDisseminationBindings()
	}
	processes := s.disseminationProcesses()
	for _, declaration := range processes {
		for _, role := range declaration.Declarations {
			found := false
			for _, graph := range selected.dissemination {
				if graph.namespace == role.Namespace {
					found = true
					break
				}
			}
			if !found {
				return opererr.New("Cannot validate peer grants for process "+declaration.Name+" in namespace "+role.Namespace+".",
					"The namespace has no configuration resolver, so Ze cannot safely start this peer-output role.", "ze show plugin declarations")
			}
		}
	}
	for _, graph := range selected.dissemination {
		if err := graph.ValidateDisseminationBindings(processes); err != nil {
			return err
		}
	}
	return nil
}
