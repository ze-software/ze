# Spec: traffic-ingress-shaping

| Field | Value |
|-------|-------|
| Status | ready |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release; existing supported behavior and its obligations remain required. |
| Scope | config |
| Depends | - |
| Phase | 0/6 |
| Handoff | - |
| Updated | 2026-10-15 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Thomas asked for Ze to do on a CPE what a VyOS procedure does today to cap a
CityFibre leased line. The VyOS procedure, by step:

| # | VyOS step | What Ze must offer |
|---|-----------|--------------------|
| 1 | A shaper policy with a bandwidth cap written as bps, kbit, mbit, gbit, tbit, or a percentage of the link speed; a default class at 100% bandwidth and 100% ceiling | A rate cap with those units, including `tbit` and a percentage |
| 2 | Apply the policy on eth5 egress | Already possible: `traffic control interface eth5 qdisc` |
| 3 | Create an IFB, redirect eth5 ingress traffic into it, apply the same shaper on the IFB egress | Shape the DOWNLOAD direction of eth5 |
| 4 | `show qos` shows it | `show traffic control` shows both directions with their rates |
| 5 | A hand-edited MOTD hook tells the operator that shaping is applied and which command shows it | A login banner notice DERIVED from the applied config, so it cannot drift |
| 6 | Deleting the config removes the redirect, the IFB and the qdiscs and restores the interface | Removal restores the interface exactly |

Owner decision, 2026-10-15: ingress is shaped through an IFB with a redirect from
the physical interface (the VyOS mechanism), not by an ingress policer. The
mechanism is settled, and so is the operator surface: an `ingress` block on
the traffic control entry, with Ze owning the IFB (Q-1).

The goal is one config stanza on a Ze CPE that caps both directions of a WAN
interface, that a logged-in operator is told about, and that leaves no trace when
it is removed.

## Owner Decisions (resolved 2026-10-15)

| ID | Question | Owner decision |
|----|----------|----------------|
| Q-1 | Operator surface for download shaping: (A) an `ingress` block on the `traffic control interface` entry, where Ze creates and owns the IFB and the redirect; or (B) the VyOS model, an `ifb` interface kind, a `redirect` leaf and an egress qdisc on the IFB | A. Ingress shaping is an `ingress { qdisc { ... } }` block on the traffic control interface entry, and Ze creates and owns the IFB. B's aggregate cap over several WAN links is not offered |
| Q-2 | VyOS binds one named shaper policy to both directions; Ze has no named policies | Rates are written inline, once per direction. Named policies are out of scope |
| Q-3 | Goodput pass band for the throughput test (A-6, AC-4) | 85% to 100% of the cap. Basis: TCP payload is about 94% of Ethernet line rate at a 1500-byte MTU (RFC 6349 framing), so a working shaper delivers goodput just under the cap and never above it |

## Required Reading

### Architecture Docs
- [ ] `docs/guide/traffic-control.md` - the operator page for `traffic control`
  → Constraint: `clsact` and `ingress` are refused as `qdisc type` values because the ingress hook belongs to mirroring and sampling, which install `clsact` at `ffff:0` themselves. Ingress shaping MUST NOT reopen that enumeration: Ze adds its redirect as one more priority-owned filter on the shared hook
  → Constraint: the VPP backend refuses by name what it cannot express, at `ze config commit`, in the form `<thing>: not supported by backend vpp`. Ingress shaping on VPP follows the same pattern
  → Constraint: the tc backend snapshots the original root qdisc before replacing it and restores it on removal; `noqueue` (every virtual device, so also an IFB) is restored by deleting the root
- [ ] `docs/architecture/traffic/tc-original-qdisc-restore.md` - persisted snapshot of the original root
  → Constraint: snapshots are keyed by device identity (boot id, name, ifindex, MAC) in the statestore. An IFB Ze creates needs NO snapshot: its original state is "absent", restored by deleting the device
- [ ] `docs/guide/command-reference.md` "show traffic control", "ze show warnings", "Login banner integration"
  → Constraint: the banner reads the report bus filtered to source `bgp` only, and a `code` switch inside the BGP config loader decides which issues it shows
  → Decision: the shaping notice is NOT a report-bus warning. A shaped CPE is healthy; raising a warning would make `show warnings`, and anything that counts warnings, report a healthy box as degraded
- [ ] `docs/architecture/traffic/cos-plugin.md` - shared registry pattern in `internal/core/cos`
  → Decision: a cross-component fact (here: login notices) lives in a small `internal/core/` registry the producer registers into, which is the pattern cos uses
- [ ] `plan/post-release/features/spec-bsd-traffic-shaping.md` - BSD shaping backend (skeleton)
  → Constraint: no overlap. That spec adds a backend off Linux; this one adds a direction on Linux tc and a refusal on VPP. A BSD backend will meet the new ingress field and must refuse it by name or implement it, which its own design phase decides
- [ ] VyOS `python/vyos/ifconfig/interface.py`, `Interface.set_mirror_redirect` (vyos-1x, `current` branch, read 2026-10-15)
  → Constraint: VyOS installs `tc qdisc add dev <src> handle ffff: ingress` and a `prio 10 u32 match u32 0 0 ... action mirred egress redirect dev <ifb>` filter, and it starts every update with `tc qdisc del dev <src> ingress`. Deleting the whole ingress qdisc is exactly the defect Ze fixed in `fixit-mirror-clsact-ownership` (journal row 2026-08-14): it erases every other owner's filters. Ze MUST NOT copy that lifecycle
  → Constraint: VyOS has no dedicated teardown; cleanup happens when the function runs again. Ze needs an explicit teardown

### RFC Summaries (Scope: protocol)
- N-A: tc shaping is a local queueing policy, with no protocol and no RFC obligation.

