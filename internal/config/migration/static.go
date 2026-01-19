package migration

import (
	"codeberg.org/thomas-mangin/zebgp/internal/config"
)

// ExtractStaticRoutes transforms static blocks to announce blocks.
//
// For each neighbor/peer/template.group/template.match block with a static child:
//   - Routes are moved to announce.<afi>.<safi> based on prefix and attributes
//   - The static block is removed
//   - Existing announce blocks are preserved and merged
//
// Note: peer+static and template.match+static are defensive; these don't exist in practice.
//
// Returns a new tree; original is not modified.
// Returns ErrNilTree for nil input.
func ExtractStaticRoutes(tree *config.Tree) (*config.Tree, error) {
	if tree == nil {
		return nil, ErrNilTree
	}

	result := tree.Clone()

	// Process neighbor blocks
	for _, entry := range result.GetListOrdered("neighbor") {
		extractStaticFromPeer(entry.Value)
	}

	// Process peer blocks (defensive)
	for _, entry := range result.GetListOrdered("peer") {
		extractStaticFromPeer(entry.Value)
	}

	// Process template.group and template.match blocks
	if tmpl := result.GetContainer("template"); tmpl != nil {
		for _, entry := range tmpl.GetListOrdered("group") {
			extractStaticFromPeer(entry.Value)
		}
		// template.match+static is defensive
		for _, entry := range tmpl.GetListOrdered("match") {
			extractStaticFromPeer(entry.Value)
		}
	}

	return result, nil
}

// extractStaticFromPeer extracts routes from static block into announce block.
func extractStaticFromPeer(peer *config.Tree) {
	if peer == nil {
		return
	}

	static := peer.GetContainer("static")
	if static == nil {
		return
	}

	// Process routes from static block
	// Routes are stored as a list keyed by prefix
	routes := static.GetListOrdered("route")

	// Only create announce if there are routes to migrate
	if len(routes) > 0 {
		announce := peer.GetOrCreateContainer("announce")

		for _, route := range routes {
			prefix := route.Key
			attrs := route.Value

			// Determine AFI
			afi := "ipv4"
			if isIPv6Prefix(prefix) {
				afi = "ipv6"
			}

			// Determine SAFI
			_, hasRD := attrs.Get("rd")
			_, hasLabel := attrs.Get("label")
			safi := detectSAFI(prefix, hasRD, hasLabel)

			// Get or create the AFI/SAFI container path
			afiContainer := announce.GetOrCreateContainer(afi)
			afiContainer.AddListEntry(safi, prefix, attrs)
		}
	}

	// Remove static block
	peer.RemoveContainer("static")
}
