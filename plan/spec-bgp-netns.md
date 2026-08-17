# spec-bgp-netns

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - (not blocked by any spec; this spec IS the fix for the shipped default, see "Why this is the default case" below) |
| Phase | 0/N (research) |
| Updated | 2026-07-16 |

Anchor refresh (2026-07-22 plan review, design unchanged): `reactor.go`
anchors shifted ~+18 lines; citations below updated in-body --
`newListenerFactory` `:1394-1422`, the MD5/GTSM
`RealListenerFactory{MD5Peers, ListenTTL}` branch `:1402`. `go.mod:23`
(`vishvananda/netns v0.0.5`) and `ze-vpp-conf.yang/201` verified exact.

**DESIGN, APPROVED IN PRINCIPLE, NOT `ready`.** Created 2026-07-16 by splitting
`plan/spec-fixit-vpp-lcp-reachability.md` (Thomas approved the split; see that spec's Task ->
"The 2026-07-16 split"). This spec is the **Problem A** half: netns-aware BGP listening. The
parent keeps **Problem B** (the LCP-presence doctor check) and ships on its own.

→ Decision (user, 2026-07-16): **A-7 is ANSWERED: yes, support a non-root netns, because it is
the DEFAULT and the documented model.** The `vpp.lcp.netns` default of `"dataplane"` **STAYS**
(see "Why this is the default case" below), so this spec is what makes the default config work.
It is NOT `blocked`. It is also NOT `ready`: promotion is Thomas's gate and has not been given.
The config-surface shape (Q3) is still open, and A-3 is an unvalidated kernel-semantics claim
that gates the design.

→ Decision (user, 2026-07-16): **PRIORITY RAISED.** ~~"Build it last; no isolation-using
operator is evidenced."~~ SUPERSEDED. That framing assumed the default would be changed to a
root-reachable namespace, which Thomas has rejected. See "Why this is the default case".

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`, `ai/rules/planning.md` (spec metadata, status vocabulary)
3. `ai/rules/evidence.md`, `ai/rules/architecture.md`, `ai/rules/platform-linux.md`
4. `internal/component/bgp/reactor/reactor.go` (`newListenerFactory`, lines 1394-1422),
   `internal/core/network/network.go` (`RealListenerFactory`, lines 147-194)
5. `plan/spec-fixit-vpp-lcp-reachability.md` (the doctor half; shares the AC-3 seam)

## Task

VPP's Linux Control Plane (LCP) creates TAP mirrors of VPP interfaces so routing daemons can
use Linux TCP on VPP-managed NICs. The TAPs land in the namespace named by `vpp.lcp.netns`,
but ze's BGP listener has zero network-namespace awareness, so it cannot bind on them unless
the TAP is in a root-reachable namespace. Goal: an operator who deliberately isolates an LCP
TAP in its own namespace can still have BGP peer over it, by telling BGP which namespace to
bind in.

Scope, as proposed and approved 2026-07-16:

| In scope | Out of scope |
|----------|--------------|
| `RealListenerFactory.Netns` as a FIELD (not a rival factory type) + `netns_linux.go` / `netns_other.go` | Changing the `vpp.lcp.netns` DEFAULT. -> Decision (user, 2026-07-16): the default STAYS `"dataplane"`; it is not a bug and no spec owns changing it |
| The `newListenerFactory` change threading netns through BOTH branches (`reactor.go`) | The LCP plugin-presence doctor check (owned by the parent spec) |
| A BGP netns config surface (generic, no VPP spelling) | The outbound dialer (`RealDialer`), web / lg / DNS listeners (Known Limitations; Q4/Q5) |
| The A-3 kernel-semantics prototype | |
| The QEMU rail | |
| AC-3: narrowing `checkVPPLCPNetns` once BGP can follow | |

### Why this is the default case (READ THIS FIRST)

→ Constraint: this section previously described a "Relationship to
`plan/spec-fixit-vpp-lcp-netns-default.md`". **That spec was never created and must not be.**
-> Decision (user, 2026-07-16): the `vpp.lcp.netns` default of `"dataplane"` **STAYS**. It is
not a bug. Every claim below that the default was a mistake is SUPERSEDED and struck through,
not deleted, so the error stays legible.

→ Decision (user, 2026-07-16): the A-7 question was **reframed** before it was answered, and
the reframing is the load-bearing part. A-7 was originally framed as "is a non-root LCP netns
worth supporting at all?", which implies a niche opt-in capability. **The code says the
opposite: the unreachable case is the DEFAULT.** Verified at the producers 2026-07-16:

| Fact | Producer (verified) |
|------|--------------------|
| `vpp.lcp.netns` defaults to `"dataplane"` | `internal/component/vpp/yang/ze-vpp-conf.yang` (`leaf netns`, `type string`, `default "dataplane"`) |
| `"dataplane"` is NOT root-reachable | `lcpNetnsIsRootReachable` (`internal/plugins/iface/vpp/doctor.go`) returns true ONLY for `""`, `"host"`, `"root"` |
| So VPP puts the TAP in `dataplane` | `lcpPairNetns` (`internal/plugins/iface/vpp/lcp.go`) maps root-reachable names to `""` and passes any other name through unchanged |
| BGP cannot bind there | `RealListenerFactory.Listen` (`internal/core/network/network.go`) binds via `net.ListenConfig` in whatever namespace the calling thread is in; nothing in the reactor references a network namespace |

→ Constraint: the shipped DEFAULT config puts LCP TAPs where BGP cannot bind. That fact is
correct and re-verified above. What was inferred FROM it was not:

~~The comment directly above `lcpPairNetns` (`lcp.go`) states the intent that the YANG
default contradicts: a root-reachable name means the TAP goes "where ze's BGP listener runs",
and any other name isolates the TAP "deliberately". The default is not deliberate isolation; it
is every operator getting isolation they did not ask for. **The YANG default contradicts its own
design intent.**~~

**SUPERSEDED, 2026-07-16. The claim was FALSE and its method was the error.** `lcp.go`
is a code comment: it records what its author believed about the seam they were writing, not
what the project decided. The actual recorded design intent says the opposite, and it was
already in the tree:

| Evidence | Source (verified 2026-07-16) |
|----------|------------------------------|
| The recorded decision: make BGP netns-aware "so LCP TAPs in a non-root netns (`vpp.lcp.netns`) are reachable by BGP **without forcing the operator to a root-reachable netns**" | `plan/deferrals.md`, row dated 2026-07-10, source `spec-followup-vpp-iface` A-4. This is the origin row of THIS spec |
| `"dataplane"` is IPng Networks' production convention, copied deliberately | `54bffb83b` ("startup.conf generator **following IPng production template**"); `docs/research/vpp-deployment-reference.md`: all LCP TAPs are created in the `dataplane` netns, which "isolates the forwarding plane from the management plane" |
| The YANG leaf describes the model, it does not concede a gap | `ze-vpp-conf.yang`: "Network namespace for LCP TAP interfaces. **Routing daemons run in this namespace.**" |

→ Decision (user, 2026-07-16): **the default STAYS `"dataplane"`.** It is not a mistake and no
spec owns changing it. The isolation model is the whole point of the IPng design ze copied.
The gap is not that the TAPs are isolated; the gap is that BGP cannot follow them there.

→ Decision (user, 2026-07-16), A-7 answered: **yes, support a non-root netns, because it is the
DEFAULT and the documented model.** ~~"Both: fix the default now, keep netns as real work."~~
SUPERSEDED: that answer was given on the false premise that the default was a bug.

→ Constraint (PRIORITY, state it plainly): **PRIORITY RAISED. This spec is what makes the
DEFAULT config work.** ~~Once the default is root-reachable, this work is no longer urgent; it
serves only deliberate isolators, a population nothing in the tree evidences.~~ SUPERSEDED, and
the conclusion INVERTS with the premise. With `"dataplane"` staying:

- Netns-aware BGP is not a niche capability. It is the missing half of the shipped default.
- Today an operator running default LCP must override `vpp.lcp.netns` to `host`/`root` to run
  BGP at all (documented at `docs/guide/vpp.md`, the `vpp.lcp.netns` leaf row), which means
  abandoning the forwarding/management isolation that is the entire point of the IPng model.
  The workaround costs the operator the design.
- So this spec serves **every default LCP install**, not a hypothetical isolator. That is a
  much stronger case than the one this spec previously made against itself.

## Origin

`plan/deferrals.md`, row dated 2026-07-10, source `spec-followup-vpp-iface` A-4 (BGP
netns-aware listener). Destination recorded there as "none yet (future `spec-bgp-netns` when
picked up)". This file is that destination. Routed through
`plan/spec-fixit-vpp-lcp-reachability.md` (research 2026-07-15/16), which split on
2026-07-16.

→ Constraint: **NAMING COLLISION, carried over.** "A-4" means two different things. In
the followup-vpp-iface record and in the source comments `doctor.go` /
`lcp.go` ("see A-4"), it is the ORIGIN spec's A-4 = the BGP-netns deferral = THIS spec. In
the parent spec's Assumptions table, A-4 = "a missing plugin is detectable" = the doctor half.
Unrelated. Those source comments point at a retired spec, and the record that replaced it
was retired with the learned corpus, so they need a live design target
(`ai/rules/planning.md`, "Design references survive closure"). Not actioned; raise with Thomas.

## Required Reading

### Source (read before designing)

<!-- Moved from plan/spec-fixit-vpp-lcp-reachability.md, not duplicated: the parent's copies
     are removed and replaced by a pointer to this file. -->

- [ ] `internal/component/bgp/reactor/reactor.go` - lines 1394-1422: `newListenerFactory` is
      the PRODUCER of every production listener factory
  → Decision: THE KEYSTONE FINDING, re-verified at the producer 2026-07-16 by this session,
    not taken on trust from the parent spec. When `md5PeersForListener(port)` is non-empty or
    `listenTTLForListener(port)` is non-zero, `newListenerFactory` returns a FRESH
    `network.RealListenerFactory{MD5Peers: md5Peers, ListenTTL: listenTTL}` and DISCARDS
    `r.listenerFactory` entirely. Only the no-MD5/no-GTSM branch returns the injected factory.
    There is no third branch and no composition. The function's own doc comment (lines
    1394-1397) says so: "Returns the configured RealListenerFactory, or the reactor's injected
    factory (chaos/test) when neither applies."
  → Constraint: netns must COMPOSE with MD5 and GTSM, not compete with them. The factory is
    chosen either/or, never stacked, so a separate wrapping `ListenerFactory` implementation
    cannot compose here. This drives the "field, not wrapper" decision and mandates a
    netns+MD5 test (AC-9, R-8).
- [ ] `internal/component/bgp/reactor/reactor.go` - lines 1019-1024 (global listener) and
      1399-1402 (per-address/port listener)
  → Constraint: verified 2026-07-16. BOTH call `NewListener(...)` then immediately
    `SetListenerFactory(r.newListenerFactory(port))`, overwriting the default set by
    `NewListener`. So `Listener.SetListenerFactory` (`listener.go`) is an INTERNAL seam
    only; no production caller outside the reactor can inject a factory into a `Listener`. The
    only external seam is `Reactor.SetListenerFactory` (`reactor.go`), which sets
    `r.listenerFactory`, which is exactly what the MD5/GTSM branch discards.
- [ ] `internal/core/network/network.go` - lines 147-162: `RealListenerFactory` carries
      `MD5Peers` and `ListenTTL`, two per-bind socket-level concerns, as FIELDS
  → Decision: netns is the same KIND of thing (a property of how the listening socket is
    created) and belongs as a third field `Netns string`, not as a rival factory type. This is
    the only shape that composes with MD5/GTSM through `newListenerFactory`.
- [ ] `internal/core/network/network.go` - lines 164-194: `Listen` builds a `net.ListenConfig`
      and applies MD5/GTSM through `lc.Control`, a callback the kernel runs on the
      already-created fd BEFORE bind
  → Constraint: THE ONE GENUINE ASYMMETRY. Netns CANNOT use `Control`: the socket must be
    CREATED inside the namespace, so the helper must wrap the whole `lc.Listen` call (lock
    thread, setns, listen, restore, unlock) rather than plug into `Control`. Same field shape,
    different insertion point.
- [ ] `internal/core/network/` - the package already splits platform-specific socket work by
      build tag: `md5_linux.go` / `md5_freebsd.go` / `md5_darwin.go` / `md5_other.go`, and
      `ttl_linux.go` / `ttl_other.go`, each serving a `RealListenerFactory` field
  → Constraint: `netns_linux.go` + `netns_other.go` serving a `Netns` field is the EXACT
    established pattern of this package, not a new idea. Placement in `internal/core/network`
    is precedented, not invented.
- [ ] `internal/test/runner/netns_linux.go` - the in-tree netns precedent: `enterTestNetns`
      uses `runtime.LockOSThread()`, `netns.Get()`, `netns.NewNamed()`, and a `restore()`
      closure that must run on the same thread; `netns_other.go` is the non-linux stub
  → Decision: ze already has a netns idiom and `github.com/vishvananda/netns v0.0.5` is
    already a DIRECT dependency (`go.mod:23`). The netns leg needs no new dependency and no
    new idiom; it reuses this lock/get/set/restore shape.
  → Constraint: the file header records that the locked thread's namespace is inherited by
    fork+exec'd children (validated by `TestNetnsLaunchChildInheritsNamespace`). That is
    evidence that namespace membership attaches at object-creation time, which supports (but
    does NOT prove) "pin only around bind". See A-3.
- [ ] `internal/core/routewatch/routewatch_linux.go` - lines 12-38: a second in-tree consumer
      of `github.com/vishvananda/netns`, holding an `netns.NsHandle` in `platformState`
  → Constraint: precedent that `internal/core/` already imports this library directly, so the
    core import-direction rule (`ai/rules/architecture.md`) is not at issue.
- [ ] `internal/plugins/iface/vpp/doctor.go` - lines 100-143: `checkVPPLCPNetns` (code
      `doctor-vpp-lcp-netns`, `SeverityWarning` at line 128) and `lcpNetnsIsRootReachable`
  → Constraint: this check is the "meanwhile" mitigation. If this spec lands, its premise
    changes and it must be revisited, not left. That is AC-3, and it is the ONE seam shared
    with the parent spec.
- [ ] `internal/plugins/iface/vpp/lcp.go` - lines 105-114: `lcpPairNetns` maps a
      root-reachable name to "" ~~(VPP's host namespace)~~ and passes any other name through
  → Constraint (A-13, 2026-07-16): **"" is NOT VPP's host namespace.** VPP resolves an empty
    per-pair netns to its GLOBAL default (`lcp_itf_pair_create`, `lcp_interface.c:856-861`:
    `if (ns == 0 || ns[0] == 0) ns = lcp_get_default_ns ();`), which ze sets from the SAME leaf
    (`startupconf.go`, `b.kv("default netns", s.LCP.Netns)`). The comment at `lcp.go`
    is FALSE, not merely a belief. Read A-13 before designing against this function.
  → ~~Decision: the existing product already bends toward "put the TAP where BGP can reach it".
    This spec is the other half of that same decision: let BGP follow when the operator
    deliberately chooses otherwise.~~ **SUPERSEDED by A-13.** The product does not bend that
    way: the `host`/`root` mapping does not deliver a host-namespace TAP. Netns-aware BGP is
    the ONLY working path, which strengthens this spec rather than weakening it.
- [ ] `internal/component/vpp/yang/ze-vpp-conf.yang` - lines 199-201: `leaf netns`,
      `default "dataplane"`
  → Constraint: NOT this spec's leaf to change, and NOT because another spec owns it.
    -> Decision (user, 2026-07-16): the default STAYS. Read it to understand why this spec is
    the DEFAULT case: the leaf's own description ("Routing daemons run in this namespace",
    lines 202-205) is the model this spec makes true for BGP.
- [ ] `mk/test-integration.mk` - line 319: `ZE_QEMU_INTEGRATION_PKGS` is computed by
      `grep -rl --include='*.go' '^//go:build integration && linux' internal/ cmd/`
  → Decision: NEW FINDING, verified 2026-07-16. The QEMU integration package list is
    AUTO-DISCOVERED from the build tag, not hand-maintained. A new
    `internal/core/network/netns_integration_linux_test.go` tagged `integration && linux` is
    picked up by `make ze-qemu-integration-test` with NO Makefile edit. This partially answers
    Q12 (see Open Questions): the rail for the netns BIND exists today. Note
    `ai/rules/platform-linux.md` step 5 ("Register the package in the Makefile", showing an
    explicit `--run` list) is stale for this target; the explicit list is now only
    `./internal/plugins/firewall/vpp/...` on line 325.

### Architecture Docs

- [ ] `ai/rules/platform-linux.md` - linux-only code needs QEMU integration tests
  → Constraint: MANDATORY for this spec; "needs hardware" is not an accepted skip. Two rails
    apply: `//go:build integration && linux` unit tests (auto-discovered, above), and a `.ci`
    that boots a daemon exercising a kernel feature MUST carry `option=needs-linux` so it
    SKIPs on darwin and runs for real under `make ze-qemu-needs-linux-test` /
    `make ze-qemu-test-all`.
