# Spec: runtime-kernel-carries-every-tunnel-kind

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | component |
| Depends | - |
| Phase | - |
| Deferral shard | `-` |
| Updated | 2026-08-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Ze models nine tunnel kinds in YANG. The appliance kernel can create one of
them. An operator can configure the other eight, have `ze config validate`
accept the config, and watch the daemon refuse it at runtime.**

`internal/component/iface/yang/ze-iface-conf.yang` carries a `case` with a full
parameter container for each of `gre`, `gretap`, `ip6gre`, `ip6gretap`, `ipip`,
`sit`, `ip6tnl`, `ipip6` and `vxlan`. `internal/component/iface/tunnel.go` names
all nine in `TunnelKind*`, and `internal/component/iface/discover.go` recognises
them on the wire side.

**Measured 2026-08-11** from the config of the kernel that was actually built and
booted (`~/.cache/ze/runtime-kernel/7.1.4-runtime-amd64-*/config`), not from the
fragments:

| Ze tunnel kind | Kernel symbol | State in the built kernel |
|----------------|---------------|---------------------------|
| `gre`, `gretap` | `CONFIG_NET_IPGRE` | absent; `CONFIG_NET_IPGRE_DEMUX` explicitly not set |
| `ip6gre`, `ip6gretap` | `CONFIG_IPV6_GRE` | absent |
| `ipip` | `CONFIG_NET_IPIP` | explicitly not set |
| `ip6tnl`, `ipip6` | `CONFIG_IPV6_TUNNEL` | explicitly not set |
| `vxlan` | `CONFIG_VXLAN` | explicitly not set |
| `sit` | `CONFIG_IPV6_SIT` | `=m`, the only one present |

**How it was found.** `test/reload/test-tx-iface-tunnel-remove.ci` declares a GRE
tunnel. In QEMU the daemon answered
`iface: create tunnel "tgrerm0" (kind gre): operation not supported`. That is a
runtime proof for GRE; the other seven come from the built kernel's own config,
which is the stronger evidence and covers kinds no test exercises.

**Owner ruling (Thomas, 2026-08-11): compile the kernel with every tunnel type.**
He chose this over narrowing the YANG. A NOS that models a tunnel and cannot
create it is the wrong half to keep.

## What this spec is NOT

