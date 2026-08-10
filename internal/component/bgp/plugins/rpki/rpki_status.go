// Design: docs/guide/rpki.md -- `show bgp rpki status` action serialization (global + per-peer)
package rpki

import (
	"sort"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// actionString maps an action constant back to its config keyword.
func actionString(a uint8) string {
	switch a {
	case ASPAPolicyReject:
		return "reject"
	case ASPAPolicyLogOnly:
		return "log-only"
	default:
		return "accept"
	}
}

// appendGlobalActions writes the effective global actions object to b, read from the same
// atomics buildDecisions enforces. Buffer-first: no fmt, no per-call allocation.
func (rp *rPKIPlugin) appendGlobalActions(b *textbuf.Buffer) {
	b.Str(`,"actions":{"invalid":"`).Str(actionString(uint8(rp.originInvalidAction.Load())))    //nolint:gosec // stored as uint8
	b.Str(`","not-found":"`).Str(actionString(uint8(rp.originNotFoundAction.Load())))           //nolint:gosec // stored as uint8
	b.Str(`","aspa-invalid":"`).Str(actionString(uint8(rp.aspaInvalidAction.Load())))           //nolint:gosec // stored as uint8
	b.Str(`","aspa-unknown":"`).Str(actionString(uint8(rp.aspaUnknownAction.Load()))).Str(`"}`) //nolint:gosec // stored as uint8
}

// appendPeerActions writes the per-peer resolved actions array to b. Each entry lists the four
// resolved actions with the config level each was resolved from (peer/group/global). Peers are
// sorted by IP for deterministic output. Reads the same per-peer map buildDecisions uses.
func (rp *rPKIPlugin) appendPeerActions(b *textbuf.Buffer) {
	b.Str(`,"peer-actions":[`)

	if p := rp.perPeerActions.Load(); p != nil && len(*p) > 0 {
		ips := make([]string, 0, len(*p))
		for ip := range *p {
			ips = append(ips, ip)
		}
		sort.Strings(ips)

		for i, ip := range ips {
			if i > 0 {
				b.Byte(',')
			}
			set := (*p)[ip]
			b.Str(`{"peer":"`).Str(ip).Byte('"')
			appendResolvedLeaf(b, "invalid", set.OriginInvalid)
			appendResolvedLeaf(b, "not-found", set.OriginNotFound)
			appendResolvedLeaf(b, "aspa-invalid", set.ASPAInvalid)
			appendResolvedLeaf(b, "aspa-unknown", set.ASPAUnknown)
			b.Byte('}')
		}
	}

	b.Byte(']')
}

// appendResolvedLeaf writes one `"name":{"action":"...","source":"..."}` field to b.
func appendResolvedLeaf(b *textbuf.Buffer, name string, r resolvedAction) {
	b.Str(`,"`).Str(name).Str(`":{"action":"`).Str(actionString(r.Action))
	b.Str(`","source":"`).Str(r.Source.String()).Str(`"}`)
}