- [ ] `ai/rules/architecture.md` - core/component/plugin placement by dependency direction
  → Constraint: `internal/core/` MUST NOT import `internal/component/` or `internal/plugins/`
    (`scripts/dev/dep_audit.py --check`, `make ze-tier-check`). The netns helper imports only
    `github.com/vishvananda/netns`, an external dep already imported from core by
    `routewatch_linux.go`. PASSES.
- [ ] `ai/rules/plugins.md` - the "delete the folder" invariant
  → Constraint: no plugin spelling in generic packages. The field is `Netns`, a Linux kernel
    concept; nothing in `internal/core/network` learns the words `vpp`, `lcp`, or `dataplane`.
    PASSES. See "ARCHITECTURAL CALL" in Design Insights for the rule-by-rule test.
- [ ] `ai/rules/config.md` (YANG vs env var) and `ai/rules/config.md`
  → Constraint: read BEFORE choosing the netns config surface shape. Q3 is open and is
    Thomas's call.
- [ ] The followup-vpp-iface record of the origin spec (retired with the learned corpus)
  → Constraint: records that real-VPP LCP proof needs a VPP image carrying the linux-cp
    plugins (`ligato/vpp-base` lacks them). That is the residual half of Q12.
- [ ] `docs/guide/vpp.md` - the operator-facing VPP guide; documents the netns constraint as a
      hard limit today
  → Constraint: must be rewritten if this spec lands.

**Key insights:**
- Netns must be a FIELD on `RealListenerFactory`, composing with `MD5Peers` and `ListenTTL`,
  not a rival factory type. The reactor's `newListenerFactory` (`reactor.go`)
  DISCARDS the injected factory whenever MD5 or GTSM applies, so a wrapper factory is silently
  dropped exactly on MD5-authenticated peers, binding in the WRONG namespace with no error.
- The reactor DOES need a (small) change. The parent spec's original assumption that no
  reactor surgery was needed is BROKEN (A-2).
- `github.com/vishvananda/netns v0.0.5` is ALREADY a direct dependency (`go.mod:23`) and
  `internal/test/runner/netns_linux.go` is an in-tree netns idiom to copy.
- The netns-bind QEMU rail EXISTS and is auto-discovering (`mk/test-integration.mk`). Only
  the full VPP+LCP end-to-end rail is missing, and it is an image-provisioning problem.
- A-3 (is pinning bind-scoped or lifetime-scoped?) is a claim about KERNEL semantics, not read
  from ze source. It is unverified and gates the design.

## Current Behavior (MANDATORY)

**Source files read:** (all re-verified at the producer by this session, 2026-07-16)

- [ ] `internal/component/bgp/reactor/reactor.go` - lines 1394-1422: `newListenerFactory(port)`
      reads `md5PeersForListener(port)` and `listenTTLForListener(port)`; if EITHER is set it
      returns a brand-new `network.RealListenerFactory{MD5Peers: md5Peers, ListenTTL: listenTTL}`,
      ignoring `r.listenerFactory`. Otherwise it returns `r.listenerFactory` (the chaos/test
      injection). No third branch, no composition.
- [ ] `internal/component/bgp/reactor/reactor.go` - lines 1019-1027: the global listener is
      built with `NewListener(r.config.ListenAddr)`, `SetClock`, then
      `SetListenerFactory(r.newListenerFactory(r.config.Port))`, then `SetHandler`. Lines
      1399-1402: `startListenerForAddressPort` does the same per address/port. Both overwrite
      whatever `NewListener` defaulted to.
- [ ] `internal/component/bgp/reactor/reactor.go` - line 553: `Reactor.SetListenerFactory(f)`
      assigns `r.listenerFactory`. Production callers: `bgp/config/loader.go`,
      `bgp/config/loader_create.go`, `bgp/cli/childmode.go`, and
      `internal/chaos/inprocess/runner.go`. Every one is defeated by the MD5/GTSM branch
      when the port carries MD5 or GTSM.
- [ ] `internal/component/bgp/reactor/listener.go` - lines 32-46: the `Listener` struct holds
      `addr string` and `listenerFactory network.ListenerFactory`, nothing namespace-related.
      Lines 50-56: `NewListener(addr)` stores the address and a `network.RealListenerFactory{}`.
      Line 108: `StartWithContext` calls `l.listenerFactory.Listen(ctx, "tcp", l.addr)`.
