# 216 — Link-Local Next-Hop Cap77

## Objective

Implement RFC 5549 / Capability 77 (Extended Next-Hop Encoding) as a plugin, allowing IPv4 NLRI to carry IPv6 next-hops.

## Decisions

- Implemented as a plugin, not as an engine-level option — explicit user directive overrode the original plan. Capability negotiation is a plugin concern when it requires no FSM changes.
- UPDATE-level link-local next-hop encoding deferred: ExaBGP test cases K and L require generating UPDATEs with IPv6 next-hops for IPv4 prefixes, which is a separate encoding concern beyond capability advertisement.

## Patterns

- When a capability only affects OPEN negotiation (not UPDATE parsing), a plugin is the right boundary — the engine advertises and records it, plugins act on it.

## Gotchas

- The original plan considered engine-level changes. The user directive "NEEDS TO BE A PLUGIN - NOT AN OPTION" reversed this mid-spec. Always confirm the implementation boundary before coding.

## Files

- `internal/component/bgp/plugins/llnh/` — link-local next-hop plugin
- `internal/component/bgp/plugins/llnh/register.go` — plugin registration with capability code 5 (Extended Next-Hop)
