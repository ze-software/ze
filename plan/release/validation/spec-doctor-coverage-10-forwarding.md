# Spec: doctor-coverage-10-forwarding

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | plugin |
| Priority | P2 |
| Release reason | `ai/META-DECISIONS.md` (Goal and scope): `ze doctor` reports every problem an operator can hit, each with its cause and the command that fixes it; O-3 of `spec-doctor-coverage-1-audit-and-gate` makes every area child release work |
| RFC gaps | - |
| Depends | `spec-doctor-coverage-1-audit-and-gate` (the `runtime` phase, `Diagnostic.Fix`, the doctor-coverage gate, the `recent-events` check: landed in commits ce58dddf53 and fc96556110); AC-7 also `plan/release/validation/spec-doctor-coverage-2-home-cpe-batch.md` phase 4 (its name-resolver check); the cross-cutting plugin codes (`doctor-plugin-stopped`, `doctor-plugin-config-unapplied`, `doctor-plugin-setup-failed`, `doctor-plugin-external-unsupported`, `doctor-internal-defect`), the event-code registration and the report-bus fix convention are `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` phase P0a; AC-25 and AC-26 raise under `doctor-dns-source-consumer-stale` and `doctor-resolv-conf-stale`, declared by the `dns-source-selection` check of `plan/release/validation/spec-doctor-coverage-5-appliance.md` (its AC-14, AC-17, AC-18). This spec builds none of them |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent: `spec-doctor-coverage-1-audit-and-gate` (AC-6, AC-9).
Source: the A8 (interfaces, sysctl, system RIB) and A9 (firewall, traffic, VPP, FIB writers, policy routing) slices of the doctor problem inventory, and the canonical code table of the merge.

## Task

Doctor reports every problem class of the forwarding area (interfaces, DHCP and RA clients, sysctl, the system RIB, firewall and traffic control, the FIB writers, policy routing and VPP) with its cause and the command that fixes it, on every surface through `show doctor`. Every Warn and Error log line, every report-bus raise and every health row of the area names a doctor code that a registered check declares (O-2), so `./le arch doctor-coverage report` counts the area covered.

| Sub-area | Packages | Gap rows | Defects owned |
|----------|----------|----------|---------------|
| Interfaces, DHCP, RA | `internal/component/iface`, `internal/plugins/iface/{dhcp,netlink,ra}` | 42 | iface health row reads healthy when it read nothing; unknown `sysctl-profile` commits |
| sysctl, sysrib | `internal/component/sysctl`, `internal/component/sysrib` | 7 | - |
| Firewall and traffic | `internal/component/firewall/**`, `internal/plugins/firewall/nft`, `internal/component/traffic`, `internal/plugins/traffic/netlink` | 15 | IRR check omits a code it emits; nf_tables judged from `/proc/modules` |
| FIB and policy routing | `internal/plugins/fib/{kernel,p4}`, `internal/plugins/policyroute` | 14 | - |
| VPP | `internal/component/vpp`, `internal/plugins/iface/vpp`, `internal/plugins/fib/vpp`, `internal/plugins/firewall/vpp`, `internal/plugins/traffic/vpp` | 11 | - |

Rows child 2 owns stay there and get no AC here: interface backend not loaded (class 4), ethernet selector runtime half (class 7), PPPoE session ended (class 6), bridge STP (class 5).

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the four questions; the report bus and the firewall table registry are process-local
  → Constraint: a check registers from its owner's `init()`; no central list of codes, checks or plugins
- [ ] `ai/doors/forwarding.md` - backends, ownership by `rtm_protocol` and the `ze_` prefix, the report bus is process-local
  → Constraint: a check that reports "left behind" state counts only what Ze owns: routes by `rtm_protocol` 250 and 251, nft tables by the `ze_` prefix, the tc qdisc of the persisted snapshot, VPP objects by their `ze/` tag
  → Constraint: a raise a forked plugin makes never reaches `show warnings`, so a runtime check over the report bus is valid only for an owner that runs in the daemon process
  → Constraint: exact or refuse at verify; a doctor row is the backstop for a config already running, never the substitute for a verify refusal
- [ ] `ai/doors/plugin-infra.md`, `ai/doors/core.md`, `ai/doors/le-tooling.md`, `ai/doors/system-plugins.md` - the doctor registries, the code metadata, the gate, the plugin codes this area's lines carry
  → Constraint: a code is declared once, in its owner's metadata or `internal/core/diagnostic/codes.go`; the gate reads the registries, never a list
- [ ] `docs/architecture/doctor-and-health-checks.md` - phases, owners, the `runtime` phase, kernel capability enrolment
  → Decision: a kernel feature a configured subsystem cannot work without is a `kernelcap.Capability` from the owning package; `/proc/modules` cannot answer it because a built-in feature lists no module
  → Constraint: a `runtime` check is handed the platform alone and reads its owner's live state, never a config tree (`RunRuntimeChecks`)
- [ ] `spec-doctor-coverage-1-audit-and-gate` - O-1, O-2, O-3, Data Flow 2a and 4b amendments
  → Decision: a report-bus code counts as covered only when its owner's check declares it; the `recent-events` check then shows each event under that code (Data Flow 2a, amended)
  → Constraint: a health row takes no code; the audit maps each row to a check by hand (Data Flow 4b)
- [ ] `plan/release/validation/spec-doctor-coverage-2-home-cpe-batch.md` - the rows it owns
  → Constraint: its codes `doctor-iface-no-backend`, `doctor-iface-name-unresolved`, `doctor-pppoe-client-no-session` are reused here, never redeclared
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: problem, why, then the fixing command on its own line (the `fix` field)
- [ ] `ai/rules/spec-no-code.md`, `ai/rules/planning.md`
  → Constraint: tables, no snippets; every open journal row is owned by a spec table

