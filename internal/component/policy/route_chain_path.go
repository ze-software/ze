// Design: docs/guide/route-policy.md -- completing and checking filter names in a chain
// Related: reference_source.go -- chainLeafAt and filterReference, which read these paths
// Related: rib_compile.go -- checkRouteChain, the check a member passes here

package policy

import (
	"slices"
	"strings"
	"sync"
)

// routeChainWildcard in a registered path matches any one token: a list key.
const routeChainWildcard = "*"

// routeChainPath is a chain leaf whose owner compiles it with
// CompileRouteChain at attachment, configured at a path that is not
// `<root> filter <direction> filters`.
type routeChainPath struct {
	pattern    []string
	attachment string
}

// routeChainPaths holds the paths MustRegisterRouteChainPath declared. Written
// from init(), read by completion and commit. Safe for concurrent use.
var routeChainPaths = struct {
	mu    sync.RWMutex
	paths []routeChainPath
}{}

// MustRegisterRouteChainPath declares that the filters leaf-list at pattern, a
// data path whose "*" tokens stand for list keys and whose last token is
// "filters", is a chain the owner compiles with CompileRouteChain at
// attachment, so the policy-filter reference source offers and checks its
// members the way that compile does. Call it from the owner's init(), after
// MustRegisterAttachment. A malformed or duplicate pattern is a Ze defect and
// panics.
func MustRegisterRouteChainPath(pattern []string, attachment string) {
	if len(pattern) < 2 {
		panic("BUG: policy: route chain path for " + attachment + " is too short")
	}
	if pattern[len(pattern)-1] != "filters" {
		panic("BUG: policy: route chain path " + strings.Join(pattern, " ") + " does not end in filters")
	}
	vocab.mu.RLock()
	_, known := vocab.attachments[attachment]
	vocab.mu.RUnlock()
	if !known {
		panic("BUG: policy: route chain path " + strings.Join(pattern, " ") + " registered before its attachment " + attachment)
	}
	routeChainPaths.mu.Lock()
	defer routeChainPaths.mu.Unlock()
	for _, p := range routeChainPaths.paths {
		if slices.Equal(p.pattern, pattern) {
			panic("BUG: policy: route chain path " + strings.Join(pattern, " ") + " registered twice")
		}
	}
	routeChainPaths.paths = append(routeChainPaths.paths, routeChainPath{pattern: slices.Clone(pattern), attachment: attachment})
}

// routeChainLeafAt answers the chain leaf of the registered path at matches.
func routeChainLeafAt(at []string) (chainLeaf, bool) {
	routeChainPaths.mu.RLock()
	defer routeChainPaths.mu.RUnlock()
	for _, p := range routeChainPaths.paths {
		if routeChainPathMatches(p.pattern, at) {
			return chainLeaf{attachment: p.attachment, routeChain: true}, true
		}
	}
	return chainLeaf{}, false
}

// routeChainPathMatches answers whether at is pattern with each wildcard
// filled by one token.
func routeChainPathMatches(pattern, at []string) bool {
	if len(pattern) != len(at) {
		return false
	}
	for i, token := range pattern {
		if token == routeChainWildcard {
			continue
		}
		if token != at[i] {
			return false
		}
	}
	return true
}

// routeChainMemberCheck is the check CompileRouteChain runs over policyCfg,
// asked of a chain holding member alone.
func routeChainMemberCheck(attachment string, policyCfg map[string]any) ChainMemberCheck {
	return func(reg *Registry, member, _ string) error {
		_, err := checkRouteChain(attachment, []string{member}, reg, policyCfg)
		return err
	}
}
