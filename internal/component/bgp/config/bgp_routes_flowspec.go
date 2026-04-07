// Design: docs/architecture/config/syntax.md — FlowSpec route extraction from config tree
// Overview: bgp_routes.go — route extraction orchestrator

package bgpconfig

import (
	"fmt"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// parseFlowSpecNLRILine parses a FlowSpec NLRI line like:
// "ipv4/flow source-ipv4 10.0.0.1/32 destination-port =80 protocol =tcp".
// RFC 8955 Section 4 defines the FlowSpec NLRI format.
func parseFlowSpecNLRILine(line string, attr *config.Tree) (FlowSpecRouteConfig, error) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return FlowSpecRouteConfig{}, fmt.Errorf("flowspec nlri requires match criteria")
	}

	fam := parts[0]
	fr := FlowSpecRouteConfig{
		IsIPv6: strings.HasPrefix(fam, "ipv6/"),
		NLRI:   make(map[string][]string),
	}

	// Parse inline rd for VPN variant: ipv4/flow-vpn rd 65000:100 add destination ...
	// RD is part of NLRI (RFC 8955), not a path attribute
	criteria := parts[1:]
	if strings.HasSuffix(fam, "-vpn") {
		if len(criteria) >= 2 && criteria[0] == "rd" {
			fr.RD = criteria[1]
			criteria = criteria[2:] // consume rd <value>
		}
	}

	// Operation keyword (add/del/eor) is mandatory
	if len(criteria) == 0 {
		return FlowSpecRouteConfig{}, fmt.Errorf("missing operation keyword (add/del/eor) for family %s", fam)
	}
	op := criteria[0]
	if op != opAdd && op != opDel && op != opEor {
		return FlowSpecRouteConfig{}, fmt.Errorf("missing operation keyword (add/del/eor) for family %s, got %q", fam, op)
	}
	criteria = criteria[1:]

	if op == opEor {
		return fr, nil
	}

	// Get next-hop from attributes
	if v, ok := attr.Get("next-hop"); ok {
		fr.NextHop = v
	}

	// Get community from attributes
	if items := attr.GetSlice("community"); len(items) > 0 {
		fr.Community = strings.Join(items, " ")
	}

	// Get extended-community from attributes (actions per RFC 8955 Section 7)
	if items := attr.GetSlice("extended-community"); len(items) > 0 {
		fr.ExtendedCommunity = strings.Join(items, " ")
	}

	// Get raw attribute (e.g., for IPv6 Extended Community attr 25)
	if items := attr.GetSlice("attribute"); len(items) > 0 {
		fr.Attribute = strings.Join(items, " ")
	}

	// Parse NLRI match criteria from remaining parts
	// Format: <criterion> <value> [<criterion> <value>]...
	// Values are stored as slices to support multi-value criteria like "protocol [ =tcp =udp ]"
	for i := 0; i < len(criteria); i++ {
		criterion := normalizeFlowSpecCriterion(criteria[i])
		// Handle bracketed lists like [ >200&<300 >400&<500 ]
		if i+1 < len(criteria) && criteria[i+1] == "[" {
			// Find closing bracket and collect all values
			j := i + 2
			for ; j < len(criteria) && criteria[j] != "]"; j++ {
				fr.NLRI[criterion] = append(fr.NLRI[criterion], criteria[j])
			}
			i = j
			continue
		}
		// Regular key-value pair (single value)
		if i+1 < len(criteria) {
			fr.NLRI[criterion] = append(fr.NLRI[criterion], criteria[i+1])
			i++
		}
	}

	return fr, nil
}

// normalizeFlowSpecCriterion normalizes FlowSpec criterion names to canonical form.
// Maps "source-ipv4", "source-ipv6" -> "source"; "destination-ipv4", "destination-ipv6" -> "destination".
// This ensures the NLRI map uses keys that buildFlowSpecNLRI expects.
func normalizeFlowSpecCriterion(criterion string) string {
	// Normalize IPv4/IPv6 source/destination variants to family-agnostic names
	switch criterion {
	case "source-ipv4", "source-ipv6":
		return "source"
	case "destination-ipv4", "destination-ipv6":
		return "destination"
	}
	return criterion
}

