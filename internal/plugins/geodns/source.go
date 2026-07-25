// Design: plan/learned/992-geodns-1-config.md -- geodns source matcher (CIDR longest-prefix)
// Design: plan/learned/1027-dns-server-harness.md -- longest-prefix mechanism moved to
// internal/core/dnsserver; only the host-set label mapping stays geodns's.

package geodns

import "github.com/ze-software/ze/internal/core/dnsserver"

// buildMatcher maps each source entry's host-set name to the core matcher's
// generic label, then builds the shared longest-prefix matcher.
func buildMatcher(sources []sourceEntry) *dnsserver.Matcher {
	entries := make([]dnsserver.Entry, len(sources))
	for i, s := range sources {
		entries[i] = dnsserver.Entry{Prefix: s.Prefix, Label: s.HostSet}
	}
	return dnsserver.BuildMatcher(entries)
}
