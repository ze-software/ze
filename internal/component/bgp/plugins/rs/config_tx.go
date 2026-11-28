// Design: docs/architecture/core-design.md — route server plugin
// RFC: rfc/short/rfc7947.md — Section 2.2.2.1 prohibits route-server AS_PATH modification
// Related: server.go — RunRouteServer registers this transaction
// Related: worker.go — setChanSize, the pool setting this transaction drives

package rs

import (
	"encoding/json"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/configvalue"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// workerQueueSizeDefault is the per-source-peer worker channel capacity when
// neither the env var nor the YANG leaf names one. It matches the leaf's YANG
// default and the env entry's registered default.
const workerQueueSizeDefault = 4096

// Config keys read when enforcing RFC 7947 Section 2.2.2.1 on rs-client import chains.
const (
	rsClientLeaf       = "rs-client"
	modifyListKey      = "modify"
	modifyPluginPrefix = "bgp-filter-modify"
	asPathPrependLeaf  = "as-path-prepend"
)

// rsPrependConflict identifies one rs-client peer or dynamic group whose active
// import chain attaches a policy { modify } definition carrying as-path-prepend.
type rsPrependConflict struct {
	target  string
	modName string
}

// newQueueSizeConfigTx returns bgp-rs's side of the config transaction for
// bgp { route-server { worker-queue-size } } and the RFC 7947 Section 2.2.2.1
// import as-path-prepend guard, on the SDK's one transaction helper: Stage 2
// validates and installs the size at once, a reload's verify validates and
// holds it, apply installs it and keeps the replaced size, and rollback puts
// that back.
//
// The size reaches the pool through setChanSize, which governs workers created
// afterwards. A worker that already exists keeps its channel until it idles
// out (poolConfig.idleTimeout) and is recreated.
//
// The ze.bgp.route-server.worker-queue-size env var wins over the leaf at boot
// and on every reload: with envOverride set, no install changes the size.
func newQueueSizeConfigTx(workers *workerPool, envOverride bool) *sdk.ConfigTx[int] {
	workers.mu.Lock()
	boot := workers.cfg.chanSize
	workers.mu.Unlock()
	return sdk.NewConfigTx(boot, parseQueueSizeSections, func(size int) error {
		if envOverride {
			logger().Info("worker-queue-size from env, config value ignored", "config-size", size)
			return nil
		}
		workers.setChanSize(size)
		logger().Info("worker-queue-size applied", "size", size, "scope", "workers created from now on")
		return nil
	})
}

// verifyRSConfig is the plugin's side-effect-free in-process config verifier,
// run by `ze config validate` and commit without starting the plugin. It runs
// the same parseQueueSizeSections parser that Stage 2 configure and reload
// verify execute, so every config entry point enforces one verdict.
func verifyRSConfig(sections []sdk.ConfigSection) error {
	_, _, err := parseQueueSizeSections(sections)
	return err
}

// parseQueueSizeSections parses the bgp and policy sections of one delivery,
// refuses any rs-client peer or dynamic group attaching an active import
// as-path-prepend modifier (RFC 7947 Section 2.2.2.1), and returns the
// worker-queue-size named by the bgp section. A delivery without the bgp root
// is not present, so a commit of another root changes nothing; a bgp tree
// without the leaf names the default, so removing the leaf restores it.
// Malformed bgp or policy JSON fails closed.
func parseQueueSizeSections(sections []sdk.ConfigSection) (int, bool, error) {
	var bgpTree, policyTree map[string]any
	var bgpData string
	for _, section := range sections {
		switch section.Root {
		case configRootBGP:
			tree, err := parseConfigSectionObject(section.Data, configRootBGP)
			if err != nil {
				return 0, false, err
			}
			bgpTree = tree
			bgpData = section.Data
		case policy.RootName:
			tree, err := parseConfigSectionObject(section.Data, policy.RootName)
			if err != nil {
				return 0, false, err
			}
			policyTree = tree
		}
	}
	if bgpTree == nil {
		return 0, false, nil
	}
	if err := validateRSNoImportASPathPrepend(bgpTree, policyTree); err != nil {
		return 0, false, err
	}
	return queueSizeOrDefault(parseWorkerQueueSize(bgpData)), true, nil
}

// parseConfigSectionObject decodes one delivered config section and unwraps its
// root container when present. Invalid JSON, JSON null, and non-object root
// values fail closed rather than looking like an empty configuration.
func parseConfigSectionObject(data, root string) (map[string]any, error) {
	const validateFix = "ze config validate <file>"
	why := "bgp-rs cannot verify route-server settings or import as-path-prepend rules without a valid " + root + " object"
	var raw map[string]any
	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		return nil, opererr.Wrap(err, "the "+root+" config section is not valid JSON", why, validateFix)
	}
	if raw == nil {
		return nil, opererr.New("the "+root+" config section is null instead of a JSON object", why, validateFix)
	}
	if val, exists := raw[root]; exists {
		inner, ok := val.(map[string]any)
		if !ok {
			return nil, opererr.New("the "+root+" config section value is not a JSON object", why, validateFix)
		}
		return inner, nil
	}
	return raw, nil
}

