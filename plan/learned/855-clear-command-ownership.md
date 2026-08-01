# 855: Clear Command Ownership

**Spec:** `plan/spec-clear-command-ownership.md`
**Date:** 2026-06-04

## Summary

Moved every owner-specific `clear` command out of the central `internal/component/cmd/clear/` package into the component that owns the behavior: dns cache clear to resolve, ipsec SA clear to ike, interface counters schema to iface. The central clear package is now a bare verb-root anchor with no owner-specific handler or schema node.

## Key Decisions

1. **Full handler+schema move, not schema-only.** Moving only schema would leave the central package importing `ike/engine` and depending on hub injection. Full move deletes both couplings without new mechanism.

2. **Resolve owner calls resolver directly.** The hub-injected `RegisterDNSCacheClearProvider` indirection existed only because the handler was central. With the handler in the resolve owner, it calls `resolvers.DNS` directly.

3. **Central clear API YANG still declares owner RPCs.** The `ze-cli-clear-api.yang` module retains `rpc interface-counters` and `rpc dns-cache` as bare names. The self-containment gate covers the cmd YANG (command tree paths) and the handler registrations; the api YANG is a separate concern not addressed by this spec.

## Pattern

The migration follows the l2tp model: owner registers `pluginserver.RegisterRPCs` in its own `init()`, owner schema declares the full `clear <noun> ...` path via container merge onto the verb root, and the generated `plugin/all/all.go` aggregator blank-imports both the cmd and schema packages.

The self-containment gate (`TestClearOwnerRemovalLeavesNoResidue`) checks both the cmd and api YANG for banned owner tokens. The contract test (`TestAllClearCommandsHaveRegisteredRPC`) verifies exactly one handler per `ze-clear:*` WireMethod.

## Applicability

The same template applies to the remaining central verb packages: `del`, `set`, and owner-specific `show` subtrees still living centrally. Each move should add the self-containment gate token on the same commit.

## Files

None recorded.
