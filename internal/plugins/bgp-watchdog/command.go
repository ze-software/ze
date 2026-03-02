// Design: docs/architecture/rib-transition.md — watchdog plugin extraction
// Related: pool.go — route pool management
// Related: watchdog.go — plugin main and SDK lifecycle

package bgp_watchdog

import (
	"fmt"
	"strings"
)

// cmdBuilder assembles "update text" commands from route attributes.
// Used by the config parser to pre-compute announce/withdraw commands
// stored in PoolEntry for efficient route injection.
type cmdBuilder struct {
	origin           string   // "igp", "egp", "incomplete"
	nextHop          string   // IP address or "self"
	localPref        uint32   // LOCAL_PREF (0 = omit)
	med              uint32   // MED (0 = omit)
	asPath           []uint32 // AS_PATH sequence
	communities      []string // "ASN:value" format
	largeCommunities []string // "ASN:value1:value2" format
	extCommunities   []string // "type:value" format
	pathID           uint32   // ADD-PATH path identifier
	labels           []uint32 // MPLS label stack
	rd               string   // Route Distinguisher ("ASN:NN" or "IP:NN")
	family           string   // "ipv4/unicast", "ipv6/unicast", etc.
	prefix           string   // "10.0.0.0/24", "2001:db8::/32", etc.
}

// announce builds an "update text ..." announce command.
// Attributes precede the NLRI section per text format grammar.
func (b *cmdBuilder) announce() string {
	var sb strings.Builder
	sb.WriteString("update text")

	// Attributes (order: origin, nhop, pref, med, path, communities, ext-communities)
	if b.origin != "" {
		sb.WriteString(" origin ")
		sb.WriteString(b.origin)
	}
	if b.nextHop != "" {
		sb.WriteString(" nhop ")
		sb.WriteString(b.nextHop)
	}
	if b.localPref != 0 {
		sb.WriteString(" pref ")
		fmt.Fprintf(&sb, "%d", b.localPref)
	}
	if b.med != 0 {
		sb.WriteString(" med ")
		fmt.Fprintf(&sb, "%d", b.med)
	}
	if len(b.asPath) > 0 {
		sb.WriteString(" path ")
		for i, asn := range b.asPath {
			if i > 0 {
				sb.WriteByte(',')
			}
			fmt.Fprintf(&sb, "%d", asn)
		}
	}
	if len(b.communities) > 0 {
		sb.WriteString(" s-com ")
		sb.WriteString(strings.Join(b.communities, ","))
	}
	if len(b.largeCommunities) > 0 {
		sb.WriteString(" l-com ")
		sb.WriteString(strings.Join(b.largeCommunities, ","))
	}
	if len(b.extCommunities) > 0 {
		sb.WriteString(" e-com ")
		sb.WriteString(strings.Join(b.extCommunities, ","))
	}

	// NLRI section
	sb.WriteString(" nlri ")
	sb.WriteString(b.family)
	b.writeNLRIModifiers(&sb)
	sb.WriteString(" add ")
	sb.WriteString(b.prefix)

	return sb.String()
}

// withdraw builds an "update text ..." withdraw command.
// Withdrawals only need family, prefix, and NLRI modifiers (no attributes).
func (b *cmdBuilder) withdraw() string {
	var sb strings.Builder
	sb.WriteString("update text nlri ")
	sb.WriteString(b.family)
	b.writeNLRIModifiers(&sb)
	sb.WriteString(" del ")
	sb.WriteString(b.prefix)

	return sb.String()
}

// writeNLRIModifiers writes per-NLRI-section modifiers (path-id, rd, label).
func (b *cmdBuilder) writeNLRIModifiers(sb *strings.Builder) {
	if b.rd != "" {
		sb.WriteString(" rd ")
		sb.WriteString(b.rd)
	}
	if len(b.labels) > 0 {
		for _, label := range b.labels {
			sb.WriteString(" label ")
			fmt.Fprintf(sb, "%d", label)
		}
	}
	if b.pathID != 0 {
		sb.WriteString(" info ")
		fmt.Fprintf(sb, "%d", b.pathID)
	}
}

// routeKey returns a unique key for this route, matching the format
// used by the reactor's StaticRoute.RouteKey() method.
func (b *cmdBuilder) routeKey() string {
	key := b.prefix
	if b.rd != "" {
		key = b.rd + ":" + key
	}
	return fmt.Sprintf("%s#%d", key, b.pathID)
}
