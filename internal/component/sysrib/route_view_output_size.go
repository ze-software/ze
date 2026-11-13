// Design: docs/plugin-development/protocol.md -- complete routing publication frames.
// Related: route_view_output_test.go -- canonical JSON and native-size cross-checks.

package sysrib

import (
	"net/netip"
	"unicode/utf8"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// These counters measure this owner's canonical payload without encoding it.
// String and address fields remain owned Go values on the native route path.
func routeViewPublicationBytes(input routeViewPublication) int {
	n := 1 + routeViewOutputFieldBytes("session", routeViewOutputStringBytes(input.Session))
	n += routeViewOutputFieldBytes("cursor", routeViewOutputUintBytes(input.Cursor))
	n += routeViewOutputFieldBytes("operation", routeViewOutputUintBytes(input.Operation))
	n += routeViewOutputFieldBytes("batch", routeViewBatchBytes(input.Batch))
	return min(n, rpc.MaxMessageSize+1)
}

func routeViewBatchBytes(batch sysribevents.BestChangeBatch) int {
	changes := 4 // nil changes serialize as null, an empty non-nil vector as [].
	if batch.Changes != nil {
		changes = 2
		for i := range batch.Changes {
			if i > 0 {
				changes++
			}
			changes += routeViewOutputEntryBytes(&batch.Changes[i])
			if changes > rpc.MaxMessageSize {
				return rpc.MaxMessageSize + 1
			}
		}
	}
	n := 1 + routeViewOutputFieldBytes("family", routeViewOutputStringBytes(batch.Family.String()))
	n += routeViewOutputFieldBytes("changes", changes)
	if batch.IsReplay() {
		n += routeViewOutputFieldBytes("replay", 4)
	}
	return min(n, rpc.MaxMessageSize+1)
}

func routeViewOutputEntryBytes(entry *sysribevents.BestChangeEntry) int {
	var prefix [64]byte
	n := 1 + routeViewOutputFieldBytes("action", routeViewOutputStringBytes(entry.Action.String()))
	n += routeViewOutputFieldBytes("prefix", routeViewOutputStringBytes(string(entry.Prefix.AppendTo(prefix[:0]))))
	n += routeViewOutputFieldBytes("protocol", routeViewOutputStringBytes(entry.Protocol))
	if entry.NextHop.IsValid() {
		n += routeViewOutputFieldBytes("next-hop", routeViewOutputAddressBytes(entry.NextHop))
	}
	if entry.Interface != "" {
		n += routeViewOutputFieldBytes("interface", routeViewOutputStringBytes(entry.Interface))
	}
	if entry.OnLink {
		n += routeViewOutputFieldBytes("on-link", 4)
	}
	if entry.Weight != 0 {
		n += routeViewOutputFieldBytes("weight", routeViewOutputUintBytes(uint64(entry.Weight)))
	}
	if len(entry.Labels) > 0 {
		n += routeViewOutputFieldBytes("labels", routeViewOutputLabelsBytes(entry.Labels))
	}
	if entry.RouteType != 0 {
		n += routeViewOutputFieldBytes("route-type", routeViewOutputUintBytes(uint64(entry.RouteType)))
	}
	if entry.Metric != 0 {
		n += routeViewOutputFieldBytes("metric", routeViewOutputUintBytes(uint64(entry.Metric)))
	}
	if entry.TableID != 0 {
		n += routeViewOutputFieldBytes("table-id", routeViewOutputUintBytes(uint64(entry.TableID)))
	}
	if entry.SRv6SID.IsValid() {
		n += routeViewOutputFieldBytes("srv6-sid", routeViewOutputAddressBytes(entry.SRv6SID))
	}
	if len(entry.ECMPPaths) > 0 {
		n += routeViewOutputFieldBytes("ecmp-paths", routeViewOutputHopsBytes(entry.ECMPPaths))
	}
	if len(entry.Backup) > 0 {
		n += routeViewOutputFieldBytes("backup", routeViewOutputHopsBytes(entry.Backup))
	}
	return min(n, rpc.MaxMessageSize+1)
}

func routeViewOutputHopsBytes(hops []sysribevents.ECMPPath) int {
	n := 2
	for i := range hops {
		if i > 0 {
			n++
		}
		hop := &hops[i]
		n += 1 + routeViewOutputFieldBytes("next-hop", routeViewOutputAddressBytes(hop.NextHop))
		if hop.Interface != "" {
			n += routeViewOutputFieldBytes("interface", routeViewOutputStringBytes(hop.Interface))
		}
		if hop.OnLink {
			n += routeViewOutputFieldBytes("on-link", 4)
		}
		if hop.Weight != 0 {
			n += routeViewOutputFieldBytes("weight", routeViewOutputUintBytes(uint64(hop.Weight)))
		}
		if len(hop.Labels) > 0 {
			n += routeViewOutputFieldBytes("labels", routeViewOutputLabelsBytes(hop.Labels))
		}
		if n > rpc.MaxMessageSize {
			return rpc.MaxMessageSize + 1
		}
	}
	return n
}

func routeViewOutputLabelsBytes(labels []uint32) int {
	n := 2
	for i, label := range labels {
		if i > 0 {
			n++
		}
		n += routeViewOutputUintBytes(uint64(label))
		if n > rpc.MaxMessageSize {
			return rpc.MaxMessageSize + 1
		}
	}
	return n
}

// Each field includes its separator. The object's opening byte supplies the
// one remaining byte after its final separator is replaced by a closing brace.
func routeViewOutputFieldBytes(name string, value int) int { return len(name) + 4 + value }

func routeViewOutputUintBytes(value uint64) int {
	n := 1
	for value >= 10 {
		n++
		value /= 10
	}
	return n
}

func routeViewOutputAddressBytes(addr netip.Addr) int {
	if !addr.IsValid() {
		return 2
	}
	var text [2048]byte
	return routeViewOutputStringBytes(string(addr.AppendTo(text[:0])))
}

func routeViewOutputStringBytes(value string) int {
	n := 2
	for at := 0; at < len(value); {
		c := value[at]
		if c < utf8.RuneSelf {
			switch c {
			case '"', '\\', '\b', '\f', '\n', '\r', '\t':
				n += 2
			case '<', '>', '&':
				n += 6
			default:
				if c < 0x20 {
					n += 6
					break
				}
				n++
			}
			at++
			continue
		}
		r, width := utf8.DecodeRuneInString(value[at:])
		if width == 1 {
			// The canonical encoder writes a UTF-8 replacement character.
			n += 3
			at++
			continue
		}
		if r == '\u2028' {
			n += 6
			at += width
			continue
		}
		if r == '\u2029' {
			n += 6
			at += width
			continue
		}
		n += width
		at += width
		if n > rpc.MaxMessageSize {
			return rpc.MaxMessageSize + 1
		}
	}
	return min(n, rpc.MaxMessageSize+1)
}