- [ ] `internal/core/network/network.go` - lines 147-162: `RealListenerFactory` with exactly
      two fields, `MD5Peers []MD5Peer` and `ListenTTL uint8`, each documented as a per-bind
      socket option applied "before bind". Lines 164-194: `Listen` creates `net.ListenConfig{}`,
      sets `lc.Control` only when MD5 or TTL is configured, and returns `lc.Listen(ctx, nw, address)`.
      It binds in whatever namespace the calling thread happens to be in. Nothing references a
      network namespace.
- [ ] `internal/core/network/` file listing: `md5_darwin.go`, `md5_freebsd.go`, `md5_linux.go`,
      `md5_other.go`, `ttl_linux.go`, `ttl_other.go`, `ttl.go`, `network.go`. The package's
      established shape: a portable field on the factory + a build-tagged helper per OS.
- [ ] `internal/test/runner/netns_linux.go` - `enterTestNetns(name)` locks the OS thread,
      `netns.Get()` for the original handle, `netns.NewNamed(name)` to enter, brings `lo` up,
      and returns a `restore()` closure that re-enters the original namespace and unlocks the
      thread. `netns_other.go` (`//go:build !linux`) stubs it with `errNetnsUnsupported`.
- [ ] `internal/core/routewatch/routewatch_linux.go` - lines 12-38: holds an `netns.NsHandle`
      in `platformState`, defaulting to `netns.None()`. Precedent for the core import.
- [ ] `internal/plugins/iface/vpp/doctor.go` - lines 100-131: `checkVPPLCPNetns` warns when
      `bgp` is configured, `vpp/lcp` exists, `enabled` is not "false", and the netns is not
      root-reachable. It defaults the netns to "dataplane" in code (line 116) when the leaf is
      omitted, mirroring the YANG default. Emits `doctor-vpp-lcp-netns` at `SeverityWarning`.
      Lines 136-143: `lcpNetnsIsRootReachable` returns true only for "", "host", "root".
- [ ] `internal/plugins/iface/vpp/lcp.go` - lines 105-114: `lcpPairNetns` maps a root-reachable
      name to "" and passes any other name through. Comment (lines 105-108) records the intent:
      root-reachable is "where ze's BGP listener runs"; another name isolates "deliberately".
- [ ] `internal/component/vpp/yang/ze-vpp-conf.yang` - lines 199-205: `leaf netns`,
      `type string`, `default "dataplane"`, described as "Network namespace for LCP TAP
      interfaces. Routing daemons run in this namespace."
- [ ] `mk/test-integration.mk` - lines 319-325: `ZE_QEMU_INTEGRATION_PKGS` is derived by
      grepping for `^//go:build integration && linux` under `internal/` and `cmd/`;
      `ze-qemu-integration-test` runs those packages plus an explicit
      `./internal/plugins/firewall/vpp/...`.

**Behavior to preserve:**

- BGP listener behavior for every non-LCP deployment: default binding must not change. An
  empty `Netns` must take exactly today's code path, with no thread pinning (AC-2).
- MD5 (`MD5Peers`) and GTSM (`ListenTTL`) application through `lc.Control`
  (`network.go`): netns composes WITH them, never at their cost (AC-9).
- `lcpPairNetns` mapping host/root/empty to "" (`lcp.go`), and passing any other name
  through deliberately. -> Constraint (A-13, 2026-07-16): preserve the MAPPING as shipped, but
  ~~so VPP places the TAP in its host namespace~~ is FALSE and must not be preserved as a
  belief: VPP resolves "" to its global default netns (`lcp_interface.c:856-861`), which ze
  sets from the same leaf (`startupconf.go`). Design against A-13, not this comment.
- The `doctor-vpp-lcp-netns` warning until this spec lands. Until BGP can bind in a non-root
  netns, the existing warning is CORRECT as shipped and must not be narrowed.
- Doctor check registration shape and ordering (`doctor.go`, Order 740/741).

**Behavior to change:**

| # | Change | Producer that must change | Why |
|---|--------|--------------------------|-----|
| 1 | `RealListenerFactory` gains a `Netns string` field; when non-empty, `Listen` creates the socket inside that named namespace | `internal/core/network/network.go` + new `netns_linux.go` / `netns_other.go` | The only shape that composes with `MD5Peers` / `ListenTTL` through `newListenerFactory` |
| 2 | `newListenerFactory` carries the netns into BOTH branches, and the MD5/GTSM branch stops discarding configured state | `internal/component/bgp/reactor/reactor.go` | Today that branch drops `r.listenerFactory` entirely (A-2 BROKEN). Without this, netns silently does nothing for MD5/GTSM peers (R-8) |
| 3 | BGP gains a listener namespace config surface (a generic netns leaf, no VPP spelling) | `internal/component/bgp/yang/`, `internal/component/bgp/reactor/config.go` | The netns value must reach `newListenerFactory`. Shape pending Thomas (Q3) |
| 4 | `checkVPPLCPNetns` NARROWS from "netns is not root-reachable" to "vpp.lcp.netns and the BGP listener netns disagree" | `internal/plugins/iface/vpp/doctor.go` | AC-3. Once BGP can bind in a namespace, the current warning is stale; the real hazard becomes a MISMATCH |

→ Decision: the fork "teach BGP netns" vs "force LCP into the host netns" is resolved in
  favour of TEACHING BGP, and Thomas's A-7 answer confirms it. ~~**Both**: the default fix
  handles "the default should not force isolation"; this spec handles "an operator who chooses
  isolation should still get BGP".~~ SUPERSEDED 2026-07-16: there is no default fix. Forcing
  LCP into the host netns is REJECTED outright, because it would delete the
  forwarding/management isolation the IPng model exists to provide (`plan/deferrals.md`
  asks for reachability "without forcing the operator to a root-reachable netns";
  `docs/research/vpp-deployment-reference.md`). Teaching BGP is the only option that
  keeps the shipped default working as documented.
