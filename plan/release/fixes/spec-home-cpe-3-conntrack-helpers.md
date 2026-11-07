# Spec: home-cpe-3-conntrack-helpers

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | config |
| Priority | P2 |
| Release reason | Release fix: `system conntrack module ftp` is accepted and does nothing on Ze's kernel (no helper built in, none assigned), so FTP, SIP, H.323, PPTP and TFTP through NAT break while the configuration says otherwise |
| RFC gaps | - |
| Depends | `plan/release/delivery/spec-ze-loads-kernel-modules-on-vendor-kernels.md` (AC-22, the helper kernel row and the `nfs`/`sqlnet` removal) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-02 |

Parent: `plan/release/fixes/spec-home-cpe-0-umbrella.md` (child 3).

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner decision: Option A (settled)

The owner decided Option A on 2026-11-02: build the helpers in and assign them
explicitly. The decision is settled; the umbrella and the lab (child 11) carry
it, and the lab's FTP clause is unconditional. One question the decision left
open, which helpers beyond the five VyOS assigns stay offered, is D-1 below.

Ze's kernel leaves every conntrack helper out, against NAT slipstreaming. A
home CPE that masquerades its LAN needs at least the FTP helper for active FTP,
and a VyOS CPE assigns ftp, h323, pptp, sip, tftp (and the user-space nfs and
sqlnet helpers) when listed. The two options weighed:

| | Option A: build helpers in, assign explicitly (chosen) | Option B: keep helpers out |
|-|-----------------------------------------------|----------------------------|
| Kernel | `CONFIG_NF_CONNTRACK_{FTP,H323,PPTP,SIP,TFTP}=y`, NAT companions follow `NF_NAT`; `IRC`, `SANE`, `AMANDA`, `NETBIOS_NS`, `SNMP` per D-1 | unchanged, every helper `=n` |
| `system conntrack module X` | creates the nft `ct helper` object for X and assigns it by protocol and port, in prerouting and output at priority filter - 5, in the families the kernel helper registers (helper table below); the list is empty by default | refused at verify on Ze's kernel with an error naming why (helpers are not built, NAT slipstreaming) and the alternatives (passive FTP, SIP over TLS or a VPN); still loads and assigns on a distro kernel that ships the module |
| FTP through masquerade | works (lab scenario `lan-services` asserts it) | active FTP breaks; passive FTP works |
| Slipstreaming exposure | limited to the helpers the operator lists, on the ports listed, from the hooks named; no automatic assignment (removed from Linux in 6.0; `v7.3` `__nf_ct_try_assign_helper` assigns only from a template an explicit rule set) | none |
| RFC 4787 Section 7 | REQ-10 ("NAT ALGs for UDP-based protocols SHOULD be turned off") holds by default because the list is empty; REQ-10 a) ("it is RECOMMENDED that the NAT allow the NAT administrator to enable or disable each ALG separately") is met | REQ-10 holds; REQ-10 a) does not apply (no ALG) |
| VyOS migration | maps one to one, minus `nfs` and `sqlnet` | the mapping guide drops the helper block with a reason |
| Other specs | AC-22 of the kernel-modules spec already says "Ze's kernel builds in the helpers it still offers": consistent | AC-22 must be rewritten to "refused on Ze's kernel"; `spec-nat44-customer-napt.md` Decision on REQ-10 stays as it is |

**Why Option A.** The exposure that made the helpers dangerous
was automatic assignment on every connection, which Linux no longer does; with
an empty default list, explicit per-helper opt-in and the kernel-modules
spec's AC-22 already written that way, Ze ships the RFC 4787 REQ-10 default
and lets the operator turn on the one ALG a home line needs. Under Option B the
configuration leaf would refuse on the appliance and work on a distro kernel,
which is a surprising split for one leaf.

## Open decision