**Key insights:**
- Five report-bus codes of this area (`firewall-stale-table`, `firewall-drift`, `fib-sync-failure`, `fib-orphan`, `fib-programming-lag`) and `iface-errors` are not doctor codes; three health rows filter on them, two as string literals that copy the owner's constants
- Most transient classes keep no state today; under O-2 each becomes a report-bus raise under a doctor code its owner's check declares, read by `recent-events`
- No check of the area reads daemon memory outside the `runtime` phase (AC-9 audit: none in A8, none in A9), so this spec moves no check

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/health.go` - `CheckAllInterfaceErrors` returns 0 when `GetBackend` is nil or `ListInterfaces` fails; `checkInterfaceErrors` skips a failed `GetStats`; `checkHealth` then answers the report-bus probe, which reads healthy; raises `iface-errors` (`reportCodeIfaceErrors`)
- [ ] `internal/component/iface/config_sysctl.go` - `applySysctlProfiles` looks each unit profile up with `sysctlreg.LookupProfile` at apply, logs "unknown sysctl profile" and skips it
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - `sysctl-profile` is a `zt:node-name` leaf-list with no reference check
- [ ] `internal/component/iface/register.go` - `verifyIfaceConfig` and `parseAndVerifyIfaceSections` run the full parse at verify
- [ ] `internal/component/iface/config.go` - the pppoe-client entry parse calls `parsePPPoEClientTiming`, so a bad timing fails the verify parse
- [ ] `internal/component/iface/pppoe_client_timing.go` - `parsePPPoEClientTiming` runs `Validate`; `reconnectDelay` re-validates and errors only on a timing `Validate` refuses
- [ ] `internal/component/iface/pppoe_client.go` - `run` logs at Error "the redial wait is invalid" and stops on a `reconnectDelay` error
- [ ] `internal/component/firewall/audit.go` - `AuditTables` raises `firewall-stale-table` and `firewall-drift` and returns an error when it cannot read the kernel (`errAuditNoBackend`, `ListTables`)
- [ ] `internal/plugins/firewall/nft/health.go` - `checkFirewallHealth` answers degraded on an audit error or timeout, then filters warnings on the literals `firewall-stale-table` and `firewall-drift`
- [ ] `internal/plugins/firewall/nft/doctor_linux.go` - `checkFirewallNftables` warns when `kernelcap.LoadedModules` (`/proc/modules`) lacks `nf_tables`
- [ ] `internal/component/firewall/plugins/irr/register.go` - `registerIRRDoctor` declares `Codes` stale and no-data only
- [ ] `internal/component/firewall/plugins/irr/doctor.go` - `checkIRRDataFreshness` emits no-data, oversized and stale; returns nil when the store or cache cannot be opened
- [ ] `internal/component/firewall/plugins/irr/irr.go` - `refreshAllNow` logs "refresh failed, keeping the cached prefixes" on a lookup error and counts outcome `error`
- [ ] `internal/component/resolve/irr/store/store.go` - `refreshLive` calls `markStale` on a lookup error as well as on an empty answer, so a failing refresh does mark the kept entry stale
- [ ] `internal/plugins/fib/kernel/fibkernel.go` - report codes `fib-sync-failure`, `fib-orphan`, `fib-programming-lag`; `pending` map, `fibLagTimeout`, `maxPendingEntries`
- [ ] `internal/plugins/fib/kernel/register.go` - `init` registers health row `fib` with `report.HealthProbeDegraded` over the three literals
- [ ] `internal/core/report/health_probe.go` - `HealthProbeDegraded` reads active warnings by code
- [ ] `internal/component/kernelcap/kernelcap.go` - `Capability`, `MustRegister`: owner-registered kernel requirement with `CodeAbsent`, `CodeUnknown`, `InUse`, `Probe`
- [ ] `internal/component/doctor/checks_recent_events.go` - `recentEventsCheck` (runtime) shows every report-bus event under its own code when that code has metadata
- [ ] `internal/component/doctor/registry.go` - `RunRuntimeChecks` runs the `runtime` phase with the platform alone
- [ ] `internal/core/diagnostic/codes.go` - code metadata, `doctor-firewall-nftables` among it

**Findings at the producer (2026-11-10):**

| Claim from the slice | Verdict | Evidence |
|----------------------|---------|----------|
| `CheckAllInterfaceErrors` reads healthy when it read nothing | confirmed | `health.go` `CheckAllInterfaceErrors`, `checkHealth` |
| Unknown `sysctl-profile` accepted at commit | confirmed | the YANG leaf has no reference check; only `applySysctlProfiles` looks it up |
| Invalid PPPoE redial wait accepted at commit | refuted | `parsePPPoEClientTiming` runs `Validate` inside the verify parse, so the commit refuses it; the `run` branch is reachable only by a defect |
| IRR `Codes` omits the oversized code | confirmed | `registerIRRDoctor` against `checkIRRDataFreshness` |
| IRR refresh errors never mark entries stale | refuted | `refreshLive` calls `markStale` on a lookup error; `checkIRRDataFreshness` then reports `doctor-firewall-irr-stale-data` |
| `firewall-nftables` wrong for a built-in nf_tables | confirmed | `/proc/modules` lists no built-in module; Ze's kernel config carries `CONFIG_NF_TABLES=y` |

**Behavior to preserve:**
- every existing check and code of the area: `iface-ethernet`, `iface-macvlan`, `iface-ra-forwarding`, `sysctl-procfs`, the five VPP checks, `kernel-nexthop`, the MPLS capabilities, `policyroute-netlink`, `firewall-domain-group-data`, `firewall-irr-data-freshness`
- the health rows `iface`, `firewall`, `fib`, `vpp` and their healthy and degraded meaning
- `doctor-firewall-nftables` stays the code for "nf_tables absent"

**Behavior to change:**
- one check, or one more code on an existing check, per gap class below
- six report-bus codes renamed to doctor codes, and the health rows that filter on them
- every Warn and Error line of the area names its code

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Doctor check registry and phases | `diagnostic.RegisterDoctorCheck`; `RunRuntimeChecks` (`internal/component/doctor/registry.go`) | owner-registered checks per phase; `runtime` runs only in the daemon | extends: every new check registers here from its owner, `runtime` for live state, `post-config` for config plus host state |
| Plugin doctor checks | `registry.DoctorCheckDef` (plugin registry) | a plugin declares checks in its `Registration` | extends: an owner that is a plugin (fib-kernel, fib-vpp, policyroute, iface-dhcp, iface-ra) declares its checks there, so they also answer when it runs forked |
| `Diagnostic.Fix` | `diagnostic.Diagnostic.Fix` | the fixing command, drawn on its own line | extends: every new diagnostic sets it (child 1 AC-7) |
| Report bus | `report.RaiseWarning`, `RaiseError`, `ClearWarning` | coded active warnings and an error ring | extends: a transient class raises under a doctor code; the six non-doctor codes are renamed |
| `recent-events` check | `recentEventsCheck` (`internal/component/doctor/checks_recent_events.go`) | one row per bus event under the event's code when that code has metadata | extends: a transient class's code is declared by its owner's check and registered with metadata, so the row carries it |
| Health registry | `health.Register`; `report.HealthProbeDegraded` (`internal/core/report/health_probe.go`) | rows `iface`, `firewall`, `fib`, `vpp` with reason, no code | extends: filters key on the renamed codes through the owner's constants; the `iface` row stops reading an unread kernel as healthy |
| Doctor-coverage gate | `./le arch doctor-coverage` | counts a site covered when its code is declared by a registered check | extends: every site of the area becomes covered; no gate change |
| Kernel capability enrolment | `kernelcap.Capability`, `MustRegister` (`internal/component/kernelcap/kernelcap.go`) | owner-declared kernel requirement with absent and unknown codes and a probe | REPLACES the `/proc/modules` check `checkFirewallNftables` (`ai/rules/no-layering.md`): the module-list check is deleted first, then nf_tables is enrolled |
| Existing area checks | `iface-ethernet`, `iface-macvlan`, `iface-ra-forwarding`, `sysctl-procfs`, `vpp-api-socket`, `vpp-hugepages`, `vpp-cpu-isolation`, `vpp-dpdk`, `kernel-nexthop`, `policyroute-netlink`, `firewall-domain-group-data`, `firewall-irr-data-freshness`, MPLS capabilities | config plus host state per check | extends: `iface-ethernet` walks every configured kind (AC-46); `vpp-dpdk` gains the left-bound case (AC-92); `firewall-irr-data-freshness` declares its third code (AC-73) |
| Firewall audit | `AuditTables` (`internal/component/firewall/audit.go`) | compares kernel `ze_` tables with the last applied set; raises stale and drift | extends: a `runtime` check calls it and answers stale, drift and could-not-run rows |
| fib-kernel pending tracking | `pending`, `checkPendingLagLocked` (`internal/plugins/fib/kernel/fibkernel.go`) | raises sync-failure and lag warnings | extends: a `runtime` check reports the pending set; fib-vpp gains the same record (AC-5) |
| Interface status producers | `PPPoEClient.status` (`internal/component/iface/pppoe_client.go`), `lastRegistryReconcile` | `show interface pppoe-client`; reconcile status read by as112 | extends: the DHCP client gains the same kind of status producer (AC-4); the registry status gets a doctor reader (AC-11) |
| Read root of the iface verify | `parseResolvedIfaceSections`, `candidateCoSProfiles` (`internal/component/iface/config.go`) | resolves a unit's class-of-service name against the candidate | extends: the same read-root route resolves `sysctl-profile` names against the candidate's `sysctl { profile }` (AC-16) |
| Owner-registered code metadata | `diagnostic.Register` from an owner (`irrDiagnosticCodes`), or `internal/core/diagnostic/codes.go` | `ze show diagnostic <code>` explains a code | extends: each new code is registered by its owner where the owner already registers metadata, else in `codes.go` |
| Persisted Ze-owned state | `pkg/zefs` keys `KeyTrafficTCSnapshot`, `KeyFirewallDomainGroup`, the IRR prefix store | owned state survives a restart | extends: the left-behind and mismatch checks read it with `storage.OpenReadOnly`, as the IRR and domain checks already do |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze doctor [--json] [config <file>]` locally; `show doctor` on SSH, MCP, REST, gRPC and web
- `./le arch doctor-coverage check` and `report` for the line codes

### Transformation Path
1. Local-route checks (`post-config`) read config plus sysfs, procfs, netlink, the zefs store or `ip rule` and `ip route` reads; they apply nothing
2. Runtime checks (`runtime`) read the owner's live state (status producers, pending maps, last-outcome records, the report bus for an in-process owner) and run only in the daemon; the local route asks the daemon (O-1)
3. A transient class raises on the report bus under its doctor code; `recent-events` shows it with its subject, count and time
4. Each diagnostic carries code, severity, message and `fix`; `show doctor` returns it on every surface

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin to doctor | `Registration.DoctorChecks` callback, in process or over RPC | Yes: `firewall-irr` and the iface plugins already declare checks |
| Component to doctor | `diagnostic.RegisterDoctorCheck` in the daemon process | Yes: `iface-ethernet` |
| Producer to report bus | `report.RaiseWarning` and `RaiseError` in process | Yes; a forked producer's raise never arrives (forwarding door), so a bus-reading check is only for an in-process owner (R-1) |

### Changed-Wire Census