→ Constraint: BGP must NOT learn about VPP or LCP. The netns leaf is generic ("bind listeners
  in this namespace"); the operator sets it to match `vpp.lcp.netns`. Inheriting the value from
  the vpp component would couple BGP to VPP and is REJECTED.

## Data Flow

### Entry Point

Config `vpp { lcp { netns "<name>" } }` (a deliberately isolated namespace) plus a `bgp`
stanza with a listen address and a matching netns setting. The LCP netns value enters via the
vpp component config (`ze-vpp-conf.yang`) and reaches VPP through
`lcp_itf_pair_add_del` (`lcp.go`). The BGP listen address enters via the bgp config and
reaches `NewListener` (`listener.go`). The BGP netns value enters via a new config surface
(shape pending Q3).

### Transformation Path

1. Config parse: `vpp.lcp.netns` lands in the vpp component's settings; the BGP netns lands in
   the reactor's config.
2. Apply: `lcpItfPair` sends `LcpItfPairAddDel` with `lcpPairNetns(settings.Netns)`
   (`lcp.go`, `:87-102`).
3. VPP creates the TAP inside the named namespace.
4. Doctor, post-config phase: `checkVPPLCPNetns` (`doctor.go`) today warns if the netns
   is not root-reachable; after this spec it warns only on a MISMATCH (AC-3).
5. BGP: `NewListener(addr)` (`listener.go`), then the reactor OVERWRITES the factory with
   `r.newListenerFactory(port)` (`reactor.go` global, `:1422` per-port), which returns
   either a fresh `RealListenerFactory{MD5Peers, ListenTTL}` or the injected `r.listenerFactory`
   (`reactor.go`). THIS is where the netns must be threaded, into BOTH branches.
6. `StartWithContext` calls `listenerFactory.Listen` (`listener.go`) ->
   `RealListenerFactory.Listen` (`network.go`) -> `lc.Listen` (`network.go`), on the
   reactor goroutine's thread: ze's namespace, not the TAP's. The bind cannot see the TAP.
   Step 6 holds the gap; step 5 holds the R-8 trap.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Config to bgp reactor | new netns config surface (shape pending Q3) -> `reactor.config` | [ ] |
| Config to vpp component | YANG `vpp/lcp` container, `netns` leaf (`ze-vpp-conf.yang`) | [ ] read 2026-07-16 |
| VPP to Linux | LCP creates a TAP inside the `netns` named per pair (`lcp.go`) | [ ] read 2026-07-16 |
| BGP to kernel socket | `net.ListenConfig` via `RealListenerFactory.Listen` (`network.go`), in ze's namespace | [ ] read 2026-07-16 |
| Namespace boundary (THE GAP) | none: no code crosses it; BGP binds where its thread runs | [ ] read 2026-07-16 |
| Reactor to factory (THE TRAP) | `newListenerFactory` (`reactor.go`) selects either/or and discards the injected factory on MD5/GTSM | [ ] read 2026-07-16 |

### Integration Points

- `network.RealListenerFactory` (`internal/core/network/network.go`) - THE extension
  point. Netns joins `MD5Peers` and `ListenTTL` as a per-bind socket concern.
- `network.ListenerFactory` (`internal/core/network/network.go`) - the interface. Note:
  NOT the extension point. A new implementation of it is discarded by `newListenerFactory` on
  MD5/GTSM ports.
- `Reactor.newListenerFactory` (`internal/component/bgp/reactor/reactor.go`) - the
  real production producer; the netns value must flow through here or it is dropped.
- `Listener.SetListenerFactory` (`internal/component/bgp/reactor/listener.go`) - NOT an
  external seam. The reactor overwrites it at `reactor.go` / `:1422`. `listener.go` needs
  no edit, but not because the seam works: because the netns rides on the factory the reactor
  builds.
- `enterTestNetns` (`internal/test/runner/netns_linux.go`) - the idiom to copy, not to import
  (a test-runner package must not become a production dependency).
- `checkVPPLCPNetns` / `lcpNetnsIsRootReachable` (`internal/plugins/iface/vpp/doctor.go`)
  - the AC-3 seam, and the ONLY file this spec shares with the parent.

### Architectural Verification

- [ ] No bypassed layers (a netns listener goes through `ListenerFactory`, not raw syscalls in
      the reactor)
- [ ] No unintended coupling (BGP does not learn about VPP or LCP; it learns about namespaces)
- [ ] No duplicated functionality (a third field on an existing factory, not a second package)
- [ ] Zero-copy preserved where applicable (N/A: control-plane path, not wire encoding)
- [ ] Registration over hardcoding: no new per-feature switch case or factory is added to a
      core/shared package. The netns is a data field on an existing struct, and the AC-3 doctor
      check stays registered from the owning plugin via `diagnostic.RegisterDoctorCheck`
      (`ai/rules/repo-maintenance.md`, `ai/rules/plugins.md`)
- [ ] Core import direction: `internal/core/network` gains only an external import
      (`github.com/vishvananda/netns`), never `internal/component/` or `internal/plugins/`
      (`ai/rules/architecture.md`, `make ze-tier-check`)

## Risks & Assumptions

→ Constraint: A-N and R-N IDs are PRESERVED from `plan/spec-fixit-vpp-lcp-reachability.md`
rather than renumbered, so that spec's Mistake Log rows, its Design Insights, and any future
learned summary keep pointing at the same things. Gaps in the numbering (A-4, A-5, A-6, R-3,
R-4, R-5, R-7, R-9, R-10) are the doctor half's rows and stay in the parent.

### Assumptions

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | BGP has zero netns awareness today | Read of `listener.go` (lines 32-46, 108) and `network.go` (147-194) plus a grep across `internal/component/bgp/reactor/` returning only event-namespace hits (`bgpevents.Namespace`) | The work is smaller than believed | Re-grep at design time | **confirmed by read, re-verified 2026-07-16** |
| A-2 | A netns-aware listener can be a `ListenerFactory` implementation with no reactor surgery | `SetListenerFactory` (`listener.go`) and the factory call site (`listener.go`) | The change reaches into reactor lifecycle and grows | Prototype a factory that binds in a named netns | **BROKEN, 2026-07-16, re-verified at the producer by this session.** `newListenerFactory` (`reactor.go`, return at `:1402`) returns a FRESH `RealListenerFactory{MD5Peers, ListenTTL}` and discards `r.listenerFactory` when MD5 or GTSM applies; `Listener.SetListenerFactory` is overwritten by the reactor at `reactor.go` / `:1422`, so it is not an external seam at all. Reactor surgery IS required (small: thread netns through `newListenerFactory`). Mistake Log row below |
| A-3 | Binding in a named netns needs OS-thread locking only around socket create+bind, not for the socket's lifetime | `setns` semantics are per-thread; `runtime.LockOSThread` is used in the tree's netns idiom (`internal/test/runner/netns_linux.go`) | The design needs a dedicated thread or an fd-passing helper process; blast radius grows (R-1) | Prototype plus a QEMU test | **UNVALIDATED. HYPOTHESIS about KERNEL semantics, NOT read from ze source; per `ai/rules/evidence.md` it must not be treated as verified.** Supporting but non-probative in-tree evidence: `netns_linux.go`'s header records fork+exec'd children inheriting the locked thread's netns (`TestNetnsLaunchChildInheritsNamespace`), which shows membership attaches at object-creation time. **Settle by prototype (Phase 1) BEFORE any design commitment.** This is now the single largest technical unknown, and it GATES the design |
| A-7 | Operators actually want LCP TAPs in a non-root netns, so teaching BGP netns is worth it rather than forcing host netns | The `netns` leaf exists and defaults to "dataplane" (`ze-vpp-conf.yang`); `lcpPairNetns` (`lcp.go`) passes a non-root netns through so the operator can isolate "deliberately" | The cheaper answer is to default LCP to the host netns and keep the doctor warning | User / operator input | **ANSWERED by Thomas, 2026-07-16: YES, support a non-root netns, because it IS the default and the documented model.** ~~"Both: fix the default now, keep netns as real work."~~ SUPERSEDED same day: that answer rested on the false premise that the `"dataplane"` default was a mistake. It is not (`plan/deferrals.md`; `54bffb83b`; `docs/research/vpp-deployment-reference.md`). The default STAYS, no default-fix spec exists, and "force LCP to the host netns" is REJECTED: it would delete the isolation model. **No longer blocking.** See "Why this is the default case" in Task |
| A-13 (NEW) | An empty per-pair netns in `lcp_itf_pair_add_del` makes VPP place the TAP in VPP's OWN (host) namespace, as asserted by the comment at `lcp.go` | The comment at `internal/plugins/iface/vpp/lcp.go`, and `lcpNetnsIsRootReachable`'s doc at `doctor.go` ("VPP's per-pair netns override maps these to the host netns") | The `host`/`root` escape hatch does not exist, and the `doctor-vpp-lcp-netns` remediation text tells operators to apply a config that BREAKS LCP pair creation | Read of VPP's linux-cp C source at the producer | **BROKEN, 2026-07-16, verified at the producer in VPP's C source (not vendored; fetched from FDio/vpp).** VPP resolves an EMPTY per-pair netns to the GLOBAL default netns, not to VPP's own namespace. `lcp_itf_pair_create` (`src/plugins/linux-cp/lcp_interface.c:856-861`, master): `/* Use interface-specific netns if supplied. Otherwise, use netns if defined, otherwise use the OS default. */ if (ns == 0 \|\| ns[0] == 0) ns = lcp_get_default_ns ();`. `lcp_get_default_ns` (`lcp.c:22-30`) returns `lcpm->default_namespace`, NULL only when unset/empty. The API handler copies `mp->netns` verbatim (`lcp_api.c:60-65`), so ze's `""` arrives as `ns[0]==0` and hits the fallback; the post-fallback `ns` is what reaches the TAP (`lcp_interface.c:1061-1062`, `args.host_namespace = ns`). ze SETS that global default from the SAME leaf: `startupconf.go` writes `default netns <s.LCP.Netns>` unconditionally when LCP is enabled, parsed by `lcp_itf_pair_config` -> `lcp_set_default_ns` (`lcp_interface.c:576-579`, `:608`). Identical in `stable/2306:818-821` and `stable/2402:827-830`: longstanding, not a master-only change. **`""` means "VPP's own namespace" ONLY when the global default is unset, which ze never leaves unset when LCP is enabled.** See the Design Insight and Mistake Log rows below for the reachable consequence |
| A-12 (NEW) | An operator who deliberately isolates an LCP TAP exists, and will configure BGP to match | No code or doc evidence found. `lcp.go` proves the CAPABILITY is deliberate, not that anyone USES it | This spec is a capability with no demonstrated user and should be sequenced behind work that has one. It does not become wrong, only less urgent | Thomas / operator input; a support case; a deployment doc | **BROKEN as framed, 2026-07-16. Re-framed, not dropped.** The row asked whether a *deliberate isolator* exists, because it assumed the default would be changed out from under this spec. The default STAYS (`ze-vpp-conf.yang`), so **this spec's user is not a hypothetical isolator: it is every default LCP install.** That population is evidenced by the shipped default itself, by the leaf's own description ("Routing daemons run in this namespace"), and by `docs/guide/vpp.md` documenting the override an operator must perform today to run BGP at all. The residual question A-12 still honestly carries: how many operators run VPP LCP at all? That is a question about VPP adoption, not about isolation, and it does not gate the design. Mistake Log row below |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Netns binding requires thread-pinning that fights the Go runtime and destabilizes the listener | Flaky accept loop or wrong-namespace binds under load | Prototype early (Phase 1); consider a bind-only helper that passes the fd back. Reuse the `enterTestNetns` shape rather than inventing one |
| R-2 | This spec silently invalidates `doctor-vpp-lcp-netns`, leaving a check that warns about a solved problem | The check still fires after BGP can bind in the netns | Treat the check's fate as an explicit AC (AC-3), not an afterthought. NARROW it to a mismatch check, do not delete it |
| R-6 | Linux-only work lands without QEMU proof | The netns leg is proven only by unit tests | `ai/rules/platform-linux.md` is mandatory. Partially mitigated by a NEW finding: the `integration && linux` rail is auto-discovering (`mk/test-integration.mk`), so the netns BIND is provable today. The residual is the VPP+LCP end-to-end rail (Q12) |
| R-8 | **Silent wrong-namespace bind via the MD5/GTSM branch.** `newListenerFactory` (`reactor.go`) rebuilds the factory from scratch when MD5 or GTSM applies. If netns is carried on the factory but that branch is not updated, an MD5 peer's listener binds in the HOST namespace with no error, and BGP peers on the wrong interface | A netns test that passes without MD5 and fails (or worse, silently binds wrong) with MD5 | Netns must be a FIELD threaded through BOTH branches. MANDATORY test: a netns listener test WITH MD5 configured, not only the bare case (AC-9). This is the concrete instance of the Critical Review "a netns bind failure NEVER silently falls back to the host namespace" row |
| R-11 (NEW) | ~~The spec lands, is correct, and no operator ever uses it: a permanent maintenance surface for a hypothetical user~~ **WITHDRAWN 2026-07-16.** Its premise was A-12 as framed plus a default fix that is not happening. With the default staying `"dataplane"`, the users are every default LCP install, and the maintenance surface serves the shipped config rather than a hypothesis | ~~Nobody asks for it after the default fix lands~~ N/A | ~~Accept deliberately or sequence behind demonstrated work~~ N/A. The honest residual risk is now R-13 |
| R-13 (NEW, 2026-07-16) | The netns gap is worked around instead of fixed: operators keep setting `vpp.lcp.netns` to `host`/`root` because the doctor warning tells them to, so the isolation model quietly dies in the field and this spec never looks urgent | Deployments and docs that treat the root-reachable override as the normal answer rather than a workaround (`docs/guide/vpp.md` documents it as the way out today) | Word the doctor check and the guide as "BGP cannot follow yet", not "use a root-reachable netns". AC-3 narrows the check to a mismatch once BGP can follow. This risk is the honest counterweight to the raised priority: the workaround is cheap, which is exactly why the gap can persist |
| R-12 (NEW) | The two halves of the split touch the same file (`internal/plugins/iface/vpp/doctor.go`) and land in either order, colliding | A merge conflict in `checkVPPLCPNetns`, or an AC-3 narrowing applied to a check the parent spec just rewrote | AC-3 is the ONLY shared file. The parent ships `test/ui/doctor-vpp-lcp-netns.ci` for the CURRENT behavior; this spec rewrites it when it narrows the check (accepted, see AC-3). Whichever lands second re-reads the file rather than trusting this spec's line numbers |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| A BGP netns config surface (shape pending Q3) plus a listen address | -> | `reactor.config` -> `newListenerFactory` (`reactor.go`) -> `RealListenerFactory{Netns}` (`network.go`) -> `listener.go` | `TestNetnsListenerFactoryBindsInNamedNamespace` |
| Same, **with MD5 configured on the port** | -> | the MD5/GTSM branch of `newListenerFactory` (`reactor.go`) | `TestNewListenerFactoryCarriesNetnsWithMD5` (R-8: without this row the netns silently vanishes for MD5 peers) |
| `vpp { lcp { netns X } }` + BGP netns Y (a MISMATCH), `ze doctor` | -> | narrowed `checkVPPLCPNetns` (`doctor.go`) | `test/ui/doctor-vpp-lcp-netns.ci` (rewritten from the parent spec's version; AC-3) |
| BGP peer establishing over an LCP TAP in a non-root netns | -> | full listener plus reactor accept path (`listener.go`) | `test/ui/bgp-listener-netns.ci` with `option=needs-linux` (runs under `make ze-qemu-needs-linux-test`) |

## Acceptance Criteria

→ Constraint: AC IDs are MOVED from `plan/spec-fixit-vpp-lcp-reachability.md`, keeping their
original numbers (AC-1, AC-2, AC-3, AC-8, AC-9, AC-10). The parent's copies are removed, not
duplicated. Gaps (AC-4 to AC-7, AC-11) are the doctor half's and stay in the parent.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `vpp.lcp.netns` names a non-root namespace and BGP is configured with a matching listener netns | BGP binds its listener inside that namespace and accepts sessions on the LCP TAP |
| AC-2 | No netns is configured anywhere (every non-LCP deployment) | Listener behavior is byte-for-byte unchanged; no thread pinning, no new failure mode. `RealListenerFactory{}` with an empty `Netns` takes exactly today's code path (`network.go`) |
| AC-3 | BGP can bind in the LCP netns | The `doctor-vpp-lcp-netns` warning's fate is explicitly decided: NARROWED to a mismatch check between `vpp.lcp.netns` and the BGP listener netns. It does not silently survive as a stale warning. -> Constraint (user, 2026-07-16): the parent spec ships `test/ui/doctor-vpp-lcp-netns.ci` for the check as it exists TODAY; this AC may REWRITE that `.ci`, and that is ACCEPTED, not a defect |
| AC-8 | Netns listener on linux | Proven by a QEMU test, not only host unit tests (`ai/rules/platform-linux.md`). Two rails: an `integration && linux` test (auto-discovered per `mk/test-integration.mk`) and a `needs-linux` `.ci` |
| AC-9 | A netns listener on a port that ALSO carries MD5 peers or a GTSM TTL | The listener binds in the configured namespace AND applies MD5/GTSM. Never one at the cost of the other (R-8; the current `newListenerFactory` would drop the netns) |
| AC-10 | A configured netns that does not exist, or ze lacks CAP_SYS_ADMIN, or a non-linux build | `Listen` returns a clear error naming the namespace. It NEVER falls back to binding in the host namespace |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|---------------------|-----------------------|
| 1 | Deliberately isolates an LCP TAP in a namespace and expects BGP to peer over it | config -> lcp_itf_pair_add_del -> TAP in the named netns -> BGP listener binds in that netns -> peer established | `test/ui/bgp-listener-netns.ci` (`option=needs-linux`), plus the QEMU rail for the VPP end (Q12) |
| 2 | Configures a BGP netns that disagrees with `vpp.lcp.netns` and runs `ze doctor` | config tree -> narrowed `checkVPPLCPNetns` -> mismatch diagnostic | `test/ui/doctor-vpp-lcp-netns.ci` (rewritten, AC-3) |
| 3 | Configures a BGP netns that does not exist | config -> `newListenerFactory` -> `RealListenerFactory.Listen` -> clear error naming the namespace, no host-netns fallback | `TestNetnsListenerFactoryUnknownNamespaceErrors` (AC-10) |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNetnsListenerSocketOutlivesThreadUnpin` | `internal/core/network/netns_integration_linux_test.go` | Accept works after the binding thread is unpinned, proving pinning is bind-scoped (A-3). GATES the design: run in Phase 1 | proposed |
| `TestNetnsListenerFactoryBindsInNamedNamespace` | `internal/core/network/netns_integration_linux_test.go` | The factory binds inside the named namespace | proposed |
| `TestNetnsListenerFactoryUnknownNamespaceErrors` | `internal/core/network/netns_integration_linux_test.go` | An absent namespace errors clearly rather than silently binding in the host netns (AC-10) | proposed |
| `TestNetnsListenerFactoryEmptyNetnsUnchanged` | `internal/core/network/netns_linux_test.go` | An empty `Netns` takes the existing path; no thread pinning, no behavior change (AC-2). Host-runnable, no kernel capability needed | proposed |
| `TestNetnsListenerFactoryAppliedWithMD5` | `internal/core/network/netns_integration_linux_test.go` | `Netns` + `MD5Peers` together: BOTH are applied (AC-9, R-8) | proposed |
| `TestNewListenerFactoryCarriesNetnsWithMD5` | `internal/component/bgp/reactor/reactor_test.go` | The producer `newListenerFactory` (`reactor.go`) threads netns through the MD5/GTSM branch instead of discarding it (R-8). Pure struct assertion, no kernel needed | proposed |
| `TestCheckVPPLCPNetnsMismatchWarns` | `internal/plugins/iface/vpp/doctor_test.go` | The narrowed check warns on a `vpp.lcp.netns` vs BGP netns MISMATCH (AC-3) | proposed |
| `TestCheckVPPLCPNetnsAgreementSilent` | `internal/plugins/iface/vpp/doctor_test.go` | Agreeing namespaces produce no diagnostic (AC-3) | proposed |

→ Constraint: tests needing a namespace need CAP_NET_ADMIN, so they carry
`//go:build integration && linux` and are auto-discovered by `ze-qemu-integration-test`
(`mk/test-integration.mk`). Use `t.Skip`, never `t.Fatal`, when the capability is absent
(`ai/rules/platform-linux.md`). Tests that need no kernel capability (`...EmptyNetnsUnchanged`,
`TestNewListenerFactoryCarriesNetnsWithMD5`) stay host-runnable.

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| BGP listener netns name | string, empty = today's behavior (AC-2) | empty (no pinning at all); a name that exists under `/run/netns/` | N/A (no numeric range) | a name containing `/` or `..`, which MUST be REJECTED at YANG validation, not passed to a filesystem path (Security Review: input validation) |
| `vpp.lcp.netns` | string, YANG default "dataplane" (`ze-vpp-conf.yang`) | any existing namespace name; "", "host", "root" mean the host netns (`doctor.go`) | N/A | N/A. NOT this spec's leaf to change: -> Decision (user, 2026-07-16) the default STAYS `"dataplane"` and no spec changes it. This spec makes BGP reach it |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-listener-netns` | `test/ui/bgp-listener-netns.ci` | BGP binds and peers over an interface in a deliberately isolated namespace. `option=needs-linux`: SKIPs on darwin, runs for real under `make ze-qemu-needs-linux-test` | proposed |
| `doctor-vpp-lcp-netns` | `test/ui/doctor-vpp-lcp-netns.ci` | An operator whose BGP netns disagrees with `vpp.lcp.netns` is warned. **Rewritten** from the version the parent spec ships (AC-3, accepted by Thomas 2026-07-16) | proposed (parent ships v1) |
| BGP over an LCP TAP in a non-root netns | QEMU rail, VPP end NOT identified (Q12) | The full VPP story, end to end | blocked on a VPP image carrying linux-cp |

### Interop Tests

**N/A, confirmed by research 2026-07-16, not assumed.** Evidence: the change is confined to
socket CREATION. `RealListenerFactory.Listen` (`network.go`) returns a `net.Listener`
and nothing downstream of it changes: `acceptLoop` (`listener.go`) and the reactor's
connection handlers receive an ordinary `net.Conn` either way. No message encoding, no
capability, no FSM path is touched, and no file under `internal/component/bgp/wire`, `message`,
or `capability` appears in Files to Modify. A peer cannot observe which namespace the socket
was created in. `ai/rules/interop-and-goal-validation.md` governs; the goal validation is the
QEMU rail (AC-8), not an interop matrix.

### Future

None deferred. Scope is set above and every AC is assigned.

## Files to Modify

- `internal/core/network/network.go` - add a `Netns string` field to `RealListenerFactory`
  (beside `MD5Peers` and `ListenTTL`, lines 147-162) and route `Listen` (lines 164-194) through
  the build-tagged helper when it is non-empty. NOT through `lc.Control`: the socket must be
  CREATED in the namespace, so the helper wraps the whole `lc.Listen` call
- `internal/component/bgp/reactor/reactor.go` - **REQUIRED (A-2 BROKEN, R-8).**
  `newListenerFactory` (lines 1394-1422) must carry the netns into BOTH branches, or netns is
  silently dropped on MD5/GTSM ports
- `internal/component/bgp/reactor/config.go` - carry the netns value from config to
  `newListenerFactory`
- `internal/component/bgp/yang/` - a netns leaf; read `ai/rules/config.md` and
  `ai/rules/config.md` first. Shape pending Thomas (Q3)
- `internal/plugins/iface/vpp/doctor.go` - narrow `checkVPPLCPNetns` /
  `lcpNetnsIsRootReachable` (lines 100-143) to a mismatch check (AC-3). THE ONLY file shared
  with the parent spec (R-12)
- `internal/core/diagnostic/codes.go` - reword the `doctor-vpp-lcp-netns` description (near
  lines 288-299) to match the narrowed check
- `docs/guide/vpp.md` - the netns constraint is operator-facing and documented there as a hard
  limit; it stops being one
- `internal/component/bgp/reactor/listener.go` - **NO EDIT.** Recorded because the parent spec
  originally expected one. The netns rides on the factory the reactor builds; the seam at
  `listener.go` is overwritten by the reactor (`reactor.go`, `:1422`) and is not
  usable

**Not to modify:**

- `internal/component/vpp/yang/ze-vpp-conf.yang` - the `netns` default. -> Decision (user,
  2026-07-16): it **STAYS** `"dataplane"`. It is not a bug, no spec owns changing it, and
  changing it would delete the forwarding/management isolation this spec exists to preserve.
  Do not touch it here or anywhere
- `mk/test-integration.mk` - no edit needed for the netns integration tests
  (`ZE_QEMU_INTEGRATION_PKGS` at line 319 auto-discovers the build tag)

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] Yes, pending Thomas (Q3). Read `ai/rules/config.md` (YANG vs env var) and `ai/rules/config.md` | `internal/component/bgp/yang/` |
| YANG validation constraints | [ ] Yes. A namespace name is a `/run/netns/<name>` path component, so constrain `pattern` + `length`; never a bare `type string` (Security Review: input validation) | per `ai/patterns/config-option.md` |
| Doctor check for runtime dependencies | [ ] Yes, the AC-3 narrowing of an EXISTING check (no new code) | `internal/plugins/iface/vpp/doctor.go`, `internal/core/diagnostic/codes.go` |
| Functional test | [ ] Yes, two | `test/ui/bgp-listener-netns.ci` (new), `test/ui/doctor-vpp-lcp-netns.ci` (rewritten) |
| QEMU rail | [ ] Yes, MANDATORY (`ai/rules/platform-linux.md`). The netns-bind rail EXISTS and auto-discovers (`mk/test-integration.mk`); the `.ci` rail is `option=needs-linux`. The VPP+LCP end-to-end rail is NOT identified (Q12) | `internal/core/network/netns_integration_linux_test.go`, `test/ui/bgp-listener-netns.ci` |
| CLI commands/flags | [ ] No | - |
| Env var registration | [ ] Only if Q3 resolves to a process-level env setting rather than a YANG leaf | per `ai/rules/config.md` |
| Prometheus counters | [ ] No | - |

### Documentation Update Checklist

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] Yes (BGP netns binding) | `docs/features.md` |
| 2 | Config syntax changed? | [ ] Yes, if a netns leaf lands (Q3) | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 6 | Has a user guide page? | [ ] Yes. The guide documents the netns constraint as a hard limit; it becomes a configurable capability | `docs/guide/vpp.md` |
| 10 | Test infrastructure changed? | [ ] Check. A new `needs-linux` `.ci` and a new integration package are additive, but `ai/rules/platform-linux.md` step 5 is stale for `ze-qemu-integration-test` (the list auto-discovers, `mk/test-integration.mk`) | `docs/functional-tests.md`, `ai/rules/platform-linux.md` |
| 12 | Internal architecture changed? | [ ] Yes. `network.go`'s package doc and `listener.go`'s `// Design:` anchor both describe listener creation; a namespace concept changes that contract | `docs/architecture/core-design.md` |
| 15 | Registered diagnostic code changed? | [ ] Yes: `doctor-vpp-lcp-netns` description reworded for the narrowed check | `internal/core/diagnostic/codes.go`, `docs/guide/vpp.md` |
| 16 | Any changed source file referenced by doc source anchors? | [ ] Grep `docs/` for the changed files. Known: `network.go` anchors `docs/architecture/chaos-web-dashboard.md`; `lcp.go` anchors `docs/research/vpp-deployment-reference.md`; `doctor.go` anchors `ai/rules/repo-maintenance.md`. Run `scripts/dev/check_doc_links.py --design-only` | per grep |