| ID | Question | Options | Evidence | Effect |
|----|----------|---------|----------|--------|
| D-1 | The YANG enumeration also offers `irc`, `sane`, `amanda`, `netbios-ns` and `snmp`, which VyOS does not assign and the Option A kernel row leaves `=n`. Which applies? | (a) build all ten in, so every offered name works on Ze's kernel; (b) stop offering the five, in the same enumeration change the kernel-modules spec makes for `nfs` and `sqlnet`; (c) keep offering them, refused on Ze's kernel by AC-7 and working on a distro kernel | VyOS `src/conf_mode/system_conntrack.py` `module_map` (vyos-1x `1.5dev0-5851-gdb29bdc89`) assigns only ftp, h323, nfs, pptp, rtsp, sip, sqlnet, tftp; AC-22 of the kernel-modules spec says the kernel "builds in the helpers it still offers"; Option A's rationale rejects a leaf that works on a distro kernel and refuses on the appliance, which is what (c) produces; slipstreaming exposure grows with each helper built in, even unassigned (R-3) | (a): five more `=y` rows and their NAT companions. (b): the five rows leave the helper table and the enumeration. (c): the five rows stay, marked distro-kernel only. The tests below cover every row that remains, whichever option is chosen. Owed to Thomas |

## Task

Make `system conntrack module <helper>` do what it says on Ze's kernel and on a
distro kernel: the helper exists in the kernel, an nft `ct helper` object is
declared for it, and new connections to its ports are assigned to it, in every
address family the kernel helper registers. Names that no kernel module
provides (`nfs`, `sqlnet`) are refused at parse time; their removal is owned by
the kernel-modules spec (AC-22), which this spec depends on.

### The helper table

The kernel registers no port for any helper: in `v7.3`, `nf_ct_helper_init`
takes no port and none of these modules has a `ports` parameter left. The port
a helper serves is therefore decided only by Ze's assignment rule, and the
table below is the single declaration of it. Families, kernel helper names and
Kconfig symbols are read from the `v7.3` sources (`net/netfilter/nf_conntrack_*.c`,
the `nf_ct_helper_init` calls; `net/netfilter/Kconfig` and
`net/ipv4/netfilter/Kconfig` for the NAT companions). Ports come from VyOS's
`module_map` for the five VyOS assigns, and from the IANA service registry
(`/etc/services` names in brackets) for the rest.

