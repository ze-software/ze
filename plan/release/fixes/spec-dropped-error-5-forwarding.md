# Spec: dropped-error-5-forwarding

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Priority | P1 |
| Release reason | 36 errors in interface config, the DHCP client, router advertisements, the netlink monitor, the VPP monitor, sysctl, and the firewall IRR reader are caught and logged only below WARN: a DHCP lease address or route that fails to be removed stays in the kernel, a sysctl apply event is lost, each with no trace at the default level |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child of `plan/release/fixes/spec-dropped-error-0-umbrella.md`, which holds the
resolution policy and the tree-wide acceptance. Door: `ai/doors/forwarding.md`.

Population: every `error-dropped-below-warn` finding of
`./le arch refuse-by-default report` under the door's base paths
(`internal/component/{iface,sysrib,vpp,firewall,traffic,trafficfeature,sysctl,mtu}`,
`internal/plugins/{iface,fib,firewall,traffic,policyroute}`). Verified against
HEAD on 2026-11-26: **36 instances in 14 files** (DHCP client 15, sysctl 9,
iface component 6, netlink monitor and VPP monitor 4, RA sender 1, firewall IRR
stored-list reader 1). None of the original 35 instances from 2026-11-02 was
retired; one new instance in `internal/component/firewall/plugins/irr/stored.go`
(`storeReader.notice`, line 112) entered the door's paths.

Each instance is resolved by exactly one answer, judged in this order
(umbrella, Key Design Decisions):

| Order | When | Answer |
|-------|------|--------|
| 1 | Going on is wrong: the caller would act on the failure | Return the error, or fix the behavior the drop hid. A kernel address or route Ze installed and failed to remove is state Ze still owns; the failure reaches the code that tracks that ownership |
| 2 | Going on is right and an operator can act | WARN with what, why and the fix (`ai/rules/user-facing-errors.md`) naming the interface and the kernel object, carrying a registered doctor `code` attribute, AND raising that event on the report bus (`internal/core/report` for `iface` and its in-process backends; SDK `RaiseIssue` via `internal/component/plugin/issuequeue` for forkable `sysctl`) so `./le arch doctor-coverage check` and `recent-events` both hold |
| 3 | Going on is right and nobody can act (closing a socket at shutdown, or a recovery branch that already schedules a full re-read) | `// refuse-by-default: guards <Symbol> -- <why>` on the line above |

