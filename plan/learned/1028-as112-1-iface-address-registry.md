# 1028 -- as112-1-iface-address-registry

## Context

The AS112 anycast DNS feature (spec-as112 set) needs the as112 plugin's four
anycast addresses to appear on `lo` the moment the service is enabled, with
no operator-duplicated YANG config, and to disappear cleanly when disabled.
The existing `desiredState()`/reconciliation path in `internal/component/iface`
only knew about YANG-declared addresses; the imperative `interface-addr-add`
RPC bypasses `desiredState()` entirely and gets reconciled away on the next
commit. This spec built a generic, plugin-owned address registry consumed by
`desiredState()`, so any in-process plugin (not just as112) can declare
address ownership on an interface and have it survive reconciliation exactly
like YANG config does.

## Decisions

- Package-level mutex-guarded registry mirroring `backend.go`'s
  `RegisterBackend` shape, over reusing the `interface-addr-add` RPC: the RPC
  is imperative-only and bypasses `desiredState()`, so anything added through
  it is stripped on the next reconcile.
- Registration fires a reconcile-trigger (coalescing channel + dedicated
  worker goroutine, same shape as the existing vpp-ready trigger), over
  relying on the next unrelated config commit to pick up the change: plugin
  handler order is non-deterministic and nothing else re-runs reconcile on
  registration.
- Restricted to in-process plugins only, over building a wire-RPC for
  out-of-process registration: as112 deploys in-process; a wire equivalent is
  disproportionate scope for one consumer.
- `staleIfaces` (interfaces that just lost their last owner, needing one more
  cleanup pass, cleared after a clean pass) over an "ever owned" permanent
  set: the permanent version stripped kernel-native addresses (127.0.0.1)
  forever after any one-time register/unregister cycle -- found by adversarial
  review, not by the original TDD cycle.
- A single `reconcileMu` serializing config commits, vpp-event reconciles,
  AND registry-triggered reconciles, over scoping it to exclude config
  commits (tried and reverted): excluding commits avoids a bounded blocking
  risk (commit waits for a slow background pass) but reintroduces an
  unbounded one (commit rolls back because it raced a registry-triggered
  reconcile over the same address) that is *more* frequent than the
  vpp-reconnect scenario the narrower scope was designed to protect against.

## Consequences

- Any future plugin gets address ownership "for free" via
  `RegisterOwnedAddresses`/`UnregisterOwnedAddresses` -- no `internal/component/iface`
  changes needed, no per-plugin special-casing.
- `desiredState()`'s registry merge and the `staleIfaces` cleanup mechanism
  are generic (not `lo`-specific), even though `lo` is the only interface
  exercised by this spec set today.
- A config commit can now block briefly on a concurrent background reconcile
  pass (bounded by the plugin's 10s `ApplyBudget`); this is a deliberate
  tradeoff, documented at the `reconcileMu` declaration, and should not be
  "optimized away" by narrowing its scope again without re-reading that
  analysis -- the narrower version was tried and found worse.
- The joint `test/parse/as112-address-registry.ci` functional test is
  deferred to spec-as112-2, which is the first spec with a real plugin
  consumer to exercise end-to-end.

## Gotchas

- `parseUnits()` returns `nil, nil` when the operator declares zero units
  under a container -- `desiredState()` never created a `"lo"` key at all in
  that case, silently defeating the registry's cleanup path. The first fix
  (unconditionally creating the `"lo"` key) was itself wrong: it treated `lo`
  as always ze-managed, stripping the kernel's own 127.0.0.1/::1 whenever
  there was no YANG loopback config -- a regression an adversarial reviewer
  caught with a throwaway repro test before this spec closed.
- A shared test double (`fakeBackend.ListInterfaces()` in `config_test.go`)
  had a latent bug: it re-wrapped an already-slashed CIDR and appended a
  second, hardcoded `/24`, silently defeating stale-address detection in any
  test that needed an accurate "current kernel state" round-trip. No prior
  test happened to need that, so it went unnoticed until this spec's tests
  did.
- The obvious "track every interface ever registered" fix for the `lo`
  regression was itself a worse bug: it never forgot an interface, so any
  plugin registering-then-unregistering even once made every later,
  unrelated reconcile strip that interface's kernel-native addresses forever.
  Caught by two independent review agents in the same pass.
- Even the corrected `staleIfaces` (clear-after-clean-pass) had a race: since
  the reconcile mutex and the registry mutex are different locks held for
  different durations, `clearStaleIfaces()` wiping the whole map could
  discard an entry a concurrent `Unregister` added mid-pass. Fixed by having
  the snapshot function return exactly which names it saw, and clearing only
  those.
- General lesson: every "obvious" fix in this spec was wrong on the first
  try, and each wrong fix was only caught by a dedicated adversarial
  re-review pass, not by the TDD cycle that produced it -- the tests that
  existed at each point were internally consistent with the (buggy) design,
  not with the real requirement. Multi-round independent review found bugs
  the author's own re-reading did not.

## Files

- `internal/component/iface/address_owner.go` (new) -- the registry
- `internal/component/iface/address_owner_test.go` (new)
- `internal/component/iface/config_apply_test.go` (new)
- `internal/component/iface/config_apply.go` -- `desiredState()` merge, `reconcileOnRegistryChange`, `reconcileMu`, `clearStaleIfaces` wiring
- `internal/component/iface/register.go` -- `registryReconcileCh` worker, trigger wiring, `nonBlockingNotify`
- `internal/component/iface/config_test.go` -- `fakeBackend.ListInterfaces()` fix
- `internal/component/iface/operation.go` -- `desiredState()` 3-value signature update
- `docs/architecture/core-design.md` -- section 14 registry entry
