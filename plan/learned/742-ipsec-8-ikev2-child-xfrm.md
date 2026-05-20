# 742 -- Child SA and Dataplane Abstraction

## Context

After ipsec-7 established IKE SAs, no ESP Child SAs were created and no traffic could flow through XFRM interfaces. The IKE engine completed IKE_AUTH and then held the SA idle forever. The goal was to complete the post-authentication lifecycle: create Child SAs with derived ESP keys, install them in the kernel (or VPP), monitor liveness via DPD, and rekey before expiry.

## Decisions

- Chose a `Dataplane` interface with `Register`/`Load`/`Get` pattern (following `iface.Backend`) over embedding XFRM calls directly in the engine, because the same engine must support both Linux XFRM and VPP dataplanes
- Chose to place the dataplane package at `internal/component/ike/dataplane/` (inside the IKE component) over `internal/plugins/` because the dataplane is tightly coupled to the IKE engine's SA lifecycle, not a standalone plugin
- Chose `dataplane.Load("xfrm")` at engine startup over auto-detection, because the backend choice is a deployment decision (Linux vs VPP)
- Chose 1-second ticker for DPD/lifetime loop over per-timer goroutines, because it's simpler and the precision loss (up to 1s) is negligible for DPD intervals of 10s+
- Chose `IfID uint32` on `SiteToSitePeer` over runtime interface lookup, because the engine runs as a plugin subprocess without access to the iface backend
- Chose local VPP API message types over importing govpp/binapi/ipsec, because the IPsec binapi module is not in the vendored govpp dependency

## Consequences

- Child SA key material flows: `DeriveChildSAKeys` -> `ChildSAKeys` -> `SAParams` -> `dp.InstallSA` -> kernel/VPP. The `Clear()` chain must be maintained for any new path
- IKE SA rekey uses self-DH (simulated) until the CREATE_CHILD_SA wire exchange is encrypted (ipsec-9 prerequisite)
- VPP backend compiles and has the right API structure but cannot run until `govpp/binapi/ipsec` is vendored
- `SiteToSitePeer.IfID` must be populated by the config parser (ipsec-3) for XFRM interface binding to work
- The `inbound.go` handler classifies inbound INFORMATIONAL and CREATE_CHILD_SA messages but does not drive negotiation; it logs and sets SA state

## Gotchas

- `dataplane.Get()` returns nil if `Load()` was never called, and `createFirstChildSA` silently skips installation when dp is nil. This looked like working code in tests but meant zero SAs in the kernel. Fixed by adding `Load("xfrm")` to `runEngine`
- `resolveIfID` was initially hardcoded to 1, which would only work if an XFRM interface happened to have if_id=1. Changed to 0 (unbound) then to `peer.IfID` from config
- DPD `lastSent` must be initialized to `time.Now()` on creation; zero-value caused immediate probe
- `reconcilePeers` calls `ps.Stop()` which triggers `cleanupChild()` inside `maintainSA`, but the child SA may not have been created yet if the session is still in IKE_SA_INIT. The explicit `removeChildSA` after `Stop()` in reconcile handles this race
- The `peersEqual` comparison must include the new `IfID` field; forgetting it means config changes to IfID don't trigger peer restart

## Files

- Created: `internal/component/ike/dataplane/` (7 files: interface, xfrm backend, vpp backend, stubs, tests)
- Created: `internal/component/ike/engine/child.go`, `dpd.go`, `rekey.go`, `established.go`, `inbound.go` + tests
- Created: `test/ipsec/` (3 functional .ci tests)
- Modified: `engine/fsm.go`, `engine/reconcile.go`, `engine/register.go`, `engine/sa.go`, `engine/events.go`
- Modified: `crypto/keys.go` (DeriveChildSAKeysPFS), `ipsec/types.go` (IfID field)
