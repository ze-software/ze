# Spec: bgp-netns -- Default VPP LCP namespace to host for v1.0

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P1 |
| Release reason | Default VPP LCP configuration isolates TAP mirrors in `"dataplane"` where BGP and other routing protocols cannot bind; defaulting `vpp.lcp.netns` to `""` makes VPP LCP and routing protocols work together out of the box for v1.0. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner decisions (2026-11-24, supersedes 2026-07-16 rulings)

| Topic | Decision |
|-------|----------|
| Per-protocol `bgp { listen { netns } }` leaf vs. full-stack isolation | Reject the per-protocol `bgp { listen { netns } }` leaf. Network namespaces are a full-stack wrapper (`netns <name> { bgp { ... } interface { ... } }`, nestable) designed post-release alongside Linux VRF support (`plan/post-release/features/spec-vrf-0-umbrella.md`), not a BGP-only listener leaf. Homed in `plan/post-release/features/spec-bgp-listen-netns.md`. |
| Release-1 `vpp { lcp { netns } }` default | Superseding the 2026-07-16 ruling: for v1.0, change `vpp { lcp { netns } }` default in `internal/component/vpp/yang/ze-vpp-conf.yang` from `"dataplane"` to `""` (host namespace) so VPP LCP TAP interfaces and routing protocols (BGP, OSPF, etc.) work together out of the box without requiring per-protocol netns plumbing. Move multi-namespace `netns <name> { ... }` design to post-release. |

## Task

VPP's Linux Control Plane (LCP) creates Linux TAP mirrors of VPP interfaces so routing protocols (BGP, OSPF, etc.) can bind ordinary Linux sockets on VPP-managed NICs. Today `vpp.lcp.netns` defaults to `"dataplane"` in three places (`internal/component/vpp/yang/ze-vpp-conf.yang`, `internal/component/vpp/config.go`, and `internal/plugins/iface/vpp/doctor.go`), while Ze runs its routing protocols in the host namespace. As a result, the shipped default VPP + BGP configuration places LCP TAPs in `/var/run/netns/dataplane` (where Ze cannot bind and where LCP pair creation fails at apply if the namespace does not exist) and triggers a `doctor-vpp-lcp-netns` warning out of the box.

Per Thomas's 2026-11-24 ruling, fix the v1.0 default by changing `vpp.lcp.netns` default from `"dataplane"` to `""` (VPP's own / host namespace) across YANG, config parsing, doctor fallback, tests, and documentation, while preserving explicit non-empty `vpp.lcp.netns` configuration, its validation, and its `doctor-vpp-lcp-netns` warnings (for named namespaces and for `"host"`/`"root"` markers). Multi-namespace isolation (`netns <name> { ... }`) is homed in `plan/post-release/features/spec-bgp-listen-netns.md` alongside `plan/post-release/features/spec-vrf-0-umbrella.md`.

## Required Reading

- [ ] `ai/doors/system.md` - platform boundaries, YANG schema authority, compile-out boundaries
  → Constraint: `vpp` compiles out under `ze_vpp`; config defaults live in the owning component's YANG and parser, and doctor checks live in the owning plugin.
- [ ] `ai/doors/forwarding.md` - VPP component, `ifacevpp` backend, exact-or-refuse contract
  → Constraint: keep VPP-specific LCP semantics inside `internal/component/vpp` and `internal/plugins/iface/vpp`; never add VPP knowledge to BGP or core packages.
- [ ] `ai/doors/le-tooling.md` - functional `.ci` test conventions
  → Constraint: `expect=stdout:contains=` matches accumulated stdout across a `.ci` file, so the default-silent doctor case and explicit-warning doctor case belong in separate `.ci` files (as documented in `test/ui/doctor-vpp-lcp-netns.ci`).
- [ ] `third_party/vpp-linux-cp/README.md`, `third_party/vpp-linux-cp/src/lcp.c`, `third_party/vpp-linux-cp/src/lcp_interface.c` - vendored VPP v25.10 linux-cp C source
  → Decision: omitting `default netns` in `startup.conf` leaves `lcp_main.default_namespace` NULL (`lcp_get_default_ns`), and passing empty `Netns: ""` in `LcpItfPairAddDel` causes `lcp_itf_pair_create` to create the TAP in VPP's own (host) namespace without opening `/var/run/netns/<name>`.
- [ ] `docs/research/vpp-deployment-reference.md` - VPP startup.conf reference, LCP section, and `## Design decisions`
  → Constraint: `### Network Namespace` currently describes IPng's `dataplane` template as if Ze ran in `dataplane`; update it to distinguish IPng's reference from Ze v1.0's `""` default.
- [ ] `docs/guide/vpp.md` - operator-facing VPP guide
  → Constraint: update "How the two halves fit together", the `vpp.lcp.netns` leaf table row, source anchors, and the Implementation status `(netns)` row in the same change.

**Key insights:**
- Every runtime piece needed for `vpp.lcp.netns == ""` is **already landed** in HEAD (`validateNetns` accepts `""`, `GenerateStartupConf` omits `default netns`, `lcpPairNetns` passes `""`, `GetLCPPair` matches `""`, and `checkVPPLCPNetns` is silent on `""`).
- Only the three `"dataplane"` default seeds (`ze-vpp-conf.yang`, `config.go`, `doctor.go`), the diagnostic description wording (`register_doctor_codes.go`), two stale source comments citing `plan/spec-bgp-netns.md` (`lcp.go`, `doctor.go`), tests, and docs remain to update.
- Explicit non-empty `vpp.lcp.netns` (including `"dataplane"` and markers `"host"`/`"root"`) keeps working as before: `startup.conf` still emits `default netns <name>`, `validateNetns` still rejects unsafe characters, and `checkVPPLCPNetns` still warns on both markers and named namespaces.

## Current Behavior (MANDATORY)

**Source files read (verified against HEAD on 2026-11-26):**

