# spec-fixit-vpp-lcp-reachability

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - (shares one file, `internal/plugins/iface/vpp/doctor.go`, with `plan/spec-bgp-netns.md`; neither blocks the other) |
| Phase | 1/1 (doctor check; research done) |
| Updated | 2026-07-17 |

Anchor refresh (2026-07-22 plan review, design unchanged and implementable;
the live citation below is updated in-body): `internal/core/diagnostic/codes.go`
registrations drifted -- `doctor-vpp-wireguard` 289 -> 301,
`doctor-vpp-lcp-netns` 295 -> 307. Also verified: `spec-bgp-netns`'s
AC-3 narrowing has NOT landed (`checkVPPLCPNetns` still root-reachable at
`doctor.go:120,157`), so the shared-file coordination note still holds.

~~**DESIGN, NOT APPROVED.** Research is done: the Open Questions below are answered and the
assumption table carries verdicts. Two assumptions came back BROKEN (A-2, A-6) and they
change the shape of the work, so this spec must NOT move to `ready` until Thomas rules on
~~the split question (A-8) and~~ ~~the BGP netns config surface~~ Q10 (Warning vs Error).
Do not implement from this file yet.~~

→ AUTONOMOUS DEFAULT (2026-07-17): **PROMOTED TO `ready`.** The one stated blocker was Q10
(Warning vs Error); it is now resolved to **Warning** (see Q10 below and Phase 1). Every other
open item is either answered by research (A-2/A-6 broken but resolved; A-4/A-5 confirmed) or
belongs to `plan/spec-bgp-netns.md` (Q3/Q4/Q5). Nothing technical remains for the doctor half.
Rationale: this is the readiness loop's autonomous default per the conservative-default protocol
(a severity call is a scope/impact judgement, resolved to the lower-risk Warning). Thomas: override
the promotion or the severity if wrong.

**2026-07-16, Thomas: the split question (A-8) is DECIDED -- SPLIT. THE SPLIT IS NOW DONE.**
This spec keeps **Problem B only**: the LCP-presence doctor check (one check, one code, one
`.ci`). **Problem A** (netns-aware BGP listening) has MOVED to **`plan/spec-bgp-netns.md`**,
created 2026-07-16. Its ACs (AC-1, AC-2, AC-3, AC-8, AC-9, AC-10) and Phases 2-6 moved with
it and are NOT duplicated here: this file keeps a pointer only. Read Task ->
"The 2026-07-16 split" before touching anything here.

**What remains in THIS spec, and it can proceed:** the LCP plugin-presence doctor check. It is
bounded (one check registered at Order 742, one `doctor-vpp-lcp-plugin` code, one
`test/ui/doctor-vpp-lcp-plugin.ci`), needs no config surface, no reactor change, and no QEMU
rail, and it depends on **nothing** that is still open. Every technical question it had is
answered (A-4 and A-5 CONFIRMED; A-6 BROKEN but resolved: the probe opens its own connector).
The only open item is Q10, a severity wording call, which is not a blocker.
~~**Status stays `design`; promotion to `ready` is Thomas's gate and has not been given.**~~
→ AUTONOMOUS DEFAULT (2026-07-17): **Status is now `ready`.** Q10 resolved to Warning (below).
Thomas: override if the promotion or the severity is wrong.

-> Decision (user, 2026-07-16): **A-7 is ANSWERED -- yes, support a non-root netns, because it
IS the default and the documented model.** This does not change this spec's scope, but it
retires the constraint below that said the netns half starts life BLOCKED. It does not. See
`plan/spec-bgp-netns.md`.

-> Decision (user, 2026-07-16), **CORRECTION, same day.** ~~A-7 is ANSWERED -- "Both: fix the
default now, keep netns as real work", so a THIRD spec,
`plan/spec-fixit-vpp-lcp-netns-default.md` (another agent), now owns fixing the default.~~
**SUPERSEDED. That answer rested on a FALSE premise and is withdrawn.** The premise was that
the `vpp.lcp.netns` default of `"dataplane"` is a mistake contradicting its own design intent,
argued from the code comment at `lcp.go:105-108`. It is not a mistake:

| Correction | Evidence (verified 2026-07-16) |
|-----------|-------------------------------|
| The default is DELIBERATE and STAYS | -> Decision (user, 2026-07-16). `plan/deferrals.md:36` (2026-07-10) records the real intent: make BGP netns-aware "so LCP TAPs in a non-root netns are reachable by BGP **without forcing the operator to a root-reachable netns**" |
| `"dataplane"` is IPng Networks' production convention, copied on purpose | `54bffb83b` ("startup.conf generator **following IPng production template**"); `docs/research/vpp-deployment-reference.md:179-180` ("isolates the forwarding plane from the management plane") |
| **`plan/spec-fixit-vpp-lcp-netns-default.md` was NEVER CREATED and must NOT be** | No such file. Every reference to it in this spec is struck through below, not deleted |
| `plan/spec-bgp-netns.md` is PRIORITY-RAISED, not deprioritized | It is the fix for the DEFAULT case, not a niche capability. See its Task -> "Why this is the default case" |

-> Constraint: the method error is the durable lesson. A code comment states what its author
believed; it is not a decision record. The recorded decision was in `plan/deferrals.md` the
whole time. See `ai/rules/fail-closed-guards.md` "Evidence corollary".

## Task

VPP's Linux Control Plane (LCP) creates TAP mirrors of VPP interfaces so routing daemons can
use Linux TCP on VPP-managed NICs. Two gaps stop that from working end to end. First, the
TAPs land in the namespace named by `vpp.lcp.netns` (default `"dataplane"`), but ze's BGP
listener has zero network-namespace awareness, so it cannot bind on them unless the operator
moves LCP to a root-reachable namespace. Second, when the VPP build lacks
`linux_cp_plugin.so`, nothing warns pre-apply: the whole config apply fails at the binapi
layer with a raw VPP error. Goal: VPP LCP interfaces are actually usable by BGP, and LCP
misconfiguration is diagnosed by `ze doctor` before apply.

~~Group rationale: both are about making VPP LCP interfaces reachable and diagnosable. They
are grouped for tracking, not because they share a root cause. Problem A is a BGP listener
capability; Problem B is a doctor check. DESIGN may split them.~~

**SUPERSEDED. -> Decision (user, 2026-07-16): SPLIT the spec.** DESIGN asked; Thomas ruled.
The two problems are **unrelated problems sharing a filename**, not one task. The grouping
("reachable and diagnosable") was a filing convenience and is withdrawn as a rationale. The
spec's own words above already conceded the point: grouped "for tracking, not because they
share a root cause".

### The 2026-07-16 split: what each half becomes

| Half | Becomes | Scope | Blocked on |
|------|---------|-------|-----------|
| **Problem B -- LCP-presence doctor check** | **THIS spec** (`plan/spec-fixit-vpp-lcp-reachability.md`), narrowed to Problem B only | One check (`checkVPPLCPPlugin`, registered beside the existing two at `internal/plugins/iface/vpp/doctor.go:27-56`, verified 2026-07-16: Order 740 `vpp-wireguard-plugin` / 741 `vpp-lcp-netns`, so the new one takes Order 742), one diagnostic code (`doctor-vpp-lcp-plugin` in `internal/core/diagnostic/codes.go`), one functional test (`test/ui/doctor-vpp-lcp-plugin.ci`). No config surface. No reactor change. No QEMU rail. **Ships now.** Keeps AC-4, AC-5, AC-6, AC-7, AC-11 and Phase 1 | Nothing technical. Q10 (Warning vs Error) is a wording/severity call, not a blocker |
| **Problem A -- netns-aware BGP listening** | **`plan/spec-bgp-netns.md`, CREATED 2026-07-16.** The name the deferrals row anticipated | The `RealListenerFactory.Netns` field + `netns_linux.go`/`netns_other.go`; the reactor change threading netns through BOTH branches of `newListenerFactory` (`reactor.go:1378-1385`, re-verified at the producer 2026-07-16: the MD5/GTSM branch returns a fresh `RealListenerFactory{MD5Peers, ListenTTL}` and discards `r.listenerFactory`); a BGP netns config surface; the A-3 kernel-semantics prototype; the AC-3 narrowing of `checkVPPLCPNetns`; the QEMU rail. **Took AC-1, AC-2, AC-3, AC-8, AC-9, AC-10 and Phases 2-6 -- they live THERE now, not here** | **Nothing.** A-7 is answered (see below). Its remaining gates are Thomas's `ready` promotion and Q3 (config-surface shape) |

-> Decision (user, 2026-07-16): **A-7 ANSWERED -- yes, support a non-root netns, because it IS
the default and the documented model.** ~~The netns half still depends on Thomas's unanswered
A-7.~~ **SUPERSEDED. The netns half is NOT blocked and does NOT start life blocked.** The answer
arrived after A-7 was REFRAMED, and the reframing is the load-bearing part, recorded here and in
full in `plan/spec-bgp-netns.md`: A-7 asked "is a non-root LCP netns worth supporting at all?",
which implies a niche opt-in. The code says the opposite. `ze-vpp-conf.yang:199-201` defaults
`netns` to `"dataplane"`; `lcpNetnsIsRootReachable` (`doctor.go:136-143`) accepts ONLY `""`,
`"host"`, `"root"`, so `"dataplane"` is not root-reachable; `lcpPairNetns` (`lcp.go:109-114`)
passes it through; BGP has zero netns awareness (`network.go:167-194`). **The DEFAULT config
puts LCP TAPs where BGP cannot bind.**

~~That contradicts the intent stated in the comment directly above the code (`lcp.go:105-108`:
root-reachable is where "ze's BGP listener runs"; another name isolates "deliberately"). So the
question split in two, and Thomas answered both halves:~~

~~| Half of the answer | Owner | Note |~~
~~| Fix the DEFAULT now | `plan/spec-fixit-vpp-lcp-netns-default.md` (another agent, concurrent) |~~
~~| Keep netns-aware BGP as real, approved work | `plan/spec-bgp-netns.md` | its PRIORITY drops once the default lands |~~

**SUPERSEDED, 2026-07-16 (same day). The "contradicts its own design intent" claim was FALSE.**
The default putting TAPs beyond BGP's reach is real; the inference that this made the default a
BUG was not. -> Decision (user, 2026-07-16): **the default STAYS `"dataplane"`**, and the
question does NOT split in two:

