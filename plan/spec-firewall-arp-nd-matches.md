# Spec: firewall-arp-nd-matches

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The firewall model holds three match types that no operator can write:
`MatchARPOperation`, `MatchARPSenderAddress` and `MatchNDTargetAddress`
(`internal/component/firewall/model.go`). Commit `850eb41b66` added them for the
VRRP address owner's ARP and ND suppression (`internal/plugins/vrrp/ownerfilter.go`),
and `2bfb569b65` split their guards into one fact each. Each type's doc comment
says "Daemon-only. No config leaf produces it, so an operator cannot write it."
The YANG `from-block` grouping (`internal/component/firewall/yang/ze-firewall-conf.yang`)
has no leaf for any of them, yet it already offers `family arp` for a table, so an
operator can build an arp table and has nothing ARP-specific to match in it.

Goal: an operator writes an ARP operation match, an ARP sender-address match and
an ND target-address match in a `firewall { table ... { chain ... { term ... { from { } } } } }`
block, Ze refuses each one in a family where it would read the wrong bytes,
lowers each to nftables through the lowering that already exists, reports a
kernel without the arp nftables family before it starts, prints the matches in
`show firewall`, and never lets an operator table break the VRRP owner tables.

Two defects found while researching this ride along because the goal depends on
them (the rows are in `plan/journal/`):

| Defect | Journal row | Why the goal depends on it |
|--------|-------------|----------------------------|
| A literal `source-address` or `destination-address` in a table of family arp is accepted and reads ARP header bytes at IPv4 offsets | `plan/journal/guard-added-to-one-half-of-a-pair.md`, 2026-09-28 | Exposing ARP matches invites operators to write arp tables; every other `from` leaf they reach for there must refuse rather than misread |
| An operator table whose `ze_` name equals a VRRP table name is merged into it by `mergeSameNameTables` | `plan/journal/two-owners-share-one-name.md`, 2026-09-28 | The brief requires that an operator rule cannot break the VRRP owner tables. Decision D-3 below settles whether the fix lands here |

## Required Reading

### Architecture Docs
- [ ] `docs/guide/firewall.md` - operator page: families, `from` leaf table, "Tables another feature installs"
  → Constraint: the match table (section "Match Types (from block)") lists config key, meaning and one example per leaf; the three new leaves get rows there, and "Table Families" gains which leaves each family admits
  → Decision: "Tables another feature installs" is where the VRRP owner tables and the interaction with an operator arp table are explained
- [ ] `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` - producer list and Rule 2 (`ze_` prefix)
  → Constraint: the producer table omits `ze_vrrp`, `ze_vrrp_owner_arp` and `ze_vrrp_owner_nd`; the page is wrong today and is repaired in the phase that touches the registry
- [ ] `docs/architecture/doctor-and-health-checks.md` - "Kernel capabilities: one enrolment, three callers"
  → Constraint: a kernel feature need is enrolled with `kernelcap.MustRegister` from the owning package; `Probe` reads netlink or procfs and MUST NOT execute a binary; Absent refuses start, reload and `ze config validate`, Unknown only warns
  → Decision: the arp-family need is enrolled by the nft backend package (`internal/plugins/firewall/nft/`), because that package is the one that programs `NFPROTO_ARP`
- [ ] `docs/architecture/config/yang-config-design.md` - `ze:backend` extension row
  → Constraint: `ze:backend "nft"` restricts a node to the nft backend, commit validates it and completion filters on it; the vpp backend has no ARP or ND lowering (`plan/immediate/spec-vrrp-owner-arp-rfc-defects.md` D2), so each new leaf carries `ze:backend "nft"`
- [ ] `ai/patterns/config-option.md` - YANG leaf definition and naming across layers
  → Constraint: leaf names are spelled in full; no env var applies (a match is per-term data, not a tunable)
- [ ] `ai/rules/config.md` - help text contract
  → Constraint: each leaf and each enum value carries `ze:help` (at most 96 characters and 25 words) and each leaf a `description` that differs from it; `./le doc yang-contract help-shape` refuses otherwise

### External References (naming)
- [ ] nftables manual, ARP HEADER EXPRESSION and ICMPV6 HEADER EXPRESSION (netfilter.org manpage, read 2026-09-28)
  → Decision: nftables spells the fields `arp operation` (request, reply), `arp saddr ip`, `arp daddr ip`, `arp saddr ether`, `arp daddr ether`, and `icmpv6 taddr` for the ND target; the arp family has only the input and output hooks
- [ ] VyOS firewall documentation, bridge section (read 2026-09-28)
  → Constraint: VyOS offers only `ethernet-type arp` and no ARP field match, so there is no VyOS spelling to follow
- [ ] Junos firewall filters (from memory, unverified): `family inet6` filters match `icmp-type neighbor-solicit` but no ND target; ARP is policed, not field-matched
  → Decision: no vendor spelling exists to copy, so the names follow Ze's own `from` leaves (`source-address`, `icmpv6-type`) and the words `show firewall` already prints for these three matches

