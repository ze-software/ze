// Design: docs/architecture/api/process-protocol.md — init()-based RPC registration

package server

import (
	"strings"

	"github.com/ze-software/ze/internal/component/command"
)

// registeredRPCs holds RPCs added via RegisterRPCs from init() in register_*.go files.
var registeredRPCs []RPCRegistration

// RegisterRPCs adds RPCs to the package-level registry.
// Called from init() in register.go files.
func RegisterRPCs(rpcs ...RPCRegistration) {
	registeredRPCs = append(registeredRPCs, rpcs...)
}

// ProcessCleanupFunc is called when a plugin process exits.
// Receives the process name for scoped cleanup.
type ProcessCleanupFunc func(processName string)

var processCleanupHooks []ProcessCleanupFunc

// RegisterProcessCleanup registers a callback invoked during cleanupProcess.
// Called from init() to avoid import cycles between server and command packages.
func RegisterProcessCleanup(fn ProcessCleanupFunc) {
	processCleanupHooks = append(processCleanupHooks, fn)
}

// runProcessCleanupHooks calls all registered cleanup hooks for a process.
func runProcessCleanupHooks(processName string) {
	for _, fn := range processCleanupHooks {
		fn(processName)
	}
}

// peerNodeName is the container keyword every peer command hangs off.
const peerNodeName = "peer"

// bgpPathKeyword is the `bgp` an operator TYPES, which is a separate fact from
// bgpParticipantName, the plugin whose apply runs last.
const bgpPathKeyword = "bgp"

// PeerKeywords is what the BGP `peer` containers declare, and which of those
// words an operator can type IMMEDIATELY after `peer`. The two are separate
// questions and a caller asks one of them: a word can be a peer keyword and
// still be safe as a peer name.
type PeerKeywords struct {
	// Declared is every keyword any BGP `peer` container declares, whatever the
	// operator must type before it. A name absent from this set names no peer
	// command at all.
	Declared map[string]bool
	// Colliding is the subset an operator types straight after `peer`, with no
	// value in between. A peer NAME from this set stands in the same slot as
	// the word, so config validation refuses it.
	Colliding map[string]bool
}

// PeerSubcommandKeywords reads the merged command tree and answers both halves
// of PeerKeywords for every BGP `peer` container in it.
//
// A verb under a `peer` container does NOT collide with a peer name when the
// model puts a mandatory value between the two. `show bgp peer <selector>
// detail` declares the selector on the `peer` container, so a peer named
// `detail` is read in the selector slot with the verb still to come, and the
// grammar cannot produce the ambiguity. `show bgp peer list` is the other case:
// it declares ze:inherit none, takes no selector, and `list` therefore follows
// `peer` directly.
//
// The MERGED tree is the input because the children of one `peer` container
// come from several modules and only the merge holds them together: the
// top-level container declares its selector in ze-cli-announce-cmd.yang while
// ze-raw-cmd.yang and ze-update-cmd.yang contribute children to it
// (BuildCommandTree, internal/component/config/yang/command.go).
//
// The derivation this replaced read adjacency in the space-joined PATH STRING.
// A path holds node names, a selector is a leaf rather than a node, so every
// verb looked adjacent to `peer` and the answer named collisions no operator
// can type.
func PeerSubcommandKeywords(root *command.Node) PeerKeywords {
	keywords := PeerKeywords{
		Declared:  make(map[string]bool),
		Colliding: make(map[string]bool),
	}
	collectPeerSubcommandKeywords(root, "", "", keywords)
	return keywords
}

// collectPeerSubcommandKeywords walks the tree and reads the keywords of every
// BGP `peer` container it meets into the two maps keywords carries. parent and
// grandparent are the two names above node, which is all isBGPPeerNode reads,
// so no path is built.
//
// The recursion is over a tree this process built from its own embedded
// modules. No peer chooses its depth (docs/contributing/ze-go-style.md).
func collectPeerSubcommandKeywords(node *command.Node, parent, grandparent string, keywords PeerKeywords) {
	if node == nil {
		return
	}
	for name, child := range node.Children {
		lower := strings.ToLower(name)
		if lower == peerNodeName && isBGPPeerNode(parent, grandparent) {
			readPeerContainer(child, keywords)
		}
		collectPeerSubcommandKeywords(child, lower, parent, keywords)
	}
}

// readPeerContainer records one `peer` container's children, and marks the ones
// the operator reaches with no selector in front.
func readPeerContainer(peer *command.Node, keywords PeerKeywords) {
	for name, child := range peer.Children {
		lower := strings.ToLower(name)
		keywords.Declared[lower] = true
		if !reachedWithoutSelector(child) {
			continue
		}
		keywords.Colliding[lower] = true
	}
}

// reachedWithoutSelector reports whether any command at or under node is typed
// with no value between `peer` and node's own keyword.
//
// The answer is the merged model's own. inheritArgDefs
// (internal/component/config/yang/command.go) carries a leaf that a container
// declares down to every command below it and ANCHORS it to that container's
// keyword, so a command taking the peer selector before its verb holds a
// mandatory ArgDef anchored to `peer`. A command holding none is one the
// operator types straight after the keyword: it declares ze:inherit none
// (`show bgp peer list`), or it declares a selector of its own, which the model
// then places after the verb (`peer raw <selector>`).
//
// A node with no command under it contributes nothing, because a keyword that
// dispatches nothing takes no peer name.
func reachedWithoutSelector(node *command.Node) bool {
	if node == nil {
		return false
	}
	if node.WireMethod != "" && !takesValueAfterPeer(node.ArgDefs) {
		return true
	}
	for _, child := range node.Children {
		if reachedWithoutSelector(child) {
			return true
		}
	}
	return false
}

// takesValueAfterPeer reports whether defs holds a mandatory value the operator
// types straight after the `peer` keyword. An optional one is not it: the
// operator can leave it out and put the verb there instead.
func takesValueAfterPeer(defs []command.ArgDef) bool {
	for i := range defs {
		if defs[i].Mandatory && strings.EqualFold(defs[i].Anchor, peerNodeName) {
			return true
		}
	}
	return false
}

// isBGPPeerNode reports whether a `peer` container under these two ancestors is
// a BGP one. `show vpn ipsec peer` and `show policy chain peer` are the peer
// containers of other subsystems, and no BGP peer name stands in their slots.
//
// An empty parent is the tree root, where `peer` is the top-level verb root.
func isBGPPeerNode(parent, grandparent string) bool {
	if parent == "" {
		return true
	}
	if parent == "request" {
		return true
	}
	if parent != bgpPathKeyword {
		return false
	}
	switch grandparent {
	case "show", "set", "delete", "update":
		return true
	}
	return false
}
