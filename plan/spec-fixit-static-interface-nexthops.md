# spec-fixit-static-interface-nexthops

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-07-17 |

**DESIGN (research complete, NOT approved).** Research was run 2026-07-16. Every Open
Question below is answered in place, every A-N carries evidence, and the design below is
proposed for approval. It is NOT `ready`: ~~two decisions (D-1 backend-mismatch gate, D-2
no-backend diagnosis shape) need Thomas before implementation starts.~~

**2026-07-16, Thomas -- the D-2/D-3 framing is SUPERSEDED.** Asked "can we not validate the
route and ensure that it can not be invalid?", the answer is **partly**:
**config-validate the interface REFERENCE where possible, AND still handle resolution
failure at runtime.** Both halves ship; the runtime error is not made redundant by the
validation. "Cannot be invalid" is **not reachable**: an interface next-hop may legitimately
name an interface ze does not configure (an externally-created tunnel), and resolution needs
a runtime ifindex lookup, so a route can be config-valid and runtime-unresolvable. ~~The
config-validate half is **blocked on a new open question**: widening `WantsConfig`
(`register.go:224`) to include `"interface"` changes what config the static plugin receives,
and that is Thomas's call.~~ **UNBLOCKED 2026-07-16: Thomas approved the widening to
`["static", "interface"]` (C-8).** The config-validate half is buildable. The accepted cost
(static is enqueued on every `interface` change) and the two mechanisms that keep it cheap
are recorded under Design Decisions -> "Decision (user, 2026-07-16): WIDEN `WantsConfig`".
~~**D-1 and D-3 remain open.**~~ Read Design Decisions ->
"Decision (user, 2026-07-16)" before implementing either leg. ~~**Status stays `design`;
promotion to `ready` is Thomas's gate and has not been given.**~~

**→ READINESS PASS (2026-07-17): D-1, D-2 and D-3 are now ALL resolved.** D-2 was answered by
Thomas 2026-07-16 (BOTH halves: widen `WantsConfig` AND keep the runtime error + doctor
check). D-1 and D-3 are resolved by conservative autonomous default in this pass -- see
"→ Autonomous Resolutions (2026-07-17)" immediately under the "Decisions Needed" table. No
open question blocks implementation. **Status advanced `design` → `ready`; Thomas may override
either autonomous default before implementation.**

**Research overturned three of the skeleton's premises.** The skeleton was written from a
call-chain read; running the code and reading the producers changed the shape of both legs:

| Skeleton premise | Research finding |
|------------------|------------------|
| Problem A might be a process-boundary bug (static as a subprocess cannot see `activeBackend`) | BROKEN. `static` runs IN-process (`process.go:456-457`), so it shares `activeBackend`. The real causes are the config-root auto-load gate and a startup-tier RACE (A-1). |
| The VPP leg needs `ifacevpp.resolveIndex` threaded into static (deferrals row + R-4) | BROKEN. `iface.Resolve` ALREADY returns the VPP `sw_if_index` when the VPP iface backend is active (`query.go:232` -> `resolve.go:216`). `ifacevpp.go` needs NO change; R-4 evaporates (A-3). |
| `SwIfIndex` + `toFibPath` is sufficient for the VPP encode (A-2) | BROKEN. `toFibPath` picks the path proto from the NEXT-HOP address, which is unset for an interface-only path, so an IPv4 route would encode as `PROTO_IP6` (A-2). |

## Task

A static route whose next-hop names an interface but carries no address
(`next { interface tun100 { } }`) fails to program on BOTH data-plane backends, for two
different reasons. On linux the next-hop interface is resolved through `iface.Resolve`,
which errors with "iface: no backend loaded" whenever no iface backend is active in the
calling process. On VPP the translator rejects interface-only next-hops outright because no
logical-name to `sw_if_index` resolver is threaded into the static backend. Address
next-hops, recursive next-hops, blackhole and reject all work on both backends, so this is a
bounded gap on one next-hop form. Goal: static interface-only next-hops work on the linux
and VPP backends, or the limitation is made explicit and diagnosable rather than failing an
entire static config section.

The linux root cause is CONFIRMED as a mechanism (the call chain is read end to end below)
but it is UNVERIFIED whether it bites in production or only in the test config that
surfaced it.

## Origin

Problem A (linux): found during `spec-fixit-migrate-sleeps-infra` work, 2026-07-15, while
looking at `test/static/005-table-interface.ci`.
Problem B (VPP): `plan/deferrals.md` row dated 2026-07-10,
"spec-test-coverage-gaps W-3 static/vpp wiring" (deferred).

## Required Reading

### Source (read before designing)

- [ ] `internal/component/iface/dispatch.go:12` - `errIfaceNoBackendLoaded`, the actual
      producer of "iface: no backend loaded"; returned by `backendOrErr()` (`:15-21`)
  → Constraint: the error text lives here, NOT in `resolve.go`; `resolve.go:61-64` only
    documents it in a doc comment. Cite this file when describing the failure.
- [ ] `internal/component/iface/resolve.go:65` - `Resolve()`, the entry point the static
      backend calls
  → Constraint: `Resolve` caches per logical name and is backend-agnostic; the failure is
    inherited from `GetInterface`, not produced here.
- [ ] `internal/component/iface/backend.go:248-250`, `:266-301` - `activeBackend`
      process-global, nil until `LoadBackend`; `GetBackend()` at `:301`
  → Constraint: backend activation is per-PROCESS global state, not passed as a parameter.
    Whether a consumer sees a backend depends on which process it runs in.
- [ ] `internal/component/iface/register.go:406-418` - the only production `LoadBackend`
      call path (also `:575`)
  → Constraint: `LoadBackend` runs only from the iface component's config path and only
    when `interface { backend ... }` is non-empty (`:406-408` returns early otherwise).
- [ ] `internal/plugins/static/backend_linux.go:97-103` - `resolveNexthopIndex` calls
      `iface.Resolve`; called from `buildRoute` at `:141` and `:159`
  → Decision: the static backend deliberately routes through the shared resolver rather
    than its own `LinkByName`, to honor os-name / mac-match selectors. Do not undo this.