### RFC Text (packet layouts only; filtering is operator policy, so no `rfc/short/` summary is owed and none exists for either RFC)
- [ ] RFC 826 layout as drawn above `lowerARPOperationMatch` (`internal/plugins/firewall/nft/lower_linux.go`); `rfc/full/` holds no copy, fetch `rfc826.txt` before quoting it
  → Constraint: ar$op precedes the variable-length addresses; ar$spa sits at offset 14 only for the Ethernet/IPv4 layout, which `lowerARPSenderAddressMatch` already guards
- [ ] `rfc/full/rfc4861.txt` - ND message formats, Sections 4.3 and 4.4
  → Constraint: the Target Address sits at ICMPv6 octet 8 in both Neighbor Solicitation (135) and Neighbor Advertisement (136) and nowhere else, so any other ICMPv6 type puts different bytes at that offset

**Key insights:**
- The model, validation, nft lowering and `show` formatting of all three matches already exist and are exercised by VRRP; this spec adds the config surface, closes the family gaps that exposure opens, and enrolls the kernel need.
- `family arp` is already a YANG enum value and a model family (`FamilyARP`), with hooks mapped to `NF_ARP_IN` and `NF_ARP_OUT` by `lowerARPHook`. No new family is needed.
- `MatchNDTargetAddress` does not restrict the ICMPv6 type; its contract says a term MUST carry `MatchICMPv6Type` 135 or 136 beside it, and nothing enforces that.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/model.go` - `TableFamily` with `FamilyARP` named "arp" in `familyNames`; `ARPOperation` (Unspecified 0, Request 1, Reply 2); the three match structs, each documented daemon-only
- [ ] `internal/component/firewall/config.go` - `parseFromBlock` maps each `from` leaf to one Match by string key; `parseICMPv6Type` and `icmp6Types` name 135 `nd-neighbor-solicit` and 136 `nd-neighbor-advert`
- [ ] `internal/component/firewall/validate.go` - `validateMatch`: `MatchARPOperation` and `MatchARPSenderAddress` refused outside `FamilyARP`, opcode other than request or reply refused, sender must be a specified IPv4 address; `MatchNDTargetAddress` refused outside ip6 and inet and must be a specified, non-mapped IPv6 address; `validateSetFamilyCompat` refuses a set address match in arp, bridge and netdev while literal address matches have no family case
- [ ] `internal/component/firewall/yang/ze-firewall-conf.yang` - typedef `table-family` has enum `arp` (help "ARP"); grouping `from-block` has source-address, destination-address, source-port, destination-port, protocol, input-interface, output-interface, connection-state, connection-mark, mark, dscp, icmp-type, icmpv6-type; the `family` leaf description says arp, bridge and netdev refuse dscp, dscp-set and tcp-mss-set
- [ ] `internal/plugins/firewall/nft/lower_linux.go` - `lowerMatch` routes the three types to `lowerARPOperationMatch` (network header offset 6, 2 octets), `lowerARPSenderAddressMatch` (layout compare of offsets 0 to 5 against 0001 0800 06 04, then offset 14, 4 octets) and `lowerNDTargetAddressMatch` (nfproto guard in inet, l4proto ICMPv6, transport offset 8, 16 octets, no type check); `lowerAddrMatch` emits `nfprotoGuard` only in inet, so in arp it reads the ARP header at IPv4 offsets; `lowerHook` maps arp hooks through `lowerARPHook`
- [ ] `internal/plugins/firewall/nft/readback_linux.go` - raises tables, chains, sets and flowtables, handles `TableFamilyARP` hooks; rules are not raised
- [ ] `internal/plugins/firewall/nft/doctor_linux.go` - `firewall-nftables` doctor check warns when nf_tables is not loaded; no arp-family check
- [ ] `internal/component/kernelcap/kernelcap.go` - `Capability` fields Subsystem, Component, Kernel, ConfigLeaf, CodeAbsent, CodeUnknown, Order, InUse, Probe; `MustRegister` from an owner's init
- [ ] `internal/component/firewall/cmd/show.go` - `formatMatch` already prints "arp operation request|reply", "arp sender address A", "nd target address A"
- [ ] `internal/component/firewall/registry.go` - `RegisterTables` requires the `ze_` prefix; `ApplyAll` concatenates every owner's tables sorted by owner and `mergeSameNameTables` joins equal name and family by concatenating chains, sets and flowtables
- [ ] `internal/plugins/vrrp/ownerfilter.go` - tables `ze_vrrp_owner_arp` (family arp) and `ze_vrrp_owner_nd` (family ip6), each one base chain `output`, filter type, hook output, priority 0, policy accept; ARP term: output-interface parent, operation reply, sender VIP, drop; ND term: output-interface parent, icmpv6-type 136, target VIP, drop; owner name `vrrp-owner`
- [ ] `internal/test/fixture/netfilter_fixture.go` and `netfilter_fixture_firewall.go` - `registerTableSnapshot(name, family, table)` runs `nft list table <family> <table>` for a `.ci` to assert against
- [ ] `gokrazy/kernel/kernel.require` - lists `CONFIG_NF_TABLES_ARP`, so the appliance kernel carries it

**Behavior to preserve:**
- The VRRP owner filter's tables, terms and lowering: `TestVRRPOwnerAnswersWithVirtualMACOnly` (QEMU) stays green
- Existing refusals and their message text in `validateMatch` for the three types (the parse tests below assert them)
- `show firewall` formatting of the three matches
- Every existing `from` leaf keeps its behavior in inet, ip and ip6 tables
- The arp hook mapping to `NF_ARP_IN` and `NF_ARP_OUT`

**Behavior to change:**
- Three new `from` leaves produce the three existing match types
- In a table of family arp, every `from` leaf that reads an IP header, a transport header or conntrack is refused at verify (today most are accepted and misread or never match)
- An ND target match is never lowered without an ICMPv6 type restriction to 135 or 136 (D-2 picks where the restriction lives)
- A config with a table of family arp on a kernel without the nftables arp family is refused at start, reload and validate with a named diagnostic
- The operator/VRRP table name collision is closed (D-3)

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config text: `firewall { table <name> { family arp|ip6|inet; chain <c> { ... term <t> { from { arp-operation reply; arp-sender-address 192.0.2.1; } ... } } } }`, or `set` commands in the CLI editor
- YANG-validated tree, delivered to the firewall component as `map[string]any` with every leaf value a string

### Transformation Path
1. YANG parse and native validation: enum for `arp-operation`, `zt:ipv4-address` and `zt:ipv6-address` patterns for the two address leaves; `ze:backend "nft"` refuses the leaf on another backend
2. `parseFromBlock` (`internal/component/firewall/config.go`) turns each leaf into `MatchARPOperation`, `MatchARPSenderAddress` or `MatchNDTargetAddress`
3. `ValidateTables` and `validateMatch` (`validate.go`) check family admissibility for every match in the term, and the ND type restriction if D-2 picks option a
4. `RegisterTables` under the firewall engine owner, then `ApplyAll` merges with other owners (VRRP among them) and calls the nft backend
5. `lowerMatch` (`internal/plugins/firewall/nft/lower_linux.go`) emits the expressions already written for VRRP
6. Kernel: nftables table `ze_<name>` in family arp, ip6 or inet
7. `show firewall` reads the last applied model and prints through `formatMatch`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config file to firewall component | YANG tree to `map[string]any`, string leaf values | No |
| Firewall component to nft backend | `[]firewall.Table` through the table registry and `Backend.Apply` | No |
| nft backend to kernel | nftables netlink batch, family `NFPROTO_ARP`, `NFPROTO_IPV6` or `NFPROTO_INET` | No |
| Kernel capability to startup gate | `kernelcap.Evaluate` and `Refuse` from `cmd/ze/hub` and `ze config validate` | No |

### Integration Points
- `parseFromBlock` - three new key reads beside `icmpv6-type`
- `validateMatch` - existing cases stay; arp-family refusals are added for the IP, transport and conntrack match types
- `kernelcap.MustRegister` - new enrolment in the nft backend package
- `mergeSameNameTables` or a verify-time name check - D-3

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes (planned) | Leaves enter through YANG and `parseFromBlock` like every other `from` leaf |
| No unintended coupling (components stay isolated) | Yes (planned) | The firewall component learns nothing about VRRP; D-3's fix is expressed in registry terms (owner, name), not VRRP terms |
| No duplicated functionality (extends existing, does not recreate) | Yes (planned) | Reuses the three match types, their validation and their lowering |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Config-time path, no wire encoding |
| Registration over hardcoding, outbound | Yes (planned) | The kernel need registers through `kernelcap.MustRegister` from the nft package; D-3 recommended option registers each producer's table names from its own package |
| Registration over hardcoding, inbound | Partly (planned) | `parseFromBlock` and `validateMatch` are per-match-type switches today and every existing leaf is added there; `TestParsedNamesMatchTheModel` gates the name tables. The ARP opcode names get one value-to-name map in `model.go` read through `nameIndex`, the way `familyNames` is |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `meta iifname`, `meta oifname` and `meta mark` evaluate normally in an nftables arp chain | `ownerARPReplyTerm` uses `MatchOutputInterface` in an arp table and the QEMU test passes | input-interface, output-interface or mark must also be refused in arp | Integration test AC-12 with `output-interface` beside `arp-operation` | unvalidated |
| A-2 | In an arp chain, a transport-header payload read and `meta l4proto` never match (no L4 info), and `ct state` reads the packet as untracked or invalid rather than breaking | Reading of kernel `nft_payload_eval` and `nft_ct_get_eval` from memory | The refusal list in AC-9 is still right (refusing is safe either way); only its wording changes | Unit refusal tests do not depend on it; recorded so the message does not claim a kernel behavior | unvalidated |
| A-3 | A netlink probe can tell a kernel without `CONFIG_NF_TABLES_ARP` from one with it, without executing a binary and without committing state, by sending an nftables batch that declares an arp table and a filter base chain and omits the batch end, so the kernel validates then aborts | nft's own check mode works this way; `github.com/google/nftables` exposes a raw batch path (unverified) | The probe falls back to `/proc/config.gz` where present and reports Unknown otherwise, which warns and never refuses | `TestARPFamilyProbeClassifiesEachErrno` plus the QEMU run of AC-11 on a kernel built without the option | unvalidated |
| A-4 | Without CAP_NET_ADMIN the probe gets EPERM and must answer Unknown | nfnetlink requires CAP_NET_ADMIN for every nftables message | A non-root `ze config validate` would refuse wrongly | `TestARPFamilyProbeEPERMIsUnknown` | unvalidated |
| A-5 | nft 1.0.x renders the lowered ND target compare as `icmpv6 taddr <addr>` and the ARP sender compare as `arp saddr ip <addr>` in `nft list table` | nftables manual lists both fields | The `.ci` assertions use a pattern that also accepts the raw `@nh,112,32` and `@th,64,128` forms | First run of the functional tests | unvalidated |
| A-6 | The functional test runner's kernel carries `CONFIG_NF_TABLES_ARP` | The VRRP QEMU runtime kernel does; the Docker runner kernel is the host's | The arp functional test needs the QEMU runner instead of `needs-linux` | First run of `test/firewall/firewall-arp-match.ci` | unvalidated |
| A-7 | Two base chains of the same name inside one merged table either fail the whole reconcile or lose one owner's chain | `mergeSameNameTables` concatenates chains without a name check | If the kernel silently accepts both, D-3 is still owed because the table is no longer one owner's | `TestApplyAllRefusesAChainTwoOwnersDeclare` (D-3 option a) | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An operator ND target leaf without an ND type reads the wrong 16 octets of every ICMPv6 message and matches nonsense | A term with only `nd-target-address` validates | D-2: the restriction is either demanded by verify or built into the lowering; a unit test covers the term without `icmpv6-type` |
| R-2 | Tightening arp-family validation refuses a config that parses today | Parse tests of existing configs, `ze config validate` over `test/` fixtures | Every refused leaf misreads or never matches in arp today, so no working config is lost; the error names the family and the leaf |
| R-3 | An operator drop rule in an arp or ip6 output chain drops the VRRP macvlan's own ARP replies or advertisements and the virtual address goes dark | VRRP functional tests with an operator table beside them | nftables runs every base chain at a hook; a drop anywhere is final, an accept is not, so an operator accept cannot undo the VRRP owner drop, but an operator drop can remove what VRRP needs. The guide says so beside the example |
| R-4 | Operator table name equal to a VRRP table name merges into it | Journal row `two-owners-share-one-name.md` | D-3 |
| R-5 | The kernelcap probe refuses a working router | Absent reported on a kernel that has the family | Unknown on every error the probe cannot classify; only the specific "family not supported" errno answers Absent |
| R-6 | Changing `lowerNDTargetAddressMatch` (D-2 option b) changes the VRRP owner's kernel rule | QEMU test red | The added type compare admits 136, the only type the VRRP term names; `TestVRRPOwnerAnswersWithVirtualMACOnly` reruns |
| R-7 | The vpp backend accepts the leaf and then refuses the match with a generic error | Parse test on `backend vpp` | `ze:backend "nft"` on each leaf refuses at commit with the backend named |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An operator arp or ip6 filter matches the wrong packets, or the VRRP owner stops suppressing physical-MAC answers; a wrong kernelcap probe stops the daemon starting |
| How is it reverted? | Single commit revert; the new leaves are additive, the arp refusals only refuse configs that misread |
| Who else touches this path? | `plan/immediate/spec-vrrp-owner-arp-rfc-defects.md` (vpp verifier D2, owner ARP request D1), `plan/spec-dataplane-seams-5-copp-non-tcp.md` (ARP policing needs a non-inet family) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config `from { arp-operation reply; arp-sender-address 192.0.2.1; }` in a family arp table | → | `parseFromBlock` to `lowerARPOperationMatch`, `lowerARPSenderAddressMatch` | `test/firewall/firewall-arp-match.ci` |
| Config `from { icmpv6-type nd-neighbor-advert; nd-target-address 2001:db8::1; }` in a family ip6 table | → | `parseFromBlock` to `lowerNDTargetAddressMatch` | `test/firewall/firewall-nd-target-match.ci` |
| `ze config validate` on an arp-family misuse | → | `validateMatch` | `test/parse/firewall-arp-source-address-rejected.ci` |
| `ze doctor` and daemon start with a family arp table | → | the nft package's kernelcap enrolment | `TestFirewallARPFamilyCapabilityInUse` and `test/parse/firewall-arp-kernelcap-absent.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Leaf `arp-operation` with value `request` or `reply` in a family arp table | Config validates; the term holds `MatchARPOperation` with opcode 1 or 2; the kernel rule compares network-header offset 6 with 0001 or 0002 |
| AC-2 | `arp-operation` with any other value | Refused by YANG enum validation, with the valid values offered |
| AC-3 | Leaf `arp-sender-address 192.0.2.1` in a family arp table | Config validates; the kernel rule carries the Ethernet/IPv4 layout compare and the 4-octet compare at offset 14 |
| AC-4 | `arp-sender-address` given `0.0.0.0`, an IPv6 address, or a prefix | Refused: YANG refuses the non-IPv4 forms; `0.0.0.0` is refused by `validateMatch` with "arp-sender-address names no IPv4 address" |
| AC-5 | `arp-operation` or `arp-sender-address` in a table of family inet, ip, ip6, bridge or netdev | Refused at verify with "valid only in family arp, got <family>" |
| AC-6 | Leaf `nd-target-address 2001:db8::1` beside `icmpv6-type nd-neighbor-solicit` or `nd-neighbor-advert` in a family ip6 or inet table | Config validates; the kernel rule restricts l4proto to ICMPv6, the ICMPv6 type to the one named, and compares transport offset 8 with the 16 octets; in inet it also carries the nfproto IPv6 guard |
| AC-7 | `nd-target-address` in a family ip, arp, bridge or netdev table; or given `::`, an IPv4 address, or an IPv4-mapped address | Refused with the existing messages ("valid only in family ip6 or inet", "names no IPv6 address") |
| AC-8 | `nd-target-address` with no `icmpv6-type`, or with an `icmpv6-type` other than 135 or 136 | D-2 option a: refused at verify with a message naming nd-neighbor-solicit and nd-neighbor-advert. D-2 option b: accepted with no type, and the lowered rule restricts the type to 135 or 136 by itself; a type other than 135 or 136 beside it is refused because the term can never match |
| AC-9 | In a table of family arp, any of source-address, destination-address, source-port, destination-port, protocol, connection-state, connection-mark, tcp-flags, a set match, an IRR or domain-group match | Refused at verify naming the leaf and "family arp carries no IP header"; icmp-type, icmpv6-type and dscp keep their existing refusals |
| AC-10 | In a table of family arp, input-interface, output-interface and mark beside an ARP leaf | Accepted and matched (A-1) |
| AC-11 | Config holding a table of family arp, host kernel without the nftables arp family | `ze config validate` reports `config-kernel-capability` at error severity; the daemon refuses to start with one line naming CONFIG_NF_TABLES_ARP and the table; `ze doctor` prints the absent code; a probe that cannot decide warns with the unknown code and starts |
| AC-12 | Traffic: an ARP reply with sender 192.0.2.1 and one with sender 192.0.2.2 cross a family arp input chain whose term matches `arp-operation reply` and `arp-sender-address 192.0.2.1` with a counter | The term's counter counts the first packet only |
| AC-13 | `show firewall` with an operator table holding the three leaves | Prints "arp operation reply", "arp sender address 192.0.2.1", "nd target address 2001:db8::1" under the term |
| AC-14 | Any of the three leaves under `backend vpp` | Refused at commit by `ze:backend`, naming the nft backend |
| AC-15 | An operator table whose kernel name equals a VRRP owner table (`vrrp_owner_arp` family arp, `vrrp_owner_nd` family ip6, `vrrp` family inet) while VRRP registers that table | Per D-3: the operator table is refused at verify naming the owner that holds the name, and the VRRP owner table in the kernel keeps its terms |
| AC-16 | An operator arp table with an accept term for ARP replies from a VRRP-owned address, beside the VRRP owner table | The VRRP owner drop still removes the parent's physical-MAC reply (drop in any base chain is final) |
| AC-17 | Every new leaf and the `arp-operation` enum values | Carry `ze:help` and `description` that `./le doc yang-contract help-shape` accepts |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator drops ARP replies claiming the gateway address arriving on a customer port | config, `parseFromBlock`, `validateMatch`, registry, `lowerMatch`, kernel arp input chain | `test/firewall/firewall-arp-match.ci`, `TestNftIntegrationARPSenderMatchCountsOnlyThatSender` |
| 2 | Operator drops Neighbor Advertisements that claim a protected IPv6 address | config, parse, validate, lowering, kernel ip6 input chain | `test/firewall/firewall-nd-target-match.ci` |
| 3 | Operator writes `source-address` in an arp table by mistake | config, `validateMatch` refusal | `test/parse/firewall-arp-source-address-rejected.ci` |
| 4 | Operator deploys an arp table on a kernel built without the arp nftables family | kernelcap Evaluate at validate and start | `test/parse/firewall-arp-kernelcap-absent.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseFromBlockARPOperation` | `internal/component/firewall/config_test.go` | request and reply map to opcodes 1 and 2; an unknown name errors (AC-1, AC-2) | |
| `TestParseFromBlockARPSenderAddress` | `internal/component/firewall/config_test.go` | IPv4 address to `MatchARPSenderAddress` (AC-3) | |
| `TestParseFromBlockNDTargetAddress` | `internal/component/firewall/config_test.go` | IPv6 address to `MatchNDTargetAddress` (AC-6) | |
| `TestParsedNamesMatchTheModel` (extended) | `internal/component/firewall/model_enum_test.go` | the ARP opcode name map covers every valid `ARPOperation` and the YANG enum | |
| `TestValidateARPFamilyRefusesIPHeaderMatches` | `internal/component/firewall/validate_test.go` | one sub-test per leaf in AC-9, each refused with the leaf named | |
| `TestValidateARPFamilyAdmitsMetaMatches` | `internal/component/firewall/validate_test.go` | AC-10 | |
| `TestValidateNDTargetTypeRestriction` | `internal/component/firewall/validate_test.go` | AC-8 for the D-2 option chosen | |
| `TestLowerNDTargetAddressRestrictsType` (D-2 option b only) | `internal/plugins/firewall/nft/lower_linux_test.go` | the expression list carries the ICMPv6 type compare | |
| `TestFirewallARPFamilyCapabilityInUse` | `internal/plugins/firewall/nft/kernelcap_linux_test.go` | InUse true only for a tree with a family arp table under backend nft (AC-11) | |
| `TestARPFamilyProbeClassifiesEachErrno`, `TestARPFamilyProbeEPERMIsUnknown` | `internal/plugins/firewall/nft/kernelcap_linux_test.go` | Present, Absent and Unknown classification (A-3, A-4) | |
| `TestApplyAllRefusesAChainTwoOwnersDeclare` or `TestVerifyRefusesAReservedTableName` | `internal/component/firewall/registry_test.go` or `validate_test.go` | AC-15 per D-3 | |
| `TestFormatMatchARPND` | `internal/component/firewall/cmd/show_test.go` | AC-13 strings | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `arp-operation` opcode | 1-2 (enum request, reply) | 2 | 0 (Unspecified, refused by `validateMatch`) | 3 (not an enum value) |
| ICMPv6 type beside `nd-target-address` | 135-136 | 136 | 134 | 137 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `firewall-arp-match` | `test/firewall/firewall-arp-match.ci`, snapshot registered as `registerTableSnapshot("firewall/firewall-arp-match", "arp", "ze_fwarp")` | nft list shows `arp operation reply` and `arp saddr ip 192.0.2.1` (or the raw payload form, A-5) in chain input | |
| `firewall-nd-target-match` | `test/firewall/firewall-nd-target-match.ci`, snapshot family ip6 | nft list shows `icmpv6 type nd-neighbor-advert` and `icmpv6 taddr 2001:db8::1` | |
| `firewall-arp-matches-accepted` | `test/parse/firewall-arp-matches-accepted.ci` | `ze config validate` exit 0 for all three leaves in their families | |
| `firewall-arp-in-inet-rejected` | `test/parse/firewall-arp-in-inet-rejected.ci` | AC-5 message | |
| `firewall-nd-target-in-ip-rejected` | `test/parse/firewall-nd-target-in-ip-rejected.ci` | AC-7 message | |
| `firewall-nd-target-type-rejected` | `test/parse/firewall-nd-target-type-rejected.ci` | AC-8 refusal for the chosen D-2 option | |
| `firewall-arp-source-address-rejected` | `test/parse/firewall-arp-source-address-rejected.ci` | AC-9 message | |
| `firewall-arp-leaf-vpp-rejected` | `test/parse/firewall-arp-leaf-vpp-rejected.ci` | AC-14 | |
| `firewall-arp-kernelcap-absent` | `test/parse/firewall-arp-kernelcap-absent.ci` | AC-11 with the probe forced Absent through the kernelcap test override | |
| `firewall-cli-show` (extended) | `test/firewall/firewall-cli-show.ci` | AC-13 | |
| `TestNftIntegrationARPSenderMatchCountsOnlyThatSender` | `internal/plugins/firewall/nft/counter_integration_linux_test.go` (integration, linux) | AC-12 with real ARP frames in `withNftNetNS` | |
| `TestVRRPOwnerSurvivesOperatorARPTable` | QEMU suite beside `TestVRRPOwnerAnswersWithVirtualMACOnly` | AC-16: operator accept table present, physical-MAC reply still dropped | |

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | A local packet filter with no protocol peer; the Linux kernel is the only other party, and AC-12 and the QEMU test exercise it with real frames | |