**Search evidence:** scope `internal/`, `test/`, `docs/`; terms `firewall-stale-table`, `firewall-drift`, `fib-sync-failure`, `fib-orphan`, `fib-programming-lag`, `iface-errors`. Positive control: `AuditTables` raises `firewall-stale-table` (read above). Found: producers `audit.go`, `fibkernel.go`, `health.go`; filters `firewall/nft/health.go`, `fib/kernel/register.go`, `iface/health.go`; tests `fibkernel_test.go` and `integration_linux_test.go` (fib kernel), `firewall/nft/health_test.go`, `firewall/audit_test.go`, `core/report/health_probe_test.go`; the `.ci` file `test/static/static-interface-nexthop-pppoe-follows-device.ci`; pages `docs/architecture/core-design.md`, `docs/architecture/static-routes.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/guide/health-checks.md`; comments in `sysrib.go`, `backend_linux.go` (fib kernel), `static/locrib.go`. `show doctor` JSON: writer `diagnostic.Diagnostic`, reader `outputJSON` and `outputText` in the doctor component (read in child 1).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `show warnings` / `show errors` code field | writer | `AuditTables` (`firewall/audit.go`), the fib-kernel raises (`fibkernel.go`), `checkErrorsFromStats` (`iface/health.go`) | constants `reportCodeFirewall*`, `reportCodeFIB*`, `reportCodeIfaceErrors` | change: each constant takes its `doctor-` name; pre-release, no compatibility alias (`ai/META-DECISIONS.md`, "Ze is pre-release") |
| same | reader | `checkFirewallHealth` (`firewall/nft/health.go`), the `fib` row in `fib/kernel/register.go`, `healthErrorProbe` (`iface/health.go`) | literal lists and constants | change: read the owner's exported constant, one declaration |
| same | reader | the tests and `.ci` file in the search evidence; an operator script outside the repository that reads `show warnings` | assertion strings | change the in-tree readers in the same commit; an outside reader of an unreleased code is owed no alias (pre-release) |
| `show doctor` JSON | writer | the new checks, through `diagnostic.Diagnostic` | keys unchanged | preserve: rows are added, no key changes |
| `show doctor` JSON | reader | `outputJSON`, `outputText` (doctor component); API clients on MCP, REST, gRPC, SSH; the `test/plugin/*doctor*.ci` files | rows are data; no key added | preserve: a reader that lists rows sees more rows, none renamed |
| `show interface dhcp-client` (new) | writer | the DHCP client status producer (AC-4) | new output | new: kebab-case keys, declared once in its YANG |
| `show interface dhcp-client` (new) | reader | `test/plugin/show-doctor-dhcp-client-no-lease.ci`; API clients | new output | new: no existing reader |

