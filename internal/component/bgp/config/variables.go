// Design: docs/architecture/config/syntax.md — config variable substitution for dynamic peers
// Related: resolve.go — config tree resolution and group inheritance

package bgpconfig

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ResolveVariables replaces $remote_as, $local_as, and $remote_ip in string
// values within static route attributes and filter chain names.
// Only applies to known variable names; unknown $-prefixed tokens are left as-is.
func ResolveVariables(s string, localAS, remoteAS uint32, remoteIP string) string {
	if !strings.Contains(s, "$") {
		return s
	}
	s = strings.ReplaceAll(s, "$remote_as", textbuf.StringUint32(remoteAS))
	s = strings.ReplaceAll(s, "$local_as", textbuf.StringUint32(localAS))
	s = strings.ReplaceAll(s, "$remote_ip", remoteIP)
	return s
}

// ResolveFilterChain returns a new slice with variables resolved in each filter name.
func ResolveFilterChain(filters []string, localAS, remoteAS uint32, remoteIP string) []string {
	if len(filters) == 0 {
		return filters
	}
	hasVar := false
	for _, f := range filters {
		if strings.ContainsRune(f, '$') {
			hasVar = true
			break
		}
	}
	if !hasVar {
		return filters
	}
	resolved := make([]string, len(filters))
	for i, f := range filters {
		if !strings.ContainsRune(f, '$') {
			resolved[i] = f
			continue
		}
		resolved[i] = ResolveVariables(f, localAS, remoteAS, remoteIP)
	}
	return resolved
}

// ResolveASPathStrings resolves variable references in AS-path string values.
// Used during config parsing when AS-path values are still strings.
func ResolveASPathStrings(asPath []string, localAS, remoteAS uint32, remoteIP string) []string {
	if len(asPath) == 0 {
		return asPath
	}
	hasVar := false
	for _, s := range asPath {
		if strings.Contains(s, "$") {
			hasVar = true
			break
		}
	}
	if !hasVar {
		return asPath
	}
	resolved := make([]string, len(asPath))
	for i, s := range asPath {
		resolved[i] = ResolveVariables(s, localAS, remoteAS, remoteIP)
	}
	return resolved
}

// ResolveCommunityStrings resolves variable references in community string values.
func ResolveCommunityStrings(communities []string, localAS, remoteAS uint32, remoteIP string) []string {
	if len(communities) == 0 {
		return communities
	}
	hasVar := false
	for _, s := range communities {
		if strings.Contains(s, "$") {
			hasVar = true
			break
		}
	}
	if !hasVar {
		return communities
	}
	resolved := make([]string, len(communities))
	for i, s := range communities {
		resolved[i] = ResolveVariables(s, localAS, remoteAS, remoteIP)
	}
	return resolved
}