// extractFlowSpecRoutes extracts FlowSpec routes from flow { route ... }.
func extractFlowSpecRoutes(tree *config.Tree) []FlowSpecRouteConfig {
	flow := tree.GetContainer("flow")
	if flow == nil {
		return nil
	}

	// Use ordered iteration to preserve config order.
	entries := flow.GetListOrdered("route")
	routes := make([]FlowSpecRouteConfig, 0, len(entries))
	for _, entry := range entries {
		r := parseFlowSpecRoute(entry.Key, entry.Value)
		routes = append(routes, r)
	}

	return routes
}

func parseFlowSpecRoute(name string, route *config.Tree) FlowSpecRouteConfig {
	r := FlowSpecRouteConfig{
		Name: name,
		NLRI: make(map[string][]string),
	}

	if v, ok := route.Get("rd"); ok {
		r.RD = v
	}
	if v, ok := route.Get("next-hop"); ok {
		r.NextHop = v
	}

	// Parse match block into NLRI criteria (RFC 8955 Section 4)
	// Freeform stores:
	// - "keyword value" -> "true" for simple values like "source 10.0.0.1/32"
	// - "keyword" -> "value" for arrays like "fragment [ last-fragment ]"
	if match := route.GetContainer("match"); match != nil {
		for _, key := range match.Values() {
			val, _ := match.Get(key)
			if val == configTrue || val == "" {
				// Legacy format: key might be "keyword value"
				parts := strings.SplitN(key, " ", 2)
				if len(parts) == 2 {
					r.NLRI[parts[0]] = []string{parts[1]}
				}
				// Skip empty keys
			} else {
				// Array format: key is keyword, val has the values
				r.NLRI[key] = strings.Fields(strings.Trim(val, "[]"))
			}
		}
	}

	// Parse then block into ExtendedCommunity (RFC 8955 Section 7)
	// Actions are encoded as Traffic Filtering Action Extended Communities
	var extComms []string
	if then := route.GetContainer("then"); then != nil {
		for _, key := range then.Values() {
			val, _ := then.Get(key)
			action, value := key, val

			// Handle legacy "keyword value" format stored as key
			if val == configTrue || val == "" {
				parts := strings.SplitN(key, " ", 2)
				if len(parts) == 2 {
					action, value = parts[0], parts[1]
				} else {
					action, value = key, ""
				}
			}

			// Convert actions to extended community format
			switch action {
			case "discard":
				extComms = append(extComms, "discard")
			case "rate-limit":
				extComms = append(extComms, "rate-limit:"+value)
			case "redirect":
				extComms = append(extComms, "redirect:"+value)
			case "redirect-to-nexthop-draft":
				extComms = append(extComms, "redirect-to-nexthop-draft")
			case "copy-to-nexthop":
				extComms = append(extComms, "copy-to-nexthop")
			case "mark":
				extComms = append(extComms, "mark "+value)
			case "action":
				extComms = append(extComms, "action "+value)
			case "community":
				r.Community = strings.Trim(value, "[]")
			case "extended-community":
				extComms = append(extComms, strings.Trim(value, "[]"))
			}
		}
	}

	// Combine explicit extended-community with action-based ones
	if len(extComms) > 0 {
		if r.ExtendedCommunity != "" {
			r.ExtendedCommunity += " " + strings.Join(extComms, " ")
		} else {
			r.ExtendedCommunity = strings.Join(extComms, " ")
		}
	}

	// Determine if IPv6 based on NLRI criteria
	for key, vals := range r.NLRI {
		if key == "source" || key == "destination" {
			for _, val := range vals {
				if strings.Contains(val, ":") {
					r.IsIPv6 = true
					break
				}
			}
		}
	}

	return r
}