### Integration Points
- `diagnostic.RegisterDoctorCheck` and `Registration.DoctorChecks` in each owner
- `report.RaiseWarning` and `RaiseError` in each producer of a transient class
- `kernelcap.MustRegister` in the nft backend

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | names resolve through the iface resolver; checks read owner state, never another component's internals |
| No unintended coupling (components stay isolated) | Yes | each check lives in the package that owns the state |
| No duplicated functionality (extends existing, does not recreate) | Yes | Existing Mechanisms; the nft module check is replaced, not kept beside a capability |
| Zero-copy preserved where applicable (refs, not copies) | Yes | no hot path; checks read snapshots |
| Registration over hardcoding, outbound | Yes | every check and code registers from its owner |
| Registration over hardcoding, inbound | Yes | searched: the health-row filters in `firewall/nft/health.go` and `fib/kernel/register.go` were literal lists; they now read the owner's constants; the gate and `recent-events` derive from the registries |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | fib-kernel, firewall, iface and sysrib raise on the report bus from the daemon process | forwarding door: every nft producer runs in process; fib-kernel is the netlink data-plane writer | a bus-reading runtime check reads an empty bus for a forked owner | each phase reads the owner's `Registration` for a forked mode; a forkable owner keeps its record in its own state and declares a plugin doctor check | unvalidated |
| A-2 | Every fix command named in an AC exists with that grammar | the command registry | the fix line names a command that does not exist | each check's unit test resolves its fix through the command registry or the YANG schema | unvalidated |
| A-3 | An nfnetlink request answers presence of nf_tables, and answers unknown without `CAP_NET_ADMIN` | the kernel's nfnetlink receive path checks the capability | the capability reads absent for an unprivileged run | `TestNftablesCapabilityProbe` under QEMU, privileged and unprivileged | unvalidated |
| A-4 | The doctor storage checks cover the resolve store directory | `checkIRRDataFreshness` returns nil, "the storage check reports unavailable or unsafe stores" | an unreadable IRR or domain cache shows no row anywhere | read the storage checks' paths in phase P4 | unvalidated |
| A-5 | The canonical code table holds no names for the A8 gap rows | the merge's table lists only `doctor-iface-errors` and `doctor-iface-no-backend` for A8 | names collide with another child's | the merge agent adds this spec's names to the inventory table | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A runtime check over the report bus reads healthy for a forked owner | the plugin runs external in a test | A-1; the owner keeps its own record and answers through its plugin doctor check |
| R-2 | A left-behind check counts state another owner installed | a row names a DHCP route or a foreign nft table | count only Ze-owned state: `rtm_protocol` 250 and 251, `ze_` tables, `ze/` VPP tags, the persisted qdisc snapshot |
| R-3 | A left-behind check runs while the daemon runs and its state is legitimate | rows on a healthy box | local-only checks that judge left-behind state run only when no daemon answers (O-1 fallback) |
| R-4 | A pending or lag check fires during a normal convergence | rows during startup | thresholds are the owner's own (`fibLagTimeout`, DHCP and RA protocol timers), never a new constant |
| R-5 | Renaming report codes breaks a test or page that reads them | red tests listed in the census | the rename phase converts every reader in the census in the same commit |
| R-6 | A recent-event code a check declares but never answers with a live row confuses a reader | `ze show diagnostic` lists a code no row shows | the code metadata says it is shown by `recent-events` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | false or missing doctor rows; the `iface`, `firewall`, `fib` health rows change their filter; `show warnings` shows new code names; a unit naming an unknown sysctl profile is refused at commit |
| How is it reverted? | one commit per phase |
| Who else touches this path? | child 2 (iface backend, names, PPPoE, STP); `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` (P0a plugin-stopped and setup codes); `plan/release/validation/spec-doctor-coverage-5-appliance.md` (learned resolvers, the dns-source codes); the storage owner (store checks) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show doctor` with a `ze_` table removed by `nft` | → | firewall runtime audit check | `test/firewall/show-doctor-firewall-drift.ci` (`option=needs-linux`) |
| `show doctor` with a route the kernel refuses | → | fib-kernel runtime sync check | `test/plugin/show-doctor-fib-sync-failure.ci` (`option=needs-linux`) |
| `ze doctor --json` with `firewall { }` on a kernel with nf_tables built in | → | nf_tables capability | `TestNftablesCapabilityProbe` (QEMU), `test/ui/doctor-firewall-nftables.ci` |
| `ze config validate` with an unknown `sysctl-profile` | → | iface verify | `test/parse/iface-sysctl-profile-unknown.ci` |
| `show doctor` on a unit whose DHCP server never answers | → | DHCP client status and check | `test/plugin/show-doctor-dhcp-client-no-lease.ci` (`option=needs-linux`) |
| `show health` with no interface backend | → | `checkHealth` degraded with cause | `TestIfaceHealthDegradedWhenCountersUnread` |
| `show doctor` with a sysctl key rewritten outside Ze | → | sysctl runtime check | `test/plugin/show-doctor-sysctl-not-applied.ci` (`option=needs-linux`) |
| `show doctor` with fib-vpp and no VPP | → | fib-vpp writer check | `TestFIBVPPNoWriterDoctor` |
| `./le arch doctor-coverage report` | → | every forwarding site resolves | `TestForwardingSitesCovered` |

## Acceptance Criteria

Every row below holds for every check it names: the message states the cause in plain words, `fix` holds the command alone, the row reaches every surface through `show doctor`, the unit test goes red when the detection is removed, and the check's `Codes` declares every code it emits. Route is `local` (`post-config`, both routes) or `runtime` (daemon only). A "recent event" code is raised on the report bus by its owner, declared by its owner's check, and shown by `recent-events`. Code names for A8 rows are chosen here (A-5).

### Shared mechanisms (before the checks that need them)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | The report-bus codes `iface-errors`, `firewall-stale-table`, `firewall-drift`, `fib-sync-failure`, `fib-orphan`, `fib-programming-lag` | each is raised as `doctor-iface-errors`, `doctor-firewall-stale-table`, `doctor-firewall-drift`, `doctor-fib-sync-failure`, `doctor-fib-orphan`, `doctor-fib-programming-lag`, with code metadata registered; the `iface`, `firewall` and `fib` health rows filter on the owner's exported constants, so each code is declared once; every reader in the Changed-Wire Census is converted in the same commit | none: two names for one code is the copy the principles forbid |
| AC-2 | The `firewall-domain` and `firewall-irr` code-registration failure lines log a `code` attribute holding the code being registered | that attribute is renamed `registering`, and the line carries `code` `doctor-plugin-setup-failed`, so the gate reads it as covered and not code-dynamic | none |
| AC-3 | No interface backend is loaded, or `ListInterfaces` or `GetStats` fails, while the `iface` health row is read | the row answers degraded with the cause ("interface error counters not read: ..."); `CheckAllInterfaceErrors` returns the read error beside its count (journal row `zero-value-as-valid-answer`, 2026-11-10) | healthy stays the answer when the counters were read and none rose |
| AC-4 | A configured DHCPv4 or DHCPv6 client on a unit | the DHCP client keeps a status per unit (worker phase, lease and its expiry, address installed, default route pending, last error, attempts), served by `show interface dhcp-client` and read by the DHCP checks | none: the DHCP checks need a producer, as child 2's PPPoE checks read `PPPoEClient.status` |
| AC-5 | fib-vpp fails to add, replace or delete a route or MPLS route | fib-vpp keeps a pending record per prefix (first failure time, last error) as fib-kernel does, and raises `doctor-fib-vpp-sync-failure` on the report bus; a later success clears it | none: AC-99 has no state to read without it |
| AC-6 | An iface or policyroute apply fails and its rollback also fails | the owner keeps the last rollback outcome (time, what was left, errors) until the next successful apply, readable by its runtime check | none: AC-10 and AC-88 need it |

### Interfaces, DHCP and RA

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-7 | A bridge `member` or a unit mirror destination whose selector resolves to no device | child 2's name-resolver check also walks bridge members and mirror destinations and reports `doctor-iface-name-unresolved` (error, runtime) naming the bridge or unit, the selector and what the kernel has; fix names `show interface` and the selector leaf | a ppp device a pppoe-client owns while that client is down (child 2 AC-7 reports it) |
| AC-8 | `Backend.ListInterfaces` fails during apply, so every ethernet binding is deferred | `doctor-iface-list-failed` (error, runtime) with the backend error; fix names `show interface` and `ze status` | none |
| AC-9 | A committed interface config is owed to the data plane (`applyIsOwed`: backend not ready, VPP reconnect apply failed, reload or monitor start failed) | `doctor-iface-apply-owed` (error, runtime) naming the cause and since when; fix names the reload command that re-applies interface config | an apply owed for less than the VPP reconnect window, the normal reconnect |
| AC-10 | AC-6 holds a failed iface rollback | `doctor-iface-rollback-partial` (error, runtime) naming what the data plane still holds of the refused config; fix names the commit that re-applies the running config | none |
| AC-11 | The last owner-registration address reconcile (`RegistryReconcileStatus`, DHCP, PD, AS112 owners) failed | `doctor-iface-address-reconcile-failed` (warning, runtime) naming owner, unit and error | none |
| AC-12 | An existing tunnel netdev was adopted and its kernel attributes (`GetLink`) differ from the tunnel config | `doctor-iface-tunnel-adopted-mismatch` (warning, runtime) naming the tunnel and each differing attribute; fix names `delete interface tunnel <name>` then commit | none |
| AC-13 | A plugin-owned macvlan drifted (MAC, parent, MTU, ownership alias) and was recreated | recent event `doctor-iface-owned-device-recreated` naming the device and what drifted | none |
| AC-14 | A configured port mirror is not installed, not torn down, or half installed (`ListMirrors` against the desired `mirrorSpec`; clsact or filters left after rollback) | `doctor-iface-mirror-not-installed` (error, runtime) naming source, direction and destination; the mirror-site lines carry it | none |
| AC-15 | A unit sets `ipv6 forwarding true` and `net.ipv6.conf.all.forwarding` is not 1 | `doctor-iface-ipv6-forwarding-off` (error, local) naming the unit; fix names `show sysctl` and the forwarding leaf | none |
| AC-16 | A unit's `sysctl-profile` names a profile neither built in nor in the candidate's `sysctl { profile }` | the commit and `ze config validate` refuse it, naming the unit, the profile and the profiles that exist (journal row `validate-accepts-what-start-refuses`, 2026-11-10); a running config that already holds one reports `doctor-iface-sysctl-profile-unknown` (error, local) with fix `delete interface ... sysctl-profile <name>` | a built-in or candidate profile commits as today |
| AC-17 | Config uses the deprecated `ipv4 rp-filter` leaf | `doctor-iface-rp-filter-deprecated` (warning, local) naming the unit and the replacing leaf as the fix | none |
| AC-18 | An offload, RPS or RFS setting is not in effect (ethtool feature, `hw-tc-offload`, `rps_cpus`, `rps_flow_cnt`, global `rps_sock_flow_entries`) | `doctor-iface-offload-not-applied` (warning, local) naming device, setting, configured and kernel value | a NIC that does not offer the feature reads as not supported, not as not applied |
| AC-19 | An interface's rx or tx error counters rose since the last read | the iface runtime check declares `doctor-iface-errors` and answers one warning per interface with the deltas; fix names `show interface <name>` | none |
| AC-20 | A delegated prefix is not applied to a consuming unit (too small for its subnet-id, no device, address held by another owner, install or removal failed, invalid prefix, no receive time) | `doctor-iface-delegated-prefix-unapplied` (error, runtime) naming unit, prefix and reason, from `delegatedState` | a prefix not received at all is child 2's `doctor-pppoe-client-pd-not-received` |
| AC-21 | An RA sender for a configured unit failed to start (desired against active senders in the RA reconcile) | `doctor-ra-sender-not-running` (error, runtime) naming the unit and the error | none |
| AC-22 | A LAN advertises Router Lifetime 0 or stale default-route state (WAN not resolved, route watcher error, existing defaults unread) | `doctor-ra-default-route-unknown` (warning, runtime) naming the LAN, the WAN and why | Router Lifetime 0 when the WAN truly has no default route is the correct advertisement, not a row |
| AC-23 | An RA withdrawal was dropped or an advertisement could not be encoded | recent event `doctor-ra-send-failed` naming unit and what hosts keep until its lifetime ends | none |
| AC-24 | A configured RDNSS resolver is not advertised (host-only, link-local of another link, beyond option capacity) | `doctor-ra-rdnss-not-advertised` (warning, runtime) naming unit, resolver and reason, from `raRDNSSSelection` | none |
| AC-25 | RDNSS selections are not published to system DNS, or the system-dns snapshot was never requested | the canonical `doctor-dns-source-consumer-stale` (warning, runtime), declared by the `dns-source-selection` check of `plan/release/validation/spec-doctor-coverage-5-appliance.md`; this spec supplies the comparison of `raPublished` with the current selections and registers no code of its own for the class | none |
| AC-26 | A learned resolver (RA RDNSS, DHCPv4, DHCPv6, PPPoE IPCP) is not in the resolver lease registry or `/etc/resolv.conf` | `doctor-iface-learned-dns-unwritten` (warning, runtime) naming source and servers | when the system-DNS child declares a code for the same state, this row uses that code and this AC owns only the iface and dhcp lines (A-5) |
| AC-27 | A received RA user option (RDNSS) is malformed or for an unknown link | recent event `doctor-ra-option-discarded` naming link and reason | none |
| AC-28 | The RA user-option reader stopped, or the kernel dropped options | `doctor-ra-option-reader-stopped` (error, runtime) from a liveness flag the reader keeps; fix names the restart that restarts the reader | none |
| AC-29 | An RA default route at its configured route-priority is not installed, or `accept_ra_defrtr` suppression or restore is not in effect | `doctor-ra-default-route-not-installed` (warning, runtime) comparing the priorities and suppressed maps with the kernel `::/0` and the sysctl store | none |
| AC-30 | A configured DHCP client is not running (creation failed, dhcp-auto bring-up failed, DHCPv6 off for an unreadable DUID device, unusable configured DUID, no link-layer address, interface lookup failed) | `doctor-dhcp-client-not-running` (error, runtime) from AC-4's status; a configured DUID that does not parse is also reported locally under the same code | none |
| AC-31 | A running DHCP client holds no lease (no answer, NAK or refused lease, renewal ignored, DHCPv6 lease expired, address declined as duplicate) | `doctor-dhcp-client-no-lease` (error, runtime) with phase, last error and since when | a client within its first request exchange is not reported |
| AC-32 | A lease is held but its address is not installed (not this client's, replace failed, registration refused, reinstall after link-up failed) | `doctor-dhcp-client-address-not-installed` (error, runtime) naming unit, address and reason | none |
| AC-33 | A DHCPv4 default route install is pending and retrying | `doctor-dhcp-client-route-pending` (warning, runtime) with the gateway and the last error | none |
| AC-34 | A DHCPv4 ACK carried no subnet mask and /24 was assumed | recent event `doctor-dhcp-client-mask-assumed` naming the server and the assumed prefix | none |
| AC-35 | `backend vpp` with a DHCPv6 client (VPP reports no DAD outcome) | `doctor-dhcp-client-dad-unreported` (warning, local) saying a duplicate lease address cannot be detected on this backend | none |
| AC-36 | `plugin { external iface-dhcp }` | the refusal line carries `doctor-plugin-external-unsupported`, declared by child 12's check (`spec-doctor-coverage-12-plugin-infra.md` phase P0a) | none |
| AC-37 | The PPPoE client `run` branch for an invalid redial wait (unreachable from a committed config, see Findings) | the line carries child 2's `doctor-pppoe-client-no-session` and its message says the timing passed verify, so a defect reached it | none |
| AC-38 | A PPPoE IPv4 default route was not added, or the old one not removed | `doctor-pppoe-client-ipv4-route-not-applied` (error, runtime) naming device, gateway and error | the IPv6 default route is child 2's |
| AC-39 | PPPoE session device IPv6 sysctls (source `pppoe-client:<name>`) are not applied or not reset | `doctor-pppoe-client-sysctl-unapplied` (warning, runtime) naming device, key, wanted and kernel value | none |
| AC-40 | An interface migration left the old address on the source interface | recent event `doctor-iface-migrate-address-left` naming both interfaces and the address; fix names `delete interface ... address` | none |
| AC-41 | An unnumbered borrowed /32 differs from its donor's addresses | `doctor-iface-unnumbered-unreconciled` (warning, runtime) naming unit and donor | none |
| AC-42 | A route Ze deleted is still held by the kernel under another protocol, or its holder is unknown | recent event `doctor-iface-route-held-by-other` naming prefix and protocol | none |
| AC-43 | Interface event monitoring degraded (link cache seed failed, address or neighbor watch ended, resolver subscriber overflowed) | `doctor-iface-monitor-degraded` (error, runtime) naming which watch and since when, from monitor state and `countResolverEventDropped` | none |
| AC-44 | An `ip6` rpf-check nft table of the netlink backend may remain and an unprivileged run cannot confirm or remove it | `doctor-iface-rpf-table-stale` (warning, local) naming the table; unprivileged it reads cannot determine with the reason | none |
| AC-45 | A link created and then rolled back left a netdev (macvlan, tunnel, wireguard, doctor probe macvlan) | `doctor-iface-netdev-left` (warning, runtime) naming the device, counted only for kinds and names Ze creates | R-2: a netdev another owner created is never counted |
| AC-46 | A configured bridge, veth, dummy, tunnel, wireguard or vlan is absent or down in the kernel | `iface-ethernet` becomes a walk of every configured kind and reports the existing `doctor-iface-missing` (absent) and a new `doctor-iface-down` (configured up, kernel down) | a kind the backend does not create (VPP-only kinds on netlink) is skipped with the reason |
| AC-47 | A configured static address failed DAD or stays tentative | `doctor-iface-address-dad-failed` (error, runtime) from `ReadAddressState` | on VPP (`DADReported` false) the row reads cannot determine (forwarding door trap) |
| AC-48 | A wireguard peer has no handshake within its keepalive bound | `doctor-iface-wireguard-no-handshake` (warning, local) naming interface and peer from the kernel peer state | a peer with no endpoint configured that never initiated is not reported |
| AC-49 | Interface-area lines whose class is already covered: `iface-ethernet` sites | each line carries `doctor-iface-missing`, `doctor-iface-selector-unmatched` or `doctor-iface-selector-ambiguous` | none |

### sysctl and the system RIB

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-50 | A managed sysctl key is not at its effective value (a default, set, rollback, restore, clear-source or clear-profile write failed; a rewrite and re-apply failed) | `doctor-sysctl-not-applied` (error, runtime) per key: effective value, source, kernel value; fix names the command that re-applies sysctl | a key the kernel refuses for this host (absent path) reads not supported |
| AC-51 | A runtime `(sysctl, set)` was refused (claimed by config, write guard, invalid) and the caller saw no error | recent event `doctor-sysctl-set-refused` naming key, caller and reason | none |
| AC-52 | Sysctl profiles applied to one interface conflict (`sysctlreg.CheckConflicts`) | `doctor-sysctl-profile-conflict` (warning, local) naming interface, profiles and key | none |
| AC-53 | A component's required sysctl default is overridden by config or overwritten by another plugin's default | `doctor-sysctl-default-overridden` (warning, runtime) naming key, the requiring component and the overriding source | none |
| AC-54 | A protocol's routes reach the FIB with no FIB import declaration or no declared administrative distance | `doctor-sysrib-protocol-undeclared` (error, runtime) naming the protocol, from `fibSpoken`, `distanceSpoken` | none |
| AC-55 | A best change or replay was not delivered to the FIB writer | recent event `doctor-sysrib-fib-undelivered` naming the batch and error | none |
| AC-56 | The Loc-RIB change channel overflowed and prefixes were re-read | recent event `doctor-sysrib-feed-overflow` with the count | none |
| AC-57 | An internal plugin's `Run` returned (iface, sysctl, sysrib, iface-ra, iface-dhcp, firewall, firewall-domain, firewall-irr, traffic, vpp, fib-kernel, fib-p4, fib-vpp, policyroute) | each line carries `doctor-plugin-stopped`, declared by child 12's check (`spec-doctor-coverage-12-plugin-infra.md` phase P0a) | none |
| AC-58 | Not problem classes: the sysrib commit-refusal lines (the operator reads the refusal at commit) | each line is logged at Info, since the refusal already reached the operator with its fix | none: a refusal returned to the committer is not a lasting state |

### Firewall and traffic control

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-59 | `ze_` tables in the kernel the applied config does not want | a `runtime` firewall check calls `AuditTables` and declares `doctor-firewall-stale-table` (warning) naming each table; fix names the commit that re-applies the firewall | none |
| AC-60 | A `ze_` table differs from config or vanished (`ze_vrrp` gone removes the RFC 9568 accept filter) | the same check declares `doctor-firewall-drift` (error) naming each table | none |
| AC-61 | The audit cannot run (tables applied, no backend, `ListTables` error, kernel call over 1 s) | `doctor-firewall-audit-failed` (error, runtime) with the audit error | none: a failed audit is never read as a clean one |
| AC-62 | The dataplane reconcile failed or timed out, so the kernel ruleset is behind the table registry (owner tables, IRR and domain sets, copp, policy routes) | `doctor-firewall-apply-behind` (error, runtime) from the last `ApplyAll` error and the apply-timeout counter, naming the tables behind | none |
| AC-63 | A table is held back because a rule names a set no owner registered | `doctor-firewall-table-held-back` (error, runtime) naming table and set, from the registry's held-back set | none |
| AC-64 | `firewall global-options` sysctl defaults were not extracted or not emitted | `doctor-firewall-global-options-unapplied` (warning, runtime) comparing the sysctl default layer (source `firewall-global-options`) with `ExtractGlobalOptions` | none |
| AC-65 | `ze_` nft tables are in the kernel while no daemon runs and `flush-on-shutdown` is true | `doctor-firewall-tables-left-after-stop` (warning, local, only when no daemon answers) naming the tables; fix names `nft delete table` for each | R-3: with a daemon running the tables are its own |
| AC-66 | `firewall { }` on the nft backend | nf_tables is a `kernelcap.Capability` of the nft backend: absent reports `doctor-firewall-nftables` (error), unknown reports `doctor-firewall-nftables-unknown` (warning, with the probe's reason); `checkFirewallNftables` and its `/proc/modules` read are deleted first (journal row `probe-inability-read-as-absence`, 2026-11-10) | a built-in nf_tables reads present, with no row |
| AC-67 | A domain group answer exceeds the per-name address cap | the domain plugin stores the truncation per name; `doctor-firewall-domain-group-truncated` (warning, runtime) naming group, name and the kept count | none |
| AC-68 | A domain cache or change-log write fails on a healthy store | recent event `doctor-firewall-domain-cache-write` naming the group and error | a broken store is the storage checks' row (A-4) |
| AC-69 | The IRR or domain cache cannot be opened for the doctor read | a row names the store path and the cause, from the storage checks when they cover the resolve store directory, else from the IRR and domain checks under the storage owner's `doctor-store-integrity` | none: a check that cannot look says so |
| AC-70 | A persisted tc original-qdisc snapshot does not match the box (boot id, name, ifindex, MAC) or the store is unreadable | `doctor-traffic-tc-snapshot-mismatch` (error, local) naming interface and the mismatched field; fix names the command that clears the snapshot | none |
| AC-71 | Ze's qdisc is still on an interface while no daemon runs (`KeyTrafficTCSnapshot` non-empty) | `doctor-traffic-qdisc-not-restored` (warning, local, only when no daemon answers) naming interface and original qdisc | R-3 |
| AC-72 | Firewall-area lines whose class is already covered: domain-group no-data and stale, IRR no-data and stale (the refresh-failed line included, see Findings) | each line carries that code | none |
| AC-73 | `firewall-irr-data-freshness` emits `doctor-firewall-irr-oversized-data` | its `Codes` declares it, and a test asserts every code the check emits is declared (journal row `declared-format-contradicts-payload`, 2026-11-10) | none |

### FIB writers and policy routing

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-74 | fib-kernel's route netlink handle could not open (`failedBackend`) | `doctor-fib-kernel-netlink` (error, runtime); a `post-config` probe opens route netlink when fib-kernel is the writer and reports the same code locally | none |
| AC-75 | The private MPLS context guard `ip rule` is missing per family | `doctor-fib-kernel-mpls-guard` (error, local) naming family, priority and mark | none |
| AC-76 | fib-kernel is loaded and `net.ipv4` or `net.ipv6` `conf.all.forwarding` is 0 with no sysctl override | `doctor-fib-kernel-forwarding-off` (error, local and runtime) naming the family | an operator `sysctl` override to 0 is reported as config, at warning |
| AC-77 | A route sysrib selected is not installed (add, replace or delete failed; pending over `fibLagTimeout`; replay lost; pending cap reached) | a `runtime` fib-kernel check declares `doctor-fib-sync-failure` and `doctor-fib-programming-lag`, naming prefix, error and age from `pending` | a route pending less than `fibLagTimeout` reports only sync-failure, not lag |
| AC-78 | Orphan Ze routes (protocol 250) were swept at start | recent event `doctor-fib-orphan`, declared by the AC-77 check | none |
| AC-79 | Ze-owned kernel routes (protocol 250) remain while no daemon runs, or after the sweep window | `doctor-fib-kernel-routes-left` (warning, local) counting protocol 250 routes; fix names `ip route flush proto 250` | R-3 |
| AC-80 | MPLS is in use, fib-kernel started, and `net.mpls.platform_labels` is still 0 | `doctor-mpls-label-space-empty` (error, runtime) | none |
| AC-81 | Another program overwrote a Ze route and the re-assert failed | recent event `doctor-fib-kernel-reassert-failed` naming prefix and error; the external-change emit lines carry it | none |
| AC-82 | The kernel route monitor gave up | `doctor-fib-kernel-monitor-stopped` (error, runtime); fix names the restart | none |
| AC-83 | An MPLS label entry (push, swap, pop) requested by `mpls-fib` is not installed | `doctor-fib-kernel-mpls-install` (error, runtime) naming label and error | none |
| AC-84 | An MPLS label entry was not removed | `doctor-fib-kernel-mpls-remove` (warning, runtime) naming label | none |
| AC-85 | `fib p4` is configured (no P4Runtime backend exists, so every route reads installed while nothing is programmed) | `doctor-fib-p4-no-backend` (error, local) | none |
| AC-86 | Policy-route `ip rule` entries and tables Ze allocated remain while no daemon runs (cleanup could not open netlink, rule not encodable, `RuleDel` errors ignored) | `doctor-policyroute-rules-left` (warning, local) naming marks and tables; the ignored `RuleDel` error is logged under it | R-3 |
| AC-87 | Lines whose class is already covered: MPLS capability sites, `policyroute-netlink` | each line carries `doctor-mpls-transit-mtu-unenforced`, `doctor-mpls-transit-mtu-unknown`, `doctor-mpls-unavailable`, `doctor-mpls-unknown` or `doctor-policyroute-netlink` | none |
| AC-88 | AC-6 holds a failed policyroute rollback | `doctor-policyroute-rollback-partial` (error, runtime) naming the rules and tables left | none |

### VPP

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-89 | `vpp { }` config changed and is not in effect until restart | `doctor-vpp-restart-required` (warning, runtime) comparing running `settings` with the committed vpp config; fix names the restart command | none |
| AC-90 | The VPP process failed to start or keeps exiting (manager failed, DPDK bind, startup conf, connect; restart backoff), including the "vpp component not running" half of the `vpp` health row | `doctor-vpp-process-restarting` (error, runtime) with the last `runOnce` error, restart count and backoff | none |
| AC-91 | VPP stats unavailable (telemetry disabled or polls failing) | `doctor-vpp-stats-down` (warning, runtime) from `statsUp` and the stats connection | telemetry disabled by config names the leaf that enables it |
| AC-92 | Configured `dpdk interface` NICs still bound to vfio while VPP is not running | `vpp-dpdk` gains `doctor-vpp-dpdk-left-bound` (warning, local) naming PCI address and driver | none |
| AC-93 | The VPP backend name map was never populated, so DPDK interfaces are unknown to Ze | `doctor-iface-vpp-name-map-empty` (error, runtime) | none |
| AC-94 | fib-vpp has no writer (no connector, channel will not open, lifecycle event lost) | `doctor-fib-vpp-no-writer` (error, runtime) with the held `unreachable` cause | the socket itself down is `doctor-vpp-unreachable` |
| AC-95 | SRv6 ownership state cannot be restored | `doctor-fib-vpp-srv6-ownership` (error, runtime) with the restore error | none |
| AC-96 | SRv6 steering differs from the selection (steer add or delete, fallback replace or cleanup, checkpoint failed) | `doctor-fib-vpp-srv6-steer` (error, runtime) naming SID and error | none |
| AC-97 | A Ze-tagged (`ze/`) NAT44 static mapping remains in VPP after an orphan delete failed | `doctor-firewall-vpp-orphan-nat` (warning, runtime) from `nat44StaticMappingDump` | R-2: untagged mappings are never counted |
| AC-98 | A stale Ze policer or classify table remains in VPP after a reconcile | `doctor-traffic-vpp-orphan` (warning, runtime) from the policer and classify dumps | R-2 |
| AC-99 | AC-5's pending record holds a route or MPLS route not in VPP | a `runtime` fib-vpp check declares `doctor-fib-vpp-sync-failure`, naming prefix, error and age | none |

### Not problem classes (each line carries the named code)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-100 | A8 programmer or wiring BUG paths (30 sites: tx rule registration, sysrib schema, malformed internal batches and the locrib invariant, malformed in-process event JSON, JSON encode failures, propagated template, no event bus, DHCPv6 encoders) | each line carries `doctor-internal-defect` | none |
| AC-101 | A8 recovered panics (6 sites) and teardown socket-close lines (4 sites) | each line carries `doctor-internal-defect` | none |
| AC-102 | A8 lines with no lasting effect: RA ICMPv6 filter optimization, update-slot race, unhandled link event kind | each line is logged at Info, with the reason in a comment beside it | none: no operator state follows from them |
| AC-103 | A9 producer BUG paths (8) and wiring defects with no event bus (3) | each line carries the writer's "not installed" code: `doctor-fib-sync-failure`, `doctor-fib-p4-no-backend`, `doctor-fib-vpp-sync-failure`, `doctor-fib-vpp-srv6-steer`; the no-bus lines `doctor-fib-kernel-forwarding-off`, `doctor-fib-p4-no-backend`, `doctor-fib-vpp-no-writer` | none |
| AC-104 | A9 "no pending config (verify not called?)" lines (firewall, traffic) and init-time registration failures (6) | the first carry `doctor-plugin-config-unapplied`, the second `doctor-plugin-setup-failed` (with AC-2) | none |
| AC-105 | A9 teardown close lines (6) and fib-vpp flush-on-stop lines (5) | each carries its owner's left-behind code: `doctor-firewall-tables-left-after-stop`, `doctor-fib-kernel-routes-left`, `doctor-fib-p4-no-backend`, `doctor-vpp-stats-down`, `doctor-fib-vpp-no-writer`; flush-on-stop lines `doctor-fib-vpp-sync-failure` | none |
| AC-106 | `./le arch doctor-coverage report` after every phase | the report lists every site under the packages of the Task table as covered, each by a code a registered check declares, and its covered count for these packages equals their site count | a site outside these packages (`internal/plugins/trafficusage` belongs to the system-plugins child) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | runs `show doctor` after `nft flush ruleset` | `HandleDoctor` -> firewall runtime check -> `AuditTables` | `test/firewall/show-doctor-firewall-drift.ci` |
| 2 | runs `show doctor` on a router whose kernel refuses a route | runtime check -> fib-kernel `pending` | `test/plugin/show-doctor-fib-sync-failure.ci` |
| 3 | commits a unit with a mistyped sysctl profile | iface verify -> refusal with the fix | `test/parse/iface-sysctl-profile-unknown.ci` |
| 4 | runs `show doctor` on a CPE whose DHCP server is silent | runtime check -> DHCP client status | `test/plugin/show-doctor-dhcp-client-no-lease.ci` |
| 5 | runs `ze doctor` on Ze's own kernel with a firewall | nf_tables capability probe | `test/ui/doctor-firewall-nftables.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReportCodesAreDoctorCodes` | `internal/plugins/fib/kernel/fibkernel_test.go`, `internal/component/firewall/audit_test.go`, `internal/component/iface/health_test.go` | AC-1: raised codes have metadata and the `doctor-` prefix | |
| `TestHealthRowsFilterOwnerConstants` | `internal/plugins/firewall/nft/health_test.go` | AC-1 | |
| `TestIfaceHealthDegradedWhenCountersUnread` | `internal/component/iface/health_test.go` | AC-3 | |
| `TestDHCPClientStatus` | `internal/plugins/iface/dhcp/status_test.go` | AC-4 | |
| `TestFIBVPPPendingRecord` | `internal/plugins/fib/vpp/fibvpp_test.go` | AC-5 | |
| `TestRollbackOutcomeKept` | `internal/component/iface/config_apply_test.go`, `internal/plugins/policyroute/register_test.go` | AC-6 | |
| `TestDoctorIface<Class>` (one per AC-7..AC-48 in iface) | `internal/component/iface/doctor_*_test.go` | each AC, red when the detection is removed | |
| `TestIfaceVerifyRefusesUnknownSysctlProfile` | `internal/component/iface/config_sysctl_test.go` | AC-16 refusal; built-in and candidate profiles accepted | |
| `TestDoctorRA<Class>` | `internal/plugins/iface/ra/doctor_test.go` | AC-21..AC-29 | |
| `TestDoctorDHCPClient<Class>` | `internal/plugins/iface/dhcp/doctor_test.go` | AC-30..AC-35 | |
| `TestDoctorSysctl<Class>` | `internal/component/sysctl/doctor_test.go` | AC-50..AC-53 | |
| `TestDoctorSysrib<Class>` | `internal/component/sysrib/doctor_test.go` | AC-54..AC-56 | |
| `TestDoctorFirewallAudit` | `internal/component/firewall/doctor_test.go` | AC-59..AC-63 | |
| `TestNftablesCapabilityProbe` | `internal/plugins/firewall/nft/kernelcap_linux_test.go` (QEMU) | AC-66, A-3 | |
| `TestIRRCheckDeclaresEveryEmittedCode` | `internal/component/firewall/plugins/irr/doctor_test.go` | AC-73 | |
| `TestDoctorTraffic<Class>` | `internal/component/traffic/doctor_test.go` | AC-70, AC-71 | |
| `TestDoctorFIBKernel<Class>` | `internal/plugins/fib/kernel/doctor_test.go` | AC-74..AC-84 | |
| `TestDoctorFIBP4NoBackend` | `internal/plugins/fib/p4/doctor_test.go` | AC-85 | |
| `TestDoctorPolicyroute<Class>` | `internal/plugins/policyroute/doctor_test.go` | AC-86, AC-88 | |
| `TestDoctorVPP<Class>` | `internal/component/vpp/doctor_test.go` | AC-89..AC-92 | |
| `TestFIBVPPNoWriterDoctor` and siblings | `internal/plugins/fib/vpp/doctor_test.go` | AC-94..AC-96, AC-99 | |
| `TestForwardingSitesCovered` | `internal/le/arch/doctorcoverage/forwarding_test.go` | AC-106 over the real tree | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| fib-kernel pending age for lag | 0 to `fibLagTimeout` | `fibLagTimeout` minus 1 s (sync-failure only) | N/A | `fibLagTimeout` (lag reported) |
| fib-vpp pending age | 0 to `fibLagTimeout` | same | N/A | same |
| DHCP no-lease age | 0 to the first request exchange | exchange minus 1 s (silent) | N/A | exchange (reported) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-doctor-firewall-drift` | `test/firewall/` | operator reads the vanished table | |
| `show-doctor-fib-sync-failure` | `test/plugin/` | operator reads the refused route | |
| `iface-sysctl-profile-unknown` | `test/parse/` | operator's commit is refused with the profile list | |
| `show-doctor-dhcp-client-no-lease` | `test/plugin/` | operator reads why the unit has no address | |
| `doctor-firewall-nftables` | `test/ui/` | operator on Ze's kernel sees no false warning | |
| `show-doctor-sysctl-not-applied` | `test/plugin/` | operator reads the drifted key | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | no wire behavior changes; doctor reads state | |