**It is not about the startup cascade.** The GRE failure also took every `bgp-*`
plugin down, because `StartupCoordinator.PluginFailed` closes the shared
`stageCh` and every plugin waiting in `WaitForStageProgress` is stopped. That was
raised as a possible defect and **Thomas ruled on 2026-08-11 that it is correct
behaviour**: a router that starts with half its features is worse than one that
does not start, and a firewall plugin failing while BGP came up anyway would
forward traffic unfiltered while looking healthy. Recorded as a load-bearing
invariant in `plan/learned/DESIGN-HISTORY.md`. Do not reopen it here.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/appliance/` - how the runtime kernel is built and packed
  -> Constraint: `gokrazy/kernel/runtime.config` is a FRAGMENT. It enables features explicitly; a symbol absent from it and from the builder's common configs is absent from the kernel.
  -> Constraint: `gokrazy/kernel/runtime.require` lists symbols the build MUST end up with. No tunnel symbol is required today, so nothing checks them.

**Key insights:** (minimal context to resume after compaction)
- Nine kinds modelled, one buildable. Evidence is the built kernel's config, not a test.
- `runtime.require` is the mechanism that stops this regressing; it currently guards none of these.
- The plugin-startup cascade is settled: Thomas ruled it correct behaviour on
  2026-08-11 and `plan/learned/DESIGN-HISTORY.md` records it. Do not reopen it.

## Current Behavior (MANDATORY)

**Source files read:** (2026-08-11, each at its producer)
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - nine `case` blocks with parameter containers
- [ ] `internal/component/iface/tunnel.go` - `TunnelKind*` constants and the kind-to-name map
- [ ] `internal/component/iface/discover.go` - the recognised-kind set
- [ ] `gokrazy/kernel/runtime.config` - the fragment; carries no tunnel symbol
- [ ] `gokrazy/kernel/runtime.require` - the must-be-present list; carries no tunnel symbol
- [ ] `gokrazy/kernel/Makefile` - selects fragments plus `tools/kernel-builder/common/*.config`

**Behavior to preserve:**
- Every interface type that works today keeps working.
- Image size stays within whatever bound the appliance build already enforces.

**Behavior to change:**
- A tunnel kind Ze offers can be created on the appliance.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator writes a `tunnel { gre { ... } }` stanza and commits.

### Transformation Path
1. YANG validates the stanza.
2. `internal/component/iface` builds a netlink link of that kind.
3. **Fails today:** the kernel has no module or built-in for it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| config -> iface | the tunnel case | Yes, read |
| iface -> kernel | netlink RTM_NEWLINK | Yes, and this is where it fails |
| kernel build -> image | `runtime.config` plus `runtime.require` | Yes, read |

### Integration Points
- `runtime.require` is the regression guard; adding the symbols there is what stops this recurring.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | Nothing is bypassed; a capability is absent |
| No unintended coupling | Yes | A kernel config change couples nothing in Go |
| No duplicated functionality | Yes | One config fragment, one require list |
| Registration over hardcoding | N-A | Kernel config is a list by nature |

## Risks & Assumptions

| # | Statement | Status |
|---|-----------|--------|
| R-1 | Enabling eight tunnel families grows the kernel and the image; the appliance may have a size budget this breaks | closed. Measured on the rebuilt amd64 kernel: `vmlinuz` 16352256 -> 16450560 bytes, +98304 (+0.60%). No target in `mk/` or `gokrazy/kernel/` bounds the `vmlinuz` size, so there is no budget to break |
| R-2 | Some of these want a companion symbol to be useful (`NET_IPGRE_DEMUX` for GRE, `NET_UDP_TUNNEL` for VXLAN, which is already `=y`) | closed, derived from linux 7.1.4 Kconfig. `NET_IPGRE` and `IPV6_GRE` both `depends on NET_IPGRE_DEMUX`, so the gate must be `=y` or both cap at `=m`. `VXLAN` selects `NET_UDP_TUNNEL`, so the existing `=y` was never what was missing. Every other dependency is a promptless symbol the seven requests `select` |
| R-3 | Built as `=m`, a module must also be packed into the image and be loadable at boot; `=y` avoids that question | closed by choosing `=y` for all seven. `enforce_required_symbols` (`tools/kernel-builder/build.py`) accepts `=y` alone, so a `.require` line and an `=m` answer cannot coexist: the guard AC-2 asks for forces the choice |
| A-1 | Linux has no tunnel support the daemon can fall back on when the symbol is absent | verified: the daemon reports `operation not supported` from the kernel |

## Blast Radius

The appliance image, every deployment. No Go code need change.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| operator configures each tunnel kind | -> | `internal/component/iface` tunnel create | one QEMU case per kind asserting the link appears |
| the build produces a kernel | -> | `runtime.require` | the required-symbol check fails if any is dropped |

## Acceptance Criteria

| AC ID | Piece | Expected Behavior |
|-------|-------|-------------------|
| AC-1 | every modelled kind is buildable | each of the nine creates successfully on the appliance kernel |
| AC-2 | the guard exists | each symbol is in `runtime.require`, so dropping one fails the build |
| AC-3 | proven, not asserted | a QEMU test creates each kind and asserts the link |
| AC-4 | size is known | the image size change is measured and recorded, not assumed |
| AC-5 | `test-tx-iface-tunnel-remove` passes | the test that found this goes green, and with it reload 37, 38 and 39. Met by giving this spec's own `.ci` its own endpoint pairs: the three failures were a collision with it, not a product defect (see Known Limitations) |

## End-to-End User Stories

An operator configures a GRE tunnel to a remote site, commits, and the tunnel
comes up. Today the commit is accepted and the daemon refuses it.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTunnelKindsAllHaveKernelSupport` | `internal/component/iface/tunnel_test.go` | every `TunnelKind*` maps to a symbol listed in `runtime.require`, so the two lists cannot drift | passes |
| `TestApplyTunnelKeepsExistingNetdevWhenPreviousSpecIsLost` | `internal/component/iface/config_apply_test.go` | an apply holding no previous spec succeeds against a tunnel netdev a previous RUN of ze created, and leaves it alone | passes; red without the fix with `[tunnel tgre0 create: file exists]` |
| `TestApplyTunnelChangedSpecStillReachesTheKernel` | `internal/component/iface/config_apply_test.go` | a changed Spec is still deleted and re-created, so the kept-netdev branch is not blind to an operator edit | passes |
| `TestApplyTunnelRollbackDoesNotDeleteAKeptNetdev` | `internal/component/iface/config_apply_test.go` | a later failing step rolls back without deleting a netdev the pass did not create | passes; red when the kept branch sets `created = true` |
| `TestApplyTunnelFailsWhenAnotherDeviceKindHoldsTheName` | `internal/component/iface/config_apply_test.go` | a dummy holding the tunnel's name fails the apply, so the later phases never push the tunnel's MTU, admin state and addresses onto it | passes; red with the kind check removed: `Should NOT be empty, but was []` |
| `TestApplyTunnelFailsWhenTheNameHoldsAnotherTunnelKind` | `internal/component/iface/config_apply_test.go` | an `ipip` link under a name the config now calls `gre` fails the apply: the encapsulation edit made while ze was down is not silently dropped | passes; red with the kind check removed: `Should NOT be empty, but was []` |
| `TestApplyTunnelReportsTheCreateErrorWhenTheNameIsFree` | `internal/component/iface/config_apply_test.go` | a create the kernel refused, with the name free, reports the kernel's error rather than a name conflict | passes; red with the read-back error path removed: the apply reports `"tgre0" is held by a device of type unreadable` |
| `TestEveryTunnelKindHasAKernelLinkType` | `internal/component/iface/config_apply_test.go` | every modelled kind maps to the link type the kernel reports, so the guard can identify a device of that kind rather than failing closed on it | passes |
| `TestCITunnelEndpointsAreUniqueAcrossTests` | `internal/test/runner/tunnel_endpoint_lint_test.go` | no two link-creating `.ci` claim one tunnel endpoint pair, which is the collision that produced the wrong diagnosis | passes; reports the pre-fix `iface-tunnel-kinds.ci` against `test-tx-iface-tunnel-remove.ci` when given it |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| image size | build budget | the measured post-change size | N/A | whatever the appliance build already refuses |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `iface-tunnel-kinds` | `test/plugin/*.ci` | each modelled kind is created and appears in `show interface` | PASSED on the rebuilt kernel in QEMU, 5.8s, all nine kinds created |
| `test-tx-iface-tunnel-create`, `-remove`, `-modify-key` | `test/reload/` | the existing tests that found this pass | all three PASSED in QEMU once this spec's `.ci` stopped taking their endpoint pairs. No product change was needed for them |

### Interop Tests (Scope: protocol)
N-A. A tunnel encapsulation is exercised against the local kernel here; no peer
daemon negotiates it. The QEMU rows above are the real-system proof.

## Files to Modify

- `gokrazy/kernel/runtime.config` - add the symbols
- `gokrazy/kernel/runtime.require` - require them, so the build refuses to drop them
- `internal/component/iface/config_apply.go` - the tunnel create step keeps a
  tunnel netdev an earlier RUN of ze created, instead of failing the whole apply
  on EEXIST, and fails closed when any other device holds the name
- `internal/component/iface/tunnel.go` - `kernelLinkTypes`, the link type the
  kernel reports per kind, which is what the kept-netdev branch compares against
- `internal/component/iface/config_test.go` - `fakeBackend.CreateTunnel` answers
  `file exists` for a name it already holds, as the kernel does, and reports the
  kernel's link type for the device it leaves behind
- `internal/component/iface/config_apply_test.go` - the tunnel apply tests
- `docs/features/interfaces.md` - "Tunnel Reload Behaviour" gains the third
  case: the apply that holds no previous spec
- `test/plugin/iface-tunnel-kinds.ci` - the endpoint-uniqueness comment now
  names the check that enforces it

## Files to Create

- `internal/component/iface/tunnel_test.go` - `TestTunnelKindsAllHaveKernelSupport`,
  the Go half of the kernel guard
- `test/plugin/iface-tunnel-kinds.ci` - the QEMU half: every modelled kind is
  created on the running kernel
- `internal/test/runner/tunnel_endpoint_lint_test.go` -
  `TestCITunnelEndpointsAreUniqueAcrossTests`, which enforces across the `.ci`
  corpus the endpoint constraint whose absence produced a wrong diagnosis
- `test/plugin/iface-tunnel-restart.ci` (added in round 3, for review ISSUE 3) -
  ze runs twice over one tunnel config in one VM, and the kernel ifindex is
  carried from the first run to the second and compared. It is the only
  evidence that reaches the restart branch on a real kernel, and it has NOT
  been run

## Implementation Steps

1. Derive the FULL symbol set from the kernel's own dependencies, not from this
   table alone: each of `NET_IPGRE`, `NET_IPGRE_DEMUX`, `IPV6_GRE`, `NET_IPIP`,
   `IPV6_TUNNEL`, `VXLAN` plus whatever they `depend on`.
2. Decide `=y` or `=m` per symbol against R-3.
3. Add them to `runtime.config` and to `runtime.require`.
4. Rebuild, and read the BUILT config back to confirm each landed. The fragment
   asking is not the kernel having.
5. Measure the image size change.
6. Write the tests.

## Design Insights

**The fragment is not the kernel.** This defect was invisible because everyone
read `runtime.config`, which is a request. The authority is the `config` file the
build emits beside `vmlinuz`, and reading it is what turned one proven gap into
eight.

**`runtime.require` is the reason this will not come back.** A symbol in
`runtime.config` can be silently dropped by a kernel version bump or a
dependency change. A symbol in `runtime.require` fails the build instead.

## Key Design Decisions

| # | Decision |
|---|----------|
| D-1 | Compile every modelled kind in, rather than narrow the YANG. Owner ruling, 2026-08-11 |
| D-2 | `=y` for all seven symbols. Three reasons, any one sufficient: `enforce_required_symbols` accepts `=y` alone, so the AC-2 guard and `=m` are mutually exclusive; a QEMU run boots this kernel beside Alpine's `/lib/modules` for another version, so no module of this kernel loads; and a modular `NET_IPGRE_DEMUX` caps `NET_IPGRE` and `IPV6_GRE` at `=m` |
| D-3 | The symbols go in `runtime.config` / `runtime.require`, not the `kernel.*` pair. `resolve_fragments` (`tools/kernel-builder/run.py`) gives the runtime profile both, and `gokrazy/kernel/` defines only that profile, so the two pairs reach the same kernel. The choice is meaning: `runtime.config` already owns the interface-kind block added 2026-08-07 for this same defect class, and `kernel.config` is the base a second profile in this directory would share |
| D-4 | `runtimeKernelRequirements` (`internal/appliance/kernelreq.go`) is left alone. It is a deliberately partial Go floor under the manifest, and `TestTunnelKindsAllHaveKernelSupport` already fails when a symbol leaves `runtime.require`, tied to the Go kind list rather than to a second copy of it |
| D-5 | The tunnel create step KEEPS a netdev it meets when that netdev is a tunnel of the configured kind, rather than deleting and re-creating it. Matches the five sibling create steps in the same function (dummy, veth, wireguard, xfrm, bridge), which each answer an existing link with "keep it, and leave `created` false so the rollback cannot delete it". Deleting to rebuild an identical link would break the traffic crossing a tunnel across every reload and every restart, which is the property the `prev == e.Spec` skip was written to protect |
| D-6 | The KIND of the kept link is compared; its parameters are not, and the WARN says so. `InterfaceInfo.Type` carries the netlink link type, which is enough to say a device is a `gre` tunnel and not a dummy or a `vxlan`, so keeping is conditional on it (`kernelLinkTypes`). It is not enough for `local`, `remote`, `key` or `ttl`: `InterfaceInfo` has no encapsulation field, and the type alone cannot even separate `ip6tnl` from `ipip6`, which differ only in a Proto the read-back drops. Inventing that read-back here would ship a comparison no test in this session can run against a kernel, and a false drift verdict deletes a working tunnel on every restart. The warning names exactly what was not verified |
| D-7 | The apply FAILS when a device of another kind holds the name, rather than keeping it with a warning. `reconcileOwnedDevices` in the same file already fails closed on that state for macvlans, and the reason is the same: Phases 2, 2c and 3 push MTU, admin state and addresses onto whatever holds the name, so keeping a foreign device silently makes it the carrier of a tunnel's config. It also keeps the pre-existing behaviour for the operator who edits `encapsulation` while ze is down: `previous` is nil at plugin start, so that edit reaches the create rather than the delete-then-create branch, and it failed loudly before this spec |

## Known Limitations

`sit` was `=m`, which was its Kconfig `default m` answering rather than a
request. It is `=y` in the rebuilt kernel with the other six, so the packing and
loading question it raised is gone.

**AC-1 and AC-3 are proven.** `test/plugin/iface-tunnel-kinds.ci` PASSED on the
rebuilt kernel in QEMU in 5.8s, creating all nine kinds.

**AC-5 is met, and the diagnosis first written here was wrong.** With the kernel
able to create a GRE tunnel, `test/reload/test-tx-iface-tunnel-create` and
`-remove` failed on a name they had never created:

```
WARN  tunnel tgretap create  err="iface: create tunnel \"tgretap\" (kind gretap): file exists"
WARN  config reload: transaction failed  error="config apply partial failure: plugin interface apply failed: ..."
```

That was read as a reload-idempotency defect and a product change was written
for it. **The real cause was a collision between two tests.** This spec's new
`test/plugin/iface-tunnel-kinds.ci` used the endpoint pairs of
`test-tx-iface-tunnel-create.ci`, it runs first in the shared VM, and it left
its links behind. The kernel refuses a second tunnel on a local/remote pair the
driver already holds, whatever the new device is named, so the reload tests met
EEXIST over another test's links. Moving this spec's endpoints to 192.0.2.201+
and 2001:db8:c8:: made all three reload tests pass with NO product change.
`TestCITunnelEndpointsAreUniqueAcrossTests`
(`internal/test/runner/tunnel_endpoint_lint_test.go`) now enforces that across
the `.ci` corpus, so the collision cannot be re-created silently.

**The product change is kept, because it fixes a different and real defect: a ze
RESTART.** `netlink.LinkAdd` sends `NLM_F_CREATE|NLM_F_EXCL`
(`vendor/github.com/vishvananda/netlink/link_linux.go`), nothing in
`internal/component/iface` deletes tunnels when the daemon stops, and
`OnConfigure` runs `applyConfig(cfg, nil, b)` at every plugin start. So the
second start of ze over its OWN leftover tunnel netdevs failed the whole
interface apply, and under the fail-closed startup cascade the daemon then
refused to start. The five sibling create steps (dummy, veth, wireguard, xfrm,
bridge) already survived that state; the tunnel step did not. No test had
reached it, which is why the QEMU failure was mistaken for it.

The fix is D-5, D-6 and D-7. One thing stays open, shared with the xfrm step
beside it: the KIND of a kept link is checked, its parameters are not, so an
edit to `local`, `remote`, `key` or `ttl` made while ze was down reaches that
link's addresses and MTU and not its encapsulation. It needs a tunnel read-back
the `Backend` interface does not have. Recorded in
`plan/journal/guard-added-to-one-half-of-a-pair.md`.

**The five sibling create steps stay kind-blind, and that is deliberate.** Round
2 raised it, judged it separable and strictly pre-existing, and the row in
`plan/journal/guard-added-to-one-half-of-a-pair.md` now names each step rather
than saying only that they keep the link. No step this spec touched has the
defect; each needs the same read-back the tunnel step now does, and six edits do
not belong in this closing commit (`ai/rules/rule-precedence.md`).

**`config_apply.go` is 1411 lines against a 1000-line threshold.** It was over
that threshold before this work at 1358, so the file did not cross it here. The
split is not this spec's to make.

## Verification Evidence (what was and was not run)

**`make ze-verify` has NOT been run for this spec.** The gates below are what
was run, and the claim in the Goal Gates checklist stays unticked until the
full gate runs before the commit (`ai/rules/git-safety.md`).

| Ran | Result |
|-----|--------|
| `make ze-test-pkg PKG=./internal/component/iface` | ok, 1.9s |
| `make ze-test-pkg PKG=./internal/test/runner RUN=TestCITunnel` | ok |
| `make ze-lint-changed` | no finding in any file this spec touches; the four `unused` findings it reports are in `internal/chaos/peer`, `internal/component/l2tp` and `internal/plugins/isis`, which other sessions are editing in this shared checkout |
| QEMU (`iface-tunnel-kinds`, reload 37/38/39) | run in the round that rebuilt the kernel, not since |

Round 3 (the round-2 review fixes) ran these, and no kernel was rebuilt:

| Ran | Result |
|-----|--------|
| `make ze-test-pkg PKG=./internal/component/iface` | ok, 1.8s |
| `make ze-test-pkg PKG=./internal/test/runner` | ok, 13.4s, the whole package |
| `make ze-lint-changed` | first run: 5 issues. One was this spec's own, and round 2's Verification Evidence had missed it: `goconst`, `string "unknown" has 3 occurrences`. Round 2's `fakeBackend.CreateTunnel` line took the package's count of that literal from two to three. Fixed by naming it, `fakeUnknownLinkType` (`internal/component/iface/config_test.go`), which is what the linter asks for and says what the fake means: a LINK TYPE no kernel reports, not `TunnelKind.String()`'s kind with no YANG spelling. Second run: the four pre-existing `unused` findings, none in a file this spec touches |
| `python3 scripts/dev/validate.py` over the changed iface files | `all checks passed`, after removing the two dead exported symbols it named |
| `make ze-relax-census` | 752 tokens, ceiling 752. The new `.ci` adds none |
| `make ze-doc-test` | FAILS on `ai/DOCS-TO-CODE.md is stale`, which is another session's closure residue (the stale rows name `spec-fixit-forward-rail-initial-sync-ordering`, a spec no longer on disk, and a `bgp/reactor` test file). Regenerating it was reverted so this spec does not carry a file it does not own. Nothing in this spec's changed set is named by that check |
| QEMU (`iface-tunnel-restart`) | **NOT RUN.** The test is written and needs a VM this session could not start |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Every kind Ze models can be created on the appliance kernel | Done | `gokrazy/kernel/runtime.config` | Seven `=y` lines: `NET_IPGRE_DEMUX`, `NET_IPGRE`, `IPV6_GRE`, `NET_IPIP`, `IPV6_TUNNEL`, `IPV6_SIT`, `VXLAN`. Nine kinds through six symbols plus the GRE demux gate |
| The build refuses to drop one | Done | `gokrazy/kernel/runtime.require` | Same seven pinned. `enforce_required_symbols` (`tools/kernel-builder/build.py`) accepts `=y` alone, so a `default m` answer now fails the build |
| Keep the YANG, do not narrow it (D-1, owner ruling) | Done | `internal/component/iface/yang/ze-iface-conf.yang` | Untouched. No `case` removed |
| The size change is measured, not assumed | Done | R-1 in Risks & Assumptions | `vmlinuz` 16352256 -> 16450560 bytes, +98304 (+0.60%), on the rebuilt amd64 kernel. No target in `mk/` or `gokrazy/kernel/` bounds that size |
| A ze RESTART over its own leftover tunnel netdevs applies | Done | `internal/component/iface/config_apply.go`, `applyConfig` tunnel create step | Found while proving the above (Known Limitations). Keeps a tunnel of the configured kind, fails closed on any other device |
| Every kernel link type Ze creates is one Ze can also delete | Done | `internal/component/iface/discover.go`, `kernelTunnelKinds` | Round 2. `"vxlan"` was in `kernelLinkTypes` and not here, so `zeManageable` answered false and the Phase 4 prune skipped it |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 every modelled kind is buildable | Done | `test/plugin/iface-tunnel-kinds.ci`, PASSED in QEMU on the rebuilt kernel, 5.8s, all nine names in `show interface` | The assertion reads back by name, so a kind whose symbol is absent produces no link and is named in the failure |
| AC-2 the guard exists | Done | `gokrazy/kernel/runtime.require` plus `TestTunnelKindsAllHaveKernelSupport` (`internal/component/iface/tunnel_test.go`) | The Go half fails when a kind reaches `tunnelKindNames` with no symbol behind it; the manifest half fails the kernel build |
| AC-3 proven, not asserted | Done | the same QEMU run as AC-1 | The kernel that ran it is the one the fragment describes: the built `config` beside `vmlinuz` was read back, per Implementation Step 4 |
| AC-4 size is known | Done | R-1 | +98304 bytes measured, not estimated |
| AC-5 `test-tx-iface-tunnel-remove` passes | Done | reload 37, 38, 39 PASSED in QEMU once this spec's `.ci` took its own endpoint pairs | No product change was needed for them; `TestCITunnelEndpointsAreUniqueAcrossTests` now enforces the constraint |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestTunnelKindsAllHaveKernelSupport` | Done | `internal/component/iface/tunnel_test.go` | passes |
| `TestApplyTunnelKeepsExistingNetdevWhenPreviousSpecIsLost` | Done | `internal/component/iface/config_apply_test.go` | passes; red without the fix with `[tunnel tgre0 create: file exists]` |
| `TestApplyTunnelChangedSpecStillReachesTheKernel` | Done | `internal/component/iface/config_apply_test.go` | passes |
| `TestApplyTunnelRollbackDoesNotDeleteAKeptNetdev` | Done | `internal/component/iface/config_apply_test.go` | passes; red when the kept branch sets `created = true` |
| `TestApplyTunnelFailsWhenAnotherDeviceKindHoldsTheName` | Done | `internal/component/iface/config_apply_test.go` | passes; red with the kind check removed |
| `TestApplyTunnelFailsWhenTheNameHoldsAnotherTunnelKind` | Done | `internal/component/iface/config_apply_test.go` | passes; red with the kind check removed |
| `TestApplyTunnelReportsTheCreateErrorWhenTheNameIsFree` | Done | `internal/component/iface/config_apply_test.go` | passes; red with the read-back error path removed |
| `TestEveryTunnelKindHasAKernelLinkType` | Done | `internal/component/iface/config_apply_test.go` | passes |
| `TestCITunnelEndpointsAreUniqueAcrossTests` | Done | `internal/test/runner/tunnel_endpoint_lint_test.go` | passes; the corpus now includes `iface-tunnel-restart.ci` |
| `iface-tunnel-kinds` (functional) | Done | `test/plugin/iface-tunnel-kinds.ci` | PASSED in QEMU, 5.8s |
| `TestKernelTunnelKindsCoversEveryModeledKind` (added round 2) | Done | `internal/component/iface/discover_test.go` | red with `"vxlan"` removed: `tunnel kind vxlan produces a "vxlan" link and kernelTunnelKinds (discover.go) does not list it` |
| `TestInfoToZeType/vxlan` (added round 2) | Done | `internal/component/iface/discover_test.go` | red with `"vxlan"` removed: `= "ethernet", want "tunnel"` |
| `TestCITunnelEndpointLintReadsAClaimWithNoLocalAddress` (added round 2) | Done | `internal/test/runner/tunnel_endpoint_lint_test.go` | red with the drop restored: `tunnelEndpointClaims on an interface-sourced stanza = [], want one claim` |
| `TestCITunnelEndpointLintCountsInterfacesNotStanzas` (added round 2) | Done | `internal/test/runner/tunnel_endpoint_lint_test.go` | red with stanza counting restored, and the corpus check goes red with it |
| `iface-tunnel-restart` (added round 2, functional) | Written, NOT RUN | `test/plugin/iface-tunnel-restart.ci` | needs QEMU, which this session cannot run. See Goal Validation |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `gokrazy/kernel/runtime.config` | Done | modified |
| `gokrazy/kernel/runtime.require` | Done | modified |
| `internal/component/iface/config_apply.go` | Done | modified; 1411 lines, see Known Limitations |
| `internal/component/iface/tunnel.go` | Done | modified: `kernelLinkTypes` added, dead `IsGREFamily` removed |
| `internal/component/iface/config_test.go` | Done | modified: `fakeBackend.CreateTunnel` answers `file exists` and reports a link type |
| `internal/component/iface/config_apply_test.go` | Done | modified |
| `docs/features/interfaces.md` | Done | modified: the third reload case, and a source anchor on the new `.ci` |
| `test/plugin/iface-tunnel-kinds.ci` | Done | created, untracked |
| `internal/component/iface/tunnel_test.go` | Done | created, untracked |
| `internal/test/runner/tunnel_endpoint_lint_test.go` | Done | created, untracked |
| `internal/component/iface/discover.go` (not in plan) | Changed | round 2: `"vxlan"` added to `kernelTunnelKinds`, dead `TunnelKindNames` removed |
| `internal/component/iface/discover_test.go` (not in plan) | Changed | round 2: the pair test and the vxlan case |
| `test/plugin/iface-tunnel-restart.ci` (not in plan) | Changed | round 2: the restart functional test |

### Audit Summary

- **Total items:** 6 requirements, 5 ACs, 15 tests, 13 files
- **Done:** all but one
- **Partial:** none
- **Skipped:** none
- **Changed:** 4 files the plan did not name, all round-2 review fixes; `iface-tunnel-restart.ci` is written and unrun

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Every modelled kind is buildable: an operator who configures one gets it | functional | `test/plugin/iface-tunnel-kinds.ci` PASSED in QEMU on the rebuilt kernel, 5.8s. All nine names read back from `show interface`. Reverting one symbol in `runtime.config` removes every name that symbol carries, so the test discriminates per symbol |
| The guard exists, so a kernel bump cannot silently drop one | build gate | The BUILT kernel's own `config` beside `vmlinuz` carries all seven as `=y`, read back after the rebuild rather than taken from the fragment. `gokrazy/kernel/runtime.require` pins the same seven, and `enforce_required_symbols` (`tools/kernel-builder/build.py`) accepts `=y` alone, so a `=m` answer fails the build |
| The image stays inside its bound | measurement | `vmlinuz` 16352256 -> 16450560 bytes, +98304 (+0.60%). No target in `mk/` or `gokrazy/kernel/` bounds the size, so there is no bound to break |
| A ze RESTART over its own leftover tunnel netdevs applies cleanly and keeps the device | **NOT PROVEN** | `test/plugin/iface-tunnel-restart.ci` is written and has NOT been run: it needs QEMU and this session could not run one. No unit test is offered in its place: every unit test of this branch drives `fakeBackend` (`internal/component/iface/config_test.go`), whose EEXIST answer and link-type read-back were hand-written in the same round as the code they exercise, so they prove the branch reacts to what the fake says and not what a kernel says. The goal has no functional evidence until that `.ci` runs |
| A tunnel kind Ze can create is a kind Ze can delete | unit | `TestKernelTunnelKindsCoversEveryModeledKind` (`internal/component/iface/discover_test.go`) fails on a kernel link type in one of the two maps and not the other. The `.ci` half is not owed here: the prune runs on the same `kernelTunnelKinds` lookup the test pins, and the QEMU proof of the create side is AC-1 |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/runtime-kernel-carries-every-tunnel-kind-640fa955-f03a-45e8-a58f-4b367f5859e6.md`, `verdict=clean rounds=3` |
| `review_gate.py check` | `review_gate: OK (clean, hashes match)` |
| Rounds | 3. Round 1 covered the whole diff and returned 2 BLOCKERs. Round 2 returned 4 ISSUEs and 3 NOTEs. Round 3 is the independent pass over the round-2 fixes, and it is clean |
| Reviewer lenses used | logic+wiring (round 1); guard audit, test-gate lens (round 2); producer verification, guard audit, test discrimination (round 3). Every round on Opus 5, and none of them the context that wrote the code |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| R1-1 | BLOCKER | the tunnel create step kept ANY device holding the name, so a dummy or a physical NIC became the carrier of the tunnel's MTU, admin state and addresses | `internal/component/iface/config_apply.go`, `applyConfig` | the step reads back the device and compares `InterfaceInfo.Type` against `kernelLinkType()`; anything else fails the apply. `TestApplyTunnelFailsWhenAnotherDeviceKindHoldsTheName`, `TestApplyTunnelFailsWhenTheNameHoldsAnotherTunnelKind` |
| R1-2 | BLOCKER | a false measured claim: `tgrerm0` was recorded as a netdev an earlier RUN had left, when the cause was a collision with this spec's own `.ci` | the journal row, Known Limitations, and the test's doc comment | all three corrected to name the test collision, and the surviving product fix re-justified against the restart path |
| R2-1 | ISSUE | Ze can create a vxlan and cannot delete one: `kernelTunnelKinds` omitted `"vxlan"`, so `zeManageable` answered false and the Phase 4 prune skipped it | `internal/component/iface/discover.go` | `"vxlan": true` added, and the two maps pinned to each other by `TestKernelTunnelKindsCoversEveryModeledKind`. The blast radius was checked at the producer and is a correction: a host vxlan was classified `ethernet` by the MAC fallback, not dropped |
| R2-2 | ISSUE | five of six create steps still fail open on device kind | `internal/component/iface/config_apply.go` | NOT fixed here, by review judgement: separable and strictly pre-existing. Recorded in `plan/journal/guard-added-to-one-half-of-a-pair.md`, whose row now says the five remain kind-blind and names each step |
| R2-3 | ISSUE | no functional test drives the restart path | `test/plugin/` | `test/plugin/iface-tunnel-restart.ci`: ze runs twice over one tunnel config in one VM, and the kernel ifindex is carried from the first run to the second and compared. Written, NOT RUN (see Goal Validation) |
| R2-4 | ISSUE | the endpoint lint dropped any claim with no local address, so an interface-sourced tunnel escaped the collision check | `internal/test/runner/tunnel_endpoint_lint_test.go` | the claim is recorded with an empty local; only a missing `remote/ip`, which YANG makes mandatory, still means the scanner failed. `TestCITunnelEndpointLintReadsAClaimWithNoLocalAddress` |
| R2-N1 | NOTE | the lint reported a collision between two stanzas in ONE file | `internal/test/runner/tunnel_endpoint_lint_test.go` | fixed, because it is wrong: a holder is now one interface NAME in one file. One name in two configs is the reload and restart shape and the kernel holds one device for it; two different names on one claim in one file is still reported. The new `.ci` makes it live, not latent |
| R2-N2 | NOTE | `IsGREFamily` has no caller | `internal/component/iface/tunnel.go` | removed, with `greFamilyKinds`. Nothing to wire it to: the `key` leaf it would guard lives inside `container gre` in the YANG. `TunnelKindNames` (`discover.go`) turned out to be dead the same way and went with it |
| R2-N3 | NOTE | GRE kind identity is derived from the local address family in the vendored netlink | `vendor/github.com/vishvananda/netlink` | no action, per the reviewer: the round trip is correct for every shape the YANG can produce |

### Run 3 (independent, over the round-2 fixes)

0 BLOCKER, 0 ISSUE, 0 NOTE. It read every round-2 fix at its producing function
and found nothing new in the product. It DID find one record defect and one lint
finding, both fixed in one edit each and neither earning a further round
(`ai/rules/planning.md`): round 2's Verification Evidence had missed a `goconst`
finding its own `fakeBackend.CreateTunnel` line created, fixed by naming the
literal `fakeUnknownLinkType` (`internal/component/iface/config_test.go`).

## Implementation Summary

### What Was Implemented
- `gokrazy/kernel/runtime.config`: seven `=y` symbols, `NET_IPGRE_DEMUX`,
  `NET_IPGRE`, `IPV6_GRE`, `NET_IPIP`, `IPV6_TUNNEL`, `IPV6_SIT`, `VXLAN`. Nine
  modelled kinds through six symbols plus the GRE demux gate.
- `gokrazy/kernel/runtime.require`: the same seven pinned, so a Kconfig
  `default m` answer fails the build rather than shipping a kind nobody can
  create. `enforce_required_symbols` (`tools/kernel-builder/build.py`) accepts
  `=y` alone.
- `internal/component/iface/config_apply.go`: the tunnel create step now answers
  EEXIST the way its five siblings do, and one way they do not. It reads the
  device back and keeps it only when `InterfaceInfo.Type` matches
  `kernelLinkType()` for the configured kind. Any other device fails the apply.
- `internal/component/iface/tunnel.go`: `kernelLinkTypes`, the kind-to-link-type
  map the read-back compares against. Dead `IsGREFamily` and `greFamilyKinds`
  removed.
- `internal/component/iface/discover.go`: `"vxlan"` added to `kernelTunnelKinds`.
  Ze could CREATE a vxlan and could not DELETE one, and `infoToZeType` called it
  an ethernet port through the MAC fallback. Dead `TunnelKindNames` removed.
- `internal/test/runner/tunnel_endpoint_lint_test.go` (new):
  `TestCITunnelEndpointsAreUniqueAcrossTests` over the whole `.ci` corpus.
- `docs/features/interfaces.md`: the third reload case, with a source anchor.
- Tests: `internal/component/iface/tunnel_test.go` (new), five new cases in
  `config_apply_test.go`, two in `discover_test.go`, two in the lint test, and
  two new `.ci` (`test/plugin/iface-tunnel-kinds.ci`,
  `test/plugin/iface-tunnel-restart.ci`).

### Bugs Found/Fixed
- **A ze RESTART over its own leftover tunnel netdevs refused to start.**
  `netlink.LinkAdd` sends `NLM_F_CREATE|NLM_F_EXCL`, nothing in
  `internal/component/iface` deletes tunnels when the daemon stops, and
  `OnConfigure` runs `applyConfig(cfg, nil, b)` at every plugin start. The five
  sibling create steps survived that state; the tunnel step returned the error
  and the fail-closed startup cascade then refused the daemon.
- **The first version of that fix had the defect this journal class collects.**
  It kept ANY device holding the name, so a dummy, a bridge or a physical NIC
  named `tgre0` became the carrier of the tunnel's MTU, admin state and
  addresses. Found by review round 1.
- **Ze could create a vxlan and not delete one.** `kernelLinkTypes` mapped
  `TunnelKindVxlan` to `"vxlan"` and `kernelTunnelKinds`, the same mapping the
  other way, never listed it. Found by review round 2.
- **The endpoint lint dropped any claim with no local address**, so an
  interface-sourced tunnel escaped the collision check it exists to make. Found
  by review round 2.

### Documentation Updates
- `docs/features/interfaces.md`: the third reload case (a restart over leftover
  tunnel netdevs), with a source anchor on the new `.ci`.
- `make ze-doc-test` FAILS on `ai/DOCS-TO-CODE.md is stale`, which is another
  session's closure residue. The measurement is in Verification Evidence, and
  nothing in this spec's changed set is named by that check.

### Deviations from Plan
- Four files the plan did not name are in the diff, all round-2 review fixes:
  `discover.go`, `discover_test.go`, `test/plugin/iface-tunnel-restart.ci`, and
  the lint test's two new cases.
- The plan expected a reload-idempotency defect. There was none: the QEMU failure
  that suggested it was a collision between two `.ci` over one endpoint pair. The
  product change is kept because it fixes a different and real defect, the ze
  restart, and Known Limitations records the correction.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | A QEMU failure on `tgretap` and `tgrerm0` was read as a reload-idempotency defect, and a product change was written for it | Two `.ci` shared one local/remote endpoint pair. The kernel refuses a second tunnel on a pair the driver already holds, whatever the device is named, and this spec's `.ci` runs first in the shared VM and leaves its links behind. Moving the endpoints made all three reload tests pass with NO product change | review round 1, which asked what produced the EEXIST | the measurement corrected in three places, the surviving fix re-justified against the restart path, and `TestCITunnelEndpointsAreUniqueAcrossTests` added so the collision cannot be re-created silently |
| approach | The first restart fix kept any device holding the name | A dummy, a bridge or a physical NIC then carried the tunnel's MTU, admin state and addresses. `reconcileOwnedDevices`, in the same file, already failed closed on that state for macvlans | review round 1 | the step reads the link type back and keeps the device only when it is a tunnel of the configured kind |
| escalation | A unit test of the restart branch was offered as evidence for the restart goal | Every unit test of that branch drives `fakeBackend`, whose EEXIST answer and link-type read-back were hand-written in the same round as the code they exercise. It proves the branch reacts to what the FAKE says, not to what a kernel says | review round 2, which asked for a functional test | `test/plugin/iface-tunnel-restart.ci` written, and the goal recorded as NOT PROVEN until it runs |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| the Deferral shard field is `-`; no shard was ever created | done | nothing was deferred through the shard machinery |
| R2-2: five of the six create steps still fail open on device kind | homed in the journal, not fixed | `plan/journal/guard-added-to-one-half-of-a-pair.md`, whose row now names each step (`CreateDummy`, `CreateVeth`, `CreateWireguardDevice`, `CreateXFRM`, `CreateBridge`, and `CreateVLAN` beside them). Review round 2 judged it separable and strictly pre-existing: no step this spec touched has it, and six edits do not belong in a closing commit (`ai/rules/rule-precedence.md`). It wants its own spec |
| A kept link's PARAMETERS are never compared, so a `local`, `remote`, `key` or `ttl` edit made while ze was down reaches the link's addresses and MTU and not its encapsulation | homed in the journal | the same row. It needs a tunnel read-back the `Backend` interface does not have. Shared with the xfrm step beside it |
| `ze config validate` warns `mandatory field "kind" is missing` once per tunnel in a config it declares valid | recorded | `plan/journal/gate-excludes-part-of-its-population.md`, committed with `spec-fixit-config-validators-bypassed-at-startup`. It does not block: validate exits 0 and the daemon applies the config. The producing function was not read, so the row says so |
| 25 `.ci` cite `ai/rules/qemu-testing.md`, a file `ad809ea43` deleted | recorded | `plan/journal/closure-deletes-a-cited-document.md`. 25 files of test comments, none of which this spec touches, and the fix is one substitution the whole set wants at once |
| `config_apply.go` is 1411 lines against a 1000-line threshold | not this spec's | it was 1358 before this work, so the file did not cross the threshold here. The split is separate work |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/iface/tunnel_test.go` | Yes | on disk, new in this commit |
| `internal/test/runner/tunnel_endpoint_lint_test.go` | Yes | on disk, new in this commit |
| `test/plugin/iface-tunnel-kinds.ci` | Yes | on disk, new in this commit |
| `test/plugin/iface-tunnel-restart.ci` | Yes | on disk, new in this commit, and UNRUN |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | every modelled kind is buildable | `test/plugin/iface-tunnel-kinds.ci` PASSED in QEMU on the rebuilt kernel, 5.8s, all nine names read back from `show interface` |
| AC-2 | the guard exists in both halves | `make ze-test-pkg PKG=./internal/component/iface` ok, carrying `TestTunnelKindsAllHaveKernelSupport`; `gokrazy/kernel/runtime.require` pins the same seven symbols `runtime.config` sets |
| AC-3 | proven, not asserted | the BUILT kernel's own `config` beside `vmlinuz` was read back after the rebuild, rather than taken from the fragment |
| AC-4 | the size change is measured | `vmlinuz` 16352256 -> 16450560 bytes, +98304 (+0.60%) |
| AC-5 | the reload tests pass | reload 37, 38 and 39 PASSED in QEMU once this spec's `.ci` took its own endpoint pairs, with no product change |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| an operator configures each tunnel kind and reads it back | `test/plugin/iface-tunnel-kinds.ci` | Read: it configures all nine kinds and asserts each name in `show interface`. PASSED in QEMU, 5.8s |
| ze restarts over its own leftover tunnel netdevs | `test/plugin/iface-tunnel-restart.ci` | Read: ze runs twice over one tunnel config in one VM, and the kernel ifindex is carried from the first run to the second and compared, so a delete-and-recreate fails it. **NOT RUN**: it needs a VM this session could not start |
| a tunnel kind Ze creates is one Ze can delete | none | unit only. `TestKernelTunnelKindsCoversEveryModeledKind` (`internal/component/iface/discover_test.go`) fails on a link type in one map and not the other, and the prune reads the same lookup |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| R-1 | measured, retired | the image grew 98304 bytes (+0.60%), and no target in `mk/` or `gokrazy/kernel/` bounds that size |
| D-1 (owner ruling) | confirmed | `internal/component/iface/yang/ze-iface-conf.yang` is untouched: no `case` was removed, which is what the ruling required |
| the `sit` `=m` question | retired | it was Kconfig's `default m` answering, not a request. It is `=y` with the other six in the rebuilt kernel |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/features/interfaces.md`'s third reload case | the branch it describes is the tunnel create step's read-back in `applyConfig` (`internal/component/iface/config_apply.go`), and the anchor names the `.ci` that drives it | Yes |
| no other doc states the tunnel create step's EEXIST behaviour | `grep -rn "component/iface" docs/` names anchors none of which describes that branch | Yes |
| `make ze-doc-test` | FAILS on `ai/DOCS-TO-CODE.md is stale`, another session's residue. Nothing in this spec's changed set is named by that check | Yes, attributed |

## Core Insight

A guard added to one member of a set of siblings is the shape to look for, and
this spec produced it twice in one file. The tunnel create step lacked the
keep-the-link branch its five siblings had; the branch was added, and it then
lacked the kind check that `reconcileOwnedDevices`, thirty lines away, already
performed for macvlans. Both were found by asking what the OTHER members of the
set do, not by reading the changed lines.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