// validateRSNoImportASPathPrepend refuses any peer or dynamic-group template
// whose effective session > rs-client is true when its active import filter
// chain (global, group, or peer level) references a policy { modify }
// definition carrying set { as-path-prepend }.
//
// RFC 7947 Section 2.2.2.1: "the route server SHOULD NOT prepend its own AS number to the AS_PATH segment nor modify the AS_PATH segment in any other way."
func validateRSNoImportASPathPrepend(bgpTree, policyTree map[string]any) error {
	prependDefs := collectPrependModifyDefs(policyTree)
	if len(prependDefs) == 0 {
		return nil
	}
	globalImport := importFilterRefs(bgpTree)

	var conflict *rsPrependConflict
	configjson.ForEachPeer(bgpTree, func(name string, peerMap, groupMap map[string]any, origin configjson.PeerOrigin) {
		if !effectiveRSClient(peerMap, groupMap) {
			return
		}
		modName, found := firstImportPrependMod(prependDefs, globalImport, importFilterRefs(groupMap), importFilterRefs(peerMap))
		if !found {
			return
		}
		target := peerTargetLabel(name, origin)
		if conflict == nil {
			conflict = &rsPrependConflict{target: target, modName: modName}
			return
		}
		if target < conflict.target {
			conflict = &rsPrependConflict{target: target, modName: modName}
		}
	})
	if conflict != nil {
		return rsImportPrependError(conflict.target, conflict.modName)
	}
	return nil
}

// collectPrependModifyDefs returns the set of policy > modify definition names
// that declare set > as-path-prepend.
func collectPrependModifyDefs(policyTree map[string]any) map[string]struct{} {
	var defs map[string]struct{}
	if modBlock, ok := policyTree[modifyListKey].(map[string]any); ok {
		for name, raw := range modBlock {
			defMap, isMap := raw.(map[string]any)
			if !isMap {
				continue
			}
			if hasASPathPrependSet(defMap) {
				if defs == nil {
					defs = make(map[string]struct{})
				}
				defs[name] = struct{}{}
			}
		}
	}
	return defs
}

// hasASPathPrependSet reports whether a modify definition map declares a
// non-empty set > as-path-prepend leaf.
func hasASPathPrependSet(defMap map[string]any) bool {
	if setBlock, ok := defMap["set"].(map[string]any); ok {
		if val, exists := setBlock[asPathPrependLeaf]; exists {
			if val == nil {
				return false
			}
			if text, isStr := val.(string); isStr {
				return text != ""
			}
			return true
		}
	}
	return false
}

// effectiveRSClient resolves session > rs-client for a peer or dynamic-group
// template: peer overrides group when explicitly set, default false.
func effectiveRSClient(peerMap, groupMap map[string]any) bool {
	if val, set := sessionRSClient(peerMap); set {
		return val
	}
	if val, set := sessionRSClient(groupMap); set {
		return val
	}
	return false
}

// sessionRSClient reads session > rs-client from one peer or group map,
// returning (value, true) when present and parseable as a boolean.
func sessionRSClient(m map[string]any) (bool, bool) {
	if m == nil {
		return false, false
	}
	session, ok := m["session"].(map[string]any)
	if !ok {
		return false, false
	}
	raw, exists := session[rsClientLeaf]
	if !exists {
		return false, false
	}
	return configvalue.Bool(raw)
}

// importFilterRefs returns the active import filter references from one config
// level (global bgp, group, or peer), or nil when absent.
func importFilterRefs(m map[string]any) []string {
	if filterBlock, ok := m["filter"].(map[string]any); ok {
		if imp, ok := filterBlock["import"].(map[string]any); ok {
			return configvalue.LeafList(imp["filters"])
		}
	}
	return nil
}

// firstImportPrependMod returns the first modify definition name in chains that
// appears in prependDefs, matching bare NAME, modify:NAME, and
// bgp-filter-modify:NAME while ignoring references qualified for other plugins.
func firstImportPrependMod(prependDefs map[string]struct{}, chains ...[]string) (string, bool) {
	for _, refs := range chains {
		for _, ref := range refs {
			modName := modifyRefName(ref)
			if modName == "" {
				continue
			}
			if _, hasPrepend := prependDefs[modName]; hasPrepend {
				return modName, true
			}
		}
	}
	return "", false
}

// modifyRefName extracts the modify instance name from a filter chain
// reference, or returns "" if the reference is qualified for another filter
// type or plugin.
func modifyRefName(ref string) string {
	prefix, name, qualified := strings.Cut(ref, ":")
	if !qualified {
		return ref
	}
	if prefix == modifyListKey {
		return name
	}
	if prefix == modifyPluginPrefix {
		return name
	}
	return ""
}

// peerTargetLabel formats a peer or dynamic-group template for operator errors.
func peerTargetLabel(name string, origin configjson.PeerOrigin) string {
	var tb textbuf.Buffer
	if origin.Template {
		return tb.Str("group ").Str(origin.Group).String()
	}
	return tb.Str("peer ").Str(name).String()
}

// rsImportPrependError builds the operator error refusing an import
// as-path-prepend modifier on a route-server client (RFC 7947 Section 2.2.2.1).
func rsImportPrependError(target, modName string) error {
	var problem, fix textbuf.Buffer
	problem.Str(target).Str(" sets session rs-client true and attaches import modify ").
		Byte('"').Str(modName).Byte('"').Str(" with ").Str(asPathPrependLeaf)
	why := "RFC 7947 Section 2.2.2.1 states that a route server should not modify AS_PATH on relayed client routes; " +
		"an import as-path-prepend rewrites the path before forwarding to other route-server clients"
	fix.Str("remove ").Byte('"').Str(modName).Byte('"').Str(" from the import filter chain of ").Str(target).
		Str(", remove set { ").Str(asPathPrependLeaf).Str(" } from policy modify ").
		Byte('"').Str(modName).Byte('"').Str(", or set session rs-client false")
	return opererr.New(problem.String(), why, fix.String())
}

// queueSizeOrDefault maps parseWorkerQueueSize's 0 to the default. The engine
// enforces the leaf's YANG range (1..1000000) before delivery, so 0 here means
// the leaf is absent from the tree.
func queueSizeOrDefault(size int) int {
	if size == 0 {
		return workerQueueSizeDefault
	}
	return size
}