## Files to Modify
- `internal/component/iface/health.go` - AC-1 code rename, AC-3 read error, AC-19
- `internal/component/iface/config_sysctl.go` - AC-15, AC-16 apply side
- `internal/component/iface/config.go`, `internal/component/iface/register.go` - AC-16 verify refusal through the read root
- `internal/component/iface/pppoe_client.go` - AC-37, AC-38 line codes and route record
- `internal/component/iface/` (the apply, mirror, delegated-prefix, RA reconcile, RA DNS, offload, unnumbered, migrate, resolver and PPPoE IPv6 files, and the iface doctor registration) - AC-6, AC-7..AC-49 checks, records and line codes
- `internal/plugins/iface/dhcp/` - AC-4 status, AC-30..AC-36
- `internal/plugins/iface/ra/` - AC-21..AC-23, AC-102
- `internal/plugins/iface/netlink/` - AC-14, AC-27, AC-28, AC-42..AC-46, AC-48
- `internal/plugins/iface/vpp/` - AC-93
- `internal/component/sysctl/` - AC-50..AC-53
- `internal/component/sysrib/` - AC-54..AC-56, AC-58
- `internal/component/firewall/audit.go` - AC-1 codes exported, AC-59..AC-61
- `internal/component/firewall/` (registry, engine, metrics) - AC-62..AC-65
- `internal/component/firewall/plugins/irr/register.go`, `internal/component/firewall/plugins/irr/doctor.go`, `internal/component/firewall/plugins/irr/irr.go` - AC-2, AC-69, AC-72, AC-73
- `internal/component/firewall/plugins/domain/` - AC-2, AC-67..AC-69, AC-72
- `internal/plugins/firewall/nft/health.go` - AC-1 filter on owner constants
- `internal/plugins/firewall/nft/doctor_linux.go` - AC-66: its check is deleted first (`remove` through `./le commit create` if the file empties), replaced by the capability
- `internal/plugins/firewall/vpp/` - AC-97
- `internal/component/traffic/`, `internal/plugins/traffic/netlink/` - AC-70, AC-71, AC-104
- `internal/plugins/traffic/vpp/` - AC-98
- `internal/plugins/fib/kernel/fibkernel.go`, `internal/plugins/fib/kernel/register.go` - AC-1, AC-77, AC-78
- `internal/plugins/fib/kernel/` (backend, monitor, MPLS entry and label space files) - AC-74..AC-84, AC-87
- `internal/plugins/fib/p4/` - AC-85
- `internal/plugins/fib/vpp/` - AC-5, AC-94..AC-96, AC-99
- `internal/plugins/policyroute/` - AC-6, AC-86..AC-88
- `internal/component/vpp/` - AC-89..AC-92
- `internal/component/iface/yang/ze-iface-conf.yang` - AC-16: the `sysctl-profile` description names the verify refusal
- `internal/core/diagnostic/codes.go` - metadata of every code with no owner-side registration
- `docs/architecture/core-design.md`, `docs/architecture/static-routes.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/guide/health-checks.md` - AC-1 renamed codes; the new rows
- `test/static/static-interface-nexthop-pppoe-follows-device.ci` - AC-1 renamed code

