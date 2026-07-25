// Design: docs/architecture/core-design.md — community-based selective forwarding for RS
// Related: server_forward.go — selectForwardTargets integration point

package rs

import "github.com/ze-software/ze/internal/component/bgp/wireu"

// CommunityPolicy is an alias for the shared wire-level community policy type.
type CommunityPolicy = wireu.CommunityPolicy

// ParseCommunityPolicy delegates to the shared wire-level parser.
func ParseCommunityPolicy(payload []byte, rsASN uint32) CommunityPolicy {
	return wireu.ParseCommunityPolicy(payload, rsASN)
}