## Files to Modify
- `internal/component/firewall/yang/ze-firewall-conf.yang` - three leaves in `from-block`; `family` leaf description names what arp admits
- `internal/component/firewall/config.go` - `parseFromBlock` reads the three leaves; ARP opcode parse through a name index
- `internal/component/firewall/model.go` - ARP opcode value-to-name map; remove "Daemon-only" sentences from the three match comments; ND target comment follows D-2
- `internal/component/firewall/validate.go` - arp-family refusals (AC-9), ND type rule (D-2 option a), comment on `validateSetFamilyCompat` no longer calls the literal gap untracked
- `internal/plugins/firewall/nft/lower_linux.go` - D-2 option b only: ICMPv6 type compare inside `lowerNDTargetAddressMatch`, comment updated
- `internal/component/firewall/registry.go` - D-3
- `internal/plugins/vrrp/ownerfilter.go` - D-3 recommended option registers its table names; comment on the ND term if D-2 option b makes its `MatchICMPv6Type` redundant (kept, because it narrows to 136)
- `internal/core/diagnostic/codes.go` - absent and unknown codes for the arp family capability
- `internal/component/firewall/cmd/show.go` - no change expected; verify AC-13
- `docs/guide/firewall.md`, wiki `firewall.md` (`~/Code/github.com/ze-software/ze/wiki/firewall.md`), `docs/architecture/firewall/table-ownership-and-shutdown-flush.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/guide/vrrp.md` (operator table interaction), `docs/features.md`
- `internal/test/fixture/netfilter_fixture.go` - two table snapshot registrations