Every new `Warn` or `Error` log line added by this spec MUST carry a `code`
attribute whose doctor code is registered (`diagnostic.Register` plus
`diagnostic.RegisterEventCode` or a doctor check's `Codes` list) in the owning
package's doctor-codes registration file (`doctor_codes.go` or
`register_doctor_codes.go`), satisfying the `doctor-coverage` commit gate
(`docs/architecture/doctor-and-health-checks.md`, "Every problem site names a
doctor code"). Every site carrying a code registered via
`diagnostic.RegisterEventCode` MUST also raise that event on the report bus so
`recent-events` reports it (never log-only): `iface` and its in-process backends
(`ifacenetlink`, `ifacevpp`, `ifacedhcp`, `ifacera`) run inside the `interface`
process and raise through `internal/core/report`, whereas `sysctl` is a forkable
plugin (`CLIHandler` + `sdk.NewWithConn` with no `!p.IsInternal()` guard) and
raises through SDK `RaiseIssue` via `internal/component/plugin/issuequeue`.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`
  → Constraint: every message a user sees names the problem, why, and the exact fix; plugins may run in another process, so a forkable plugin never touches daemon-local registries directly
- [ ] `ai/doors/forwarding.md`
  → Constraint: Ze owns only what it marked; a failed removal of a DHCP-installed address or route leaves Ze-owned state in the kernel, so the failure must reach whatever reconciles ownership
  → Constraint: the report bus (`internal/core/report`) is process-local; `sysctl` is a forkable plugin, so it raises issues through SDK `RaiseIssue` via `internal/component/plugin/issuequeue` rather than importing `internal/core/report` directly; `iface` and its in-process backends (`ifacenetlink`, `ifacevpp`, `ifacedhcp`, `ifacera`) run inside the `interface` process and use `internal/core/report` directly
  → Constraint: kernel tunables go through `sysctl`; its event emits are how plugins learn a value was applied
- [ ] `docs/architecture/core-design.md`, sections 14 and 15 - backend split and the report bus
  → Constraint: a backend that cannot apply exactly refuses at verify; a runtime error is defence in depth, not the gate; report-bus warnings are state-based and errors are event-based
- [ ] `docs/architecture/doctor-and-health-checks.md` - "Every problem site names a doctor code" and "Recent events: the report bus in show doctor"
  → Constraint: every new `Warn` or `Error` log line MUST carry a `code` attribute naming a doctor code declared by a registered doctor check or `diagnostic.RegisterEventCode` with `diagnostic.Register` metadata; every site carrying a `RegisterEventCode` code MUST raise that event on the report bus (never log-only); `./le arch doctor-coverage check` and `./le commit create` refuse uncovered additions
- [ ] `docs/architecture/plugin/plugin-system.md` - "The process boundary" and "Proximity"
  → Constraint: forkable plugins reach the daemon through SDK RPCs (`RaiseIssue` via `issuequeue`), never daemon-local globals; doctor code registrations live in each owning package
- [ ] `docs/architecture/firewall/firewall-irr.md` and `docs/architecture/irr-owner.md`
  → Constraint: in `storeReader.notice`, a dataset notice that fails `store.DecodeDatasetNotice` already sets `r.all = true` so the next worker pass reads every stored list from the store and recovers
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: what, why, fix, a covered doctor `code` attribute on every `Warn` or `Error` log line, and a report-bus raise for every transient event code
- [ ] `docs/contributing/ze-go-style.md` - "Error dropped below WARN"
  → Constraint: return, count, or WARN; the annotation names a guard

**Key insights:**
- `runSysctlPlugin` drops the EventBus emit error after a successful apply (five `EventApplied` sites) and after three CLI query responses (`EventShowResult`, `EventListResult`, `EventDescribeResult`): the value is applied or queried but its subscribers or callers never hear it, which is answer 2 at least. Because `sysctl` is forkable (`CLIHandler` + `sdk.NewWithConn`), its raises go through `issuequeue.Queue` -> SDK `RaiseIssue`, never `internal/core/report`.
- The DHCPv4 (`removeV4Route`, `removeV4Address`) and DHCPv6 (`drop`, `shutdown`) paths remove Ze-owned kernel state; a non-ENOENT failure leaves Ze-owned state in the kernel and must distinguish "already gone" from a real removal failure.
- `storeReader.notice` in `internal/component/firewall/plugins/irr/stored.go` already sets `r.all = true` before logging at Debug, so the next worker pass reads every stored list from the store; it is resolved with answer 3 naming the recovery guard (or answer 2 under the already-declared `codeIRRStoredCopyBehind`).
- Every package that gains a new `Warn` or `Error` log line registers its doctor code in that package's `doctor_codes.go` or `register_doctor_codes.go` and raises every `RegisterEventCode` site on the report bus (`internal/plugins/iface/netlink/monitor_report_linux.go` is the in-process precedent).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/arch/refusebydefault/dropped.go` - `droppedError`, `caughtError`, `belowWarn`, `drops`: defines the `error-dropped-below-warn` AST rule
- [ ] `internal/component/firewall/plugins/irr/stored.go` - `storeReader.notice` (L112) sets `r.all = true` on `DecodeDatasetNotice` error and logs at Debug
- [ ] `internal/component/iface/config_apply.go` - `addDesiredAddresses` (L2388) logs `b.AddAddress` errors at Debug
- [ ] `internal/component/iface/config_sysctl.go` - `applySysctl` (L30) and `applySysctlProfiles` (L161, L178) log EventBus sysctl emit errors at Debug
- [ ] `internal/component/iface/register.go` - `runEngine` rollback callback (L836) logs `EventRollback` emit error at Debug; `cleanupStaleIPv6DefaultRoutes` (L1864) logs `ListRoutes` error at Debug
- [ ] `internal/component/sysctl/register.go` - `runSysctlPlugin` (L185, L206, L222, L237, L275, L294, L359, L429) logs EventBus `applied` and query-result emit errors at Debug while write failures beside them already log at WARN
- [ ] `internal/component/sysctl/sysctl.go` - `store.saveOriginal` (L188) logs `/proc/sys` read failure before first write at Debug
- [ ] `internal/plugins/iface/dhcp/dhcp_linux.go` - `DHCPClient.publishDHCP` (L264, L268) logs marshal and emit failures at Debug
- [ ] `internal/plugins/iface/dhcp/dhcp_v4_exchange_linux.go` - `closeV4Socket` (L215) and `closeV4Client` (L230) log close failures at Debug
- [ ] `internal/plugins/iface/dhcp/dhcp_v4_linux.go` - `removeV4Route` (L640) and `removeV4Address` (L699) log netlink removal errors at Debug
- [ ] `internal/plugins/iface/dhcp/dhcp_v6_attach_linux.go` - `dropAddresses` (L338) and `decline` (L581) log `registerLeased` errors at Debug
- [ ] `internal/plugins/iface/dhcp/dhcp_v6_linux.go` - `closeV6Link` (L219), `drop` (L592), `expireDue` (L609), and `shutdown` (L628, L635, L651, L656) log close, address removal, lease registration, Release build, and socket open errors at Debug
- [ ] `internal/plugins/iface/netlink/monitor_linux.go` - `monitor.emit` (L572, L576) logs marshal and EventBus emit errors at Debug
- [ ] `internal/plugins/iface/ra/sender_linux.go` - `Sender.run` send closure (L294) logs `pc.WriteTo` errors at Debug
- [ ] `internal/plugins/iface/vpp/monitor.go` - `monitor.emit` (L456, L461) logs marshal and EventBus emit errors at Debug
- [ ] `internal/component/iface/doctor_codes.go`, `internal/component/sysctl/register_doctor_codes.go`, `internal/plugins/iface/netlink/register_doctor_codes.go`, `internal/plugins/iface/netlink/monitor_report_linux.go`, `internal/plugins/iface/netlink/register_monitor_linux.go`, `internal/plugins/iface/ra/register_doctor_codes.go`, `internal/plugins/iface/vpp/register_doctor_codes.go`, `internal/component/firewall/plugins/irr/doctor.go`, `internal/component/plugin/issuequeue/queue.go`, `internal/component/plugin/issuequeue/raises.go` - existing doctor code registrations, in-process report-bus precedent, and forkable-plugin report queue

**Behavior to preserve:**
- What Ze installs in the kernel or VPP, and ownership rules (only Ze-owned routes/addresses are removed; "already gone" on removal is treated as success; non-fatal runtime failures never cause `ze start` to exit).
- Existing EventBus payload formats, RPC contracts, and plugin process-boundary isolation (forkable `sysctl` never writes to daemon-local `internal/core/report`).

**Behavior to change:**
- Each of the 36 listed instances across 14 files is resolved by exactly one of the umbrella's three answers (return/propagate, WARN with registered doctor code + mandatory report-bus raise + operator fix, or guard annotation).

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| refuse-by-default dropped-error shape | `internal/le/arch/refusebydefault/dropped.go` `droppedError` | Lists these 36 instances across 14 files under the forwarding door's base paths (zero of the original 35 were retired at HEAD; 1 new site appeared in `firewall/plugins/irr/stored.go`) | Applies: the primary acceptance check (AC-1) |
| Guard annotation | `internal/le/arch/refusebydefault/annotation.go` | Accepts `// refuse-by-default: guards <Symbol> -- <why>` naming a visible guard symbol | Applies: answer 3 only (teardown socket closes and recovery guards) |
| Doctor-code coverage gate and event-code registry | `internal/core/diagnostic/event_codes.go` `RegisterEventCode`; `internal/component/iface/doctor_codes.go`, `internal/component/sysctl/register_doctor_codes.go`, `internal/plugins/iface/{netlink,ra,vpp}/register_doctor_codes.go`; `./le arch doctor-coverage check` | Requires every new `Warn`/`Error` log line to carry a `code` attribute whose doctor code is registered via `diagnostic.Register` and declared by a doctor check or `diagnostic.RegisterEventCode`; every site carrying a `RegisterEventCode` code MUST raise on the report bus | Extends: every package gaining a new `Warn` log registers its code metadata and event code in its owning doctor-codes file (creating `internal/plugins/iface/dhcp/register_doctor_codes.go` for DHCP) and raises every `RegisterEventCode` site on the report bus |
| Operator warnings and error events bus (in-process) | `internal/core/report` (`show warnings`, `show errors`, `docs/architecture/core-design.md` section 15); precedent in `internal/plugins/iface/netlink/monitor_report_linux.go` (`reportAddressUnattributed`) and `internal/plugins/iface/netlink/register_monitor_linux.go` (`diagnostic.RegisterEventCode`) | Process-local report bus rendered by `recent-events` in `show doctor`; `monitor_report_linux.go` logs at WARN with `code` and raises via `report.RaiseErrorFix`, with its event code registered in `init()` | Extends: `iface` and its in-process backends (`ifacenetlink`, `ifacevpp`, `ifacedhcp`, `ifacera`) all run inside the `interface` process and follow this same warn + `internal/core/report` raise + `RegisterEventCode` pattern |
| Forkable-plugin report queue | `internal/component/plugin/issuequeue` (`Queue`, `RunRaises`); SDK `RaiseIssue` (`pkg/plugin/sdk/report.go`) | Carries a forkable plugin's report-bus raises and clears to the daemon over the `report-issue` RPC on a bounded worker queue without blocking event handlers or touching a subprocess-local `internal/core/report` | Extends: `sysctl` (`internal/component/sysctl/register.go` has `CLIHandler` + `sdk.NewWithConn` and no `!p.IsInternal()` guard, so it is forkable) uses `issuequeue.Queue` (and `RunRaises` where repeating) to raise its new `RegisterEventCode` events to the daemon's report bus |
| Interface and RA Prometheus metrics | `internal/component/iface/rate.go` (`ifaceMetrics`), `internal/plugins/iface/ra/ifacera.go` (`raMetrics`); `docs/features/interfaces.md` | Only `internal/component/iface/rate.go` and `internal/plugins/iface/ra/ifacera.go` currently register Prometheus counters; `internal/plugins/iface/dhcp`, `internal/plugins/iface/netlink`, and `internal/plugins/iface/vpp` currently register no Prometheus counters | Extends: `ifacera` can extend `raMetrics` for RA send failures; repeating monitor paths (`ifacenetlink`, `ifacevpp`) rate-limit WARN + report-bus raises once per interface/condition (and document any newly added counter in `docs/features/interfaces.md`) |
| Ownership reconcile | `internal/plugins/fib/kernel/monitor.go` `handleExternalChange`; `internal/component/iface/address_owner.go` `RegisterLeasedAddresses` | Reasserts what Ze owns and keeps leased addresses registered while held | Applies: the consumer of failed DHCP removal and lease-registration errors under answer 1 |
| Firewall IRR full-reread recovery | `internal/component/firewall/plugins/irr/stored.go` `storeReader.notice`, `r.all` | Sets `r.all = true` when a dataset notice cannot be decoded so the next worker pass reads every stored list from the store | Applies: the existing guard for `stored.go` L112 under answer 3 (or answer 2 reusing `codeIRRStoredCopyBehind` already declared in `doctor.go`) |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "Every message a user sees names the problem, why, and the exact fix" | agrees | Every answer-2 WARN added by this spec names what failed, why it matters, and the exact fix, and respects process boundaries for forkable plugins |
| `ai/doors/forwarding.md` | Boundaries | the door covering all 14 source files and their doctor-code files | "A plugin raises or clears issues through SDK `RaiseIssue` or `ClearIssue`, whose `report-issue` RPC reaches the daemon's store" | agrees | Forkable `sysctl` raises its new events through SDK `RaiseIssue` via `internal/component/plugin/issuequeue` rather than importing `internal/core/report` directly; `iface` and its in-process backends (`ifacenetlink`, `ifacevpp`, `ifacedhcp`, `ifacera`) run inside the `interface` process and use `internal/core/report`; kernel tunables stay routed through `sysctl`, and Ze-owned DHCP/interface cleanup failures are no longer dropped |
| `docs/features/interfaces.md` | Key Design Decisions | the `// Design:` page of `config_apply.go`, `config_sysctl.go`, `register.go`, `dhcp_*.go`, `netlink/monitor_linux.go`, `ra/sender_linux.go` | "The config owns its addresses, not every address on the unit." | agrees | Preserves address ownership and leased-address registration rules while surfacing failed kernel removals and lease registration failures instead of hiding them at Debug |
| `docs/features/interfaces.md` | Design decisions | the `// Design:` page of `config_apply.go`, `config_sysctl.go`, `register.go`, `dhcp_*.go`, `netlink/monitor_linux.go`, `ra/sender_linux.go` | "`ze init` claims every discovered device by default, including the ones another program owns. `ze init --confirm` asks about each discovered device and writes only the ones the operator confirms" | agrees | This spec does not alter `ze init` discovery or `--confirm` behavior |
| `docs/features/interfaces.md` | Design decisions | the `// Design:` page of `config_apply.go`, `config_sysctl.go`, `register.go`, `dhcp_*.go`, `netlink/monitor_linux.go`, `ra/sender_linux.go` | "`ze init --confirm` takes only an explicit answer: `yes`, `y`, `no` or `n`. Anything else, an empty line included, is not accepted, and the same device is asked again" | agrees | This spec does not alter `ze init --confirm` prompting or input acceptance |
| `docs/architecture/core-design.md` | 15. Operational Report Bus | anchored to `config_apply.go`, `config_sysctl.go`, `register.go`, `sysctl/register.go`, `sysctl/sysctl.go`, `netlink/monitor_linux.go`; `// Design:` of `sysctl/sysctl.go` | "Warnings are state-based (condition is currently problematic, may resolve). Errors are event-based (something already happened; no clear API)." | agrees | Answer-2 transient event raises use event-based error raises and lasting conditions use state-based warnings on the existing report bus, with no new reporting path |
| `docs/architecture/core-design.md` | Design decisions | anchored to `config_apply.go`, `config_sysctl.go`, `register.go`, `sysctl/register.go`, `sysctl/sysctl.go`, `netlink/monitor_linux.go` | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | This spec does not alter VPP/kernel backend validation or commit refusal rules |
| `docs/architecture/core-design.md` | Design decisions | anchored to `config_apply.go`, `config_sysctl.go`, `register.go`, `sysctl/register.go`, `sysctl/sysctl.go`, `netlink/monitor_linux.go` | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | This spec does not alter mismatch validation verdicts (as narrowed for start by the 2026-11-26 row below) |
| `docs/architecture/core-design.md` | Design decisions | anchored to `config_apply.go`, `config_sysctl.go`, `register.go`, `sysctl/register.go`, `sysctl/sysctl.go`, `netlink/monitor_linux.go` | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | This spec touches no VPP firewall verifier or YANG `ze:backend` annotation |
| `docs/architecture/core-design.md` | Design decisions | anchored to `config_apply.go`, `config_sysctl.go`, `register.go`, `sysctl/register.go`, `sysctl/sysctl.go`, `netlink/monitor_linux.go` | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | This spec changes no data-plane selector name or `ze:backend` value |
| `docs/architecture/core-design.md` | Design decisions | anchored to `config_apply.go`, `config_sysctl.go`, `register.go`, `sysctl/register.go`, `sysctl/sysctl.go`, `netlink/monitor_linux.go` | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | This spec touches no BGP export filter or next-hop rewriting code |
| `docs/architecture/core-design.md` | Design decisions | anchored to `config_apply.go`, `config_sysctl.go`, `register.go`, `sysctl/register.go`, `sysctl/sysctl.go`, `netlink/monitor_linux.go` | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | This spec touches no BGP policy modify or next-hop chain reference code |
| `docs/architecture/core-design.md` | Design decisions | anchored to `config_apply.go`, `config_sysctl.go`, `register.go`, `sysctl/register.go`, `sysctl/sysctl.go`, `netlink/monitor_linux.go` | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | This spec touches no BGP policy chain compilation or FilterInsertion code |
| `docs/architecture/core-design.md` | Design decisions | anchored to `config_apply.go`, `config_sysctl.go`, `register.go`, `sysctl/register.go`, `sysctl/sysctl.go`, `netlink/monitor_linux.go` | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | This spec touches no BGP export chain evaluation or wireOverride provenance code |
| `docs/architecture/core-design.md` | Design decisions | anchored to `config_apply.go`, `config_sysctl.go`, `register.go`, `sysctl/register.go`, `sysctl/sysctl.go`, `netlink/monitor_linux.go` | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | This spec touches no BGP next-hop attribute or import chain validation code |
| `docs/architecture/core-design.md` | Design decisions | anchored to `config_apply.go`, `config_sysctl.go`, `register.go`, `sysctl/register.go`, `sysctl/sysctl.go`, `netlink/monitor_linux.go` | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | This spec touches no BGP forwarding or locally originated route next-hop code |
| `docs/architecture/core-design.md` | Design decisions | anchored to `config_apply.go`, `config_sysctl.go`, `register.go`, `sysctl/register.go`, `sysctl/sysctl.go`, `netlink/monitor_linux.go` | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | This spec touches no BGP filter chain or `\| implied` display code |
| `docs/architecture/core-design.md` | Design decisions | anchored to `config_apply.go`, `config_sysctl.go`, `register.go`, `sysctl/register.go`, `sysctl/sysctl.go`, `netlink/monitor_linux.go` | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | This spec never turns a non-fatal runtime or startup error into a `ze start` exit, and pairs every new `Warn`/`Error` log line with a registered doctor code and fix |
| `docs/architecture/core-design.md` | Design decisions | anchored to `config_apply.go`, `config_sysctl.go`, `register.go`, `sysctl/register.go`, `sysctl/sysctl.go`, `netlink/monitor_linux.go` | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | This spec does not alter daemon startup or VPP/kernel mismatch handling |
| `docs/architecture/doctor-and-health-checks.md` | Every problem site names a doctor code | the `// Design:` page of `iface/doctor_codes.go`, `sysctl/register_doctor_codes.go`, `netlink/register_doctor_codes.go`, `ra/register_doctor_codes.go`, `vpp/register_doctor_codes.go`, `dhcp/register_doctor_codes.go` | "A site is covered when its code is one that a doctor check declares." | agrees | Every new `Warn`/`Error` log line carries a `code` attribute registered via `diagnostic.Register` and declared by a doctor check or `diagnostic.RegisterEventCode` in its owning package, AND every site carrying a `RegisterEventCode` code raises it on the report bus (`internal/core/report` in `iface` and its in-process backends; `issuequeue`/SDK `RaiseIssue` in forkable `sysctl`) |
| `docs/architecture/doctor-and-health-checks.md` | Recent events: the report bus in show doctor | the `// Design:` page of `iface/doctor_codes.go`, `sysctl/register_doctor_codes.go`, `netlink/register_doctor_codes.go`, `ra/register_doctor_codes.go`, `vpp/register_doctor_codes.go`, `dhcp/register_doctor_codes.go` | "Registering a code whose sites only log, and never raise, would declare a row this check never shows: every site carrying a registered event code raises it." | agrees | Every answer-2 site carrying a code registered via `diagnostic.RegisterEventCode` raises that event on the report bus (`internal/core/report` for `iface` and its in-process backends; `internal/component/plugin/issuequeue` + SDK `RaiseIssue` for forkable `sysctl`), never log-only |
| `docs/architecture/firewall/firewall-irr.md` | The irr plugin fetches and stores, this plugin reads | the `// Design:` page of `internal/component/firewall/plugins/irr/stored.go` | "A notice whose generation skips one the plugin has not applied means a notice was lost, so the pass reads every reference again." | agrees | `storeReader.notice` already sets `r.all = true` when a dataset notice fails to decode so the next worker pass reads every stored list; resolving line 112 preserves that recovery behavior |
| `docs/architecture/irr-owner.md` | What a consumer reads | anchored to `internal/component/firewall/plugins/irr/stored.go` | "A notice whose generation skips one the consumer has not applied means a notice was lost: read every pointer again." | agrees | Preserves the consumer recovery contract in `firewall/plugins/irr/stored.go` without altering stored key formats, read order, or notice decoding |
| `docs/architecture/iface/logical-name-resolution.md` | Event reality differs from the older prose | anchored to `config_apply.go`, `register.go`, `netlink/monitor_linux.go` | "The payload delivered to an in-process subscriber is a JSON string: the monitor marshals the payload and emits the string, so a `Subscribe` handler unmarshals it." | agrees | Preserves the monitor JSON string event contract while surfacing marshal and emit failures in `monitor.emit` at WARN with a doctor code instead of hiding them at Debug |
| `docs/architecture/iface/management.md` | Sysctl writes go through an overridable root | anchored to `config_sysctl.go`, `register.go`, `ra/sender_linux.go` | - | silent | This page covers YANG interface layers, name validation, sysctl root overrides, partial link cleanup, SLAAC address origin, and link-event queueing; it states no requirement on log levels or doctor codes for dropped sysctl, rollback, or RA send errors |
| `docs/architecture/appliance/kernel-profiles.md` | Go owns the guarantee | anchored to `config_apply.go`, `netlink/monitor_linux.go` | - | silent | This page governs kernel build profiles, digest pinning, and the PPP original-resource kernel ABI; it says nothing about userspace dropped-error handling in `config_apply.go` or `monitor_linux.go` |
| `docs/architecture/iface/offload.md` | Three kernel interfaces behind one container | anchored to `config_apply.go` | - | silent | This page covers ethtool offload and RPS/RFS settings; `addDesiredAddresses` in `config_apply.go` does not touch offload or packet steering |
| `docs/architecture/plugin/plugin-system.md` | The process boundary | anchored to `config_apply.go`, `register.go` | "Plugin code reaches the engine through the SDK's RPC layer, which hides the transport difference." | agrees | Forkable `sysctl` raises its new events to the daemon's report bus through SDK `RaiseIssue` via `internal/component/plugin/issuequeue` rather than touching daemon-local `internal/core/report`, so internal and forked runs behave identically |
| `docs/architecture/plugin/plugin-system.md` | Proximity | anchored to `config_apply.go`, `register.go` | "A doctor check follows the same rule. The check, its registration and its unit test live in the plugin, component, backend or command package that owns the dependency." | agrees | Every package gaining a new `Warn` log registers its doctor code metadata and event code in its own `doctor_codes.go` or `register_doctor_codes.go` file (`iface`, `sysctl`, `dhcp`, `netlink`, `ra`, `vpp`) |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | anchored to `config_apply.go`, `register.go` | "With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration" | agrees | This spec does not alter offline config parsing or external plugin query mode |
| `docs/architecture/traffic/cos-plugin.md` | The interface resolves against the candidate's profiles | anchored to `config_apply.go`, `register.go` | - | silent | This page covers class-of-service VLAN QoS profile resolution; none of the dropped-error sites in `config_apply.go` or `register.go` touches CoS profiles or VLAN QoS maps |
| `docs/architecture/config/apply-ordering.md` | The design | anchored to `register.go`, `vpp/monitor.go` | - | silent | This page specifies the five-phase binder/provider operation-graph ordering during config apply; this spec changes only error visibility and cleanup ownership reporting without changing apply ordering |
| `docs/architecture/config/transaction-protocol.md` | 1. Design Principles | anchored to `register.go` | "The engine emits a single rollback event when any plugin's apply fails or times out. All plugins that already applied undo via their journals." | agrees | Surfacing a failed `EventRollback` emit in `internal/component/iface/register.go` at WARN with a doctor code makes rollback notification failures visible instead of silent |
| `docs/architecture/l2tp/cpe-1-pppoe-client.md` | Decisions | anchored to `register.go` | - | silent | This page covers the PPPoE client dialer and LCP/IPCP/IPv6CP negotiation; neither flagged site in `internal/component/iface/register.go` touches PPPoE client code |
| `docs/architecture/plugin/plugin-relationships.md` | 1. System Plugins | anchored to `register.go` | - | silent | This page inventories plugin config roots, dependencies, and bus topics; this spec changes no dependency, config root, or bus topic |
| `docs/architecture/api/process-protocol.md` | Event Delivery | anchored to `netlink/monitor_linux.go`, `vpp/monitor.go` | "Every participant registers the messages it sends and receives at compile time" | agrees | Surfacing `monitor.emit` marshal and emit failures at WARN with doctor codes preserves existing registered message types and makes delivery failures observable |
| `docs/architecture/api/process-protocol.md` | Design decisions | anchored to `netlink/monitor_linux.go`, `vpp/monitor.go` | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | This spec adds no engine operation and changes no plugin authorization profile |
| `docs/architecture/api/process-protocol.md` | Design decisions | anchored to `netlink/monitor_linux.go`, `vpp/monitor.go` | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | This spec never introduces lossy drop-on-full delivery; it surfaces emit failures at WARN so no failed delivery goes unseen |
| `docs/architecture/rsvpte/mpls-rsvp-te.md` | Decision: link failure comes from the interface component | anchored to `netlink/monitor_linux.go` | "RSVP-TE loads the interface component and consumes its link-down notifications." | agrees | Surfacing `monitor.emit` failures in `netlink/monitor_linux.go` at WARN ensures lost link notifications are reported to operators rather than hidden at Debug |
| `docs/research/vpp-deployment-reference.md` | GoVPP Connection | the `// Design:` page of `internal/plugins/iface/vpp/monitor.go` | - | silent | This page covers VPP deployment, startup.conf, LCP, and GoVPP connection/supervision; it says nothing about VPP monitor EventBus marshal/emit error logging |
| `docs/research/vpp-deployment-reference.md` | Design decisions | the `// Design:` page of `internal/plugins/iface/vpp/monitor.go` | "Ze refuses to run VPP unmanaged. When VPP is external, the operator accepts that Ze may terminate and restart it" | agrees | `vpp/monitor.go` changes only `monitor.emit` error reporting and does not change VPP management policy |
| `docs/research/vpp-deployment-reference.md` | Design decisions | the `// Design:` page of `internal/plugins/iface/vpp/monitor.go` | "Ze knows how to manage and restart an external VPP: through a systemd unit, or by starting the binary directly, as it does with the embedded runtime" | agrees | This spec does not alter VPP restart mechanisms |
| `docs/research/vpp-deployment-reference.md` | Design decisions | the `// Design:` page of `internal/plugins/iface/vpp/monitor.go` | "After a failure, when the VPP process is alive, forwarding continued, and Ze reconnects and can read the state it configured, Ze continues from where it is" | agrees | This spec does not alter VPP reconnect or adoption behavior |
| `docs/research/vpp-deployment-reference.md` | Design decisions | the `// Design:` page of `internal/plugins/iface/vpp/monitor.go` | "When VPP could have crashed or stopped forwarding, Ze reflects it in its own model and makes sure IGP and BGP peers are updated" | agrees | This spec does not alter VPP crash detection or model updates |
| `docs/research/vpp-deployment-reference.md` | Design decisions | the `// Design:` page of `internal/plugins/iface/vpp/monitor.go` | "One timer: the alive test and the loss declaration use the same time, and a probe missed inside it is not a loss. This narrows the row "After a failure, when the VPP process is alive": Ze continues from where it is only through form (1) below, inside the timer, and a declared loss is never adopted" | agrees | This spec does not alter VPP supervision timers (as narrowed by the "Two limits" row below) |
| `docs/research/vpp-deployment-reference.md` | Design decisions | the `// Design:` page of `internal/plugins/iface/vpp/monitor.go` | "As soon as a loss is declared, Ze makes sure the process is terminal (kill through the retained pidfd, then wait for the exit proof), then restarts and reconfigures VPP. Ze is the only restarter: a systemd unit it drives does not restart VPP itself (Restart=no)" | agrees | This spec does not alter VPP termination or restart supervision |
| `docs/research/vpp-deployment-reference.md` | Design decisions | the `// Design:` page of `internal/plugins/iface/vpp/monitor.go` | ""Recovered" has exactly two forms. (1) Inside the single timer Ze reconnects to the same pidfd-proven process and a full readback of what it installed (interfaces, LCP pairs, owned routes, PPPoE sessions) matches: Ze continues, with no kill and no withdrawal. Any mismatch means form (1) failed. (2) A forced restart (kill through the retained pidfd, exit proof) followed by a correct reconfiguration, exactly as at boot. Routes are withdrawn only when neither form succeeds within the recovery deadline and the crash-loop limit" | agrees | This spec does not alter VPP recovery forms or route withdrawal conditions |
| `docs/research/vpp-deployment-reference.md` | Design decisions | the `// Design:` page of `internal/plugins/iface/vpp/monitor.go` | "Blackholing during the restart is accepted. Routes are withdrawn (VPP-backed interfaces down in Ze's model, so IGP and BGP react) only when neither recovery form succeeds within the recovery deadline and the crash-loop limit. This narrows the row "When VPP could have crashed or stopped forwarding": Ze updates IGP and BGP peers only then" | agrees | This spec does not alter VPP restart blackholing or withdrawal timing |
| `docs/research/vpp-deployment-reference.md` | Design decisions | the `// Design:` page of `internal/plugins/iface/vpp/monitor.go` | "systemd configuration: Ze cannot be sure that an OS update will not change the unit's settings, so it detects the settings and adapts. No unit policy is required: Ze reads the unit's effective settings each time it acts on VPP, and adapts its restart to them. This supersedes the second sentence of the row "As soon as a loss is declared" (Ze the only restarter, Restart=no) and its forbids clause "or leaving the restart to systemd or another supervisor"; correcting that row needs Thomas's decision-change" | agrees | This spec does not alter systemd unit detection or restart adaptation |
| `docs/research/vpp-deployment-reference.md` | Design decisions | the `// Design:` page of `internal/plugins/iface/vpp/monitor.go` | "Two limits. The alive test and the loss declaration use the same time: a probe missed inside it is not a loss, and a failed alive test is the loss declaration. A second, separate limit then bounds the reconnect-and-verify attempt of form (1). When that limit expires without form (1), Ze makes the process terminal (kill through the retained pidfd, then wait for the exit proof), then restarts and reconfigures VPP. This supersedes the row "One timer" (its single timer, "a declared loss is never adopted" and its forbids clause "a second window after the declaration"), and narrows the row "As soon as a loss is declared": the kill follows the second limit, not the declaration" | agrees | This spec does not alter VPP alive-test or reconnect-and-verify timers |
| `docs/research/vpp-deployment-reference.md` | Design decisions | the `// Design:` page of `internal/plugins/iface/vpp/monitor.go` | "Form (1) adopts on the presence of data ("We should take the presence of data as a sign that the software is still running and reconciliation may be then happening in the background."). Inside the reconnect-and-verify limit, the same pidfd-proven process answering over the API with the data Ze configured is the sign it is alive: Ze adopts it, with no kill and no withdrawal, and reconciles in the background, as the fib-vpp replay and sweep and the iface reconcile do. A mismatch the background reconciliation finds is repaired, not treated as a loss. Form (2) stays a forced restart followed by a correct reconfiguration, exactly as at boot. This narrows the row "Recovered has exactly two forms": a full readback match is no longer required, and a mismatch no longer fails form (1)" | agrees | This spec does not alter VPP form (1) adoption or background reconciliation |
| `docs/research/vpp-deployment-reference.md` | Design decisions | the `// Design:` page of `internal/plugins/iface/vpp/monitor.go` | "A recovery deadline and a crash-loop limit (N restarts inside a window) bound recovery. Past either, Ze withdraws: VPP-backed interfaces go down in Ze's model, so IGP and BGP react, and ze doctor reports it. Blackholing while Ze restarts inside both limits is accepted. The default values are proposed at design review for Thomas's approval. This adopts the recovery deadline and the crash-loop limit that the rows "Recovered has exactly two forms" and "Blackholing during the restart is accepted" carried as proposals, and narrows the row "When VPP could have crashed or stopped forwarding": Ze updates IGP and BGP peers only past a limit" | agrees | This spec does not alter VPP recovery deadlines or crash-loop limits |
| `docs/research/vpp-deployment-reference.md` | Design decisions | the `// Design:` page of `internal/plugins/iface/vpp/monitor.go` | "After a withdrawal Ze keeps retrying recovery with a growing backoff. When VPP recovers, the VPP-backed interfaces come up in Ze's model and the routes return by themselves, and ze doctor reports the state throughout. This narrows the crash-loop limit: past it Ze withdraws and keeps restarting" | agrees | This spec does not alter VPP post-withdrawal recovery backoff |
| `docs/research/vpp-deployment-reference.md` | Design decisions | the `// Design:` page of `internal/plugins/iface/vpp/monitor.go` | "The VPP loss itself triggers no withdrawal: no VPP-backed interface goes down in Ze's model while Ze recovers. Ze's own protocol timers (BFD detection, OSPF and IS-IS dead intervals, BGP hold) keep running during recovery, as their RFCs say" | agrees | This spec does not alter VPP loss handling or protocol timers |
| `docs/research/vpp-deployment-reference.md` | Design decisions | the `// Design:` page of `internal/plugins/iface/vpp/monitor.go` | "A process on VPP's API socket whose pidfd does not resolve to the configured systemd unit is reported, never refused or killed ("We should report but not refuse if the customers have changed the software, for example. It may mean that we have an out of date configuration. We should make sure the customer updates it but not cause a routing outage."). ze doctor names the process, the configured unit, that the configuration may be out of date, and the exact fix, and Ze keeps working with that process" | agrees | This spec does not alter VPP pidfd/systemd unit mismatch reporting |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A failed netlink call, VPP call, `/proc` read, RA socket write, dataset-notice decode, or EventBus marshal/emit inside the forwarding components and plugins.

### Transformation Path
1. The branch catches the error.
2. Under answer 1, it returns the error to the owner of the kernel state (distinguishing "already gone" ENOENT from a real failure).
3. Under answer 2, it logs at WARN with what, why, the fix, and a registered doctor `code` attribute (rate-limited once per interface/condition on repeating paths, and counted where metrics apply), AND for every site carrying a `diagnostic.RegisterEventCode` code it raises that event on the report bus: through `internal/core/report` in `iface` and its in-process backends (`ifacenetlink`, `ifacevpp`, `ifacedhcp`, `ifacera`), and through SDK `RaiseIssue` via `internal/component/plugin/issuequeue` in forkable `sysctl`.
4. Under answer 3, it retains the debug line with a `// refuse-by-default: guards <Symbol> -- <why>` annotation naming the visible guard symbol.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze ↔ kernel or VPP | netlink sockets, GoVPP, `/proc/sys` | Yes |
| Component ↔ subscribers | EventBus emit | Yes |
| Forkable `sysctl` ↔ daemon report bus | SDK `RaiseIssue` (`report-issue` RPC) via `internal/component/plugin/issuequeue` | Yes |
| Ze ↔ operator | Log at WARN with registered doctor `code`, `show warnings`/`show errors`, `show doctor` (`recent-events`) | Yes |

### Changed-Wire Census

No wire format, RPC payload, EventBus schema, or persisted state key changes: every edit in this spec changes only Go error returns, structured log levels/attributes, doctor diagnostic code registrations in `diagnostic.Register`/`diagnostic.RegisterEventCode`, report-bus raises over the existing `internal/core/report` API and existing SDK `RaiseIssue` (`report-issue` RPC via `internal/component/plugin/issuequeue`), and `// refuse-by-default:` source comments on existing internal control-plane paths verified in the 14 files listed under `## Files to Modify`.

### Integration Points
- Subsystem `slogutil.Logger` instances, each owning package's `diagnostic.Register` / `diagnostic.RegisterEventCode` registration (`doctor_codes.go` / `register_doctor_codes.go`), the report bus (`internal/core/report` for `iface` and its in-process backends `ifacenetlink`, `ifacevpp`, `ifacedhcp`, `ifacera`; `internal/component/plugin/issuequeue` + SDK `RaiseIssue` for forkable `sysctl`), and ownership reconcile (`RegisterLeasedAddresses`).

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Errors flow through existing return paths, loggers, doctor code registries, `internal/core/report` (in-process `iface` family), and `issuequeue` -> SDK `RaiseIssue` (forkable `sysctl`) |
| No unintended coupling (components stay isolated) | Yes | Forkable `sysctl` never imports `internal/core/report` (routes raises through `issuequeue` + SDK `RaiseIssue`); each package registers only its own doctor codes |
| No duplicated functionality (extends existing, does not recreate) | Yes | Uses existing `diagnostic.Register`, `diagnostic.RegisterEventCode`, `slogutil.Logger`, `internal/core/report`, and `internal/component/plugin/issuequeue` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Control-plane and error paths only |
| Registration over hardcoding, outbound | Yes | New doctor codes register from each owning package's `init()` via `diagnostic.Register` and `diagnostic.RegisterEventCode` |
| Registration over hardcoding, inbound | Yes | No central switch, seed map, or hardcoded code table is edited; `./le arch doctor-coverage check` and `ze show diagnostic` discover registered codes dynamically |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The 36 instances across 14 files are the current population under `ai/doors/forwarding.md`'s base paths | `./le arch refuse-by-default report` run on 2026-11-26 | Work from the fresh report | Report filtered to this child's paths before the first edit and re-checked after implementation (`findings: 0, annotated: 4`) | confirmed |
| A-2 | A failed DHCP removal leaves the object in the kernel | Netlink error on delete | An ENOENT (already gone) is success/answer 3, not a failure | `isKernelObjectAbsent` distinguishes `ENOENT`, `ESRCH`, `EADDRNOTAVAIL`, and `os.ErrNotExist` from real removal failures; verified by `TestDHCPv4RemovalReportsRealErrorAndIgnoresAbsent` and `TestDHCPv6ShutdownReportsFailedRemoval` | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A WARN per netlink/VPP monitor message or RA send under a link flap storm floods the log | `test/iface` log grows per event | Count in existing metrics and WARN once per interface and condition |
| R-2 | ENOENT on removal at shutdown reported as a failure | WARN at every clean shutdown | Treat "already gone" as success |
| R-3 | A newly added `Warn`/`Error` log line lacks a declared doctor `code` and is refused at commit | `./le arch doctor-coverage check` fails | Register every new code via `diagnostic.Register` and `diagnostic.RegisterEventCode` (or a doctor check `Codes` list) in the owning package's doctor-codes file |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Log noise, commit refusal from `doctor-coverage`, or a reconcile that now acts on a reported removal failure |
| How is it reverted? | Single commit revert |
| Who else touches this path? | Specs on iface, DHCP, sysctl, VPP, and firewall-irr |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le arch refuse-by-default report` | → | `droppedError` over this child's paths | Report filtered to this child's paths: zero findings of `error-dropped-below-warn` |
| A sysctl default applied whose `applied` emit fails | → | `runSysctlPlugin` (`internal/component/sysctl/register.go`) | `TestSysctlAppliedEmitFailureIsWarned` |
| A DHCPv6 address that fails to be removed at shutdown | → | `v6Worker.shutdown` (`internal/plugins/iface/dhcp/dhcp_v6_linux.go`) | `TestDHCPv6ShutdownReportsFailedRemoval` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `./le arch refuse-by-default report '|' json`, filtered to this child's paths | Report completes over all forwarding base paths and lists 0 `error-dropped-below-warn` entries in `findings` | Only answer-3 sites in `annotated` carrying a valid `// refuse-by-default: guards <Symbol> -- <why>` (teardown socket closes and recovery guards where going on is proven safe) |
| AC-2 | A removal of Ze-owned kernel state (`removeV4Route`, `removeV4Address`, `v6Worker.drop`, `v6Worker.shutdown`) fails for a reason other than "already gone" | The failure reaches its owner (answer 1) or emits a WARN naming the interface, the kernel object, the fix, and its registered doctor `code` while raising that event on `internal/core/report`; "already gone" (ENOENT) completes cleanly as success | ENOENT ("already gone") is treated as already removed, because the kernel no longer holds the Ze-owned object |
| AC-3 | Each answer-1 instance | A unit test drives the failure and asserts the caller observes the returned error or updated ownership state | None; every answer-1 instance carries a direct unit test |
| AC-4 | Each file with an answer-2 WARN carrying a `diagnostic.RegisterEventCode` code | A unit test asserts both (a) one WARN line names what, why, the fix, and its registered doctor `code`, and (b) the site raises that event on the report bus (`internal/core/report` for `iface` and its in-process backends `ifacenetlink`, `ifacevpp`, `ifacedhcp`, `ifacera`; `internal/component/plugin/issuequeue` -> SDK `RaiseIssue` for forkable `sysctl`), and `./le arch doctor-coverage check` passes with exit 0 | Repeating event paths (monitor emit, RA send) suppress duplicate WARNs and raises after the first occurrence per interface/condition (or rate window) while counting where metrics apply to prevent log and bus floods |
| AC-5 | Each answer-3 annotation | Every annotation in `annotated` names a guard symbol visible in that file and states why going on is correct | None; every annotation is checked by `refuse-by-default` |
| AC-6 | Scoped package tests (`go test` over changed packages), `./le test integration iface`, and reached `.ci` suites | All targeted unit, Linux integration (in `./le test qemu all` for Linux-only packages), and `.ci` suites pass | None; any failure in scope is fixed before closure |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSysctlAppliedEmitFailureIsWarned` | `internal/component/sysctl/register_test.go` | AC-4 (WARN with what, why, fix, registered doctor `code`, and `issuequeue` -> SDK `RaiseIssue` report-bus raise) | |
| `TestDHCPv6ShutdownReportsFailedRemoval` | `internal/plugins/iface/dhcp/dhcp_v6_linux_test.go` | AC-2, AC-4 (non-ENOENT removal failure reported + raised on `internal/core/report`; ENOENT treated as success) | |
| One test per other answer-1 and answer-2 package | The package of each instance (`iface`, `sysctl`, `dhcp`, `netlink`, `ra`, `vpp`, `firewall/plugins/irr`) | AC-2, AC-3, AC-4 (including report-bus raise assertion for every `RegisterEventCode` site) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| unannotated findings under this child's paths | 0 | 0 | N/A | 1 |
| uncovered new `Warn`/`Error` sites in `./le arch doctor-coverage check` | 0 | 0 | N/A | 1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `.ci` the change reaches under `test/` (from `./le repo changed scope print both`) | `test/` | Unchanged kernel state on happy paths; WARN with registered doctor `code` where a `.ci` drives a failure | |

## Files to Modify
- `internal/component/firewall/plugins/irr/stored.go` - 1: `storeReader.notice` 112
- `internal/component/iface/config_apply.go` - 1: `addDesiredAddresses` 2388
- `internal/component/iface/config_sysctl.go` - 3: `applySysctl.func` 30; `applySysctlProfiles` 161, 178
- `internal/component/iface/doctor_codes.go` - register `diagnostic.CodeMeta` for every new `Warn` log code added in `internal/component/iface`
- `internal/component/iface/register.go` - 2: `runEngine.func.func` 836; `cleanupStaleIPv6DefaultRoutes` 1864; plus `diagnostic.RegisterEventCode` calls for new iface doctor codes
- `internal/component/sysctl/register.go` - 8: `runSysctlPlugin.func` 185, 206, 222, 237, 275, 294, 429; `runSysctlPlugin.func.func` 359
- `internal/component/sysctl/register_doctor_codes.go` - register `diagnostic.CodeMeta` and `diagnostic.RegisterEventCode` for every new `Warn` log code added in `internal/component/sysctl`
- `internal/component/sysctl/sysctl.go` - 1: `store.saveOriginal` 188
- `internal/plugins/iface/dhcp/dhcp_linux.go` - 2: `DHCPClient.publishDHCP` 264, 268
- `internal/plugins/iface/dhcp/dhcp_v4_exchange_linux.go` - 2: `closeV4Socket` 215; `closeV4Client` 230
- `internal/plugins/iface/dhcp/dhcp_v4_linux.go` - 2: `DHCPClient.removeV4Route` 640; `DHCPClient.removeV4Address` 699
- `internal/plugins/iface/dhcp/dhcp_v6_attach_linux.go` - 2: `v6Worker.dropAddresses` 338; `v6Worker.decline` 581
- `internal/plugins/iface/dhcp/dhcp_v6_linux.go` - 7: `closeV6Link` 219; `v6Worker.drop` 592; `v6Worker.expireDue` 609; `v6Worker.shutdown` 628, 635, 651, 656
- `internal/plugins/iface/netlink/monitor_linux.go` - 2: `monitor.emit` 572, 576
- `internal/plugins/iface/netlink/register_doctor_codes.go` - register `diagnostic.CodeMeta` and `diagnostic.RegisterEventCode` for every new `Warn` log code added in `internal/plugins/iface/netlink`
- `internal/plugins/iface/ra/register_doctor_codes.go` - register `diagnostic.CodeMeta` and `diagnostic.RegisterEventCode` for the new `Warn` log code added in `internal/plugins/iface/ra`
- `internal/plugins/iface/ra/sender_linux.go` - 1: `Sender.run.func` 294
- `internal/plugins/iface/vpp/monitor.go` - 2: `monitor.emit` 456, 461
- `internal/plugins/iface/vpp/register_doctor_codes.go` - register `diagnostic.CodeMeta` and `diagnostic.RegisterEventCode` for every new `Warn` log code added in `internal/plugins/iface/vpp`

Lines are those of HEAD on 2026-11-26; the report is the current list.

## Files to Create
- `internal/plugins/iface/dhcp/register_doctor_codes.go` - register `diagnostic.CodeMeta` and `diagnostic.RegisterEventCode` for every new `Warn` log code added in `internal/plugins/iface/dhcp` (the package currently has no doctor-codes file)
- `internal/component/sysctl/register_test.go` - unit test `TestSysctlAppliedEmitFailureIsWarned` and sysctl emit-failure tests
- `internal/plugins/iface/dhcp/dhcp_v6_linux_test.go` - unit test `TestDHCPv6ShutdownReportsFailedRemoval` and DHCPv6 removal/registration failure tests
- Additional focused unit test additions in the remaining modified packages as needed to cover every answer-1 and answer-2 branch

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config leaf or RPC |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | N-A | No command |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No command |
| Functional test for new RPC/API | N-A | No RPC |
| Pipe completeness | N-A | No output change |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new runtime dependency |
| Doctor code per problem class | Yes | Every new `Warn`/`Error` log line registers its `diagnostic.CodeMeta` and `diagnostic.RegisterEventCode` (or doctor check `Codes` entry) in its owning package: `internal/component/iface/doctor_codes.go` (+ `register.go`), `internal/component/sysctl/register_doctor_codes.go`, `internal/plugins/iface/dhcp/register_doctor_codes.go`, `internal/plugins/iface/netlink/register_doctor_codes.go`, `internal/plugins/iface/ra/register_doctor_codes.go`, `internal/plugins/iface/vpp/register_doctor_codes.go`; every site carrying a `RegisterEventCode` code raises it on the report bus (`internal/core/report` in `iface` and its in-process backends; `internal/component/plugin/issuequeue` + SDK `RaiseIssue` in forkable `sysctl`); verified by `./le arch doctor-coverage check` |
| Prometheus counters/metrics | Yes, if answer 2 adds a counter | Existing counters exist only in `internal/component/iface/rate.go` (`ifaceMetrics`) and `internal/plugins/iface/ra/ifacera.go` (`raMetrics`); `dhcp`, `netlink`, and `vpp` currently register none; name and document any counter added at implementation in `docs/features/interfaces.md` |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | If answer 2 adds a counter | `docs/features/interfaces.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Declared `// Design:` docs: `docs/architecture/core-design.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/architecture/firewall/firewall-irr.md`, `docs/features/interfaces.md`, `docs/research/vpp-deployment-reference.md`. Log levels and registered event codes change none of their architectural contracts unless answer 1 changes described ownership behavior; confirm at closure with `./le spec citation anchors spec plan/release/fixes/spec-dropped-error-5-forwarding.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Design presentation

### What changes

This spec resolves all 36 `error-dropped-below-warn` instances across 14 files under the forwarding door (`iface`, `sysctl`, `dhcp`, `netlink` monitor, `ra` sender, `vpp` monitor, and `firewall-irr`) using the umbrella's three-step rule: propagate or repair failures where going on leaves Ze-owned kernel state behind (distinguishing ENOENT "already gone" from real removal errors), promote operator-actionable failures to WARN with what, why, fix, and a registered doctor `code` while raising every `diagnostic.RegisterEventCode` event on the report bus (`internal/core/report` for `iface` and its in-process backends `ifacenetlink`, `ifacevpp`, `ifacedhcp`, `ifacera`; `internal/component/plugin/issuequeue` -> SDK `RaiseIssue` for forkable `sysctl`, rate-limited once per interface/condition on repeating event paths) so `./le arch doctor-coverage check` and `recent-events` both hold, and annotate genuine no-action teardown or self-recovering guards with `// refuse-by-default: guards <Symbol> -- <why>`.

### Before and after

1. **Sysctl apply event emission failure (`internal/component/sysctl/register.go`)**
   - Before: `log.Debug("sysctl: applied emit failed", "err", emitErr)` — hidden at the default WARN log level, leaving subscribers silently unaware a kernel tunable changed and invisible to `show errors` / `show doctor`.
   - After: `log.Warn(...)` naming the sysctl event/key, why subscribers missed the update, the fix (`ze show sysctl`), and its registered doctor `code` from `internal/component/sysctl/register_doctor_codes.go`, AND raises that event to the daemon's report bus through `internal/component/plugin/issuequeue` -> SDK `RaiseIssue` (so it works whether `sysctl` runs internal or forked).
2. **DHCPv6 or DHCPv4 leased address/route removal failure (`internal/plugins/iface/dhcp/dhcp_v6_linux.go`, `dhcp_v4_linux.go`)**
   - Before: `loggerPtr.Load().Debug("iface dhcp v6: address removal failed", ...)` — a real netlink removal failure at drop or shutdown leaves a Ze-owned leased address or route in the kernel with no default-level trace, while an ENOENT is treated identically to a real failure.
   - After: ENOENT ("already gone") completes silently as success; any real removal failure reaches ownership tracking or logs at WARN with the interface, address/route, fix, and a registered doctor `code` from `internal/plugins/iface/dhcp/register_doctor_codes.go` AND raises that event on `internal/core/report` (matching the in-process `ifacenetlink` precedent in `monitor_report_linux.go`).
3. **Teardown socket close and self-recovering IRR notice decode (`dhcp_v4_exchange_linux.go`, `dhcp_v6_linux.go`, `firewall/plugins/irr/stored.go`)**
   - Before: bare `Debug` log flagged as `error-dropped-below-warn`.
   - After: annotated with `// refuse-by-default: guards <Symbol> -- <why>` pointing at the teardown lifecycle or `r.all` full-reread recovery guard, so `./le arch refuse-by-default report` reports zero unannotated findings under forwarding paths.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Promote all 36 sites mechanically to `Warn` without classifying | Violates `ai/rules/points/go-standards/directives/log-a-dropped-error-at-warn-or-above.md`: a WARN does not make going on right when Ze-owned kernel state stays behind, causes log noise on ENOENT/teardown closes, and floods logs on repeating monitor/RA paths |
| Silence all 36 sites with `// refuse-by-default:` annotations | Hides genuine operator-actionable failures (lost sysctl events, failed kernel removals, dropped monitor notifications) and defeats the refuse-by-default check |
| Emit `Warn` lines without doctor codes or without raising `RegisterEventCode` events on the report bus | Refused by `./le arch doctor-coverage check` and violates `docs/architecture/doctor-and-health-checks.md` ("every site carrying a registered event code raises it") |
| Call `internal/core/report` directly from `sysctl` | `sysctl` is a forkable plugin (`CLIHandler` + `sdk.NewWithConn`, no `!p.IsInternal()` guard); direct `internal/core/report` calls mutate only the subprocess's disconnected store and never reach `show warnings`/`show doctor` |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring** -- run `./le arch refuse-by-default report` filtered to this child's paths to confirm A-1; write the failing wiring tests `TestSysctlAppliedEmitFailureIsWarned` and `TestDHCPv6ShutdownReportsFailedRemoval`.
2. **Phase: Classify and resolve** -- record each of the 36 instances' answer and reason in Design Insights; register every new doctor code in its owning package's `doctor_codes.go` / `register_doctor_codes.go`; write unit tests first for answers 1 and 2; add answer-3 annotations last.
3. **Phase: Prove** -- verify AC-1 through AC-6 with `./le arch refuse-by-default report`, `./le arch doctor-coverage check`, scoped package tests, `./le test integration iface`, reached `.ci` suites from `./le repo changed scope print both`, and Linux-only tests in `./le test qemu all` (run by the main thread in the main checkout).

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every one of the 36 instances has its answer at file:line |
| Correctness | "Already gone" (ENOENT) is never reported as a failure; a real failed removal is never silent |
| Operator errors and doctor codes | Every new `Warn`/`Error` line names the interface/object, why, the exact fix, and a registered doctor `code` listed in the owning package's doctor-codes file; every site carrying a `diagnostic.RegisterEventCode` code raises it on the report bus (`internal/core/report` in `iface` and its in-process backends; `internal/component/plugin/issuequeue` -> SDK `RaiseIssue` in forkable `sysctl`); `./le arch doctor-coverage check` passes |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Zero unannotated dropped-error findings under this child's paths | `./le arch refuse-by-default report '|' json` filtered to forwarding base paths; `./le arch refuse-by-default check` |
| Every new `Warn`/`Error` log line covered by a registered doctor code | `./le arch doctor-coverage check` |
| Scoped package tests | Main thread runs `./le test unit plugins` limited to the changed packages, and `./le job run label forwarding command go test -race ./internal/component/iface/ ./internal/component/sysctl/ ./internal/component/firewall/plugins/irr/ ./internal/plugins/iface/dhcp/ ./internal/plugins/iface/netlink/ ./internal/plugins/iface/ra/ ./internal/plugins/iface/vpp/` |
| Linux integration and reached `.ci` suites | Main thread runs `./le repo changed scope print both`, `./le test integration iface`, reached `.ci` suites, and `./le test qemu all` for the Linux-only packages |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | N-A: no new input |
| Error leakage | Confirm new WARN lines carry no secrets or credentials (interface names, prefixes, sysctl keys, and error causes only) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

Resolution of all 36 `error-dropped-below-warn` instances across 14 files (32 answer-2 WARN + report-bus raises across 13 registered doctor event codes; 4 answer-3 guard annotations; 0 answer-1 returns):

| # | File | Symbol (site count) | Answer | Resolution (Doctor Code or Guard) | Proof |
|---|------|---------------------|--------|-----------------------------------|-------|
| 1 | `internal/component/firewall/plugins/irr/stored.go` | `storeReader.notice` (1) | 3 | `guards readEvery` (`r.all = true` triggers full reread on next pass) | `./le arch refuse-by-default report` (`annotated: 4`) |
| 2 | `internal/component/iface/config_apply.go` | `addDesiredAddresses` (1) | 2 | `doctor-iface-address-add` via `internal/core/report` (`isDeferredAddressAddBenign` skips `ErrBackendNotReady`, `EEXIST`, `os.ErrExist`) | `TestAddDesiredAddressesReportsRealFailureAndSkipsBenign` |
| 3-5 | `internal/component/iface/config_sysctl.go` | `applySysctl` (1), `applySysctlProfiles` (2) | 2 | `doctor-iface-sysctl-emit` via `reportSysctlEmitFailed` -> `internal/core/report` | `TestApplySysctlAndProfilesReportEmitFailure` |
| 6 | `internal/component/iface/register.go` | `runEngine` rollback via `emitInterfaceRollback` (1) | 2 | `doctor-iface-rollback-emit` via `internal/core/report` | `TestEmitInterfaceRollbackReportsEmitFailure` |
| 7 | `internal/component/iface/register.go` | `cleanupStaleIPv6DefaultRoutes` (1) | 2 | `doctor-iface-stale-route-cleanup` via `internal/core/report` | `TestCleanupStaleIPv6DefaultRoutesReportsListFailure` |
| 8-15 | `internal/component/sysctl/register.go` | `runSysctlPlugin` (`EventDefault`, `EventSet`, `EventShowRequest`, `EventListRequest`, `EventDescribeRequest`, `applyFromJSON`, `OnConfigApply`, `OnExecuteCommand`) (8) | 2 | `doctor-sysctl-event-emit` via `reportSysctlEventEmitFailed` -> `issuequeue` -> SDK `RaiseIssue` | `TestSysctlAppliedEmitFailureIsWarned` (wire + bridge), `TestSysctlOriginalUnreadAndQueryEmitFailuresAreReported` |
| 16 | `internal/component/sysctl/sysctl.go` | `store.saveOriginal` (1) | 2 | `doctor-sysctl-original-unread` via `s.issues.RaiseError` -> SDK `RaiseIssue` | `TestSysctlOriginalUnreadAndQueryEmitFailuresAreReported` |
| 17-18 | `internal/plugins/iface/dhcp/dhcp_linux.go` | `DHCPClient.publishDHCP` marshal + emit (2) | 2 | `doctor-iface-dhcp-event-emit` rate-limited via `report.RaiseErrorFixOnce` (1m window) | `TestDHCPPublishReportsEmitFailureRateLimited` |
| 19-20 | `internal/plugins/iface/dhcp/dhcp_v4_exchange_linux.go` | `closeV4Socket` (1), `closeV4Client` (1) | 3 | `guards newV4UnicastClient`, `guards nclient4Exchanger` (best-effort teardown closes) | `./le arch refuse-by-default report` (`annotated: 4`) |
| 21-22 | `internal/plugins/iface/dhcp/dhcp_v4_linux.go` | `removeV4Route` (1), `removeV4Address` (1) | 2 | `doctor-iface-dhcp-remove-failed` via `internal/core/report` (`isKernelObjectAbsent` treats `ENOENT`/`ESRCH`/`EADDRNOTAVAIL`/`os.ErrNotExist` as clean success) | `TestDHCPv4RemovalReportsRealErrorAndIgnoresAbsent` |
| 23-24 | `internal/plugins/iface/dhcp/dhcp_v6_attach_linux.go` | `v6Worker.dropAddresses` (1), `v6Worker.decline` (1) | 2 | `doctor-iface-dhcp-lease-register` via `reportLeaseRegisterFailed` -> `internal/core/report` | `TestDHCPv6DropRegisterAndReleaseFailuresAreReported` |
| 25 | `internal/plugins/iface/dhcp/dhcp_v6_linux.go` | `closeV6Link` (1) | 3 | `guards v6Worker` (best-effort per-exchange socket close) | `./le arch refuse-by-default report` (`annotated: 4`) |
| 26-27 | `internal/plugins/iface/dhcp/dhcp_v6_linux.go` | `v6Worker.drop` (1), `v6Worker.shutdown` remove (1) | 2 | `doctor-iface-dhcp-remove-failed` via `w.removeAddress` -> `internal/core/report` (`isKernelObjectAbsent` silent) | `TestDHCPv6ShutdownReportsFailedRemoval`, `TestDHCPv6DropRegisterAndReleaseFailuresAreReported` |
| 28-29 | `internal/plugins/iface/dhcp/dhcp_v6_linux.go` | `v6Worker.expireDue` (1), `v6Worker.shutdown` register (1) | 2 | `doctor-iface-dhcp-lease-register` via `reportLeaseRegisterFailed` -> `internal/core/report` | `TestDHCPv6DropRegisterAndReleaseFailuresAreReported` |
| 30-31 | `internal/plugins/iface/dhcp/dhcp_v6_linux.go` | `v6Worker.shutdown` Release build (1) + open-socket (1) | 2 | `doctor-iface-dhcp-release-failed` via `reportReleaseFailed` -> `internal/core/report` | `TestDHCPv6DropRegisterAndReleaseFailuresAreReported` |
| 32-33 | `internal/plugins/iface/netlink/monitor_linux.go` | `monitor.emit` marshal + emit (2) | 2 | `doctor-iface-netlink-monitor-emit` latched per `(eventType, stage)` until recovery + `report.RaiseErrorFixOnce` (1m window) | `TestMonitorEmitReportsFailuresRateLimitedAndResetsOnRecovery` |
| 34 | `internal/plugins/iface/ra/sender_linux.go` | `Sender.run` via `sendPacket` (1) | 2 | `doctor-iface-ra-send` via `raSendReporter` latched per error until recovery + `report.RaiseErrorFixOnce` (1m window; silent on canceled `net.ErrClosed`) | `TestSenderSendPacketReportsFailureRateLimitedAndResetsOnRecovery` |
| 35-36 | `internal/plugins/iface/vpp/monitor.go` | `monitor.emit` marshal + emit (2) | 2 | `doctor-vpp-monitor-emit` latched per `(eventType, stage)` until recovery + `report.RaiseErrorFixOnce` (1m window) | `TestVPPMonitorEmitReportsFailuresRateLimitedAndResetsOnRecovery` |

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

The class row for the whole population is owned by the umbrella.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A failed removal of Ze-owned kernel state reaches its owner (with ENOENT treated as already gone) | WARN only | Ownership is what keeps Ze from leaking or deleting state another owner installed, and treating ENOENT as success avoids false warnings on clean shutdown |
| The umbrella's three answers, in order, elsewhere | Raise every line to WARN | A WARN does not make a default correct |
| Register a doctor `code` per new `Warn`/`Error` problem class in each owning package's doctor-codes file and raise every `RegisterEventCode` site on the report bus (`internal/core/report` for `iface` + in-process backends; `issuequeue` -> SDK `RaiseIssue` for forkable `sysctl`) | Bare WARN lines without `code`, log-only `RegisterEventCode` sites, or direct `internal/core/report` calls from `sysctl` | Required by `ai/rules/user-facing-errors.md` and `docs/architecture/doctor-and-health-checks.md`, enforced by `./le arch doctor-coverage check` at commit time, and preserves forkable process-boundary isolation for `sysctl` |

## Known Limitations
- None.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] AC-N rows carry testable assertions with positive Expected Behavior and Permitted Survivors
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, `./le arch refuse-by-default check`, `./le arch doctor-coverage check`, `./le test integration iface`, `./le test qemu all` for Linux-only packages, and the suites `./le repo changed scope print both` reaches (GitHub CI verifies the pushed commit)
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason): N-A, no wire change

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/release/fixes/spec-dropped-error-5-forwarding.md` only, in the same `./le commit create` script

---

## Implementation Summary

### What Was Implemented
- Resolved all 36 `error-dropped-below-warn` sites across 14 forwarding files (`findings: 0, annotated: 4` under `ai/doors/forwarding.md` base paths):
  - **Firewall IRR (`internal/component/firewall/plugins/irr/stored.go`):** annotated `storeReader.notice` (`guards readEvery`) because `r.all = true` already triggers a full store re-read on the next worker pass.
  - **Interface component (`internal/component/iface/{config_apply,config_sysctl,doctor_codes,register}.go`):** registered 4 doctor event codes (`doctor-iface-address-add`, `doctor-iface-sysctl-emit`, `doctor-iface-rollback-emit`, `doctor-iface-stale-route-cleanup`); promoted deferred `addDesiredAddresses` (skipping benign `ErrBackendNotReady`/`EEXIST`/`os.ErrExist`), `applySysctl`/`applySysctlProfiles`, `emitInterfaceRollback`, and `cleanupStaleIPv6DefaultRoutes` failures to WARN with fix + `internal/core/report` error raises.
  - **Forkable sysctl plugin (`internal/component/sysctl/{register,register_doctor_codes,sysctl}.go`):** registered 2 doctor event codes (`doctor-sysctl-event-emit`, `doctor-sysctl-original-unread`); wired `issuequeue.Start(p)` with `defer issues.Stop()` ordered after `unsubscribeAll()` and before `p.Close()`, so all 8 EventBus emit failures and `store.saveOriginal` read failures warn and raise via SDK `RaiseIssue` without importing `internal/core/report`.
  - **DHCPv4/DHCPv6 client (`internal/plugins/iface/dhcp/{dhcp_linux,dhcp_v4_exchange_linux,dhcp_v4_linux,dhcp_v6_attach_linux,dhcp_v6_linux,register_doctor_codes}.go`):** created `register_doctor_codes.go` declaring 4 doctor event codes (`doctor-iface-dhcp-event-emit`, `doctor-iface-dhcp-remove-failed`, `doctor-iface-dhcp-lease-register`, `doctor-iface-dhcp-release-failed`); added `isKernelObjectAbsent` (`ENOENT`, `ESRCH`, `EADDRNOTAVAIL`, `os.ErrNotExist`) so already-removed kernel addresses/routes complete cleanly while real failures warn and raise on `internal/core/report`; rate-limited `publishDHCP` via `report.RaiseErrorFixOnce`; annotated 3 best-effort teardown closes (`closeV4Socket`, `closeV4Client`, `closeV6Link`).
  - **Netlink monitor, VPP monitor, RA sender (`internal/plugins/iface/{netlink,vpp,ra}/`):** registered 3 doctor event codes (`doctor-iface-netlink-monitor-emit`, `doctor-vpp-monitor-emit`, `doctor-iface-ra-send`); added rate-limited latch + recovery reset + `report.RaiseErrorFixOnce` (1m window) on both monitor `emit` paths and `ra` `sendPacket` (staying silent on canceled `net.ErrClosed`).
- Added unit tests across all 6 answer-2 packages (`iface`, `sysctl`, `dhcp` v4 + v6, `netlink`, `ra`, `vpp`) covering WARN fields, registered doctor codes, report-bus delivery (including both wire and bridge transports for `sysctl`), ENOENT/benign suppression, and rate-limit + recovery behavior.
- Wrote operator news fragment `website/changes/fragments/dropped-error-5-forwarding.md`.

### Bugs Found/Fixed
- None beyond the 36 dropped-error sites resolved by this spec.

### Documentation Updates
- None required in `docs/`: no config syntax, CLI grammar, wire format, RPC contract, or Prometheus counter changed.
- Verified with `./le spec citation anchors spec plan/release/fixes/spec-dropped-error-5-forwarding.md` (17 docs mention modified files; none describe dropped-error log levels or doctor codes) and `./le doc check all` (0 broken source anchors or digest anchors on any owned file; the two reported failures belong to unrelated uncommitted files in other sessions: `internal/component/plugin/doctor/register_external.go` and `ze-policy-conf`).
- Operator-facing change recorded in `website/changes/fragments/dropped-error-5-forwarding.md`.

### Deviations from Plan
- **No answer-1 return conversions (0 answer-1, 32 answer-2, 4 answer-3):** Every DHCP removal site (`removeV4Route`, `removeV4Address`, `v6Worker.drop`, `v6Worker.shutdown`) executes during lease expiry, decline, or teardown where no higher retry loop keeps the dropped lease; AC-2 explicitly permits answer 2 ("The failure reaches its owner (answer 1) or emits a WARN naming the interface, the kernel object, the fix, and its registered doctor `code` while raising that event on `internal/core/report`; 'already gone' (ENOENT) completes cleanly as success").
- **No new Prometheus counters added:** Repeating monitor and RA send failure paths prevent log/bus floods via per-condition latches plus `report.RaiseErrorFixOnce` (1-minute window), so `docs/features/interfaces.md` needed no metrics table update.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| none | | | | |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Resolve all 36 `error-dropped-below-warn` instances across 14 forwarding files | Done | See Design Insights table (14 source files, 36 sites) | `./le arch refuse-by-default report`: `findings: 0, annotated: 4`; `./le arch refuse-by-default check`: OK |
| Every new `Warn` carries a registered doctor `code` and raises on report bus | Done | `iface/doctor_codes.go` + `register.go`, `sysctl/register_doctor_codes.go`, `dhcp/register_doctor_codes.go`, `netlink/register_doctor_codes.go`, `ra/register_doctor_codes.go`, `vpp/register_doctor_codes.go` | `./le arch doctor-coverage check`: OK |
| Forkable `sysctl` raises via `issuequeue` -> SDK `RaiseIssue`, never `internal/core/report` | Done | `internal/component/sysctl/register.go` (`runSysctlPlugin`, `reportSysctlEventEmitFailed`), `internal/component/sysctl/sysctl.go` (`saveOriginal`) | `./le plugin boundary check`: OK; tested over wire + bridge transports |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `./le arch refuse-by-default report` over `ai/doors/forwarding.md` base paths (`findings: 0, annotated: 4`); `./le arch refuse-by-default check` OK | Main thread verified |
| AC-2 | Done | `isKernelObjectAbsent` (`dhcp_linux.go`), `removeV4Route`/`removeV4Address` (`dhcp_v4_linux.go`), `v6Worker.removeAddress` (`dhcp_v6_linux.go`); `TestDHCPv4RemovalReportsRealErrorAndIgnoresAbsent`, `TestDHCPv6ShutdownReportsFailedRemoval`, `TestDHCPv6DropRegisterAndReleaseFailuresAreReported` | Non-absent errors warn + raise `doctor-iface-dhcp-remove-failed`; `ENOENT`/`ESRCH`/`EADDRNOTAVAIL`/`os.ErrNotExist` complete cleanly |
| AC-3 | Done | N-A (0 answer-1 sites; all 36 resolved by 32 answer-2 + 4 answer-3 per AC-2) | Recorded in Deviations |
| AC-4 | Done | Unit tests in all 6 answer-2 packages (`iface`, `sysctl`, `dhcp`, `netlink`, `ra`, `vpp`) + `./le arch doctor-coverage check` OK | Every test asserts WARN attributes, registered `CodeMeta` + `EventCode`, and report-bus raise |
| AC-5 | Done | 4 annotations (`stored.go` `readEvery`, `dhcp_v4_exchange_linux.go` `newV4UnicastClient` & `nclient4Exchanger`, `dhcp_v6_linux.go` `v6Worker`) | All 4 accepted by `./le arch refuse-by-default report` (`annotated: 4`) |
| AC-6 | Done | Main thread ran `./le test unit pkg` (including Linux `userns` integration tests) across all 7 modified packages + 6-flavor lint (0 issues) + `plugin boundary check` + `weakened check` (0 findings) | All owned tests PASS (1 unrelated pre-existing `synctest` ordering race in untouched `netlink/native_access_linux_test.go:559`) |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestSysctlAppliedEmitFailureIsWarned` | Done | `internal/component/sysctl/register_test.go` | Runs over both bridge and wire transports |
| `TestSysctlOriginalUnreadAndQueryEmitFailuresAreReported` | Done | `internal/component/sysctl/register_test.go` | Covers `saveOriginal` + `show-result` emit |
| `TestDHCPv6ShutdownReportsFailedRemoval` | Done | `internal/plugins/iface/dhcp/dhcp_v6_linux_test.go` | Covers `EPERM` warn+raise and `ENOENT` silence |
| `TestDHCPv6DropRegisterAndReleaseFailuresAreReported` | Done | `internal/plugins/iface/dhcp/dhcp_v6_linux_test.go` | Covers `drop`, `expireDue` register conflict, shutdown Release socket failure |
| `TestDHCPv4RemovalReportsRealErrorAndIgnoresAbsent` | Done | `internal/plugins/iface/dhcp/dhcp_v4_dropped_errors_linux_test.go` | Covers `removeV4Route` + `removeV4Address` (`EPERM`, `ENOENT`, `ESRCH`, `EADDRNOTAVAIL`) |
| `TestDHCPPublishReportsEmitFailureRateLimited` | Done | `internal/plugins/iface/dhcp/dhcp_v4_dropped_errors_linux_test.go` | Covers `publishDHCP` rate-limited warn+raise |
| `TestAddDesiredAddressesReportsRealFailureAndSkipsBenign` | Done | `internal/component/iface/dropped_errors_test.go` | Covers `addDesiredAddresses` (`EPERM` vs `ErrBackendNotReady`/`EEXIST`/`os.ErrExist`) |
| `TestApplySysctlAndProfilesReportEmitFailure` | Done | `internal/component/iface/dropped_errors_test.go` | Covers `applySysctl` + `applySysctlProfiles` |
| `TestEmitInterfaceRollbackReportsEmitFailure` | Done | `internal/component/iface/dropped_errors_test.go` | Covers `emitInterfaceRollback` |
| `TestCleanupStaleIPv6DefaultRoutesReportsListFailure` | Done | `internal/component/iface/dropped_errors_test.go` | Covers `cleanupStaleIPv6DefaultRoutes` |
| `TestMonitorEmitReportsFailuresRateLimitedAndResetsOnRecovery` | Done | `internal/plugins/iface/netlink/monitor_linux_test.go` | Covers marshal + emit rate limit + recovery reset |
| `TestSenderSendPacketReportsFailureRateLimitedAndResetsOnRecovery` | Done | `internal/plugins/iface/ra/sender_linux_test.go` | Covers rate limit + recovery reset + canceled `net.ErrClosed` silence |
| `TestVPPMonitorEmitReportsFailuresRateLimitedAndResetsOnRecovery` | Done | `internal/plugins/iface/vpp/monitor_emit_test.go` | Covers marshal + emit rate limit + recovery reset |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| All 19 files in Files to Modify | Done | Modified in working tree, plus `dhcp_v4_rig_linux_test.go`, `netlink/monitor_linux_test.go`, `ra/sender_linux_test.go` for unit test coverage |
| `internal/plugins/iface/dhcp/register_doctor_codes.go` | Done | Created with 4 doctor codes + `RegisterEventCode` |
| `internal/component/sysctl/register_test.go` | Done | Created |
| `internal/plugins/iface/dhcp/dhcp_v6_linux_test.go` | Done | Created |
| `internal/component/iface/dropped_errors_test.go` | Done | Created (additional focused unit tests) |
| `internal/plugins/iface/dhcp/dhcp_v4_dropped_errors_linux_test.go` | Done | Created (additional focused unit tests) |
| `internal/plugins/iface/vpp/monitor_emit_test.go` | Done | Created (additional focused unit tests) |
| `website/changes/fragments/dropped-error-5-forwarding.md` | Done | Created at closure |