- [ ] `internal/component/vpp/yang/ze-vpp-conf.yang` (lines 323-337) - `vpp/lcp/netns` has `type string;` and `default "dataplane";`; description states that an absent leaf seeds `"dataplane"` while an empty value omits `default netns` and leaves TAPs in VPP's own namespace.
- [ ] `internal/component/vpp/config.go` (lines 145-182, 264-270, 570-591) - `ParseSettings` seeds `LCP.Netns` to `"dataplane"` when omitted; `parseLCP` overwrites `lcp.Netns` when `"netns"` is present in JSON (including `""`); `validateNetns` accepts `""` and rejects `/`, `\`, whitespace, non-printable characters, and lengths > 255.
- [ ] `internal/component/vpp/startupconf.go` (lines 127-161) - `GenerateStartupConf` writes `default netns <s.LCP.Netns>` only when `s.LCP.Netns != ""` and omits the directive when `s.LCP.Netns == ""`.
- [ ] `internal/plugins/iface/vpp/lcp.go` (lines 46-128) - `SetupLCPPair` / `RemoveLCPPair` pass `lcpPairNetns(settings.Netns)` into `LcpItfPairAddDel`; `lcpPairNetns` maps `""`/`"host"`/`"root"` to `""` and passes any other string through. Line 120 carries a stale comment citing `plan/spec-bgp-netns.md`.
- [ ] `internal/plugins/iface/vpp/lcp_query.go` (lines 43-71, 165-168) - `GetLCPPair` reads `settings.Netns` and compares it with VPP's returned `details.Netns` (`""` matches `""`).
- [ ] `internal/plugins/iface/vpp/doctor.go` (lines 173-298, 346-372) - `checkVPPLCPNetns` defaults `netns` to `"dataplane"` when the `vpp/lcp/netns` leaf is omitted from the config tree; returns `nil` when `netns == ""`, emits `lcpNetnsMarkerDiagnostic` for `"host"`/`"root"`, emits `lcpNetnsConfigDiagnostic` when `bgp` is configured with any other non-empty name, and emits `lcpNetnsHostDiagnostic` on Linux when `/var/run/netns/<name>` is absent or unreadable. Line 359 carries a stale comment citing `plan/spec-bgp-netns.md`.
- [ ] `internal/plugins/iface/vpp/register_doctor_codes.go` (lines 19-25) - registers `doctor-vpp-lcp-netns`; description currently says "netns-aware BGP binding is specced but not implemented" and tells operators to leave `vpp.lcp.netns` empty or run `ze` in that namespace.
- [ ] `internal/component/vpp/config_test.go`, `internal/component/vpp/managed_startup_test.go`, `internal/component/vpp/startupconf_test.go`, `internal/plugins/iface/vpp/register_test.go`, `test/ui/doctor-vpp-lcp-netns.ci` - existing unit and functional tests pin `"dataplane"` as the omitted-leaf default.
- [ ] `docs/guide/vpp.md` and `docs/research/vpp-deployment-reference.md` - document `"dataplane"` as the shipped default and describe BGP netns as specced/not implemented.

**Behavior to preserve:**

- Explicit non-empty `vpp.lcp.netns` values (for example `vpp { lcp { netns dataplane; } }` or `netns mgmt;`): `ParseSettings` retains the configured name, `validateNetns` enforces printable/no-whitespace/no-path-separator/max-255 rules, `GenerateStartupConf` emits `default netns <name>`, `lcpItfPair` passes `<name>` to VPP, and `GetLCPPair` matches `<name>`.
- Explicit empty `vpp.lcp.netns ""`: continues to parse as `""`, pass validation, omit `default netns` in `startup.conf`, and stay silent in `ze doctor`.
- `doctor-vpp-lcp-netns` warnings for explicit non-empty values: `"host"` and `"root"` markers emit the single marker warning without running the host probe; other explicit non-empty names emit the BGP config warning when `bgp` is present and the host probe warning on Linux when `/var/run/netns/<name>` is absent or unreadable.
- `lcpNetnsRequiredAdvice` and `lcpNetnsBannedAdvice` invariants (`internal/plugins/iface/vpp/register_test.go`): every `doctor-vpp-lcp-netns` surface names `vpp.lcp.netns`, names both working remedies (leaving `vpp.lcp.netns` empty and running `ze` in that namespace), and never advises setting `vpp.lcp.netns` to `"host"` or `"root"`.
- `vpp.lcp.enabled false`: continues to skip netns validation, omit `linux-cp`/`linux-nl` sections from `startup.conf`, and emit zero `doctor-vpp-lcp-netns` diagnostics.
- BGP listener code (`internal/component/bgp/reactor/reactor.go`, `internal/core/network/network.go`): untouched in v1.0.

**Behavior to change:**

| # | Change | Producer | Why |
|---|--------|----------|-----|
| 1 | Change `vpp/lcp/netns` YANG default from `"dataplane"` to `""` and update its `description` and `ze:help` | `internal/component/vpp/yang/ze-vpp-conf.yang` | Thomas's 2026-11-24 ruling: v1.0 defaults LCP TAPs to the host namespace so BGP and other routing protocols work out of the box |
| 2 | Change `ParseSettings` omitted-leaf `LCP.Netns` seed from `"dataplane"` to `""` and update `validateNetns` doc comment | `internal/component/vpp/config.go` | Keeps config parsing aligned with the YANG default `""` so omitting `vpp.lcp.netns` omits `default netns` in `startup.conf` |
| 3 | Change `checkVPPLCPNetns` omitted-leaf fallback from `"dataplane"` to `""` | `internal/plugins/iface/vpp/doctor.go` | Keeps `ze doctor` aligned with the YANG default so default `vpp { lcp { enabled true; } }` + `bgp` emits zero `doctor-vpp-lcp-netns` warnings |
| 4 | Update `doctor-vpp-lcp-netns` code description and repoint stale `plan/spec-bgp-netns.md` comments | `internal/plugins/iface/vpp/register_doctor_codes.go`, `internal/plugins/iface/vpp/doctor.go`, `internal/plugins/iface/vpp/lcp.go` | Removes stale "netns-aware BGP binding is specced" claim and clears dangling citations to this spec before closure |
| 5 | Update unit tests, functional tests, and VPP docs to reflect default `""` while keeping explicit non-empty netns coverage | `internal/component/vpp/config_test.go`, `internal/component/vpp/managed_startup_test.go`, `internal/plugins/iface/vpp/register_test.go`, `test/ui/doctor-vpp-lcp-netns.ci`, `test/ui/doctor-vpp-lcp-netns-default.ci`, `docs/guide/vpp.md`, `docs/research/vpp-deployment-reference.md` | Pins the new default and preserves full regression coverage for explicit non-empty netns and markers |

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| VPP LCP netns YANG leaf | `internal/component/vpp/yang/ze-vpp-conf.yang` (`vpp/lcp/netns`) | Declares `leaf netns` with `type string` and `default "dataplane"` | Extends: changes `default` to `""` and updates `description` and `ze:help` per Thomas's 2026-11-24 ruling |
| VPP config parser and validator | `internal/component/vpp/config.go` (`ParseSettings`, `parseLCP`, `validateNetns`) | Seeds `LCP.Netns` to `"dataplane"` when omitted; already accepts `""` and rejects `/`, `\`, whitespace, non-printable chars, and >255 bytes | Extends: changes the `ParseSettings` seed from `"dataplane"` to `""`; preserves `parseLCP` and `validateNetns` unchanged |
| VPP `startup.conf` generator | `internal/component/vpp/startupconf.go` (`GenerateStartupConf`) | Already omits `default netns` in the `linux-cp` section when `s.LCP.Netns == ""` and emits `default netns <name>` when non-empty | Reuses unchanged: the empty-netns emission path is already landed and proven by `TestGenerateStartupConfEmptyNetnsOmitsDirective` |
| VPP LCP pair creation and readback | `internal/plugins/iface/vpp/lcp.go` (`SetupLCPPair`, `lcpPairNetns`), `internal/plugins/iface/vpp/lcp_query.go` (`GetLCPPair`) | Maps `""` to `""` in `LcpItfPairAddDel` (placing TAPs in VPP's own namespace when `default netns` is omitted) and compares `details.Netns == settings.Netns` on readback | Reuses unchanged in behavior; repoints the stale `plan/spec-bgp-netns.md` comment in `lcp.go` |
| VPP LCP netns doctor check and diagnostic code | `internal/plugins/iface/vpp/doctor.go` (`checkVPPLCPNetns`, `lcpNetnsIsRootMarker`), `internal/plugins/iface/vpp/register_doctor_codes.go` (`doctor-vpp-lcp-netns`) | Returns `nil` for `""`, warns on `"host"`/`"root"` markers, warns on explicit non-empty names with BGP or missing `/var/run/netns/<name>`; defaults omitted leaf to `"dataplane"` | Extends: changes the omitted-leaf default in `checkVPPLCPNetns` from `"dataplane"` to `""`, updates `doctor-vpp-lcp-netns` description, and repoints the stale `plan/spec-bgp-netns.md` comment |
| VPP LCP plugin presence doctor check | `internal/plugins/iface/vpp/doctor.go` (`checkVPPLCPPlugin`, `doctor-vpp-lcp-plugin`) | Already landed (closed with `spec-fixit-vpp-lcp-reachability` on 2026-08-24); probes running VPP for `linux_cp_plugin.so` | Does not apply: independent of `vpp.lcp.netns` default |
| BGP listener factory | `internal/component/bgp/reactor/reactor.go` (`newListenerFactory`), `internal/core/network/network.go` (`RealListenerFactory`) | Binds BGP listeners in Ze's current network namespace | Does not apply: per Thomas's 2026-11-24 ruling, BGP gains no per-protocol netns leaf; full-stack `netns <name> { ... }` moves to `plan/post-release/features/spec-bgp-listen-netns.md` |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | always | "Config is YANG-modeled, and `ConfigProvider` is its single authority" | agrees | `vpp/lcp/netns` keeps its single YANG definition in `ze-vpp-conf.yang` and aligns `ParseSettings` and `checkVPPLCPNetns` with the YANG default `""` |
| `ai/doors/forwarding.md` | Where it sits | door covering `internal/component/vpp/*`, `internal/plugins/iface/vpp/*` | "`ze_vpp` removes `component/vpp` and every `vpp` backend" | agrees | Every change stays inside `internal/component/vpp` and `internal/plugins/iface/vpp`; BGP and always-on code gain zero VPP imports or references |
| `docs/research/vpp-deployment-reference.md` | Network Namespace | `// Design:` of `internal/component/vpp/config.go` and `internal/plugins/iface/vpp/lcp.go`; modified by this spec | "All LCP TAPs are created in the `dataplane` netns. This isolates the forwarding plane from the management plane. Ze's BGP reactor, SSH, web UI all run in this netns." | departs | Departs per Thomas's 2026-11-24 owner ruling changing v1.0 `vpp.lcp.netns` default from `"dataplane"` to `""` (host/VPP's own namespace) so LCP TAPs and routing protocols work together out of the box; this spec updates `docs/research/vpp-deployment-reference.md` in the same change to distinguish IPng's `dataplane` template from Ze v1.0's `""` default and post-release full-stack `netns <name> { ... }` |
| `ai/rules/repo-maintenance.md` | Directives | `// Design:` of `internal/plugins/iface/vpp/doctor.go` | "A change that adds or changes a surface future agents use, verify, document or avoid MUST update its discovery path in the same work" | agrees | Updates `docs/guide/vpp.md`, `docs/research/vpp-deployment-reference.md`, and the registered `doctor-vpp-lcp-netns` description in the same change so discovery surfaces stay current |
| `docs/architecture/doctor-and-health-checks.md` | Every finding names the command that fixes it | `// Design:` of `internal/plugins/iface/vpp/register_doctor_codes.go` | "A doctor diagnostic carries `fix`: the command or config statement the operator runs to clear the problem, alone and with no label" | agrees | Preserves `Fix` (`ze show diagnostic doctor-vpp-lcp-netns` or `sudo ip netns add <name>`) on every `doctor-vpp-lcp-netns` finding and keeps the code registered in `internal/plugins/iface/vpp` |
| `docs/guide/vpp.md` | How the two halves fit together | `// Design:` of `internal/component/vpp/managed_startup_test.go`; modified by this spec | "LCP creates them in the namespace named by `vpp.lcp.netns`, which defaults to `dataplane`, following the IPng production convention that isolates the forwarding plane from the management plane." | departs | Departs per Thomas's 2026-11-24 owner ruling changing v1.0 `vpp.lcp.netns` default from `"dataplane"` to `""` (host/VPP's own namespace) so BGP and other routing protocols bind on LCP TAPs out of the box; this spec updates `docs/guide/vpp.md` in the same change |
| `ai/doors/le-tooling.md` | Where it sits | door covering `test/ui/doctor-vpp-lcp-netns.ci` and `test/ui/doctor-vpp-lcp-netns-default.ci` | "Functional `.ci` files live under `test/<suite>/` and run through `./le test <suite>` on the functional runner" | agrees | Keeps `test/ui/doctor-vpp-lcp-netns.ci` and adds `test/ui/doctor-vpp-lcp-netns-default.ci` under `test/ui/`, each running one `ze doctor --json` command to avoid accumulated-stdout masking |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "Ze refuses to run VPP unmanaged. When VPP is external, the operator accepts that Ze may terminate and restart it" | agrees | Changes only `vpp.lcp.netns` default; touches no VPP process supervision or external-mode policy |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "Ze knows how to manage and restart an external VPP: through a systemd unit, or by starting the binary directly, as it does with the embedded runtime" | agrees | Changes only `vpp.lcp.netns` default; touches no VPP start or restart mechanism |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "After a failure, when the VPP process is alive, forwarding continued, and Ze reconnects and can read the state it configured, Ze continues from where it is" | agrees | Changes only `vpp.lcp.netns` default; `GetLCPPair` continues to read back and match `settings.Netns` (including `""`) during state readback |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "When VPP could have crashed or stopped forwarding, Ze reflects it in its own model and makes sure IGP and BGP peers are updated" | agrees | Changes only `vpp.lcp.netns` default; touches no VPP failure detection or route update path |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "One timer: the alive test and the loss declaration use the same time, and a probe missed inside it is not a loss. This narrows the row "After a failure, when the VPP process is alive": Ze continues from where it is only through form (1) below, inside the timer, and a declared loss is never adopted" | agrees | Changes only `vpp.lcp.netns` default; touches no VPP liveness or loss-declaration timer |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "As soon as a loss is declared, Ze makes sure the process is terminal (kill through the retained pidfd, then wait for the exit proof), then restarts and reconfigures VPP. Ze is the only restarter: a systemd unit it drives does not restart VPP itself (Restart=no)" | agrees | Changes only `vpp.lcp.netns` default; touches no VPP process termination or restart path |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | ""Recovered" has exactly two forms. (1) Inside the single timer Ze reconnects to the same pidfd-proven process and a full readback of what it installed (interfaces, LCP pairs, owned routes, PPPoE sessions) matches: Ze continues, with no kill and no withdrawal. Any mismatch means form (1) failed. (2) A forced restart (kill through the retained pidfd, exit proof) followed by a correct reconfiguration, exactly as at boot. Routes are withdrawn only when neither form succeeds within the recovery deadline and the crash-loop limit" | agrees | Changes only `vpp.lcp.netns` default; LCP pair readback (`GetLCPPair`) matches `""` identically to any named namespace, and recovery logic is untouched |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "Blackholing during the restart is accepted. Routes are withdrawn (VPP-backed interfaces down in Ze's model, so IGP and BGP react) only when neither recovery form succeeds within the recovery deadline and the crash-loop limit. This narrows the row "When VPP could have crashed or stopped forwarding": Ze updates IGP and BGP peers only then" | agrees | Changes only `vpp.lcp.netns` default; touches no route withdrawal or restart-blackholing behavior |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "systemd configuration: Ze cannot be sure that an OS update will not change the unit's settings, so it detects the settings and adapts. No unit policy is required: Ze reads the unit's effective settings each time it acts on VPP, and adapts its restart to them. This supersedes the second sentence of the row "As soon as a loss is declared" (Ze the only restarter, Restart=no) and its forbids clause "or leaving the restart to systemd or another supervisor"; correcting that row needs Thomas's decision-change" | agrees | Changes only `vpp.lcp.netns` default; touches no systemd unit detection or restart adaptation |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "Two limits. The alive test and the loss declaration use the same time: a probe missed inside it is not a loss, and a failed alive test is the loss declaration. A second, separate limit then bounds the reconnect-and-verify attempt of form (1). When that limit expires without form (1), Ze makes the process terminal (kill through the retained pidfd, then wait for the exit proof), then restarts and reconfigures VPP. This supersedes the row "One timer" (its single timer, "a declared loss is never adopted" and its forbids clause "a second window after the declaration"), and narrows the row "As soon as a loss is declared": the kill follows the second limit, not the declaration" | agrees | Changes only `vpp.lcp.netns` default; touches no VPP liveness or reconnect-and-verify timers |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "Form (1) adopts on the presence of data ("We should take the presence of data as a sign that the software is still running and reconciliation may be then happening in the background."). Inside the reconnect-and-verify limit, the same pidfd-proven process answering over the API with the data Ze configured is the sign it is alive: Ze adopts it, with no kill and no withdrawal, and reconciles in the background, as the fib-vpp replay and sweep and the iface reconcile do. A mismatch the background reconciliation finds is repaired, not treated as a loss. Form (2) stays a forced restart followed by a correct reconfiguration, exactly as at boot. This narrows the row "Recovered has exactly two forms": a full readback match is no longer required, and a mismatch no longer fails form (1)" | agrees | Changes only `vpp.lcp.netns` default; touches no VPP adoption or background reconciliation path |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "A recovery deadline and a crash-loop limit (N restarts inside a window) bound recovery. Past either, Ze withdraws: VPP-backed interfaces go down in Ze's model, so IGP and BGP react, and ze doctor reports it. Blackholing while Ze restarts inside both limits is accepted. The default values are proposed at design review for Thomas's approval. This adopts the recovery deadline and the crash-loop limit that the rows "Recovered has exactly two forms" and "Blackholing during the restart is accepted" carried as proposals, and narrows the row "When VPP could have crashed or stopped forwarding": Ze updates IGP and BGP peers only past a limit" | agrees | Changes only `vpp.lcp.netns` default; touches no recovery deadline or crash-loop limit |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "After a withdrawal Ze keeps retrying recovery with a growing backoff. When VPP recovers, the VPP-backed interfaces come up in Ze's model and the routes return by themselves, and ze doctor reports the state throughout. This narrows the crash-loop limit: past it Ze withdraws and keeps restarting" | agrees | Changes only `vpp.lcp.netns` default; touches no post-withdrawal retry backoff |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "The VPP loss itself triggers no withdrawal: no VPP-backed interface goes down in Ze's model while Ze recovers. Ze's own protocol timers (BFD detection, OSPF and IS-IS dead intervals, BGP hold) keep running during recovery, as their RFCs say" | agrees | Changes only `vpp.lcp.netns` default; touches no protocol timers or VPP loss handling |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "A process on VPP's API socket whose pidfd does not resolve to the configured systemd unit is reported, never refused or killed ("We should report but not refuse if the customers have changed the software, for example. It may mean that we have an out of date configuration. We should make sure the customer updates it but not cause a routing outage."). ze doctor names the process, the configured unit, that the configuration may be out of date, and the exact fix, and Ze keeps working with that process" | agrees | Changes only `vpp.lcp.netns` default; touches no pidfd/systemd unit check or reporting |

## Data Flow (MANDATORY)

### Entry Point
- Operator configuration tree containing `vpp { enabled true; ... }` (with `vpp/lcp/netns` omitted, explicitly set to `""`, or explicitly set to a named namespace) and optionally `bgp { ... }`.
- Reaches both the VPP component (`ParseSettings` -> `Validate` -> `GenerateStartupConf` -> `GetActiveLCPSettings` -> `ifacevpp.SetupLCPPair` / `GetLCPPair`) and `ze doctor` (`checkVPPLCPNetns`).

### Transformation Path
1. **Schema & validation:** `ze-vpp-conf.yang` declares `vpp/lcp/netns` with `default ""`; `ze config validate` and commit load the tree with no forced `"dataplane"` default.
2. **VPP config parse:** `ParseSettings` (`internal/component/vpp/config.go`) initializes `LCP.Netns` to `""`. If the operator wrote `netns "<name>"`, `parseLCP` sets `LCP.Netns = "<name>"`. `Validate` runs `validateNetns` when `LCP.Enabled` is true (accepting `""` and valid names, refusing unsafe characters).
3. **VPP `startup.conf` generation:** `GenerateStartupConf` (`internal/component/vpp/startupconf.go`) omits `default netns` when `LCP.Netns == ""` and writes `default netns <name>` when non-empty. VPP's `lcp_get_default_ns` (`third_party/vpp-linux-cp/src/lcp.c`) returns `NULL` when omitted.
4. **LCP pair creation & readback:** `SetupLCPPair` (`internal/plugins/iface/vpp/lcp.go`) sends `LcpItfPairAddDel` with `Netns: ""` when `settings.Netns == ""`. VPP's `lcp_itf_pair_create` (`third_party/vpp-linux-cp/src/lcp_interface.c`) creates the TAP in VPP's own (host) namespace, where Ze's BGP/OSPF/routing sockets bind directly. `GetLCPPair` (`lcp_query.go`) reads `details.Netns == ""` and matches `settings.Netns == ""`.
5. **Doctor check:** `checkVPPLCPNetns` (`internal/plugins/iface/vpp/doctor.go`) reads `vpp/lcp/netns` with omitted fallback `""`. When `""`, it returns `nil` (clean). When non-empty (`"host"`/`"root"` marker or explicit name such as `"dataplane"`), it emits the existing `doctor-vpp-lcp-netns` diagnostics.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Operator config -> `internal/component/vpp` | YANG `vpp/lcp/netns` leaf -> JSON config section -> `ParseSettings` (`config.go`) | Yes (read 2026-11-26) |
| `internal/component/vpp` -> VPP process | Generated `startup.conf` `linux-cp` section (`startupconf.go`) -> VPP `lcp_itf_pair_config` (`third_party/vpp-linux-cp/src/lcp_interface.c`) | Yes (read 2026-11-26) |
| `internal/component/vpp` -> `internal/plugins/iface/vpp` | `vppcomp.GetActiveLCPSettings()` -> `SetupLCPPair` (`lcp.go`) and `GetLCPPair` (`lcp_query.go`) | Yes (read 2026-11-26) |
| `internal/plugins/iface/vpp` -> VPP binary API | GoVPP `LcpItfPairAddDel` / `LcpItfPairGetV2` (`lcp.go`, `lcp_query.go`) | Yes (read 2026-11-26) |
| Config tree -> `ze doctor` | `*config.Tree` -> `checkVPPLCPNetns` (`doctor.go`) | Yes (read 2026-11-26) |

### Changed-Wire Census

No wire format, protocol message, RPC envelope, or config file grammar changes in this spec.

**Search evidence:** Searched `internal/`, `test/`, `docs/`, and `third_party/vpp-linux-cp/` via `git grep` for `"dataplane"`, `LCP.Netns`, `GetActiveLCPSettings`, `lcpPairNetns`, `default netns`, `validateNetns`, and `doctor-vpp-lcp-netns` (positive controls: `internal/component/vpp/config.go:268`, `internal/component/vpp/startupconf.go:153-155`, `internal/plugins/iface/vpp/doctor.go:183`, `internal/plugins/iface/vpp/lcp.go:65`, `internal/plugins/iface/vpp/lcp_query.go:70`, `third_party/vpp-linux-cp/src/lcp_interface.c:584`). Every wire carrier already supports empty `netns == ""` with unchanged framing: `GenerateStartupConf` already omits `default netns` when `s.LCP.Netns == ""` (`startupconf.go:153-155`), VPP's `lcp_itf_pair_config` / `lcp_get_default_ns` already treat absent `default netns` as `NULL` (`third_party/vpp-linux-cp/src/lcp.c`), `LcpItfPairAddDel` and `LcpItfPairGetV2` carry the unchanged `Netns` string field as `""` (`lcp.go:96`, `lcp_query.go:165-168`), and YANG config JSON keeps the exact same `vpp.lcp.netns` leaf name and string type (`config.go:587-589`). Only the omitted-leaf default value changes from `"dataplane"` to `""`.

### Integration Points
- `internal/component/vpp/yang/ze-vpp-conf.yang` - YANG schema leaf default read by `config.yangToLeaf`, CLI help, config validation, and docs.
- `internal/component/vpp/config.go` (`ParseSettings`) - Go default seed matching YANG when `lcp.netns` is omitted from JSON.
- `internal/plugins/iface/vpp/doctor.go` (`checkVPPLCPNetns`) - Doctor check default fallback matching YANG when `vpp/lcp/netns` is absent from `*config.Tree`.
- `internal/plugins/iface/vpp/register_doctor_codes.go` (`doctor-vpp-lcp-netns`) - Registered diagnostic explanation displayed by `ze show diagnostic doctor-vpp-lcp-netns`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Config flows through YANG -> `ParseSettings` -> `startupconf.go` / `GetActiveLCPSettings` -> `ifacevpp`; doctor reads `*config.Tree` via `diagnostic.RegisterDoctorCheck` |
| No unintended coupling (components stay isolated) | Yes | BGP and `internal/core/network` are completely untouched; VPP LCP default stays wholly inside `component/vpp` and `plugins/iface/vpp` |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses the already-landed empty-netns handling in `validateNetns`, `GenerateStartupConf`, `lcpPairNetns`, `GetLCPPair`, and `checkVPPLCPNetns` |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Control-plane startup/config/doctor path only; no packet or BGP UPDATE hot path touched |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them (`ai/rules/plugins.md`) | Yes | No new commands, handlers, or core switch cases added; existing `vpp-lcp-netns` doctor check stays registered from `internal/plugins/iface/vpp` |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name (`ai/rules/principles.md`) | Yes | Searched `"dataplane"`, `LCP.Netns`, `ze-vpp-conf` across `internal/`, `test/`, and `docs/`; no new leaf name or feature identifier is introduced |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Omitting `default netns` in `startup.conf` and passing `Netns: ""` in `LcpItfPairAddDel` places LCP TAPs in VPP's own (host) namespace | Verified in vendored VPP C source: `third_party/vpp-linux-cp/src/lcp.c` (`lcp_get_default_ns` returns `NULL` when zero-initialized) and `src/lcp_interface.c` (`lcp_itf_pair_create` falls back to `lcp_get_default_ns()`, passing `NULL` to `tap_create_if`) | TAPs would fail creation or land in another namespace | Verified by reading `third_party/vpp-linux-cp/src/lcp.c` and `src/lcp_interface.c`, plus `TestGenerateStartupConfEmptyNetnsOmitsDirective` | confirmed |
| A-2 | `default "";` on a `type string` leaf is valid in Ze's YANG schema builder (`yangToLeaf`, `validateLeafDefaults`) | Read `internal/component/config/yang_schema.go` lines 789-801 and 860-874: `checkModuleDefault` validates `""` against `type string` with no `length`/`pattern` prohibiting empty | Schema build would record a build error | Scoped `internal/component/config/...` and `internal/component/vpp/...` unit tests | confirmed by read; verify via unit tests |
| A-3 | No test or runtime caller outside `internal/component/vpp`, `internal/plugins/iface/vpp`, and `test/ui/doctor-vpp-lcp-netns.ci` relies on `vpp.lcp.netns` defaulting to `"dataplane"` when omitted | `git grep` across `internal/`, `test/`, `docs/` for `"dataplane"`, `LCP.Netns`, and `ze-vpp-conf`: all `l2tp/pppoe` and `ifacevpp/lcp_query*` test fixtures pass explicit `"dataplane"` values rather than reading the omitted default; `internal/le/site/testdata/published-*` are frozen `gh-pages` fixtures | An unexpected test would fail during implementation | Scoped unit and functional test runs by main thread | confirmed by grep; verify via scoped tests |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An existing pre-release deployment or test config that omitted `vpp.lcp.netns` expecting `"dataplane"` now places TAPs in the host namespace | `TestManagedStartupDefaultsAndExplicitHugepagesPreserveOutput`, `TestParseSettings`, `TestDoctorLCPNetnsDefaultIsolated`, or `test/ui/doctor-vpp-lcp-netns.ci` fail until updated | Ze is pre-release and Thomas explicitly ruled on 2026-11-24 to change the v1.0 default to `""`. Operators who want `"dataplane"` isolation set `vpp { lcp { netns dataplane; } }` explicitly; update all affected tests and docs in the same commit |
| R-2 | Rewording `doctor-vpp-lcp-netns` breaks `lcpNetnsRequiredAdvice` or `lcpNetnsBannedAdvice` assertions in `internal/plugins/iface/vpp/register_test.go` | `TestDoctorLCPNetnsCodeDescription` or `TestDoctorLCPNetnsRemediation` fails | Keep the required remedy wording (mentioning `vpp.lcp.netns`, leaving `vpp.lcp.netns` empty, and running `ze` in that namespace) and never recommend setting `vpp.lcp.netns` to `"host"`/`"root"` |
| R-3 | Updating `test/ui/doctor-vpp-lcp-netns.ci` drops coverage of either the default-silent path or the explicit-warning path | Only one path tested, or accumulated stdout hides a regression | Keep `test/ui/doctor-vpp-lcp-netns.ci` testing explicit `netns dataplane;` (single command) and add separate `test/ui/doctor-vpp-lcp-netns-default.ci` testing omitted `netns` (single command) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Either VPP `startup.conf` emits an invalid `default netns` line or `ze doctor` falsely warns on default VPP + BGP configs or goes silent on explicit non-empty `vpp.lcp.netns` configs |
| How is it reverted? | Single commit revert; no persistent on-disk state or wire format changes |
| Who else touches this path? | `internal/component/vpp` and `internal/plugins/iface/vpp` are shared with VPP lifecycle and PPPoE-over-VPP specs (`plan/post-release/features/spec-vpp-failure-lifecycle.md`, `plan/post-release/features/spec-pppoe-vpp-dataplane.md`); neither requires `"dataplane"` as the omitted default (both read explicit `settings.Netns`) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze doctor --json config <file>` with `bgp` + `vpp { lcp { enabled true; } }` (omitting `netns`) | → | `ze-vpp-conf.yang` default `""` -> `checkVPPLCPNetns` (`internal/plugins/iface/vpp/doctor.go`) | `test/ui/doctor-vpp-lcp-netns-default.ci` |
| `ze doctor --json config <file>` with `bgp` + `vpp { lcp { enabled true; netns dataplane; } }` | → | `checkVPPLCPNetns` (`internal/plugins/iface/vpp/doctor.go`) -> `lcpNetnsConfigDiagnostic` | `test/ui/doctor-vpp-lcp-netns.ci` |
| Delivered `vpp { enabled true; }` config tree (omitting `lcp.netns`) | → | `ParseConfigSections` -> `ParseSettings` (`internal/component/vpp/config.go`) -> `GenerateStartupConf` (`startupconf.go`) | `TestManagedStartupDefaultsAndExplicitHugepagesPreserveOutput` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `vpp/lcp/netns` leaf in `internal/component/vpp/yang/ze-vpp-conf.yang` | The YANG schema defines `vpp/lcp/netns` with `default ""`, passes schema validation, and documents that omitting or emptying the leaf leaves LCP TAPs in VPP's own (host) namespace | Explicit non-empty string values remain permitted so operators running Ze inside a dedicated namespace can still configure `vpp.lcp.netns` explicitly |
| AC-2 | `ParseSettings` (`internal/component/vpp/config.go`) given `{"enabled":"true"}` or `{"enabled":"true","lcp":{"enabled":"true"}}` (with `netns` omitted) | `VPPSettings.LCP.Netns` parses as `""`, `Validate()` returns `nil`, and `GenerateStartupConf` emits a `linux-cp` block containing `lcp-sync` and `lcp-auto-subint` while omitting `default netns` | Explicit `lcp.netns` values (for example `"dataplane"` or `"mgmt"`) remain parsed verbatim and still emit `default netns <name>` in `startup.conf` because operators who run Ze inside a named namespace still need that directive |
| AC-3 | `checkVPPLCPNetns` (`internal/plugins/iface/vpp/doctor.go`) and `ze doctor --json` given `bgp { ... }` and `vpp { lcp { enabled true; } }` (with `netns` omitted), even with a live `/var/run/netns` probe directory | `checkVPPLCPNetns` returns an empty diagnostic slice (`len == 0`) and `ze doctor --json` completes with valid JSON output that reports zero `doctor-vpp-lcp-netns` findings | Unrelated doctor checks (such as `doctor-vpp-lcp-plugin` or `doctor-runtime-not-checked`) remain emitted because only the namespace mismatch is resolved by defaulting `vpp.lcp.netns` to `""` |
| AC-4 | `checkVPPLCPNetns` and `ze doctor --json` given `bgp { ... }` and explicit `vpp { lcp { enabled true; netns dataplane; } }`, or explicit marker `netns host` / `netns root` | Explicit `netns dataplane` emits `doctor-vpp-lcp-netns` quoting `"dataplane"` with `"is not root-reachable"` (and the absent-namespace host diagnostic on Linux when absent); explicit `"host"` or `"root"` emits the single marker `doctor-vpp-lcp-netns` warning naming `/var/run/netns/<marker>`; `ze show diagnostic doctor-vpp-lcp-netns` satisfies all `lcpNetnsRequiredAdvice` and `lcpNetnsBannedAdvice` checks | Existing non-empty namespace warnings survive intentionally because Ze v1.0 still runs routing protocols in a single namespace and cannot cross into a separately configured `vpp.lcp.netns` |
| AC-5 | Repository citations and operator docs (`docs/guide/vpp.md`, `docs/research/vpp-deployment-reference.md`, `internal/plugins/iface/vpp/lcp.go`, `internal/plugins/iface/vpp/doctor.go`) | Both docs state `vpp.lcp.netns` defaults to `""` (host/VPP's own namespace) for v1.0 with multi-namespace `netns <name> { ... }` planned post-release, and source comments in `lcp.go` and `doctor.go` cite live sources rather than `plan/spec-bgp-netns.md` | Historical IPng AS8298 production reference values (`dataplane` with Bird2) survive in `docs/research/vpp-deployment-reference.md`'s IPng reference tables because those tables document IPng Networks' deployment, not Ze's default |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|---------------------|-----------------------|
| 1 | Enables VPP (`vpp { enabled true; lcp { enabled true; } }`) alongside BGP without specifying `vpp.lcp.netns` and runs `ze doctor` | `ze-vpp-conf.yang` default `""` -> `ParseSettings` -> `GenerateStartupConf` (omits `default netns`) / `checkVPPLCPNetns` (returns 0 diagnostics) | `test/ui/doctor-vpp-lcp-netns-default.ci`, `TestManagedStartupDefaultsAndExplicitHugepagesPreserveOutput` |
| 2 | Explicitly configures `vpp { lcp { enabled true; netns dataplane; } }` alongside BGP and runs `ze doctor` | config tree -> `checkVPPLCPNetns` -> `doctor-vpp-lcp-netns` warning naming `"dataplane"` and valid remedies | `test/ui/doctor-vpp-lcp-netns.ci`, `TestDoctorLCPNetnsIsolated` |
| 3 | Runs `ze show diagnostic doctor-vpp-lcp-netns` after seeing the warning on an explicit non-empty `vpp.lcp.netns` | `diagnostic.Lookup("doctor-vpp-lcp-netns")` (`register_doctor_codes.go`) -> clear explanation naming default empty leaf and running `ze` in that namespace | `TestDoctorLCPNetnsCodeDescription` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseSettings` ("minimal enabled") | `internal/component/vpp/config_test.go` | Omitted `lcp.netns` defaults to `""` (AC-2) | update existing |
| `TestEmptyNetnsSurvivesParseAndValidate` | `internal/component/vpp/config_test.go` | Both omitted `lcp.netns` and explicit `lcp.netns: ""` yield `""` and pass `Validate()`; explicit `"dataplane"` / `"mgmt"` stay preserved (AC-2) | update existing |
| `TestManagedStartupDefaultsAndExplicitHugepagesPreserveOutput` | `internal/component/vpp/managed_startup_test.go` | Default `vpp { enabled true; }` omits `default netns` from generated `startup.conf`; explicit `netns: "dataplane"` still emits `default netns dataplane` (AC-2) | update existing |
| `TestDoctorLCPNetnsDefaultSilent` | `internal/plugins/iface/vpp/register_test.go` | Replaces `TestDoctorLCPNetnsDefaultIsolated`: omitted `vpp/lcp/netns` yields 0 diagnostics even with a live `lcpNetnsDir` probe (AC-3) | replace existing |
| `TestDoctorLCPNetnsIsolated`, `TestDoctorLCPNetnsRootMarkerWarns`, `TestDoctorLCPNetnsAbsentFromHost`, `TestDoctorLCPNetnsProbeError`, `TestDoctorLCPNetnsCodeDescription` | `internal/plugins/iface/vpp/register_test.go` | Explicit `"dataplane"`, markers `"host"`/`"root"`, absent/unreadable `/var/run/netns/<name>`, and registered code description continue to pass (AC-4) | preserve existing |

### Boundary Tests
N/A: no numeric range changes; string validation boundaries in `validateNetns` (`""`, printable ASCII, 255-char max, 256-char refusal, whitespace/path-separator refusal) are already covered by `TestValidateNetnsRefusesWhatVPPCannotCarry` in `internal/component/vpp/config_test.go`.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `doctor-vpp-lcp-netns-default` | `test/ui/doctor-vpp-lcp-netns-default.ci` | Default `vpp { lcp { enabled true; } }` (omitting `netns`) + `bgp` produces zero `doctor-vpp-lcp-netns` warnings in `ze doctor --json` (AC-3) | new |
| `doctor-vpp-lcp-netns` | `test/ui/doctor-vpp-lcp-netns.ci` | Explicit `vpp { lcp { enabled true; netns dataplane; } }` + `bgp` emits `doctor-vpp-lcp-netns` naming `dataplane` and `is not root-reachable` (AC-4) | update existing |

### Interop Tests
N/A (Scope: config). No BGP wire format, capability, FSM transition, or GoVPP binary API message layout changes.

## Files to Modify
- `internal/component/vpp/yang/ze-vpp-conf.yang` - change `vpp/lcp/netns` `default "dataplane";` to `default "";` and update its `description` and `ze:help`.
- `internal/component/vpp/config.go` - change `ParseSettings` `LCP.Netns` seed from `"dataplane"` to `""` and update `validateNetns` doc comment.
- `internal/plugins/iface/vpp/doctor.go` - change `checkVPPLCPNetns` omitted-leaf fallback from `"dataplane"` to `""` and repoint the stale `plan/spec-bgp-netns.md` comment in `lcpNetnsIsRootMarker`.
- `internal/plugins/iface/vpp/lcp.go` - repoint the stale `plan/spec-bgp-netns.md` comment in `lcpPairNetns` (no behavioral change).
- `internal/plugins/iface/vpp/register_doctor_codes.go` - update `doctor-vpp-lcp-netns` description to match default `""` and Thomas's 2026-11-24 ruling while keeping required/banned advice invariants.
- `internal/component/vpp/config_test.go` - update omitted-leaf default assertions from `"dataplane"` to `""`.
- `internal/component/vpp/managed_startup_test.go` - update default `startup.conf` golden string to omit `default netns dataplane`, and assert explicit `netns: "dataplane"` still emits it.
- `internal/plugins/iface/vpp/register_test.go` - replace `TestDoctorLCPNetnsDefaultIsolated` with `TestDoctorLCPNetnsDefaultSilent` (omitted leaf produces 0 diagnostics with live probe).
- `test/ui/doctor-vpp-lcp-netns.ci` - add explicit `netns dataplane;` so the test continues to verify the warning for explicit non-empty `vpp.lcp.netns`.
- `docs/guide/vpp.md` - update "How the two halves fit together", leaf table default, source anchor, and Implementation status `(netns)` row.
- `docs/research/vpp-deployment-reference.md` - update `### Network Namespace` to distinguish IPng's `dataplane` reference from Ze v1.0's `""` default and post-release full-stack `netns <name> { ... }`.

## Files to Create
- `test/ui/doctor-vpp-lcp-netns-default.ci` - single-command functional test proving `ze doctor --json` emits zero `doctor-vpp-lcp-netns` findings for default `vpp { lcp { enabled true; } }` + `bgp`.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/vpp/yang/ze-vpp-conf.yang` (default value and description updated; verify `./le yang glue check`) |
| YANG validation constraints | No | Existing `validateNetns` enforces VPP character/length constraints; empty string `""` is intentionally valid |
| YANG custom validators | No | Existing `VPPSettings.Validate` (`validateNetns`) unchanged |
| CLI commands/flags | No | No CLI command added or changed |
| CLI grammar (keyword before value) | No | No CLI grammar change |
| Editor autocomplete | No | Leaf path and type unchanged |
| Functional test for new RPC/API | Yes | `test/ui/doctor-vpp-lcp-netns-default.ci` (new) and `test/ui/doctor-vpp-lcp-netns.ci` (updated) |
| Pipe completeness | No | No command output handler added or changed |
| Env var registration | No | No env var added or changed |
| Doctor check for runtime dependencies | Yes | Existing `vpp-lcp-netns` check (`internal/plugins/iface/vpp/doctor.go`) updated so omitted leaf matches YANG default `""` |
| Doctor code per problem class | Yes | Existing `doctor-vpp-lcp-netns` description (`internal/plugins/iface/vpp/register_doctor_codes.go`) updated to match default `""` |
| Prometheus counters/metrics | No | No metric added or changed |
| BGP family surface (new SAFI / capability / attribute) | N-A | Config-only VPP LCP default change; no BGP protocol surface touched |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Default value fix only; no new feature entry |
| 2 | Config syntax changed? | No | Leaf path `vpp.lcp.netns` and syntax unchanged; only default value changes |
| 3 | CLI command added/changed? | No | No CLI change |
| 4 | API/RPC added/changed? | No | No API/RPC change |
| 5 | Plugin added/changed? | No | No plugin added or removed |
| 6 | Has a user guide page? | Yes | `docs/guide/vpp.md` ("How the two halves fit together", leaf table, Implementation status) |
| 7 | Wire format changed? | No | No wire format changed (see Changed-Wire Census) |
| 8 | Plugin SDK/protocol changed? | No | No SDK or RPC change |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | Not a protocol/RFC change |
| 10 | Test infrastructure changed? | No | Uses existing `.ci` runner unchanged |
| 11 | Affects daemon comparison? | No | No daemon comparison row claims `dataplane` default |
| 12 | Internal architecture changed? | Yes | `docs/research/vpp-deployment-reference.md` (`### Network Namespace`) |
| 13 | Route metadata keys added/changed? | No | None touched |
| 14 | Prometheus counters added/changed? | No | None touched |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Existing `doctor-vpp-lcp-netns` code name and registration unchanged |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Verified via `./le spec citation anchors spec plan/release/fixes/spec-bgp-netns.md`. Updated in this spec: `docs/guide/vpp.md` and `docs/research/vpp-deployment-reference.md`. Declared `// Design:` docs unaffected: `ai/rules/repo-maintenance.md` (declared by `internal/plugins/iface/vpp/doctor.go`; defines the repo maintenance rule that discovery paths update with code and does not hardcode VPP LCP defaults) and `docs/architecture/doctor-and-health-checks.md` (declared by `internal/plugins/iface/vpp/register_doctor_codes.go`; specifies doctor check phases, severities, and `fix` formatting without enumerating `doctor-vpp-lcp-netns` or its default value) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/vpp.md` minimal VPP config and leaf table updated to match default `""` |

## Design presentation

### What changes
For v1.0, `vpp.lcp.netns` changes its default from `"dataplane"` to `""` (VPP's own / host network namespace) across the YANG schema, VPP config parser, and `vpp-lcp-netns` doctor check, so VPP LCP TAP mirrors and Ze's routing protocols (BGP, OSPF, etc.) work together out of the box without per-protocol netns configuration. Explicit non-empty `vpp.lcp.netns` values (such as `"dataplane"`) remain supported and still emit `default netns <name>` in `startup.conf` and `doctor-vpp-lcp-netns` warnings; full-stack multi-namespace isolation (`netns <name> { ... }`) moves to `plan/post-release/features/spec-bgp-listen-netns.md` alongside VRF (`plan/post-release/features/spec-vrf-0-umbrella.md`).

### Before and after
1. **Default VPP + BGP config (`vpp { enabled true; lcp { enabled true; } }` with `netns` omitted):**
   - **Before:** `ParseSettings` seeds `LCP.Netns = "dataplane"`, `GenerateStartupConf` emits `default netns dataplane` in `linux-cp`, VPP attempts to create LCP TAPs in `/var/run/netns/dataplane` (where Ze BGP cannot bind), and `ze doctor --json` emits `doctor-vpp-lcp-netns` (`"is not root-reachable"` and, on Linux when absent, `"absent from /var/run/netns/"`).
   - **After:** `ParseSettings` seeds `LCP.Netns = ""`, `GenerateStartupConf` omits `default netns` from `linux-cp`, VPP creates LCP TAPs in VPP's own (host) namespace where Ze BGP binds directly, and `ze doctor --json` emits zero `doctor-vpp-lcp-netns` warnings.
2. **Explicitly isolated LCP namespace (`vpp { lcp { enabled true; netns dataplane; } }` with `bgp`):**
   - **Before:** Emits `default netns dataplane` and `doctor-vpp-lcp-netns`.
   - **After:** Unchanged; still emits `default netns dataplane` and `doctor-vpp-lcp-netns` naming `"dataplane"` and valid remedies.
3. **`ze show diagnostic doctor-vpp-lcp-netns`:**
   - **Before:** States that netns-aware BGP binding is specced but not implemented.
   - **After:** States that `vpp.lcp.netns` defaults to empty (VPP's own namespace) and that any explicit non-empty value places TAPs outside Ze's namespace, keeping the two working remedies (leave `vpp.lcp.netns` empty, or run `ze` in that namespace).

### Alternatives set aside
| Alternative | Why set aside |
|-------------|---------------|
| Keep `vpp.lcp.netns` default `"dataplane"` and add a BGP-only `bgp { listen { netns } }` leaf | Rejected by Thomas on 2026-11-24: network namespaces are a full-stack nestable wrapper (`netns <name> { bgp { ... } interface { ... } }`) designed post-release alongside VRF, not a BGP-only listener leaf |
| Remove `vpp.lcp.netns` entirely for v1.0 | Unnecessary breakage: operators running Ze inside a dedicated namespace (matching IPng's model) can still set `vpp.lcp.netns` explicitly while default installs work out of the box |

### Documents departed from
1. `docs/guide/vpp.md`, section `How the two halves fit together`: currently states "LCP creates them in the namespace named by `vpp.lcp.netns`, which defaults to `dataplane`, following the IPng production convention that isolates the forwarding plane from the management plane." After this work (per Thomas's 2026-11-24 owner ruling), the page states that `vpp.lcp.netns` defaults to `""` (empty, VPP's own / host namespace) for v1.0 so BGP and other routing protocols bind on LCP TAPs out of the box, while explicit non-empty `vpp.lcp.netns` values still isolate TAPs under `/var/run/netns/<name>` and emit `doctor-vpp-lcp-netns`, and multi-namespace `netns <name> { ... }` isolation moves post-release (`plan/post-release/features/spec-bgp-listen-netns.md`).
2. `docs/research/vpp-deployment-reference.md`, section `Network Namespace`: currently states "All LCP TAPs are created in the `dataplane` netns. This isolates the forwarding plane from the management plane. Ze's BGP reactor, SSH, web UI all run in this netns." After this work (per Thomas's 2026-11-24 owner ruling), the section distinguishes IPng Networks' `dataplane` reference deployment (Bird2 running inside `dataplane`) from Ze v1.0's default (`vpp.lcp.netns` = `""`, placing TAPs in VPP's own / host namespace alongside Ze's routing protocols), with full-stack `netns <name> { ... }` isolation planned post-release.

### Choices for Thomas
none

## Implementation Steps

1. **Phase 1: Wiring and failing tests** -- update `TestParseSettings`, `TestEmptyNetnsSurvivesParseAndValidate`, `TestManagedStartupDefaultsAndExplicitHugepagesPreserveOutput`, and `TestDoctorLCPNetnsDefaultSilent` plus add `test/ui/doctor-vpp-lcp-netns-default.ci` (failing against `"dataplane"` default).
   - Tests: `TestParseSettings`, `TestEmptyNetnsSurvivesParseAndValidate`, `TestManagedStartupDefaultsAndExplicitHugepagesPreserveOutput`, `TestDoctorLCPNetnsDefaultSilent`, `test/ui/doctor-vpp-lcp-netns-default.ci`
   - Files: `internal/component/vpp/config_test.go`, `internal/component/vpp/managed_startup_test.go`, `internal/plugins/iface/vpp/register_test.go`, `test/ui/doctor-vpp-lcp-netns.ci`, `test/ui/doctor-vpp-lcp-netns-default.ci`
   - Verify: new/updated default assertions fail red before production code edits.
2. **Phase 2: Default change across YANG, config, and doctor** -- update the three `"dataplane"` default seeds to `""`, update `doctor-vpp-lcp-netns` description, and repoint stale `plan/spec-bgp-netns.md` comments.
   - Tests: all unit and functional tests from Phase 1 plus existing `doctor-vpp-lcp-netns` marker, absent-namespace, probe-error, and advice tests
   - Files: `internal/component/vpp/yang/ze-vpp-conf.yang`, `internal/component/vpp/config.go`, `internal/plugins/iface/vpp/doctor.go`, `internal/plugins/iface/vpp/lcp.go`, `internal/plugins/iface/vpp/register_doctor_codes.go`
   - Verify: main thread runs `./le test unit pkg path ./internal/component/vpp/`, `./le test unit pkg path ./internal/plugins/iface/vpp/`, `./le test unit pkg path ./internal/component/config/`, `./le yang glue check`, `./le test functional ui`, and `./le go lint run`.
3. **Phase 3: Documentation updates** -- update `docs/guide/vpp.md` and `docs/research/vpp-deployment-reference.md` and verify doc anchors.
   - Files: `docs/guide/vpp.md`, `docs/research/vpp-deployment-reference.md`
   - Verify: main thread runs `./le doc check all`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All three default seeds (`ze-vpp-conf.yang`, `config.go`, `doctor.go`) agree on `""` |
| Feature completeness | Default omitted `vpp.lcp.netns` is clean end-to-end in `startup.conf` and `ze doctor`; explicit non-empty `vpp.lcp.netns` still emits `default netns <name>` and `doctor-vpp-lcp-netns` |
| Correctness | `doctor-vpp-lcp-netns` description and messages still satisfy `lcpNetnsRequiredAdvice` and `lcpNetnsBannedAdvice` |
| Data flow | BGP and `internal/core/network` have zero edits; no VPP knowledge leaks outside `component/vpp` and `plugins/iface/vpp` |
| Operator errors and doctor codes | `doctor-vpp-lcp-netns` carries `Fix` (`ze show diagnostic doctor-vpp-lcp-netns` or `sudo ip netns add <name>`) and stays registered in `internal/plugins/iface/vpp` |
| Spec closure hygiene | Zero remaining citations to `plan/spec-bgp-netns.md` in Go source (`lcp.go`, `doctor.go`) |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| YANG default is `""` | `grep -n 'default ""' internal/component/vpp/yang/ze-vpp-conf.yang` and `./le test unit pkg path ./internal/component/config/` |
| `ParseSettings` and `startup.conf` default omits `default netns` | `./le test unit pkg path ./internal/component/vpp/` |
| `checkVPPLCPNetns` is silent on omitted `netns` and warns on explicit non-empty `netns` / markers | `./le test unit pkg path ./internal/plugins/iface/vpp/` |
| Functional tests prove both default-silent and explicit-warning paths | `./le test functional ui` (`doctor-vpp-lcp-netns-default.ci` and `doctor-vpp-lcp-netns.ci`) |
| Docs and source anchors consistent | `./le doc check all` and `./le yang glue check` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `validateNetns` continues to reject `/`, `\`, whitespace, non-printable chars, and names > 255 bytes for every non-empty value |
| No silent isolation drop on explicit config | Explicit `vpp { lcp { netns <name>; } }` is never overwritten with `""` by `ParseSettings` or `checkVPPLCPNetns` |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error or YANG schema error | Fix in Phase 2 |
| `lcpNetnsRequiredAdvice` or `lcpNetnsBannedAdvice` failure | Fix diagnostic wording in `register_doctor_codes.go` / `doctor.go` |
| Unrelated test assumes omitted `lcp.netns == "dataplane"` | Inspect the test: update if testing default, pass explicit `"dataplane"` if testing isolated netns |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- VPP's linux-cp netns model is two-level (`third_party/vpp-linux-cp/src/lcp.c`, `src/lcp_interface.c`): an empty per-pair netns falls back to VPP's global `default netns`. Earlier work already made `validateNetns` accept `""`, `GenerateStartupConf` omit `default netns` when `""`, and `checkVPPLCPNetns` treat `""` as the sole silent value. Changing the omitted-leaf default from `"dataplane"` to `""` therefore aligns the default with the already-shipped empty-netns path without touching `startupconf.go`, `lcp.go` behavior, or BGP.
- On 2026-11-24 Thomas superseded the 2026-07-16 ruling (which had kept `"dataplane"` and proposed a BGP-only listener netns leaf) in favor of defaulting `vpp.lcp.netns` to `""` for v1.0 and designing full-stack nestable `netns <name> { ... }` post-release alongside VRF.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Default `vpp.lcp.netns` to `""` (host/VPP's own namespace) for v1.0 | Keep `"dataplane"` default and add a BGP-only `listen { netns }` leaf | Thomas's 2026-11-24 ruling: makes VPP LCP TAPs and all routing protocols (BGP, OSPF, etc.) work out of the box in v1.0 without introducing a dead-end per-protocol netns leaf |
| Home full-stack `netns <name> { ... }` in `plan/post-release/features/spec-bgp-listen-netns.md` | Drop multi-namespace support entirely or bundle it into v1.0 | Keeps v1.0 focused and aligns post-release multi-namespace isolation with `plan/post-release/features/spec-vrf-0-umbrella.md` |
| Split default-silent and explicit-warning `.ci` checks into two files (`doctor-vpp-lcp-netns-default.ci` and `doctor-vpp-lcp-netns.ci`) | Combine both into one `.ci` file | `expect=stdout:contains=` matches accumulated stdout across commands, so combining them would make the warning/silence assertions vacuous |

## Known Limitations
- Multi-namespace isolation (`netns <name> { bgp { ... } interface { ... } }`) is deferred post-release to `plan/post-release/features/spec-bgp-listen-netns.md`. Operators who explicitly configure a non-empty `vpp.lcp.netns` in v1.0 must run `ze` inside that namespace so routing protocols can bind on the LCP TAPs.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry positive testable assertions and Permitted Survivors
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist and Documentation Update Checklist answered Yes/No/N-A with evidence

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread (`./le test unit pkg path ./internal/component/vpp/`, `./le test unit pkg path ./internal/plugins/iface/vpp/`, `./le test unit pkg path ./internal/component/config/`, `./le test functional ui`, `./le yang glue check`, `./le doc check all`, `./le go lint run`)
- [ ] Feature code integrated (`internal/component/vpp`, `internal/plugins/iface/vpp`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Post-release multi-namespace scope homed in `plan/post-release/features/spec-bgp-listen-netns.md`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests pass (`doctor-vpp-lcp-netns-default.ci` and `doctor-vpp-lcp-netns.ci`)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `./le spec review record`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-bgp-netns.md` only, in the same `./le commit create` script

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")
