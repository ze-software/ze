// Design: docs/architecture/config/exabgp-syntax.md -- ExaBGP flow route scope
// Overview: migrate_routes.go -- flow route conversion to Ze update blocks

package migration

import (
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/exabgp/bridge"
)

// interfaceSetKeyword is the one keyword an ExaBGP flow route's scope block
// takes (src/exabgp/configuration/flow/scope.py, ParseFlowScope.schema).
const interfaceSetKeyword = "interface-set"

// flowScopeExtCommunities converts a flow route's scope block into the ze
// extended communities that carry the same interface sets.
//
// The scope block is the third section of an ExaBGP flow route, beside match
// and then, and it holds interface-set values alone. A keyword ExaBGP does not
// declare there is refused rather than dropped: the migrated config would
// otherwise announce a filter with a wider reach than the one it was written
// from.
func flowScopeExtCommunities(routeName string, scope *config.Tree) ([]string, error) {
	var extComms []string

	for _, key := range scope.Values() {
		val, _ := scope.Get(key)
		keyword, value := parseFlowMatchEntry(key, val)
		if keyword != interfaceSetKeyword {
			return nil, fmt.Errorf("flow route %s: scope keyword %q is not %s, the one keyword exabgp declares there",
				routeName, keyword, interfaceSetKeyword)
		}

		// Both `interface-set [ a b ];` and `interface-set a;` reach here, and
		// the freeform parser writes the first with its brackets kept.
		for item := range strings.FieldsSeq(strings.Trim(value, "[] ")) {
			extComm, err := interfaceSetExtCommunity(item)
			if err != nil {
				return nil, fmt.Errorf("flow route %s: %w", routeName, err)
			}
			extComms = append(extComms, extComm)
		}
	}

	return extComms, nil
}

// interfaceSetExtCommunity converts one ExaBGP interface-set value into the
// ze extended community that carries the same eight octets, written in the
// name ze renders it with (`interface-set:<transitivity>:<direction>:<asn>:<group-id>`).
//
// ExaBGP takes two forms (configuration/grammar/types/flow.py,
// _one_interface_set), the second transitive:
//
//	<transitive|non-transitive>:<input|output|input-output>:<asn>:<group-id>
//	<input|output|input-output>:<asn>:<group-id>
//
// The attribute package owns the encoding (attribute.ParseExtendedCommunityKeyword),
// and every ze extended community parser reads its name, so the value is parsed
// once here to refuse a malformed one at migration, and written back in the
// four-field form so the migrated config states its transitivity. ExaBGP's one
// stricter rule, no asdot AS number, is checked first (bridge.CheckInterfaceSetAS),
// because the attribute package takes every RFC 5396 spelling.
func interfaceSetExtCommunity(value string) (string, error) {
	// ExaBGP refuses an asdot AS number here, which Ze's own parser accepts.
	if err := bridge.CheckInterfaceSetAS(value); err != nil {
		return "", err
	}
	ec, matched, err := attribute.ParseExtendedCommunityKeyword(interfaceSetKeyword + ":" + value)
	if !matched {
		return "", fmt.Errorf("BUG: the attribute package no longer registers %s", interfaceSetKeyword)
	}
	if err != nil {
		return "", err
	}
	return string(ec.AppendDecoded(nil)), nil
}
