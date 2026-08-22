# BGP

Ze includes a native BGP engine for external and internal peering, route-server work, policy-driven imports and exports, graceful operation, and programmable route injection.

Use this page as the BGP entry point. It links the operational guides and the generated feature pages instead of making the homepage choose one narrow BGP topic.

## Start here

- [BGP peering](../bgp-peering/) covers peers, groups, dynamic peer groups, address families, prefix limits, attached programs, ADD-PATH, BGP Role, and session verification.
- [BGP policy](../bgp-policy/) covers import filters, export filters, redistribution, and route checks.
- [BGP resilience](../bgp-resilience/) covers route refresh, graceful restart, RIB persistence, and route reflection.
- [BGP Role](../bgp-role/) covers RFC 9234 role negotiation and OTC route-leak prevention.
- [BGP healthcheck](../bgp-healthcheck/) covers service checks that control BGP announcements.
- [Graceful Restart](../graceful-restart/) covers RFC 4724 restart behavior in detail.

## Feature references

- [Native BGP engine](../../features/bgp-protocol/) documents the protocol implementation, negotiated capabilities, path handling, capture, replay, and wire-level behavior.
- [BGP configuration](../../features/bgp-configuration/) documents the BGP configuration surface and points to the generated configuration reference.
- [Programmable APIs](../../features/api-commands/) documents REST, gRPC, and gNMI access to the same command and API engine.
- [MCP integration](../../features/mcp-integration/) documents AI-facing tools generated from the same command registry.

## Labs and use cases

- [BGP interop lab](../../labs/bgp-interop/) runs Ze against FRR, BIRD, and GoBGP.
- [Route server at an IXP](../../use-cases/route-server/) shows Ze as an RFC 7947 route server.
- [Transit edge with RPKI](../../use-cases/transit-edge-rpki/) shows a two-transit edge with origin validation.
- [FlowSpec injection](../../use-cases/flowspec-injection/) shows controlled BGP FlowSpec announcements.
- [BGP performance testing](../../use-cases/bgp-performance/) shows repeatable route-server measurement.

## When to use the feature page directly

Use [Native BGP engine](../../features/bgp-protocol/) when you need protocol internals, RFC behavior, or proof links. Use [BGP peering](../bgp-peering/) when you need to configure or operate a session.