**Key insights:**
- The `traffic.InterfaceQoS` model already carries an ingress direction: `Ingress Policer`, set only by the L2TP shaper (not reachable from `traffic control` YANG), installed by tc as matchall+police at clsact priority 200, refused by VPP.
- The clsact ingress hook has three priority owners today: mirror 1, flow sampling 100, L2TP policer 200. The redirect becomes a fourth.
- `show traffic control` today reads back only the root qdisc TYPE; its `class-count` is always 0, a silently wrong value.
- The tc backend fails an interface entry that has no `qdisc` block, although the YANG says such an entry attaches nothing. A download-only entry hits it.
- The SSH login banner is wired only on the BGP startup path; a CPE without `bgp { }` gets no banner at all.
- The appliance kernel lacks `CONFIG_IFB`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/traffic/yang/ze-traffic-control-conf.yang` (178L) - `traffic/control/interface[name]` with a presence `qdisc` (type, default-class, class list with rate, ceil, priority, match). `rate-bps` pattern accepts `bit|kbit|mbit|gbit|bps|kbps|mbps|gbps`: no `tbit`, no percentage
  → Constraint: `qdisc` is a presence container whose mandatory `type` is required only when present (revision 2026-10-12). A new `ingress` container MUST follow the same presence pattern
- [ ] `internal/component/traffic/config.go` (239L) - `parseInterfaceQoS` returns early when `qdisc` is absent; `ParseRateBps` walks `rateSuffixes` longest first with an overflow check; it is shared with the RADIUS Filter-Id parser (`filterid_rate.go`)
  → Constraint: `ParseRateBps` is fed by a remote party (RADIUS). Adding `tbit`/`tbps` there widens what RADIUS can send, and the overflow guard MUST cover the new multipliers. A percentage MUST NOT be accepted by `ParseRateBps` itself, because a RADIUS Filter-Id has no interface whose speed it could be relative to
- [ ] `internal/component/traffic/model.go` (268L) - `InterfaceQoS{Interface, Qdisc, Ingress Policer}`; its doc comment says ingress "has no queue to delay into, so the only enforcement available there is to drop"
  → Constraint: that comment becomes false once an IFB gives ingress a queue; it MUST be rewritten in the same phase (`ai/rules/stale-comments.md`)
- [ ] `internal/component/traffic/backend.go` (188L) - `Backend{Apply, ListQdiscs, Close}`, optional `OriginalStateRestorer`, `RegisterBackend`, `RegisterVerifier`/`RunVerifier`
  → Constraint: backends register; the component never names one. The VPP refusal goes in the VPP verifier, never in a switch in the component
- [ ] `internal/component/traffic/register.go` (410L) - `OnConfigVerify` parses and runs the backend verifier; `OnConfigApply` restores interfaces that left the config via `RestoreOriginal`, then calls `Apply(desired)` for the rest, inside an `sdk.Journal` whose rollback re-applies the previous config
  → Constraint: an interface that STAYS in the config but drops its `ingress` block is not restored by `RestoreOriginal` (only removed interfaces are). The backend `Apply` MUST tear down ingress shaping for an entry whose ingress is now absent: Apply is a desired-state call
- [ ] `internal/component/traffic/cmd/traffic.go` - `handleShowTraffic` (`ze-traffic:show-traffic`) answers `qdisc`, `class-count`, `default-class` per interface from `backend.ListQdiscs`
- [ ] `internal/component/traffic/cmd/show.go` - `FormatQoS`, `formatRate` (`gbit`/`mbit`/`kbit`/`bit`)
- [ ] `internal/plugins/traffic-cmd/yang/ze-traffic-cmd.yang` - `show traffic control [<ifname>]`
- [ ] `internal/plugins/traffic/netlink/backend_linux.go` (380L) - `Apply` per sorted interface: `ensureSnapshot`, `applyInterface` (translate root, replace, classes, filters, then `applyIngressPolicer`); `restoreOriginalLocked` removes the policer first, then restores the root; `ListQdiscs` fills only `Qdisc.Type`
  → Constraint: `applyInterface` calls `translateQdisc` unconditionally; with no `qdisc` block the type is unknown and `translateQdisc`'s default arm returns `unsupported qdisc type`. The ingress-only entry MUST skip the root, and restore it when Ze installed one earlier
- [ ] `internal/plugins/traffic/netlink/policer_linux.go` - `applyIngressPolicer` ADDS the clsact at `ffff:0` and tolerates EEXIST, never replaces; `removeIngressPolicer` deletes priority 200 only and tolerates ENOENT/EINVAL (`isPolicerAbsent`); the header comment states the priority order (mirror 1, sampling 100, policer 200 last so earlier owners still see the packet)
  → Constraint: the redirect is a TERMINAL verdict (mirred egress redirect steals the packet), so it MUST sit after mirror (1) and sampling (100); a policer at 200 would never see a redirected packet, so ingress shaping and an ingress policer on one interface MUST be refused
  → Decision: the redirect owns priority 150, and its add/remove follows the same add-tolerate-EEXIST and delete-tolerate-absent shape
  → Constraint: `applyIngressPolicer` returns early when no policer is asked for and does not remove one left from a previous apply; no caller reaches that gap today, because the only producer of a policer, the L2TP shaper's `applyTC`, always sends one (a rate change without an upload rate keeps the session's existing rate) and `restoreOriginalLocked` removes it when the session ends. The ingress shaping path MUST NOT copy that early return, because the traffic config CAN drop an `ingress` block from an entry that stays, and AC-8 requires removal on re-apply
- [ ] `internal/plugins/traffic/netlink/translate_linux.go` - `translateQdisc` (root at `1:0`, HTB `Defcls` from `findDefaultClassMinor`), `translateClass`, `translateFilter`
  → Constraint: the IFB root qdisc is built by the SAME translation as the egress root; no second translator
- [ ] `internal/plugins/traffic/vpp/verify.go` - `errIngressPolicerNotSupportedByBackend`, `verifyInterface`
  → Decision: VPP refuses `ingress` shaping the same way. VPP's userspace data plane does not pass received packets through a Linux IFB, and an input policer is the policing the owner rejected
- [ ] `internal/plugins/iface/netlink/mirror_linux.go` - `mirrorFilterPriority = 1`; teardown removes its own filters and leaves the shared clsact
- [ ] `internal/plugins/flowexport/sampling/sample.go` - `SampleFilterPriority = 100`
- [ ] `internal/component/iface/device_owner.go`, `macvlan.go`, `config_apply.go` `reconcileOwnedDevices` - owned-device registry: alias `ze:owned:<owner>`, async reconcile on the iface worker, orphan scan over MACVLANS only; `ComposeOwnedDeviceName(prefix, parentIfindex, id)` composes a name inside IFNAMSIZ and refuses rather than truncates
  → Decision: reuse `ComposeOwnedDeviceName` and the `ze:owned:` alias convention for the IFB name and ownership mark; do NOT route creation through the iface registry, because its reconcile is asynchronous and the traffic Apply must attach a qdisc to the IFB in the same call (see Key Design Decisions)
  → Constraint: the iface orphan scan deletes only `macvlan` kinds, so an IFB marked `ze:owned:traffic` is never deleted by iface. The traffic backend owns its own orphan scan
- [ ] `internal/component/iface/config.go` `parseIfaceConfig` - each interface kind (`ethernet`, `dummy`, `veth`, ...) is read by its literal key, matching one hand-written `list` per kind in `ze-iface-conf.yang`
  → Constraint: design B adds an `ifb` kind to that hand-listed set; design A leaves it untouched
- [ ] `internal/plugins/iface/netlink/show_linux.go` - `LinkSpeedDuplex` reads `/sys/class/net/<n>/speed`, answers 0 for a down link and for devices with no speed; the VPP iface backend always answers 0; consumers call it themselves (`iface.LinkSpeedDuplex`, used by OSPF auto-cost and flow export)
  → Constraint: a percentage resolved against 0 MUST NOT become a zero rate. Unknown speed is a stated condition, never a number
- [ ] `internal/component/iface/resolve.go`, `iface.go` - `iface.Subscribe(name)` delivers `LinkEvent{LinkAppeared, LinkUp, LinkDown}` for a logical name
  → Decision: percentage rates are re-resolved on `LinkUp` of the shaped interface, which covers boot with the link down and speed renegotiation (a renegotiation drops carrier)
- [ ] `internal/component/bgp/config/loader.go` `collectPrefixWarnings`, `internal/component/config/infra/hook.go` `HookParams.CollectLoginWarnings`, `cmd/ze/hub/service_ssh.go` `SetLoginWarnings`, `cmd/ze/hub/session_factory.go`
  → Constraint: the banner source is ONE function supplied by the BGP loader through a hook field, filtering `report.Warnings()` to source `bgp` and switching on two codes; the standalone (no-bgp) SSH path, `sshBuildStandaloneImpl`, never calls `SetLoginWarnings`. A traffic notice cannot reach it without editing BGP code, which is the central-enumeration shape `ai/rules/principles.md` forbids
  → Decision: replace the hook field with a login-notice registry in `internal/core/loginnotice`; BGP and traffic each register a provider; both SSH startup paths read the registry. No layering: the hook field and its plumbing are deleted, not kept beside it
- [ ] `internal/component/kernelcap/kernelcap.go` - `Capability{Subsystem, Kernel, ConfigLeaf, InUse, Probe, CodeAbsent, CodeUnknown}` registered by the owner via `MustRegister`, which also registers the doctor check
- [ ] `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require` - `CONFIG_NET_SCH_HTB`, `CONFIG_NET_SCH_INGRESS`, `CONFIG_NET_CLS_MATCHALL`, `CONFIG_NET_ACT_MIRRED` are `=y` and required; `CONFIG_IFB` is absent from both
  → Constraint: the appliance image MUST gain `CONFIG_IFB=y` and the require row, or the feature cannot work on the CPE it was asked for

**Behavior to preserve:**
- Every existing egress config applies, shows and restores as today.
- The L2TP shaper's ingress policer (priority 200) is unchanged.
- Mirror (priority 1) and sampling (priority 100) filters are never removed or reordered by traffic control, and the shared clsact qdisc is never replaced or deleted by teardown.
- The BGP prefix warnings appear on the banner exactly as today (same text, same collapse to `N warnings` / `show warnings`).
- `ParseRateBps` keeps refusing overflow and keeps its existing units.

**Behavior to change:**
- `traffic control interface X` gains an `ingress { qdisc { ... } }` block shaping received traffic through a Ze-owned IFB (tc backend), refused by name on VPP.
- `rate`/`ceil` accept `tbit`/`tbps` and an integer percentage of the shaped interface's link speed.
- An interface entry with no `qdisc` block programs no root on the tc backend (today it fails).
- `show traffic control` reports both directions, each class's rate and ceil read back from the kernel, the IFB device, and for percentages the configured share and the speed used.
- The login banner shows a traffic-shaping notice derived from the applied config, on every SSH startup path.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `traffic { control { interface eth5 { qdisc { ... } ingress { qdisc { ... } } } } }`, delivered to the traffic plugin as a `map[string]any` section (`WantsConfig: traffic`).
- CLI: `show traffic control [<ifname>]` (RPC `ze-traffic:show-traffic`).
- SSH login: the session factory asks for login notices.

### Transformation Path
1. `parseInterfaceQoS` reads `qdisc` into `Qdisc` and `ingress/qdisc` into a new ingress-qdisc field of `InterfaceQoS` (name settled in Phase 1); rates parse as either an absolute bps value or a link share.
2. `OnConfigVerify`: YANG validation, then `RunVerifier(backend)`. tc verifier: refuses `ingress` together with an ingress policer on one interface. VPP verifier: refuses `ingress` and any percentage rate.
3. `OnConfigApply`: the component resolves every link share to bps with `iface.LinkSpeedDuplex(name)`. An unresolved share leaves that direction of that interface unprogrammed, raises a report-bus warning naming the interface and the fix, and is retried on `LinkUp`. The backend receives bps only.
4. tc backend `Apply` per interface: root qdisc only when configured, restored when it was removed; then ingress: compose the IFB name, create the IFB with alias `ze:owned:traffic` when absent and set it up, build the IFB root from the same translation, ensure clsact on the source (add, tolerate EEXIST), install the priority-150 matchall + mirred egress redirect filter to the IFB. Order: IFB qdisc first, redirect last, so no packet is redirected to an unshaped IFB.
5. Teardown (entry or `ingress` removed): delete the priority-150 filter first, then delete the IFB (its qdiscs go with it), leave the clsact qdisc. Orphan scan: any `ifb` link with alias `ze:owned:traffic` that no desired entry names is deleted (crash and reboot leftovers).
6. `ListQdiscs` reads the root and its classes (rate, ceil) and, when the priority-150 redirect is present, the IFB name and the IFB's root and classes.
7. The traffic component registers a login-notice provider that reads the APPLIED config (`activeCfg`), so the notice exists exactly when Ze has shaping installed from config.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config → traffic plugin | SDK config section, `map[string]any` | No |
| traffic component → backend | `Backend.Apply(map[string]InterfaceQoS)` with resolved bps | No |
| tc backend → kernel | rtnetlink: `LinkAdd` kind `ifb`, `LinkSetAlias`, `LinkSetUp`, `QdiscAdd/Replace`, `ClassAdd`, `FilterAdd/Del` (matchall + mirred), `LinkDel` | No |
| traffic → iface | `iface.LinkSpeedDuplex`, `iface.Subscribe`, `iface.ComposeOwnedDeviceName` | No |
| traffic, bgp → SSH banner | `internal/core/loginnotice` registry, read by `cmd/ze/hub` on each login | No |

### Integration Points
- `traffic.RegisterVerifier` (tc and vpp verifiers) - new refusals.
- `kernelcap.MustRegister` - new `traffic-ingress` capability probing the `ifb` link kind.
- `report.Raise`/`Clear` - the unresolved-share warning.
- `iface.Subscribe` - re-resolution on `LinkUp`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Config → component → registered backend → kernel, the existing path; the IFB is created inside the backend that programs it |
| No unintended coupling (components stay isolated) | Yes | traffic already imports iface (`cmd/traffic.go` uses `iface.ListInterfaces`); bgp and traffic meet only in a core registry |
| No duplicated functionality (extends existing, does not recreate) | Yes | IFB root reuses `translateQdisc`/`translateClass`; naming reuses `ComposeOwnedDeviceName`; clsact handling follows `applyIngressPolicer`'s add-tolerate-EEXIST shape |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Control plane only; Ze handles no packet |
| Registration over hardcoding, outbound | Yes | Login notices register; the VPP refusal lives in the VPP verifier; the kernel requirement registers through kernelcap |
| Registration over hardcoding, inbound | Yes | Lists searched for the new names: `qdisc-type` enum (untouched: no new qdisc type), `rateSuffixes` (gains `tbit`/`tbps`; the table IS the declaration), `collectPrefixWarnings` code switch (no longer the banner's only source; stays BGP's own provider), iface kind lists in `ze-iface-conf.yang` and `parseIfaceConfig` (untouched under design A) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The traffic component runs in the daemon process, so a Go registry in `internal/core` reaches the SSH server | `handleShowTraffic` already reads `traffic.GetBackend()` in the hub process | The notice never reaches the banner when traffic runs external | Wiring test `TestLoginBannerShowsTrafficShaping` through the real session factory | unvalidated |
| A-2 | A link-speed change always passes through carrier down/up, so re-resolving on `LinkUp` follows it | Ethernet autonegotiation drops carrier to renegotiate | A share stays resolved against a stale speed | `TestTrafficIngressShareFollowsLinkUp`; documented limitation for a driver that changes speed without a carrier event | unvalidated |
| A-3 | Packets redirected to an IFB re-enter the source device's receive path after the IFB's egress qdisc, so routing and firewalling see them as received on eth5 | Linux `drivers/net/ifb.c`, `ifb_ri_tasklet`, re-injects with `skb->dev` restored to the original device | Firewall rules keyed on input interface stop matching | Throughput integration test: a shaped download still reaches a socket in the netns; read the kernel source at implementation and quote it in `ingress_linux.go` | unvalidated |
| A-4 | The mirror and the sampler still see every ingress packet, because their filters run at lower priority numbers before the terminal redirect | Kernel walks the clsact chain in priority order (rationale in `policer_linux.go`) | Port mirroring or flow export silently loses the download direction | `TestNetlinkIntegration_IngressShapingLeavesOtherHookOwnersAlone`: mirror filter at 1 still present and counting after shaping applies | unvalidated |
| A-5 | `vishvananda/netlink` exposes an `Ifb` link type and `MatchAll` + `MirredAction` with `TCA_EGRESS_REDIR` | The library is already used for matchall + mirred in `mirror_linux.go` | Hand-built netlink attributes needed | Compile of Phase 3 | unvalidated |
| A-6 | An HTB default class with rate = ceil = cap on the IFB limits TCP download goodput to between 85% and 100% of the cap over 5 s | Owner decision Q-3: TCP payload is about 94% of Ethernet line rate at 1500 MTU (RFC 6349 framing) | The bound is the owner's; a result outside it is reported, never absorbed by widening the bound | `TestIngressShapingLimitsDownloadThroughput` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A redirect installed before the IFB has its qdisc lets an unshaped burst through, or drops traffic if the IFB is down | Throughput test spikes at apply | Apply order: IFB up with qdisc, then redirect; teardown reverses it |
| R-2 | Teardown deletes the shared clsact and erases mirror/sampling filters (the VyOS lifecycle) | Mirror integration test goes red | Remove priority 150 only; never `QdiscDel` on clsact; the test asserts a foreign filter survives |
| R-3 | A crash between IFB create and redirect, or a reboot, leaves an orphan IFB or redirect | An IFB with Ze's alias and no config | Alias `ze:owned:traffic` set at create; Apply runs an orphan scan over `ifb` links with that alias and over priority-150 redirect filters on configured interfaces |
| R-4 | IFB name collision with an operator's device | `LinkAdd` returns EEXIST on a device without Ze's alias | Refuse naming the device and the fix; never adopt or delete a device without the alias |
| R-5 | A percentage on a down link at boot fails the whole traffic apply, leaving the CPE unshaped | Apply error at boot | An unresolved share leaves only that interface direction unprogrammed, raises `traffic / rate-unresolved` on the report bus, re-applies on `LinkUp` |
| R-6 | A share resolved against 0 becomes a zero rate (HTB at 0 drops all traffic) | Download stalls | The share type has no zero meaning; resolution refuses speed 0; `TestResolveShareRefusesUnknownSpeed` |
| R-7 | Ingress shaping plus the L2TP ingress policer on one interface: the policer never sees redirected packets, so its limit silently stops working | Policer counters flat | The tc verifier refuses the combination by name |
| R-8 | `tbit` widens what RADIUS Filter-Id can carry; a large value overflows | Boundary test | The overflow guard covers the new multipliers; boundary rows below |
| R-9 | Appliance kernel without `CONFIG_IFB` makes `LinkAdd` fail at apply with an opaque errno | Apply error on the CPE | `CONFIG_IFB=y` in the image; kernelcap capability gives `ze doctor` and the startup refusal a named diagnostic |
| R-10 | The banner refactor changes BGP banner text or drops a path | Existing banner tests | Existing BGP banner tests unchanged and green; new standalone-path wiring test |
| R-11 | `show traffic control` class names: the kernel stores handles, not names | Show prints `1:3` instead of `default` | Map handles back through the applied config; a kernel class with no config counterpart is shown by handle and marked as not Ze's |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Download traffic on the shaped WAN interface (stalls if the IFB path is wrong); mirror/sampling on that interface if the clsact protocol is broken; the SSH banner for everyone if the registry refactor is wrong |
| How is it reverted? | Single commit revert; no persisted format changes (IFBs are runtime state, the snapshot blob is untouched) |
| Who else touches this path? | Mirror (iface netlink), flow-export sampling, L2TP shaper (ingress policer), anomaly/shape (per-source rate limits through traffic), BGP config loader (banner) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config `traffic control interface X ingress qdisc` at boot | → | `parseInterfaceQoS` → tc `Apply` → IFB + redirect | `test/traffic/traffic-ingress-shaping-boot.ci` |
| Config reload removing the `ingress` block | → | tc `Apply` teardown of redirect and IFB | `test/traffic/traffic-ingress-shaping-remove.ci` |
| `show traffic control eth0` | → | `handleShowTraffic` → `ListQdiscs` reading both directions | `test/traffic/traffic-ingress-shaping-show.ci` |
| SSH login with shaping configured, no `bgp { }` section | → | `loginnotice.Collect` → traffic provider | `TestLoginBannerShowsTrafficShaping` (`cmd/ze/hub/session_factory_test.go`) and `test/traffic/traffic-ingress-shaping-banner.ci` |
| `ze config commit` with `backend vpp` and `ingress` | → | VPP verifier refusal | `test/traffic/traffic-vpp-refuse-ingress-shaping.ci` |
| `ze doctor` on a kernel without IFB | → | kernelcap `traffic-ingress` check | `TestTrafficIngressCapabilityAbsent` (`internal/plugins/traffic/netlink/register_kernelcap_linux_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `rate 500mbit`, `rate 2tbit`, `rate 1tbps`, `rate 100bps`, `rate 50%` | Each accepted; `2tbit` = 2,000,000,000,000 bps; `1tbps` = 8,000,000,000,000 bps; `50%` held as a share of link speed |
| AC-2 | `rate 0%`, `rate 101%`, `rate 50.5%`, `rate 5pbit` | Refused at validation, naming the leaf and the accepted forms |
| AC-3 | `ingress { qdisc { type htb; default-class default; class default { rate 500mbit; ceil 500mbit } } }` on eth0, tc backend | An IFB device carrying alias `ze:owned:traffic` exists and is up; its root is HTB with that class; eth0's clsact ingress hook carries a priority-150 matchall filter redirecting to that IFB |
| AC-4 | Same config, then a TCP download into the shaped interface | Measured goodput is between 85% and 100% of the cap over a 5 s window (A-6, owner decision Q-3) |
| AC-5 | The same entry without the `ingress` block, then the same TCP download | Goodput exceeds the cap by at least 2x (proves AC-4 is not vacuous) |
| AC-6 | Entry with `ingress` and no `qdisc` block | Applies; eth0 root unchanged from its original; ingress shaped |
| AC-7 | Entry with `qdisc` and `ingress`, egress and ingress caps differ | Each direction capped at its own rate (upload by eth0 root, download by IFB root) |
| AC-8 | Remove the `ingress` block, keep the entry | Priority-150 filter gone, IFB deleted, egress root untouched, clsact still present with any foreign filters intact |
| AC-9 | Remove the whole `traffic` section | eth0 root restored to its snapshot, no priority-150 filter, no Ze-owned IFB. The clsact qdisc is left in place (Key Design Decisions), carrying no Ze filter |
| AC-10 | A mirror (priority 1) and an ingress shaping config on the same interface | Both work: the mirror destination receives the received frames, download is capped; removing shaping leaves the mirror filter in place |
| AC-11 | `ingress` plus an L2TP ingress policer on one interface | Refused at verify: names the interface, says the redirect would bypass the policer, and how to fix |
| AC-12 | `backend vpp` with `ingress` | Refused at `ze config commit`: `ingress shaping: not supported by backend vpp`, with why and the fix (`backend tc`) |
| AC-13 | `backend vpp` with a percentage rate | Refused at commit naming the leaf: VPP interfaces report no link speed; give an absolute rate |
| AC-14 | Percentage rate, shaped interface link down at apply | Other interfaces apply; this direction is left unprogrammed; `show warnings` carries `traffic / rate-unresolved` naming the interface and the fix; on link up the share resolves, shaping installs, the warning clears |
| AC-15 | Percentage rate, link speed 1000 Mbit/s, `rate 50%` | Kernel class rate is 500 Mbit/s; `show traffic control` reports `50%` and the 1000 Mbit/s speed used |
| AC-16 | Daemon killed between IFB creation and redirect install, then restarted with the same config | One IFB, one redirect, no duplicate |
| AC-17 | Daemon restarted with the `ingress` block removed from the config while it was down | The orphan IFB (alias `ze:owned:traffic`) is deleted and its redirect filter removed |
| AC-18 | A device already exists under the composed IFB name without Ze's alias | Apply refuses naming the device and the fix; the foreign device is untouched |
| AC-19 | `show traffic control` and `show traffic control eth0` with the AC-7 config | Both directions shown: qdisc type, each class's name, rate and ceil read back from the kernel, the IFB device name for ingress; `class-count` matches the configured classes |
| AC-20 | SSH login, shaping applied on eth0 only, no `bgp { }` | Banner line saying traffic shaping is applied on eth0, with command `show traffic control eth0` |
| AC-21 | SSH login, shaping on two interfaces | One collapsed line naming the count, with command `show traffic control` |
| AC-22 | SSH login after the traffic config is removed | No traffic notice |
| AC-23 | SSH login with BGP prefix warnings and no traffic config | Banner identical to today's |
| AC-24 | Kernel without the `ifb` link kind, ingress configured | `ze doctor` names the missing `CONFIG_IFB`; daemon start behaves per kernelcap's existing policy with the code and the fix |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Caps upload and download of a WAN port at 500 Mbit/s | config → traffic verify → apply → tc root + IFB + redirect | `traffic-ingress-shaping-boot.ci`, `TestIngressShapingLimitsDownloadThroughput` |
| 2 | Checks what is applied | `show traffic control` → `ListQdiscs` → kernel read-back | `traffic-ingress-shaping-show.ci` |
| 3 | Logs in and is told shaping is applied | SSH session factory → `loginnotice.Collect` → traffic provider | `traffic-ingress-shaping-banner.ci` |
| 4 | Removes the cap | reload → `Apply` teardown / `RestoreOriginal` → orphan scan | `traffic-ingress-shaping-remove.ci`, `TestNetlinkIntegration_IngressShapingTeardownRestoresInterface` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseRateTbit` | `internal/component/traffic/config_test.go` | AC-1 `tbit`/`tbps` and their overflow bound | |
| `TestParseRateShare` | `internal/component/traffic/config_test.go` | AC-1/AC-2 percentage parse, 1..100, integer only | |
| `TestParseRateBpsRefusesShare` | `internal/component/traffic/filterid_rate_test.go` | The RADIUS path never accepts `%` | |
| `TestParseInterfaceQoSIngress` | `internal/component/traffic/config_test.go` | `ingress/qdisc` parsed into the model; absent block is absent | |
| `TestResolveShareRefusesUnknownSpeed` | `internal/component/traffic/share_test.go` | R-6, AC-14 | |
| `TestResolveShareRoundsAndBounds` | `internal/component/traffic/share_test.go` | AC-15 arithmetic, overflow at high speeds | |
| `TestTCVerifierRefusesShapingWithPolicer` | `internal/plugins/traffic/netlink/verify_test.go` | AC-11 | |
| `TestVPPVerifierRefusesIngressShaping` | `internal/plugins/traffic/vpp/verify_test.go` | AC-12 | |
| `TestVPPVerifierRefusesShare` | `internal/plugins/traffic/vpp/verify_test.go` | AC-13 | |
| `TestApplyIngressOrdersQdiscBeforeRedirect` | `internal/plugins/traffic/netlink/backend_linux_test.go` | R-1, through the fake ops recorder | |
| `TestApplyWithoutQdiscProgramsNoRoot` | `internal/plugins/traffic/netlink/backend_linux_test.go` | AC-6 | |
| `TestTeardownRemovesOnlyPriority150` | `internal/plugins/traffic/netlink/backend_linux_test.go` | R-2, AC-8 | |
| `TestIFBNameRefusesForeignDevice` | `internal/plugins/traffic/netlink/ingress_linux_test.go` | AC-18 | |
| `TestLoginNoticeRegistryCollects` | `internal/core/loginnotice/loginnotice_test.go` | Registration, ordering by provider name, per-provider collapse | |
| `TestTrafficLoginNoticeDerivedFromAppliedConfig` | `internal/component/traffic/notice_test.go` | AC-20, AC-21, AC-22 | |
| `TestBGPBannerUnchanged` | `internal/component/bgp/config/loader_test.go` | AC-23 (existing banner tests re-pointed at the provider) | |
| `TestShowTrafficReportsBothDirections` | `internal/component/traffic/cmd/show_test.go` | AC-19 formatting, handle-to-name mapping (R-11) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| rate share | 1..100 % | 1%, 100% | 0% | 101% |
| rate tbit | 1..18446744 tbit (uint64 bps) | 18446744tbit | 0tbit (zero rate refused as today) | 18446745tbit (overflow) |
| rate tbps | 1..2305843 tbps (uint64 bps) | 2305843tbps | 0tbps | 2305844tbps (overflow) |
| resolved share bps | link Mbit/s × 1,000,000 × share / 100 | 100% of the highest speed sysfs reports | speed 0 refused | product above uint64 refused |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `traffic-ingress-shaping-boot` | `test/traffic/traffic-ingress-shaping-boot.ci` | Boot with egress and ingress caps; `tc` shows the IFB root and the priority-150 redirect (`option=needs-linux`) | |
| `traffic-ingress-shaping-remove` | `test/traffic/traffic-ingress-shaping-remove.ci` | Reload without `ingress`; IFB and redirect gone, root untouched | |
| `traffic-ingress-shaping-show` | `test/traffic/traffic-ingress-shaping-show.ci` | `show traffic control eth0` reports both directions and rates | |
| `traffic-ingress-shaping-banner` | `test/traffic/traffic-ingress-shaping-banner.ci` | SSH login without bgp shows the notice and the command | |
| `traffic-vpp-refuse-ingress-shaping` | `test/traffic/traffic-vpp-refuse-ingress-shaping.ci` | Commit refused with the VPP message | |
| `traffic-rate-share-out-of-range` | `test/parse/traffic-rate-share-out-of-range.ci` | `rate 101%` refused at validate | |

### Integration Tests (`//go:build integration && linux`, run under `./le test qemu`)
| Test | File | Proves |
|------|------|--------|
| `TestNetlinkIntegration_IngressShapingReachesTheKernel` | `internal/plugins/traffic/netlink/integration_linux_test.go` | AC-3, AC-6 read back from a real kernel in a netns |
| `TestIngressShapingLimitsDownloadThroughput` | `internal/plugins/traffic/netlink/throughput_linux_test.go` | AC-4, AC-5, AC-7: veth pair across two netns, a Go TCP sender in the peer netns, a receiver in the shaped netns, goodput measured over 5 s with and without the config; the shaped run asserts 85% to 100% of the cap. This is the goal evidence; it MUST be shown red by disabling the redirect install, and the red recorded |
| `TestNetlinkIntegration_IngressShapingLeavesOtherHookOwnersAlone` | `internal/plugins/traffic/netlink/integration_linux_test.go` | AC-10, A-4: a real mirror filter at 1 and a foreign filter survive apply and teardown |
| `TestNetlinkIntegration_IngressShapingTeardownRestoresInterface` | `internal/plugins/traffic/netlink/integration_linux_test.go` | AC-8, AC-9 |
| `TestNetlinkIntegration_IngressOrphanScan` | `internal/plugins/traffic/netlink/integration_linux_test.go` | AC-16, AC-17 |
| `TestTrafficIngressShareFollowsLinkUp` | `internal/component/traffic/share_linux_test.go` | AC-14 with a veth brought down and up |