## Files to Create

- `internal/core/network/netns_linux.go` - the linux netns bind helper serving the new
  `RealListenerFactory.Netns` field. NOT a standalone `ListenerFactory` implementation: see
  A-2 / R-8 for why a rival factory type is silently discarded by the reactor
- `internal/core/network/netns_other.go` - the `//go:build !linux` stub, mirroring
  `md5_other.go` / `ttl_other.go`. A configured netns here MUST be a clear error, never a
  silent host bind (AC-10)
- `internal/core/network/netns_integration_linux_test.go` - `//go:build integration && linux`.
  Auto-discovered by `ze-qemu-integration-test` (`mk/test-integration.mk`), no Makefile edit
- `internal/core/network/netns_linux_test.go` - the host-runnable unit tests that need no
  kernel capability
- `test/ui/bgp-listener-netns.ci` - new functional test, `option=needs-linux`

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 13. /ze-review gate | Review Gate section |

### Implementation Phases

**BLOCKING: this spec is `design`, not `ready`. A-7 is answered, so the work is approved in
principle, but two gates precede any coding: Thomas promotes the spec to `ready`, and Q3 (the
config-surface shape) is decided. Phase 1 is a throwaway prototype and settles A-3, which gates
the design of every phase after it.**

→ Constraint: phases are MOVED from `plan/spec-fixit-vpp-lcp-reachability.md` Phases 2-6 and
renumbered 1-5 here. Mapping, for anyone following a reference to the old numbering: parent
Phase 2 -> Phase 1; parent Phase 3 -> Phase 2; parent Phase 4 -> Phase 3; parent Phase 5 ->
Phase 4; parent Phase 6 -> Phase 5. The parent's Phase 1 (the doctor check) stays in the parent.

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase 1: Prototype the netns bind (settles A-3 BEFORE committing to a design).**
   Spike only, discarded after. Does a socket created in a named netns keep serving after the
   binding thread is unpinned? If NO, the whole design changes (dedicated thread or fd-passing
   helper, R-1) and the phases below are rewritten.
   - Test: `TestNetnsListenerSocketOutlivesThreadUnpin`
   - Verify: run it under QEMU (`make ze-qemu-integration-test`), not on the host. A darwin
     pass proves nothing about `setns`