### Audit Summary
- **Total items:** 6 ACs, 36 dropped-error sites across 14 files, 13 new doctor event codes, 28 Go files + 1 news fragment
- **Done:** All 6 ACs, all 36 sites, all 13 doctor codes, all planned files and tests
- **Partial:** None
- **Skipped:** None
- **Changed:** 2 minor deviations recorded above (0 answer-1 returns per AC-2; no new Prometheus counters)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Zero `error-dropped-below-warn` findings under forwarding base paths | Structural check | `./le arch refuse-by-default report` over `ai/doors/forwarding.md` base paths: `findings: 0, annotated: 4`; `./le arch refuse-by-default check`: OK |
| Failed DHCP kernel removals distinguish absent objects from real failures and report real failures | Unit + Linux `userns` integration tests | `TestDHCPv4RemovalReportsRealErrorAndIgnoresAbsent`, `TestDHCPv6ShutdownReportsFailedRemoval`, `TestDHCPv6DropRegisterAndReleaseFailuresAreReported` PASS in `internal/plugins/iface/dhcp` (67.9s) |
| Operator-actionable failures emit WARN with doctor code and raise on daemon report bus | Unit tests + structural gate | All 13 new codes verified by unit tests across `iface`, `sysctl`, `dhcp`, `netlink`, `ra`, `vpp` + `./le arch doctor-coverage check` OK |
| Forkable `sysctl` reports issues across process boundary without touching daemon-local `internal/core/report` | Dual-transport unit test + boundary gate | `TestSysctlAppliedEmitFailureIsWarned` PASS on both `bridge` and `wire` transports (1.1s); `./le plugin boundary check` OK |
| Repeating monitor and RA failure paths do not flood logs or report bus | Unit tests | `TestMonitorEmitReportsFailuresRateLimitedAndResetsOnRecovery`, `TestVPPMonitorEmitReportsFailuresRateLimitedAndResetsOnRecovery`, `TestSenderSendPacketReportsFailureRateLimitedAndResetsOnRecovery`, `TestDHCPPublishReportsEmitFailureRateLimited` PASS |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | All 36 forwarding dropped-error sites and all 6 ACs are complete | none |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/dropped-error-5-forwarding-b2c30003-3333-4000-8000-000000000003.md` |
| `./le spec review check` | `review_gate: OK (28 code files, clean, hashes match tmp/review/dropped-error-5-forwarding-b2c30003-3333-4000-8000-000000000003.md)` |
| Rounds | 1 |
| Reviewer lenses used | Lens 1: logic, wiring, concurrency, removed-behavior, and 7-question Go style pass (`ze-go-style.md`); Lens 2: security, error leakage, bounded allocations, rate limiting, and forkable process boundary; Lens 3: spec completeness (`/ze-review-spec`), 36-site census, doctor codes, and `## Design documents` check |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| none | - | 0 BLOCKER, 0 ISSUE, 0 NOTE | - | - |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/plugins/iface/dhcp/register_doctor_codes.go` | Yes | `-rw-r--r-- 1 thomas thomas 4410 Nov 27 09:34 internal/plugins/iface/dhcp/register_doctor_codes.go` |
| `internal/component/sysctl/register_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 9460 Nov 27 09:34 internal/component/sysctl/register_test.go` |
| `internal/plugins/iface/dhcp/dhcp_v6_linux_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 10535 Nov 27 09:04 internal/plugins/iface/dhcp/dhcp_v6_linux_test.go` |
| `internal/component/iface/dropped_errors_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 9081 Nov 27 09:10 internal/component/iface/dropped_errors_test.go` |
| `internal/plugins/iface/dhcp/dhcp_v4_dropped_errors_linux_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 7186 Nov 27 09:10 internal/plugins/iface/dhcp/dhcp_v4_dropped_errors_linux_test.go` |
| `internal/plugins/iface/vpp/monitor_emit_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 3797 Nov 27 08:58 internal/plugins/iface/vpp/monitor_emit_test.go` |
| `website/changes/fragments/dropped-error-5-forwarding.md` | Yes | `-rw-r--r-- 1 thomas thomas 2985 Nov 27 10:08 website/changes/fragments/dropped-error-5-forwarding.md` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | 0 unannotated `error-dropped-below-warn` findings under forwarding base paths | `./le arch refuse-by-default report` (`findings: 0, annotated: 4`); `./le arch refuse-by-default check` OK |
| AC-2 | Non-ENOENT DHCP removals warn + raise; absent kernel objects complete cleanly | `isKernelObjectAbsent` in `dhcp_linux.go` + `TestDHCPv4RemovalReportsRealErrorAndIgnoresAbsent` + `TestDHCPv6ShutdownReportsFailedRemoval` + `TestDHCPv6DropRegisterAndReleaseFailuresAreReported` PASS |
| AC-3 | Answer-1 sites tested | 0 answer-1 sites (all 36 resolved by 32 answer-2 + 4 answer-3 per AC-2) |
| AC-4 | Every answer-2 package warns with doctor code + raises on report bus; `doctor-coverage` passes | Unit tests PASS in all 6 answer-2 packages (`iface`, `sysctl`, `dhcp`, `netlink`, `ra`, `vpp`); `./le arch doctor-coverage check` OK; `./le plugin boundary check` OK |
| AC-5 | Every answer-3 annotation names a visible guard symbol | `git grep` verified `readEvery` (`stored.go:270`), `newV4UnicastClient` & `nclient4Exchanger` (`dhcp_v4_exchange_linux.go`), `v6Worker` (`dhcp_v6_linux.go`); accepted in `annotated: 4` |
| AC-6 | Scoped unit + Linux integration tests and 6-flavor lint pass | Main thread verified all 7 packages PASS (`iface` 191.1s, `sysctl` 1.1s, `firewall/plugins/irr` 1.6s, `dhcp` 67.9s, `ra` 31.8s, `vpp` 1.9s, `netlink` `monitor_linux_test.go` + integration PASS), 6-flavor lint 0 issues, `./le test weakened check` 0 findings |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le arch refuse-by-default report` over forwarding base paths | Structural check (`droppedError`) | Yes (`findings: 0, annotated: 4`; `./le arch refuse-by-default check` OK) |
| `runSysctlPlugin` EventBus applied emit failure -> `issuequeue` -> SDK `RaiseIssue` | `TestSysctlAppliedEmitFailureIsWarned` (wire + bridge transports) | Yes (PASS in `internal/component/sysctl`, 1.1s) |
| `v6Worker.shutdown` failed address removal -> WARN + `internal/core/report` | `TestDHCPv6ShutdownReportsFailedRemoval` | Yes (PASS in `internal/plugins/iface/dhcp`, 67.9s) |
| Every new doctor event code registered in owning package `init()` | `./le arch doctor-coverage check` + `./le plugin boundary check` | Yes (both OK) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `./le arch refuse-by-default report` confirmed all 36 instances across 14 files under `ai/doors/forwarding.md` base paths and now reports `findings: 0, annotated: 4` |
| A-2 | confirmed | `isKernelObjectAbsent` (`dhcp_linux.go`) distinguishes `ENOENT`, `ESRCH`, `EADDRNOTAVAIL`, and `os.ErrNotExist` from real netlink removal failures; verified by `TestDHCPv4RemovalReportsRealErrorAndIgnoresAbsent` and `TestDHCPv6ShutdownReportsFailedRemoval` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `## Design documents` (20 pages, 33 decision rows) | `./le spec design check spec plan/release/fixes/spec-dropped-error-5-forwarding.md` -> `ready to present` | Yes |
| Existing doc source anchors (`./le spec citation anchors` + `./le doc check all`) | 0 broken source anchors and 0 broken digest anchors on any of the 28 owned files; no config/CLI/wire/metric surface changed | Yes |
| Operator news fragment | `website/changes/fragments/dropped-error-5-forwarding.md` written with valid `topic: Diagnostics` from `website/data/topics.json` and all 6 fixed headings | Yes |
| Spec citations (`git grep dropped-error-5-forwarding`) | Only `plan/release/fixes/spec-dropped-error-0-umbrella.md:83` cites this spec (main thread updates that row to bare stem `spec-dropped-error-5-forwarding` right before Commit A after parallel `dropped-error-3` lands) | Yes |