### Interop Tests (Scope: protocol)
N-A: no protocol peer and no wire-visible protocol change. Goal validation is the throughput test above (`ai/rules/interop-and-goal-validation.md`: the goal evidence this feature owes is the measured cap).

## Files to Modify
- `internal/component/traffic/yang/ze-traffic-control-conf.yang` - a grouping shared by `qdisc` and `ingress/qdisc`; the `ingress` presence container; `rate-bps` pattern gains `tbit|tbps` and `N%`; new revision
- `internal/component/traffic/config.go` - parse `ingress`; share parse beside `ParseRateBps`; `rateSuffixes` gains `tbit`, `tbps`
- `internal/component/traffic/model.go` - ingress qdisc field; link-share type with an explicit absent state; rewrite the `InterfaceQoS` comment (ingress now has a queue through the IFB; the policer remains for subscriber sessions)
- `internal/component/traffic/register.go` - resolve shares before `Apply`; `LinkUp` re-apply; register the login-notice provider; raise and clear `traffic / rate-unresolved`
- `internal/component/traffic/cmd/traffic.go`, `internal/component/traffic/cmd/show.go` - both directions, classes with rate and ceil, IFB name, share and speed
- `internal/plugins/traffic-cmd/yang/ze-traffic-cmd.yang` - description of the answer
- `internal/plugins/traffic/netlink/backend_linux.go` - skip an absent root; ingress apply and teardown; orphan scan; `ListQdiscs` class and ingress read-back
- `internal/plugins/traffic/netlink/ops_linux.go` - link add, delete, alias, up; class list and filter list on the ops seam
- `internal/plugins/traffic/netlink/register.go` - tc verifier registration (AC-11)
- `internal/plugins/traffic/netlink/policer_linux.go` - header comment: four hook owners
- `internal/plugins/traffic/vpp/verify.go` - AC-12, AC-13 refusals
- `internal/component/bgp/config/loader.go`, `internal/component/bgp/config/loader_create.go` - register `collectPrefixWarnings` as a login-notice provider
- `internal/component/config/infra/hook.go` - delete `CollectLoginWarnings` and its `LoginWarning` mirror type
- `cmd/ze/hub/service_ssh.go`, `cmd/ze/hub/session_factory.go` - read `loginnotice.Collect` on both the BGP and the standalone path
- `internal/core/diagnostic/codes.go` - two codes for the `traffic-ingress` capability
- `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require` (and `internal/appliance/kernelreq.go` if it carries the symbol list) - `CONFIG_IFB`
- `docs/guide/traffic-control.md`, `docs/guide/command-reference.md`, `docs/architecture/core-design.md` (traffic section), `features/traffic-control-lifecycle.md`