## Files to Create
- `internal/plugins/firewall/nft/kernelcap_linux.go` - `kernelcap.MustRegister` for CONFIG_NF_TABLES_ARP, InUse and Probe
- `internal/plugins/firewall/nft/kernelcap_linux_test.go`
- `test/firewall/firewall-arp-match.ci`, `test/firewall/firewall-nd-target-match.ci`
- `test/parse/firewall-arp-matches-accepted.ci`, `firewall-arp-in-inet-rejected.ci`, `firewall-nd-target-in-ip-rejected.ci`, `firewall-nd-target-type-rejected.ci`, `firewall-arp-source-address-rejected.ci`, `firewall-arp-leaf-vpp-rejected.ci`, `firewall-arp-kernelcap-absent.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/firewall/yang/ze-firewall-conf.yang`, grouping `from-block` |
| YANG validation constraints | Yes | enum for `arp-operation`; `zt:ipv4-address` and `zt:ipv6-address` for the address leaves |
| YANG custom validators | No | Family admissibility needs the table's family, which `validateMatch` already has; no `ze:validate` hook |
| CLI commands/flags | No | No command added; `show firewall` already formats the matches |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | Yes | Automatic from the enum; `ze:backend "nft"` filters the leaves off under vpp |
| Functional test for new RPC/API | Yes | `test/firewall/*.ci` and `test/parse/*.ci` listed above |
| Pipe completeness | N-A | No command output added |
| Env var registration | N-A | Per-term match data, not a tunable |
| Doctor check for runtime dependencies | Yes | CONFIG_NF_TABLES_ARP via `kernelcap.MustRegister` in `internal/plugins/firewall/nft/kernelcap_linux.go`, codes in `internal/core/diagnostic/codes.go` |
| Prometheus counters/metrics | No | Per-rule counters already exist through the `counter` action |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` firewall row names ARP and ND matches |
| 2 | Config syntax changed? | Yes | `docs/guide/firewall.md` match table and families; `docs/guide/configuration.md` only if it lists `from` leaves (check) |
| 3 | CLI command added/changed? | No | `show firewall` output unchanged in shape |
| 4 | API/RPC added/changed? | No | No RPC |
| 5 | Plugin added/changed? | No | The nft backend gains an enrolment, not a plugin surface |
| 6 | Has a user guide page? | Yes | `docs/guide/firewall.md`, wiki `firewall.md`, `docs/guide/vrrp.md` (operator table beside the owner tables) |
| 7 | Wire format changed? | No | No Ze wire format |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | No | Filtering ARP and ND is operator policy, not an RFC obligation; the VRRP rows are unchanged |
| 10 | Test infrastructure changed? | No | Two snapshot registrations use the existing helper |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if it has a firewall ARP row (check) |
| 12 | Internal architecture changed? | Yes | `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` (VRRP producers, D-3), `docs/architecture/doctor-and-health-checks.md` (third enrolment) |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | kernel capability inventory in `docs/architecture/doctor-and-health-checks.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Declared by `// Design:` headers of files this spec changes: `docs/architecture/core-design.md` (declared by `internal/component/firewall/config.go`, `model.go` and the nft package) is unaffected, because it describes registration and composition, which this spec does not alter; `docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md` (declared by `internal/plugins/vrrp/ownerfilter.go`) is updated in phase 5 with the operator-table interaction and the D-3 outcome; `docs/features/ai-first.md` (declared by `internal/core/diagnostic/codes.go`) is unaffected, because it describes `ze explain` generically and lists no individual code. Advisory mentions: `docs/architecture/firewall/firewall-irr.md` (mentions `registry.go`, `config.go`, `model.go`, `validate.go`) is re-read in phase 5, because the IRR sets rely on the same-name merge D-3 touches; `docs/architecture/ddos/cp-survival-5-detect-5-characterization.md`, `docs/architecture/traffic/cp-survival-2-copp-port179.md`, `docs/architecture/bgp/as112-coordination.md`, `docs/guide/redistribution.md` and `docs/guide/vpp.md` mention the changed files for matches, codes and parsing this spec does not alter. Re-run `./le spec citation anchors spec plan/spec-firewall-arp-nd-matches.md` at implementation and name each doc it lists |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | guide and wiki firewall examples; verify against YANG after the change |

