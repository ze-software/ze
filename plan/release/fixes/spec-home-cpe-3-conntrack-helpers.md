# Spec: home-cpe-3-conntrack-helpers

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P2 |
| Release reason | Release fix once the owner decides: `system conntrack module ftp` is accepted and does nothing on Ze's kernel (no helper built in, none assigned), so FTP, SIP, H.323, PPTP and TFTP through NAT break while the configuration says otherwise |
| RFC gaps | - (RFC 4787 Section 7 REQ-10 bears on the default; no requirement row is changed) |
| Depends | owner decision (below); `plan/release/delivery/spec-ze-loads-kernel-modules-on-vendor-kernels.md` (AC-22, the helper kernel row and the `nfs`/`sqlnet` removal) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-31 |

Parent: `plan/home-cpe/spec-home-cpe-0-umbrella.md` (child 3).

Recovery after compaction: `.claude/rules/post-compaction.md`.

## OWNER DECISION OPEN

Ze's kernel leaves every conntrack helper out, against NAT slipstreaming. A
home CPE that masquerades its LAN needs at least the FTP helper for active FTP,
and a VyOS CPE assigns ftp, h323, pptp, sip, tftp (and the user-space nfs and
sqlnet helpers) by default. The two options:

| | Option A: build helpers in, assign explicitly | Option B: keep helpers out |
|-|-----------------------------------------------|----------------------------|
| Kernel | `CONFIG_NF_CONNTRACK_{FTP,H323,PPTP,SIP,TFTP}=y`, NAT companions follow `NF_NAT`; `IRC`, `SANE`, `AMANDA`, `NETBIOS_NS`, `SNMP` stay `=n` unless listed | unchanged, every helper `=n` |
| `system conntrack module X` | creates the nft `ct helper` object for X and assigns it by protocol and port, in prerouting and output at priority filter - 5, IPv4 and IPv6 (pptp IPv4 only); the list is empty by default | refused at verify on Ze's kernel with an error naming why (helpers are not built, NAT slipstreaming) and the alternatives (passive FTP, SIP over TLS or a VPN); still loads and assigns on a distro kernel that ships the module |
| FTP through masquerade | works (lab scenario `lan-services` asserts it) | active FTP breaks; passive FTP works |
| Slipstreaming exposure | limited to the helpers the operator lists, on the ports listed, from the hooks named; no automatic assignment (Linux removed it in 6.0) | none |
| RFC 4787 Section 7 | REQ-10 ("NAT ALGs for UDP-based protocols SHOULD be turned off") holds by default because the list is empty; REQ-10 a) ("it is RECOMMENDED that the NAT allow the NAT administrator to enable or disable each ALG separately") is met | REQ-10 holds; REQ-10 a) does not apply (no ALG) |
| VyOS migration | maps one to one, minus `nfs` and `sqlnet` | the mapping guide drops the helper block with a reason |
| Other specs | AC-22 of the kernel-modules spec already says "Ze's kernel builds in the helpers it still offers": consistent | AC-22 must be rewritten to "refused on Ze's kernel"; `spec-nat44-customer-napt.md` Decision on REQ-10 stays as it is |

**Recommendation: Option A.** The exposure that made the helpers dangerous
was automatic assignment on every connection, which Linux no longer does; with
an empty default list, explicit per-helper opt-in and the kernel-modules
spec's AC-22 already written that way, Ze ships the RFC 4787 REQ-10 default
and lets the operator turn on the one ALG a home line needs. Under Option B the
configuration leaf would refuse on the appliance and work on a distro kernel,
which is a surprising split for one leaf. The recommendation changes nothing
until the owner decides; this spec stays `design` until then.

The rest of this spec is written for Option A, with the Option B delta in its
own section at the end.

## Task

Make `system conntrack module <helper>` do what it says on Ze's kernel and on a
distro kernel: the helper exists in the kernel, an nft `ct helper` object is
declared for it, and new connections to its well-known ports are assigned to
it, for IPv4 and IPv6 alike. Names that no kernel module provides (`nfs`,
`sqlnet`) are refused at parse time; their removal is owned by the
kernel-modules spec (AC-22), which this spec depends on.