## Files to Create
- `internal/plugins/firewall/nft/kernelcap_linux.go` - AC-66 capability enrolment and probe
- `internal/plugins/iface/dhcp/status.go` - AC-4
- one doctor file per owner package that has none today, each with its test
- the six `.ci` files in the Functional Tests table

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `show interface dhcp-client` (AC-4) in the iface YANG beside `pppoe-client` |
| YANG validation constraints | No | `sysctl-profile` stays `zt:node-name`; a profile name is checked against the candidate in verify (AC-16), which YANG cannot express |
| YANG custom validators | No | the verify check reads two roots; not a single-leaf validator |
| CLI commands/flags | Yes | `show interface dhcp-client` |
| CLI grammar (keyword before value) | Yes | `ai/rules/cli.md`; `./le cli grammar` |
| Editor autocomplete | Yes | `sysctl-profile` completion from the built-in and configured profiles, so the refusal is avoidable |
| Functional test for new RPC/API | Yes | `test/plugin/show-doctor-dhcp-client-no-lease.ci` reads `show interface dhcp-client` too |
| Pipe completeness | Yes | the new show output through the json pipe |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | Yes | this spec; nf_tables becomes a kernel capability |
| Prometheus counters/metrics | No | reads existing counters (apply timeouts, IRR outcome, resolver drops); adds none |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | the doctor feature page under `features/` |
| 2 | Config syntax changed? | No | `sysctl-profile` syntax unchanged; an unknown name is refused |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (`show interface dhcp-client`) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` (the new show) |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/health-checks.md`, `docs/guide/firewall.md`, `docs/guide/vpp.md` |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | none |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/doctor-and-health-checks.md` (nf_tables capability, runtime checks of the area); `docs/architecture/core-design.md` (report codes) |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/guide/status.md` (new show command, nf_tables capability) |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | named now: `docs/architecture/static-routes.md` (fib sync-failure code); `docs/architecture/firewall/backend-command-dispatch.md` (declared by `firewall/nft/health.go`: updated, the row filters on owner constants); `docs/architecture/l2tp/cpe-1-pppoe-client.md` (declared by `pppoe_client.go`: updated, the IPv4 route and session sysctl rows); `docs/features/interfaces.md` (declared by the iface files: updated, the unknown `sysctl-profile` refusal and the interface doctor rows); `docs/features/ai-first.md` (declared by `codes.go`: unaffected, the diagnostic record keeps its fields and gains codes only); the rest derived by `./le spec citation anchors spec plan/release/validation/spec-doctor-coverage-10-forwarding.md` in each phase |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/health-checks.md` sample `show warnings` codes |

## Implementation Steps

1. **Phase: Wiring** -- register one check per owner returning nothing; the wiring tests fail
2. **Phase: the packages below**, in order of dependency; each lands as its own commit when its package tests pass

### Implementation Phases

Each package is sized for one agent of about 60 tool calls. Agents edit; every test runs in the main thread, one at a time. `∥` marks packages that touch disjoint files.

| Phase | Owns | Covers | Depends on | Runs with |
|-------|------|--------|------------|-----------|
| P1 codes and health rows | report codes and filters in `iface/health.go`, `firewall/audit.go`, `firewall/nft/health.go`, `fib/kernel/fibkernel.go`, `fib/kernel/register.go`; code metadata; census readers and pages | AC-1, AC-3, AC-19 | - | ∥ P2 |
| P2 setup-line collisions | `firewall/plugins/{domain,irr}/register.go`; IRR `Codes` | AC-2, AC-73 | - | ∥ P1 |
| P3 firewall runtime | audit, apply-behind and held-back checks | AC-59..AC-63 | P1 | ∥ P6 |
| P4 firewall local and store | global options, tables left, IRR and domain store rows, domain truncation and cache write, covered lines | AC-64, AC-65, AC-67..AC-69, AC-72 | P2 | ∥ P6 |
| P5 nf_tables capability and traffic | delete the module-list check, enrol the capability; tc snapshot and qdisc checks | AC-66, AC-70, AC-71 | - | ∥ P7 |
| P6 fib-kernel runtime | sync, lag, orphan, reassert, monitor, MPLS install and remove, label space | AC-77, AC-78, AC-80..AC-84 | P1 | ∥ P3 |
| P7 fib-kernel local, p4, policyroute | netlink probe, MPLS guard, forwarding off, routes left, p4, rules left, rollback record and check, covered lines | AC-6 (policyroute half), AC-74..AC-76, AC-79, AC-85..AC-88 | - | ∥ P5 |
| P8 VPP component | restart required, process restarting, stats, dpdk left bound, name map | AC-89..AC-93 | - | ∥ P9 |
| P9 fib-vpp and VPP backends | pending record, no writer, SRv6 ownership and steer, sync check, NAT and traffic orphans | AC-5, AC-94..AC-99 | - | ∥ P8 |
| P10 sysctl and sysrib | the sysctl and sysrib checks and lines | AC-50..AC-58 | - | ∥ P11 |
| P11 iface apply state | list failed, apply owed, rollback record and check, registry reconcile, tunnel adopted, macvlan recreated, netdev left | AC-6 (iface half), AC-8..AC-13, AC-45 | - | ∥ P10 |
| P12 iface local checks | IPv6 forwarding switch, sysctl-profile verify and check, rp-filter, offload, every configured kind, wireguard handshake, rpf table, covered lines | AC-15..AC-18, AC-44, AC-46, AC-48, AC-49 | - | ∥ P14 |
| P13 iface data-plane state | members and mirror selectors, mirror, delegated prefix, migrate, unnumbered, route held, monitor degraded, DAD | AC-7, AC-14, AC-20, AC-40..AC-43, AC-47 | child 2 phase 4 for AC-7 | ∥ P14 |
| P14 RA | sender, default route, send failed, RDNSS advertised and published, learned DNS, option discarded, reader stopped, defrtr | AC-21..AC-29 | - | ∥ P12 |
| P15 DHCP client | status and show command, then the six DHCP checks, the external refusal line | AC-4, AC-30..AC-36 | - | ∥ P16 |
| P16 PPPoE and plugin lines | PPPoE IPv4 route, session sysctls, redial line, plugin-stopped lines | AC-37..AC-39, AC-57 | child 12 phase P0a declares `doctor-plugin-stopped` | ∥ P15 |
| P17 not problem classes | the remaining lines of both slices | AC-100..AC-105 | P3..P16 (codes exist) | - |
| P18 coverage proof | `TestForwardingSitesCovered`, the coverage report | AC-106 | all | - |

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC at file and symbol; every site of the area covered (AC-106) |
| Correctness | no check reads healthy for state it could not read; left-behind checks count only Ze-owned state (R-2) and run only when no daemon answers (R-3) |
| Naming | codes kebab-case with the `doctor-` prefix, one declaration each; the renamed report codes have no second spelling |
| Data flow | a bus-reading runtime check only for an in-process owner (A-1) |
| Rule: user-facing-errors | every fix a command on its own line |
| Rule: no-layering | the `/proc/modules` nft check is deleted before the capability lands |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| every new code explained | `ze show diagnostic <code>` answers for each |
| area covered | `./le arch doctor-coverage report` lists every forwarding site as covered |
| renamed codes | grep of the old names over `internal/`, `test/`, `docs/` answers nothing |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Secrets | DHCP and PPPoE rows never print credentials |
| Resource use | runtime checks read snapshots; kernel reads bounded by a timeout, as `checkFirewallHealth` does |
| Privilege | an unprivileged probe answers cannot determine, never absent |

### Failure Routing

| Failure | Route To |
|---------|----------|
| A class has no readable state | add the owner record in a shared-mechanism phase first |
| A named fix command does not exist | A-2: name the verified command |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- Two slice claims fell at the producer: the PPPoE redial timing is validated inside the verify parse, and an IRR lookup error marks the kept entry stale (`refreshLive`). Neither needs a fix; their lines take the codes that already describe the outcome (AC-37, AC-72).
- `checkFirewallHealth` already refuses to read an audit that did not run as healthy; the iface health row is the same shape without that guard (AC-3).

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `plan/journal/zero-value-as-valid-answer.md` | 2026-11-10 | `CheckAllInterfaceErrors` returns 0 when it read nothing |
| `plan/journal/validate-accepts-what-start-refuses.md` | 2026-11-10 | unit `sysctl-profile` unknown name commits |
| `plan/journal/declared-format-contradicts-payload.md` | 2026-11-10 | `firewall-irr-data-freshness` `Codes` omits the oversized code |
| `plan/journal/probe-inability-read-as-absence.md` | 2026-11-10 | `checkFirewallNftables` reads `/proc/modules` |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Rename the six report-bus codes to doctor codes | keep both, map one to the other in `recent-events` | a second name for one fact is a copy; pre-release owes no alias |
| Unknown `sysctl-profile` refused at verify, doctor row as backstop | doctor row only | exact or refuse at verify (forwarding door); the row covers configs already running |
| nf_tables as a `kernelcap.Capability` | keep `/proc/modules` and add a built-in check | the doctor page rules a configured subsystem's kernel feature a capability; no-layering deletes the old check |
| Code names for the A8 gap rows chosen here | wait for the inventory merge | the canonical table holds none for A8 (A-5); the merge adds these names |
| Left-behind checks run only when no daemon answers | run on both routes | with a daemon up the state is its own, and the daemon route already reports it through the runtime checks |
| Lines with no lasting effect go to Info | give them a code | a line with no operator state is not a problem site; logging it at Warn is the class `normal-state-logged-at-error` |

## Known Limitations
- The cross-cutting plugin codes and their checks are `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md`'s (phase P0a); the lines here carry them, and they count as covered once child 12's checks declare them.
- `internal/plugins/trafficusage` matched this area's path prefix and belongs to the system-plugins child.

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
- [ ] AC-1..AC-106 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Scoped package tests under `./le job run`, with the tags `./le repo changed scope print both` names, for each package a phase changed; the named `.ci` files run alone; `needs-linux` files under `./le test qemu`. No agent runs any `./le verify` command
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: no wire change)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec + the four journal rows turned `fixed:`
- [ ] **Commit B:** `remove plan/release/validation/spec-doctor-coverage-10-forwarding.md` only