## Implementation Steps

1. **Phase: Wiring** - YANG leaves, `parseFromBlock` reads, fixture registrations, the two `test/firewall` `.ci` files and the accepted parse test; they fail until the leaves parse
2. **Phase: Validation** - arp-family refusals (AC-9, AC-10), ND type rule per D-2, `ze:backend` (AC-14), parse `.ci` refusals; `TestValidate*` red then green
3. **Phase: Lowering** - D-2 option b only; otherwise confirm the existing lowering satisfies AC-1, AC-3, AC-6 through the functional tests
4. **Phase: Kernel capability** - enrolment, probe, codes, doctor, AC-11 tests
5. **Phase: VRRP interaction** - D-3 fix, AC-15 and AC-16 tests, `docs/architecture/firewall/table-ownership-and-shutdown-flush.md`
6. **Phase: Docs** - each page is edited in the phase whose change makes it wrong; this phase verifies examples against YANG (row 17)

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | No `from` leaf reaches the nft lowering in an arp table unless it reads the ARP header or packet meta |
| Correctness | An ND target compare never reaches the kernel without a 135 or 136 restriction in the same rule |
| Naming | Leaf names match the words `show firewall` prints and the messages `validateMatch` already emits |
| Data flow | The firewall component names no VRRP symbol |
| Rule: principles, silent wrong value | Every misread path is refused, not narrowed silently |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Three YANG leaves with help text | `./le doc yang-contract help-shape` |
| Arp-family refusals | `test/parse/firewall-arp-source-address-rejected.ci` |
| Kernel enrolment | `ze doctor` output in `TestFirewallARPFamilyCapabilityInUse` |
| Real traffic match | `TestNftIntegrationARPSenderMatchCountsOnlyThatSender` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Address leaves refuse unspecified and wrong-family addresses; opcode limited to the enum |
| Fail-open | An operator table cannot remove a VRRP drop (AC-16) or merge into a VRRP table (AC-15) |
| Resource exhaustion | None new: one match per leaf per term |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood, back to RESEARCH |
| Lint failure | Fix inline. If architectural, back to DESIGN |
| Functional test fails | Check the AC: wrong AC to DESIGN, correct AC to IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- nftables runs every base chain registered at a hook in priority order; accept ends only that chain, drop is final. This is why an operator table can never cancel the VRRP owner's drop but can drop traffic VRRP needs.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Leaf names `arp-operation`, `arp-sender-address`, `nd-target-address` | nft spellings `arp-saddr-ip`, `icmpv6-taddr`; VyOS or Junos spellings | No vendor has an ARP field match to copy; Ze's `from` leaves spell words in full (`source-address`), and these names are the words `show firewall` and `validateMatch` already use |
| Reuse `family arp`, no new family | A dedicated `arp { }` container | The family exists in YANG, model and lowering; a second surface would declare the same fact twice |
| Bare address, not a prefix, for both address leaves | Prefix like `source-address` | The model holds one address and the VRRP use needs one; a prefix is new model work (D-1) |
| D-1 (open) | See the table below | Owner decides |
| D-2 (open) | See the table below | Owner decides |
| D-3 (open) | See the table below | Owner decides |

