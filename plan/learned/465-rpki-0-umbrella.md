# 465 -- RPKI Umbrella Architecture

## Objective

Design and implement RPKI origin validation as an optional plugin (`bgp-rpki`) that coordinates with `bgp-adj-rib-in` to gate routes pending validation. When not loaded, zero overhead.

## Decisions

- **Plugin-based, not engine-integrated:** RPKI is a plugin (`bgp-rpki`) that coordinates with adj-rib-in via DispatchCommand. Engine stays RPKI-unaware.
- **Validation gate pattern:** bgp-rpki sends `adj-rib-in enable-validation` at startup. Routes held as pending until accept/reject command received. Modeled after GR retain/release pattern.
- **Fail-open on timeout (30s default):** Pending routes promoted if validator does not respond. Prevents route black-holing if RPKI plugin crashes.
- **Per-prefix validation, not per-UPDATE:** A single UPDATE can contain multiple prefixes with different ROA coverage. Each prefix validated independently.
- **RTR v1 only (RFC 8210):** v0 compatibility deferred. v1 includes timing parameters and version negotiation.
- **ASPA deferred:** Separate concern with its own PDU types and algorithm.
- **Dependency ordering:** bgp-rpki depends on bgp-adj-rib-in so adj-rib-in starts first and registers command handlers.

## Patterns

- **Inter-plugin coordination via DispatchCommand:** established pattern (GR uses retain/release, RPKI uses enable-validation/accept/reject). No direct imports between plugins.
- **Validation states are local, not BGP attributes:** Valid/Invalid/NotFound computed locally per route + cache. RFC 8097 extended community is decorative only.
- **Async validation worker:** handleEvent() queues to validationWorker goroutine to avoid blocking SDK event callback.
- **Atomic ROA cache updates:** ApplyDelta() removes + adds VRPs under single lock to prevent partial-state visibility.

## Gotchas

- Phases 2-4 (RTR client, origin validation, config/YANG) were implemented directly without individual spec files. The umbrella spec planned them but they were built in implementation sessions.
- Cache server preference is parsed from config but all servers are queried independently (preference not used for selection).
- Router Key PDU (Type 9, for BGPsec) is parsed but ignored.

## Files

- `internal/component/bgp/plugins/rpki/` -- plugin directory (rpki.go, roa_cache.go, validate.go, rtr_session.go, rtr_pdu.go, emit.go, rpki_config.go, register.go)
- `internal/component/bgp/plugins/rpki/schema/ze-rpki.yang` -- YANG schema
- `internal/component/bgp/plugins/adj_rib_in/rib_validation.go` -- validation gate in adj-rib-in
- `internal/component/bgp/plugins/adj_rib_in/rib_commands.go` -- accept/reject/enable-validation commands
- `cmd/ze-rtr-mock/main.go` -- mock RTR server for testing
- `docs/guide/rpki.md` -- user guide
- `test/plugin/rpki-*.ci` -- 13 functional tests
