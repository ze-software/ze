// Design: docs/architecture/config/apply-ordering.md -- the BGP root's operation labels

// Package configop defines the config operation labels of the BGP root.
//
// A label belongs to the component that emits and applies it, never to the
// shared plugin ABI: a list there is a central enumeration a new root would
// edit because it looks like the place labels belong (ai/rules/principles.md).
// The BGP root emits its operations from internal/component/bgp/plugin and
// applies them in internal/component/bgp/reactor, and the reactor cannot
// import the plugin without pulling a plugin's registration into the engine,
// so the two sides share this leaf instead of each spelling the labels again.
//
// The engine names none of these. It orders an operation by its verb and the
// kind of the resource it targets, and carries the label unchanged to the
// owner that dispatches on it.
package configop

import "github.com/ze-software/ze/pkg/plugin/rpc"

// Operation labels of the `bgp` config root.
const (
	AddPeer        rpc.ConfigOperationType = "add-peer"
	RemovePeer     rpc.ConfigOperationType = "remove-peer"
	ModifyPeer     rpc.ConfigOperationType = "modify-peer"
	AddListener    rpc.ConfigOperationType = "add-listener"
	RemoveListener rpc.ConfigOperationType = "remove-listener"
)
