# Spec: doctor-coverage-2-home-cpe-batch

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Priority | P2 |
| Release reason | the eight problem classes the home-CPE lab met on 2026-11-07, each visible today only in a log line or by reading kernel state by hand; `ai/META-DECISIONS.md` makes each a gap owed a doctor check |
| RFC gaps | - |
| Depends | `plan/release/validation/spec-doctor-coverage-1-audit-and-gate.md` phase 1 (the `runtime` phase and the `fix` field); class 3 also `spec-home-cpe-23b-dns-source-consumers` (closed; `show system dns` and the DNS source doctor checks), after `spec-home-cpe-23-dns-source-fall-through` (23a: the resolver registry and the router's selection, design in `docs/features/dns-resolver.md`); class 8's last-failure text (A-3) also `plan/release/validation/spec-doctor-coverage-3-access-security.md` phase P1, which builds the IKE per-peer last failure record this spec reads. Sibling children that build on this spec, not the reverse: `plan/release/validation/spec-doctor-coverage-3-access-security.md` (extends the L2TP per-peer record, phase 3), `plan/release/validation/spec-doctor-coverage-10-forwarding.md` AC-7 (extends the name-resolver check, phase 4), and `plan/release/validation/spec-doctor-coverage-5-appliance.md` (leaves the resolver selection states, class 3, here) |
| Phase | design |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent: `plan/release/validation/spec-doctor-coverage-0-umbrella.md`.

## Task

Thomas: "create a spec to add all those to ze doctor so it can be used to find
any problem". "Those" are the problem classes the home-CPE lab work found that
doctor did not report. One registered check per class, in the package that owns
the state, each naming the cause in plain words and the exact fixing command,
on every surface through the daemon, with a test that goes red when the
condition is not reported.

| # | Class | Owner package | Route |
|---|-------|---------------|-------|
| 1 | IPsec discard catch-all in force while the box's own traffic (ICMP, DNS, NTP, SSH, BGP) has no bypass | `internal/component/ike/engine` | local and daemon (config) |
| 2 | L2TP peer that never acknowledged control messages; tunnels and sessions stuck establishing | `internal/component/l2tp` | runtime |
| 3 | Ze's own system resolver: no usable server, one server where several are configured or learned, learned DNS not followed | the system DNS owner `spec-home-cpe-23` creates | runtime |
| 4 | No interface backend loaded while interface-dependent features are configured | `internal/component/iface` | local (config) and runtime |
| 5 | Bridge STP in the kernel not matching config | `internal/plugins/iface/netlink` | local and daemon (kernel) |
| 6 | PPPoE client: no session; session up with IPCP or IPv6CP not opened; IPv6 with no default route or no address from RA/SLAAC; DHCPv6-PD requested and not received | `internal/component/iface` (PPPoE client) | runtime |
| 7 | Configured logical interface name resolving to no kernel device; pppoe-client source-interface not resolving | `internal/component/iface` (name resolver) | runtime |
| 8 | XFRM interface (VTI) bound peer with no Child SA installed; tunnel traffic blocked because the if_id 0 exemptions are missing | `internal/component/ike/engine` | runtime |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/doctor-and-health-checks.md` - registration owners and the two tests
  → Constraint: an internal plugin registers through `Registration.DoctorChecks` (IKE does, `ike/engine/register.go`); a component registers with `diagnostic.RegisterDoctorCheck` from its own `init()` (iface does, `register_linux.go`; the netlink backend does, `doctor.go` plus `register.go`)
  → Constraint: a unit test in the owning package proves the check fires only when its config is present and emits its registered code; a functional test exposes it through `ze doctor --json` or `show doctor`
- [ ] `docs/architecture/iface/logical-name-resolution.md` - the shared resolver
  → Constraint: names go through `Resolve`, `ResolveIndex`, `ResolveDevice`; `Binding` holds no address, `Addresses` reads them; a check never reads the kernel by name directly (`./le arch iface-resolution`)
- [ ] `ai/doors/access-security.md` - IKE and L2TP
  → Constraint: readers of IKE state outside the engine use `ipsecinventory.Tunnels()`, never `ike/engine` (core-design section 19); a check inside the engine package may read engine state
  → Constraint: "Configured is not negotiated": a check reports the installed state, never the configured one as if installed
- [ ] `ai/doors/forwarding.md` - iface backends
  → Constraint: a backend method is added to the `Backend` interface and every backend answers it; VPP answers what it can and says when it cannot (its `BridgeSetSTP` already answers `errNotSupported`)
  → Constraint: the report bus is process-local, so a check never relies on a forked plugin's warnings
- [ ] `ai/doors/hub-engine.md` - the hub
  → Constraint: no always-on hub file imports a gated package; class 3's check registers from its owner, not from `cmd/ze/hub`, once that owner exists
- [ ] `ai/doors/system-plugins.md` - the DNS plugins
  → Constraint: a plugin owns its doctor checks in its folder
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: problem, why, and the fixing command on its own line (the `fix` field from child 1)

**Key insights:**
- Two existing IKE checks already read engine runtime state at `post-config` (`ipsec-virtual-ip-pool` reads "the running SA table", `ipsec-udp-encap` its socket state); on the local route that state is empty, so they can read healthy for a state they never saw. They move to `runtime` here (AC-9)
- The implicit-discard text is already declared once (`implicitDiscardCuts`, `implicitDiscardICMPExample` in `unmatched.go`); class 1's message and fix reuse them, never a copy
- L2TP registers a health row (`health.Register("l2tp", checkHealth)`) and no doctor check at all

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/unmatched.go` - `refuseImplicitDiscard` refuses a commit that turns the discard on unwritten; `warnImplicitDiscardAtStart` logs at Error at start; `countBypassEntries` counts bypass policies but never checks what they cover
- [ ] `internal/component/ike/engine/register.go` - six `DoctorChecks` at `post-config`, Orders 730 to 735, none about the discard catch-all or bound XFRM interfaces
- [ ] `internal/component/ike/engine/unmatched_xfrm_interface.go` - `xfrmInterfaceEntry` and `xfrmInterfaceExemptions`: the if_id 0 BYPASS entries for each bound interface; a failed read keeps the entries
- [ ] `internal/core/ipsecinventory/registry.go` - `Tunnel{Peer, Up, IfID, ...}`, `Tunnels()`, `ErrNotRegistered`
- [ ] `internal/component/l2tp/reactor.go` `handleTick` - retransmit budget exhausted on a not-closed tunnel logs the Warn "peer never acknowledged a control message" (an uncommitted working-tree edit on 2026-11-08) and tears down; nothing keeps the fact once the tunnel is gone
- [ ] `internal/component/l2tp/snapshot.go` - `Snapshot{Tunnels, ...}`, `TunnelSnapshot{PeerAddr, State, CreatedAt, LastActivity, Sessions}`
- [ ] `internal/component/l2tp/health.go` `checkHealth` - degraded when the subsystem is absent or has no kernel data plane; no code, no fix
- [ ] `internal/component/l2tp/register.go` - tree validators and `health.Register`; no doctor registration
- [ ] `cmd/ze/hub/main_system.go` `newResolvers` - the system resolver gets `sc.NameServers[0]` only
- [ ] `internal/component/iface/dispatch.go` - `ErrNoBackendLoaded` "iface: no backend loaded"
- [ ] `internal/component/iface/backend.go` `Backend` - `BridgeAddPort`, `BridgeDelPort`, `BridgeSetSTP`; no read of the STP state
- [ ] `internal/component/iface/register_linux.go` - registers `ethernetDoctorCheck` with `diagnostic.RegisterDoctorCheck`
- [ ] `internal/plugins/iface/netlink/bridge_linux.go` `BridgeSetSTP` - writes `IFLA_BR_STP_STATE` over rtnetlink; no read
- [ ] `internal/plugins/iface/netlink/doctor.go` - `macvlanDoctorCheck` (`iface-macvlan`, Order 742), silent when the backend is vpp
- [ ] `internal/plugins/iface/vpp/ifacevpp.go` `BridgeSetSTP` - `errNotSupported`
- [ ] `internal/component/iface/config_apply.go` bridge step - undo sets `!e.STP`, not the prior state (journal row below)
- [ ] `internal/component/iface/pppoe_client.go` - `PPPoEClient{state, lastErr, attempts, upSince, ...}`, `PPPoEClientStatus{Name, State, SourceInterface, PPPInterface, SessionID, LocalIP, PeerIP, Uptime, DNSServers}`: no NCP state, no IPv6 facts

**Behavior to preserve:**
- every existing IKE, L2TP, iface and DNS check and code; the L2TP health row
- `show interface pppoe-client` keys (new keys are added, none renamed)
- the implicit-discard commit refusal and start log

**Behavior to change:**
- one registered check per class, as in the Task table

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| IKE plugin doctor checks | `internal/component/ike/engine/register.go` `DoctorChecks` | Six config-time checks | extends: classes 1 and 8 join the list; two existing runtime readers move to `runtime` |
| Implicit discard refusal and log | `ike/engine/unmatched.go` `refuseImplicitDiscard`, `warnImplicitDiscardAtStart`, `implicitDiscardCuts` | Refuses or logs the unwritten discard | extends: class 1 reuses the strings and adds what neither checks: which own-traffic flows have no bypass |
| XFRM interface exemptions | `ike/engine/unmatched_xfrm_interface.go` `xfrmInterfaceExemptions`, `xfrmInterfaceEntry` | Builds and tracks the if_id 0 entries | extends: class 8 compares the entries Ze holds with the kernel SPD |
| IPsec inventory | `internal/core/ipsecinventory` `Tunnels` | Per-peer `Up`, `IfID` | extends: class 8 reads it |
| L2TP snapshot | `internal/component/l2tp/snapshot.go` `Snapshot` | Live tunnels and sessions | extends: adds the per-peer unanswered record class 2 needs |
| L2TP health row | `internal/component/l2tp/health.go` `checkHealth` | Degraded reason, no code | does not apply: kept; the doctor check carries the code and fix |
| Interface backend registry | `internal/component/iface/backend.go` `Backend`, `GetBackend` | The loaded backend, nil when none | extends: class 4 reads it; class 5 adds a read method |
| Name resolver | `internal/component/iface/resolve.go` `Resolve`, `Addresses` | Logical name to device | extends: class 7 asks it for every configured entry |
| PPPoE client status | `internal/component/iface/pppoe_client.go` `status`, `PPPoEClientStatus` | Session state for `show interface pppoe-client` | extends: adds NCP and IPv6 facts; class 6 reads the same producer |
| netlink backend doctor | `internal/plugins/iface/netlink/doctor.go` (`iface-macvlan`) | Backend-owned check | extends: class 5 registers beside it |
| Static route skipped check | `doctor-static-route-skipped` | Reports a route skipped at apply | does not apply: class 7 excludes static next-hops, which that check covers |
| System DNS sources | `spec-home-cpe-23b-dns-source-consumers` (closed; built the producer of `show system dns`, `internal/plugins/system-dns-cmd`, and a check for an empty router selection, over the registry `spec-home-cpe-23-dns-source-fall-through` builds) | Fixes the resolver's behavior | extends: class 3 reads the same producer and adds the three resolver states; the check spec-23 plans is one of class 3's codes, not a second check |
| Operator error text | `internal/component/iface/dispatch.go` `ErrNoBackendLoaded` (an `opererr.Error`, spec-operator-errors-name-the-fix, closed), journal row `plan/journal/error-names-no-fix-command.md` 2026-11-07 (`ErrNoBackendLoaded`) | Fixes the error text | extends: class 4 reports the state; both name one fix command, declared once in `iface` |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze doctor [--json] [config <file>]` locally; `show doctor` on SSH, MCP, REST, gRPC, web

### Transformation Path
1. `doctor.Answer` runs the registered checks; config-readable classes (1, 4 config half, 5) run on both routes; `runtime` classes (2, 3, 4 live half, 6, 7, 8) run in the daemon
2. Each check reads its owner's state (table in Task) and returns diagnostics with code, severity, message, fix
3. `show doctor` returns them on every surface; `ze doctor` draws them

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| IKE plugin to doctor | `DoctorChecks` callback (in-process through DirectBridge, or RPC) | Yes, existing six checks |
| Component to doctor | `diagnostic.RegisterDoctorCheck` in the daemon process | Yes, iface `register_linux.go` |

### Changed-Wire Census

**Search evidence:** the only output changed outside doctor is `show interface pppoe-client`, which gains keys. Scope `internal/`, `test/`; terms `PPPoEClientStatus`, `pppoe-client` in `.ci` expectations. Positive control: `pppoeClientAnswer` in `iface/pppoe_client_command.go` produces it.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `show interface pppoe-client` JSON | writer | `iface/pppoe_client.go` `status`, `pppoe_client_command.go` `pppoeClientAnswer` | read above | add `ipcp-state`, `ipv6cp-state`, `last-error`, `attempts`, `ipv6-default-route`, `ipv6-addresses`, `delegated-prefixes`; no key renamed |
| same | reader | `.ci` files and interoplab checks that parse it | to search at implementation | preserve: additions only |

### Integration Points
- `ike/engine/register.go` `DoctorChecks`; `l2tp/register.go`; `iface/register_linux.go`; `plugins/iface/netlink/register.go`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | names through the resolver; IKE state through the engine or `ipsecinventory` |
| No unintended coupling (components stay isolated) | Yes | each check in its owner |
| No duplicated functionality (extends existing, does not recreate) | Yes | Existing Mechanisms |
| Zero-copy preserved where applicable (refs, not copies) | Yes | read-time snapshots only |
| Registration over hardcoding, outbound | Yes | every check registers |
| Registration over hardcoding, inbound | Yes | class 4 derives the interface-dependent leaves from the YANG schema (the interface-name type), never a list of features |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every config leaf naming an interface uses one YANG type, so class 4 can derive "interface-dependent" from the schema | logical-name-resolution page: completion from the iface resolver | class 4 misses a feature | grep of YANG modules for the interface-name typedef at implementation | unvalidated |
| A-2 | The L2TP Warn in `reactor.go` lands (it is another session's uncommitted edit) | `git diff` on 2026-11-08 | class 2's site carries no code yet | read HEAD before phase 3; if absent, the check's own record is the evidence | unvalidated |
| A-3 | The IKE engine keeps or can keep the last negotiation failure per peer, so class 8 names why no Child SA exists | none yet; `plan/release/validation/spec-doctor-coverage-3-access-security.md` phase P1 builds the record | class 8 says only "no Child SA since T" | read the record child 3 P1 lands, or the `ike/engine` session failure paths if it has not landed | unvalidated |
| A-4 | `RTM_GETLINK` returns `IFLA_BR_STP_STATE` without privilege | rtnetlink link dumps are unprivileged | class 5 needs the daemon route | `TestBridgeSTPStateRead` under QEMU | unvalidated |
| A-5 | A reload command re-applies interface config when the kernel drifted (class 5 fix line) | iface reconciles on apply | the fix line names the wrong command | implementer reads the reload path and names the verified command | unvalidated |
| A-6 | spec-home-cpe-23 names the package that owns system DNS selection | its Files to Create | class 3 has no owner | read spec-23 before phase 6; until it lands, the check reads the hub's resolver configuration through a registered seam | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A check reports "stuck establishing" during a normal slow start | doctor fires during boot | thresholds come from the protocol's own timers: L2TP total retransmit budget, PPPoE client redial timing, IKE retransmit budget; never a new constant |
| R-2 | Class 7 reports a dialed ppp device as unresolved while its client dials | a false row on every reconnect | class 7 skips a device a pppoe-client owns while that client is not up; class 6 reports it |
| R-3 | Class 1 calls a flow uncovered when a tunnel's selectors carry it | false error on a full-tunnel box | a flow a Child SA selector covers counts as covered |
| R-4 | Class 5 on VPP has no STP | false row | the VPP backend answers not supported and the check says it cannot determine, at warning |
| R-5 | Moving two IKE checks to `runtime` removes them from the local route | an operator sees fewer rows locally | the local route's `doctor-runtime-not-checked` row (child 1) names `show doctor` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | false or missing doctor rows; `show interface pppoe-client` gains keys |
| How is it reverted? | single commit per class |
| Who else touches this path? | home-CPE children 1 (names), 4 (IKE dynamic WAN), 12, 13, 17 (PPPoE client), 8 (IPv6 chain), 20 (XFRM state), 23 (DNS) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze doctor --json config` with `vpn ipsec` discard and no ICMP bypass | → | IKE `ipsec-own-traffic-bypass` | `test/ui/doctor-ipsec-discard-own-traffic.ci` |
| `show doctor` with an L2TP peer that drops control messages | → | l2tp `l2tp-peer-unresponsive` | `test/plugin/show-doctor-l2tp-peer-unresponsive.ci` |
| `show doctor` with several name servers | → | system resolver check | `test/plugin/show-doctor-system-resolver.ci` |
| `ze doctor --json config` with an interface consumer and no `interface` block | → | iface `iface-backend-loaded` | `test/ui/doctor-iface-no-backend.ci` |
| `ze doctor --json` with a bridge whose kernel STP differs | → | netlink `iface-bridge-stp` | `test/ui/doctor-iface-bridge-stp.ci` (`option=needs-linux`) |
| `show doctor` with a pppoe-client facing no access concentrator | → | iface `pppoe-client-state` | `test/plugin/show-doctor-pppoe-client.ci` (`option=needs-linux`) |
| `show doctor` with an interface `mac match` no device has | → | iface `iface-name-resolves` | `test/plugin/show-doctor-iface-name-unresolved.ci` (`option=needs-linux`) |
| `show doctor` with a VTI-bound peer and no Child SA | → | IKE `ipsec-xfrm-interface-tunnel` | `test/plugin/show-doctor-ipsec-xfrm-peer.ci` (`option=needs-linux`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Effective `vpn ipsec unmatched discard` (written or default), and an own-traffic flow (ICMPv4, ICMPv6 with neighbor discovery, each configured DNS or NTP server, each SSH, web or API listener, each BGP peer) that no bypass entry and no Child SA selector covers | `doctor-ipsec-discard-cuts-own-traffic` names each uncovered flow; error when the discard is unwritten, warning when written; fix names the bypass entry to add (from `implicitDiscardICMPExample` for ICMP) or `set vpn ipsec unmatched bypass` | none: a covered flow is not listed |
| AC-2 | An L2TP tunnel torn down because its retransmit budget ran out, no tunnel to that peer established since | `doctor-l2tp-peer-unresponsive` names the peer, the local address, since when and the state it reached; fix names the peer-side acceptance (accel-ppp `client-ip-range` as the lab case), UDP 1701 reachability, and the `show` command that lists L2TP tunnels; cleared once a tunnel to that peer establishes | none |
| AC-3 | A tunnel or session not established after its full retransmit budget | `doctor-l2tp-tunnel-not-established` or `doctor-l2tp-session-not-established` with peer, age and state | a tunnel younger than the budget is not reported (normal setup) |
| AC-4 | System resolver with no usable server, or querying one server while several are configured or learned, or not using learned DNS while the config asks it to | `doctor-system-resolver-no-server`, `doctor-system-resolver-uses-one-of-several`, `doctor-system-resolver-ignores-learned`, each naming the servers configured, learned and in use; fix names the config command | the uses-one row disappears when the resolver uses every server (spec-23 behavior); the check reads what the resolver uses, never the config alone |
| AC-5 | Config naming interfaces (any leaf of the interface-name type) and no `interface` block, or a daemon whose iface backend is nil | `doctor-iface-no-backend` error naming the first leaves that need it; fix `set interface backend netlink` | none |
| AC-6 | A configured bridge whose kernel STP state differs from `stp` | `doctor-iface-bridge-stp-mismatch` naming bridge, configured and kernel value; the read is a new `Backend` method; VPP answers not supported and the row reads cannot determine | none |
| AC-7 | A pppoe-client: no session past its redial timing; session up and IPCP not Opened (IPv4 configured) or IPv6CP not Opened (`ipv6` configured); IPv6 configured and no default route on the device, or no global address from RA/SLAAC; DHCPv6-PD requested and no delegated prefix | one code each: `doctor-pppoe-client-no-session` (with last error and attempts), `doctor-pppoe-client-ipcp-not-open`, `doctor-pppoe-client-ipv6cp-not-open`, `doctor-pppoe-client-ipv6-no-default-route`, `doctor-pppoe-client-ipv6-no-address`, `doctor-pppoe-client-pd-not-received`; `show interface pppoe-client` carries the same facts | a client within its redial timing is not reported |
| AC-8 | A configured interface whose selector (os-name, mac match) resolves to no device; a pppoe-client whose source-interface does not resolve | `doctor-iface-name-unresolved` and `doctor-pppoe-client-source-unresolved`, each naming the entry, the selector and what the kernel has; fix names `show interface` and the selector leaf | a ppp device a pppoe-client owns while that client is not up (AC-7 reports it) |
| AC-9 | A VTI-bound peer (IfID non-zero) whose inventory reads `Up` false past its IKE retransmit budget; a bound interface whose if_id 0 exemptions Ze holds are missing from the kernel SPD while the discard is in force | `doctor-ipsec-xfrm-peer-no-child-sa` (with the last failure when A-3 holds) and `doctor-ipsec-xfrm-exemption-missing` naming interface and if_id; the existing `ipsec-virtual-ip-pool` and `ipsec-udp-encap` checks move to `runtime` | none |
| AC-10 | Every check above | message states the cause in plain words; `fix` holds the exact command on its own line; reachable by `show doctor` on every surface; its unit test goes red when the detection is removed (recorded at implementation) | none |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | runs `show doctor` on a CPE whose LNS ignores it | `HandleDoctor` -> runtime check -> l2tp snapshot | `test/plugin/show-doctor-l2tp-peer-unresponsive.ci` |
| 2 | runs `ze doctor` before committing an IPsec discard | `doctor.Answer` -> IKE check -> config tree | `test/ui/doctor-ipsec-discard-own-traffic.ci` |
| 3 | runs `show doctor` on a CPE whose PPPoE has no IPv6 | runtime check -> PPPoE status -> kernel routes | `test/plugin/show-doctor-pppoe-client.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDoctorIPsecDiscardOwnTraffic` | `internal/component/ike/engine/doctor_unmatched_test.go` | AC-1, covered and uncovered flows, written and unwritten | |
| `TestDoctorL2TPPeerUnresponsive` | `internal/component/l2tp/doctor_test.go` | AC-2, set and cleared | |
| `TestDoctorL2TPNotEstablished` | `internal/component/l2tp/doctor_test.go` | AC-3, budget boundary | |
| `TestDoctorSystemResolver` | the system DNS owner's doctor test | AC-4 | |
| `TestDoctorIfaceNoBackend` | `internal/component/iface/doctor_backend_test.go` | AC-5 | |
| `TestBridgeSTPStateRead` | `internal/plugins/iface/netlink/bridge_linux_test.go` | AC-6 read | |
| `TestDoctorBridgeSTPMismatch` | `internal/plugins/iface/netlink/doctor_test.go` | AC-6 | |
| `TestDoctorPPPoEClient` | `internal/component/iface/doctor_pppoe_client_test.go` | AC-7, one case per code | |
| `TestDoctorIfaceNameUnresolved` | `internal/component/iface/doctor_resolve_test.go` | AC-8 and its survivor | |
| `TestDoctorIPsecXFRMPeer` | `internal/component/ike/engine/doctor_xfrm_interface_test.go` | AC-9 | |
| `TestDoctorChecksNameTheirFix` | each file above | AC-10 fix field | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| L2TP not-established age | 0 to retransmit budget | budget minus 1 s (silent) | N/A | budget (reported) |
| PPPoE no-session age | 0 to redial timing | timing minus 1 s (silent) | N/A | timing (reported) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `doctor-ipsec-discard-own-traffic` | `test/ui/` | operator reads which flows the discard cuts | |
| `show-doctor-l2tp-peer-unresponsive` | `test/plugin/` | operator reads the silent peer | |
| `show-doctor-system-resolver` | `test/plugin/` | operator reads the resolver servers | |
| `doctor-iface-no-backend` | `test/ui/` | operator reads the missing backend | |
| `doctor-iface-bridge-stp` | `test/ui/` | operator reads the STP mismatch | |
| `show-doctor-pppoe-client` | `test/plugin/` | operator reads the PPPoE fault | |
| `show-doctor-iface-name-unresolved` | `test/plugin/` | operator reads the unresolved selector | |
| `show-doctor-ipsec-xfrm-peer` | `test/plugin/` | operator reads the bound peer with no SA | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | no wire behavior changes; doctor reads state | |

## Files to Modify
- `internal/component/ike/engine/register.go` - two new `DoctorChecks`, two moved to `runtime`
- `internal/component/ike/engine/unmatched.go` - shared flow coverage helper beside `countBypassEntries`
- `internal/component/l2tp/reactor.go` - per-peer unanswered record set at the Warn site, cleared on establishment; code attribute on the Warn
- `internal/component/l2tp/snapshot.go` - the record in the snapshot
- `internal/component/l2tp/register.go` - doctor registration
- `internal/component/iface/backend.go` - `BridgeSTPState` on `Backend`
- `internal/component/iface/config_apply.go` - STP undo reads the prior state (journal row)
- `internal/component/iface/pppoe_client.go` - NCP and IPv6 facts in status
- `internal/component/iface/register_linux.go` - doctor registrations
- `internal/plugins/iface/netlink/bridge_linux.go` - the STP read
- `internal/plugins/iface/netlink/doctor.go` - class 5 check
- `internal/plugins/iface/vpp/ifacevpp.go` - `BridgeSTPState` not supported
- `cmd/ze/hub/main_system.go` - resolver exposes the servers it uses (with spec-23)
- `docs/guide/health-checks.md`, `docs/guide/ipsec.md`, `docs/guide/l2tp.md` - the new rows

## Files to Create
- `internal/component/ike/engine/doctor_unmatched.go`, `doctor_xfrm_interface.go`
- `internal/component/l2tp/doctor.go`
- `internal/component/iface/doctor_backend.go`, `doctor_pppoe_client.go`, `doctor_resolve.go`
- the eight `.ci` files in the Functional Tests table

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no new config; `show doctor` exists |
| YANG validation constraints | N-A | no leaf |
| YANG custom validators | N-A | no leaf |
| CLI commands/flags | No | none added |
| CLI grammar (keyword before value) | N-A | no grammar |
| Editor autocomplete | N-A | no leaf |
| Functional test for new RPC/API | Yes | eight `.ci` files |
| Pipe completeness | Yes | `show interface pppoe-client` new keys through the json pipe |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | Yes | eight checks, codes registered by their owners |
| Prometheus counters/metrics | No | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | the doctor feature page under `features/` |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | Yes | `show interface pppoe-client` keys in `docs/guide/command-reference.md` |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/health-checks.md`, `docs/guide/ipsec.md`, `docs/guide/l2tp.md` |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | child 1 owns it |
| 9 | RFC behavior implemented, changed, or newly proven? | No | none |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/doctor-and-health-checks.md` (the runtime checks list) |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | declared by the files this spec changes, each judged: `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` updated (the exemption check reads the SPD dispositions it describes); `docs/architecture/ike/ipsec-7-ikev2-engine.md` updated (two doctor checks move to `runtime`); `docs/architecture/l2tp/cpe-1-pppoe-client.md` updated (the status gains NCP and IPv6 facts); `docs/features/interfaces.md` updated (the bridge STP read and the doctor rows); `ai/rules/repo-maintenance.md` unaffected (the doctor ownership rule it states is followed, not changed); `docs/architecture/hub-architecture.md` unaffected (the resolver construction keeps its place in startup); `docs/research/l2tpv2-ze-integration.md` and `docs/research/vpp-deployment-reference.md` unaffected (research records, no behavior claim changes); the rest derived by `./le spec citation anchors spec plan/release/validation/spec-doctor-coverage-2-home-cpe-batch.md` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/ipsec.md` discard examples |

## Implementation Steps

1. **Phase: Wiring** -- register the eight checks returning nothing; the `.ci` files fail
2. **Phase: Config classes** -- 1, 4 (config half), 5, with the STP read and the journal-row undo fix
3. **Phase: L2TP** -- the unanswered record, class 2
4. **Phase: PPPoE and names** -- status facts, classes 6 and 7
5. **Phase: IKE runtime** -- class 8, move the two runtime readers
6. **Phase: Resolver** -- class 3, after spec-23 lands
7. **Phase: Docs** -- each phase edits its page as it lands

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | each AC at file:line |
| Correctness | no check reads healthy for state it could not see; thresholds from protocol timers |
| Naming | codes kebab-case, `doctor-` prefix |
| Data flow | names through the resolver; IKE through engine or inventory |
| Rule: user-facing-errors | every fix a command on its own line |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| eight checks | `ze show diagnostic <code>` answers for every code |
| tests | the unit and `.ci` tests above pass |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Secrets | PPPoE and L2TP rows never print `AuthSecret` or shared secrets |
| Resource use | runtime checks read snapshots, no probe floods |

### Failure Routing

| Failure | Route To |
|---------|----------|
| A class has no readable state | back to child 1 inventory |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The bridge STP undo guesses the old state (`!e.STP`); the read class 5 needs also fixes it.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `plan/journal/rollback-forgets-partial-apply.md` | 2026-11-08 | bridge STP undo sets `!e.STP` |
| `plan/journal/parser-refuses-a-form-its-producer-emits.md` | 2026-11-09 | `bridgeLinkInfo` keeps the IFLA_INFO_KIND NUL |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| L2TP keeps a per-peer unanswered record | read the report bus for the Warn | the bus is process-local and evicts; the record is the owner's state, cleared when the fault ends |
| Class 4 derives consumers from the YANG type | a list of interface-using features | a list is a central enumeration (`ai/rules/principles.md`) |
| Class 1 checks coverage per flow | report only that the discard is in force | the start log already says it is in force; the operator needs to know which flow is cut |

## Known Limitations
- Class 3 depends on spec-23's producer; until it lands the check reads the hub's resolver configuration (A-6).

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests pass in the main checkout: `./le job run label <pkg> command go test -tags <tags>` over `./internal/component/ike/engine/`, `./internal/component/l2tp/`, `./internal/component/iface/`, `./internal/plugins/iface/netlink/`; `./le test functional ui`, `./le test functional plugin`, and `./le test qemu` for the `needs-linux` files, with the tags `./le repo changed scope print both` names
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
- [ ] **Commit A:** code + tests + docs + edited spec + the journal row
- [ ] **Commit B:** `remove plan/release/validation/spec-doctor-coverage-2-home-cpe-batch.md` only