2. **Phase 2: `RealListenerFactory.Netns` (wiring).** Add the field + `netns_linux.go` /
   `netns_other.go`, reusing the `enterTestNetns` idiom (`internal/test/runner/netns_linux.go`)
   without importing the test-runner package.
   - Tests: `TestNetnsListenerFactoryBindsInNamedNamespace`,
     `TestNetnsListenerFactoryUnknownNamespaceErrors` (must ERROR, never fall back: AC-10),
     `TestNetnsListenerFactoryEmptyNetnsUnchanged` (AC-2),
     `TestNetnsListenerFactoryAppliedWithMD5` (AC-9)
   - Files: `internal/core/network/network.go`, `netns_linux.go`, `netns_other.go`
   - Verify: `make ze-tier-check` still passes (core import direction)
3. **Phase 3: Config surface + thread it through the reactor.**
   - Add the BGP netns leaf (shape pending Q3), carry it via `config.go` into
     `newListenerFactory` (`reactor.go`) so BOTH branches set `Netns` (R-8)
   - Tests: `TestNewListenerFactoryCarriesNetnsWithMD5`
   - Files: `internal/component/bgp/reactor/reactor.go`, `config.go`,
     `internal/component/bgp/yang/`
4. **Phase 4: Netns doctor check reconciliation (AC-3).** Narrow `checkVPPLCPNetns` to a
   mismatch check between `vpp.lcp.netns` and the BGP listener netns; reword its code
   description; rewrite `test/ui/doctor-vpp-lcp-netns.ci` (the parent spec ships v1).
   - Tests: `TestCheckVPPLCPNetnsMismatchWarns`, `TestCheckVPPLCPNetnsAgreementSilent`
   - Verify: re-read `doctor.go` first. The parent spec may have changed it (R-12)
5. **Phase 5: Functional and QEMU tests.** Prove BGP peers over an interface in a non-root
   netns via `test/ui/bgp-listener-netns.ci` (`option=needs-linux`) under
   `make ze-qemu-needs-linux-test`. -> Constraint: the VPP+LCP END-TO-END rail is still NOT
   identified (Q12) and needs a VPP image carrying the linux-cp plugins. Identify it BEFORE
   Phase 2, or Phase 5 becomes an unbounded task discovered at the end.
