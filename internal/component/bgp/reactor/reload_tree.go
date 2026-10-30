// Design: docs/architecture/config/transaction-protocol.md -- an outer rejection compensates its applied tree
// Related: reactor_api.go -- VerifyConfig, ApplyConfigDiff, loadPeersFullOrTree
// Related: reactor_dynamic.go -- dynamicGroupsLoad

package reactor

// TreeReloadFunc builds the peers, the global defaults and the dynamic groups
// of the whole configuration tree it is handed, in the ToPluginMap shape the
// reload coordinator passes to ApplyConfigDiff. It runs the
// pipeline ReloadFunc runs over the file (PruneInactive, ResolveBGPTree,
// CheckRequiredFields, YANG defaults, routes), so a peer it builds compares
// equal to the one the startup load built.
//
// It MUST apply nothing, for the reason ReloadFunc MUST NOT: it returns what
// the reconcile then applies under its journal.
type TreeReloadFunc func(tree map[string]any) ([]*PeerSettings, Globals, []*DynamicGroupConfig, error)

// SetTreeReloadFunc sets the loader ApplyConfigDiff runs over the tree it is
// handed. A file-configured daemon sets it beside
// SetReloadFunc (CreateReactor, ../config/loader.go).
func (r *Reactor) SetTreeReloadFunc(fn TreeReloadFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.treeReloadFunc = fn
}

// loadPeersFromConfig loads the peers of the whole configuration tree the
// reload coordinator hands ApplyConfigDiff.
//
// VerifyConfig keeps the file route (loadPeersFullOrTree): the coordinator
// verifies the candidate it read from that file, and the file-backed verifier
// is what keeps the optional schemas selected at boot
// (TestReloadVerifierRetainsSelectedPluginSchema, ../config/loader_test.go).
//
// The tree is the answer, never the file. Reading the file instead re-read
// the staged candidate (storage.ReadReloadConfig prefers it), so an outer
// compensation that handed ApplyConfigDiff the prior tree re-applied the very
// candidate it was undoing, before the hub cleared it. The loaded dynamic
// groups are the tree's own (dynamicGroupsReadAs), so the dynamic step
// reconciles the population against the tree being applied.
//
// A reactor with no TreeReloadFunc is one CreateReactor did not build (a
// test, a non-file daemon): it keeps loadPeersFullOrTree over the bgp block.
func (a *reactorAPIAdapter) loadPeersFromConfig(tree map[string]any) (loadedConfig, error) {
	a.r.mu.RLock()
	treeReloadFn := a.r.treeReloadFunc
	a.r.mu.RUnlock()

	if treeReloadFn != nil {
		peers, globals, groups, err := treeReloadFn(tree)
		if err != nil {
			return loadedConfig{}, err
		}
		return loadedConfig{peers: peers, globals: globals, groups: dynamicGroupsReadAs(groups)}, nil
	}

	bgpTree, err := configBGPBlock(tree)
	if err != nil {
		return loadedConfig{}, err
	}
	return a.loadPeersFullOrTree(bgpTree)
}