Assignment, matching the reference behaviour of a VyOS CPE:

| Helper | Kernel helper name | Protocol and ports | Families |
|--------|--------------------|--------------------|----------|
| ftp | `ftp` | tcp 21 | ip, ip6 |
| h323 | `RAS` and `Q.931` | udp 1719 (RAS), tcp 1720 (Q.931) | ip, ip6 |
| pptp | `pptp` | tcp 1723 | ip only (no IPv6 helper exists) |
| sip | `sip` | tcp and udp 5060, 5061 | ip, ip6 |
| tftp | `tftp` | udp 69 | ip, ip6 |
| irc, sane, amanda, netbios-ns, snmp | as the kernel names them | their kernel default ports | per the kernel helper's families |

Hooks: prerouting (forwarded and inbound connections) and output (locally
originated), priority filter - 5, so assignment happens before the operator's
filter chains at priority filter.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/configuration.md` (conntrack section, about l.3490 to 3534) - "Load kernel conntrack ALG helpers via `modprobe`. Valid modules: ftp, h323, sip, pptp, tftp, sane, irc, amanda, netbios-ns, snmp ... On gokrazy, modules are built-in and loading is skipped."
  → Constraint: the page is wrong twice: the list omits `nfs` and `sqlnet` that YANG accepts today, and "On gokrazy, modules are built-in" is false (every helper is `=n`). The page is corrected in this spec's first commit, whatever the decision (documentation rule)
- [ ] `docs/architecture/config/syntax.md` - `// Design:` target of `conntrack_linux.go`
- [ ] `docs/guide/firewall.md` - ownership of nft tables, hook and priority vocabulary
  → Decision: the helper table is a ze-owned table produced from config and installed through the firewall backend, the way policy routing does (`internal/plugins/policyroute/translate.go` builds a `firewall.Table` at prerouting priority -150)
- [ ] `plan/release/delivery/spec-ze-loads-kernel-modules-on-vendor-kernels.md` - AC-22, the enrolment that replaces `LoadConntrackModules`, and its owned journal row `silent-fall-through.md` 2026-10-29
  → Constraint: module loading (modprobe, the enrolment) is that spec's; this spec owns only the kernel build of the helpers it keeps and the nft objects and assignment
