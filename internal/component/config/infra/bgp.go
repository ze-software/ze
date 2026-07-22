// Design: ai/rules/feature-gate-registration.md -- inversion-of-control seams for the gated BGP engine
// Related: hook.go -- the daemon-startup hook the same engine calls back through

package infra

import (
	"errors"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// The seams below invert three edges that used to run always-on -> BGP engine.
// Always-on code (`ze config dump|diff|validate`, the daemon's reboot path)
// needs work only the BGP engine can do; rather than importing
// internal/component/bgp -- which would pin it into every binary and defeat
// //go:build ze_bgp -- the engine self-registers here from its own init().
// With the engine compiled out every seam stays nil and the caller takes the
// no-BGP path, which is correct: a bgp{} block cannot exist in that build
// (its YANG schema is gated out too, so the parser rejects it as unknown).

// BGPTreeResolver resolves the bgp{} section of a parsed config tree into the
// flattened engine-facing peer map (group/peer template inheritance applied).
type BGPTreeResolver func(tree *config.Tree) (map[string]any, error)

// BGPPeerValidator reports whether every peer in the tree resolves to complete,
// consistent settings. It returns only the error because callers use it purely
// as a validation gate; the built peer objects are BGP types the always-on side
// must not name.
type BGPPeerValidator func(tree *config.Tree) error

// GRMarkerWriter persists the RFC 4724 graceful-restart marker so the engine
// sets the Restarting bit on its next OPEN. Called on operator-initiated
// restart/reboot, with the capabilities every loaded plugin injected.
type GRMarkerWriter func(caps []plugin.InjectedCapability, store storage.Storage)

var (
	bgpTreeResolver  BGPTreeResolver
	bgpPeerValidator BGPPeerValidator
	grMarkerWriter   GRMarkerWriter
)

// SetBGPTreeResolver registers the engine's bgp{} resolver. Called from the
// gated BGP config package's init().
func SetBGPTreeResolver(fn BGPTreeResolver) { bgpTreeResolver = fn }

// SetBGPPeerValidator registers the engine's peer validator. Called from the
// gated BGP config package's init().
func SetBGPPeerValidator(fn BGPPeerValidator) { bgpPeerValidator = fn }

// SetGRMarkerWriter registers the engine's graceful-restart marker writer.
// Called from the gated BGP config package's init().
func SetGRMarkerWriter(fn GRMarkerWriter) { grMarkerWriter = fn }

// errNoBGPEngine reports a tree that carries BGP configuration on a binary with
// no BGP engine to resolve it.
var errNoBGPEngine = errors.New(
	"config contains a bgp block but this binary was built without the BGP engine")

// ResolveBGPTree resolves the bgp{} section through the registered engine.
//
// With no engine registered it resolves to an EMPTY tree: a BGP-less build has
// no BGP configuration, so there is nothing to resolve and every caller carries
// on with the rest of the config. An empty map rather than nil so callers read
// and range it exactly as they would a real result.
//
// It FAILS CLOSED on the one combination that should be impossible: a tree that
// does carry a bgp{} block while no resolver is registered. The bgp{} YANG
// schema is gated by the same tag as the engine, so a parser that accepted the
// block means schema and seam have drifted apart -- and silently returning an
// empty tree there would let `ze config dump` print a config with its whole BGP
// section missing, or `ze config validate` call it valid without having checked
// any of it. The guard lives here, not in each caller, so every consumer of the
// seam inherits it (ai/rules/fail-closed-guards.md).
func ResolveBGPTree(tree *config.Tree) (map[string]any, error) {
	if bgpTreeResolver == nil {
		if tree != nil && tree.GetContainer("bgp") != nil {
			return nil, errNoBGPEngine
		}
		return map[string]any{}, nil
	}
	return bgpTreeResolver(tree)
}

// ValidateBGPPeers runs the engine's peer validation. It returns nil when no
// engine is compiled in (nothing to validate).
func ValidateBGPPeers(tree *config.Tree) error {
	if bgpPeerValidator == nil {
		return nil
	}
	return bgpPeerValidator(tree)
}

// WriteGRMarker persists the graceful-restart marker through the registered
// engine. A no-op when no engine is compiled in: without a BGP speaker there is
// no session for a peer to treat as restarting.
func WriteGRMarker(caps []plugin.InjectedCapability, store storage.Storage) {
	if grMarkerWriter == nil {
		return
	}
	grMarkerWriter(caps, store)
}