- [ ] `internal/plugins/static/backend_vpp_linux.go:73-102` - `toVPPRoute`; `:91-95` is the
      interface-only rejection
  → Decision: the current rejection is deliberate ("failing loudly beats programming a
    wrong (index-0) path silently", `:69-72`). Any fix must keep index-0 unreachable.
- [ ] `internal/plugins/static/vpp/backend.go:24-30`, `:111-115` - `Path.SwIfIndex` already
      exists and `toFibPath` already carries it into `fib_types.FibPath`
  → Constraint: the VPP encode path is ALREADY capable. Only parent-side name resolution
    is missing, which makes the VPP leg much smaller than the deferrals row implies.
- [ ] `internal/plugins/iface/vpp/ifacevpp.go:225-238` - `resolveIndex`, an existing name to
      `SwIfIndex` lookup backed by `b.names.LookupIndex`
  → ~~Constraint: it is an unexported method on `vppBackendImpl` and is channel-gated by
    `ensureChannel()`. Reaching it from the static plugin needs a boundary decision.~~
  → Decision (2026-07-16): DO NOT reach it. There is no boundary decision to make, because
    static does not need this method. The VPP backend already publishes its `sw_if_index`
    through the shared `iface` interface (`query.go:133` `GetInterface` -> `detailsToInfo`
    `:232` sets `Index: int(d.SwIfIndex)`), which `iface.Resolve` returns as
    `Binding.Ifindex` (`resolve.go:216`). `resolveIndex` stays a private fast path; this
    file is NOT modified. See A-3a, R-4 (retired).
- [ ] `internal/plugins/static/inject.go:62-93`, `:138-171` - error join / propagation path
  → Constraint: per-route errors are joined and returned to `OnConfigure`, so ONE bad
    next-hop fails the whole static section. Any fix must decide whether that stays.
- [ ] `test/static/005-table-interface.ci:53-73` - the config that surfaced Problem A
  → Constraint: the config has NO `interface { backend ... }` stanza and never creates
    `tun100`. Both facts matter before "fixing" the test.
- [ ] `internal/component/plugin/inprocess.go:93-124` - in-process runner
  → Decision: plugins have BOTH an in-process runner and a subprocess runner
    (`internal/component/plugin/cli/cli.go:164`). Which one `static` uses decides whether
    Problem A is a product bug or a test-config bug.

### Architecture Docs

- [ ] `plan/learned/650-static-routes.md` - cited as the design doc by both static backends
      (`backend_linux.go:1`, `backend_vpp_linux.go:1`)
  → Constraint: read before changing backend selection or route programming.
- [ ] `plan/learned/950-iface-resolve-2-resolver.md` - cited as the design doc by the shared
      resolver (`resolve.go:1`)
  → Constraint: the resolver is the single owner of logical-name to device resolution for
    external consumers. Do not add a second resolution path.
- [ ] `plan/learned/1107-test-coverage-gaps.md` - Gotchas section, static/vpp interface-only
      nh (named by the deferrals row as the destination)
  → Decision: this is where the VPP half was recorded when deferred.
- [ ] `ai/rules/plugin-self-containment.md` - constrains cross-plugin spelling
  → Constraint: the static plugin must not spell `ifacevpp` internals directly.
- [ ] `ai/rules/no-workarounds-for-missing-behavior.md` - applies directly to R-1
  → Constraint: do not make the `.ci` green by removing the interface-only next-hop.

**Key insights (updated 2026-07-16 after research):**
- The "no backend loaded" error is produced in `dispatch.go:12`, not `resolve.go`. (holds)
- ~~iface backend activation is process-global state, so the fix depends on the static
  plugin's process model, which is not yet established.~~ SUPERSEDED: the process model is
  established (in-process, shared state) and it is NOT the problem. The problems are the
  config-root auto-load gate (A-1b) and an unanticipated startup-tier race (A-1c).
- ~~The VPP encode path already supports `SwIfIndex`; only name resolution is missing.~~
  SUPERSEDED on both halves: name resolution is NOT missing (`iface.Resolve` already returns
  the VPP index, A-3a), and the encode path is NOT already correct (`toFibPath` derives the
  proto from the absent next-hop, A-2a).
- One unresolvable next-hop currently fails the ENTIRE static section AND aborts daemon
  startup. (holds, and is worse than recorded -- observed in the 2026-07-16 `005` run)
- NEW: one resolver already serves both dataplanes; the hazard is that nothing makes the
  iface backend and the static dataplane agree (R-7).

## Current Behavior (MANDATORY)

**Source files read:**

- [ ] `internal/plugins/static/backend_linux.go` - lines 135-147: single next-hop sets
      `route.Gw` when the address is valid, and sets `route.LinkIndex` from
      `resolveNexthopIndex` when `nh.Interface != ""`. Multipath does the same per next-hop
      at lines 150-167. Lines 97-103: `resolveNexthopIndex(name)` calls `iface.Resolve(name)`
      and wraps any error as `interface %q: %w`.
  → Decision: this mechanism is CORRECT and needs no functional change. With an iface
    backend loaded it returns the right index. The linux leg's defects are elsewhere
    (ordering in `register.go`, message quality here). See A-1.
- [ ] `internal/component/iface/resolve.go` - lines 82-100 and 117-142: `Resolve` to
      `resolver.resolve` to `osDeviceFor` to `GetInterface` (line 136).
  → Constraint: `Resolve` is BACKEND-AGNOSTIC. It returns whatever the ACTIVE iface backend
    reports, which is the keystone fact for the whole VPP leg (see next row).
- [ ] `internal/component/iface/resolve.go:214-223` - `bindingFromInfo` sets
      `Ifindex: info.Index` -- it copies the backend's index verbatim, with no kernel
      assumption.
  → Decision: KEYSTONE. Combined with `query.go:232` below, `iface.Resolve(name).Ifindex`
    ALREADY yields a VPP `sw_if_index` when the VPP iface backend is active. The VPP leg
    needs NO new resolver and NO `ifacevpp` export. This is the spec's central finding.
- [ ] `internal/plugins/iface/vpp/query.go:221-240` - `detailsToInfo` builds the
      `iface.InterfaceInfo` returned by the VPP backend's `GetInterface` (`:133-149`) and
      sets `Index: int(d.SwIfIndex)` (`:232`).
  → Constraint: the VPP backend already publishes its `sw_if_index` through the SHARED
    `iface` interface. `resolveIndex`/`b.names` (`ifacevpp.go:225-238`) is an INTERNAL
    fast path, not the only door. Do not export it.
- [ ] `internal/component/iface/dispatch.go` - lines 274-278: `GetInterface` calls
      `backendOrErr()`, which returns `errIfaceNoBackendLoaded` (line 12) when `GetBackend()`
      is nil.
- [ ] `internal/component/iface/backend.go` - lines 266-298: `LoadBackend` is the only writer
      of `activeBackend` (lines 248-250); nothing else sets it. `GetBackend()` at `:301-305`.
      `vppBackendName = "vpp"` (`:41`) is UNEXPORTED and there is NO exported accessor for
      the active backend's NAME (grep: no `ActiveBackendName`).
  → Constraint: a "is the iface backend VPP?" gate (D-1) has no existing public seam. It
    needs a small exported accessor on the iface component.
- [ ] `internal/component/iface/register.go:135-142` - the iface component registers as
      plugin `"interface"` with `ConfigRoots: ["interface"]` and `Dependencies: ["sysctl"]`.
      `:400-417`: `OnConfigure` errors with `errInterfaceNoBackendConfiguredAndNo` (`:34`)
      when `cfg.Backend == ""`, else calls `LoadBackend(cfg.Backend)` (`:414`).
  → Constraint: `LoadBackend` runs ONLY from this config path. NO `interface` stanza means
    NO iface plugin, hence NO backend, hence every `iface.Resolve` fails. There is no
    OS-default fallback (`:180-181` errors instead).
- [ ] `internal/component/plugin/server/startup_autoload.go:78-140` - `getConfigPathPlugins`
      auto-loads a plugin only when a present config path matches its `ConfigRoots`
      (`:94-112`), and marks it `Internal: true` (`:135`).
  → Decision: settles the skeleton's process question. `static` and `interface` are BOTH
    internal, so they share the process and `activeBackend`. A-1's premise is re-framed.
- [ ] `internal/component/plugin/process/process.go:446-461`, `:465-520` -
      `StartWithContext` routes `config.Internal == true` to `startInternal`, which runs the
      plugin's `RunEngine` in a GOROUTINE of the daemon process (`:501-517`).
  → Constraint: PRODUCER of the process model. In-process is confirmed, not inferred.
- [ ] `internal/component/plugin/registry/registry.go:970-1006` - `TopologicalTiers` orders
      startup by `Dependencies` + `OptionalDependencies` only. Optional deps constrain order
      ONLY when the dep is in the resolved set (`:998-1006`).
  → Decision: this is the seam for the ordering fix. `OptionalDependencies` has exactly the
    "order me after it IF it is present" semantics the linux leg needs. Precedent:
    `internal/component/traffic/register.go:148` (`OptionalDependencies: ["vpp"]`).
- [ ] `internal/component/plugin/server/startup.go:263-348` - `runPluginPhase` computes tiers
      (`:300`) and, PER TIER, runs every member's 5-stage handshake CONCURRENTLY in
      goroutines joined only at the end of the tier (`:341-348`).
  → Constraint: RACE PRODUCER. Same-tier plugins have NO ordering guarantee between them.
    `static` (dep `routing-table`) and `interface` (dep `sysctl`) both land in tier 1, so
    static's config stage races the iface backend load. See A-1(b).
- [ ] `internal/plugins/static/register.go:46-61` - `Dependencies: ["routing-table"]`. The
      static plugin does NOT declare `interface`, optional or otherwise.
  → Decision: this is the linux leg's actual defect and the file the fix lands in.
- [ ] `internal/plugins/static/backend_vpp_linux.go` - lines 91-95: `toVPPRoute` returns
      `static/vpp: interface-only next-hop %q needs a VPP sw_if_index (not yet supported)`
      for any next-hop whose `Address.IsValid()` is false. Lines 78-89: blackhole and reject
      return early before the next-hop loop, so they are unaffected. Line 30-42:
      `newVPPStaticBackend` selects VPP purely on `vppcomp.GetActiveConnector() != nil`.
  → Constraint: the VPP static backend and the VPP IFACE backend are selected by two
    INDEPENDENT globals. They can disagree. This produces R-7 / D-1.
- [ ] `internal/plugins/static/vpp/backend.go:111-133` - `toFibPath` selects the path proto
      from the NEXT-HOP address: `p.NextHop.Is4()` -> `PROTO_IP4`, else `PROTO_IP6` +
      `As16()`.
  → Decision: A-2 BREAKER. For an interface-only path `NextHop` is the zero `netip.Addr`,
    whose `Is4()` is false, so the path encodes as `PROTO_IP6` with an all-zero v6 next-hop
    even for an IPv4 route. The VPP leg needs encode work here, not only name resolution.
- [ ] `internal/component/vpp/vpp.go:75-85` - `GetActiveConnector` / `setActiveConnector`
      guard a process-global independent of iface's `activeBackend`.
- [ ] `internal/plugins/static/config.go:184-193` - `parseInterfaceNextHop` sets
      `nh.Interface = ifName`, leaves `nh.Address` unset (invalid), and rejects a BFD profile
      for it ("BFD requires a peer address", `:193`).
  → Constraint: interface-only next-hops are a first-class PARSED form, and BFD is already
    correctly excluded for them. AC-6's BFD behavior is unaffected by this spec.
- [ ] `internal/plugins/static/yang/ze-static-conf.yang:121-131` - `list interface`,
      "Forward via interface only (no gateway address)"; revision note `:17` "Add table
      grouping and interface-only next-hops".
  → Decision: A-4 CONFIRMED by design intent. The form was deliberately added, not
    accidentally accepted. Rejecting it at config-verify would revert a shipped decision.
- [ ] `internal/plugins/static/backend_other.go` - `//go:build !linux`; `newStaticBackend`
      returns `unsupportedStaticBackend` whose every method returns
      `"static routes: not supported on this platform (Linux required)"`.
  → Decision: A-5 BREAKER. On darwin EVERY static route fails here, so `005` never reaches
    `iface.Resolve` at all. The `.ci` is missing `option=needs-linux`.
- [ ] `internal/plugins/static/inject.go` - lines 87-92: `applyRoutes` collects per-route
      errors and joins them.
- [ ] `internal/plugins/static/register.go` - lines 138-141: a non-nil `applyRoutes` result
      becomes an `OnConfigure` error, so `"static routes loaded"` is never logged.

**What actually happens today (verified 2026-07-16 by running `make ze-static-test`):**

| Scenario | Actual behavior | Producer |
|----------|-----------------|----------|
| `005` on darwin (the suite's real state) | FAILS. All 3 routes error `static routes: not supported on this platform (Linux required)`; `OnConfigure` fails; plugin exits 1; daemon aborts startup. Never reaches `iface.Resolve` or `tun100`. | `backend_other.go` `unsupportedStaticBackend`, joined by `inject.go:87-92`, surfaced by `register.go:138-140` |
| linux, interface-only NH, NO `interface` stanza | `iface.Resolve` -> `GetInterface` -> `backendOrErr` returns `iface: no backend loaded`; wrapped as `interface "tun100": iface: no backend loaded`. The message never names the missing `interface { backend ... }` stanza. | `dispatch.go:12`+`:15-21`, wrapped at `backend_linux.go:100` |
| linux, interface-only NH, WITH `interface` stanza | RACE. static and interface share tier 1 and their config stages run concurrently, so `iface.Resolve` may observe `activeBackend == nil` and fail with the same error, nondeterministically. | `startup.go:341-348` + `registry.go:970-1006`; static lacks the dep (`static/register.go:52`) |
| VPP dataplane, interface-only NH | Rejected outright before any `staticvpp.Path` is built. | `backend_vpp_linux.go:91-95` |
| VPP dataplane, address NH | Works; `Path.SwIfIndex` left 0 and `toFibPath` sets proto from the address. | `backend_vpp_linux.go:96-99`, `static/vpp/backend.go:111-133` |

`toVPPRoute` today does exactly ONE thing with an interface next-hop: it tests
`!nh.Address.IsValid()` and returns an error (`backend_vpp_linux.go:93-95`). It never reads
`nh.Interface` except to name it in that error, never calls any resolver, and never
constructs a `staticvpp.Path` for it. The missing resolution is the whole of `:93-95`: there
is no lookup to fix, only an absent one to add.

**Behavior to preserve:**

- Address next-hops (`next { hop 192.168.1.1 { } }`) on both backends, single and ECMP.
- Blackhole and reject actions on both backends (`backend_vpp_linux.go:78-89`,
  `backend_linux.go:122-133`).
- The os-name / mac-match selector remapping the linux backend gets by going through
  `iface.Resolve` rather than a raw `LinkByName`
  (`internal/plugins/static/resolve_integration_linux_test.go:26-59` pins this).
- Failing loudly rather than programming an index-0 path silently
  (`backend_vpp_linux.go:69-72`).
- VPP backend selection when a connector is active (`backend_linux.go:25-37`).

**Behavior to change:**

| # | Change | File | Why (evidence) |
|---|--------|------|----------------|
| C-1 | `static` declares `OptionalDependencies: ["interface"]` so the iface backend is loaded before static applies, WHEN an `interface` stanza exists | `internal/plugins/static/register.go:46-61` | Closes the tier race (`startup.go:341-348`); optional-dep semantics match exactly (`registry.go:998-1006`); precedent `traffic/register.go:148` |
| C-2 | The no-backend error names the missing `interface { backend ... }` stanza instead of the bare `iface: no backend loaded` | `internal/plugins/static/backend_linux.go:97-103` | Today's text (`dispatch.go:12`) is accurate but not actionable; D-2 decides whether this is the final answer or a doctor check joins it |
| C-3 | `toVPPRoute` resolves `nh.Interface` via the SHARED `iface.Resolve` and emits `Path.SwIfIndex`, replacing the rejection | `internal/plugins/static/backend_vpp_linux.go:91-95` | `iface.Resolve` already returns the VPP index (`query.go:232` -> `resolve.go:216`) |
| C-4 | `toVPPRoute` refuses to resolve when the active iface backend is NOT vpp | `backend_vpp_linux.go`, + a new exported accessor on `internal/component/iface` | R-7: the two backends are independent globals (`vpp.go:75`, `iface/backend.go:271`); resolving through a netlink backend would emit a KERNEL ifindex as a VPP `sw_if_index` -- a silently wrong path. ~~Subject to D-1~~ **→ D-1 RESOLVED (a) 2026-07-17: the accessor is `iface.ActiveBackendName() string`; `toVPPRoute` errors when it != `"vpp"` before resolving** |
| C-5 | `toFibPath` selects the path proto from the ROUTE's family when the next-hop address is unset, instead of defaulting to IP6 | `internal/plugins/static/vpp/backend.go:111-133` | A-2 broken: zero `netip.Addr` -> `Is4()==false` -> `PROTO_IP6` for an IPv4 route |
| C-6 | `005-table-interface.ci` gains `option=needs-linux`, an `interface { backend netlink }` stanza, and creates `tun100` | `test/static/005-table-interface.ci` | A-5: it fails on darwin at `backend_other.go`, never reaching the path it claims to test |
| ~~C-7~~ | ~~Per-route error isolation (AC-3)~~ | ~~`internal/plugins/static/inject.go:62-93`~~ | ~~Subject to D-3; today one bad next-hop drops the whole section. **NEW constraint (2026-07-16):** whatever D-3 answers, C-7 must PRESERVE the `routesEqual` diff at `:84-86` -- C-8 depends on it (see R-10)~~ **→ D-3 RESOLVED (a) 2026-07-17: whole-section failure is KEPT and documented. C-7 is OUT OF SCOPE for this spec; per-route isolation moves to a follow-up spec (`plan/spec-fixit-static-per-route-isolation.md`). `inject.go` is NOT modified here** |
| C-8 | **NEW (Thomas, 2026-07-16).** `WantsConfig: []string{pluginName, "interface"}` so the static plugin is delivered the `interface` subtree and the config-validate half becomes buildable | `internal/plugins/static/register.go:224` | `BuildPluginConfigSections` sends only declared roots (`plugin_verify.go:143-157`), so this is the ONLY way static can see interface config. Precedent: `bmp.go:270` declares two roots. Cost owned in R-10 |

→ Decision: C-1 is the linux leg. There is NO change to `resolveNexthopIndex`'s mechanism
  (`backend_linux.go:97-103`) beyond message quality: going through `iface.Resolve` is
  correct and is pinned by `resolve_integration_linux_test.go:26-59`.
→ Decision: `internal/plugins/iface/vpp/ifacevpp.go` is NOT modified. The skeleton listed it;
  research removed it (A-3).

## Data Flow

### Entry Point

- Config: a `static { table <t> { route <prefix> { next { interface <name> { } } } } }`
  stanza, parsed into `staticRoute.NextHops[i].Interface` with an invalid
  (unset) `Address`. Entry via the plugin SDK `OnConfigure` / `OnConfigApply`
  (`internal/plugins/static/register.go:126-144`, `:148-189`).
- Format at entry: `[]sdk.ConfigSection` with `Root == "static"`.

### Transformation Path

1. `parseStaticConfig` builds `[]staticRoute` from the section
   (`internal/plugins/static/register.go:131`).
2. `routeManager.applyRoutes` diffs against the installed set and calls
   `applyRouteLocked` per changed route (`internal/plugins/static/inject.go:62-93`).
3. `programRouteLocked` selects active next-hops and calls `backend.applyRoute`
   (`internal/plugins/static/inject.go:138-171`).
4. Backend split (`internal/plugins/static/backend_linux.go:25-37`): VPP backend when a
   connector is active, else the netlink backend.
5a. Linux: `buildRoute` calls `resolveNexthopIndex` per interface next-hop
    (`backend_linux.go:141`, `:159`), which calls `iface.Resolve` (`:98`), which reaches
    `GetInterface` (`resolve.go:136`) and fails at `backendOrErr` (`dispatch.go:15-21`)
    when `activeBackend` is nil.
5b. VPP: `toVPPRoute` (`backend_vpp_linux.go:73-102`) rejects the interface-only next-hop
    at `:91-95` before any `staticvpp.Path` is built.
6. Error joins back up through `applyRoutes` (`inject.go:87-92`) into the `OnConfigure`
   return (`register.go:138-140`), failing the whole static section.

### Boundaries Crossed

Every row below was read to its producer during research; the "Evidence" column records where.

| Boundary | How | Evidence |
|----------|-----|----------|
| Config to static plugin | `[]sdk.ConfigSection`, root `static`, over the plugin SDK RPC conn | read: `register.go:126-144` |
| Static plugin to iface component | `iface.Resolve(name)` package call reading the `activeBackend` process-global (`backend.go:248`) | read: `backend_linux.go:98` -> `resolve.go:65` |
| Static plugin to kernel | netlink `RouteReplace` via `netlink.Handle` (`backend_linux.go:51`) | read: `backend_linux.go:46-52` |
| Static plugin to VPP | GoVPP channel, `staticvpp.Backend.ApplyRoute` (`backend_vpp_linux.go:49`) | read: `backend_vpp_linux.go:44-50` |
| static/vpp to VPP binapi | `fib_types.FibPath` with `SwIfIndex` (`static/vpp/backend.go:111-115`) | read: proto bug found here (A-2a) |
| Process boundary (RESOLVED) | Both `static` and `interface` auto-load `Internal: true` (`startup_autoload.go:135`) and run via `startInternal` as goroutines of the daemon (`process.go:456-457`, `:465-520`). `activeBackend` IS shared. The subprocess runner (`plugin/cli/cli.go:164`) serves `ze plugin static`, not the auto-load path | read: `process.go:446-520` (producer) |
| Startup ordering (NEW) | Same-tier plugins run their config stage concurrently (`startup.go:341-348`); static declares no `interface` dep, so both sit in tier 1 | read: `startup.go:263-348`, `registry.go:970-1006` |
| iface backend identity (NEW) | NONE exists. `vppBackendName` is unexported (`iface/backend.go:41`) and no `ActiveBackendName` accessor exists. C-4/D-1 must add one | grep: no `ActiveBackendName` in `internal/component/iface/` |

### Integration Points

- `iface.Resolve` (`internal/component/iface/resolve.go:65`) - the existing shared resolver
  the linux static backend already consumes; the fix must extend usage, not duplicate it.
- `staticvpp.Path.SwIfIndex` (`internal/plugins/static/vpp/backend.go:28`) - the existing
  field the VPP fix must populate; already carried to `fib_types.FibPath` by `toFibPath`.
- ~~`resolveIndex` (`internal/plugins/iface/vpp/ifacevpp.go:229`) - the existing VPP name to
  index lookup; currently unexported, so the integration needs a boundary decision.~~
  SUPERSEDED 2026-07-16: not an integration point. `iface.Resolve` already returns the VPP
  index via `GetInterface` -> `detailsToInfo` (`query.go:232`), so `resolveIndex` stays
  private and `ifacevpp.go` is untouched. See A-3.
- `iface.Binding.Ifindex` (`internal/component/iface/resolve.go:214-223`) - the value type
  already returned across the boundary. `bindingFromInfo` copies the ACTIVE backend's index
  verbatim (`:216`), which is what makes one resolver serve both dataplanes.
- `registry.Registration.OptionalDependencies` (`registry.go:998-1006`) - the existing
  startup-ordering seam the linux fix uses (C-1).

### Startup ordering (the linux root cause, verified 2026-07-16)

The failure is not in the resolve chain; it is in WHEN the chain runs relative to
`LoadBackend`. Both plugins auto-load `Internal: true` into the SAME process, so the only
question is order, and order is decided by declared dependencies alone.

| Step | What happens | Producer |
|------|--------------|----------|
| 1 | Config contains `static { }` (and maybe `interface { }`); each matching `ConfigRoots` plugin is auto-loaded `Internal: true` | `startup_autoload.go:94-112`, `:132-136` |
| 2 | `ResolveDependencies` pulls `routing-table` in for `static` (`static/register.go:52`) and `sysctl` in for `interface` (`iface/register.go:140`) | `startup_autoload.go:119` |
| 3 | `TopologicalTiers` sorts on `Dependencies` + `OptionalDependencies` ONLY | `registry.go:970-1006` |
| 4 | Result: tier 0 = {`routing-table`, `sysctl`}, tier 1 = {`static`, `interface`} -- the SAME tier, because neither declares the other | `registry.go:987-1007` |
| 5 | Every member of a tier runs its 5-stage handshake (including Stage 2 config delivery) CONCURRENTLY, joined only at tier end | `startup.go:341-348` |
| 6 | So static's `OnConfigure` -> `iface.Resolve` -> `GetBackend()` races interface's `OnConfigure` -> `LoadBackend` | `static/register.go:126-141` vs `iface/register.go:414` |

→ Decision: the fix is `OptionalDependencies: ["interface"]` on static (C-1), which moves
  static to tier 2 when an `interface` stanza is present and leaves it unconstrained when it
  is absent. This is the mechanism's designed purpose (`registry.go:998-1006`), not a
  workaround.
→ Constraint: NO ordering fix can help the no-stanza case. If there is no `interface`
  stanza there is no backend to wait for, and the error is correct -- only its wording and
  blast radius are in question (D-2, D-3).

### VPP resolution path (as designed, after C-3)

`toVPPRoute` calls the SAME `iface.Resolve` the netlink backend uses. No new resolver, no
`ifacevpp` export, no second resolution path. Each arrow is a verified producer:

| Hop | Producer | Yields |
|-----|----------|--------|
| `nh.Interface` -> `iface.Resolve(name)` | `resolve.go:65`, `:82-100` | cached `Binding` |
| -> `osDeviceFor` -> `GetInterface` | `resolve.go:117-142` (`:136`) | active backend's `InterfaceInfo` |
| -> ifacevpp `GetInterface` -> `detailsToInfo` | `query.go:133-149`, `:221-240` | `Index = int(d.SwIfIndex)` (`:232`) |
| -> `bindingFromInfo` | `resolve.go:214-223` | `Binding.Ifindex` = the VPP index (`:216`) |
| -> `staticvpp.Path.SwIfIndex` | `backend_vpp_linux.go` (new, C-3) | index-carrying path |
| -> `toFibPath` | `static/vpp/backend.go:111-133` | `fib_types.FibPath.SwIfIndex` (`:114`), proto per C-5 |

This honors `plan/learned/950-iface-resolve-2-resolver.md` ("the resolver is the single
owner of logical-name to device resolution for external consumers; do not add a second
resolution path") and `ai/rules/plugin-self-containment.md` (static never spells `ifacevpp`).

→ Constraint: correctness depends on the active iface backend BEING vpp. `iface.Resolve`
  reports whatever backend is loaded, so a netlink-backed resolve under a VPP static
  backend returns a KERNEL ifindex. C-4/D-1 gates this; R-7 records the hazard.

### Architectural Verification

- [ ] No bypassed layers (static keeps resolving through `iface.Resolve`, not raw netlink)
- [ ] No unintended coupling (static does not spell `ifacevpp` internals)
- [ ] No duplicated functionality (no second name-to-index resolver is introduced)
- [ ] Zero-copy preserved where applicable (value `Binding`, no cross-boundary pointers)
- [ ] Registration over hardcoding: any new resolver reaches the static plugin through an
      existing registry / component boundary; no per-backend switch case or factory is added
      to a core/shared package (small-core/registration; `ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The linux failure is reachable in production, not only in a test config missing an `interface` stanza | Call chain read (`dispatch.go:12` to `register.go:414`); `activeBackend` is a process-global (`backend.go:248`) | Problem A shrinks to a test-config fix and the spec is mostly Problem B | Determine whether `static` runs in-process or as a subprocess in a normal `ze` run | **confirmed (re-framed)** 2026-07-16 |
| A-1a | (supersedes A-1's premise) `static` runs IN-process and therefore SHARES `activeBackend` with the iface component | `startup_autoload.go:132-136` (`Internal: true`); `process.go:456-457` -> `startInternal` `:465-520` runs `RunEngine` in a daemon goroutine | The spec's process-boundary framing would have been right | Read the producer `Process.StartWithContext` | **confirmed** |
| A-1b | With NO `interface` stanza the iface plugin never loads, so `iface.Resolve` always fails | `iface/register.go:139` (`ConfigRoots: ["interface"]`) + `startup_autoload.go:94-112` gate; `LoadBackend` called only at `register.go:414`/`:575`; no OS-default fallback (`:180-181`) | Some other path loads a default backend | Grep every `LoadBackend` caller | **confirmed** |
| A-1c | With an `interface` stanza, static's apply RACES the iface backend load | `static/register.go:52` lacks an `interface` dep; `registry.go:970-1006` orders on deps only, putting both in tier 1; `startup.go:341-348` runs tier members concurrently | The ordering is already safe and only A-1b matters | Read `runPluginPhase` + `TopologicalTiers` | **confirmed (new, unanticipated)** |
| A-2 | `staticvpp.Path.SwIfIndex` plus `toFibPath` is sufficient to program an interface-scoped VPP path with no further encode work | `static/vpp/backend.go:24-30`, `:111-115`, `static/vpp/translate_test.go:54-62` | The VPP leg grows to include FibPath flags / proto work for the no-address case | A real VPP apply with an index-only path | **BROKEN** 2026-07-16 |
| A-2a | (replaces A-2) `toFibPath` mis-encodes an interface-only path: proto comes from the unset next-hop, so an IPv4 route becomes `PROTO_IP6` | `static/vpp/backend.go:111-133` reads `p.NextHop.Is4()`; a zero `netip.Addr` returns `IsValid()=false`, `Is4()=false`, `As16()=all-zero` (executed 2026-07-16). `translate_test.go:53-68` only ever passes a VALID next-hop, so it never covered this | The encode is fine and C-5 is unnecessary | Executed a Go program against `netip.Addr{}`; a unit test on `toFibPath` with a zero next-hop will pin it | **confirmed broken** |
| A-3 | The iface/vpp name to index map is reachable from the static plugin's process and is populated when static applies | `resolveIndex` exists (`ifacevpp.go:229`) but is unexported and channel-gated (`ensureChannel`) | The resolver must be exported AND its lifetime coordinated with static's apply ordering | Trace who owns `b.names` and when it is filled (`ifacevpp/query.go:194` mentions a construction-time dump) | **BROKEN (moot)** 2026-07-16 |
| A-3a | (replaces A-3) static needs NO access to `b.names`/`resolveIndex`: the shared `iface.Resolve` already returns the VPP `sw_if_index` | `query.go:221-240` sets `Index: int(d.SwIfIndex)` (`:232`); `resolve.go:214-223` copies it to `Binding.Ifindex` (`:216`); `GetInterface` is a `Backend` interface method (`query.go:133`) | The VPP leg needs an export and R-4 returns | Unit test: interface-only NH under a fake iface backend reporting index N yields `Path.SwIfIndex == N` | **confirmed** |
| A-4 | Interface-only next-hops are a form operators actually use, so this is worth fixing rather than rejecting at config-verify | The YANG accepts it and `test/static/005-table-interface.ci:68-72` was written for it | The cheaper correct answer is a config-verify rejection with a clear message | User / operator input | **confirmed (design intent)**; operator-demand half still open for Thomas |
| A-4a | The form is deliberate, not an accident of a permissive schema | `yang/ze-static-conf.yang:121-131` declares `list interface` "Forward via interface only (no gateway address)"; revision `:17` "Add table grouping and interface-only next-hops"; `config.go:184-193` parses it and rejects BFD for it (`:193`). Kernel precedent: `ip route add default dev tun100` is the normal way to route over a P2P tunnel | Rejecting at config-verify would be a cheap fix | Read the YANG + parser | **confirmed** |
| A-5 | `test/static/005-table-interface.ci` currently fails | Mechanism read above predicts it; NOT run (research-only spec) | The mechanism analysis is wrong somewhere and needs re-reading before design | Run the `.ci` and read the actual error | **confirmed, WRONG REASON** 2026-07-16 |
| A-5a | (replaces A-5) `005` fails on darwin at the PLATFORM guard, never reaching `iface.Resolve` or `tun100` | `make ze-static-test` 2026-07-16: `static: apply route failed ... error="static routes: not supported on this platform (Linux required)"` x3, then `OnConfigure` fails, plugin exits 1, daemon aborts. Producer: `backend_other.go` `unsupportedStaticBackend`. The `.ci` lacks `option=needs-linux` (siblings `001`-`003` have it) | The failure would already be the iface one and C-6 would be smaller | Ran the suite; log kept in the session scratchpad | **confirmed** |
| A-5b | The static suite is not in the default gate, which is why `005` (and `004`) rotted unnoticed | `mk/test-functional.mk:20` "(release evidence only)"; `:49` names static as excluded; `mk/test-release.mk:71` runs it only as release extra | The reds would have been caught earlier and mean something else | Read the makefiles | **confirmed** |
| A-6 | The two problems share enough design to stay one spec | Both are the same user-visible symptom on one next-hop form | Split into two specs at DESIGN | DESIGN review | **confirmed (stronger than assumed)** |
| A-6a | The two legs now share ONE mechanism, so splitting would duplicate the design | Both legs resolve through `iface.Resolve` (`backend_linux.go:98`; C-3) and both depend on the same backend-ordering/identity questions (C-1, C-4) | Split into two specs | DESIGN review | **confirmed** |
| A-8 | **NEW (C-8).** Re-running static's verify/apply on an unrelated `interface` change is idempotent and cheap enough to accept | `register.go:110-124`/`:148-157` no-op today when no `static` section is delivered (`pendingRoutes` stays nil, `:155` early-returns); after the feature lands, `applyRoutes` (`inject.go:62-93`) skips unchanged routes via `routesEqual` (`diff.go:10-25`), which compares next-hop `Address` AND `Interface` | C-8's accepted cost would be wrong: every interface edit would reprogram routes. Thomas would need to re-decide | Read the producers (done 2026-07-16). At implementation: a test delivering an interface-only reload asserts zero `applyRouteLocked` calls | **confirmed** 2026-07-16 |
| A-9 | **NEW (C-8).** `VerifyBudget: 1` / `ApplyBudget: 2` (`register.go:225-226`) need no change under C-8 | `registration.go:72-73` documents them as "Estimated verify/apply time in seconds". C-8 makes static participate in MORE transactions but makes each interface-triggered run CHEAPER than a static-triggered one (A-8), so the existing worst-case estimates stay valid | The budgets under-estimate and a reload transaction times out on interface edits | Confirm no timeout regression when the full functional suite runs under C-1+C-8 (R-8 already mandates the full suite) | **confirmed by reading; runtime unvalidated** |
| A-7 | `tun100` does not exist in the test environment and nothing in `005` creates it | `005-table-interface.ci:52-74`: the config declares only `routing-table` and `static`; no `interface` stanza, no device creation; the driver (`:9-50`) only polls and queries | A device already exists and R-2 is moot | Read the `.ci` in full | **confirmed** |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Fixing the test by adding an `interface { backend netlink }` stanza masks a real production gap instead of fixing it | The stanza makes the test green with no product change | **Live.** A-1 is settled: the stanza is REQUIRED for AC-1 (there is no backend without it), but it is not sufficient and must not be the only change. C-1 (ordering) and C-6 (`needs-linux` + device creation) are product/test fixes that stand on their own, and AC-2 keeps a separate test for the no-stanza case so the gap stays visible. See `ai/rules/no-workarounds-for-missing-behavior.md` |
| R-2 | The device (`tun100`) does not exist in the test environment, so a backend alone will not make the test pass | Resolve fails with a device-absent error rather than a no-backend error | **Live, confirmed by A-7.** `005` must create the device. A dummy link is the cheapest substitute (`ai/rules/qemu-testing.md`: "Network interface -> veth pair or dummy"); a real `tun100` is not needed to prove index resolution |
| R-3 | Whole-section failure on one bad next-hop is a bigger operational problem than the next-hop form itself | An operator loses all static routes because one interface is absent | **Live.** D-3 decides. Today `inject.go:87-92` joins every per-route error and `register.go:138-140` turns it into a startup abort -- verified in the `005` run, where 3 route failures killed the daemon |
| R-4 | Exporting a VPP name to index resolver leaks iface/vpp internals into the static plugin | The change requires the static plugin to spell `ifacevpp` | **RETIRED 2026-07-16.** A-3a: no export is needed; static uses the shared `iface.Resolve`, and `ifacevpp.go` is not modified |
| R-5 | The two problems are only superficially related and want separate designs | The linux fix is config-shaped, the VPP fix is resolver-shaped | **RETIRED 2026-07-16.** A-6a: both legs resolve through one mechanism (`iface.Resolve`); splitting would duplicate the design |
| R-6 | A VPP fix cannot be proven without a live VPP, so it lands unverified | No QEMU/VPP path exercises interface-only static next-hops | **Live.** No VPP static rail exists (`test/vpp/` has no static-route case; `internal/plugins/static/vpp` has only unit tests). Mitigation: prove C-3/C-5 by unit tests at the two seams (`toVPPRoute` with a fake iface backend; `toFibPath` proto for a zero next-hop). Do NOT claim the FIB write is proven end-to-end without a rail; record it as a Known Limitation |
| R-7 | **NEW.** The static dataplane and the iface backend are chosen by two independent globals, so a VPP static backend can resolve names against a NETLINK iface backend and emit a kernel ifindex as a VPP `sw_if_index` -- a silently wrong path | Config has a VPP connector plus `interface { backend netlink }`; the route programs "successfully" to the wrong VPP interface | C-4/D-1: gate resolution on the active iface backend being vpp, and error otherwise. Producers: `backend_vpp_linux.go:31` (`GetActiveConnector`) vs `iface/register.go:414` (`LoadBackend`); `vpp.go:75-85` and `iface/backend.go:248` are unrelated globals. This is the same class of hazard as the index-0 trap `backend_vpp_linux.go:69-72` guards |
| R-8 | **NEW.** `OptionalDependencies: ["interface"]` changes daemon startup ordering for EVERY config that has both stanzas, not just interface-next-hop ones | Startup-order-sensitive tests shift; a latent ordering assumption elsewhere breaks | The dependency is optional and additive (`registry.go:998-1006` constrains only when present), and it moves static LATER, never earlier. Run the full functional suite, not just `test/static/` |
| R-9 | **NEW.** `iface.Resolve` caches per logical name (`resolve.go:82-100`) and the cache is invalidated by monitor link events. A name resolved BEFORE the device exists could cache a failure or a stale index | A static route keeps a stale ifindex after an interface is recreated | Read `resolve.go`'s invalidation path before implementing; failures are NOT cached today (`:90-93` returns before the cache write), so the risk is stale-index-after-recreate, which the monitor's `LinkEvent` path is designed to handle. Confirm during implementation |
| R-10 | **NEW (the OWNED cost of C-8, accepted by Thomas 2026-07-16).** Widening `WantsConfig` to include `interface` enqueues static into the reload transaction on EVERY `interface` change, including edits that cannot affect any static route (an MTU tweak). Its verify/apply re-runs where today it would not run at all | An `interface`-only config edit shows static in the reload transaction's participant set / its `VerifyBudget`+`ApplyBudget` charged to that transaction | **Accepted, not mitigated away.** The cost is bounded by two independent mechanisms, both verified at their producers: (1) `applyRoutes` is diff-based -- `routesEqual` (`diff.go:10-25`) short-circuits every unchanged route at `inject.go:84-86`, so an MTU tweak yields NO netlink/VPP call and NO redistribute event; (2) `rootHasChanges` (`reload.go:297-319`) still gates on the `interface` root actually changing, so unrelated roots (`bgp`, `firewall`) never enqueue static. Net cost: a JSON parse plus a map diff. **NOT route churn, NOT a FIB rewrite.** Guard: R-10 is only true while (1) holds -- see the C-7 constraint |
| R-11 | **NEW.** C-8 makes static's handlers receive section lists that contain `interface` but NOT `static` (an interface-only reload builds a section only per CHANGED root, `reload.go:224-243`). Feature code that assumes a `static` section is always present would nil-deref or silently wipe the route set | An interface-only reload logs a static error, or drops all static routes | Today's handlers are already correct by construction (`register.go:113`, `:128` skip non-`static` roots; `:155` early-returns on nil `pendingRoutes`). The risk is a REGRESSION introduced by the feature work, not an existing bug. Pin it with a test that delivers an interface-only section list and asserts the installed route set is unchanged |

### Decisions Needed (BLOCKING approval -- Thomas)

The design is complete except for three choices that change scope. ~~The spec stays `design`
until they are settled.~~ **RESOLVED 2026-07-17 -- all three are settled (D-2 by Thomas; D-1
and D-3 by autonomous default). See "→ Autonomous Resolutions (2026-07-17)" immediately below
the table. The spec is now `ready`.**

| ID | Question | Options | Recommendation |
|----|----------|---------|----------------|
| D-1 | How to stop a VPP static backend resolving names against a netlink iface backend (R-7)? | (a) Runtime gate: add a small exported accessor to the iface component (e.g. `iface.ActiveBackendName() string`, set by `LoadBackend`) and have `toVPPRoute` error when it is not `vpp`. (b) Config-verify rejection: refuse a config that pairs a VPP connector with a non-vpp iface backend. (c) Do nothing and accept the hazard. | **(a)**, and consider (b) later as a separate hardening. (a) is local, cheap, keeps "never program a wrong index" (`backend_vpp_linux.go:69-72`), and needs no cross-plugin spelling. (c) is rejected: it silently programs a wrong path, the exact failure the current code refuses. (b) alone cannot cover a backend switched at runtime (`iface/register.go:574-578`) |
| D-2 | Should an interface next-hop with no loadable iface backend stay a runtime error (A-1b), or become a config-verify rejection / doctor check? | (a) Runtime error with an actionable message (C-2). (b) Also register a doctor check for the runtime dependency (`ai/rules/doctor-checks.md`). (c) Config-verify rejection. | **(a) + (b)**. (c) is not available to the static plugin as it stands: `WantsConfig: ["static"]` (`register.go:224`) means it never sees the `interface` section, so it cannot know whether a backend will load. (b) makes the dependency discoverable without a new cross-section validator |
| D-3 | Should one unresolvable next-hop fail the whole static section (today, `inject.go:87-92`), or only its own route (AC-3)? | (a) Keep whole-section failure, document it. (b) Isolate per-route: log and skip the bad route, keep the rest. (c) Out of scope; split to its own spec. | ~~**Needs Thomas.**~~ **→ RESOLVED (a) 2026-07-17 [STAKES: scope], see "→ Autonomous Resolutions" below.** (b) is what an operator wants (R-3: one absent interface should not drop every static route) but it changes an established failure contract for ALL static routes, not just this next-hop form, and interacts with `OnConfigApply`'s journal/rollback (`register.go:148-189`). (b) is deferred to `plan/spec-fixit-static-per-route-isolation.md` per `ai/rules/deferral-tracking.md` |

→ Constraint: AC-3 as written ("blast radius is a deliberate, documented choice") is
  satisfiable by (a) or (b); it is NOT satisfiable by leaving the question unanswered.

### → Autonomous Resolutions (2026-07-17, readiness pass; APPEND-ONLY, override if wrong)

These settle D-1, D-2 and D-3 so a fresh implementer can start with zero questions. Every
cited `file:line` was re-verified against source on 2026-07-17.

**→ AUTONOMOUS DEFAULT (2026-07-17) -- D-1 [STAKES: arch]: adopt option (a).** Add a small
exported accessor `iface.ActiveBackendName() string` to the iface component (recorded by
`LoadBackend`, `internal/component/iface/backend.go:271-298`), and have `toVPPRoute`
(`internal/plugins/static/backend_vpp_linux.go:91-95`) return an error when the active iface
backend name is not `"vpp"` BEFORE it resolves the interface name. Rationale: this is the
spec's own RECOMMEND; it is local and cheap, keeps "never program a wrong index-0 path"
(`backend_vpp_linux.go:69-72`), and needs no cross-plugin spelling
(`ai/rules/plugin-self-containment.md`). The accessor exposes an EXISTING concept, it does not
invent one: `iface.DefaultBackendName()` already exists as an exported accessor at
`backend.go:239`, and the package already gates internally on `vppBackendName = "vpp"`
(`backend.go:41`, whose doc comment says it exists precisely so a handler can gate on "is the
active backend vpp?"). Option (b) (config-verify rejection of a VPP-connector +
non-vpp-iface-backend pairing) cannot cover a backend switched at runtime -- `LoadBackend`
(`backend.go:271-298`) is re-entrant and swaps `activeBackend` live -- and option (c) silently
programs a wrong path (the exact failure `backend_vpp_linux.go:69-72` exists to prevent). Both
(b) and (c) rejected. **Thomas: override if wrong** (if you also want the config-verify pairing
check as belt-and-braces, that is additive hardening on top of (a), not a replacement).

**→ CONFIRMED (2026-07-17) -- D-2 [STAKES: low]: already ANSWERED by Thomas 2026-07-16 as BOTH
halves; the spec body already reflects it in full.** (1) Widen `WantsConfig` to
`["static", "interface"]` (C-8, `internal/plugins/static/register.go:224`; precedent
`internal/component/bgp/plugins/bmp/bmp.go:270` ships two roots) so the interface REFERENCE can
be config-validated where possible; AND (2) keep the runtime error (C-2) PLUS a doctor check
(D-2 option (b)) as the backstop for the cases config-validation structurally cannot close (an
interface-only next-hop may legitimately name an externally-created interface ze does not
configure, and resolution still needs a runtime ifindex lookup). Verified the spec records
this across "→ Decision (user, 2026-07-16): validate at config time ... AND still handle
resolution failure at runtime", "→ Decision (user, 2026-07-16): WIDEN `WantsConfig` ...
ANSWERED", and C-8 / R-10 / R-11 / A-8 / A-9. **Consequence: the doctor check is IN SCOPE for
this spec, not conditional** -- every "only if D-2 picks (b)" is now settled YES. No new
decision needed.

**→ AUTONOMOUS DEFAULT (2026-07-17) -- D-3 [STAKES: scope]: adopt option (a) -- KEEP
whole-section failure and DOCUMENT it as a deliberate blast-radius choice.** Rationale: it is
the least-change option. It does NOT alter the established failure contract that ALREADY
governs ALL static routes: per-route errors are joined (`internal/plugins/static/inject.go:92`
`errors.Join`) and a non-nil result becomes the `OnConfigure` error
(`internal/plugins/static/register.go:138-140`). AC-3 ("blast radius is a deliberate,
documented choice") is satisfied by (a) per the Constraint above. Per-route isolation (option
(b)) is the operator-friendlier behavior (R-3), but it changes that contract for EVERY static
route, not just interface-only next-hops; it interacts with `OnConfigApply`'s journal/rollback
(`register.go:148-189`); and it MUST preserve the `routesEqual` diff (`diff.go:10-25`,
short-circuited at `inject.go:84-86`) that C-8 / R-10 depend on. That is a scope too large to
fold into this spec. **Option (b) is recorded as a NOTED FOLLOW-UP requiring its own spec**
(`ai/rules/deferral-tracking.md`): destination `plan/spec-fixit-static-per-route-isolation.md`
(to be created -- per-route error isolation for ALL static routes, not only this next-hop
form). **Consequence for THIS spec: C-7 and its `inject.go` edit are OUT OF SCOPE; Phase 6 is
NOT executed; `inject.go` is NOT in Files to Modify. AC-3 becomes a DOCUMENTATION deliverable**
(record the whole-section blast radius in `plan/learned/650-static-routes.md` and the
static-routes doc). **Thomas: override if wrong** -- if you want per-route isolation folded
into this spec, re-open DESIGN (per `ai/rules/no-partial-completion.md` this scope change needs
your approval, which is why the default is the least-change (a)).

### -> Decision (user, 2026-07-16): validate at config time where possible, AND still handle resolution failure at runtime

**This SUPERSEDES the D-2/D-3 framing above.** Thomas asked: *"can we not validate the route
and ensure that it can not be invalid?"* The honest answer is **partly**, and the spec must
record both halves rather than pick one.

**Why config-time validation is not possible TODAY (the mechanism, verified at the
producers 2026-07-16):**

| Step | Producer | What it does |
|------|----------|--------------|
| Declaration | `internal/plugins/static/register.go:224` | `WantsConfig: []string{pluginName}`, and `pluginName = "static"` (`register.go:24`). The static plugin declares exactly one config root |
| Delivery | `internal/component/plugin/server/startup.go:701-705` | `if len(reg.WantsConfigRoots) > 0 ... BuildPluginConfigSections(configTree, reg.WantsConfigRoots)` -- the ONLY source of a plugin's config payload |
| The cut | `internal/component/config/plugin_verify.go:143-157` `BuildPluginConfigSections` | Iterates **only over the declared roots**, calling `ExtractConfigSubtree(configTree, root)` per root and marshalling each into one `rpc.ConfigSection`. Nothing outside the declared roots is ever sent |

-> Constraint: therefore the static plugin **receives ONLY the `static` section and is
structurally blind to `interface` config**. It cannot today check whether `tun100` is
declared, because the string `tun100`'s declaration lives in a section it never sees. This
is a **structural** limit, not a missing `if`: no amount of code inside the static plugin
can reach config it is not sent.

**Why widening `WantsConfig` would help (and is mechanically available):** declaring
`WantsConfig: []string{"static", "interface"}` would deliver the `interface` subtree
alongside `static`, letting config-verify catch **the common case: a typo'd interface
name**. Multi-root is a supported, in-use shape, not a new mechanism:
`internal/component/bgp/plugins/bmp/bmp.go:270` already declares
`WantsConfig: []string{"bgp", "environment"}`, and `internal/component/iface/register.go:717`
confirms `"interface"` is the root's name.

-> Constraint: **"cannot be invalid" is NOT reachable, and the spec must not promise it.**
Two independent reasons, either alone sufficient:
1. An interface next-hop may legitimately name an interface **ze does not configure at
   all** -- an externally-created tunnel. Such a route is correct and must keep working, so
   "the name must appear in ze's `interface` section" cannot be a hard rejection without
   breaking a valid deployment.
2. Resolution needs a **runtime ifindex lookup**. An interface can be declared in config and
   still be absent, renamed, or down when the route is programmed.
   So a route can be **config-valid and runtime-unresolvable**. Validation narrows the
   failure window; it cannot close it.

-> Decision (user, 2026-07-16): **config-validate the REFERENCE, and still fail gracefully
at runtime.** Both, not either. The runtime path (D-2's (a)+(b): actionable error + doctor
check) is NOT superseded or made redundant by config validation -- it remains the backstop
for cases 1 and 2 above. Any implementation that adds config validation and then treats the
runtime failure as unreachable is wrong.

-> Constraint: this reframes D-2. Option (c) ("config-verify rejection") was rejected on the
grounds that it "is not available to the static plugin **as it stands**" -- correct as
written, but the qualifier is load-bearing: it is unavailable **because of a declaration the
spec can change** (`register.go:224`), not because of a law of the architecture. The
decision above is not D-2's (c) either: it is validate-the-reference **plus** keep the
runtime error, whereas (c) proposed rejection **instead of** it.

-> Constraint: D-3 (blast radius) is **not** answered by this decision and stays open.
Config-time validation of the reference makes the whole-section-failure question *less
frequent*, never moot: reasons 1 and 2 above still produce runtime failures that D-3 governs.

~~**-> OPEN QUESTION FOR THOMAS (new, raised by this decision): is widening `WantsConfig` to
include `"interface"` acceptable?** It changes what config the static plugin receives, which
is a real coupling decision, not a detail:~~
- ~~It gives static a read dependency on the `interface` section (read-only; `WantsConfig` is
  "roots this plugin reads, not owner" per `internal/component/config/transaction/orchestrator.go:58`).~~
- ~~It widens the reload surface: `reload.go:214-226` selects affected plugins by matching
  changed roots against `WantsConfigRoots`, so static would be reconfigured on **every**
  `interface` change, not just `static` ones.~~
- ~~Without the widening, the config-validate half of this decision **cannot be implemented at
  all** and only the runtime half survives.~~
  ~~Not actioned by the recording session: this is Thomas's call.~~

### -> Decision (user, 2026-07-16): WIDEN `WantsConfig` to `["static", "interface"]`. ANSWERED.

**Thomas approved the widening**, on the grounds that without it the config-validate half of
the decision above is not buildable at all: no amount of code inside the static plugin can
reach a section it is never sent. The question above is CLOSED; the cost is accepted with
open eyes and recorded as C-8 / R-10 / A-8 below rather than left to be rediscovered.

**Mechanism (every hop verified at its producer 2026-07-16):**

| Step | Producer | What it does |
|------|----------|--------------|
| Today's declaration | `internal/plugins/static/register.go:224` | `WantsConfig: []string{pluginName}` -- exactly one root, `"static"` (`register.go:24`) |
| Why that blinds static | `internal/component/config/plugin_verify.go:143-157` `BuildPluginConfigSections` | `for _, root := range roots { subtree := ExtractConfigSubtree(configTree, root) ... }` -- it iterates **only the declared roots**. A section outside them is never marshalled and never sent. This is the structural cut |
| Multi-root is already in use | `internal/component/bgp/plugins/bmp/bmp.go:270` | `WantsConfig: []string{"bgp", "environment"}` -- two roots, shipping today. The widening uses an existing shape, it does not invent one |

-> Constraint: the change is a ONE-LINE declaration edit at `register.go:224`. It is
mechanically available and carries no new mechanism. What it does carry is a runtime cost,
below, which is OWNED as of this decision.

**The OWNED cost, measured at the producers (2026-07-16). It is MILDER than the
characterisation Thomas approved on, and the spec records the true shape:**

| Hop | Producer | What actually happens |
|-----|----------|----------------------|
| Reload selects affected plugins | `internal/component/plugin/server/reload.go:214-226` | Iterates every process's `reg.WantsConfigRoots` and builds a section per root. Confirmed: matching IS by declared root |
| ...but only for roots that changed | `reload.go:227` -> `rootHasChanges` (`:297-319`) | `if !rootHasChanges(diff, root) { continue }`. `rootHasChanges` prefix-matches the root against `diff.Added` / `Removed` / `Changed`. So static is enqueued on an `interface` change, and ONLY then; an unrelated `bgp` edit does not touch it |
| A plugin with no changed section is dropped | `reload.go:245-247` | `if len(sections) > 0` gates the `affected` append. So the transaction only carries plugins with a genuinely changed root |

-> Constraint: **so yes, an unrelated MTU tweak DOES enqueue static into the reload
transaction.** That half of the characterisation is correct and is the accepted cost. What it
does NOT do is rewrite the FIB, and the spec must not let a later session assume it does.

**Is the re-apply idempotent and cheap? YES, at two independent layers:**

| Layer | Producer | Why the re-apply is a no-op |
|-------|----------|----------------------------|
| 1. Today, before the feature exists | `register.go:110-124` (`OnConfigVerify`) and `:148-157` (`OnConfigApply`) | Verify skips any section whose `Root != pluginName` (`:113`), so an interface-only change leaves `pendingRoutes` nil. Apply then hits `if newRoutes == nil { return nil }` (`:155-157`) and returns before touching the journal or the route manager. A pure no-op |
| 2. After the feature exists, when static DOES consume `interface` | `inject.go:62-93` (`applyRoutes`) | `applyRoutes` is DIFF-BASED, not a rewrite: it removes only keys absent from the new set (`:72-79`), and for each incoming route `if existing != nil && routesEqual(existing.route, r) { continue }` (`:84-86`) skips it entirely. No `applyRouteLocked`, no `programRouteLocked`, no netlink/VPP call, no redistribute event |
| The equality test is the right one | `diff.go:10-25` (`routesEqual`) | Compares table/action/metric/tag/description and the SORTED next-hop set including `Address` **and** `Interface` (`:17-23`, `sortedNextHops` `:27-37`). So a genuine interface-derived next-hop change IS detected and reprogrammed; an MTU tweak yields equal routes and is skipped |

-> Decision (user, 2026-07-16): the cost is **a parse plus a map diff on every `interface`
edit**, not route churn and not a FIB rewrite. It is accepted. The two layers above are the
REASON it is acceptable, so neither may be removed casually: layer 2 (`routesEqual`) is what
stands between this decision and the route churn Thomas was warned about.

-> Constraint: **`routesEqual` (`diff.go:10-25`) is load-bearing for this decision.** Any
future change that makes `applyRoutes` unconditional, or that drops the `routesEqual`
short-circuit at `inject.go:84-86`, converts every interface edit into a full static-route
reprogram. If C-7 (D-3, per-route isolation) reworks `inject.go:62-93`, it MUST preserve the
diff, and that is now a constraint on D-3's answer, not a detail of it.

-> Constraint: the real recurring charge is the **transaction budget**, not the dataplane.
`VerifyBudget: 1` / `ApplyBudget: 2` (`register.go:225-226`) are "estimated verify/apply time
in seconds" (`internal/component/plugin/registration.go:72-73`). Widening `WantsConfig` means
static joins the reload transaction for every `interface` change and charges its 1s/2s
estimate to that transaction's budget. The estimates stay CORRECT (the work shrinks, it does
not grow), so no budget edit is required by this decision; the change is that they are now
charged more often. If implementation shows the interface-triggered path is measurably
cheaper than the static-triggered one, the budgets are estimates for the WORST case and
should stay as they are.

-> Constraint: static must keep tolerating a section list that does NOT contain `static`. On
an interface-only reload the transaction delivers `[{Root: "interface"}]` alone
(`reload.go:224-243` builds a section only per CHANGED root). Today's handlers already do
this correctly by construction (`register.go:113`, `:128` skip non-`static` roots; `:155`
early-returns). The feature work must not regress it by assuming a `static` section is always
present.

## Wiring Test (MANDATORY)

Rows confirmed by the 2026-07-16 research unless marked.

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `static { table lns { route 0.0.0.0/0 { next { interface tun100 } } } }` + `interface { backend netlink }`, device present | -> | `resolveNexthopIndex` (`backend_linux.go:97`) then `route.LinkIndex` (`:145`) | `test/static/005-table-interface.ci` (needs C-6: `needs-linux`, stanza, device) |
| Same config, no `interface` stanza (A-1b) | -> | `backendOrErr` (`dispatch.go:15`) error, re-worded by C-2, surfaced to the operator | `test/static/006-interface-nexthop-no-backend.ci` (new) |
| Both stanzas present, startup ordering (A-1c) | -> | `OptionalDependencies: ["interface"]` (`static/register.go:46-61`) placing static in a later tier than `interface` | `TestStaticDeclaresOptionalInterfaceDependency` (unit, on the registration) + `005` as the live proof |
| Same config, VPP data plane active, iface backend vpp | -> | `toVPPRoute` (`backend_vpp_linux.go:73`) resolving via `iface.Resolve` and emitting `Path.SwIfIndex` | `TestToVPPRouteInterfaceOnlyNextHopResolvesIndex` |
| Interface-only next-hop, interface unknown to the iface backend | -> | `toVPPRoute` rejection path (`backend_vpp_linux.go:91`) | `TestToVPPRouteInterfaceOnlyUnknownInterfaceErrors` |
| VPP static backend + NON-vpp iface backend (R-7) | -> | C-4 gate in `toVPPRoute` ~~(shape per D-1)~~ **(D-1 = (a) resolved 2026-07-17: error when `iface.ActiveBackendName() != "vpp"`)** | `TestToVPPRouteRefusesResolveWhenIfaceBackendNotVPP` |
| Interface-only next-hop on an IPv4 route, VPP encode (A-2a) | -> | `toFibPath` proto selection (`static/vpp/backend.go:111-133`) | `TestToFibPathInterfaceOnlyIPv4UsesIP4Proto` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Static route with an interface-only next-hop, linux backend, iface backend loaded, device present | Route programs with `route.LinkIndex` set to the resolved ifindex |
| AC-2 | Static route with an interface-only next-hop, linux backend, no iface backend loaded | Diagnosable failure naming the missing `interface { backend ... }` stanza, not the bare `iface: no backend loaded` (`dispatch.go:12`). ~~Shape per D-2~~ **D-2 = (a)+(b) (resolved): actionable runtime error (C-2) AND a doctor check** |
| AC-3 | Static section mixing a good address-next-hop route and an unresolvable interface next-hop | Blast radius is a deliberate, documented choice; today the whole section fails AND aborts daemon startup (`inject.go:87-92` -> `register.go:138-140`, observed in the 2026-07-16 `005` run). ~~Resolved by D-3~~ **D-3 = (a) (resolved 2026-07-17): whole-section failure is KEPT and DOCUMENTED; this AC is a documentation deliverable, no `inject.go` change** |
| AC-4 | Static route with an interface-only next-hop, VPP backend, iface backend vpp, interface known | `toVPPRoute` emits a `staticvpp.Path` with the `SwIfIndex` resolved via `iface.Resolve`, and `toFibPath` encodes it with the ROUTE's proto (AC-9) |
| AC-5 | Static route with an interface-only next-hop, VPP backend, interface unknown | Clear error naming the interface; never an index-0 path |
| AC-6 | Address next-hops, ECMP, blackhole, reject, on both backends; BFD on address next-hops | Unchanged. (BFD is already excluded for interface-only next-hops at `config.go:193`, so this spec does not touch BFD) |
| AC-7 | `test/static/005-table-interface.ci` | Passes IN QEMU (`option=needs-linux`), exercising the interface-only next-hop for real rather than working around it. Must fail first for the RIGHT reason: the current darwin failure (A-5a) proves nothing |
| AC-8 | **NEW.** Config has both `static` and `interface` stanzas | Static's apply happens AFTER the iface backend is loaded, deterministically, not by tier accident (A-1c). Asserted on the registration, not by repeated runs |
| AC-9 | **NEW.** Interface-only next-hop on an IPv4 route, VPP encode | `FibPath.Proto == FIB_API_PATH_NH_PROTO_IP4`, never IP6-by-default (A-2a). The IPv6 equivalent yields IP6 |
| AC-10 | **NEW.** VPP static backend active while the iface backend is NOT vpp (R-7) | Resolution is refused with a clear error; a kernel ifindex is NEVER emitted as a VPP `sw_if_index`. ~~Shape per D-1~~ **D-1 = (a) (resolved 2026-07-17): `toVPPRoute` errors when `iface.ActiveBackendName() != "vpp"`** |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|---------------------|-----------------------|
| 1 | Configures a default route out of a tunnel with no gateway address, linux data plane (`ip route add default dev tun100`, the normal P2P-tunnel idiom) | config -> parseStaticConfig -> applyRoutes -> buildRoute -> resolveNexthopIndex -> iface.Resolve -> netlink RouteReplace. Requires the iface backend loaded FIRST (C-1) | `test/static/005-table-interface.ci` (in QEMU, after C-6) |
| 2 | Same config on a VPP data plane | config -> applyRoutes -> toVPPRoute -> iface.Resolve (same shared resolver, returning the VPP `sw_if_index` via `query.go:232`) -> staticvpp.Path.SwIfIndex -> toFibPath (proto from the route family, C-5) -> VPP FIB | `TestToVPPRouteInterfaceOnlyNextHopResolvesIndex` + `TestToFibPathInterfaceOnlyIPv4UsesIP4Proto`. FIB write itself unproven (R-6, Known Limitations) |
| 3 | Configures an interface next-hop with no `interface` stanza anywhere in the config | config -> resolveNexthopIndex -> iface.Resolve -> backendOrErr error -> operator-visible message naming the missing stanza (C-2) | `test/static/006-interface-nexthop-no-backend.ci` (new) |
| 4 | Configures an interface next-hop for an interface that does not exist, backend loaded | config -> resolveNexthopIndex -> iface.Resolve -> device-absent error naming the interface | `TestToVPPRouteInterfaceOnlyUnknownInterfaceErrors` (VPP side); linux side covered by `005`'s device creation failing loudly if the device is absent |
| 5 | Runs a VPP dataplane but leaves `interface { backend netlink }` | toVPPRoute refuses to resolve rather than emitting a kernel ifindex as a VPP index (C-4) | `TestToVPPRouteRefusesResolveWhenIfaceBackendNotVPP` (shape per D-1) |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestToVPPRouteInterfaceOnlyNextHopResolvesIndex` | `internal/plugins/static/backend_vpp_linux_test.go` | AC-4. Interface-only next-hop produces a `Path` with the resolved `SwIfIndex`, replacing the `:113` rejection assertion | proposed |
| `TestToVPPRouteInterfaceOnlyUnknownInterfaceErrors` | `internal/plugins/static/backend_vpp_linux_test.go` | AC-5. An unknown interface errors and never yields index 0 | proposed |
| `TestToVPPRouteMixedAddressAndInterfaceNextHops` | `internal/plugins/static/backend_vpp_linux_test.go` | ECMP mixing an address next-hop and an interface next-hop | proposed |
| `TestToVPPRouteRefusesResolveWhenIfaceBackendNotVPP` | `internal/plugins/static/backend_vpp_linux_test.go` | AC-10 / R-7. A non-vpp iface backend under a VPP static backend refuses to resolve rather than emitting a kernel ifindex | proposed (shape per D-1) |
| `TestToFibPathInterfaceOnlyIPv4UsesIP4Proto` | `internal/plugins/static/vpp/translate_test.go` | AC-9 / A-2a. A zero next-hop + `SwIfIndex` on an IPv4 route encodes `PROTO_IP4`. Sibling: the IPv6 case. Closes the hole `translate_test.go:53-68` left by only passing valid next-hops | proposed |
| `TestStaticDeclaresOptionalInterfaceDependency` | `internal/plugins/static/register_test.go` | AC-8 / A-1c. The registration declares `interface` as an optional dependency, so `TopologicalTiers` orders static after it when present. Asserts the REGISTRATION, since the race itself is not reliably observable by re-running | proposed |
| `TestResolveNexthopIndexNoBackendErrorIsDiagnosable` | `internal/plugins/static/backend_linux_test.go` | AC-2 / C-2. The no-backend error names the interface AND the missing `interface { backend ... }` stanza | proposed |

→ Constraint: `backend_vpp_linux_test.go` and `backend_linux_test.go` are `//go:build linux`
  and the VPP tests must not need a live VPP. The `iface.Resolve` seam is what makes this
  testable: a fake `iface.Backend` registered via `iface.RegisterBackend` + `LoadBackend`
  can report any index, so the translation is provable on any Linux host. Verify during
  implementation that a test-only backend can be loaded without racing other tests that use
  the same process-global (`iface/backend.go:248-250`).

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `Path.SwIfIndex` | 0 to 2^32-1 (`static/vpp/backend.go:28`) | index of a real interface | 0 is the trap value: it must never be emitted for an unresolved name | N/A |
| `Path.Weight` | 0 to 255 after `capWeight` (`backend_vpp_linux.go:104-111`) | 255 | N/A | uint16 inputs above 255 cap to 255 (existing behavior, preserve) |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `005-table-interface` | `test/static/005-table-interface.ci` | Named-table route with an interface-only next-hop loads and programs | exists, **FAILING today at the platform guard, not the target path** (A-5a). C-6 must add `option=needs-linux`, an `interface { backend netlink }` stanza, and create `tun100` (a dummy link suffices, R-2) |
| `006-interface-nexthop-no-backend` | `test/static/006-interface-nexthop-no-backend.ci` | An interface next-hop with no iface backend gives a clear, diagnosable error | proposed; keeps A-1b's gap visible so C-6 cannot become the workaround R-1 warns about. `option=needs-linux` (it boots a daemon that applies static config) |
| `004-show` | `test/static/004-show.ci` | (Not this spec's target) | **FAILING today for an UNRELATED reason**: `load config: parse config: line 7: unknown field in route: next-hop`. Its config uses a `next-hop` field the parser no longer accepts. Out of scope; see Design Insights |

→ Constraint: the whole `test/static/` suite is release-evidence-only (`mk/test-functional.mk:20`,
  `:49`; `mk/test-release.mk:71`), which is why both reds went unnoticed (A-5b). After C-6,
  `005`/`006` are `needs-linux` and therefore validated by `make ze-qemu-needs-linux-test` /
  `make ze-qemu-all-test` (`ai/rules/qemu-testing.md`), NOT by a native run. Whether the
  static suite should join the default gate is a question for Thomas, not a silent change
  by this spec.

### Interop Tests

Not applicable. This spec changes no wire protocol behavior: it is data-plane route
programming (netlink and VPP binapi), not a BGP/IPsec/L2TP wire format change.

### Future

None deferred. This is a skeleton; scope is set at DESIGN.

## Files to Modify

Confirmed by the 2026-07-16 research.

- `internal/plugins/static/backend_vpp_linux.go` - **C-3/C-4.** `toVPPRoute` resolves
  `nh.Interface` via the shared `iface.Resolve` and emits `Path.SwIfIndex` instead of
  rejecting (`:91-95`), gated on the active iface backend being vpp (D-1). The file already
  imports neither `iface` nor `ifacevpp`; C-3 adds the `iface` component import, the same one
  `backend_linux.go:12` already uses.
- `internal/plugins/static/vpp/backend.go` - **C-5 (NEW, not in the skeleton).** `toFibPath`
  must pick the path proto from the route family when the next-hop address is unset
  (`:111-133`). Without this an interface-only IPv4 route encodes as `PROTO_IP6` (A-2a).
- `internal/plugins/static/register.go` - **C-1 (NEW, not in the skeleton).** Add
  `OptionalDependencies: ["interface"]` to the registration (`:46-61`) to close the tier
  race (A-1c). **C-8 (NEW, Thomas 2026-07-16):** widen `WantsConfig` at `:224` from
  `[]string{pluginName}` to `[]string{pluginName, "interface"}`. One-line declaration edit;
  `BuildPluginConfigSections` (`plugin_verify.go:143-157`) then delivers the `interface`
  subtree too. Precedent `bmp.go:270`. Budgets at `:225-226` are unchanged (A-9). The verify
  and configure handlers (`:110-124`, `:126-144`) keep their `Root != pluginName` skips, so
  the new section is inert until the config-validate leg consumes it (R-11).
- `internal/component/iface/` - **C-4 (NEW, ~~subject to D-1~~ D-1 RESOLVED (a) 2026-07-17).**
  Add a small exported accessor `iface.ActiveBackendName() string` for the active backend's
  NAME. None exists today: `vppBackendName` is unexported (`backend.go:41`) and `LoadBackend`
  (`:271-298`) does not record the name -- so the accessor must record it (e.g. store the name
  alongside the swap at `backend.go:291`). Precedent for the exported-accessor shape:
  `iface.DefaultBackendName()` at `backend.go:239`. Additive; no existing caller changes.
- `internal/plugins/static/backend_linux.go` - **C-2, message only.** No mechanism change:
  `resolveNexthopIndex` (`:97-103`) routing through `iface.Resolve` is correct and is pinned
  by `resolve_integration_linux_test.go:26-59`. Reword the wrap at `:100` to name the
  missing stanza.
- ~~`internal/plugins/iface/vpp/ifacevpp.go` - expose name to `SwIfIndex` resolution across
  the plugin boundary (`resolveIndex`, `:225-238`), subject to R-4~~
  **REMOVED 2026-07-16.** A-3a: `iface.Resolve` already returns the VPP index
  (`query.go:232` -> `resolve.go:216`). No export, no change to this file, R-4 retired.
  This also removes the only overlap with `plan/spec-fixit-vpp-lcp-reachability.md`.
- ~~`internal/plugins/static/inject.go` - **C-7, only if D-3 chooses per-route isolation**
  (`:62-93`).~~ **→ D-3 RESOLVED (a) 2026-07-17: whole-section failure KEPT and documented;
  `inject.go` is NOT modified by this spec. Per-route isolation (C-7) is a follow-up spec,
  `plan/spec-fixit-static-per-route-isolation.md`.**
- `test/static/005-table-interface.ci` - **C-6.** `option=needs-linux` + an
  `interface { backend netlink }` stanza + create `tun100`. Coordinate with
  `plan/spec-fixit-sleeps-cli-harness.md`, which also edits this file (see Design Insights).
- `plan/learned/650-static-routes.md` - the `// Design:` anchor on both static backends
  (`backend_linux.go:1`, `backend_vpp_linux.go:1`) and on `static/vpp/backend.go:1` and
  `backend_other.go:1`. Updates needed (do NOT edit during design; recorded here):
  - `## Consequences` line 22 says "VPP and kernel backends are independent". After C-3 that
    is no longer wholly true: both now depend on the shared `iface` resolver for interface
    next-hops. Amend rather than delete.
  - `## Consequences` line 21 ("auto-loads when config contains `static { }`") gains the
    C-1 nuance: static now also orders itself after `interface` when that stanza is present.
  - `## Decisions` line 13 (the VPP sub-package's own `Path` type with capped `uint8` weight)
    stays true; C-5 adds that the proto is route-derived when the next-hop is address-less.
  - A new `## Gotchas` entry for A-2a (zero `netip.Addr` silently means IPv6 in
    `toFibPath`) and for R-7 (the two dataplane globals can disagree).
  - **NEW (D-3 = (a), 2026-07-17): document the AC-3 blast radius** -- one unresolvable
    next-hop fails the WHOLE static section (`inject.go:92` `errors.Join` ->
    `register.go:138-140`) and aborts daemon startup. Record this as a deliberate, documented
    choice (whole-section failure kept; per-route isolation deferred to
    `plan/spec-fixit-static-per-route-isolation.md`). This is AC-3's documentation deliverable.
  → Constraint: per `ai/rules/planning.md` Spec Closure, if this spec's learned summary
    supersedes 650 on these points, the `// Design:` anchors must still resolve. Amend 650;
    do not repoint the anchors.

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] **No.** Confirmed: `list interface` already exists (`ze-static-conf.yang:121-131`) and `parseInterfaceNextHop` (`config.go:184-193`) already parses it. This spec adds no leaf | `internal/plugins/static/yang/` |
| Doctor check for runtime dependencies | [ ] **Yes (D-2 = (a)+(b), resolved).** ~~Recommended Yes, pending D-2.~~ An interface next-hop depends on a loaded iface backend, which is a runtime dependency per `ai/rules/doctor-checks.md`. The check belongs to the STATIC plugin (Proximity Principle), not to iface | `internal/plugins/static/` + `internal/core/diagnostic/codes.go` |
| Plugin dependency declaration | [ ] **Yes (C-1, NEW).** `OptionalDependencies: ["interface"]`. Not a "registration over hardcoding" violation: it uses the existing declarative seam and adds no switch/case to shared code | `internal/plugins/static/register.go:46-61` |
| iface active-backend accessor | [ ] **Yes if D-1 picks (a).** Additive export on the iface component; no existing caller changes | `internal/component/iface/backend.go` |
| Functional test | [ ] **Yes**, and both must be `option=needs-linux` (they boot a daemon applying Linux-only config, per `ai/rules/qemu-testing.md`) | `test/static/005-*.ci`, `test/static/006-*.ci` |
| CLI commands/flags | [ ] No | - |
| Prometheus counters | [ ] No | - |

### Documentation Update Checklist

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] Decide at DESIGN (a next-hop form starts working) | `docs/features.md` |
| 2 | Config syntax changed? | [ ] Expected No (syntax already exists) | `docs/guide/configuration.md` |
| 6 | Has a user guide page? | [ ] Check static routes guide coverage | `docs/guide/<topic>.md` |
| 12 | Internal architecture changed? | [ ] If a resolver crosses a new boundary | `plan/learned/650-static-routes.md`, `docs/architecture/core-design.md` |
| 16 | Any changed source file referenced by doc source anchors? | [ ] Grep `docs/` for the changed files | per grep |

## Files to Create

- `test/static/006-interface-nexthop-no-backend.ci` - AC-2. Confirmed needed under the D-2
  recommendation (runtime error + doctor check). It is what keeps A-1b's gap visible after
  C-6 adds an `interface` stanza to `005`, so the test fix cannot quietly become the
  workaround R-1 warns about. Must carry `option=needs-linux`.
- A doctor check + its unit test in `internal/plugins/static/` - ~~only if D-2 picks (b)~~
  **IN SCOPE (D-2 = (a)+(b), resolved 2026-07-17).**

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 13. /ze-review gate | Review Gate section |

### Implementation Phases

**Phase 0 (research) is COMPLETE (2026-07-16).** The phases below are the settled design.
~~BLOCKING: D-1, D-2 and D-3 must be answered by Thomas before Phase 3 and Phase 5; Phases 1-2
and 4 are unaffected by all three.~~ **→ RESOLVED 2026-07-17: D-1, D-2 and D-3 are all
answered (see "→ Autonomous Resolutions"). D-2 = (a)+(b): Phase 3 includes the doctor check.
D-1 = (a): Phase 5 adds the `iface.ActiveBackendName()` gate. D-3 = (a): Phase 6 is DROPPED
(whole-section failure kept and documented). No phase is blocked.**

1. **Phase 1: Wiring / make the red honest** (no product change yet).
   - Add `option=needs-linux` to `005-table-interface.ci` so it stops failing at the
     platform guard (A-5a) and starts running where the path exists.
   - Run `make ze-qemu-needs-linux-test`. Record the ACTUAL error. It should now be the
     iface one (`interface "tun100": iface: no backend loaded`) or a device-absent error.
   - Gate: do not proceed until `005` fails for the RIGHT reason. If it fails for a fourth
     reason, STOP (see Failure Routing).
   - Files: `test/static/005-table-interface.ci`
2. **Phase 2: Linux ordering (C-1)** -- the actual linux product fix.
   - Test first: `TestStaticDeclaresOptionalInterfaceDependency` (AC-8).
   - Add `OptionalDependencies: ["interface"]` to static's registration.
   - Files: `internal/plugins/static/register.go`
   - Verify: `TopologicalTiers` places `static` in a later tier than `interface` when both
     are present, and in an unconstrained tier when `interface` is absent.
3. **Phase 3: Linux diagnosis (C-2, + doctor check ~~per D-2~~ D-2 = (a)+(b) resolved 2026-07-17)**.
   - Tests: `TestResolveNexthopIndexNoBackendErrorIsDiagnosable` (AC-2);
     `test/static/006-interface-nexthop-no-backend.ci`; a unit test for the doctor check.
   - Files: `internal/plugins/static/backend_linux.go`, plus a doctor check in the static
     plugin's own package ~~if D-2 picks (b)~~ **(D-2 (b) chosen -- IN SCOPE)**
     (`ai/rules/doctor-checks.md`, `ai/rules/plugin-self-containment.md`: the check belongs to
     static, and its code to `internal/core/diagnostic/codes.go`).
4. **Phase 4: VPP encode (C-5)** -- independent of D-1/D-2/D-3, do it early.
   - Test first: `TestToFibPathInterfaceOnlyIPv4UsesIP4Proto` + the IPv6 sibling (AC-9).
   - Fix `toFibPath` to derive the proto from the route family when the next-hop is unset.
   - Files: `internal/plugins/static/vpp/backend.go`
   - Note: `toFibPath` takes only a `Path` today, so the route family must reach it. Prefer
     passing it explicitly over inferring from `SwIfIndex != 0`; `buildFibPaths` (`:91-109`)
     already has the `Route`.
5. **Phase 5: VPP resolution (C-3, C-4)**.
   - Tests: `TestToVPPRouteInterfaceOnlyNextHopResolvesIndex` (AC-4),
     `TestToVPPRouteInterfaceOnlyUnknownInterfaceErrors` (AC-5),
     `TestToVPPRouteRefusesResolveWhenIfaceBackendNotVPP` (AC-10),
     `TestToVPPRouteMixedAddressAndInterfaceNextHops`.
   - Replace the `:91-95` rejection with a resolve through `iface.Resolve`, gated per D-1 =
     (a): `toVPPRoute` errors when `iface.ActiveBackendName() != "vpp"` before resolving.
   - Files: `internal/plugins/static/backend_vpp_linux.go`, `internal/component/iface/`
     (the D-1 accessor `ActiveBackendName`).
   - Verify: an unresolved name NEVER yields `SwIfIndex` 0.
6. ~~**Phase 6: Blast radius (C-7)** -- only if D-3 picks per-route isolation.~~
   ~~- Tests: AC-3's mixed-section case.~~
   ~~- Files: `internal/plugins/static/inject.go`~~
   **→ D-3 RESOLVED (a) 2026-07-17: Phase 6 is NOT executed by this spec.** Whole-section
   failure is kept and documented (AC-3 is a documentation deliverable, Phase 9). Per-route
   isolation is a follow-up spec, `plan/spec-fixit-static-per-route-isolation.md`.
7. **Phase 7: Functional close**: `005` passes in QEMU because the product works, plus `006`.
8. **Phase 8: Full verification**: `make ze-verify`, `make ze-qemu-needs-linux-test`, and
   `make ze-static-test`. R-8: run the FULL functional suite, not just `test/static/`,
   because C-1 shifts startup ordering for every config carrying both stanzas.
9. **Phase 9: Complete spec**: amend `plan/learned/650-static-routes.md` per Files to Modify,
   write the learned summary, two-commit closure.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | An unresolved name NEVER produces `SwIfIndex` 0 (`backend_vpp_linux.go:69-72` rationale) |
| Data flow | Static still resolves via `iface.Resolve` only; no second resolver introduced |
| Registration over hardcoding | Any new resolver crosses the boundary via an existing registry / component seam; no per-backend switch case or factory added to a core/shared package (`ai/rules/plugin-self-containment.md`) |
| Doctor checks | If the iface-backend dependency is made explicit, a doctor check is registered per `ai/rules/doctor-checks.md` |
| Rule: no-workarounds | `005-table-interface.ci` passes because the product works, not because the test was weakened |
| Blast radius | The whole-section-failure decision (AC-3) is deliberate and documented |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Interface-only next-hop programs on linux | `test/static/005-table-interface.ci` passes |
| Interface-only next-hop programs on VPP | `TestToVPPRouteInterfaceOnlyNextHopResolvesIndex` passes |
| No index-0 path is reachable | `grep -n "SwIfIndex" internal/plugins/static/backend_vpp_linux.go` plus the unknown-interface test |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | A config-supplied interface name reaches a kernel/VPP index lookup; an unknown or malformed name must error, never resolve to index 0 (which would program a wrong path) |
| Error leakage | Error messages name the interface; confirm they carry no unexpected host detail |

### Failure Routing

| Failure | Route To |
|---------|----------|
| ~~`005-table-interface.ci` fails for a third reason not analyzed here~~ | **THIS FIRED (2026-07-16).** It failed for exactly that: the darwin platform guard (A-5a). Research was re-run and the producer read; the design below reflects it. The rule now applies to a FOURTH reason: if `005` fails in QEMU for anything other than the iface/device error Phase 1 predicts, STOP and re-research |
| VPP leg cannot be proven without live VPP | **Confirmed: no rail exists** (Q10). Prove C-3/C-5 at the translation seams by unit test, record the FIB write as a Known Limitation, and do NOT claim end-to-end. Building a VPP static rail is a separate spec |
| D-1/D-2/D-3 answered in a way that changes scope | Re-open DESIGN; do not improvise. Per `ai/rules/no-partial-completion.md` scope reduction needs explicit approval |
| C-1 shifts startup ordering and breaks an unrelated test (R-8) | Do NOT remove the dependency to make the test green. The dependency is correct; the other test's ordering assumption is the bug. Report it |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| (A-2) `SwIfIndex` + `toFibPath` already suffice for an interface-scoped VPP path; only parent-side lookup is missing | `toFibPath` (`static/vpp/backend.go:111-133`) derives the path proto from the NEXT-HOP address. An interface-only path's next-hop is the zero `netip.Addr` (`Is4()==false`), so an IPv4 route would encode as `PROTO_IP6` with an all-zero v6 next-hop | Read the producer instead of trusting the cited test; `translate_test.go:53-68` only ever passes a VALID next-hop, so it never covered the case it was cited for. Confirmed by executing a Go program against `netip.Addr{}` | VPP leg grows by C-5. Had this shipped as assumed, interface-only IPv4 routes would encode with the wrong proto -- a silent wrong-path bug of exactly the class `backend_vpp_linux.go:69-72` exists to prevent |
| (A-3) The iface/vpp name map must be reached from static, so `resolveIndex` needs exporting (R-4) | Static needs none of it. The VPP backend already publishes `sw_if_index` through the shared `iface` interface (`query.go:232` -> `resolve.go:216`), so `iface.Resolve` -- the call the netlink backend ALREADY makes -- returns it | Traced `Binding.Ifindex` back to its producer (`bindingFromInfo`) and then to the VPP backend's `GetInterface`, rather than assuming the VPP index only lived in `b.names` | VPP leg SHRINKS: `ifacevpp.go` leaves Files to Modify, R-4 retired, the overlap with `spec-fixit-vpp-lcp-reachability.md` disappears, and the "no second resolution path" constraint of `plan/learned/950` is honored for free |
| (A-5) `005` fails at `iface.Resolve` with "iface: no backend loaded" | It fails far earlier, on darwin, at the non-linux platform guard (`backend_other.go`), never reaching `iface.Resolve` or `tun100`. The `.ci` lacks `option=needs-linux` that its siblings `001`-`003` carry | RAN `make ze-static-test` instead of predicting from a call-chain read | C-6 grows (`needs-linux` first, before anything else can be observed). Also surfaced that `004-show` is independently red and that the whole suite is outside the default gate (A-5b) |
| (A-1) The linux failure might be a process-boundary issue (static as a subprocess cannot see `activeBackend`) | Static runs IN-process and shares `activeBackend`. The real causes are the `ConfigRoots` auto-load gate (no `interface` stanza -> no backend at all) and a startup-tier RACE nobody had noticed | Read `Process.StartWithContext` (`process.go:446-461`) and `runPluginPhase` (`startup.go:263-348`), the producers, rather than reasoning from the plugin SDK's RPC shape | The linux fix moved file: from `backend_linux.go` (message only) to `register.go` (a missing `OptionalDependencies` edge). The race would have survived a "fix" that only reworded the error |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- The deferrals row for the VPP half (2026-07-10, W-3) says the work "needs the iface/vpp
  name to sw_if_index resolver threaded into the static backend". ~~A code read narrows this:
  `staticvpp.Path.SwIfIndex` and `toFibPath` already exist and are tested
  (`static/vpp/translate_test.go:54-62`), so only the parent-side lookup is missing.~~
  **Research 2026-07-16 narrows it further AND widens it:** no resolver needs threading at
  all (the shared `iface.Resolve` already returns the VPP index, A-3a), but `toFibPath` is
  NOT already correct for this case (A-2a). The deferrals row pointed at the wrong file in
  both directions.
- ~~The linux half was recorded as a static problem, but the mechanism is entirely in the
  iface component's process-global backend activation. The static plugin is the victim, not
  the cause.~~ **SUPERSEDED.** The static plugin IS the cause: it consumes a component whose
  readiness it never declares a dependency on (`static/register.go:52`). The process-global
  is fine; the missing `OptionalDependencies` edge is the bug.
- **One resolver, two dataplanes.** The deepest finding is that `iface.Resolve` is already
  dataplane-agnostic: `bindingFromInfo` copies whatever index the active backend reports
  (`resolve.go:216`), and the VPP backend reports its `sw_if_index` there (`query.go:232`).
  The VPP "gap" was never a missing lookup; it was a missing CALL. Any future consumer
  needing a dataplane-correct interface index should reach for `iface.Resolve` first.
- **The corollary hazard (R-7).** Because one call serves both dataplanes, its correctness
  depends on the iface backend and the consumer's dataplane agreeing, and nothing enforces
  that: `vpp.GetActiveConnector()` and `iface.activeBackend` are independent globals. This is
  a general trap for any future VPP-aware consumer of `iface.Resolve`, not a static-only one.
- **Two independent tests were red and nobody knew** (A-5b). `test/static/` is
  release-evidence-only, so `004` (stale config field) and `005` (missing `needs-linux`)
  both rotted. A test excluded from the default gate decays to documentation.
- **The skeleton's value was its uncertainty markers.** Every one of its `unvalidated`
  assumptions that mattered (A-2, A-3, A-5) turned out wrong. Had they been written as
  findings, the VPP leg would have been implemented against `ifacevpp` internals (R-4) with
  a silently wrong proto (A-2a).

## Cross-Spec Interactions

| Spec | Overlap | Resolution |
|------|---------|------------|
| `plan/spec-fixit-vpp-lcp-reachability.md` (concurrent) | Both read `internal/plugins/iface/vpp/` | **NO conflict.** That spec touches `iface/vpp/doctor.go` and `lcp.go`; this spec's research REMOVED `ifacevpp.go` from Files to Modify (A-3a). The two no longer share a file. If D-1 picks option (a), this spec adds an accessor to `internal/component/iface/` (a different package) -- still no overlap |
| `plan/spec-fixit-sleeps-cli-harness.md` (skeleton, unowned) | Both edit `test/static/005-table-interface.ci` | **Real but ordered.** That spec's own Required Reading (`:62-63`) already records that `005` is blocked on THIS spec's outcome. This spec changes `005`'s options + config (C-6: `needs-linux`, `interface` stanza, device creation); that spec changes its driver's blind sleeps (`005:24`). Land this spec first: it decides whether `005`'s config is valid at all, and moving `005` to `needs-linux` changes where the harness must work (QEMU, not native). Its A-4 worry ("interface-only might be rejected at config-verify, turning 005 into a rewrite") is now settled NO -- the form stays (A-4a), so `005` remains a harness transplant, not a rewrite |
| `plan/learned/1107-test-coverage-gaps.md` | Records the VPP half when deferred | Its Gotchas entry should be corrected at closure: the gap is not "resolver needs threading" (A-3a) |

## Known Limitations

- Recursive next-hop resolution is out of scope; this spec covers interface-only next-hops.
- **The VPP FIB write is not proven end-to-end** (R-6, Q10). No `.ci` or QEMU rail exercises
  a VPP-backed static route, and building one is out of scope. C-3/C-5 are proven at the
  translation seams by unit tests (`toVPPRoute` against a fake iface backend, `toFibPath`
  proto). The spec must NOT claim the VPP leg is end-to-end verified.
- **Whether VPP wants an explicit attached/P2P path TYPE for an address-less path is
  unsettled** (Q8). C-5 fixes the proto, which is demonstrably wrong today; if a live VPP
  later shows `FIB_API_PATH_TYPE_*` is also needed, that is a follow-up, not a silent
  addition here.
- Sibling `iface.Resolve` consumers (e.g. `internal/plugins/ldp/`) are not audited (Q9).

## Open Questions (ANSWERED 2026-07-16 unless marked OPEN)

| # | Question | Answer | Evidence |
|---|----------|--------|----------|
| 1 | Does `static` run in-process (sharing `activeBackend`) or as a subprocess? | **IN-PROCESS.** So `activeBackend` IS shared, and the process-boundary hypothesis is dead. Problem A is a product bug, but for two other reasons (A-1b, A-1c). | `startup_autoload.go:132-136` (`Internal: true`); `process.go:456-457` -> `startInternal` `:465-520` runs `RunEngine` in a daemon goroutine |
| 2 | Does `005` fail today, and with WHICH error? | **FAILS, with a THIRD error the spec never considered:** `static routes: not supported on this platform (Linux required)`. It dies at the platform guard on darwin and never reaches `iface.Resolve` or `tun100`. | `make ze-static-test` run 2026-07-16; producer `backend_other.go`. Also `004-show` fails unrelatedly (stale `next-hop` config field) |
| 3 | Is `tun100` expected to exist, and who creates it? | **Nobody creates it.** The `.ci` declares only `routing-table` + `static`; no device is made. C-6 must create one (a dummy link suffices). | `005-table-interface.ci:52-74`; A-7 |
| 4 | Config-verify rejection or runtime error for the no-backend case? | ~~**Runtime error + doctor check recommended (D-2).** Config-verify is NOT available to the static plugin: `WantsConfig: ["static"]` means it never sees the `interface` section, so it cannot know whether a backend will load.~~ **SUPERSEDED -> Decision (user, 2026-07-16): BOTH -- config-validate the reference where possible, AND still handle resolution failure at runtime.** The "not available" finding is correct but its qualifier matters: config-verify is unavailable **as it stands**, because of a declaration the spec can change (`register.go:224`), not a law of the architecture. Widening `WantsConfig` to `["static", "interface"]` (precedent: `bmp.go:270` declares two roots) would deliver the `interface` subtree and catch the common typo'd-name case. It would NOT make routes un-invalidatable: an interface next-hop may name an externally-created interface ze does not configure, and resolution still needs a runtime ifindex lookup. So the runtime error + doctor check STAY as the backstop. ~~**New open question for Thomas: is widening `WantsConfig` acceptable?**~~ **ANSWERED (user, 2026-07-16): YES -- widen to `["static", "interface"]` (C-8).** Cost accepted and recorded in R-10/R-11/A-8/A-9. See Design Decisions -> "Decision (user, 2026-07-16): WIDEN `WantsConfig`" | `static/register.go:224`, `plugin_verify.go:143-157`, `startup.go:701-705`; supersedes D-2 |
| 5 | Whole-section failure or per-route isolation? | ~~**OPEN -- D-3, needs Thomas.**~~ **RESOLVED 2026-07-17 (autonomous default, [STAKES: scope]): D-3 = (a) -- KEEP whole-section failure and DOCUMENT it (least-change; AC-3 satisfied). Per-route isolation (b) is a NOTED FOLLOW-UP, `plan/spec-fixit-static-per-route-isolation.md`.** Today's behavior is worse than the skeleton recorded: it fails the section AND aborts daemon startup. | `inject.go:87-92` -> `register.go:138-140`; observed in the `005` run |
| 6 | What is the right boundary for VPP name -> `sw_if_index` resolution? | **No new boundary.** The existing shared `iface.Resolve` already returns the VPP index; static uses the same call the netlink backend uses. `resolveIndex` stays private, `ifacevpp.go` is untouched, R-4 retired. | `query.go:232` -> `resolve.go:216`; A-3a |
| 7 | Is `b.names` populated when static applies (ordering)? | **MOOT for `b.names`** (static never touches it). But the ORDERING question is real and was the hidden linux bug: static races the iface backend load because it declares no dependency on `interface`. | A-1c; `startup.go:341-348`, `registry.go:970-1006` |
| 8 | Does an interface-only next-hop need a FibPath flag beyond `SwIfIndex`? | **YES -- the proto.** A zero `Nh.Address` does NOT do the right thing: `toFibPath` reads `p.NextHop.Is4()`, so an address-less path silently encodes as `PROTO_IP6`. Whether an explicit attached/P2P path TYPE is also wanted is a VPP-semantics question to settle against a live VPP (see Known Limitations). | `static/vpp/backend.go:111-133`; zero-`netip.Addr` behavior executed 2026-07-16; A-2a |
| 9 | Is there an equivalent gap in other `iface.Resolve` consumers (e.g. `ldp`)? | **OPEN, deliberately out of scope.** ~~Flagged for Thomas as a possible follow-up.~~ **→ CONFIRMED out of scope 2026-07-17 (readiness pass, [STAKES: scope], smaller-scope default): NOT implementation-blocking for this spec, which is bounded to static; recorded as a possible follow-up, not silently widened.** Not investigated: this spec is bounded to static. | `ai/rules/no-partial-completion.md`: scope changes need approval |
| 10 | How is a VPP-backed static route proven today, and is there a rail? | **There is NO rail.** `internal/plugins/static/vpp` has unit tests only, and no `.ci` exercises a VPP static route. R-6 stands: the VPP leg is provable at the translation seams (unit) but its FIB write is not provable end-to-end without a rail. Do not claim otherwise. | R-6; Known Limitations |

### Still open for Thomas

- ~~**Widening `WantsConfig` to include `"interface"`**~~ **ANSWERED 2026-07-16: approved
  (C-8).** Cost owned in R-10/R-11, A-8/A-9.
- ~~**D-1, D-2, D-3** (see Decisions Needed). D-3 now carries an extra constraint from C-8:
  any rework of `inject.go:62-93` must preserve the `routesEqual` diff (R-10).~~
  **RESOLVED 2026-07-17 -- see "→ Autonomous Resolutions (2026-07-17)" under Decisions Needed.
  D-2 answered by Thomas 2026-07-16; D-1 = (a) and D-3 = (a) by autonomous default. Because
  D-3 = (a) keeps whole-section failure, `inject.go` is NOT reworked by this spec, so the
  C-8/`routesEqual` constraint is moot here; it becomes a constraint on the follow-up spec
  `plan/spec-fixit-static-per-route-isolation.md`.**
- ~~**Q9**: fix sibling `iface.Resolve` consumers in the same work, or leave them?~~
  **→ AUTONOMOUS DEFAULT (2026-07-17, [STAKES: scope]): LEAVE them -- out of scope for this
  spec (bounded to static), recorded as a possible follow-up. Not implementation-blocking.**
- **A-4's operator half**: the YANG proves the form was designed deliberately (A-4a), but
  whether operators actually rely on it is a product judgement, not a code fact.
  **→ AUTONOMOUS DEFAULT (2026-07-17, [STAKES: scope]): KEEP the form (do NOT reject it at
  config-verify) -- A-4a confirms deliberate design intent (`ze-static-conf.yang:121-131`).
  Operator-demand is a product judgement, NOT an implementation blocker; the design proceeds.**
- **A-5b**: should `test/static/` join the default functional gate? Two tests rotted red
  because it is release-evidence-only. Out of scope here; flagged, not silently changed.
  **→ AUTONOMOUS DEFAULT (2026-07-17, [STAKES: scope]): OUT OF SCOPE for this spec -- do not
  change the gate here; flagged for Thomas as a separate decision. Smaller-scope, self-contained
  default. Not implementation-blocking.**

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete (every row has a concrete test name, none deferred)
- [ ] `/ze-review` gate clean (Review Gate section filled: 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
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
- [ ] Interop tests for protocol features (N/A: no wire protocol change)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING, before ANY commit)
- [ ] Critical Review passes: all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only