## Files to Create
- `internal/core/loginnotice/loginnotice.go` - registry: register a provider, collect notices; collapse stays with each provider
- `internal/component/traffic/share.go` - link-share resolution with `iface.LinkSpeedDuplex`
- `internal/component/traffic/notice.go` - login-notice provider derived from `activeCfg`
- `internal/plugins/traffic/netlink/ingress_linux.go` - IFB lifecycle, redirect filter at priority 150, orphan scan
- `internal/plugins/traffic/netlink/verify.go` - tc verifier
- `internal/plugins/traffic/netlink/register_kernelcap_linux.go` - `traffic-ingress` capability
- `docs/architecture/traffic/ingress-shaping.md` - IFB ownership, hook priorities, apply and teardown order, orphan scan
- The tests and `.ci` files named in the test plan
- `features/traffic-ingress-shaping.md` - feature record, unless `./le feature report` places it as a scope row of `traffic-control-lifecycle.md`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-traffic-control-conf.yang`: `ingress` container, shared grouping, rate pattern |
| YANG validation constraints | Yes | `rate-bps` pattern: `tbit`, `tbps`, an integer 1..100 followed by `%`; `ingress` presence |
| YANG custom validators | No | The pattern covers the forms; cross-leaf refusals (policer, VPP) are backend verifiers |
| CLI commands/flags | Yes | `show traffic control` answer shape (no new verb) |
| CLI grammar (keyword before value) | Yes | Unchanged grammar; `rate 50%` is a value form of an existing leaf |
| Editor autocomplete | No | Automatic from YANG; no dynamic values |
| Functional test for new RPC/API | Yes | `traffic-ingress-shaping-show.ci` |
| Pipe completeness | Yes | `show traffic control` answers a `plugin.Map` through the existing pipe path; the `.ci` checks `| json` on the new shape |
| Env var registration | No | No new env var |
| Doctor check for runtime dependencies | Yes | `ifb` link kind: `register_kernelcap_linux.go`, `internal/core/diagnostic/codes.go`, `TestTrafficIngressCapabilityAbsent` |
| Prometheus counters/metrics | No | Kernel class counters are shown by `show traffic control`; no metric was asked for |
| BGP family surface | N-A | Not a BGP change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/traffic-ingress-shaping.md` or `features/traffic-control-lifecycle.md` (level from `./le feature report`) |
| 2 | Config syntax changed? | Yes | `docs/guide/traffic-control.md` (ingress section, rate units, percentage); `docs/guide/configuration.md` if it lists rate units |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`: "show traffic control" answer; "Login banner integration" (registry, every startup path, traffic notice) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` if it documents the `ze-traffic:show-traffic` answer |
| 5 | Plugin added/changed? | No | No plugin added; the traffic plugin's behavior is documented on its guide page |
| 6 | Has a user guide page? | Yes | `docs/guide/traffic-control.md`: ingress, hook ownership table (1 mirror, 100 sampling, 150 redirect, 200 policer), VPP matrix rows, failure-mode rows |
| 7 | Wire format changed? | N-A | No wire format |
| 8 | Plugin SDK/protocol changed? | N-A | No SDK change |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC |
| 10 | Test infrastructure changed? | Yes, if the netns throughput helper is shared | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | Yes, if `docs/comparison.md` carries a QoS row | `docs/comparison.md` (VyOS parity: ingress shaping) |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` traffic section; new `docs/architecture/traffic/ingress-shaping.md` |
| 13 | Route metadata keys added/changed? | N-A | None |
| 14 | Prometheus counters added/changed? | N-A | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | kernelcap capability: `docs/guide/status.md` if it lists doctor checks |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Design docs declared by files this spec changes: `docs/architecture/traffic/fw-7-traffic-vpp.md` (declared by `vpp/verify.go`; UPDATE: its rejection list gains ingress shaping and percentage rates beside the ingress-policer paragraph); `docs/architecture/hub-architecture.md` (declared by `cmd/ze/hub/service_ssh.go`, `session_factory.go`, `config/infra/hook.go`; UPDATE: the login-warning hook is replaced by the `loginnotice` registry on both SSH paths); `docs/architecture/config/syntax.md` (declared by `bgp/config/loader.go`, `loader_create.go`; unaffected: the change moves the banner registration and touches no config loading, verify at implementation); `docs/features/ai-first.md` (declared by `internal/core/diagnostic/codes.go`; unaffected unless it lists diagnostic codes, verify at implementation). Source anchors: run `./le spec citation anchors spec plan/post-release/features/spec-traffic-ingress-shaping.md` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/traffic-control.md` examples; `docs/guide/command-reference.md` show example |

