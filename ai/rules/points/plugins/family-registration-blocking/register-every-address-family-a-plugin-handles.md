---
kind: note
level: MUST
stage:
---
NLRI plugins MUST register the address families they handle via
`family.MustRegister(afi, safi, afiStr, safiStr)` at package init time. The four
RFC 4760 base families (`IPv4Unicast`, `IPv6Unicast`, `IPv4Multicast`,
`IPv6Multicast`) live in `internal/core/family/registry.go` itself; everything
else is owned by the plugin's `types.go`.