- [ ] `plan/post-release/features/spec-nat44-customer-napt.md` - records "every conntrack helper is `=n`" as the evidence that REQ-10 is met
  → Constraint: under Option A that evidence becomes "the helper list is empty by default"; its Decision line is repointed in this spec's commit

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4787.md` (present) - Section 7, REQ-10 and REQ-10 a)
  → Constraint: RFC 4787 Section 7: "To eliminate interference with UNSAF NAT traversal mechanisms and allow integrity protection of UDP communications, NAT ALGs for UDP-based protocols SHOULD be turned off."
  → Constraint: RFC 4787 Section 7: "If a NAT includes ALGs, it is RECOMMENDED that the NAT allow the NAT administrator to enable or disable each ALG separately."

**Key insights:**
- Since Linux 6.0 the kernel never assigns a helper on its own (`nf_conntrack_helper` auto-assignment was removed); a helper module present and unassigned does nothing. So a build-in with no assignment is as broken as no build-in.
- An inet-family `ct helper` object resolves both the IPv4 and the IPv6 helper of the same name; pptp has no IPv6 helper, so it is declared in an ip table (A-2).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `gokrazy/kernel/kernel.config` l.102-112 - "Explicitly disable nftables helper modules to prevent NAT slipstreaming attacks", then `CONFIG_NF_CONNTRACK_{AMANDA,FTP,H323,IRC,NETBIOS_NS,SNMP,PPTP,SANE,SIP,TFTP}=n`; `CONFIG_NFT_CT=y` (l.62)
- [ ] `internal/component/config/system/yang/ze-system-conf.yang` l.394-412 - `leaf-list module`, enumeration of twelve names including `nfs` and `sqlnet`; "The leaf-list only loads a module"
- [ ] `internal/component/config/system/conntrack.go` - `CheckConntrackModule` guards modprobe with the YANG enumeration
- [ ] `internal/component/config/system/conntrack_linux.go` - `LoadConntrackModules` returns `nil, nil` when no modprobe is on PATH (the appliance), so a configured helper silently loads nothing; `toKernelModName` builds `nf_conntrack_<name>`
- [ ] `cmd/ze/hub/main_system.go` - `applyConntrackConfig` (l.235) loads modules at start and reload
- [ ] `vendor/github.com/google/nftables/expr/ct.go` - `CtHelper` object type with name, L3 and L4 protocol, already vendored
- [ ] grep of `internal/` for `ct helper` and `CtHelper`: nothing assigns a helper

**Behavior to preserve:**
- Conntrack sysctls (`table-size`, timeouts, TCP behaviour) and their dual-setting refusal.
- `show system conntrack` output shape, which gains the assigned helpers.

**Behavior to change:**
- Helpers listed are built into Ze's kernel (Option A) and assigned through nft.
- The YANG description and the guide stop saying "only loads".

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config `system conntrack module <name>` at boot and on reload.

### Transformation Path
1. Parse: the enumeration (minus `nfs`, `sqlnet`, removed by the kernel-modules spec) yields the helper list.
2. Verify: each name is checked against a helper table that declares, per helper, its kernel helper names, protocols, ports and families. The table is the single declaration; the YANG enumeration is generated from it or checked against it by a test (principles: declare once).
3. Kernel presence: the kernel-modules spec's enrolment answers whether the helper is present (built in or loadable). Absent: verify refuses with the helper name and the fix.
4. Lower: a ze-owned nft table per family set (inet for every helper with both families, ip for pptp) holding one `ct helper` object per kernel helper and two base chains, prerouting and output at priority filter - 5, with one rule per protocol and port setting the helper.
5. Install: through the firewall backend that owns nft tables, replaced atomically on reload; an empty list removes the table.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to system apply | `ExtractConntrackFromMap` | No |
| System apply to firewall backend | an abstract `firewall.Table` with helper objects and a set-helper action | No |
| Backend to kernel | nftables netlink: object add, rule add | No |

### Integration Points
- `internal/component/firewall/model.go` - gains a helper object declaration on a table and a set-helper action; the VPP backend refuses both with an error naming the backend.
- `internal/plugins/firewall/nft/lower_linux.go` - lowers them to `CtHelper` objects and `objref` assignment.
- `cmd/ze/hub/main_system.go` `applyConntrackConfig` - submits the helper table.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes (design) | the helper table goes through the firewall backend, never a second nft client |
| No unintended coupling | Yes (design) | the system config produces an abstract table; it imports no nft package |
| No duplicated functionality | Yes (design) | reuses the firewall table install and policy routing's produced-table precedent |
| Zero-copy preserved | N-A | config path |
| Registration over hardcoding, outbound | Yes (design) | one helper declaration table; adding a helper is one row |
| Registration over hardcoding, inbound | Yes (design) | the YANG enumeration and the guide list derive from, or are tested against, the helper table |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Kernel 7.x has no automatic helper assignment | removal of `nf_conntrack_helper` auto-assign in Linux 6.0 | slipstreaming exposure would be wider than stated | read the sysctl on Ze's kernel in the integration test | unvalidated |
| A-2 | An inet `ct helper` object resolves both family helpers and refuses only when neither exists | `nft_ct_helper_obj_init` in `net/netfilter/nft_ct.c` | per-family tables needed (ip and ip6 as VyOS does) | read the kernel source; integration test with IPv6 FTP | unvalidated |
| A-3 | The NAT companions (`nf_nat_ftp` and siblings) are selected with `NF_NAT=y` and the helper `=y` | Kconfig `NF_NAT_FTP` default | FTP through masquerade still breaks | `runtime.config` check and the lab FTP assertion | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The owner picks Option B | decision | apply the Option B delta below; the doc correction lands either way |
| R-2 | Operator's own chain at priority below filter - 5 runs before assignment | helper not applied | document the priority; verify warns when an operator chain at prerouting has priority lower than -5 |
| R-3 | Helpers built in enlarge the attack surface even unassigned | review | unassigned helpers process nothing; the integration test proves no expectation is created without assignment |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | NAT pinholes opened by a helper (slipstreaming); or FTP and SIP through NAT keep failing |
| How is it reverted? | kernel config revert plus code revert; the table is ze-owned and removed on the next apply |
| Who else touches this path? | the kernel-modules spec (AC-22, enrolment), `spec-nat44-customer-napt.md`, the firewall nft backend |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `system conntrack module ftp` in config | → | helper table lowered and installed | `test/firewall/conntrack-helper-ftp-assigned.ci` (reads `nft list table`) |
| FTP active mode through masquerade | → | helper plus NAT companion in the kernel | lab scenario `lan-services` (child 11), FTP step |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `module ftp` on Ze's kernel | an inet table holds `ct helper` ftp and rules assigning tcp 21 in prerouting and output at priority -5 |
| AC-2 | `module pptp` | the assignment exists in an ip table only; no IPv6 rule |
| AC-3 | `module h323` | both RAS (udp 1719) and Q.931 (tcp 1720) objects and rules |
| AC-4 | `module sip` | tcp and udp 5060 and 5061 |
| AC-5 | Empty list, or the leaf removed on reload | the helper table is absent; no helper assigned (RFC 4787 REQ-10 default) |
| AC-6 | Active FTP from a LAN client to a server beyond masquerade with `module ftp` | the data connection succeeds; without `module ftp` it fails (red phase) |
| AC-7 | A helper whose module the running kernel lacks | verify refuses: names the helper, says the kernel does not provide it, names the fix |
| AC-8 | `firewall` backend vpp with any helper | verify refuses with an error naming the backend |
| AC-9 | `show system conntrack` | lists each assigned helper with its protocol and ports |
| AC-10 | `docs/guide/configuration.md` conntrack section | lists exactly the accepted names and says how helpers are assigned |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Enables the FTP helper and downloads with active FTP from a LAN host | config → helper table → kernel helper → NAT expectation | lab `lan-services` FTP step |
| 2 | Removes the helper | reload → table removed → active FTP fails | `conntrack-helper-removed-on-reload.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestHelperTableMatchesModel` | `internal/component/config/system/conntrack_helpers_test.go` | every YANG enum has a helper row and the reverse | |
| `TestHelperLowering` | same | AC-1..AC-4 abstract tables | |
| `TestLowerCtHelperObject` | `internal/plugins/firewall/nft/lower_linux_test.go` | object and objref expressions | |
| `TestVPPRefusesCtHelper` | `internal/plugins/firewall/vpp/verify_test.go` | AC-8 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | the feature takes no number from the operator | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `conntrack-helper-ftp-assigned` | `test/firewall/conntrack-helper-ftp-assigned.ci` | AC-1, AC-9 | |
| `conntrack-helper-removed-on-reload` | `test/firewall/conntrack-helper-removed-on-reload.ci` | AC-5 | |
| integration (Ze's kernel, QEMU) | `test/integration/` per `docs/architecture/testing/` | AC-6 with a red phase; A-1, A-2 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `lan-services` (FTP step) | owned by child 11 | an FTP server (vsftpd) and client beyond Ze's masquerade | AC-6 against third-party FTP | |

## Files to Modify
- `gokrazy/kernel/kernel.config`, `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require` - helpers kept `=y`, comment rewritten to state the explicit-assignment model
- `internal/component/config/system/yang/ze-system-conf.yang` - description of `module`
- `internal/component/config/system/conntrack.go` - helper table, verify
- `cmd/ze/hub/main_system.go` - `applyConntrackConfig` submits the helper table
- `internal/component/firewall/model.go`, `validate.go` - helper object, set-helper action
- `internal/plugins/firewall/nft/lower_linux.go` - lowering
- `internal/plugins/firewall/vpp/verify.go` - refusal
- `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` - corrected
- `plan/post-release/features/spec-nat44-customer-napt.md` - REQ-10 Decision evidence repointed

## Files to Create
- `internal/component/config/system/conntrack_helpers.go`, `conntrack_helpers_test.go`
- `test/firewall/conntrack-helper-ftp-assigned.ci`, `conntrack-helper-removed-on-reload.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | Yes | description only; enum edits are the kernel-modules spec's |
| YANG validation constraints | No | enumeration exists |
| YANG custom validators | Yes | kernel-presence check at verify |
| CLI commands/flags | No | `show system conntrack` extended, no new command |
| CLI grammar | N-A | no new command |
| Editor autocomplete | N-A | enum completes already |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | Yes | the extended `show` output goes through the existing pipe path |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | Yes | a configured helper the kernel lacks is a doctor finding (shared with the kernel-modules enrolment) |
| Prometheus counters/metrics | No | none |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `features/` conntrack entry |
| 2 | Config syntax changed? | No | semantics only; `configuration.md` corrected |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` for `show system conntrack` |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md`, `docs/guide/firewall.md` (ze-owned table) |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior changed? | Yes | `rfc/short/rfc4787.md` REQ-10 evidence |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` ALG row if present |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/syntax.md` |
| 13 | Route metadata keys? | No | none |
| 14 | Prometheus counters? | No | none |
| 15 | Registered inventory changed? | No | none |
| 16 | Doc source anchors on changed files? | Yes | `./le spec citation anchors spec plan/home-cpe/spec-home-cpe-3-conntrack-helpers.md` |
| 17 | Existing examples? | Yes | `configuration.md` l.3494 and l.3567 conntrack examples |