### Open Decisions for the Owner

| ID | Question | Options | Recommendation |
|----|----------|---------|----------------|
| D-1 | Which ARP fields to expose | a: only the three existing matches. b: also `arp-target-address` (nft `arp daddr ip`) and sender and target hardware address, which need new model types, validation and lowering | a, and b as its own spec if wanted |
| D-2 | Where the ND type restriction lives | a: verify refuses `nd-target-address` unless the same term names `icmpv6-type` 135 or 136. b: `lowerNDTargetAddressMatch` adds the type compare itself, so the match can never read another type's bytes | b: the match carries its own guard the way `arp-sender-address` carries its layout guard, and no producer can forget it |
| D-3 | Operator table name collides with a VRRP table | a: fix here: each producer registers its table names from its own package, and verify refuses an operator table whose kernel name another owner holds. b: journal row only (written), fix in its own spec | a, because the brief requires that an operator rule cannot break the VRRP owner tables |

## Known Limitations

- VRRP's own need for the arp family (the owner ARP table) is not enrolled with kernelcap by this spec: its InUse depends on runtime addresses, not config. The appliance kernel carries the option (`gokrazy/kernel/kernel.require`).
- The vpp backend gains no ARP or ND lowering; `ze:backend "nft"` refuses the leaves there, and the daemon-side refusal stays with `plan/immediate/spec-vrrp-owner-arp-rfc-defects.md` D2.

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
- [ ] AC-1..AC-17 all demonstrated
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
- [ ] Interop tests for protocol features (N-A, reason in the Interop table)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/spec-firewall-arp-nd-matches.md` only, in the same `./le commit create` script
