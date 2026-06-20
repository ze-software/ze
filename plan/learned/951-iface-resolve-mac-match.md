# 951 -- iface-resolve: bind by hardware MAC (mac/match)

## Context

Follow-on to the logical-name resolver (950, `spec-iface-resolve-0-umbrella`). The
resolver shipped with one selector, `os-name` (bind a logical name to a kernel device by
*name*). The umbrella's stated intent was stronger: the **MAC is the physical binding** --
pin a logical interface to a NIC by its hardware address so it survives a name change
(slot move, NIC reorder) *and* an operational MAC override. The user asked to "link the
interface by matching a kernel device by its permanent MAC and not os-name." This unit adds
the `mac/match` selector: `interface { ethernet uplink { mac { match a0:..:56 } } }` binds
`uplink` to whichever kernel device carries that MAC, name irrelevant.

## Decisions

- **Match permanent MAC, fall back to current MAC.** `deviceMatchMAC(info)` returns the
  device's `PermanentMAC` (`IFLA_PERM_ADDRESS`) when present, else its current `MAC`.
  Matching the *permanent* address is the point -- the binding survives a `mac { address }`
  operational override on the same NIC. The current-MAC fallback exists because **every
  virtual device reports no permanent MAC** (confirmed: `ethtool -P` says "Permanent
  address: not set" for dummy, veth, and even a Docker/QEMU `eth0`). Without the fallback
  the feature would be unusable in veth/VM labs and untestable in CI (no perm-addr NICs).
  The permanent-MAC path is unit-tested with a stub; the real-backend scan is
  integration-tested via the fallback. (Flagged to the user; permanent-only is a one-line
  change to `deviceMatchMAC` if they want it.)
- **mac/match takes precedence over os-name.** When both are set on one interface, the MAC
  wins (the authoritative physical identity). `osDeviceFor` checks `permMACs[name]` first
  and only falls through to `os-name`/name when no MAC selector is set.
- **Resolution = scan, not lookup.** Unlike os-name (a direct `GetInterface(osName)`),
  matchByMAC must `ListInterfaces()` and compare every device's MAC. Lowest ifindex wins on
  a (kernel-anomalous) tie, for determinism. The backend call runs outside `r.mu`; only the
  small map reads/writes are locked.
- **Event routing needs a learned reverse binding (`boundOS`).** A mac/match name's kernel
  device is unknown until runtime, so the static `os->logical` reverse map can't route its
  events. Two additions: (1) on an up/appeared event, the resolver reads the device's MAC
  (`GetInterface`, outside the lock, only when mac/match selectors exist) and routes to
  names whose selector equals it -- so a deferred binding attaches when its device appears;
  (2) `boundOS[logical]=device` records each successful resolve, so a **down** event (where
  the device's MAC is no longer readable) still reaches the binding via the last-known
  device. Both feed the unified `logicalsForLocked(kernelName, devMatchMAC)`.
- **YANG `unique` is decorative here; enforce uniqueness in Go.** Added `unique "mac/match"`
  to the ethernet list for documented intent (parallel to `unique "mac/address"`), but
  `ze config validate` does NOT reject a duplicate -- ze's validator does not enforce YANG
  `unique` on this path (the umbrella already flagged this regression for `mac/address`). So
  `validateUniqueMatchMAC` (called from the iface verify path) rejects two ethernet
  interfaces selecting the same MAC, with canonical-form comparison so `AA:..`/`aa:..`
  collide.

## Consequences

- `iface.Resolve` / `iface.Addresses` transparently honor mac/match for every existing
  consumer (IS-IS today; sub-specs 4-7 as they migrate) -- no consumer change needed.
- `setMapping` is now `setMapping(osNames, permMACs)`; `setResolverConfig` feeds it both
  `cfg.osNameMap()` and `cfg.permMACMap()` (both ethernet-only). The old single-arg callers
  (3 unit, 2 integration) were updated.
- `resolve()`/`addresses()` route through a shared `osDeviceFor`; `effectiveOSName` is gone
  (its doc anchor in configuration.md was updated to `osDeviceFor`).

## Gotchas

- **Virtual devices have no permanent MAC.** dummy/veth/bridge and the CI `eth0` all report
  an empty `IFLA_PERM_ADDRESS`. Any test or design that assumes a perm MAC on a created
  device is wrong; verify with `ethtool -P <dev>` (or `ip -d link`, which prints `permaddr`
  only when present). This is *why* matchByMAC falls back to the current MAC.
- **The integration test exercises the fallback, not the permanent path.** Because CI hosts
  expose no perm-addr NICs, `TestResolveByMACBindsToDevice` sets a dummy's MAC and matches
  it (current-MAC path). The permanent-over-current preference is proven separately with a
  stub (`TestResolveByPermMACPrefersPermanentOverCurrent`). Do not "fix" the integration
  test to assert a permanent MAC -- there isn't one to assert.
- **A down event can't re-read the device's MAC.** Once a device is gone, `GetInterface`
  fails, so MAC-based routing of a down event is impossible. `boundOS` (last-known binding)
  is the only way the down event reaches a mac/match name. Drop it and a removed NIC leaves
  a stale binding cached.
- **Don't rely on YANG `unique` for ze config validation.** It parses but is not enforced on
  the iface path; a real constraint needs a Go check in the verify path.

## Verification (all green)

- Host unit: 7 new resolver tests (`TestResolveByPermMAC`, `...PrefersPermanentOverCurrent`,
  `...FallsBackToCurrent`, `...Precedence`, `...Absent`, `TestSubscribePermMACAppeared`,
  `TestPermMACDownInvalidatesBoundDevice`) + 3 config tests (`TestParseIfaceEntryMatchMAC`,
  `TestPermMACMapEthernetOnly`, `TestValidateUniqueMatchMAC`); no regression in the os-name
  suite.
- QEMU integration (verbose PASS): `TestResolveByMACBindsToDevice` (real netlink backend
  binds a logical name to a dummy by its MAC, not by name), alongside the unchanged
  `TestResolveRemapsLogicalNameToOSDevice` / `TestAddressesRemapAndClassify`.
- Config surface: `ze config validate` directly (darwin) accepts a valid mac/match config,
  rejects a malformed MAC (`does not match pattern`), and rejects a duplicate match
  (`both match MAC ...`). `test/parse/iface-mac-match.ci` covers all three for linux CI.
- `make ze-lint-changed` 0 issues; `make ze-doc-test` PASSED (anchors valid).

## Files

- `internal/component/iface/resolve.go` -- `osDeviceFor`, `matchByMAC`, `deviceMatchMAC`,
  `normalizeMAC`, `recordBinding`/`boundOS`, `permMACs`/`permMACOf`, `setMapping(os,mac)`,
  `logicalsForLocked(kernel,devMAC)`, `onLinkEvent` MAC lookup; removed `effectiveOSName`
- `internal/component/iface/config.go` -- `ifaceEntry.MatchMAC`, `parseIfaceEntry` mac/match
  read, `ifaceConfig.permMACMap`, `validateUniqueMatchMAC`
- `internal/component/iface/register.go` -- wire `validateUniqueMatchMAC` into verify path
- `internal/component/iface/yang/ze-iface-conf.yang` -- `mac { match }` leaf + `unique
  "mac/match"` on ethernet
- `internal/component/iface/resolve_test.go`, `config_macmatch_test.go`,
  `resolve_integration_linux_test.go` -- tests (+ updated `setMapping` call sites)
- `docs/guide/configuration.md` -- "Binding by Hardware MAC (mac/match)" + MAC Address
  Binding rewrite + fixed `effectiveOSName` anchor
- `test/parse/iface-mac-match.ci` -- config-surface (valid / bad pattern / duplicate)