6. **Full verification**: `make ze-precommit-verify`
7. **Complete spec**: learned summary, two-commit closure per `ai/rules/planning.md`.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | A netns bind failure NEVER silently falls back to the host namespace (that would peer on the wrong interface) |
| Correctness (R-8) | `newListenerFactory` sets `Netns` in BOTH branches. Grep the function; a netns test without MD5 does not prove this |
| Data flow | BGP learns about namespaces, not about VPP or LCP; no VPP spelling in the reactor or in `internal/core/network` |
| Registration over hardcoding | No new per-feature switch case or factory in a core/shared package; the doctor check stays registered from the owning plugin (`ai/rules/repo-maintenance.md`, `ai/rules/plugins.md`) |
| Module tiers | `internal/core/network` imports no `internal/component/` or `internal/plugins/` package (`make ze-tier-check`) |
| YANG validation | The netns leaf has maximum native constraints (`pattern`, `length`); no bare `type string` |
| Rule: no-workarounds | The netns constraint is fixed at the source, not documented away |
| Stale check | `doctor-vpp-lcp-netns` does not survive as a warning for a solved problem (AC-3) |
| Split hygiene | `internal/plugins/iface/vpp/doctor.go` was re-read, not assumed, before editing (R-12) |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| BGP binds in a named netns | `TestNetnsListenerFactoryBindsInNamedNamespace` passes under `make ze-qemu-integration-test` |
| Netns composes with MD5 | `TestNetnsListenerFactoryAppliedWithMD5` + `TestNewListenerFactoryCarriesNetnsWithMD5` both pass |
| Default deployments unchanged | Existing BGP tests pass with no listener behavior change; `TestNetnsListenerFactoryEmptyNetnsUnchanged` |
| End-to-end peering over an isolated interface | `test/ui/bgp-listener-netns.ci` passes under `make ze-qemu-needs-linux-test` |
| The stale warning is gone | `test/ui/doctor-vpp-lcp-netns.ci` asserts mismatch-only behavior |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | A config-supplied namespace name reaches a `setns`-style operation and a `/run/netns/<name>` path; validate it at YANG (`pattern`, `length`) and never traverse to an arbitrary path (no `/`, no `..`) |
| Privilege | Entering a namespace needs CAP_SYS_ADMIN; confirm the failure mode when ze lacks it is a clear error, not a silent host-netns bind (AC-10) |
| Resource exhaustion | Thread pinning per listener must not leak OS threads across restarts (R-1) |
| Error leakage | Bind errors name the namespace; confirm they carry no unexpected host detail |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Netns bind works in host unit tests but not under QEMU | Back to RESEARCH on thread pinning (A-3); do not weaken the test |
| Phase 1 prototype shows pinning is lifetime-scoped | STOP. A-3 is broken; present the dedicated-thread and fd-passing options to Thomas before redesigning (R-1) |
| `make ze-tier-check` fails | The helper reached for a component/plugin import. Redesign; do not baseline the violation |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| (A-2) A netns listener can be a new `ListenerFactory` implementation injected through the existing seam, with no reactor surgery | `Listener.SetListenerFactory` (`listener.go`) is overwritten by the reactor immediately after `NewListener` (`reactor.go`, `:1422`), so it is not an external seam. And the producer `newListenerFactory` (`reactor.go`) DISCARDS the injected `r.listenerFactory` whenever MD5 or GTSM applies to the port, returning a fresh `RealListenerFactory{...}` | Read the producer `newListenerFactory` instead of stopping at the consumer `listener.go`. The original skeleton had read only the call site and the setter, and inferred the seam was usable | Design change: netns becomes a FIELD on `RealListenerFactory`, not a rival factory. Reactor DOES change. Prevented a silent wrong-namespace bind for every MD5/GTSM peer (R-8) |
| (A-3, basis only) The `runtime.LockOSThread` netns precedent lives at `internal/plugins/static/resolve_integration_linux_test.go` | That path was not verified. The real in-tree netns idiom is `internal/test/runner/netns_linux.go` (`enterTestNetns`), plus `internal/core/routewatch/routewatch_linux.go` | Grepped for netns across the tree instead of trusting the cited path | Basis corrected. The assumption's substance (pinning is needed) still needs a prototype; only its citation was wrong |
| (A-7, framing) The question is "is a non-root LCP netns worth supporting", implying a niche opt-in capability | The unreachable case is the DEFAULT: `ze-vpp-conf.yang` defaults `netns` to "dataplane" and `doctor.go` accepts only "", "host", "root", so the shipped default puts LCP TAPs where BGP cannot bind | Reframed by reading the YANG default and the root-reachable predicate together, then presenting both to Thomas | The question split in two. Recorded because the framing, not the answer, was the mistake: a spec's assumption can be mis-framed such that BOTH possible answers are wrong |
| (A-7, second framing error) The default `"dataplane"` "contradicts its own design intent", cited from the comment at `lcp.go` | The default is DELIBERATE and correct. `plan/deferrals.md` (2026-07-10) records the actual decision: make BGP netns-aware "so LCP TAPs in a non-root netns are reachable by BGP **without forcing the operator to a root-reachable netns**". `"dataplane"` is IPng's production convention, copied on purpose (`54bffb83b` "following IPng production template"; `docs/research/vpp-deployment-reference.md`) | Thomas rejected the premise, 2026-07-16. The session had read `plan/deferrals.md` EARLIER IN THE SAME SESSION (it is this spec's Origin row) and still went to a code comment for design intent | Produced a phantom spec (`plan/spec-fixit-vpp-lcp-netns-default.md`, never created) and INVERTED this spec's priority: it argued itself down to "build last, no user evidenced" when it is in fact the fix for the default config. **A comment states what its author believed; recorded decisions live in `plan/deferrals.md`, `plan/learned/`, and specs.** Escalated below |
| (A-13) An empty per-pair netns puts the TAP in VPP's own (host) namespace, so `netns host` is a working escape hatch. Asserted by `lcp.go` and `doctor.go`, and repeated by three prior sessions of this spec lineage as settled background | VPP's linux-cp netns model is TWO-LEVEL. `lcp_itf_pair_create` (`lcp_interface.c:856-861`) resolves an empty per-pair netns to the GLOBAL default (`lcp_get_default_ns`), which ze itself sets from the SAME leaf (`startupconf.go`). `""` means "VPP's own namespace" only when the global default is unset, which ze never leaves unset when LCP is enabled | Read VPP's C source at the producer. The vendored binapi (`vendor/go.fd.io/govpp/binapi/lcp/lcp.ba.go`) is generated stubs and documents only the FIELD ("optional tap netns", `:354`); it cannot express the resolution rule, so no amount of reading it would have caught this | The `host`/`root` escape hatch does not work: it makes VPP look for `/var/run/netns/host` and fail the pair (loudly). `doctor-vpp-lcp-netns` actively recommends it (`doctor.go`). Strengthens this spec (netns-aware BGP is the ONLY working path) and puts a real defect on the parent's doctor half |
| (A-12) This spec's user is an operator who deliberately isolates an LCP TAP, and none is evidenced | This spec's user is EVERY DEFAULT LCP INSTALL. A-12 only looked unevidenced because it presumed the default would be changed. With the default staying, the shipped config itself is the evidence | Followed from Thomas's ruling that the default stays | The "no demonstrated user" argument, R-11, and the "build it last" priority all collapse together. They shared one premise |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| A netns-aware `ListenerFactory` implementation injected via `Reactor.SetListenerFactory` | Silently discarded by `newListenerFactory` (`reactor.go`) on every MD5/GTSM port: the factory is chosen either/or, never stacked | A `Netns string` FIELD on `RealListenerFactory`, threaded through both branches |
| A new `internal/core/netns` package | `internal/core/network` already owns listener socket creation; a second package splits one concern across two | `netns_linux.go` / `netns_other.go` inside `internal/core/network`, mirroring `md5_*.go` / `ttl_*.go` |
| Inheriting the BGP netns from `vpp.lcp.netns` | Couples BGP to VPP and inverts the dependency (`ai/rules/architecture.md` anti-pattern "translation layer") | A generic BGP netns leaf; the operator sets both; the doctor check catches disagreement (AC-3) |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| A blocking question framed as "is X worth supporting?" when the code shows X is the DEFAULT, not an opt-in | Seen once (A-7), but the class is general: a question's FRAMING can be wrong independently of its answer | Before asking the user a scoping question, read the default that decides who is affected. "Is this worth supporting" and "does the shipped default already do this to everyone" are different questions with different answers | Candidate for `ai/rules/evidence.md` (a recommendation's PREMISE must be traced, and so must a QUESTION's) |
| Treating a vendored GENERATED binapi stub as the authority on a foreign system's SEMANTICS. `lcp.ba.go` documents the netns field as "optional tap netns; netns[0] == 0 if none" and that is all it can ever say: the resolution rule lives in VPP's C, which is not vendored. Three sessions built on `""` == "VPP's own namespace" without reading a line of VPP | Seen once (A-13), but the class is broad: ze vendors generated bindings for VPP, netlink, and gNMI, and every one describes WIRE SHAPE while the BEHAVIOR sits in the peer's implementation | A generated binding is evidence of a field's existence and encoding, never of what the remote end DOES with it. When a claim about a foreign system's behavior is load-bearing, the producer is that system's source, not our stubs. `ai/rules/evidence.md`'s "read the producer" already says this; what it does not say is that the producer can be OUTSIDE the repo | Candidate for `ai/rules/evidence.md`: extend "read the producer" to cross-boundary claims, naming vendored binapi/stubs as a non-source. **Raise with Thomas; not actioned by this investigation** |
| Reading a code comment for DESIGN INTENT and concluding the code "contradicts its own intent", when the recorded decision was in `plan/deferrals.md` all along | Seen once here (A-7's second framing error), but the shape is general and cheap to repeat: comments are the nearest text to the code, so they are what a source-reading session finds first | A comment states what its author believed, not what was decided. Recorded decisions live in `plan/deferrals.md`, `plan/learned/`, and specs; check them before asserting intent | **ACTIONED 2026-07-16:** added to `ai/rules/evidence.md` ("Behavioral claims and recommendations" + a Banned row). Chosen over `ai/rules/evidence.md`'s Evidence corollary because that corollary is scoped to SAFETY properties verified at a producing function, while this is the same rule's core method (read the producer, not the nearby text) applied to a claim about INTENT, whose producer is the decision record |

## Design Insights

- The keystone: `newListenerFactory` (`reactor.go`) is an either/or selector, not a
  composer. Anything a listener needs at socket-creation time must be a FIELD on
  `RealListenerFactory`, because that is the only thing both branches can carry. `MD5Peers` and
  `ListenTTL` are already fields for exactly this reason. This generalizes beyond netns: the
  next per-bind socket concern faces the same constraint.
- The failure mode this prevents is the dangerous kind: not a crash, but a silent bind in the
  wrong namespace for MD5-authenticated peers only. It would pass every test that did not
  configure MD5. Hence AC-9 and the mandatory netns+MD5 test.
- `vpp.lcp.netns` defaults to "dataplane" and `lcpNetnsIsRootReachable` accepts only
  "", "host", "root", so the SHIPPED DEFAULT trips the netns warning whenever BGP is
  configured. That reframed the whole question (see the Mistake Log).
- ~~A-7's answer approves this work but does not evidence a user for it (A-12, R-11).~~
  **SUPERSEDED 2026-07-16.** The default stays `"dataplane"`, so the user is every default LCP
  install and the shipped config is the evidence. The "approved but not demanded" framing was
  an artifact of assuming the default would change.
- **VPP's linux-cp netns model is TWO-LEVEL, and ze drives both levels from ONE leaf. A-13 is
  BROKEN (2026-07-16).** An empty per-pair netns does NOT mean "VPP's own namespace"; it means
  "fall back to the global default", and ze sets that global default from the same
  `vpp.lcp.netns` leaf (`startupconf.go`). Tracing the reachable cases:
  `validateNetns` (`config.go`, enforced at `config.go`) REJECTS an empty
  `netns` when LCP is enabled, so `lcpPairNetns` (`lcp.go`) returns `""` only for
  `host` or `root`. Configuring `netns host` therefore yields startup.conf `default netns host`
  PLUS per-pair `""`, and VPP resolves the `""` to `"host"`: it opens `/var/run/netns/host`
  (`lcp.c:65-68`) and passes `args.host_namespace = "host"` to `tap_create_if`
  (`lcp_interface.c:1061-1062`). `"host"` is a namespace NAME to VPP, not a synonym for its own
  namespace.
  → Consequence, and it is the honest good news: this is a LOUD failure, not a silent
  misplacement. With no `/var/run/netns/host`, `tap_create_if` fails, `lcp_itf_pair_create`
  returns `args.rv < 0` (`lcp_interface.c:1066-1071`), the API reply carries a non-zero retval,
  and `lcpItfPair` errors (`lcp.go`). Note `lcp_set_default_ns` returns 0 even when the
  `open` fails (`lcp.c:65-68`), so VPP itself starts cleanly and the failure surfaces per pair.
  The silent-wrong-namespace case needs a namespace literally NAMED `host`/`root` to exist.
  → Consequence for the doctor check: `doctor-vpp-lcp-netns`'s DETECTION still fires on the
  right condition, but its REMEDIATION is wrong. It tells the operator to "Set vpp.lcp.netns to
  host or root" (`doctor.go`), which is precisely the config that breaks LCP pair
  creation, and `lcpNetnsIsRootReachable`'s doc comment (`doctor.go`) asserts the same
  false premise. **Not fixed here (investigation only); reported to Thomas.**
  → Consequence for THIS spec: the approach is UNAFFECTED and strengthened. The `host`/`root`
  escape hatch that netns-aware BGP was an alternative to does not actually work, so teaching
  BGP to bind in the named namespace is not one of two options; it is the only one. AC-3's
  narrowing must NOT be written as though `host`/`root` is a working configuration.
- The deepest lesson here is not about netns. This spec twice argued from a code comment
  (`lcp.go`) about what the project INTENDED, while the recorded decision sat in
  `plan/deferrals.md` and is quoted in this spec's own Origin section. A comment is evidence
  of an author's belief; it is not a decision record. The first error cost a phantom spec; the
  second inverted this spec's own priority against itself.

### ARCHITECTURAL CALL: where does netns awareness belong? (answered 2026-07-16)

The original skeleton proposed a netns-aware `ListenerFactory` in `internal/core/network/`, a
core leaf package, to serve a VPP-specific need. That deserved challenge. **Verdict: the
PACKAGE is right, the SHAPE was wrong.**

| Rule | Test | Result |
|------|------|--------|
| `ai/rules/plugins.md` "no plugin spelling in generic packages" | Does the code spell a plugin? | **PASSES.** The field is `Netns`, a Linux kernel concept. Nothing in `internal/core/network` learns the words `vpp`, `lcp`, or `dataplane`. Delete the vpp plugin and `RealListenerFactory.Netns` still makes sense for any operator isolating a listener. The rule forbids plugin SPELLING, not "any feature a plugin happens to want" |
| `ai/rules/architecture.md` core import-direction rule | Does `internal/core/` import `internal/component/` or `internal/plugins/`? | **PASSES.** The only new import is `github.com/vishvananda/netns` (external, `go.mod:23`). `internal/core/routewatch/routewatch_linux.go` already imports it from core, so the precedent is exact |
| `ai/rules/architecture.md` axis A (config-driven engine?) | Does it call `sdk.NewWithConn(`? | **PASSES.** No. It is a pure library with no lifecycle: `internal/core/` is correct per the authoring rule ("pure library, no `sdk.NewWithConn`, no plugin lifecycle, no component domain owner") |
| `ai/rules/architecture.md` "abstract at 2+ use cases" | Is one caller enough to justify core placement? | **NEUTRAL, not a violation.** `internal/core/network` ALREADY exists and already serves exactly this class of concern. Adding a third field beside `MD5Peers`/`ListenTTL` is not a new abstraction; it is an instance of the existing one |

Why the shape (field, not wrapper) is forced, not chosen:

- `newListenerFactory` (`reactor.go`) selects a factory **either/or**: a fresh
  `RealListenerFactory{MD5Peers, ListenTTL}` OR the injected one. There is no stacking. A
  wrapper factory is therefore silently discarded exactly when MD5/GTSM is configured (R-8).
- `MD5Peers` and `ListenTTL` are already fields for the identical reason: both are per-bind
  socket-creation concerns that must compose. Netns is a third of the same kind.
- The package's file layout already encodes this: `md5_{linux,freebsd,darwin,other}.go` and
  `ttl_{linux,other}.go` each implement one field's platform behavior. `netns_{linux,other}.go`
  is the same pattern, not a new one.

→ Decision: `Netns string` on `RealListenerFactory`, helper in
  `internal/core/network/netns_linux.go` + `netns_other.go`. The VPP-specific knowledge stays
  where it belongs: in `ifacevpp`'s doctor check, the only place that knows `vpp.lcp.netns` and
  the BGP listener netns should agree.
→ Constraint: one genuine asymmetry to respect. `MD5Peers`/`ListenTTL` are applied via
  `lc.Control` (`network.go`), a callback on the already-created fd. Netns CANNOT use
  `Control`: the socket must be CREATED inside the namespace, so the helper must wrap the whole
  `lc.Listen` call (lock thread, setns, listen, restore, unlock) rather than plug into
  `Control`. Same field shape, different insertion point.

### Where netns does NOT belong

- **Not in the reactor.** BGP must not learn about VPP or LCP. It learns "bind in namespace X".
- **Not inherited from `vpp.lcp.netns`.** That couples BGP to VPP and inverts the dependency.
  The operator sets both; the doctor check catches disagreement (AC-3).
- **Not a new `internal/core/netns` package.** `internal/core/network` already owns listener
  socket creation; a second package would split one concern across two.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `Netns` as a FIELD on `RealListenerFactory` | A rival `ListenerFactory` implementation; a wrapping/decorating factory | `newListenerFactory` (`reactor.go`) chooses either/or and discards the injected factory on MD5/GTSM ports, so a rival or wrapper is silently dropped exactly where it matters (R-8). Verified at the producer |
| Helper in `internal/core/network` | A new `internal/core/netns` package; inline syscalls in the reactor | The package already owns listener socket creation and already splits per-OS work per field (`md5_*.go`, `ttl_*.go`). A second package splits one concern; reactor syscalls bypass the layer |
| A generic BGP netns leaf | Inheriting `vpp.lcp.netns` into the BGP config | Inheriting couples BGP to VPP and inverts the dependency. The operator sets both; the doctor check catches disagreement |
| Split from the parent spec | One combined spec | Thomas, 2026-07-16: unrelated problems sharing a filename. The doctor half is bounded and ships now; this half needs a config surface, reactor surgery, a kernel prototype, and a QEMU rail |
| Keep A-N / AC-N / R-N numbering from the parent | Renumber from 1 | Preserves cross-references from the parent's Mistake Log, Design Insights, and any future learned summary. Gaps are self-documenting: they mark the doctor half's rows |

## Known Limitations

- **Outbound BGP connections stay in ze's namespace.** Only the LISTENER gains netns awareness.
  `RealDialer` (`network.go`) would need its own `Netns` field for a peer whose outbound
  session must egress the LCP TAP. Sufficient for listener-side LCP peering; insufficient if ze
  must INITIATE over the TAP. -> Open: Q4.
- **Other listeners stay in ze's namespace.** Web (`web/server.go`), looking-glass
  (`lg/server.go`), and the DNS server (`dnsserver/secure.go`) construct listeners
  directly and do not route through `network.ListenerFactory`, so they gain nothing from this
  work. BGP is genuinely the special case today. -> Open: Q5.
- **The netns leg is linux-only.** `netns_other.go` stubs it; a configured netns on a non-linux
  build must be a clear error, never a silent host bind (AC-10).
- **The VPP end-to-end rail is not identified.** The netns BIND is provable today
  (`mk/test-integration.mk`); proving BGP peering over a real VPP LCP TAP needs a VPP image
  carrying the linux-cp plugins, which the followup-vpp-iface work recorded as
  absent from `ligato/vpp-base`. -> Open: Q12.
- ~~**This spec serves a user population that is not evidenced.** See A-12 and R-11.~~
  **WITHDRAWN 2026-07-16.** The premise (that the `"dataplane"` default would be fixed away)
  was rejected by Thomas. The default stays, so this spec serves every default LCP install.
  The residual honest limitation is narrower: **VPP LCP adoption itself is not evidenced in
  the tree.** That bounds how many operators this reaches, but it is a question about VPP
  uptake, not about namespace isolation, and it does not weaken the design.

## Open Questions

| # | Question | Answer |
|---|----------|--------|
| 1 | Mechanism for binding inside a named netns from Go? | **ANSWERED (design), pending prototype.** Reuse the in-tree idiom `enterTestNetns` (`internal/test/runner/netns_linux.go`): `runtime.LockOSThread` + `netns.Get` + enter + same-thread `restore()`. `github.com/vishvananda/netns v0.0.5` is already a direct dep (`go.mod:23`) and `internal/core/routewatch/routewatch_linux.go` already imports it from core. No new dependency, no new idiom. Copy the shape; do NOT import the test-runner package from production code |
| 2 | Does the socket STAY in the namespace after bind, or is the namespace bind-time only? | **HYPOTHESIS, must be prototyped (A-3, Phase 1).** Believed bind-time only: namespace membership attaches at socket creation, so pinning wraps create+bind and `Accept` then works from any thread. This is a claim about KERNEL semantics, NOT read from ze source, so per `ai/rules/evidence.md` it is labelled unverified and gates Phase 2 via `TestNetnsListenerSocketOutlivesThreadUnpin`. If false, the design changes to a dedicated thread or fd-passing helper (R-1) |
| 3 | Should the netns be a BGP config leaf, or a process-level setting? | **PARTLY ANSWERED. Inheriting from `vpp.lcp.netns` is REJECTED** (couples BGP to VPP, inverts the dependency). A generic BGP leaf is the recommendation. **NEEDS THOMAS:** the exact shape (`bgp { listen { netns } }` vs a process-level setting) and whether it is YANG or env per `ai/rules/config.md`. Note a process-level setting would answer Q5 for free. This is the remaining gate on `ready` |
| 4 | Do outbound BGP connections (the dialer) need the same treatment? | **NEEDS THOMAS / follow-up.** `RealDialer` (`network.go`) has the same structure and would need a matching `Netns` field for a peer whose OUTBOUND session must egress the TAP. For a listener-only LCP peering story it is not required. Recorded as a Known Limitation rather than silently scoped out |
| 5 | Do web / gnmi / looking-glass listeners need this too? | **NEEDS THOMAS / follow-up.** They do NOT go through `network.ListenerFactory` (`web/server.go`, `lg/server.go`, `dnsserver/secure.go` each build listeners directly), so BGP is genuinely special TODAY. If an operator wants the whole box in the dataplane namespace, a process-level netns (Q3) is a better answer than per-service leaves |
| 6 | What happens to `doctor-vpp-lcp-netns` if BGP can bind in the LCP netns? | **ANSWERED: NARROW, do not delete.** It becomes a mismatch check: warn when `vpp.lcp.netns` and the BGP listener netns disagree. That is a real, permanent hazard, whereas "netns is not root-reachable" becomes false once BGP can follow. AC-3, Behavior-to-change #4, Phase 4 |
| 7 | Should the `vpp.lcp.netns` default stay "dataplane"? | **ANSWERED: YES, IT STAYS.** -> Decision (user, 2026-07-16). ~~The default is fixed now, as `plan/spec-fixit-vpp-lcp-netns-default.md` (another agent).~~ SUPERSEDED: **that spec was never created and must not be.** The default is deliberate, not a defect: `plan/deferrals.md` records the intent as reachability "without forcing the operator to a root-reachable netns", and `"dataplane"` is IPng's production convention (`54bffb83b`; `docs/research/vpp-deployment-reference.md`). Changing it would delete the isolation model. Q7 is CLOSED, not moved |
| 12 | What QEMU rail can prove BGP peering over an LCP TAP? | **PARTLY ANSWERED, 2026-07-16.** The netns BIND rail EXISTS: `ZE_QEMU_INTEGRATION_PKGS` (`mk/test-integration.mk`) auto-discovers any package with `//go:build integration && linux`, and a daemon-level `.ci` marked `option=needs-linux` runs under `make ze-qemu-needs-linux-test`. Neither needs a Makefile edit. **The residual, still unanswered:** the VPP+LCP end-to-end rail. The followup-vpp-iface work recorded that real-VPP LCP proof needs a VPP image WITH the linux-cp plugins (`ligato/vpp-base` lacks them), which is an image-provisioning problem on top of a test-rail problem. -> Constraint: identify it before Phase 2 or the netns leg has an unbounded tail |

## Decisions Needed From Thomas (blocking `ready`)

| # | Decision | Why it cannot be decided from code |
|---|----------|-----------------------------------|
| 1 | **Q3:** the BGP netns config surface shape (per-listener YANG leaf vs process-level setting) | A design choice with a downstream effect on Q4/Q5; `ai/rules/config.md` governs but does not decide |
| 2 | ~~**Priority:** should this be built now, or sequenced behind work with a demonstrated user?~~ **ANSWERED (user, 2026-07-16): PRIORITY RAISED.** No longer a decision needed. The question presumed the default would be fixed elsewhere; it is not being fixed, so nothing else closes this gap. This spec is what makes the shipped default config work | ~~A-7 approved the capability; A-12 records that no operator using it is evidenced~~ Answered. The code could not say how many operators run LCP, but it COULD say the default puts every LCP TAP where BGP cannot bind (`ze-vpp-conf.yang` + `network.go`), which is what raised the priority |
| 3 | **Q4 / Q5:** do the dialer and the other listeners come in scope? | Recorded as Known Limitations. A process-level netns (Q3) would answer Q5 for free, which is why Q3 should be decided with Q5 in view |

## RFC Documentation

N/A. No protocol behavior changes: the work is confined to socket creation, and a peer cannot
observe which namespace the socket was created in. See "Interop Tests" for the evidence.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1, AC-2, AC-3, AC-8, AC-9, AC-10 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete (every row has a concrete test name, none deferred)
- [ ] `/ze-review` gate clean (Review Gate section filled: 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-standard-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] QEMU integration test for the linux-only netns leg (`ai/rules/platform-linux.md`)
- [ ] `make ze-tier-check` passes (core import direction)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`, including A-3 and A-12)

### Quality Gates (SHOULD pass, defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING, before ANY commit)
- [ ] Critical Review passes: all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only

## Review Gate

<!-- Filled by /ze-implement's /ze-review gate before closure. Not started: this spec is design. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | | (not started) | | |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

<!-- Filled at completion. Not started: this spec is design. -->

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| (not started) | | |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| (not started) | | |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| (not started) | | |
