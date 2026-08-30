---
kind: directive
level: MUST
stage:
---
- **An NLRI plugin MUST register every address family it handles with `family.MustRegister(afi, safi, afiStr, safiStr)` at package init time.** Each family gets ONE canonical name and no alias, the plugin owns its SAFI spelling, and a number collision under a different name panics. The four RFC 4760 base families live in `internal/core/family/registry.go`; everything else is owned by its plugin. What registration guarantees, and how a forked plugin declares families over the wire, is `docs/architecture/plugin/plugin-system.md`, "Address family registration".