## Implementation Steps

0. **Before the decision:** the guide correction (AC-10 for today's behaviour: the list as accepted, "On gokrazy ... built-in" removed) is owed now by the documentation rule and lands as its own commit.
1. **Phase: Wiring** - the helper table and an empty lowering path reach the backend; `conntrack-helper-ftp-assigned.ci` fails.
2. **Phase: Model and lowering** - firewall helper object and action, nft lowering, VPP refusal.
3. **Phase: Kernel** - helpers `=y`, runtime config and require lists, integration test with red phase.
4. **Phase: Show and doctor** - AC-7, AC-9.
5. **Phase: Docs** - guide, syntax page, NAT44 spec Decision repoint.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | No helper assigned when the list is empty; table removed on reload |
| Correctness | pptp never in an IPv6 path |
| Data flow | only the firewall backend talks nftables |
| Rule: principles | no silent no-op: a listed helper either works or is refused |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Helpers in the kernel | `runtime.config` lists them; integration test reads `/proc/config.gz` |
| Assignment | `nft list ruleset` assertion in the `.ci` |
| FTP through NAT | lab FTP step green, red without the helper |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Exposure | assignment only on listed ports; no catch-all; default empty |
| Fail-open | a lowering error refuses the commit, never installs a partial table |

### Failure Routing

| Failure | Route To |
|---------|----------|
| A-2 refuted | DESIGN: per-family ip and ip6 tables |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Option B delta

If the owner keeps helpers out: kernel unchanged; verify refuses
`system conntrack module` when the running kernel lacks the helper (always on
Ze's kernel), with the reason and the alternatives; on a distro kernel the
module loads (kernel-modules spec) and this spec's assignment still runs, so
the leaf works where the kernel has the helper. AC-1..AC-4 and AC-6 move to
"distro kernel" integration runs; the kernel-modules spec's AC-22 is rewritten
by its owner; the mapping guide (child 10) drops the VyOS helper block with the
reason.

## Design Insights

- Building a helper in without assigning it changes nothing on a 6.0+ kernel, so "load the module" was never enough on any kernel Ze supports.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

`plan/journal/silent-fall-through.md` row 2026-10-29 (`LoadConntrackModules` silent no-op) is owned by `plan/release/delivery/spec-ze-loads-kernel-modules-on-vendor-kernels.md` and stays there; this spec depends on that fix.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Recommend Option A (open, owner decides) | Option B | see the decision table; REQ-10 holds through an empty default |
| One helper declaration table drives YANG, guide and lowering | per-helper switch in the lowering | principles: declare once |
| Install through the firewall backend | a second nft client in config/system | one owner of nft state |
| Priority filter - 5, prerouting and output | forward hook | matches the reference CPE and assigns before NAT and filter decisions |

## Known Limitations

- `nfs` and `sqlnet` need user-space helpers (`nfnetlink_cthelper` and a daemon); Ze provides neither. Their refusal is the kernel-modules spec's AC-22.
- Per-interface or per-direction restriction of assignment is not offered; a later feature if wanted.
