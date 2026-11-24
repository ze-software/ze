// Design: docs/architecture/api/process-protocol.md -- candidate Stage 1 grants.
// Related: startup_removal.go -- the exact Configure lifetime owns this candidate scope.
package server

import (
	"fmt"
	"slices"
	"sync"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/opererr"
)

// DisseminationConfigResolver projects one namespace's resolved participants
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

// candidateDisseminationScope holds cloned candidate DeliveryPeers per
// namespace and the candidate central BusConfig for one Configure attempt.
type candidateDisseminationScope struct {
	peersByNS map[string][]DeliveryPeer
	busCfg    plugin.BusConfig
}

// captureCandidateDissemination runs before removalMu is acquired. A Configure
// scope owns its candidate DeliveryPeers and BusConfig snapshot, compiled on
// demand once Stage 1 runtime declarations arrive.
func captureCandidateDissemination(tree map[string]any) (*candidateDisseminationScope, error) {
	disseminationConfigResolvers.RLock()
	resolvers := make(map[string]DisseminationConfigResolver, len(disseminationConfigResolvers.byNamespace))
	namespaces := make([]string, 0, len(disseminationConfigResolvers.byNamespace))
	for namespace, resolve := range disseminationConfigResolvers.byNamespace {
		resolvers[namespace] = resolve
		namespaces = append(namespaces, namespace)
	}
	disseminationConfigResolvers.RUnlock()
	slices.Sort(namespaces)

	busCfg, err := plugin.ExtractBusConfigFromMap(tree)
	if err != nil {
		return nil, err
	}
	peersByNS := make(map[string][]DeliveryPeer, len(namespaces))
	for _, namespace := range namespaces {
		peers, err := resolvers[namespace](tree)
		if err != nil {
			return nil, err
		}
		peersByNS[namespace] = cloneDeliveryPeers(peers)
	}
	return &candidateDisseminationScope{
		peersByNS: peersByNS,
		busCfg:    busCfg,
	}, nil
}

// validateDisseminationForProcess uses the candidate owned by this exact
// Configure attempt, never an active graph with different filters. A process
// outside a Configure attempt is judged against the active graph, as a runtime
// restart is.
func (s *Server) validateDisseminationForProcess(proc *process.Process) error {
	selected, scoped := s.pluginConfigureConfig(proc)
	if !scoped {
		return s.ValidateDisseminationBindings()
	}
	if selected.tree == nil {
		return s.ValidateDisseminationBindings()
	}
	if selected.dissemination == nil {
		return s.ValidateDisseminationBindings()
	}
	processes := s.disseminationProcesses()
	for _, declaration := range processes {
		for _, role := range declaration.Declarations {
			if _, found := selected.dissemination.peersByNS[role.Namespace]; !found {
				return opererr.New("Cannot validate grants for process "+declaration.Name+" in namespace "+role.Namespace+".",
					"The namespace has no configuration resolver, so Ze cannot safely start this dissemination role.", "ze show plugin declarations")
			}
		}
	}
	namespaces := make([]string, 0, len(selected.dissemination.peersByNS))
	for ns := range selected.dissemination.peersByNS {
		namespaces = append(namespaces, ns)
	}
	slices.Sort(namespaces)
	for _, ns := range namespaces {
		graph, err := s.buildDeliveryGraphForPeers(ns, selected.dissemination.peersByNS[ns], selected.dissemination.busCfg)
		if err != nil {
			return err
		}
		if err := graph.ValidateDisseminationBindings(processes); err != nil {
			return err
		}
	}
	return nil
}