## Implementation Steps

1. **Phase: Wiring** -- YANG `ingress` container and rate pattern; ingress field on `InterfaceQoS`; login-notice registry with BGP moved onto it and both SSH paths reading it; failing wiring `.ci` files and `TestLoginBannerShowsTrafficShaping`
   - Verify: the BGP banner tests stay green (AC-23); the new wiring tests fail because nothing programs ingress yet
2. **Phase: Rates** -- `tbit`/`tbps`, link-share type, resolution, `rate-unresolved` warning, `LinkUp` re-apply
   - Tests: `TestParseRateTbit`, `TestParseRateShare`, `TestParseRateBpsRefusesShare`, `TestResolveShareRefusesUnknownSpeed`, `TestResolveShareRoundsAndBounds`, `TestTrafficIngressShareFollowsLinkUp`
3. **Phase: tc ingress** -- absent-root fix, IFB lifecycle, redirect at 150, teardown, orphan scan, tc verifier, kernelcap capability, `CONFIG_IFB`
   - Tests: the backend unit tests, the netlink integration tests, `TestTrafficIngressCapabilityAbsent`
4. **Phase: VPP refusals** -- AC-12, AC-13 and their `.ci`
5. **Phase: Show** -- kernel read-back of both directions, handle-to-name mapping
6. **Phase: Notice and goal** -- traffic login-notice provider; throughput test forced red, then green. Each doc page lands in the phase whose code makes it wrong (`ai/rules/documentation.md`)

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Hook ownership | No code path calls `QdiscDel` or `QdiscReplace` on the clsact qdisc; teardown matches priority 150 AND a redirect target that is Ze's IFB |
| Ordering | IFB qdisc before redirect on apply; redirect before IFB delete on teardown |
| Zero guard | No share, speed or rate uses 0 to mean absent without a named type |
| Errors | Every refusal names the interface, why, and the fix (`ai/rules/user-facing-errors.md`) |
| No layering | `HookParams.CollectLoginWarnings` and `infra.LoginWarning` are deleted, not left beside the registry |
| Stale comments | `InterfaceQoS` doc comment, the `policer_linux.go` owner list (now four owners), the YANG `qdisc-type` description |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Ingress shaping in the kernel | `TestNetlinkIntegration_IngressShapingReachesTheKernel` under `./le test qemu` |
| Download actually capped | `TestIngressShapingLimitsDownloadThroughput` output pasted, plus its forced red |
| Banner on a no-bgp CPE | `traffic-ingress-shaping-banner.ci` |
| IFB in the appliance kernel | `grep CONFIG_IFB gokrazy/kernel/runtime.config gokrazy/kernel/runtime.require` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `tbit` overflow on the RADIUS path; `%` refused on the RADIUS path |
| Foreign state | Never delete or adopt a device without the `ze:owned:traffic` alias; never remove another owner's filter |
| Fail open | An unresolved share leaves the direction unshaped and says so on the report bus and in `show`; it never installs a zero or a guessed rate |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- VyOS's redirect lifecycle deletes the whole ingress qdisc on every update. Copying it would reopen the defect Ze closed in `fixit-mirror-clsact-ownership`.
- `show traffic control` has always reported `class-count` 0, because `ListQdiscs` reads back only the root type. The show phase fixes it as part of goal 4.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Operator surface A: `ingress { qdisc { } }` on the traffic-control entry, Ze owns the IFB (owner decision Q-1) | B: an `ifb` interface kind, a `redirect` leaf and an egress qdisc on the IFB (VyOS) | (1) `InterfaceQoS` already models both directions of one interface, with ingress as a field; A extends that model, while B splits one intent over three stanzas in two components with two journals, so a partial delete can leave a redirect to a missing IFB. (2) iface kinds are a hand-listed set read by literal key in `parseIfaceConfig`; B adds to that enumeration. (3) The banner, `show` and teardown each derive from one entry under A. B's advantage: several interfaces redirected into one IFB share one cap |
| The traffic backend creates the IFB itself, synchronously, inside `Apply` | Extend iface's owned-device registry to IFB | The registry reconciles on iface's own worker after a trigger; the traffic `Apply` must attach a qdisc to the IFB in the same call and roll back with the traffic journal. Naming and the alias convention are reused |
| Redirect at clsact priority 150 | 10 (VyOS), 200 | It must run after mirror (1) and sampling (100) because mirred redirect is terminal; 200 is the policer's |
| Refuse ingress shaping and an ingress policer together | Allow both | The policer would never see redirected packets: a configured limit that does nothing |
| Leave the clsact qdisc at teardown | Delete it when Ze created it | The journal row of 2026-08-14 decided this for the mirror: a teardown cannot know who else attached between read and delete |
| VPP refuses `ingress` | Map it to a VPP input policer | The owner decided against policing; VPP traffic never traverses a Linux IFB |
| Percentage resolved by the component, re-resolved on `LinkUp` | Resolve once at commit (VyOS); refuse when unknown | A CPE often boots with the WAN link down; refusing would leave it unshaped with an error, and resolving once goes stale after renegotiation |
| Percentage is of the shaped interface's link speed, for both directions | Of a policy bandwidth (VyOS class semantics) | Ze has no policy-level bandwidth; a single class with rate = ceil is the cap |
| Login notices through an `internal/core/loginnotice` registry | Raise a report-bus warning for shaping | Shaping is not a fault; a warning would mark a healthy CPE degraded in `show warnings` and in anything that counts warnings |
| Use `show traffic control`; no `show qos` alias | Add `show qos` | Ze keeps one name per surface; the banner names the real command |

## Known Limitations

- One IFB per shaped interface: a single download cap shared by several WAN links is not expressible: the owner chose design A over B (Q-1).
- No named shaper policy: rates are written inline once per direction, and named policies are out of scope (owner decision Q-2).
- Classes hang directly under each root as today; no hierarchical parent class caps the sum of several classes. Unchanged from today's egress model.
- A driver that changes link speed without a carrier event leaves a percentage resolved against the old speed until the next apply (A-2).

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-24 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests: N-A (no protocol peer); goal evidence is the throughput test

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/post-release/features/spec-traffic-ingress-shaping.md` only, in the same `./le commit create` script