| Correction | Evidence (verified 2026-07-16) |
|-----------|-------------------------------|
| The recorded design intent is the OPPOSITE of the comment-derived reading: BGP should follow the TAP, the TAP should not be dragged to BGP | `plan/deferrals.md:36` (2026-07-10, this work's origin row): teach the BGP listener to bind in a named netns "so LCP TAPs in a non-root netns are reachable by BGP **without forcing the operator to a root-reachable netns**" |
| `"dataplane"` is IPng's production convention, adopted deliberately | `54bffb83b` ("following IPng production template"); `docs/research/vpp-deployment-reference.md:179-180`: LCP TAPs live in `dataplane`, which "isolates the forwarding plane from the management plane" |
| There is ONE owner, not two | **`plan/spec-fixit-vpp-lcp-netns-default.md` was never created and must not be.** `plan/spec-bgp-netns.md` owns the whole answer, and its priority is RAISED: it makes the shipped default work |

-> Constraint: Q7 ("should the `vpp.lcp.netns` default stay dataplane?") is **ANSWERED: YES, it
stays.** ~~It moved with the answer to `plan/spec-fixit-vpp-lcp-netns-default.md`.~~ It is
CLOSED, not moved. Nothing changes the default.

-> Constraint: the split is filing, not scope reduction. No AC is dropped; each is assigned
to exactly one half in the table above. AC-3 (the fate of the `doctor-vpp-lcp-netns` warning)
travels with the **netns** half, because its premise -- "BGP can now bind in the LCP netns" --
only becomes true if Problem A lands. Until then the existing warning stays correct as
shipped and must not be narrowed.

-> Decision (user, 2026-07-16): **the missing `test/ui/doctor-vpp-lcp-netns.ci` ships with the
DOCTOR half -- THIS spec.** ~~This recording assigns it to the netns half to avoid writing a
`.ci` that AC-3 immediately rewrites.~~ SUPERSEDED by Thomas's ruling. It covers the check that
exists TODAY (`checkVPPLCPNetns`, `doctor.go:100-131`, confirmed absent by `ls`, Q11), and a
delivered check with no functional test is a gap now, not a gap contingent on a spec that may
never be promoted. **Recorded and accepted: `spec-bgp-netns`'s AC-3 narrows `checkVPPLCPNetns`
and will REWRITE this `.ci`.** That is the accepted cost, not a defect. The alternative was
leaving a shipped check untested indefinitely.

## Origin

Both from `plan/deferrals.md` rows dated 2026-07-10, source `spec-followup-vpp-iface`:
- Problem A: the "spec-followup-vpp-iface A-4" row (BGP netns-aware listener). Destination
  recorded as "none yet (future `spec-bgp-netns` when picked up)".
- Problem B: the "spec-followup-vpp-iface" row (doctor check for VPP linux-cp plugin
  presence). Destination recorded as "none yet (future doctor follow-up)".

## Required Reading

### Source (read before designing)

- [ ] `internal/component/bgp/reactor/listener.go` - lines 32-46: the `Listener` struct holds
      `addr string` and `listenerFactory network.ListenerFactory`, nothing namespace-related
  → Constraint: CONFIRMED no netns awareness. A grep for netns/namespace across
    `internal/component/bgp/reactor/` returns only EVENT-namespace hits (`bgpevents.Namespace`),
    never a network namespace. The deferrals row's claim holds.
- [ ] `internal/component/bgp/reactor/listener.go` - line 108: `StartWithContext` calls
      `l.listenerFactory.Listen(ctx, "tcp", l.addr)`; `SetListenerFactory` at lines 66-68
  → ~~Decision: an injection seam ALREADY exists. A netns-aware listener is plausibly a new
    `ListenerFactory` implementation rather than surgery on `Listener` itself.~~
    SUPERSEDED 2026-07-16: the seam exists but is NOT reachable from outside, and it is
    conditionally discarded. See the `reactor.go` rows below. A-2 is BROKEN.
- [ ] `internal/component/bgp/reactor/reactor.go` - lines 1001-1004 (global listener) and
      1399-1402 (per-address/port listener): BOTH call `NewListener(...)` and then
      immediately `SetListenerFactory(r.newListenerFactory(port))`, overwriting the default
      set by `NewListener` (`listener.go:54`)
  → Constraint: `Listener.SetListenerFactory` is an INTERNAL seam only. No production caller
    outside the reactor can inject a listener factory into a `Listener`; the reactor
    overwrites it unconditionally right after construction. The only external seam is
    `Reactor.SetListenerFactory` (`reactor.go:553`), which sets `r.listenerFactory`.
- [ ] `internal/component/bgp/reactor/reactor.go` - lines 1378-1385: `newListenerFactory` is
      the PRODUCER of every production listener factory
  → Decision: THE KEYSTONE FINDING. When `md5PeersForListener(port)` is non-empty or
    `listenTTLForListener(port)` is non-zero, `newListenerFactory` returns a FRESH
    `network.RealListenerFactory{MD5Peers: ..., ListenTTL: ...}` and DISCARDS
    `r.listenerFactory` entirely. Only the no-MD5/no-GTSM branch returns the injected
    factory. So a netns-aware factory injected via `Reactor.SetListenerFactory` would be
    silently dropped exactly on MD5-authenticated or GTSM peers -- a common BGP config, and
    the failure would be a silent bind in the WRONG namespace, not an error.
  → Constraint: netns must COMPOSE with MD5 and GTSM, not compete with them. A separate
    wrapping `ListenerFactory` implementation cannot compose here; the factory is chosen
    either/or, not stacked. This drives the "field, not wrapper" decision below.
- [ ] `internal/core/network/network.go` - lines 147-162: `RealListenerFactory` already
      carries `MD5Peers` and `ListenTTL`, two per-bind socket-level concerns, as FIELDS
  → Decision: netns is the same KIND of thing (a property of how the listening socket is
    created) and belongs as a third field `Netns string`, not as a rival factory type. This
    is the only shape that composes with MD5/GTSM through `newListenerFactory`.
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
    new idiom -- it reuses this lock/get/set/restore shape.
  → Constraint: the file header records that the locked thread's namespace is inherited by
    fork+exec'd children (A-5 there, validated by `TestNetnsLaunchChildInheritsNamespace`).
    That is evidence that namespace membership attaches at object-creation time, which
    supports (but does not prove) "pin only around bind". See A-3.
- [ ] `internal/core/network/network.go` - lines 32-40: the `ListenerFactory` interface;
      `RealListenerFactory` at lines 147-149 with `Listen` at line 167 using `net.ListenConfig`
  → Constraint: this is the seam every listener goes through. A netns variant belongs here
    or beside it, not inline in the reactor.
- [ ] `internal/plugins/iface/vpp/doctor.go` - lines 100-131: `checkVPPLCPNetns`, the
      DELIVERED netns doctor check (code `doctor-vpp-lcp-netns`, `SeverityWarning` at line 128)
  → Constraint: this check is the "meanwhile" mitigation named by the deferrals row. If
    Problem A is fixed, this check's premise changes and it must be revisited, not left.
- [ ] `internal/plugins/iface/vpp/doctor.go` - lines 136-143: `lcpNetnsIsRootReachable`
      treats only "", "host", "root" as root-reachable
  → Constraint: the YANG default `"dataplane"` is NOT in this set, so the default config
    warns. Confirm this is intended before designing.
- [ ] `internal/plugins/iface/vpp/doctor.go` - lines 62-85: `checkVPPWireguardPlugin`, the
      check Problem B is told to parallel; `wireguardPluginEnabled` at lines 88-95
  → Constraint: it is a CONFIG-ONLY check (reads the config tree for the
    `vpp.plugins.wireguard` toggle). It never probes the running VPP. The LCP case has no
    equivalent toggle, so "parallel to doctor-vpp-wireguard" is true in INTENT only, not in
    mechanism.
- [ ] `internal/plugins/iface/vpp/doctor.go` - lines 27-56: `registerDoctorChecks`, both
      checks at `DoctorPhasePostConfig`, Order 740 and 741, Component "vpp"
  → Constraint: a third check follows this registration shape and travels with the plugin
    (`ai/rules/doctor-checks.md`).
- [ ] `internal/component/vpp/startupconf.go` - lines 82-91: `linux_cp_plugin.so` and
      `linux_nl_plugin.so` are enabled in startup.conf whenever `s.LCP.Enabled`
  → Constraint: ze already ASKS for the plugin. The gap is a build that lacks the `.so`,
    which no config-level check can see. This is why a runtime probe is required.
- [ ] `internal/plugins/iface/vpp/lcp.go` - lines 87-102: `lcpItfPair` issues
      `LcpItfPairAddDel`; failure surfaces at line 97 (error) or line 100 (retval)
  → Constraint: this is the "binapi layer" where the whole apply fails today. The doctor
    check must fire BEFORE this.
- [ ] `internal/plugins/iface/vpp/lcp.go` - lines 105-115: `lcpPairNetns` maps a
      root-reachable name to "" (VPP's host namespace) and passes any other name through
  → Decision: the existing product already bends toward "put the TAP where BGP can reach
    it". Problem A is the other half of that same decision.
- [ ] `internal/component/doctor/checks_linux.go` - line 251: `checkVPPVersion` runs
      `vppctl show version` (line 266) with a 3s timeout
  → Decision: a runtime VPP probe precedent EXISTS, but via `vppctl` exec from the CENTRAL
    doctor package, not GoVPP from the owning plugin. The deferrals row asks for GoVPP.
    That tension is a real design question, not a detail.
  → Constraint (2026-07-16): the reason it uses `vppctl` is now understood. `ze doctor` is a
    SEPARATE PROCESS from the daemon (see the register.go row), so it has no in-process VPP
    connector to borrow. `vppctl` works cross-process because it talks to VPP's CLI socket.
    Any GoVPP probe must likewise open its OWN connection. This is a process-boundary fact,
    not a stylistic preference.
- [ ] `internal/component/doctor/register.go` - lines 13-21: `registry.RegisterRoot("doctor",
      registry.Meta{Mode: "offline", ...})` and `registry.MustRegisterLocalMeta("doctor", Run,
      ...)`; `Run` (`doctor/doctor.go:31`) parses args and calls `runChecks(configPath)`
      (`doctor.go:68`), which builds only a storage handle and a platform value
  → Decision: A-6 IS BROKEN. Doctor is an OFFLINE, LOCAL command: it runs inside the
    `ze doctor` CLI process, not inside the daemon. Nothing in `runChecks` starts a VPP
    Manager.
- [ ] `internal/component/vpp/vpp.go` - lines 67-80: `GetActiveConnector` returns
      `connectorRef`, a package-level var written ONLY by `setActiveConnector`, which the VPP
      Manager calls from its `Run` path (`vpp.go:157` constructs `NewConnector(settings.APISocket)`)
  → Constraint: THE PRODUCER READ THAT KILLS THE SPEC'S OWN PROPOSAL. `connectorRef` is
    process-local state set by the running daemon. In the `ze doctor` process the Manager
    never runs, so `GetActiveConnector()` returns nil ALWAYS. The spec's Integration Points
    entry naming `vppcomp.GetActiveConnector` as "a candidate for the GoVPP probe" is WRONG
    and is corrected below. `GetActiveLCPSettings` (`vpp.go:99`) is nil/false in doctor for
    the same reason, which is why `checkVPPLCPNetns` reads the CONFIG TREE, not the Manager.
- [ ] `go.fd.io/govpp@v0.13.0/api/api.go` - line 109: the `api.Channel` interface includes
      `CheckCompatiblity(msgs ...Message) error`; implemented at `core/channel.go:184-200`,
      which calls `msgIdentifier.GetMessageID(msg)` per message and collects an
      `adapter.UnknownMsgError` into `api.CompatibilityError.IncompatibleMessages`
  → Decision: this IS the GoVPP plugin-presence probe, and it resolves A-5 in GoVPP's
    favour. A message registered only by `linux_cp_plugin.so` cannot resolve a message ID on
    a VPP that did not load the plugin, so `CheckCompatiblity` returns a `CompatibilityError`
    naming it. `Connector.NewChannel()` (`internal/component/vpp/conn.go:93`) returns
    `api.Channel`, so the method is already reachable through ze's own type.
- [ ] `go.fd.io/govpp@v0.13.0/binapi/lcp/lcp.ba.go` - line 63: `LcpDefaultNsGet` is an empty,
      read-only LCP request (`LcpDefaultNsGetReply` at line 94)
  → Decision: probe with `&lcp.LcpDefaultNsGet{}`. It is side-effect free (a getter), unlike
    `LcpItfPairAddDel`, so the probe cannot mutate dataplane state. `CheckCompatiblity` does
    not even SEND it -- it only resolves the message ID -- so the probe is a pure lookup.
- [ ] `internal/component/vpp/yang/ze-vpp-conf.yang` - lines 39-43: `leaf api-socket`,
      `default "/run/vpp/api.sock"`
  → Constraint: the doctor probe gets its socket path from the CONFIG TREE (the same tree the
    check already reads), then opens its own `vppcomp.NewConnector(apiSocket)`. No daemon
    state is needed, so the check stays honest in the offline doctor process.
- [ ] `internal/component/vpp/yang/ze-vpp-conf.yang` - lines 171-205: the `lcp` container;
      `enabled` defaults true (lines 176-181), `netns` defaults "dataplane" (lines 198-205)
  → Constraint: LCP is ON by default and its netns default is NOT root-reachable. Any check
    keyed on `lcp.enabled` fires for essentially every VPP deployment.

### Architecture Docs

- [ ] `ai/rules/doctor-checks.md` - cited by `doctor.go:1` as the design rule for
      self-contained checks owned by the plugin that owns the runtime dependency
  → Constraint: the new check belongs in `internal/plugins/iface/vpp/`, and a new code must
    be registered in `internal/core/diagnostic/codes.go`.
- [ ] `plan/learned/1098-followup-vpp-iface.md` - the learned summary of the spec both rows
      came from
  → Decision: read for why A-4 was deferred rather than solved.
- [ ] `docs/guide/vpp.md` - the operator-facing VPP guide (mentions the doctor codes)
  → Constraint: update if the netns constraint or the diagnosis changes.
- [ ] `ai/rules/qemu-testing.md` - linux-only code needs QEMU integration tests
  → Constraint: a netns-binding listener is linux-only and MUST be QEMU-tested; "needs
    hardware" is not an accepted skip.
- [ ] `ai/rules/plugin-self-containment.md` - constrains where the check and any netns code live
  → Constraint: no plugin spelling in generic packages.

**Key insights (rewritten 2026-07-16 after research):**
- ~~The BGP listener already has a `ListenerFactory` injection seam; netns support likely
  belongs there, not in the reactor.~~ SUPERSEDED. The seam is real but the reactor
  overwrites it (`reactor.go:1004`, `:1402`) and `newListenerFactory` (`reactor.go:1378-1385`)
  DISCARDS the injected factory whenever MD5 or GTSM TTL applies to the port. Netns must be a
  FIELD on `RealListenerFactory`, composing with `MD5Peers` and `ListenTTL`, not a rival
  factory type. A-2 is BROKEN: the reactor does need a (small) change.
- The existing `doctor-vpp-wireguard` check is config-only; the LCP plugin-presence check
  cannot copy it, because LCP has no config toggle to read. It needs a runtime probe.
- ~~The only runtime VPP probe precedent uses `vppctl` exec from the central doctor package,
  which conflicts with both the "GoVPP probe" phrasing and plugin self-containment.~~
  REFINED. The tension dissolves: `api.Channel.CheckCompatiblity` (govpp `api/api.go:109`)
  resolves a message ID against the connected VPP and reports `UnknownMsgError` as a
  `CompatibilityError`. Probing `&lcp.LcpDefaultNsGet{}` detects plugin absence with no exec,
  no text parsing, and no side effects, from inside the owning plugin. GoVPP wins on merit.
- The real constraint on the probe is a PROCESS boundary, not a mechanism choice: `ze doctor`
  is an offline local command (`doctor/register.go:13-21`), so `vppcomp.GetActiveConnector()`
  (`vpp.go:75`) is nil there. The probe must open its own short-lived connector from the
  config's `vpp.api-socket` (`ze-vpp-conf.yang:39-43`). A-6 is BROKEN as written.
- `vpp.lcp.netns` defaults to "dataplane", which is NOT root-reachable, so the shipped
  default already trips the netns warning.
- `github.com/vishvananda/netns v0.0.5` is ALREADY a direct dependency (`go.mod:23`) and
  `internal/test/runner/netns_linux.go` is an in-tree netns idiom to copy. The netns leg
  needs no new dependency.
- The two halves have very different costs. Problem B (doctor) is bounded, host-testable, and
  shippable on its own. Problem A (netns) needs a new config surface, thread pinning, a QEMU
  rail, and a decision about every OTHER listener. See A-8: recommend a SPLIT.

## Current Behavior (MANDATORY)

**Source files read:**

- [ ] `internal/component/bgp/reactor/listener.go` - lines 50-56: `NewListener(addr)` stores
      the address and a `network.RealListenerFactory{}`. Line 108: `StartWithContext` calls
      `l.listenerFactory.Listen(ctx, "tcp", l.addr)`. Nothing anywhere in the file, or in
      `internal/component/bgp/reactor/`, references a network namespace.
- [ ] `internal/component/bgp/reactor/reactor.go` - lines 1378-1385: `newListenerFactory(port)`
      is the producer of every production listener factory. It reads `md5PeersForListener(port)`
      and `listenTTLForListener(port)`; if EITHER is set it returns a brand-new
      `network.RealListenerFactory{MD5Peers: md5Peers, ListenTTL: listenTTL}`, ignoring
      `r.listenerFactory`. Otherwise it returns `r.listenerFactory` (the chaos/test injection).
      There is no third branch and no composition.
- [ ] `internal/component/bgp/reactor/reactor.go` - lines 1001-1004: the global listener is
      built with `NewListener(r.config.ListenAddr)` then `SetListenerFactory(r.newListenerFactory(r.config.Port))`.
      Lines 1399-1402: `startListenerForAddressPort` does the same per address/port. Both
      overwrite whatever `NewListener` defaulted to.
- [ ] `internal/component/bgp/reactor/reactor.go` - line 553: `Reactor.SetListenerFactory(f)`
      assigns `r.listenerFactory`. Production callers: `bgp/config/loader.go:193`,
      `bgp/config/loader_create.go:251`, `bgp/cli/childmode.go:278`, and
      `internal/chaos/inprocess/runner.go:169`. Every one of them is defeated by the
      MD5/GTSM branch above when the port carries MD5 or GTSM.
- [ ] `internal/core/network/network.go` - lines 147-167: `RealListenerFactory.Listen` binds
      via `net.ListenConfig` in whatever namespace the calling thread happens to be in. Its
      `MD5Peers` and `ListenTTL` fields are applied through `lc.Control` (lines 172-192), a
      callback the kernel runs on the already-created fd BEFORE bind.
- [ ] `internal/core/network/` file listing: `md5_darwin.go`, `md5_freebsd.go`, `md5_linux.go`,
      `md5_other.go`, `ttl_linux.go`, `ttl_other.go`, `ttl.go`, `network.go`. The package's
      established shape is: a portable field on the factory + a build-tagged helper per OS.
- [ ] `internal/test/runner/netns_linux.go` - `enterTestNetns(name)` locks the OS thread,
      `netns.Get()` for the original handle, `netns.NewNamed(name)` to enter, brings `lo` up,
      and returns a `restore()` closure that re-enters the original namespace and unlocks the
      thread. `netns_other.go` (`//go:build !linux`) stubs it with `errNetnsUnsupported`.
- [ ] `internal/core/routewatch/routewatch_linux.go` - lines 12-38: a second in-tree consumer
      of `github.com/vishvananda/netns`, holding an `netns.NsHandle` in `platformState` and
      defaulting to `netns.None()`. Precedent that `internal/core/` already imports this
      library directly.
- [ ] `internal/plugins/iface/vpp/doctor.go` - lines 100-131: `checkVPPLCPNetns` warns when
      `bgp` is configured, `vpp/lcp` exists, `enabled` is not "false", and the netns is not
      root-reachable. Emits `doctor-vpp-lcp-netns` at `SeverityWarning`.
- [ ] `internal/plugins/iface/vpp/doctor.go` - lines 62-85: `checkVPPWireguardPlugin` emits
      `doctor-vpp-wireguard` at `SeverityError` when a wireguard interface is configured
      under backend vpp but the `vpp.plugins.wireguard` toggle is off. Config-only.
- [ ] `internal/plugins/iface/vpp/lcp.go` - lines 87-102: `lcpItfPair` sends
      `LcpItfPairAddDel` and returns a raw error (line 97) or a retval error (line 100) when
      the plugin is not loaded in VPP. Nothing catches this earlier.
- [ ] `internal/component/vpp/startupconf.go` - lines 82-91: startup.conf enables
      `linux_cp_plugin.so` and `linux_nl_plugin.so` whenever LCP is enabled, on top of
      `plugin default { disable }`.
- [ ] `internal/component/doctor/checks_linux.go` - line 251: `checkVPPVersion` shows the
      only runtime VPP probe pattern in the tree: `vppctl show version` via `exec` (line 266),
      3s timeout (line 264), degrading to `SeverityWarning` "cannot determine VPP version"
      when the exec fails (lines 268-274).
- [ ] `internal/component/doctor/register.go` - lines 13-21: doctor registers as a root
      command with `Mode: "offline"` plus a LOCAL handler (`MustRegisterLocalMeta("doctor",
      Run, ...)`), and registers `runChecks` as the diagnostic provider.
- [ ] `internal/component/doctor/doctor.go` - line 31 `Run(args []string) int` parses
      `--json` and a config path, then calls `runChecks(configPath)` (line 68). `runChecks`
      resolves storage (line 69) and platform (line 82), then runs phase checks (line 89). It
      never constructs a VPP Manager or connector.
- [ ] `internal/component/vpp/vpp.go` - lines 67-80: `GetActiveConnector()` returns the
      package-level `connectorRef`, set only by `setActiveConnector` from the Manager's Run
      path. Lines 91-102: `GetActiveLCPSettings()` returns `activeLCP, activeLCPSet`, both
      set only alongside the connector. In a process where the Manager never ran, these are
      nil / false.
- [ ] `internal/component/vpp/conn.go` - line 37 `NewConnector(apiSocket string) *Connector`;
      line 44 `Connect(ctx, maxAttempts, retryInterval) error`; line 93 `NewChannel() (api.Channel, error)`;
      line 154 `Close()`. A caller with only a socket path can build and drive its own connector.
- [ ] `internal/component/vpp/yang/ze-vpp-conf.yang` - lines 39-43: `leaf api-socket`,
      `type string`, `default "/run/vpp/api.sock"`. Lines 171-205: the `lcp` container with
      `enabled` (default true, lines 177-181) and `netns` (default "dataplane", lines 198-205).
- [ ] `go.fd.io/govpp@v0.13.0/api/api.go:109` - `api.Channel` declares
      `CheckCompatiblity(msgs ...Message) error`. `core/channel.go:184-200` implements it:
      per message it calls `GetMessageID`, and on `*adapter.UnknownMsgError` appends to
      `CompatibilityError.IncompatibleMessages`; it returns nil only when none are incompatible.
- [ ] `go.fd.io/govpp@v0.13.0/binapi/lcp/lcp.ba.go:63` - `LcpDefaultNsGet struct{}`, an empty
      read-only request supplied by the linux_cp plugin's API.
- [ ] `internal/core/diagnostic/codes.go` - lines 288-299: the registry rows for
      `doctor-vpp-wireguard` and `doctor-vpp-lcp-netns`.
- [ ] `test/ui/` listing: `doctor-vpp-wireguard.ci` EXISTS; `doctor-vpp-lcp-netns.ci` DOES
      NOT (`ls` returns "No such file or directory"). This answers the Open Question: the
      delivered netns check has a unit test but no functional `.ci`.
- [ ] `plan/learned/1098-followup-vpp-iface.md` - Gotchas, lines 73-82: "LCP enabled + no
      linux_cp_plugin.so = whole-apply failure. `ligato/vpp-base` ships `wireguard_plugin.so`
      but NOT `linux_cp_plugin.so`/`linux_nl_plugin.so` ... the binapi call returns
      **'unknown message'** and, by exact-or-reject, fails the entire interface config apply
      (ze exits at startup)." Also: the evidence script already "gates on `show plugins`".

→ Verification (2026-07-17, readiness pass): every in-scope Problem B citation was re-read
against source and CONFIRMED real. Exact today: `registerDoctorChecks` (`doctor.go:27-56`,
Order 740 `vpp-wireguard-plugin` / 741 `vpp-lcp-netns`, so the new check takes Order 742);
`checkVPPWireguardPlugin` (`doctor.go:62-85`) / `wireguardPluginEnabled` (`doctor.go:88-95`);
`codes.go` rows (`doctor-vpp-wireguard` line 301, `doctor-vpp-lcp-netns` line 307);
`api.Channel.CheckCompatiblity` (govpp `api/api.go:109`, impl `core/channel.go:184-200`);
`LcpDefaultNsGet` (`binapi/lcp/lcp.ba.go:63`); `NewConnector`/`Connect`/`NewChannel`/`Close`
(`conn.go:36/44/93/154`); `GetActiveConnector` (`vpp.go:75`); doctor offline/local
(`register.go:13-21`); `api-socket` (`ze-vpp-conf.yang:39`); `lcp` container (`:171`, `enabled`
`:177`, `netns` `:199`); `linux_cp_plugin.so` (`startupconf.go:83-87`); `LcpItfPairAddDel`
(`lcp.go:88-100`). LINE DRIFT to respect when implementing (behavior unchanged, numbers grew as
comments were added): `checkVPPLCPNetns` now spans `doctor.go:100-141` (SeverityWarning at line
138, not 128); `lcpNetnsIsRootReachable` now `doctor.go:157-164` (spec cites 136-143);
`lcpPairNetns` now `lcp.go:119` (spec cites 105-115). Also `test/ui/doctor-vpp-socket.ci` now
exists alongside `doctor-vpp-wireguard.ci` (Q11 said only wireguard); `doctor-vpp-lcp-netns.ci`
and `doctor-vpp-lcp-plugin.ci` remain ABSENT, as this spec requires them created.

**Behavior to preserve:**

- The `doctor-vpp-lcp-netns` warning while BGP cannot bind in a non-root netns
  (`doctor.go:100-131`); it is the only thing making the constraint visible today.
- `lcpPairNetns` mapping host/root/empty to "" so VPP places the TAP in its host namespace
  (`lcp.go:109-115`), and passing any other name through deliberately.
- The `doctor-vpp-wireguard` check's current config-toggle behavior (`doctor.go:62-85`).
- Doctor check registration shape and ordering (`doctor.go:27-56`, Order 740/741).
- BGP listener behavior for every non-LCP deployment: default binding must not change.

**Behavior to change (settled by research 2026-07-16, pending Thomas's approval):**

| # | Change | Producer that must change | Why |
|---|--------|--------------------------|-----|
| 1 | `RealListenerFactory` gains a `Netns string` field; when non-empty, `Listen` creates the socket inside that named namespace | `internal/core/network/network.go:147-167` + new `netns_linux.go` / `netns_other.go` | The only shape that composes with `MD5Peers` / `ListenTTL` through `newListenerFactory` |
| 2 | `newListenerFactory` carries the netns into BOTH branches, and the MD5/GTSM branch stops discarding configured state | `internal/component/bgp/reactor/reactor.go:1378-1385` | Today the MD5/GTSM branch drops `r.listenerFactory` entirely (A-2 BROKEN). Without this, netns silently does nothing for MD5/GTSM peers |
| 3 | BGP gains a listener namespace config surface (a generic netns leaf, no VPP spelling) | `internal/component/bgp/yang/`, `internal/component/bgp/reactor/config.go` | The netns value must reach `newListenerFactory`. Requires Thomas's ruling: see A-7 / Open Questions |
| 4 | New `checkVPPLCPPlugin` doctor check + `doctor-vpp-lcp-plugin` code | `internal/plugins/iface/vpp/doctor.go:27-56`, `internal/core/diagnostic/codes.go:288-299` | Problem B |
| 5 | `checkVPPLCPNetns` NARROWS from "netns is not root-reachable" to "vpp.lcp.netns and the BGP listener netns disagree" | `internal/plugins/iface/vpp/doctor.go:100-143` | AC-3. Once BGP can bind in a namespace, the current warning is stale; the real hazard becomes a MISMATCH, not non-root-ness |
| 6 | Non-scope, flagged: the `doctor-vpp-wireguard` description over-claims a runtime check it does not do | `internal/core/diagnostic/codes.go:291` | See Design Insights; needs Thomas's ruling on whether to fix here or separately |

→ Decision: the fork "teach BGP netns" vs "force LCP into the host netns" is resolved in
  favour of TEACHING BGP, because `lcpPairNetns` (`lcp.go:109-115`) already deliberately
  passes a non-root netns through "so the operator can isolate the TAP deliberately". The
  product has already chosen that operators may isolate; removing the capability would be a
  regression, and the YANG default is "dataplane" (`ze-vpp-conf.yang:198-205`). Confirm with
  Thomas (A-7) -- this is the one place where the cheaper answer is a real alternative.
→ Constraint: BGP must NOT learn about VPP or LCP. The netns leaf is generic ("bind listeners
  in this namespace"); the operator sets it to match `vpp.lcp.netns`. Inheriting the value
  from the vpp component would couple BGP to VPP and is rejected.

## Data Flow

### Entry Point

- Problem A: config `vpp { lcp { netns "dataplane" } }` plus a `bgp` stanza with a listen
  address. The netns value enters via the vpp component config
  (`internal/component/vpp/yang/ze-vpp-conf.yang:198-205`, default "dataplane") and reaches
  VPP through `lcp_itf_pair_add_del` (`lcp.go:87-102`). The BGP listen address enters via the
  bgp config and reaches `NewListener` (`listener.go:50`).
- Problem B: the same `vpp.lcp` config, plus the VPP binary's actual plugin set, which is a
  property of the RUNNING system and not of any config file.

### Transformation Path

1. Config parse: `vpp.lcp.netns` and `lcp.enabled` land in the vpp component's settings.
2. startup.conf generation: `linux_cp_plugin.so` and `linux_nl_plugin.so` are enabled when
   LCP is on (`startupconf.go:82-91`).
3. Doctor, post-config phase: `checkVPPLCPNetns` warns if the netns is not root-reachable
   (`doctor.go:100-131`). No check exists for plugin presence.
4. Apply: `lcpItfPair` sends `LcpItfPairAddDel` with `lcpPairNetns(settings.Netns)`
   (`lcp.go:62`, `:87-102`). A missing `linux_cp_plugin.so` fails HERE, failing the apply.
5. VPP creates the TAP in the named namespace.
6. BGP: `NewListener(addr)` (`listener.go:50`), then the reactor replaces the factory with
   `r.newListenerFactory(port)` (`reactor.go:1004` global, `:1402` per-port), which returns
   either a fresh `RealListenerFactory{MD5Peers, ListenTTL}` or the injected
   `r.listenerFactory` (`reactor.go:1378-1385`). `StartWithContext` calls
   `listenerFactory.Listen` (`listener.go:108`) -> `RealListenerFactory.Listen`
   (`network.go:167`) -> `lc.Listen` (`network.go:193`), on the reactor goroutine's thread:
   ze's namespace, not the TAP's. The bind cannot see the TAP. (Step 6 holds both the gap and
   the A-2 trap; see Design Insights.)

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Config to vpp component | YANG `vpp/lcp` container, `netns` leaf (`ze-vpp-conf.yang:198-205`) | [ ] |
| ze to VPP (control) | GoVPP binapi `LcpItfPairAddDel` (`lcp.go:88-95`) | [ ] |
| ze to VPP (startup) | generated startup.conf plugin stanzas (`startupconf.go:82-91`) | [ ] |
| VPP to Linux | LCP creates a TAP inside the `netns` named per pair | [ ] |
| BGP to kernel socket | `net.ListenConfig` via `RealListenerFactory.Listen` (`network.go:167`), in ze's namespace | [ ] read 2026-07-16 |
| Namespace boundary (THE GAP) | none: no code crosses it; BGP binds where its thread runs | [ ] read 2026-07-16 |
| Doctor to running VPP | today only `vppctl` exec (`checks_linux.go:266`); no GoVPP probe from the plugin | [ ] read 2026-07-16 |
| Doctor process to daemon process (NEWLY FOUND) | none. Doctor is offline/local (`doctor/register.go:13-21`); `GetActiveConnector()` (`vpp.go:75`) is nil there | [ ] read 2026-07-16 |

### Integration Points

- `network.ListenerFactory` (`internal/core/network/network.go:32-40`) - the interface.
  Note: NOT the extension point. A new implementation of it is discarded by
  `newListenerFactory` on MD5/GTSM ports.
- `network.RealListenerFactory` (`internal/core/network/network.go:147-162`) - THE extension
  point. Netns joins `MD5Peers` and `ListenTTL` as a per-bind socket concern.
- ~~`Listener.SetListenerFactory` (`internal/component/bgp/reactor/listener.go:66-68`) - the
  existing injection point; no reactor surgery needed to swap the factory.~~ SUPERSEDED: the
  reactor overwrites it at `reactor.go:1004` / `:1402`. It is not an external seam.
- `Reactor.newListenerFactory` (`internal/component/bgp/reactor/reactor.go:1378-1385`) - the
  real production producer; the netns value must flow through here or it is dropped.
- `diagnostic.RegisterDoctorCheck` (`internal/plugins/iface/vpp/doctor.go:51`) - where a
  third check registers.
- `internal/core/diagnostic/codes.go` (lines 288-299) - where a new code's registry row goes.
- ~~`vppcomp.GetActiveConnector` (used by `internal/plugins/static/backend_vpp_linux.go:31`) -
  an existing way to reach a live VPP connector, a candidate for the GoVPP probe.~~
  **WRONG, corrected 2026-07-16.** `GetActiveConnector` (`vpp.go:75`) returns process-local
  state set by the daemon's Manager Run. `ze doctor` is a different process
  (`doctor/register.go:13-21`), so it returns nil there, always. The static plugin can use it
  only because it runs INSIDE the daemon.
- `vppcomp.NewConnector(apiSocket)` (`internal/component/vpp/conn.go:37`) + `Connect`
  (`:44`) + `NewChannel` (`:93`) + `Close` (`:154`) - the correct probe path: the check reads
  `vpp/api-socket` from the config tree and drives its own short-lived connection.
- `api.Channel.CheckCompatiblity` (govpp `api/api.go:109`) with `&lcp.LcpDefaultNsGet{}`
  (`binapi/lcp/lcp.ba.go:63`) - the plugin-presence test itself.

### Architectural Verification

- [ ] No bypassed layers (a netns listener goes through `ListenerFactory`, not raw syscalls
      in the reactor)
- [ ] No unintended coupling (BGP does not learn about VPP or LCP; it learns about namespaces)
- [ ] No duplicated functionality (the probe reuses an existing VPP connector rather than a
      second connection path)
- [ ] Zero-copy preserved where applicable (N/A: control-plane path, not wire encoding)
- [ ] Registration over hardcoding: the new doctor check registers via
      `diagnostic.RegisterDoctorCheck` from the owning plugin and the core discovers it; no
      per-check switch case or field is added to a core/shared package
      (`ai/rules/doctor-checks.md`, `ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | BGP has zero netns awareness today | Read of `listener.go` (lines 32-46, 108) plus a grep across `internal/component/bgp/reactor/` returning only event-namespace hits | The work is smaller than believed | Re-grep at design time | confirmed by read, 2026-07-15 |
| A-2 | A netns-aware listener can be a `ListenerFactory` implementation with no reactor surgery | `SetListenerFactory` (`listener.go:66-68`) and the factory call site (`listener.go:108`) | The change reaches into reactor lifecycle and grows a lot | Prototype a factory that binds in a named netns | **BROKEN, 2026-07-16.** Read the producer `newListenerFactory` (`reactor.go:1378-1385`): when `md5PeersForListener(port)` is non-empty or `listenTTLForListener(port)` is non-zero it returns a FRESH `RealListenerFactory{...}` and discards `r.listenerFactory`. Also `Listener.SetListenerFactory` is overwritten by the reactor at `reactor.go:1004` / `:1402`, so it is not an external seam at all. A netns factory injected as a separate implementation is silently dropped on MD5/GTSM ports. Reactor surgery IS required (small: thread netns through `newListenerFactory`). Mistake Log row added |
| A-3 | Binding in a named netns needs OS-thread locking for the socket's lifetime, not just at bind | `setns` semantics are per-thread; `runtime.LockOSThread` is used in the tree's netns tests | The design needs a dedicated thread or a helper process; blast radius grows | Prototype plus a QEMU test | **REFINED, still unvalidated.** Basis corrected: the cited path `internal/plugins/static/resolve_integration_linux_test.go:65-86` was not verified; the real in-tree idiom is `internal/test/runner/netns_linux.go` (`enterTestNetns`: `runtime.LockOSThread` + `netns.Get` + `netns.NewNamed` + same-thread `restore()`). HYPOTHESIS (kernel semantics, NOT read from ze source, must not be treated as verified): a socket's namespace is fixed at creation, so pinning is needed only around socket create+bind, not for the socket's lifetime; `Accept` then works from any thread. Supporting but non-probative in-tree evidence: `netns_linux.go`'s header records fork+exec'd children inheriting the locked thread's netns (validated by `TestNetnsLaunchChildInheritsNamespace`). Settle by prototype + QEMU before coding |
| A-4 | A missing `linux_cp_plugin.so` is detectable from a live VPP before apply | `startupconf.go:82-91` asks for the plugin; the failure lands at `lcp.go:97` | The check cannot exist pre-apply and the answer is a better error at the binapi layer | Probe a VPP built without the plugin | **CONFIRMED, 2026-07-16.** Two independent lines. (1) Mechanism: `api.Channel.CheckCompatiblity` (govpp `api/api.go:109`, impl `core/channel.go:184-200`) resolves each message ID and reports `adapter.UnknownMsgError` as `CompatibilityError.IncompatibleMessages`; `LcpDefaultNsGet` (`binapi/lcp/lcp.ba.go:63`) is supplied only by linux_cp. (2) Real-VPP evidence already recorded: `plan/learned/1098-followup-vpp-iface.md:73-82` states `ligato/vpp-base` lacks `linux_cp_plugin.so` and the binapi call returns **"unknown message"** -- the exact condition `CheckCompatiblity` detects. Premise UNCHANGED, so `1098` needs no correction (see note below) |
| A-5 | A GoVPP probe is the right mechanism, as the deferrals row states | deferrals 2026-07-10 row; but the only precedent (`checks_linux.go:266`) uses `vppctl` exec | The check uses vppctl instead, which conflicts with plugin self-containment | DESIGN decision with the user | **CONFIRMED, 2026-07-16.** GoVPP wins on merit, not just on the deferrals row's say-so: `CheckCompatiblity` is a pure message-ID lookup (it never sends the message), needs no `vppctl` binary, parses no human text, and lets the check live in the owning plugin per `ai/rules/plugin-self-containment.md`. The vppctl precedent exists only because the central doctor package had no VPP client; ifacevpp already has one |
| A-6 | Doctor runs with a live VPP available at `DoctorPhasePostConfig` | `checkVPPVersion` execs vppctl at doctor time (`checks_linux.go:251-266`) | A runtime probe is impossible in that phase; the check needs a different phase or trigger | Trace the doctor phase's runtime context | **BROKEN, 2026-07-16.** Doctor is an OFFLINE, LOCAL command (`doctor/register.go:13-21`: `Mode: "offline"` + `MustRegisterLocalMeta("doctor", Run, ...)`); `Run` -> `runChecks` (`doctor/doctor.go:31,68`) builds only storage + platform and never starts a VPP Manager. The producer `GetActiveConnector` (`vpp.go:67-80`) returns `connectorRef`, set only by `setActiveConnector` on the daemon's Run path, so it is nil in the doctor process. Consequence: the probe MUST open its own connector from `vpp/api-socket` (`ze-vpp-conf.yang:39-43`). Not fatal to the check, but it invalidates the spec's own proposed integration point. Mistake Log row added |
| A-7 | Operators actually want LCP TAPs in a non-root netns, so teaching BGP netns is worth it rather than forcing host netns | The `netns` leaf exists and defaults to "dataplane" (`ze-vpp-conf.yang:199-201`) | The cheaper answer is to default LCP to the host netns and keep the doctor warning | User / operator input | ~~**UNVALIDATED -- NEEDS THOMAS.**~~ ~~**ANSWERED (user, 2026-07-16): "Both: fix the default now, keep netns as real work."**~~ **CORRECTED, 2026-07-16 (same day). The real answer: YES, support a non-root netns, because it IS the default and the documented model.** The QUESTION was REFRAMED first, and that is the load-bearing part: A-7 as written implies a niche opt-in, but the unreachable case is the **DEFAULT** (`ze-vpp-conf.yang:199-201` defaults to "dataplane"; `lcpNetnsIsRootReachable` at `doctor.go:136-143` accepts only "", "host", "root"). ~~which contradicts the design intent stated at `lcp.go:105-108`~~ **FALSE, withdrawn:** `lcp.go:105-108` is a code comment recording its author's belief, not a decision. The recorded decision is `plan/deferrals.md:36` (2026-07-10): reachability "**without forcing the operator to a root-reachable netns**". `"dataplane"` is IPng's production convention, copied on purpose (`54bffb83b`; `docs/research/vpp-deployment-reference.md:179-180`). -> Decision (user, 2026-07-16): **the default STAYS**; there is no default-fix spec and `plan/spec-fixit-vpp-lcp-netns-default.md` must not be created. The whole answer is owned by `plan/spec-bgp-netns.md`, whose priority is RAISED. **No longer gates anything in THIS spec** (it never gated Problem B), and no longer blocks the netns half either |
| A-8 | The two problems belong in one spec | Both concern LCP usability | Split into `spec-bgp-netns` (as deferrals anticipated) plus a doctor follow-up | DESIGN review | ~~**RECOMMEND SPLIT -- NEEDS THOMAS.**~~ **BROKEN / RESOLVED -- -> Decision (user, 2026-07-16): SPLIT.** The assumption "the two problems belong in one spec" is rejected. Thomas: the LCP-presence doctor check and netns-aware BGP listening are **unrelated problems sharing a filename**. Original recommendation (upheld): Problem B is bounded (one check, one code, one `.ci`, no new config surface, no QEMU rail) and shippable now. Problem A needs a BGP YANG leaf, reactor surgery (A-2), thread-pinning validation (A-3), a QEMU rail (R-6), the AC-3 reconciliation, and a ruling on the other listeners. They share a subject, not a root cause (the spec's own Task section says so). See "The 2026-07-16 split" below for what each half concretely becomes. ~~Not actioned by the recording session: the second spec file is NOT created; its name and scope are proposed for Thomas to confirm~~ **ACTIONED 2026-07-16: `plan/spec-bgp-netns.md` exists.** Its scope, ACs (AC-1/2/3/8/9/10) and Phases were MOVED there, not copied |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Netns binding requires thread-pinning that fights the Go runtime and destabilizes the listener | Flaky accept loop or wrong-namespace binds under load | Prototype early; consider a bind-only helper that passes the fd back. Reuse the `enterTestNetns` shape (`internal/test/runner/netns_linux.go`) rather than inventing one |
| R-2 | Fixing A-4 silently invalidates `doctor-vpp-lcp-netns`, leaving a check that warns about a solved problem | The check still fires after BGP can bind in the netns | Treat the check's fate as an explicit AC, not an afterthought. Design answer: NARROW it to a mismatch check (see Behavior to change #5), do not delete it |
| R-8 (NEW, 2026-07-16) | **Silent wrong-namespace bind via the MD5/GTSM branch.** `newListenerFactory` (`reactor.go:1378-1385`) rebuilds the factory from scratch when MD5 or GTSM applies. If netns is carried on the factory but that branch is not updated, an MD5 peer's listener binds in the HOST namespace with no error, and BGP peers on the wrong interface | A netns test that passes without MD5 and fails (or worse, silently binds wrong) with MD5 | Netns must be a FIELD on `RealListenerFactory` threaded through BOTH branches. MANDATORY test: a netns listener test WITH MD5 configured, not only the bare case. This is the concrete instance of the Critical Review "a netns bind failure NEVER silently falls back to the host namespace" row |
| R-9 (NEW, 2026-07-16) | The doctor probe opens a GoVPP connection to a socket path taken from operator config; a wrong/hostile path makes `ze doctor` hang or touch an unexpected socket | `ze doctor` stalls instead of reporting | Bound the probe with a short context timeout (3s, matching `checkVPPVersion` at `checks_linux.go:264`) and always `Close()` the connector. Validate the socket path per the Security Review checklist |
| R-10 (NEW, 2026-07-16) | `CheckCompatiblity` reports "incompatible" for a reason OTHER than plugin absence (e.g. a genuine CRC/version mismatch against a future VPP), producing a false "plugin missing" claim | The check fires on a VPP that demonstrably has linux_cp loaded | The message is `LcpDefaultNsGet`; distinguish `IncompatibleMessages` (plugin absent or CRC drift) from transport errors, and word the diagnostic as "linux_cp API not available on the running VPP" rather than asserting the `.so` file is missing. AC-6 covers the unreachable case; this covers the ambiguous case |
| R-3 | "Parallel to doctor-vpp-wireguard" misleads: the wireguard check is config-only and the LCP one cannot be | A design that greps the config tree for a non-existent LCP toggle | Confirmed already by reading `doctor.go:62-95`; the LCP check needs a runtime probe |
| R-4 | A GoVPP probe from the doctor path opens a VPP connection at diagnosis time, with its own failure modes (socket absent, VPP down) | Doctor errors on a box where VPP simply is not running yet | Degrade to a warning when VPP is unreachable, as `checkVPPVersion` does (`checks_linux.go:268-274`) |
| R-5 | LCP is enabled by default (`ze-vpp-conf.yang:176-181`), so a new error-severity check could fail apply for existing working deployments | Deployments that were fine start failing doctor | Choose severity deliberately; the netns check chose Warning (`doctor.go:128`) |
| R-6 | Linux-only work lands without QEMU proof | The netns leg is proven only by unit tests | `ai/rules/qemu-testing.md` is mandatory; identify the rail during research |
| R-7 | The `doctor-vpp-wireguard` code description already over-claims (see Design Insights); copying its wording into a new code repeats the error | The new code's description promises a runtime check it does not do | Write the description to match the mechanism actually implemented |

## Wiring Test (MANDATORY)

Rows refined 2026-07-16. `test/ui/doctor-vpp-lcp-netns.ci` was confirmed ABSENT by `ls`. The
netns rows MOVED to `plan/spec-bgp-netns.md` with the split; only the doctor half's rows
remain.

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `vpp { lcp { netns dataplane } }` plus a `bgp` stanza, `ze doctor` | -> | `checkVPPLCPNetns` (`doctor.go:100`), the check AS SHIPPED TODAY | `test/ui/doctor-vpp-lcp-netns.ci` (NEW: confirmed absent; the delivered check has unit coverage but no `.ci`). -> Decision (user, 2026-07-16): ships with THIS half. `spec-bgp-netns` AC-3 may rewrite it later; accepted |
| `vpp { lcp { enabled true } }` on a VPP build without `linux_cp_plugin.so`, `ze doctor` | -> | `checkVPPLCPPlugin` (new, `internal/plugins/iface/vpp/doctor.go`) | `test/ui/doctor-vpp-lcp-plugin.ci` (new) |
| (netns listener wiring: config surface -> `newListenerFactory` -> `RealListenerFactory{Netns}`; the MD5 branch; the QEMU rail) | -> | (see the new spec) | **MOVED to `plan/spec-bgp-netns.md`** |

## Acceptance Criteria

-> Constraint (2026-07-16 split): **AC-1, AC-2, AC-3, AC-8, AC-9 and AC-10 MOVED to
`plan/spec-bgp-netns.md`** and are deliberately NOT reproduced here. They keep their original
numbers there, so the gaps below are self-documenting. Do not re-add them: two copies of an AC
means one of them goes stale. The remaining ACs are the doctor half's, and they are complete
on their own.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1, AC-2, AC-3 | (netns-aware BGP listening) | **MOVED to `plan/spec-bgp-netns.md`.** AC-3 (the fate of the `doctor-vpp-lcp-netns` warning) travels with the netns half because its premise, "BGP can now bind in the LCP netns", only becomes true if that spec lands |
| AC-4 | `lcp.enabled` is true and the running VPP lacks `linux_cp_plugin.so` | `ze doctor` reports an actionable diagnostic BEFORE apply, naming the linux_cp API as unavailable on the running VPP |
| AC-5 | `lcp.enabled` is true and the plugin IS present | No diagnostic |
| AC-6 | `lcp.enabled` is true and VPP is unreachable at doctor time (socket absent, VPP down) | Degrades to a warning about the probe, never a false "plugin missing" claim. Mirrors `checkVPPVersion` (`checks_linux.go:268-274`) |
| AC-7 | The new diagnostic code | Registered in `internal/core/diagnostic/codes.go` with a description matching what the check actually does, and `ze explain <code>` works |
| AC-8, AC-9, AC-10 | (netns listener: QEMU proof, MD5 composition, no host-netns fallback) | **MOVED to `plan/spec-bgp-netns.md`** |
| AC-11 (NEW) | `ze doctor` on a non-linux host, or with `lcp.enabled false` | The LCP plugin check skips cleanly; no probe connection is opened |
| AC-12 (NEW, 2026-07-16) | The DELIVERED `checkVPPLCPNetns` (`doctor.go:100-131`), as it behaves TODAY | Covered by `test/ui/doctor-vpp-lcp-netns.ci`, which does not exist (Q11). -> Decision (user, 2026-07-16): this `.ci` ships with the DOCTOR half, testing the check's CURRENT root-reachable behavior. Accepted: `spec-bgp-netns`'s AC-3 will rewrite it when it narrows the check |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|---------------------|-----------------------|
| 1 | Runs VPP with default LCP settings and expects BGP to peer over a VPP NIC | config -> startup.conf -> lcp_itf_pair_add_del -> TAP in "dataplane" netns -> BGP listener binds in that netns -> peer established | QEMU integration test (rail to identify) |
| 2 | Runs `ze doctor` on a VPP build lacking linux_cp_plugin.so | config tree + live VPP probe -> `checkVPPLCPPlugin` -> diagnostic code | `test/ui/doctor-vpp-lcp-plugin.ci` (new) |
| 3 | Runs `ze explain doctor-vpp-lcp-plugin` | code registry (`codes.go`) -> explain output | `test/ui/doctor-vpp-lcp-plugin.ci` (new) |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCheckVPPLCPPluginMissingWarns` | `internal/plugins/iface/vpp/doctor_test.go` | `lcp.enabled` true plus a probe reporting no linux_cp yields the diagnostic | proposed |
| `TestCheckVPPLCPPluginPresentSilent` | `internal/plugins/iface/vpp/doctor_test.go` | Plugin present yields no diagnostic | proposed |
| `TestCheckVPPLCPPluginProbeUnavailable` | `internal/plugins/iface/vpp/doctor_test.go` | VPP unreachable degrades to a probe warning, never a false negative claim (AC-6) | proposed |
| `TestCheckVPPLCPPluginDisabledSkips` | `internal/plugins/iface/vpp/doctor_test.go` | `lcp.enabled` false skips the check entirely and opens NO connection (AC-11) | proposed |
| `TestCheckVPPLCPNetnsWarnsOnNonRootNetns` | `internal/plugins/iface/vpp/doctor_test.go` | The DELIVERED check's current behavior, backing the new `.ci` (AC-12). Verify against existing coverage first; do not duplicate it | proposed |
| (the six netns tests: `TestNetnsListenerFactory*`, `TestNetnsListenerSocketOutlivesThreadUnpin`, `TestNewListenerFactoryCarriesNetnsWithMD5`) | `internal/core/network/`, `internal/component/bgp/reactor/` | **MOVED to `plan/spec-bgp-netns.md`** | moved |
| ~~`TestListenerUsesInjectedFactory`~~ | ~~`internal/component/bgp/reactor/listener_test.go`~~ | ~~The existing injection seam carries a netns factory through `StartWithContext`~~ SUPERSEDED: A-2 is broken, that seam is overwritten by the reactor. Replaced by `TestNewListenerFactoryCarriesNetnsWithMD5`, which moved with the netns half | dropped |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `vpp.lcp.netns` | string, YANG default "dataplane" (`ze-vpp-conf.yang:198-205`) | any existing namespace name; "", "host", "root" mean the host netns (`doctor.go:136-143`) | N/A (no numeric range) | N/A |
| Probe timeout | candidate 3s, matching `checkVPPVersion` (`checks_linux.go:264`) | 3s | a timeout so short the probe flaps | a timeout that stalls `ze doctor` |
| BGP listener netns name | string, empty = today's behavior (AC-2) | empty (no pinning at all); a name that exists under `/run/netns/` | N/A | a name containing `/` or `..`, which must be REJECTED at YANG validation, not passed to a filesystem path (Security Review: input validation) |
| LCP host TAP name | existing constraint, unchanged by this spec: <= 15 bytes (`lcp.go:32` `lcpMaxHostName`, Linux IFNAMSIZ) | 15 bytes (`lcp.go:51-53` rejects longer) | N/A | 16 bytes -> error, never silent truncation |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `doctor-vpp-lcp-plugin` | `test/ui/doctor-vpp-lcp-plugin.ci` | An operator on a VPP build without linux_cp_plugin.so is warned before apply | proposed |
| `doctor-vpp-lcp-netns` | `test/ui/doctor-vpp-lcp-netns.ci` | An operator whose LCP netns is not root-reachable is warned that BGP cannot bind there. -> Decision (user, 2026-07-16): ships with THIS half; covers the check as it exists today (AC-12) | proposed |
| `doctor-vpp-wireguard` | `test/ui/doctor-vpp-wireguard.ci` | The existing sibling test to model the new ones on | exists |
| BGP over LCP TAP in a non-root netns | QEMU rail | **MOVED to `plan/spec-bgp-netns.md`** | moved |

### Interop Tests

**N/A, confirmed by research 2026-07-16, not assumed.** Evidence: the change is confined to
socket CREATION. `RealListenerFactory.Listen` (`network.go:167-194`) returns a `net.Listener`
and nothing downstream of it changes: `acceptLoop` (`listener.go:149-202`) and the reactor's
connection handlers receive an ordinary `net.Conn` either way. No message encoding, no
capability, no FSM path is touched, and no file under `internal/component/bgp/wire`,
`message`, or `capability` appears in Files to Modify. A peer cannot observe which namespace
the socket was created in. `ai/rules/interop-and-goal-validation.md` governs; the goal
validation is the QEMU rail (AC-8), not an interop matrix.

### Future

None deferred. This is a skeleton; scope is set at DESIGN.

## Files to Modify

Confirmed by research 2026-07-16. ~~Grouped by problem, because A-8 recommends a split.~~
**The split is DONE: Problem A's files moved to `plan/spec-bgp-netns.md`.** Only Problem B's
files remain in this spec.

**Problem B (doctor check) -- bounded, no config surface, shippable alone:**

- `internal/plugins/iface/vpp/doctor.go` - add `checkVPPLCPPlugin` and register it alongside
  the existing two (`registerDoctorChecks`, lines 27-56; new Order 742, Component "vpp",
  Dependencies `["vpp-lcp"]`). The check reads `vpp/lcp/enabled` and `vpp/api-socket` from the
  config tree, opens its own connector, and calls `CheckCompatiblity(&lcp.LcpDefaultNsGet{})`
- `internal/core/diagnostic/codes.go` - add the `doctor-vpp-lcp-plugin` row beside lines
  288-299. Description must state the MECHANISM actually implemented (R-7)
- `test/ui/doctor-vpp-lcp-plugin.ci` - new functional test
- `test/ui/doctor-vpp-lcp-netns.ci` - new functional test for the DELIVERED netns check
  (AC-12). -> Decision (user, 2026-07-16): assigned to this half

**Problem A (netns listener) -- MOVED, do not implement from this file:**

-> Constraint: every Problem A file (`internal/core/network/network.go`, `netns_linux.go`,
`netns_other.go`, `internal/component/bgp/reactor/reactor.go`, `config.go`,
`internal/component/bgp/yang/`, the AC-3 narrowing of `internal/plugins/iface/vpp/doctor.go`,
the `doctor-vpp-lcp-netns` reword in `internal/core/diagnostic/codes.go`, and `docs/guide/vpp.md`)
now lives in **`plan/spec-bgp-netns.md`**, with the load-bearing `newListenerFactory` finding
and the "field, not wrapper" reasoning. Deliberately not duplicated here.

-> Constraint (R-12 there): `internal/plugins/iface/vpp/doctor.go` is the ONE file both halves
touch. This spec ADDS `checkVPPLCPPlugin`; the netns spec NARROWS `checkVPPLCPNetns`. Whichever
lands second must re-read the file rather than trust the other spec's line numbers.

**Not to modify:**

- `plan/learned/1098-followup-vpp-iface.md` - ~~update if A-4's premise changes~~ **NO CHANGE
  NEEDED.** A-4's premise is CONFIRMED, not changed: `1098`'s Gotcha (lines 73-82) recorded
  the "unknown message" failure and that `doctor-vpp-lcp-netns` does NOT catch plugin absence.
  Research agrees with it on every point; it correctly predicted this work. Nothing to correct.
  → Constraint: if Problem A later lands, `1098`'s Decisions bullet "any other value passes
    through and the doctor check warns" (lines 37-42) becomes stale in its IMPLICATION (the
    warning narrows), but a learned summary records what was decided THEN and must not be
    rewritten. The new learned summary for this spec carries the supersession instead.
- **NAMING COLLISION, flagged:** "A-4" means two different things. In `1098` and in
  `doctor.go:15` / `lcp.go:13` ("see A-4"), it is the ORIGIN spec's A-4 = the BGP-netns
  deferral (Problem A). In THIS spec's Assumptions table, A-4 = "a missing plugin is
  detectable" (Problem B). They are unrelated. The source comments' "(A-4)" references point
  at a retired spec (`spec-followup-vpp-iface.md`) and per `ai/rules/planning.md` "Design
  references survive closure" should be repointed at `plan/learned/1098-followup-vpp-iface.md`.
  Not actioned here (out of this spec's approved scope); raise with Thomas.

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] Problem A only, pending Thomas (Q3). Read `ai/rules/config-surface.md` (YANG vs env var) and `ai/rules/config-naming.md` | `internal/component/bgp/yang/` |
| YANG validation constraints | [ ] If a netns leaf lands: a namespace name is a `/run/netns/<name>` path component, so constrain pattern + length; never a bare `type string` (Security Review: input validation) | per `ai/patterns/config-option.md` |
| Doctor check for runtime dependencies | [ ] Yes, that is Problem B | `internal/plugins/iface/vpp/doctor.go`, `internal/core/diagnostic/codes.go` |
| Functional test | [ ] Yes, two: the new plugin check AND the missing netns-check test | `test/ui/doctor-vpp-lcp-plugin.ci`, `test/ui/doctor-vpp-lcp-netns.ci` |
| CLI commands/flags | [ ] No | - |
| Prometheus counters | [ ] No | - |
| QEMU rail | [ ] Problem A only. Mandatory (`ai/rules/qemu-testing.md`) and NOT identified: no VPP QEMU rail found, and `1098` records that real-VPP LCP proof needs a VPP image carrying the linux-cp plugins | to identify before Phase 3 (Q12) |

### Documentation Update Checklist

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] Problem A: Yes (BGP netns binding). Problem B: No (a diagnostic, not a feature) | `docs/features.md` |
| 2 | Config syntax changed? | [ ] Problem A only, if a netns leaf lands (Q3) | `docs/guide/configuration.md` |
| 6 | Has a user guide page? | [ ] Yes for both. Problem B adds the new code; Problem A rewrites the netns constraint the guide currently documents as a hard limit | `docs/guide/vpp.md` |
| 12 | Internal architecture changed? | [ ] Problem A: Yes. `network.go:1-12`'s package doc and `listener.go:1`'s `// Design:` anchor both describe listener creation; a namespace concept changes that contract | `docs/architecture/core-design.md` |
| 15 | Registered diagnostic code changed? | [ ] Yes: `doctor-vpp-lcp-plugin` added; `doctor-vpp-lcp-netns` description reworded if Problem A lands | `internal/core/diagnostic/codes.go`, `docs/guide/vpp.md` |
| 16 | Any changed source file referenced by doc source anchors? | [ ] Grep `docs/` for the changed files. Known: `lcp.go:1` anchors `docs/research/vpp-deployment-reference.md`; `network.go:7` anchors `docs/architecture/chaos-web-dashboard.md`; `doctor.go:1` anchors `ai/rules/doctor-checks.md`. Run `scripts/dev/check_doc_links.py --design-only` | per grep |

## Files to Create

- `test/ui/doctor-vpp-lcp-plugin.ci` - proposed functional test for the new check
- `test/ui/doctor-vpp-lcp-netns.ci` - the EXISTING netns check has no functional test
  (confirmed absent by `ls`). ~~Add it with the narrowed behavior (AC-3)~~ -> Decision (user,
  2026-07-16): add it here with the check's **CURRENT** behavior (AC-12). The narrowing is
  `plan/spec-bgp-netns.md`'s AC-3 and will rewrite this `.ci`; that is accepted
- (`internal/core/network/netns_linux.go`, `netns_other.go`, `netns_linux_test.go`) -
  **MOVED to `plan/spec-bgp-netns.md`**

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

**BLOCKING: Phase 0 (research) is DONE (2026-07-16). ~~Two gates precede any coding: Thomas
rules on the A-8 split and on the A-7 / config-surface question.~~ BOTH GATES ARE NOW PASSED:
A-8 answered (SPLIT, done) and A-7 answered ("Both"). The spec is still NOT `ready`: promotion
is Thomas's gate and has not been given. Phase 1 is the ONLY phase left here and it depends on
nothing that is still open.**

0. ~~**Phase 0: Research (`/ze-spec`)**~~ DONE 2026-07-16. Open Questions answered below;
   A-2 and A-6 came back BROKEN; A-4 and A-5 CONFIRMED; A-3 refined; ~~A-7 and A-8 need
   Thomas~~ A-7 and A-8 both ANSWERED by Thomas 2026-07-16.

1. **Phase 1: Doctor check (Problem B, the bounded half).** No config surface, no reactor
   change, no QEMU rail. Ships independently of every open question.
   - Read `vpp/lcp/enabled` and `vpp/api-socket` from the config tree; skip when LCP is off
     (AC-11). Open a short-lived `vppcomp.NewConnector(apiSocket)` with a 3s timeout (R-9),
     `NewChannel()`, then `CheckCompatiblity(&lcp.LcpDefaultNsGet{})`. Always `Close()`.
   - Unreachable VPP -> probe warning, never a "plugin missing" claim (AC-6, R-10).
   - Severity: **Warning**, not Error. LCP is enabled by default (`ze-vpp-conf.yang:177-181`)
     so an Error would fail doctor for deployments that are fine (R-5), and the probe has
     ambiguous failure modes (R-10). The netns check chose Warning (`doctor.go:128`) on the
     same reasoning. → Decision: Warning. Flag to Thomas: the apply-time failure is fatal
     (`1098` Gotcha: "ze exits at startup"), so an argument for Error exists.
   - Tests: `TestCheckVPPLCPPluginMissingWarns`, `TestCheckVPPLCPPluginPresentSilent`,
     `TestCheckVPPLCPPluginProbeUnavailable`, `TestCheckVPPLCPPluginDisabledSkips`
   - Files: `internal/plugins/iface/vpp/doctor.go`, `internal/core/diagnostic/codes.go`
   - Verify: `test/ui/doctor-vpp-lcp-plugin.ci` passes
   - Also in this phase (AC-12): add `test/ui/doctor-vpp-lcp-netns.ci` for the DELIVERED netns
     check's current behavior. It is a separate check and a separate `.ci`, but the same file
     and the same phase of work

~~**GATE: Thomas rules on A-7 (is a non-root LCP netns worth supporting?) and A-8 (split?).
Phases 2-6 do not start until then.**~~ **Both answered 2026-07-16. Phases 2-6 MOVED to
`plan/spec-bgp-netns.md` (renumbered 1-5 there) and are deliberately not reproduced:**

| Was, here | Is now, in `plan/spec-bgp-netns.md` |
|-----------|-------------------------------------|
| Phase 2: prototype the netns bind (A-3) | Phase 1 |
| Phase 3: `RealListenerFactory.Netns` | Phase 2 |
| Phase 4: config surface + reactor threading | Phase 3 |
| Phase 5: netns doctor check reconciliation (AC-3) | Phase 4 |
| Phase 6: functional and QEMU tests | Phase 5 |

2. **Full verification**: `make ze-verify`
3. **Complete spec**: learned summary, two-commit closure.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | A netns bind failure NEVER silently falls back to the host namespace (that would peer on the wrong interface) |
| Data flow | BGP learns about namespaces, not about VPP or LCP; no VPP spelling in the reactor |
| Registration over hardcoding | The new doctor check registers via `diagnostic.RegisterDoctorCheck` from the owning plugin; no switch case or field added to a core/shared package (`ai/rules/doctor-checks.md`, `ai/rules/plugin-self-containment.md`) |
| Doctor checks | New runtime dependency has a check, a code in `codes.go`, a unit test, and a functional test per `ai/rules/doctor-checks.md` |
| YANG validation | If a netns leaf lands: maximum native constraints, no bare `type string` |
| Rule: no-workarounds | The netns constraint is fixed at the source, not documented away |
| Stale check | `doctor-vpp-lcp-netns` does not survive as a warning for a solved problem (AC-3) |
| Code description accuracy | The new code's description in `codes.go` matches the mechanism actually implemented (see R-7) |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| LCP plugin-presence doctor check | `test/ui/doctor-vpp-lcp-plugin.ci` passes |
| New diagnostic code registered | `ze explain <new-code>` returns the entry; grep `internal/core/diagnostic/codes.go` |
| BGP binds in a named netns | QEMU integration test showing an established peer over the LCP TAP |
| Default deployments unchanged | Existing BGP tests pass with no listener behavior change |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | A config-supplied namespace name reaches a `setns`-style operation; validate it and never traverse to an arbitrary path |
| Privilege | Entering a namespace needs CAP_SYS_ADMIN; confirm the failure mode when ze lacks it is a clear error, not a silent host-netns bind |
| Resource exhaustion | Thread pinning per listener must not leak OS threads across restarts |
| Error leakage | Probe and bind errors name the namespace; confirm they carry no unexpected host detail |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Netns bind works in unit tests but not under QEMU | Back to RESEARCH on thread pinning (A-3); do not weaken the test |
| The GoVPP probe cannot run at doctor time | Re-open A-5/A-6 with the user; vppctl exec is the fallback precedent |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| (A-2) A netns listener can be a new `ListenerFactory` implementation injected through the existing seam, with no reactor surgery | `Listener.SetListenerFactory` (`listener.go:66-68`) is overwritten by the reactor immediately after `NewListener` (`reactor.go:1004`, `:1402`), so it is not an external seam. And the producer `newListenerFactory` (`reactor.go:1378-1385`) DISCARDS the injected `r.listenerFactory` whenever MD5 or GTSM applies to the port, returning a fresh `RealListenerFactory{...}` | Read the producer `newListenerFactory` instead of stopping at the consumer `listener.go:108`. The skeleton had read only the call site and the setter, and inferred the seam was usable | Design change: netns becomes a FIELD on `RealListenerFactory`, not a rival factory. Reactor DOES change. Prevented a silent wrong-namespace bind for every MD5/GTSM peer (R-8) |
| (A-6) A live VPP connector is available to doctor checks, via `vppcomp.GetActiveConnector` | Doctor is an offline LOCAL command (`doctor/register.go:13-21`) running in the `ze doctor` process; `GetActiveConnector` (`vpp.go:67-80`) returns `connectorRef`, set only by the daemon's Manager Run, so it is nil there | Read the producer `setActiveConnector`/`GetActiveConnector` and doctor's registration, rather than reasoning from `internal/plugins/static/backend_vpp_linux.go:31` which uses it successfully (that plugin runs INSIDE the daemon) | The spec's own proposed Integration Point was wrong. The probe must open its own connector from `vpp/api-socket`. Also explains WHY the vppctl precedent exists, dissolving the A-5 "tension" |
| (A-3, basis only) `runtime.LockOSThread` netns precedent lives at `internal/plugins/static/resolve_integration_linux_test.go:65-86` | That path was not verified. The real in-tree netns idiom is `internal/test/runner/netns_linux.go` (`enterTestNetns`), plus `internal/core/routewatch/routewatch_linux.go` | Grepped for netns across the tree instead of trusting the cited path | Basis corrected. The assumption's substance (pinning is needed) still needs a prototype; only its citation was wrong |
| (A-7) The `vpp.lcp.netns` default `"dataplane"` is a MISTAKE that "contradicts its own design intent", per the comment at `lcp.go:105-108` | The default is DELIBERATE. `plan/deferrals.md:36` (2026-07-10) records the actual decision: reachability "**without forcing the operator to a root-reachable netns**". `"dataplane"` is IPng's production convention, copied on purpose (`54bffb83b` "following IPng production template"; `docs/research/vpp-deployment-reference.md:179-180` "isolates the forwarding plane from the management plane"). The YANG leaf states the model plainly: "Routing daemons run in this namespace" | Thomas rejected the premise, 2026-07-16. The session had ALREADY read `plan/deferrals.md` earlier the same day (it is this spec's Origin section) and still took a code comment as the intent record | Invented a phantom spec (`plan/spec-fixit-vpp-lcp-netns-default.md`, never created) referenced from both specs, and inverted `plan/spec-bgp-netns.md`'s priority to "build it last". **A comment states what its author believed, not what was decided.** Rule updated: `ai/rules/fail-closed-guards.md` "Evidence corollary" |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- The deferrals row frames Problem B as "parallel to `doctor-vpp-wireguard`". A code read
  shows the parallel is intent-only: `checkVPPWireguardPlugin` (`doctor.go:62-85`) is a
  CONFIG check reading the `vpp.plugins.wireguard` toggle, while LCP has no such toggle
  (`startupconf.go:82-91` enables the plugin straight from `lcp.enabled`). The new check
  cannot be modeled on the old one's mechanism, only on its shape and registration.
- FINDING (pre-existing, unrelated to this spec's scope): the `doctor-vpp-wireguard` code
  description in `internal/core/diagnostic/codes.go:291` claims the check covers "not enabled
  (vpp.plugins.wireguard) **or not loaded in the running VPP**", but `checkVPPWireguardPlugin`
  only reads the config toggle and never probes VPP. The description over-claims. Decide
  during research whether to correct it here or in its own fixit.
- `vpp.lcp.netns` defaults to "dataplane" (`ze-vpp-conf.yang:198-205`) and
  `lcpNetnsIsRootReachable` (`doctor.go:136-143`) accepts only "", "host", "root". So the
  SHIPPED DEFAULT trips the netns warning whenever BGP is configured. That reframes A-4 from
  an edge case to the default experience, and is the strongest argument for fixing it.
- ~~The BGP listener already takes an injectable `ListenerFactory` (`listener.go:36`, `:66-68`,
  `:108`). The netns work may need no reactor change at all, which would make A-4 far cheaper
  than "BGP has zero netns awareness" suggests.~~ **WRONG, superseded 2026-07-16.** See the
  Mistake Log. The reactor overwrites that seam and conditionally discards the injected
  factory. The netns work needs a small but real reactor change.

### ARCHITECTURAL CALL: where does netns awareness belong? (answered 2026-07-16)

The skeleton proposed a netns-aware `ListenerFactory` in `internal/core/network/`, a core leaf
package, to serve a VPP-specific need. That deserved challenge. **Verdict: the PACKAGE is
right, the SHAPE was wrong.** Netns belongs in `internal/core/network` as a `Netns string`
field on `RealListenerFactory`, not as a separate factory type.

Why the package is right, against each rule that could forbid it:

| Rule | Test | Result |
|------|------|--------|
| `ai/rules/plugin-self-containment.md` "no plugin spelling in generic packages" | Does the code spell a plugin? | **PASSES.** The field is `Netns`, a Linux kernel concept. Nothing in `internal/core/network` learns the words `vpp`, `lcp`, or `dataplane`. Delete the vpp plugin and `RealListenerFactory.Netns` still makes sense for any operator isolating a listener. The rule forbids plugin SPELLING, not "any feature a plugin happens to want" |
| `ai/rules/module-tiers.md` core import-direction rule | Does `internal/core/` import `internal/component/` or `internal/plugins/`? | **PASSES.** The only new import is `github.com/vishvananda/netns` (external, `go.mod:23`). `internal/core/routewatch/routewatch_linux.go:12` already imports it from core, so the precedent is exact |
| `ai/rules/module-tiers.md` tier test A (config-driven engine?) | Does it call `sdk.NewWithConn(`? | **PASSES.** No. It is a pure library with no lifecycle: `internal/core/` is correct per the authoring rule ("pure library, no `sdk.NewWithConn`, no plugin lifecycle, no component domain owner") |
| `ai/rules/design-principles.md` "abstract at 2+ use cases" | Is one caller enough to justify core placement? | **NEUTRAL, not a violation.** `internal/core/network` ALREADY exists and already serves exactly this class of concern. Adding a third field beside `MD5Peers`/`ListenTTL` is not a new abstraction; it is an instance of the existing one |

Why the shape (field, not wrapper) is forced, not chosen:

- `newListenerFactory` (`reactor.go:1378-1385`) selects a factory **either/or**: a fresh
  `RealListenerFactory{MD5Peers, ListenTTL}` OR the injected one. There is no stacking. A
  wrapper factory is therefore silently discarded exactly when MD5/GTSM is configured (R-8).
- `MD5Peers` and `ListenTTL` are already fields for the identical reason: both are per-bind
  socket-creation concerns that must compose. Netns is a third of the same kind.
- The package's file layout already encodes this: `md5_{linux,freebsd,darwin,other}.go` and
  `ttl_{linux,other}.go` each implement one field's platform behavior. `netns_{linux,other}.go`
  is the same pattern, not a new one.

→ Decision: `Netns string` on `RealListenerFactory`, helper in
  `internal/core/network/netns_linux.go` + `netns_other.go`. The VPP-specific knowledge stays
  where it belongs: in `ifacevpp`'s doctor check, which is the only place that knows
  `vpp.lcp.netns` and the BGP listener netns should agree.
→ Constraint: one genuine asymmetry to respect. `MD5Peers`/`ListenTTL` are applied via
  `lc.Control` (`network.go:172-192`), a callback on the already-created fd. Netns CANNOT use
  `Control`: the socket must be CREATED inside the namespace, so the helper must wrap the
  whole `lc.Listen` call (lock thread, setns, listen, restore, unlock) rather than plug into
  `Control`. Same field shape, different insertion point.

### Where netns does NOT belong

- **Not in the reactor.** BGP must not learn about VPP or LCP. It learns "bind in namespace X".
- **Not inherited from `vpp.lcp.netns`.** That couples BGP to VPP and inverts the dependency.
  The operator sets both; the doctor check catches disagreement (AC-3).
- **Not a new `internal/core/netns` package.** `internal/core/network` already owns listener
  socket creation; a second package would split one concern across two.

## Known Limitations

Set at DESIGN 2026-07-16. These are deliberate scope boundaries, NOT deferred work: each is
recorded here because it is a real limitation of the proposed design, and Q4/Q5 above ask
Thomas whether any should be pulled in.

- **Outbound BGP connections stay in ze's namespace.** Only the LISTENER gains netns
  awareness. `RealDialer` (`network.go:56-117`) would need its own `Netns` field for a peer
  whose outbound session must egress the LCP TAP. Sufficient for listener-side LCP peering;
  insufficient if ze must INITIATE over the TAP. ~~→ Open: Q4.~~ → RESOLVED 2026-07-17: OUT OF
  SCOPE here; follow-up belongs to `plan/spec-bgp-netns.md` (see Q4).
- **Other listeners stay in ze's namespace.** Web (`web/server.go:198`), looking-glass
  (`lg/server.go:346`), and the DNS server (`dnsserver/secure.go:315`) construct listeners
  directly and do not route through `network.ListenerFactory`, so they gain nothing from this
  work. BGP is genuinely the special case today. ~~→ Open: Q5.~~ → RESOLVED 2026-07-17: OUT OF
  SCOPE here; follow-up belongs to `plan/spec-bgp-netns.md` (see Q5).
- **The netns leg is linux-only.** `netns_other.go` stubs it; a configured netns on a
  non-linux build must be a clear error, never a silent host bind (AC-10).
- **The doctor probe reports API availability, not file presence.** `CheckCompatiblity` proves
  the linux_cp API is unreachable on the running VPP; it cannot distinguish "the `.so` is
  absent" from "the `.so` is present but failed to load" or a CRC drift (R-10). The diagnostic
  must be worded accordingly (R-7 warns against repeating the wireguard code's over-claim).
- **The probe needs a reachable VPP.** On a box where VPP is not yet running, the check
  degrades to a warning (AC-6) and cannot answer the question. It is a pre-apply aid, not a
  guarantee.

## Open Questions

**Answered by research 2026-07-16.** Each carries its verdict; ~~the four still open are marked
NEEDS THOMAS and are the reason this spec is not `ready`.~~ → AUTONOMOUS DEFAULT (2026-07-17):
**all four formerly-open items are now resolved for this doctor-only spec.** Q3 is a Problem A
config-surface question that MOVED to `plan/spec-bgp-netns.md`; Q4 and Q5 are OUT OF SCOPE here
with follow-up pointers to `plan/spec-bgp-netns.md`; Q10 is resolved to Warning. No open question
remains in THIS spec's scope, so it is `ready`.

| # | Question | Answer |
|---|----------|--------|
| 1 | Mechanism for binding inside a named netns from Go? | **ANSWERED (design), pending prototype.** Reuse the in-tree idiom `enterTestNetns` (`internal/test/runner/netns_linux.go`): `runtime.LockOSThread` + `netns.Get` + enter + same-thread `restore()`. `github.com/vishvananda/netns v0.0.5` is already a direct dep (`go.mod:23`) and `internal/core/routewatch/routewatch_linux.go:12` already imports it from core. No new dependency, no new idiom |
| 2 | Does the socket STAY in the namespace after bind, or is the namespace bind-time only? | **HYPOTHESIS, must be prototyped (A-3, Phase 2).** Believed bind-time only: namespace membership attaches at socket creation, so pinning wraps create+bind and `Accept` then works from any thread. This is a claim about KERNEL semantics, NOT read from ze source, so per `ai/rules/no-fabrication.md` it is labelled unverified and gates Phase 3 via `TestNetnsListenerSocketOutlivesThreadUnpin`. If false, the design changes to a dedicated thread or fd-passing helper (R-1) |
| 3 | Should the netns be a BGP config leaf, inherited from `vpp.lcp.netns`, or process-level? | **PARTLY ANSWERED. Inheriting is REJECTED** (couples BGP to VPP, inverts the dependency; `ai/rules/design-context.md` anti-pattern "translation layer"). A generic BGP leaf is the recommendation. **NEEDS THOMAS:** the exact shape (`bgp { listen { netns } }` vs a process-level setting) and whether it is YANG or env per `ai/rules/config-surface.md`. Note a process-level setting would answer Q5 for free. → AUTONOMOUS DEFAULT (2026-07-17): **MOVED to `plan/spec-bgp-netns.md`** — a Problem A config-surface question, not this doctor spec's decision (the doctor half has no config surface). It is the remaining gate on that spec's `ready`, not this one's |
| 4 | Do outbound BGP connections (the dialer) need the same treatment? | **NEEDS THOMAS / follow-up.** Not answered by research. `RealDialer` (`network.go:56-117`) has the same structure and would need a matching `Netns` field for a peer whose OUTBOUND session must egress the TAP. For a listener-only LCP peering story it is not required. Recording as a Known Limitation rather than silently scoping it out. → AUTONOMOUS DEFAULT (2026-07-17): **OUT OF SCOPE for this doctor-check spec.** The dialer is a netns/listener concern, not a diagnostic concern; it belongs to the netns effort. FOLLOW-UP POINTER: carry this into `plan/spec-bgp-netns.md` (the owner of `RealDialer`/`RealListenerFactory` netns work). This spec adds no dialer behavior. Thomas: reassign if you want the dialer treated here |
| 5 | Do web / gnmi / looking-glass listeners need this too? | **NEEDS THOMAS / follow-up.** Research shows they do NOT go through `network.ListenerFactory` (`web/server.go:198`, `lg/server.go:346`, `dnsserver/secure.go:315` each build listeners directly), so BGP is genuinely special TODAY. If an operator wants the whole box in the dataplane namespace, a process-level netns (Q3) is a better answer than per-service leaves. → AUTONOMOUS DEFAULT (2026-07-17): **OUT OF SCOPE for this doctor-check spec.** Other-listener netns coverage is a netns-architecture decision tied to Q3 (config-surface shape), which already lives in `plan/spec-bgp-netns.md`. FOLLOW-UP POINTER: `plan/spec-bgp-netns.md`. This doctor spec touches no listener. Thomas: reassign if you want it scoped here |
| 6 | What happens to `doctor-vpp-lcp-netns` if BGP can bind in the LCP netns? | **ANSWERED: NARROW, do not delete.** It becomes a mismatch check: warn when `vpp.lcp.netns` and the BGP listener netns disagree. That is a real, permanent hazard, whereas "netns is not root-reachable" becomes false once BGP can follow. AC-3, Behavior-to-change #5, Phase 5 |
| 7 | Should `vpp.lcp.netns` default stay "dataplane"? | **ANSWERED (user, 2026-07-16): YES, IT STAYS.** The original research verdict, ~~"RECOMMEND KEEP, needs Thomas"~~, was RIGHT. ~~**ANSWERED and MOVED OUT: the default is being FIXED now**, owned by `plan/spec-fixit-vpp-lcp-netns-default.md` (another agent, concurrent).~~ **SUPERSEDED: that reversal was argued from a code comment (`lcp.go:105-108`) against the recorded decision in `plan/deferrals.md:36`, and the spec it named was never created and must not be.** The default is deliberate: IPng's production convention (`54bffb83b`; `docs/research/vpp-deployment-reference.md:179-180`), isolating forwarding from management. CLOSED, not moved |
| 8 | GoVPP probe or `vppctl show plugins`? | **ANSWERED: GoVPP** (A-5 confirmed). `api.Channel.CheckCompatiblity` (`api/api.go:109`, impl `core/channel.go:184-200`) with `&lcp.LcpDefaultNsGet{}` (`lcp.ba.go:63`). No exec, no text parsing, no side effects (it only resolves a message ID; it never sends). Keeps the check inside the owning plugin per `ai/rules/plugin-self-containment.md`. The vppctl precedent exists only because the central doctor package has no VPP client |
| 9 | Is a live VPP connection available at `DoctorPhasePostConfig`? Is `GetActiveConnector` usable? | **ANSWERED: NO, and NO** (A-6 BROKEN). Doctor is an offline LOCAL command (`doctor/register.go:13-21`) in its own process; `GetActiveConnector` (`vpp.go:67-80`) returns daemon-process state and is nil there. The check opens its own connector from `vpp/api-socket` (`ze-vpp-conf.yang:39-43`). The phase stays `DoctorPhasePostConfig`; only the connection source changes |
| 10 | Severity for the LCP plugin check? | **ANSWERED: Warning** (with a flag to Thomas). LCP is on by default (`ze-vpp-conf.yang:177-181`) so Error risks failing doctor for working deployments (R-5), and the probe has an ambiguous failure mode (R-10). Counter-argument worth Thomas's attention: the apply-time consequence is fatal (`1098`: "ze exits at startup"), which argues for Error. → AUTONOMOUS DEFAULT (2026-07-17): **Warning** is adopted as the plan. Rationale: fail-closed for the OPERATOR here means "warn, do not block a box that VPP simply is not running on yet" — Error would turn the ambiguous probe (R-10: cannot distinguish `.so` absent from CRC drift) into a false apply-blocker for deployments that work, and it matches the sibling `checkVPPLCPNetns` (`doctor.go:138`, SeverityWarning). This is the lower-risk, more-reversible option (a Warning can be raised to Error later without regressing anyone). Thomas: override to Error if the fatal apply consequence should hard-stop doctor |
| 11 | Does `test/ui/doctor-vpp-lcp-netns.ci` exist? | **ANSWERED: NO.** `ls` confirms only `test/ui/doctor-vpp-wireguard.ci` exists among the vpp doctor tests. The delivered netns check has unit coverage but no functional test. Added to Files to Create |
| 12 | What QEMU rail can prove BGP peering over an LCP TAP? Does any VPP QEMU test exist? | **MOVED to `plan/spec-bgp-netns.md`** (it is a netns-leg question; this half needs no QEMU rail). Partly answered there 2026-07-16: the netns BIND rail exists and auto-discovers (`ZE_QEMU_INTEGRATION_PKGS`, `mk/test-integration.mk:319`). The residual, still unanswered, is the VPP+LCP end-to-end rail: `1098` records that real-VPP LCP proof needs a VPP image WITH the linux-cp plugins (`ligato/vpp-base` lacks them), an image-provisioning problem on top of a test-rail problem. It was the strongest argument for the A-8 split, which is now done |
| 13 | Split into `spec-bgp-netns` + a doctor follow-up? | ~~**RECOMMEND SPLIT, needs Thomas** (A-8).~~ **ANSWERED -- -> Decision (user, 2026-07-16): SPLIT.** No longer open. Problem B is one check, one code, one `.ci`, no config surface, no QEMU rail. Problem A needs a config surface, reactor surgery, a kernel-semantics prototype, a QEMU rail that does not exist, and rulings on Q3/Q4/Q5/Q7. Bundling holds a ready fix behind an unapproved design. The deferrals row already anticipated `spec-bgp-netns`. See Task -> "The 2026-07-16 split" for what each half becomes. ~~**The new spec file is NOT created by the recording session** -- its name and scope are proposed for Thomas to confirm first~~ **DONE 2026-07-16: `plan/spec-bgp-netns.md` created, with AC-1/2/3/8/9/10 and Phases 2-6 moved into it** |

## Decisions Needed From Thomas (blocking `ready`)

-> Constraint: after the 2026-07-16 rulings, **only decision 4 (Q10) remains for this spec, and
it is not a blocker.** Decisions 1 and 3 are answered or moved. Nothing technical stands
between this spec and implementation; it waits only on Thomas's `ready` promotion.

→ AUTONOMOUS DEFAULT (2026-07-17): **all decisions in this table are now resolved for `ready`.**
Decision 4 (Q10) → **Warning** (autonomous default, reversible; see Q10). Decision 5 → **OUT OF
SCOPE, raise as a separate fixit** (both are pre-existing defects outside this spec's Task; the
conservative default for a scope question is the smaller, self-contained option, so they are not
folded in here). The spec is promoted to `ready` on that basis. Thomas: override any row if wrong.

| # | Decision | Why it cannot be decided from code |
|---|----------|-----------------------------------|
| 1 | ~~**A-7:** is a non-root LCP netns a capability worth supporting, or should LCP be forced to the host netns with the doctor warning kept?~~ **ANSWERED (user, 2026-07-16): YES, support it. It is the default and the documented model; forcing LCP to the host netns is REJECTED.** No longer blocking anything. ~~"Both: fix the default now, keep netns as real work"; the default fix is `plan/spec-fixit-vpp-lcp-netns-default.md`~~ **SUPERSEDED: the default STAYS and that spec was never created.** The whole answer is `plan/spec-bgp-netns.md`, priority RAISED | ~~Code shows the capability is deliberate, but only Thomas knows whether operators want it~~ Answered. The code could not say whether operators want it, but it COULD say the unreachable case is the DEFAULT (`ze-vpp-conf.yang:199-201` + `doctor.go:136-143`). That reframing was right; the follow-on claim that the default therefore CONTRADICTED its intent was wrong, because it read a code comment (`lcp.go:105-108`) instead of the recorded decision (`plan/deferrals.md:36`) |
| 2 | ~~**A-8:** split Problem B into its own spec and let it ship now?~~ **DECIDED (user, 2026-07-16): SPLIT.** No longer blocking. This spec keeps Problem B (the doctor check) and ships it; the netns half moves to a new spec that is not yet created (name/scope pending Thomas's confirmation). See Task -> "The 2026-07-16 split" | ~~A scope call, not a technical one~~ Decided by Thomas: the two are unrelated problems sharing a filename. Note the split does NOT unblock the netns half, which still waits on decision 1 (A-7) below |
| 3 | ~~**Q3:** the BGP netns config surface shape (per-listener YANG leaf vs process-level setting)~~ **MOVED to `plan/spec-bgp-netns.md`**, where it is the remaining gate on that spec's `ready`. Not this spec's decision: the doctor half has no config surface | Design choice with a downstream effect on Q4/Q5; `ai/rules/config-surface.md` governs but does not decide |
| 4 | ~~**Q10:** Warning vs Error for `doctor-vpp-lcp-plugin`~~ **RESOLVED → Warning** (autonomous default 2026-07-17; reversible, matches sibling `doctor-vpp-lcp-netns`). No longer blocking. Thomas: override to Error if the fatal apply consequence should hard-stop doctor | A judgement about operator impact: Warning is safer (R-5), Error matches the fatal apply consequence |
| 5 | ~~Out-of-scope but found: the `doctor-vpp-wireguard` description (`codes.go:291`) over-claims a runtime check that does not exist; and `doctor.go:15` / `lcp.go:13` reference "(A-4)" in a retired spec that should point at `plan/learned/1098-followup-vpp-iface.md`~~ **RESOLVED → OUT OF SCOPE; raise as a separate fixit** (autonomous default 2026-07-17). Both are pre-existing defects outside this spec's Task; the conservative default for a scope question is the smaller, self-contained option, so they are not folded into this doctor-check spec. NOTE (2026-07-17): the current source line numbers have drifted — `codes.go` now has `doctor-vpp-wireguard` at line 289 (not 291) and `lcp.go` still carries a "(see A-4)"-class reference; a follow-up fixit should re-locate before editing. No longer blocking | Both are pre-existing defects outside this spec's Task. Fix here or as a separate fixit? |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-4, AC-5, AC-6, AC-7, AC-11, AC-12 all demonstrated (AC-1/2/3/8/9/10 moved to `plan/spec-bgp-netns.md`)
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete (every row has a concrete test name, none deferred)
- [ ] `/ze-review` gate clean (Review Gate section filled: 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] QEMU: N/A for this half. The doctor check is host-testable; the linux-only netns leg and its QEMU gate moved to `plan/spec-bgp-netns.md` (`ai/rules/qemu-testing.md`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

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
