// Design: docs/architecture/config/syntax.md — FlowSpec route extraction from config tree
// Overview: bgp_routes.go — route extraction orchestrator

package bgpconfig

import (
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// NOTE: The update{} nlri form (ipv4/flow ...) is parsed by the bgp-nlri-flowspec
// plugin's config route parser (plugins/nlri/flowspec/config.go). The functions
// below handle only the legacy ExaBGP flow{ route{ match{} then{} } } block.

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
				rate, unit, hasUnit := strings.Cut(value, " ")
				if hasUnit {
					switch unit {
					case "packets":
						extComms = append(extComms, "rate-limit:"+rate+":packets")
					case "bytes":
						extComms = append(extComms, "rate-limit:"+rate)
					default:
						extComms = append(extComms, "rate-limit:"+value)
					}
				} else {
					extComms = append(extComms, "rate-limit:"+value)
				}
			case "rate-limit-packets":
				extComms = append(extComms, "rate-limit:"+value+":packets")
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
		joined := textbuf.Join(extComms, " ")
		if r.ExtendedCommunity != "" {
			var tb textbuf.Buffer
			r.ExtendedCommunity = tb.Str(r.ExtendedCommunity).Byte(' ').Str(joined).String()
		} else {
			r.ExtendedCommunity = joined
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