| Helper | Kernel helper name(s) | Protocol and ports | Families the kernel registers | Kernel symbol, NAT companion | Ze table | Port source |
|--------|-----------------------|--------------------|-------------------------------|------------------------------|-----------|-------------|
| ftp | `ftp` | tcp 21 | ip, ip6 (one `NFPROTO_UNSPEC` helper) | `NF_CONNTRACK_FTP`; `NF_NAT_FTP` (default `NF_NAT && NF_CONNTRACK_FTP`) | inet | VyOS |
| h323 | `RAS`, `Q.931` (`H.245` is reached through Q.931's expectation, never by port) | udp 1719 (RAS), tcp 1720 (Q.931) | ip, ip6 (one helper per family for RAS and for Q.931) | `NF_CONNTRACK_H323`; `NF_NAT_H323` (IPv4 NAT, default `NF_CONNTRACK_H323`) | inet | VyOS |
| pptp | `pptp` | tcp 1723 | ip only (`AF_INET`) | `NF_CONNTRACK_PPTP`; `NF_NAT_PPTP` (default `NF_CONNTRACK_PPTP`) | ip | VyOS |
| sip | `sip` | udp and tcp 5060, 5061 | ip, ip6 (`NFPROTO_UNSPEC`, one helper per transport) | `NF_CONNTRACK_SIP`; `NF_NAT_SIP` | inet | VyOS (5061 is SIP over TLS, which the helper cannot read; kept for VyOS parity) |
| tftp | `tftp` | udp 69 | ip, ip6 (`NFPROTO_UNSPEC`) | `NF_CONNTRACK_TFTP`; `NF_NAT_TFTP` | inet | VyOS |
| irc (D-1) | `irc` | tcp 6667 | ip only (`AF_INET`) | `NF_CONNTRACK_IRC`; `NF_NAT_IRC` | ip | IANA (`ircd`) |
| sane (D-1) | `sane` | tcp 6566 | ip, ip6 (`NFPROTO_UNSPEC`) | `NF_CONNTRACK_SANE`; no NAT companion | inet | IANA (`sane-port`) |
| amanda (D-1) | `amanda` | udp 10080 | ip, ip6 (one helper per family) | `NF_CONNTRACK_AMANDA`; `NF_NAT_AMANDA` | inet | IANA (`amanda`) |
| netbios-ns (D-1) | `netbios-ns` | udp 137 | ip only (`AF_INET`) | `NF_CONNTRACK_NETBIOS_NS` (selects `NF_CONNTRACK_BROADCAST`); no NAT companion | ip | IANA (`netbios-ns`) |
| snmp (D-1) | `snmp` | udp 161 | ip only (`AF_INET`) | `NF_CONNTRACK_SNMP`; `NF_NAT_SNMP_BASIC` (needs `NETFILTER_ADVANCED`, selects `ASN1`) | ip | IANA (`snmp`) |

A helper the kernel registers for IPv4 only is declared in an ip table, so no
IPv6 rule ever names it. An inet-family `ct helper` object would be accepted
for it (`v7.3` `nft_ct_helper_obj_init` refuses only when neither family
resolves), but its IPv6 rule would assign an object with no IPv6 helper.

Hooks: prerouting (forwarded and inbound connections) and output (locally
originated), priority filter - 5, so assignment happens before the operator's
filter chains at priority filter.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/configuration.md`, the `system conntrack` section - "Load kernel conntrack ALG helpers via `modprobe`. Valid modules: ftp, h323, sip, pptp, tftp, sane, irc, amanda, netbios-ns, snmp ... On gokrazy, modules are built-in and loading is skipped."
  → Constraint: the page is wrong twice: the list omits `nfs` and `sqlnet` that YANG accepts today, and "On gokrazy, modules are built-in" is false (every helper is `=n`). The page is corrected in this spec's first commit (documentation rule)
- [ ] `docs/architecture/config/syntax.md` - `// Design:` target of `conntrack_linux.go`
- [ ] `docs/guide/firewall.md` - ownership of nft tables, hook and priority vocabulary
  → Decision: the helper table is a ze-owned table produced from config and installed through the firewall backend, the way policy routing does (`internal/plugins/policyroute/translate.go`, `allocator.translate`, builds a `firewall.Table` at prerouting priority -150)
- [ ] `plan/release/delivery/spec-ze-loads-kernel-modules-on-vendor-kernels.md` - AC-22, the enrolment that replaces `LoadConntrackModules`, and its owned journal row `silent-fall-through.md` 2026-10-29
  → Constraint: module loading (modprobe, the enrolment) is that spec's; this spec owns only the kernel build of the helpers it keeps and the nft objects and assignment
- [ ] `plan/post-release/features/spec-nat44-customer-napt.md` - records "every conntrack helper is `=n`" as the evidence that REQ-10 is met
  → Constraint: under Option A that evidence becomes "the helper list is empty by default"; its Decision line is repointed in this spec's commit

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4787.md` (present) - Section 7, REQ-10 and REQ-10 a)
  → Constraint: RFC 4787 Section 7: "To eliminate interference with UNSAF NAT traversal mechanisms and allow integrity protection of UDP communications, NAT ALGs for UDP-based protocols SHOULD be turned off."
  → Constraint: RFC 4787 Section 7: "If a NAT includes ALGs, it is RECOMMENDED that the NAT allow the NAT administrator to enable or disable each ALG separately."

**Key insights:**
- Since Linux 6.0 the kernel never assigns a helper on its own; a helper present and unassigned does nothing. So a build-in with no assignment is as broken as no build-in.
- The kernel declares no helper port; Ze's helper table is the only place the port lives.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `gokrazy/kernel/kernel.config` - the block headed "Explicitly disable nftables helper modules to prevent NAT slipstreaming attacks", then `CONFIG_NF_CONNTRACK_{AMANDA,FTP,H323,IRC,NETBIOS_NS,SNMP,PPTP,SANE,SIP,TFTP}=n`; `CONFIG_NFT_CT=y`
- [ ] `gokrazy/kernel/runtime.config` - `CONFIG_NF_CONNTRACK=y`, `CONFIG_NF_CT_NETLINK=y` (ctnetlink, which the integration test reads)
- [ ] `internal/component/config/system/yang/ze-system-conf.yang` - `leaf-list module` under `conntrack`, enumeration of twelve names including `nfs` and `sqlnet`; "The leaf-list only loads a module"
- [ ] `internal/component/config/system/conntrack.go` - `CheckConntrackModule` guards modprobe with the YANG enumeration
- [ ] `internal/component/config/system/conntrack_linux.go` - `LoadConntrackModules` returns `nil, nil` when no modprobe is on PATH (the appliance), so a configured helper silently loads nothing; `toKernelModName` builds `nf_conntrack_<name>`
- [ ] `cmd/ze/hub/main_system.go` - `applyConntrackConfig` loads modules at start and reload
- [ ] `vendor/github.com/google/nftables/expr/ct.go` - `CtHelper` object type with name, L3 and L4 protocol, already vendored
- [ ] grep of `internal/` for `ct helper` and `CtHelper`: nothing assigns a helper
- [ ] Linux `v7.3` (`git.kernel.org` stable tag, read 2026-11-02): `net/netfilter/nf_conntrack_{ftp,h323_main,pptp,sip,tftp,irc,sane,amanda,netbios_ns,snmp}.c` (`nf_ct_helper_init` calls: names, families, protocols, no port); `net/netfilter/nft_ct.c` `nft_ct_helper_obj_init`; `net/netfilter/nf_conntrack_helper.c` `__nf_ct_try_assign_helper`; both `Kconfig` files

**Behavior to preserve:**
- Conntrack sysctls (`table-size`, timeouts, TCP behaviour) and their dual-setting refusal.
- `show system conntrack` output shape, which gains the assigned helpers.

**Behavior to change:**
- Helpers listed are built into Ze's kernel (Option A) and assigned through nft.
- The YANG description and the guide stop saying "only loads".

## Existing Mechanisms (MANDATORY)

| Mechanism | Where | What it does today | This spec |
|-----------|-------|--------------------|-----------|
| Firewall table registry | `internal/component/firewall/registry.go` `RegisterTables`, `ApplyAll` | Each owner registers desired nft tables under an owner key; every name carries the `ze_` prefix so the backend reconciles and sweeps only what Ze owns; tables of the same name and family from several owners merge (`ai/doors/forwarding.md`, "The firewall table registry is process-local"; `docs/architecture/firewall/table-ownership-and-shutdown-flush.md`) | extends: the helper objects and the assignment chains are one more owner's tables, registered here, never a second nft client |
| Firewall backend selection | `internal/component/firewall/backend.go` `RegisterBackend`, `LoadBackend`, `RegisterVerifier` | nft backend for Linux, VPP backend selected by the `backend` leaf; a backend that cannot implement a node refuses it at verify (`ze:backend`, registered verifiers) | extends: `ct helper` lowering goes into the nft backend; VPP refuses `system conntrack module` at verify through its registered verifier |
| Conntrack module loading | `internal/component/config/system/conntrack_linux.go` `LoadConntrackModules`, `cmd/ze/hub/main_system.go` `applyConntrackConfig` | modprobes `nf_conntrack_<name>`, and silently does nothing where no modprobe exists (the appliance) | extends on a distro kernel (module load stays, owned by the kernel-modules spec); on Ze's kernel the helpers are built in, so the silent no-op stops mattering once assignment exists |
| Kernel module loading on vendor kernels | `plan/release/delivery/spec-ze-loads-kernel-modules-on-vendor-kernels.md` (AC-22) | plans built-in helpers on Ze's kernel and modprobe on a vendor kernel | does not duplicate: this spec depends on it for the module half and adds the nft assignment it lacks |
| Kernel tunables | `internal/component/sysctl` | owns every sysctl, config over plugin defaults | does not apply: helper assignment is nft state, not a sysctl (`net.netfilter.nf_conntrack_helper` auto-assignment is gone since Linux 6.0) |
| Interface name resolution | `docs/architecture/iface/logical-name-resolution.md` | turns configured names into devices | does not apply: assignment matches by protocol and port, not by interface |

RFC 4787 Section 7 REQ-10 bears on the default (helpers off unless listed); no
requirement row changes.

Design documents the code this spec touches declares, each checked for what
the change makes wrong: `docs/architecture/core-design.md` (sections 14a and
14b: the backend gate and the firewall backends, which gain the `ct helper`
lowering and the VPP refusal), `docs/architecture/firewall/fw-6-firewall-vpp.md`
(the VPP backend's list of refused features gains `system conntrack module`),
`docs/architecture/hub-architecture.md` (names the hub's system apply step;
the conntrack apply moves from module loading alone to module loading plus table
registration).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config `system conntrack module <name>` at boot and on reload.

### Transformation Path
1. Parse: the enumeration (minus `nfs`, `sqlnet`, removed by the kernel-modules spec, and per D-1) yields the helper list.
2. Verify: each name is checked against the helper table, which declares per helper its kernel helper names, protocols, ports and families. The table is the single declaration; the YANG enumeration is generated from it or checked against it by a test (principles: declare once).
3. Kernel presence: the kernel-modules spec's enrolment answers whether the helper is present (built in or loadable). Absent: verify refuses with the helper name and the fix.
4. Lower: a ze-owned inet table for the helpers registered in both families and a ze-owned ip table for the IPv4-only ones, each holding one `ct helper` object per kernel helper and protocol and two base chains, prerouting and output at priority filter - 5, with one rule per protocol and port setting the helper.
5. Install: the firewall engine owns the helper tables through the config transaction. Its registration takes `system/conntrack` as a second config root, so a config listing a helper starts the engine with no `firewall {}` block. Verify lowers the helpers and judges them with the firewall tables through `ValidateTables` and the selected backend's verifier, so `ze config validate` and the commit refuse what cannot apply (VPP). Apply registers them under their own owner beside the firewall tables and reconciles once; the apply journal restores the previous helpers on a failed reload or a rollback. A reload delivers only the roots that changed, so the root a delivery does not carry keeps its running value, and a deleted block arrives as an empty section, which is an empty list: the tables are removed. A clean stop removes them (only the engine's own owners when no `firewall {}` block exists). The hub keeps the module load and the sysctls only.
   → Decision: the hub's post-commit call was deleted (no layering); review findings B-1 and I-1 on it are fixed by construction.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to firewall engine | transaction section for root `system/conntrack`, read by `conntrackSectionModules` through `ExtractConntrackFromMap` | `TestConntrackSectionRemovedWithdrawsHelpers`, `TestConntrackSectionUndeliveredKeepsRunningValue` |
| Firewall engine to backend | an abstract `firewall.Table` with helper objects and a set-helper action, registered under `conntrack-helpers` | `TestVerifyRunsBackendVerifierOverHelperTables`, `TestHelperTablesInstallWithdrawAndShow` |
| Backend to kernel | nftables netlink: object add, rule add | No |

### Integration Points
- `internal/component/firewall/model.go` - gains a helper object declaration on a table and a set-helper action; the VPP backend refuses both with an error naming the backend.
- `internal/plugins/firewall/nft/lower_linux.go` - lowers them to `CtHelper` objects and `objref` assignment.
- `internal/component/firewall/register.go` and `engine.go` - the second config root and the verify, apply, rollback and stop paths that carry the helper tables.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes (design) | the helper tables go through the firewall backend, never a second nft client |
| No unintended coupling | Yes (design) | the system config produces an abstract table; it imports no nft package |
| No duplicated functionality | Yes (design) | reuses the firewall table install and policy routing's produced-table precedent |
| Zero-copy preserved | N-A | config path |
| Registration over hardcoding, outbound | Yes (design) | one helper declaration table; adding a helper is one row |
| Registration over hardcoding, inbound | Yes (design) | the YANG enumeration and the guide list derive from, or are tested against, the helper table |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Kernel 7.x has no automatic helper assignment | `v7.3` `__nf_ct_try_assign_helper` assigns only from a template's helper (read 2026-11-02) | slipstreaming exposure would be wider than stated | source read; `TestConntrackHelperNotAssignedWhenUnlisted` on Ze's kernel | source-confirmed, test owed |
| A-2 | An inet `ct helper` object resolves both family helpers and refuses only when neither exists | `v7.3` `nft_ct_helper_obj_init` (`NFPROTO_INET` case, "only error if INET found neither ipv4 or ipv6") | per-family tables needed (ip and ip6 as VyOS does) | source read; `TestConntrackHelperAssignment` IPv6 rows | source-confirmed, test owed |
| A-3 | The NAT companions are selected by default once the helper is `=y` with `NF_NAT=y` | `v7.3` Kconfig: `NF_NAT_{FTP,SIP,TFTP,IRC,AMANDA}` default `NF_NAT && NF_CONNTRACK_X`; `NF_NAT_{PPTP,H323}` default `NF_CONNTRACK_X`; `NF_NAT_SNMP_BASIC` also needs `NETFILTER_ADVANCED` | FTP through masquerade still breaks | `runtime.config` and `runtime.require` list each companion; `TestConntrackHelperFTPThroughMasquerade` | source-confirmed, test owed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-2 | Operator's own chain at priority below filter - 5 runs before assignment | helper not applied | document the priority; verify warns when an operator chain at prerouting has priority lower than -5 |
| R-3 | Helpers built in enlarge the attack surface even unassigned | review | unassigned helpers process nothing; `TestConntrackHelperNotAssignedWhenUnlisted` proves no helper and no expectation without assignment |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | NAT pinholes opened by a helper (slipstreaming); or FTP and SIP through NAT keep failing |
| How is it reverted? | kernel config revert plus code revert; the tables are ze-owned and removed on the next apply |
| Who else touches this path? | the kernel-modules spec (AC-22, enrolment), `spec-nat44-customer-napt.md`, the firewall nft backend |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `system conntrack module ftp` in config | → | helper table lowered and installed | `test/firewall/conntrack-helper-ftp-assigned.ci` (reads `nft list table`) |
| every helper row, every family it registers | → | object, assignment rule, kernel helper | `TestConntrackHelperAssignment` |
| FTP active mode through masquerade | → | helper plus NAT companion in the kernel | `TestConntrackHelperFTPThroughMasquerade`; lab scenario `lan-services` (child 11), FTP clause |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `module ftp` on Ze's kernel | an inet table holds `ct helper` ftp and rules assigning tcp 21 in prerouting and output at priority -5 |
| AC-2 | `module pptp` (and every other IPv4-only row kept by D-1) | the assignment exists in an ip table only; no IPv6 rule; an IPv6 connection to the port carries no helper |
| AC-3 | `module h323` | RAS (udp 1719) and Q.931 (tcp 1720) objects and rules |
| AC-4 | `module sip` | udp and tcp 5060 and 5061 |
| AC-5 | Empty list, or the leaf removed on reload | the helper tables are absent; no helper assigned (RFC 4787 REQ-10 default) |
| AC-6 | Active FTP from a LAN client to a server beyond masquerade with `module ftp` | the data connection succeeds; without `module ftp` it fails (red phase) |
| AC-7 | A helper whose module the running kernel lacks | verify refuses: names the helper, says the kernel does not provide it, names the fix |
| AC-8 | `firewall` backend vpp with any helper | verify refuses with an error naming the backend |
| AC-9 | `show system conntrack` | lists each assigned helper with its protocol and ports |
| AC-10 | `docs/guide/configuration.md` conntrack section | lists exactly the accepted names and says how helpers are assigned |
| AC-11 | Each helper row kept after D-1, each family its row names, each protocol and port | on Ze's kernel the object is accepted, and a new connection to that protocol and port carries the row's kernel helper name in its conntrack entry (read over ctnetlink) |
| AC-12 | `module tftp` | udp 69, both families |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Enables the FTP helper and downloads with active FTP from a LAN host | config → helper table → kernel helper → NAT expectation | `TestConntrackHelperFTPThroughMasquerade`; lab `lan-services` FTP clause |
| 2 | Removes the helper | reload → tables removed → active FTP fails | `conntrack-helper-removed-on-reload.ci`; the red phase of `TestConntrackHelperFTPThroughMasquerade` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestHelperTableMatchesModel` | `internal/component/config/system/conntrack_helpers_test.go` | every YANG enum has a helper row and the reverse | |
| `TestHelperLowering` | same | AC-1..AC-4, AC-12 abstract tables, one case per row and family | |
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
| `conntrack-helper-block-deleted-on-reload` | `test/firewall/conntrack-helper-block-deleted-on-reload.ci` | AC-5: the second config has no `system conntrack` block at all | |
| `TestConntrackHelperAssignment` | `internal/component/config/system/conntrack_helpers_integration_linux_test.go` (`integration && linux`) | AC-2, AC-11: one subtest per row, family, protocol and port; installs the lowered tables through the nft backend in a throwaway netns, opens the connection, reads the helper name from the conntrack entry; IPv4-only rows also assert the IPv6 connection carries none | |
| `TestConntrackHelperNotAssignedWhenUnlisted` | same file | A-1, R-3, AC-5: with the helpers built in and the list empty, a connection to tcp 21 carries no helper and a PORT command creates no expectation | |
| `TestConntrackHelperFTPThroughMasquerade` | same file | AC-6 in both families (PORT over IPv4, EPRT over IPv6): client netns, masquerading router netns, server netns; the data connection completes; red phase: the ftp row removed from the configured list, the data connection fails | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `lan-services` (FTP clause, AC-7 of child 11) | owned by child 11, `test/interop-topology/scenarios/lan-services/` | an FTP server and client beyond Ze's masquerade | AC-6 against third-party FTP | |

FTP is the helper with a third-party proof. Every other row is covered by
`TestConntrackHelperAssignment` (object accepted, connection assigned, in each
family the row names); what each kernel helper then parses in its protocol's
messages is the kernel's code and is not re-proven per helper here.

## Files to Modify
- `docs/architecture/core-design.md` - sections 14a and 14b: the nft backend lowers `ct helper`, the VPP backend refuses it at verify
- `docs/architecture/firewall/fw-6-firewall-vpp.md` - the VPP backend's refused features gain `system conntrack module`
- `docs/architecture/hub-architecture.md` - the system apply step registers helper tables as well as loading modules
- `gokrazy/kernel/kernel.config`, `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require` - helpers and NAT companions kept `=y` per the helper table and D-1, comment rewritten to state the explicit-assignment model
- `internal/component/config/system/yang/ze-system-conf.yang` - description of `module`
- `internal/component/config/system/conntrack.go` - helper table, verify
- `cmd/ze/hub/main_system.go` - `applyConntrackConfig` submits the helper tables
- `internal/component/firewall/model.go`, `validate.go` - helper object, set-helper action
- `internal/plugins/firewall/nft/lower_linux.go` - lowering
- `internal/plugins/firewall/vpp/verify.go` - refusal
- `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` - corrected
- `plan/post-release/features/spec-nat44-customer-napt.md` - REQ-10 Decision evidence repointed

## Files to Create
- `internal/component/config/system/conntrack_helpers.go`, `conntrack_helpers_test.go`
- `internal/component/config/system/conntrack_helpers_integration_linux_test.go`
- `test/firewall/conntrack-helper-ftp-assigned.ci`, `conntrack-helper-removed-on-reload.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | Yes | description only; enum edits are the kernel-modules spec's (and D-1's) |
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
| 16 | Doc source anchors on changed files? | Yes | `./le spec citation anchors spec plan/release/fixes/spec-home-cpe-3-conntrack-helpers.md` |
| 17 | Existing examples? | Yes | the conntrack examples in the `system conntrack` section of `configuration.md` |

## Implementation Steps

0. **Phase: Guide correction** - the guide's statement of today's behaviour (the list as accepted, "On gokrazy ... built-in" removed) is owed now by the documentation rule and lands as its own commit.
1. **Phase: Wiring** - the helper table and an empty lowering path reach the backend; `conntrack-helper-ftp-assigned.ci` fails.
2. **Phase: Model and lowering** - firewall helper object and action, nft lowering, VPP refusal.
3. **Phase: Kernel** - helpers and companions `=y` per the table and D-1, runtime config and require lists, the three integration tests with the red phase.
4. **Phase: Show and doctor** - AC-7, AC-9.
5. **Phase: Docs** - guide, syntax page, NAT44 spec Decision repoint.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | No helper assigned when the list is empty; tables removed on reload |
| Correctness | an IPv4-only helper never in an IPv6 path |
| Data flow | only the firewall backend talks nftables |
| Rule: principles | no silent no-op: a listed helper either works or is refused |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Helpers in the kernel | `runtime.config` lists them; `TestConntrackHelperAssignment` passes on Ze's kernel |
| Assignment | `nft list ruleset` assertion in the `.ci`; ctnetlink readback per row and family |
| FTP through NAT | `TestConntrackHelperFTPThroughMasquerade` and the lab FTP clause green, red without the helper |

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

## Design Insights

- Building a helper in without assigning it changes nothing on a 6.0+ kernel, so "load the module" was never enough on any kernel Ze supports.
- The kernel no longer carries a default port for any helper, so the port list is Ze's own declaration and must be complete.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

`plan/journal/silent-fall-through.md` row 2026-10-29 (`LoadConntrackModules` silent no-op) is owned by `plan/release/delivery/spec-ze-loads-kernel-modules-on-vendor-kernels.md` and stays there; this spec depends on that fix.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Option A, decided by the owner on 2026-11-02: helpers built in, assigned only when listed, empty default list | Option B: keep every helper out and refuse `system conntrack module` on Ze's kernel, working only on a distro kernel that ships the module | see the decision table; REQ-10 holds through an empty default, and one leaf behaves the same on the appliance and on a distro kernel |
| One helper declaration table drives YANG, guide and lowering | per-helper switch in the lowering | principles: declare once |
| Install through the firewall backend | a second nft client in config/system | one owner of nft state |
| Priority filter - 5, prerouting and output | forward hook | matches the reference CPE and assigns before NAT and filter decisions |
| IPv4-only helpers in an ip table | one inet table for all | the kernel would accept an inet object, but its IPv6 rule would assign an object with no IPv6 helper |

## Known Limitations

- `nfs` and `sqlnet` need user-space helpers (`nfnetlink_cthelper` and a daemon); Ze provides neither. Their refusal is the kernel-modules spec's AC-22.
- Per-interface or per-direction restriction of assignment is not offered; a later feature if wanted.

## Checklist

### Pre-Spec Verification
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Existing Mechanisms table names every mechanism this spec extends
- [ ] No code snippets
- [ ] D-1 answered by Thomas before Phase 3

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Targeted tests pass: `./le test unit plugins`; `./le test firewall`; `./le test integration firewall`; on Ze's kernel, `./le test qemu run kernel tmp/kernel/build/vmlinuz packages "nftables iproute2 libcap" timeout 1200s command 'go test -tags integration -count=1 -run TestConntrackHelper ./internal/component/config/system/'`; then `./le verify changed` over the closing commits
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests: the `lan-services` FTP clause of child 11

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
