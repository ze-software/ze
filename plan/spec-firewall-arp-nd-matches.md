# Spec: firewall-arp-nd-matches

| Field | Value |
|-------|-------|
| Status | ready |
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

Goal (D-1, decided 2026-09-28): an operator can match every ARP and ND field the
nftables ARP header expression and the ND target read, in a
`firewall { table ... { chain ... { term ... { from { } } } } }` block:

| Leaf | Field | Model type | Exists today |
|------|-------|------------|--------------|
| `arp-operation` | ARP opcode (ar$op), nft `arp operation` | `MatchARPOperation` | yes |
| `arp-sender-address` | ARP sender protocol address (ar$spa), nft `arp saddr ip` | `MatchARPSenderAddress` | yes |
| `arp-target-address` | ARP target protocol address (ar$tpa), nft `arp daddr ip` | `MatchARPTargetAddress` | no, new |
| `arp-sender-hardware-address` | ARP sender hardware address (ar$sha), nft `arp saddr ether` | `MatchARPSenderHardwareAddress` | no, new |
| `arp-target-hardware-address` | ARP target hardware address (ar$tha), nft `arp daddr ether` | `MatchARPTargetHardwareAddress` | no, new |
| `nd-target-address` | ICMPv6 ND Target Address, nft `icmpv6 taddr` | `MatchNDTargetAddress` | yes |

Ze refuses each leaf in a family where it would read the wrong bytes, lowers
each to nftables (the three new ones through new lowering built the way the
existing three are), reports a kernel without the arp nftables family before it
starts, prints every match in `show firewall` and on the web firewall page, and
never lets an operator table break a table another producer owns.

Two defects found while researching this ride along because the goal depends on
them (the rows are in `plan/journal/`):

| Defect | Journal row | Why the goal depends on it |
|--------|-------------|----------------------------|
| A literal `source-address` or `destination-address` in a table of family arp is accepted and reads ARP header bytes at IPv4 offsets | `plan/journal/guard-added-to-one-half-of-a-pair.md`, 2026-09-28 | Exposing ARP matches invites operators to write arp tables; every other `from` leaf they reach for there must refuse rather than misread |
| An operator table whose `ze_` name equals a VRRP table name is merged into it by `mergeSameNameTables` | `plan/journal/two-owners-share-one-name.md`, 2026-09-28 | The brief requires that an operator rule cannot break the VRRP owner tables. D-3 fixes it here: each producer reserves the table names it owns and verify refuses an operator table that takes one |

## Required Reading

### Architecture Docs
- [ ] `docs/guide/firewall.md` - operator page: families, `from` leaf table, "Tables another feature installs"
  → Constraint: the match table (section "Match Types (from block)") lists config key, meaning and one example per leaf; the six leaves get rows there, and "Table Families" gains which leaves each family admits
  → Decision: "Tables another feature installs" is where the reserved table names, the VRRP owner tables and the interaction with an operator arp table are explained
- [ ] `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` - producer list and Rule 2 (`ze_` prefix)
  → Constraint: the producer table omits `ze_vrrp`, `ze_vrrp_owner_arp` and `ze_vrrp_owner_nd`; the page is wrong today and is repaired in the phase that adds the name reservation (phase 5), and it names the reservation registry as the source of the list
- [ ] `docs/architecture/doctor-and-health-checks.md` - "Kernel capabilities: one enrolment, three callers"
  → Constraint: a kernel feature need is enrolled with `kernelcap.MustRegister` from the owning package; `Probe` reads netlink or procfs and MUST NOT execute a binary; Absent refuses start, reload and `ze config validate`, Unknown only warns
  → Decision: the arp-family need is enrolled by the nft backend package (`internal/plugins/firewall/nft/`), because that package is the one that programs `NFPROTO_ARP`
- [ ] `docs/architecture/config/yang-config-design.md` - `ze:backend` extension row
  → Constraint: `ze:backend "nft"` restricts a node to the nft backend, commit validates it and completion filters on it; the vpp backend has no ARP or ND lowering (`plan/immediate/spec-vrrp-owner-arp-rfc-defects.md` D2), so each of the six leaves carries `ze:backend "nft"`
- [ ] `ai/patterns/config-option.md` - YANG leaf definition and naming across layers
  → Constraint: leaf names are spelled in full; no env var applies (a match is per-term data, not a tunable)
- [ ] `ai/rules/config.md` - help text contract
  → Constraint: each leaf and each enum value carries `ze:help` (at most 96 characters and 25 words) and each leaf a `description` that differs from it; `./le doc yang-contract help-shape` refuses otherwise

### External References (naming)
- [ ] nftables manual, ARP HEADER EXPRESSION and ICMPV6 HEADER EXPRESSION (netfilter.org manpage, read 2026-09-28)
  → Decision: nftables spells the fields `arp htype`, `arp ptype`, `arp hlen`, `arp plen`, `arp operation` (request, reply), `arp saddr ether`, `arp saddr ip`, `arp daddr ether`, `arp daddr ip`, and `icmpv6 taddr` for the ND target; the arp family has only the input and output hooks
  → Decision: htype, ptype, hlen and plen are not exposed as leaves. Ze reads them only as the layout guard every address match carries, and a term that wants "only Ethernet/IPv4 ARP" gets it from any address leaf
  → Constraint: whether nft adds its own layout dependency to `arp saddr ip` is not known and not relied on; Ze's guard is Ze's own compare
- [ ] VyOS firewall documentation, bridge section (read 2026-09-28)
  → Constraint: VyOS offers only `ethernet-type arp` and no ARP field match, so there is no VyOS spelling to follow
- [ ] Junos firewall filters (from memory, unverified): `family inet6` filters match `icmp-type neighbor-solicit` but no ND target; ARP is policed, not field-matched
  → Decision: no vendor spelling exists to copy, so the names follow Ze's own `from` leaves: an address leaf is `<who>-address` (`source-address`, `destination-address`), and a MAC leaf spells `hardware-address` in full, prefixed `arp-` so the family it needs is in the name. `sender` and `target` are RFC 826's words and the ones `show firewall` already prints for the sender match
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` leaf `address` under an interface
  → Constraint: the existing MAC leaf shape is `type string` with pattern `[0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){5}` plus `ze:validate "mac-address"` (validator registered in `internal/component/config/validators_register.go`); the two hardware-address leaves reuse that shape

### RFC Text (packet layouts only; filtering is operator policy, so no `rfc/short/` summary is owed and none exists for either RFC)
- [ ] RFC 826 layout as drawn above `lowerARPOperationMatch` (`internal/plugins/firewall/nft/lower_linux.go`); `rfc/full/` holds no copy, fetch `rfc826.txt` before quoting it
  → Constraint: ar$op precedes the variable-length addresses; ar$sha sits at offset 8, ar$spa at 14, ar$tha at 18 and ar$tpa at 24 only for the Ethernet/IPv4 layout, which `lowerARPSenderAddressMatch` already guards with one compare of offsets 0 to 5
- [ ] `rfc/full/rfc4861.txt` - ND message formats, Sections 4.3 and 4.4
  → Constraint: the Target Address sits at ICMPv6 octet 8 in both Neighbor Solicitation (135) and Neighbor Advertisement (136) and nowhere else, so any other ICMPv6 type puts different bytes at that offset

**Key insights:**
- Model, validation, nft lowering and `show` formatting of three matches already exist and are exercised by VRRP. This spec adds the config surface for those three, adds three new match types end to end (model, validation, lowering, show, web), closes the family gaps that exposure opens, enrolls the kernel need, and reserves producer table names.
- `family arp` is already a YANG enum value and a model family (`FamilyARP`), with hooks mapped to `NF_ARP_IN` and `NF_ARP_OUT` by `lowerARPHook`. No new family is needed.
- `MatchNDTargetAddress` does not restrict the ICMPv6 type today; D-2 moves the 135-or-136 restriction into its lowering.
- `mergeSameNameTables` is load-bearing: the IRR and domain plugins register sets into `ze_<operator table>` names and rely on the merge. The reservation (D-3) covers only names a producer owns outright, so the merge stays for contributions.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/model.go` - `TableFamily` with `FamilyARP` named "arp" in `familyNames`; `ARPOperation` (Unspecified 0, Request 1, Reply 2); the three match structs, each documented daemon-only; `matchMarker` list names every Match type
- [ ] `internal/component/firewall/config.go` - `parseFromBlock` maps each `from` leaf to one Match by string key; `parseICMPv6Type` and `icmp6Types` name 135 `nd-neighbor-solicit` and 136 `nd-neighbor-advert`; `tableNamePrefix` "ze_" is prepended to the operator table name
- [ ] `internal/component/firewall/validate.go` - `ValidateTables`, `validateTerm`, `validateMatch`: `MatchARPOperation` and `MatchARPSenderAddress` refused outside `FamilyARP`, opcode other than request or reply refused, sender must be a specified IPv4 address; `MatchNDTargetAddress` refused outside ip6 and inet and must be a specified, non-mapped IPv6 address; `validateSetFamilyCompat` refuses a set address match in arp, bridge and netdev while literal address matches have no family case
- [ ] `internal/component/firewall/yang/ze-firewall-conf.yang` - typedef `table-family` has enum `arp` (help "ARP"); grouping `from-block` has source-address, destination-address, source-port, destination-port, protocol, input-interface, output-interface, connection-state, connection-mark, mark, dscp, icmp-type, icmpv6-type; the `family` leaf description says arp, bridge and netdev refuse dscp, dscp-set and tcp-mss-set
- [ ] `internal/plugins/firewall/nft/lower_linux.go` - `lowerMatch` routes the three types to `lowerARPOperationMatch` (network header offset 6, 2 octets), `lowerARPSenderAddressMatch` (layout compare of offsets 0 to 5 against `arpEthernetIPv4` 0001 0800 06 04, then offset 14, 4 octets) and `lowerNDTargetAddressMatch` (nfproto guard in inet, l4proto ICMPv6, transport offset 8, 16 octets, no type check); `lowerAddrMatch` emits `nfprotoGuard` only in inet, so in arp it reads the ARP header at IPv4 offsets; `lowerHook` maps arp hooks through `lowerARPHook`
- [ ] `internal/plugins/firewall/nft/readback_linux.go` - raises tables, chains, sets and flowtables, handles `TableFamilyARP` hooks; rules are not raised
- [ ] `internal/plugins/firewall/nft/doctor_linux.go` - `firewall-nftables` doctor check warns when nf_tables is not loaded; no arp-family check
- [ ] `internal/component/kernelcap/kernelcap.go` - `Capability` fields Subsystem, Component, Kernel, ConfigLeaf, CodeAbsent, CodeUnknown, Order, InUse, Probe; `MustRegister` from an owner's init
- [ ] `internal/component/firewall/cmd/show.go` - `formatMatch` prints "arp operation request|reply", "arp sender address A", "nd target address A"; `matchTypeName` names each Match type
- [ ] `internal/component/web/page_firewall.go` - `matchSummary` has no case for any ARP or ND match and prints the Go type name through its default arm
- [ ] `internal/plugins/firewall/vpp/verify.go` - the vpp verifier's NAT match switches refuse unknown matches through their default arms; filter chains are covered by `plan/immediate/spec-vrrp-owner-arp-rfc-defects.md` D2
- [ ] `internal/component/firewall/registry.go` - `RegisterTables` requires the `ze_` prefix; `ApplyAll` concatenates every owner's tables sorted by owner and `mergeSameNameTables` joins equal name and family by concatenating chains, sets and flowtables
- [ ] Producers calling `RegisterTables` (grep 2026-09-28) and the table names each declares as a constant: vrrp owner filter `ze_vrrp_owner_arp`, `ze_vrrp_owner_nd` (`internal/plugins/vrrp/ownerfilter.go`); vrrp accept filter `ze_vrrp` (`internal/plugins/vrrp/acceptfilter.go`); flowspec `ze_flowspec` (`internal/plugins/flowspec-firewall/state.go`); copp `ze_copp` (`internal/plugins/copp/translate.go`); policy-routes `ze_pr` (`internal/plugins/policyroute/translate.go`); ddos-local `ze_ddos-local` (`internal/plugins/ddos/local/responder.go`); gtsm `ze_gtsm` (`internal/component/gtsm/gtsm.go`); firewall-irr `ze_irr_iface` (`internal/component/firewall/plugins/irr/sets.go`) plus sets into operator tables; domain plugin sets into operator tables only (`internal/component/firewall/plugins/domain/domain.go`)
- [ ] `internal/plugins/vrrp/ownerfilter.go` - tables `ze_vrrp_owner_arp` (family arp) and `ze_vrrp_owner_nd` (family ip6), each one base chain `output`, filter type, hook output, priority 0, policy accept; ARP term: output-interface parent, operation reply, sender VIP, drop; ND term: output-interface parent, icmpv6-type 136, target VIP, drop; owner name `vrrp-owner`
- [ ] `internal/test/fixture/netfilter_fixture.go` and `netfilter_fixture_firewall.go` - `registerTableSnapshot(name, family, table)` runs `nft list table <family> <table>` for a `.ci` to assert against
- [ ] `gokrazy/kernel/kernel.require` - lists `CONFIG_NF_TABLES_ARP`, so the appliance kernel carries it

**Behavior to preserve:**
- The VRRP owner filter's tables, terms and resulting packet behavior: `TestVRRPOwnerAnswersWithVirtualMACOnly` (QEMU) stays green
- Existing refusals and their message text in `validateMatch` for the three existing types (the parse tests below assert them)
- `show firewall` formatting of the three existing matches
- Every existing `from` leaf keeps its behavior in inet, ip and ip6 tables
- The arp hook mapping to `NF_ARP_IN` and `NF_ARP_OUT`
- Set contributions by IRR and domain into operator tables still merge through `mergeSameNameTables`

**Behavior to change:**
- Six new `from` leaves: three produce the existing match types, three produce new match types (D-1)
- In a table of family arp, every `from` leaf that reads an IP header, a transport header or conntrack is refused at verify (today most are accepted and misread or never match)
- The ND target lowering restricts the ICMPv6 type to 135 or 136 itself; a term naming `nd-target-address` with an `icmpv6-type` other than 135 or 136 is refused at verify because it can never match (D-2)
- A config with a table of family arp on a kernel without the nftables arp family is refused at start, reload and validate with a named diagnostic
- Each producer reserves the kernel table names it owns; verify refuses an operator table whose kernel name another owner reserved, naming that owner, and `RegisterTables` refuses any owner registering a name another owner reserved (D-3)
- The web firewall page prints the six matches in words instead of Go type names

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config text: `firewall { table <name> { family arp|ip6|inet; chain <c> { ... term <t> { from { arp-operation reply; arp-sender-address 192.0.2.1; arp-target-hardware-address 02:00:5e:00:01:01; } ... } } } }`, or `set` commands in the CLI editor
- YANG-validated tree, delivered to the firewall component as `map[string]any` with every leaf value a string

### Transformation Path
1. YANG parse and native validation: enum for `arp-operation`; `zt:ipv4-address` for the two ARP protocol-address leaves; `zt:ipv6-address` for `nd-target-address`; the MAC pattern plus `ze:validate "mac-address"` for the two hardware-address leaves; `ze:backend "nft"` refuses every one of the six on another backend
2. `parseFromBlock` (`internal/component/firewall/config.go`) turns each leaf into its Match
3. `ValidateTables` and `validateMatch` (`validate.go`) check family admissibility for every match in the term, address validity, and the ND term's ICMPv6 type; the table-level check refuses a reserved kernel name
4. `RegisterTables` under the firewall engine owner refuses a name another owner reserved, then `ApplyAll` merges with other owners and calls the nft backend
5. `lowerMatch` (`internal/plugins/firewall/nft/lower_linux.go`) emits the expressions: existing lowering for the ARP opcode and sender address, new lowering for the three new types, the ND target lowering with its own type restriction
6. Kernel: nftables table `ze_<name>` in family arp, ip6 or inet
7. `show firewall` reads the last applied model and prints through `formatMatch`; the web page prints through `matchSummary`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config file to firewall component | YANG tree to `map[string]any`, string leaf values | No |
| Firewall component to nft backend | `[]firewall.Table` through the table registry and `Backend.Apply` | No |
| nft backend to kernel | nftables netlink batch, family `NFPROTO_ARP`, `NFPROTO_IPV6` or `NFPROTO_INET` | No |
| Kernel capability to startup gate | `kernelcap.Evaluate` and `Refuse` from `cmd/ze/hub` and `ze config validate` | No |
| Producer package to firewall registry | name reservation from each producer's `init` | No |

### Integration Points
- `parseFromBlock` - six new key reads beside `icmpv6-type`
- `validateMatch` - cases for the three new types; arp-family refusals for the IP, transport and conntrack match types; the ND type check reads the term's `MatchICMPv6Type`
- `lowerMatch` - three new cases; `lowerNDTargetAddressMatch` gains the type restriction
- `formatMatch`, `matchTypeName` (`cmd/show.go`) and `matchSummary` (`web/page_firewall.go`) - cases for the new types, and for the existing three on the web page
- `kernelcap.MustRegister` - new enrolment in the nft backend package
- Table registry - a name reservation beside `RegisterTables`, read by `ValidateTables` and by `RegisterTables`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes (planned) | Leaves enter through YANG and `parseFromBlock` like every other `from` leaf |
| No unintended coupling (components stay isolated) | Yes (planned) | The firewall component learns nothing about VRRP or any other producer; each producer reserves its own names from its own package |
| No duplicated functionality (extends existing, does not recreate) | Yes (planned) | Reuses the three existing types; the new address lowerings reuse `arpEthernetIPv4` and the guard shape of `lowerARPSenderAddressMatch` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Config-time path, no wire encoding |
| Registration over hardcoding, outbound | Yes (planned) | The kernel need registers through `kernelcap.MustRegister` from the nft package; each producer reserves its table names through the firewall registry from its own `init`, using the constant it already declares |
| Registration over hardcoding, inbound | Partly (planned) | `parseFromBlock`, `validateMatch`, `lowerMatch`, `formatMatch`, `matchTypeName` and `matchSummary` are per-match-type switches today and every existing match is added there; searched on 2026-09-28 with a grep for `MatchARPSenderAddress` and `MatchICMPv6Type`. `TestParsedNamesMatchTheModel` gates the name tables; the ARP opcode names get one value-to-name map in `model.go` read through `nameIndex`, the way `familyNames` is. The firewall registry holds no producer name: the reservation list is whatever producers registered |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `meta iifname`, `meta oifname` and `meta mark` evaluate normally in an nftables arp chain | `ownerARPReplyTerm` uses `MatchOutputInterface` in an arp table and the QEMU test passes | input-interface, output-interface or mark must also be refused in arp | Integration test AC-17 with `output-interface` beside `arp-operation` | unvalidated |
| A-2 | In an arp chain, a transport-header payload read and `meta l4proto` never match (no L4 info), and `ct state` reads the packet as untracked or invalid rather than breaking | Reading of kernel `nft_payload_eval` and `nft_ct_get_eval` from memory | The refusal list in AC-12 is still right (refusing is safe either way); only its wording changes | Unit refusal tests do not depend on it; recorded so the message does not claim a kernel behavior | unvalidated |
| A-3 | A netlink probe can tell a kernel without `CONFIG_NF_TABLES_ARP` from one with it, without executing a binary and without committing state, by sending an nftables batch that declares an arp table and a filter base chain and omits the batch end, so the kernel validates then aborts | nft's own check mode works this way; `github.com/google/nftables` exposes a raw batch path (unverified) | The probe falls back to `/proc/config.gz` where present and reports Unknown otherwise, which warns and never refuses | `TestARPFamilyProbeClassifiesEachErrno` plus the QEMU run of AC-14 on a kernel built without the option | unvalidated |
| A-4 | Without CAP_NET_ADMIN the probe gets EPERM and must answer Unknown | nfnetlink requires CAP_NET_ADMIN for every nftables message | A non-root `ze config validate` would refuse wrongly | `TestARPFamilyProbeEPERMIsUnknown` | unvalidated |
| A-5 | nft 1.0.x renders the lowered compares as `arp operation`, `arp saddr ip`, `arp daddr ip`, `arp saddr ether`, `arp daddr ether`, `icmpv6 type` and `icmpv6 taddr` in `nft list table` | nftables manual lists every field | The `.ci` assertions use a pattern that also accepts the raw `@nh,<bit>,<len>` and `@th,<bit>,<len>` forms | First run of the functional tests | unvalidated |
| A-6 | The functional test runner's kernel carries `CONFIG_NF_TABLES_ARP` | The VRRP QEMU runtime kernel does; the Docker runner kernel is the host's | The arp functional test needs the QEMU runner instead of `needs-linux` | First run of `test/firewall/firewall-arp-match.ci` | unvalidated |
| A-7 | Two base chains of the same name inside one merged table either fail the whole reconcile or lose one owner's chain | `mergeSameNameTables` concatenates chains without a name check | No design change: D-3 refuses the colliding table before the merge, so the kernel outcome is no longer load-bearing. Recorded so no page claims an outcome nobody measured | Not measured by this spec; the ownership page states the refusal, not a kernel outcome | unvalidated (not load-bearing after D-3) |
| A-8 | Apart from IRR and domain set contributions into operator tables, every producer registers only tables it alone owns, so reserving each producer's own names refuses no legitimate registration | Grep of `RegisterTables` callers and their table-name constants, 2026-09-28 (Current Behavior) | A producer that contributes into another producer's table is refused by `RegisterTables`; the reservation then admits named contributors, which is a design change back to DESIGN | `TestRegisterTablesRefusesAnotherOwnersReservedName` plus the existing functional tests of every producer (copp, flowspec, policy-routes, ddos-local, gtsm, IRR, domain, VRRP) staying green | unvalidated |
| A-9 | The process that runs `ze config validate` and commit verify links every producer package, so every reservation is registered when verify runs | Producers register through `init` and the composition root `internal/component/plugin/all/all.go` imports them (not checked for `ze config validate`) | Validate accepts a colliding name that the daemon then refuses at `RegisterTables`; the kernel stays protected (AC-22) but the operator learns late | `test/parse/firewall-table-name-reserved-rejected.ci` runs `ze config validate` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An operator ND target leaf reads the wrong 16 octets of an ICMPv6 message that is not ND | A term with only `nd-target-address` matches an echo request | D-2: the lowering restricts the type to 135 or 136 itself; `TestLowerNDTargetAddressRestrictsType` and AC-18's traffic test prove it |
| R-2 | Tightening arp-family validation refuses a config that parses today | Parse tests of existing configs, `ze config validate` over `test/` fixtures | Every refused leaf misreads or never matches in arp today, so no working config is lost; the error names the family and the leaf |
| R-3 | An operator drop rule in an arp or ip6 output chain drops the VRRP macvlan's own ARP replies or advertisements and the virtual address goes dark | VRRP functional tests with an operator table beside them | nftables runs every base chain at a hook; a drop anywhere is final, an accept is not, so an operator accept cannot undo the VRRP owner drop, but an operator drop can remove what VRRP needs. The guide says so beside the example |
| R-4 | Operator table name equal to a producer's table name merges into it | Journal row `two-owners-share-one-name.md` | D-3: verify refuses it naming the owner; `RegisterTables` refuses it on any path that bypassed verify |
| R-5 | The kernelcap probe refuses a working router | Absent reported on a kernel that has the family | Unknown on every error the probe cannot classify; only the specific "family not supported" errno answers Absent |
| R-6 | The ND type restriction added to `lowerNDTargetAddressMatch` changes the VRRP owner's kernel rule | QEMU test red | The added restriction admits 136, the only type the VRRP term names; `TestVRRPOwnerAnswersWithVirtualMACOnly` reruns |
| R-7 | The vpp backend accepts a leaf and then refuses the match with a generic error | Parse test on `backend vpp` | `ze:backend "nft"` on each leaf refuses at commit with the backend named |
| R-8 | An operator config that already names a table `copp`, `pr`, `flowspec`, `gtsm`, `vrrp`, `vrrp_owner_arp`, `vrrp_owner_nd`, `ddos-local` or `irr_iface` stops validating | `ze config validate` over `test/` fixtures | Ze is pre-release; the error names the owner so the operator renames the table. Each such config merged into a producer's table today, so none worked as written |
| R-9 | A new ARP address match lowered without the layout guard matches a non-Ethernet/IPv4 packet on the wrong bytes | Unit test of the expression list | Each of the four address lowerings starts with the `arpEthernetIPv4` compare; `TestLowerARPAddressMatchesCarryTheLayoutGuard` asserts it per type |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An operator arp or ip6 filter matches the wrong packets; the VRRP owner stops suppressing physical-MAC answers; a producer's registration is wrongly refused and its tables leave the kernel; a wrong kernelcap probe stops the daemon starting |
| How is it reverted? | Single commit revert; the new leaves are additive, the arp refusals only refuse configs that misread, the reservation only refuses configs that collided |
| Who else touches this path? | `plan/immediate/spec-vrrp-owner-arp-rfc-defects.md` (vpp verifier D2, owner ARP request D1), `plan/spec-dataplane-seams-5-copp-non-tcp.md` (ARP policing needs a non-inet family); every table producer listed in Current Behavior gains a reservation call |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config `from { arp-operation reply; arp-sender-address 192.0.2.1; }` in a family arp table | → | `parseFromBlock` to `lowerARPOperationMatch`, `lowerARPSenderAddressMatch` | `test/firewall/firewall-arp-match.ci` |
| Config `from { arp-target-address 192.0.2.2; }`, `from { arp-sender-hardware-address 02:00:5e:00:01:01; arp-target-hardware-address 02:00:5e:00:01:02; }` in the same arp table | → | `parseFromBlock` to the three new ARP lowerings | `test/firewall/firewall-arp-match.ci` |
| Config `from { nd-target-address 2001:db8::1; }` and `from { icmpv6-type nd-neighbor-advert; nd-target-address 2001:db8::2; }` in a family ip6 table | → | `parseFromBlock` to `lowerNDTargetAddressMatch` | `test/firewall/firewall-nd-target-match.ci` |
| `ze config validate` on an arp-family misuse | → | `validateMatch` | `test/parse/firewall-arp-source-address-rejected.ci` |
| `ze config validate` on an operator table named `vrrp_owner_nd` | → | the table check reading the name reservations | `test/parse/firewall-table-name-reserved-rejected.ci` |
| `ze doctor` and daemon start with a family arp table | → | the nft package's kernelcap enrolment | `TestFirewallARPFamilyCapabilityInUse` and `test/parse/firewall-arp-kernelcap-absent.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Leaf `arp-operation` with value `request` or `reply` in a family arp table | Config validates; the term holds `MatchARPOperation` with opcode 1 or 2; the kernel rule compares network-header offset 6 with 0001 or 0002 |
| AC-2 | `arp-operation` with any other value | Refused by YANG enum validation, with the valid values offered |
| AC-3 | Leaf `arp-sender-address 192.0.2.1` in a family arp table | Config validates; the kernel rule carries the Ethernet/IPv4 layout compare and the 4-octet compare at network-header offset 14 |
| AC-4 | Leaf `arp-target-address 192.0.2.2` in a family arp table | Config validates; the term holds `MatchARPTargetAddress`; the kernel rule carries the Ethernet/IPv4 layout compare and the 4-octet compare at offset 24 |
| AC-5 | Leaf `arp-sender-hardware-address 02:00:5e:00:01:01` in a family arp table | Config validates; the term holds `MatchARPSenderHardwareAddress`; the kernel rule carries the Ethernet/IPv4 layout compare and the 6-octet compare at offset 8 |
| AC-6 | Leaf `arp-target-hardware-address 02:00:5e:00:01:02` in a family arp table | Config validates; the term holds `MatchARPTargetHardwareAddress`; the kernel rule carries the Ethernet/IPv4 layout compare and the 6-octet compare at offset 18. Any 6-octet value, including all zeros, is accepted |
| AC-7 | `arp-sender-address` or `arp-target-address` given `0.0.0.0`, an IPv6 address, or a prefix; a hardware-address leaf given anything but six colon-separated hex octets | YANG refuses the non-IPv4 and non-MAC forms; `0.0.0.0` is refused by `validateMatch` with "arp-sender-address names no IPv4 address" or "arp-target-address names no IPv4 address" |
| AC-8 | Any of the five ARP leaves in a table of family inet, ip, ip6, bridge or netdev | Refused at verify with "<leaf> match is valid only in family arp, got <family>" |
| AC-9 | Leaf `nd-target-address 2001:db8::1` in a family ip6 or inet table, alone or beside `icmpv6-type nd-neighbor-solicit` or `nd-neighbor-advert` | Config validates; the kernel rule restricts l4proto to ICMPv6, restricts the ICMPv6 type to 135 through 136, carries the named type's compare when `icmpv6-type` is present, and compares transport offset 8 with the 16 octets; in inet it also carries the nfproto IPv6 guard |
| AC-10 | `nd-target-address` in a family ip, arp, bridge or netdev table; or given `::`, an IPv4 address, or an IPv4-mapped address | Refused with the existing messages ("valid only in family ip6 or inet", "names no IPv6 address") |
| AC-11 | `nd-target-address` beside an `icmpv6-type` other than 135 or 136 | Refused at verify with a message saying the term can never match and naming nd-neighbor-solicit and nd-neighbor-advert |
| AC-12 | In a table of family arp, any of source-address, destination-address, source-port, destination-port, protocol, connection-state, connection-mark, tcp-flags, a set match, an IRR or domain-group match | Refused at verify naming the leaf and "family arp carries no IP header"; icmp-type, icmpv6-type and dscp keep their existing refusals |
| AC-13 | In a table of family arp, input-interface, output-interface and mark beside an ARP leaf | Accepted and matched (A-1) |
| AC-14 | Config holding a table of family arp, host kernel without the nftables arp family | `ze config validate` reports `config-kernel-capability` at error severity; the daemon refuses to start with one line naming CONFIG_NF_TABLES_ARP and the table; `ze doctor` prints the absent code; a probe that cannot decide warns with the unknown code and starts |
| AC-15 | An operator table whose kernel name equals a name another producer reserved (`vrrp_owner_arp`, `vrrp_owner_nd`, `vrrp` among them), in any family, whether or not that producer is active | Refused at verify naming the table, its kernel name and the owner that holds it; nothing is registered, so the VRRP owner table in the kernel keeps its terms |
| AC-16 | An operator arp table with an accept term for ARP replies from a VRRP-owned address, beside the VRRP owner table | The VRRP owner drop still removes the parent's physical-MAC reply (drop in any base chain is final) |
| AC-17 | Traffic through a family arp input chain with one counted term per address leaf: sender 192.0.2.1, target 192.0.2.2, sender hardware 02:00:5e:00:01:01, target hardware 02:00:5e:00:01:02, the first also carrying `arp-operation reply` and `output-interface` or `input-interface` | For each term, a frame carrying the named value is counted and a frame differing only in that field is not; a non-Ethernet/IPv4 frame is counted by none |
| AC-18 | Traffic through a family ip6 input chain whose counted term names only `nd-target-address 2001:db8::1` | A Neighbor Advertisement for 2001:db8::1 is counted; an ICMPv6 echo request whose octets 8 to 23 equal 2001:db8::1 is not |
| AC-19 | `show firewall` and the web firewall page with an operator table holding all six leaves | Both print "arp operation reply", "arp sender address 192.0.2.1", "arp target address 192.0.2.2", "arp sender hardware address 02:00:5e:00:01:01", "arp target hardware address 02:00:5e:00:01:02", "nd target address 2001:db8::1"; the web page prints no Go type name |
| AC-20 | Any of the six leaves under `backend vpp` | Refused at commit by `ze:backend`, naming the nft backend |
| AC-21 | Every new leaf and the `arp-operation` enum values | Carry `ze:help` and `description` that `./le doc yang-contract help-shape` accepts |
| AC-22 | Any owner calls `RegisterTables` with a table name another owner reserved | Refused with an error naming both owners; the registry keeps the reserving owner's tables unchanged. IRR and domain registering sets into an operator table's name stay accepted |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator drops ARP replies claiming the gateway address arriving on a customer port | config, `parseFromBlock`, `validateMatch`, registry, `lowerMatch`, kernel arp input chain | `test/firewall/firewall-arp-match.ci`, `TestNftIntegrationARPAddressMatchesCountOnlyTheirPackets` |
| 2 | Operator drops ARP from a spoofing MAC, or ARP requests for one target address | same path, the new hardware and target lowerings | `test/firewall/firewall-arp-match.ci`, `TestNftIntegrationARPAddressMatchesCountOnlyTheirPackets` |
| 3 | Operator drops Neighbor Advertisements that claim a protected IPv6 address | config, parse, validate, lowering, kernel ip6 input chain | `test/firewall/firewall-nd-target-match.ci`, `TestNftIntegrationNDTargetIgnoresOtherICMPv6Types` |
| 4 | Operator writes `source-address` in an arp table by mistake | config, `validateMatch` refusal | `test/parse/firewall-arp-source-address-rejected.ci` |
| 5 | Operator names a table `vrrp_owner_nd` | config, table check against the reservations | `test/parse/firewall-table-name-reserved-rejected.ci` |
| 6 | Operator deploys an arp table on a kernel built without the arp nftables family | kernelcap Evaluate at validate and start | `test/parse/firewall-arp-kernelcap-absent.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseFromBlockARPOperation` | `internal/component/firewall/config_test.go` | request and reply map to opcodes 1 and 2; an unknown name errors (AC-1, AC-2) | |
| `TestParseFromBlockARPAddresses` | `internal/component/firewall/config_test.go` | one sub-test per ARP address leaf: the value lands in the right Match type with the right bytes (AC-3 to AC-6) | |
| `TestParseFromBlockNDTargetAddress` | `internal/component/firewall/config_test.go` | IPv6 address to `MatchNDTargetAddress` (AC-9) | |
| `TestParsedNamesMatchTheModel` (extended) | `internal/component/firewall/model_enum_test.go` | the ARP opcode name map covers every valid `ARPOperation` and the YANG enum | |
| `TestValidateARPMatchesOnlyInFamilyARP` | `internal/component/firewall/validate_test.go` | each of the five ARP match types refused in inet, ip, ip6, bridge and netdev with its leaf named (AC-8) | |
| `TestValidateARPTargetAddress` | `internal/component/firewall/validate_test.go` | unspecified and non-IPv4 refused (AC-7) | |
| `TestValidateARPFamilyRefusesIPHeaderMatches` | `internal/component/firewall/validate_test.go` | one sub-test per leaf in AC-12, each refused with the leaf named | |
| `TestValidateARPFamilyAdmitsMetaMatches` | `internal/component/firewall/validate_test.go` | AC-13 | |
| `TestValidateNDTargetTypeRestriction` | `internal/component/firewall/validate_test.go` | no type, 135 and 136 accepted; 134, 137 and 128 refused with the can-never-match message (AC-9, AC-11) | |
| `TestLowerNDTargetAddressRestrictsType` | `internal/plugins/firewall/nft/lower_linux_test.go` | the expression list carries the ICMPv6 type restriction to 135 through 136 before the target compare, in ip6 and inet (AC-9) | |
| `TestLowerARPAddressMatchesCarryTheLayoutGuard` | `internal/plugins/firewall/nft/lower_linux_test.go` | each of the four ARP address lowerings starts with the `arpEthernetIPv4` compare and reads its offset and length (8/6, 14/4, 18/6, 24/4) (AC-3 to AC-6, R-9) | |
| `TestLowerARPAddressMatchesFailClosed` | `internal/plugins/firewall/nft/lower_linux_test.go` | an unspecified or non-IPv4 protocol address passed around validation is refused by the lowering | |
| `TestFirewallARPFamilyCapabilityInUse` | `internal/plugins/firewall/nft/kernelcap_linux_test.go` | InUse true only for a tree with a family arp table under backend nft (AC-14) | |
| `TestARPFamilyProbeClassifiesEachErrno`, `TestARPFamilyProbeEPERMIsUnknown` | `internal/plugins/firewall/nft/kernelcap_linux_test.go` | Present, Absent and Unknown classification (A-3, A-4) | |
| `TestVerifyRefusesAReservedTableName` | `internal/component/firewall/validate_test.go` | for every name in the reservation registry (read from it, never a hand list), an operator table taking it in any family is refused with the owner named; an unreserved name passes (AC-15) | |
| `TestRegisterTablesRefusesAnotherOwnersReservedName` | `internal/component/firewall/registry_test.go` | AC-22 refusal, the reserving owner's tables unchanged, and an IRR-style set contribution into an operator table name still accepted and merged | |
| `TestReservationsDeclaredByEachProducer` | `internal/plugins/vrrp/ownerfilter_test.go` and one per producer package | each producer's table-name constant is reserved under its owner name | |
| `TestFormatMatchARPND` | `internal/component/firewall/cmd/show_test.go` | AC-19 strings for all six | |
| `TestMatchSummaryARPND` | `internal/component/web/page_firewall_test.go` | AC-19 strings for all six, no Go type name | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `arp-operation` opcode | 1-2 (enum request, reply) | 2 | 0 (Unspecified, refused by `validateMatch`) | 3 (not an enum value) |
| ICMPv6 type beside `nd-target-address` | 135-136 | 136 | 134 | 137 |
| Hardware address length | exactly 6 octets | 6 | 5 octets (YANG pattern) | 7 octets (YANG pattern) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `firewall-arp-match` | `test/firewall/firewall-arp-match.ci`, snapshot registered as `registerTableSnapshot("firewall/firewall-arp-match", "arp", "ze_fwarp")` | nft list shows `arp operation reply`, `arp saddr ip 192.0.2.1`, `arp daddr ip 192.0.2.2`, `arp saddr ether 02:00:5e:00:01:01`, `arp daddr ether 02:00:5e:00:01:02` (or the raw payload forms, A-5) in chain input | |
| `firewall-nd-target-match` | `test/firewall/firewall-nd-target-match.ci`, snapshot family ip6 | nft list shows the ICMPv6 type restriction and `icmpv6 taddr 2001:db8::1` for the term without `icmpv6-type`, and `icmpv6 type nd-neighbor-advert` for the term with it | |
| `firewall-arp-matches-accepted` | `test/parse/firewall-arp-matches-accepted.ci` | `ze config validate` exit 0 for all six leaves in their families | |
| `firewall-arp-in-inet-rejected` | `test/parse/firewall-arp-in-inet-rejected.ci` | AC-8 message | |
| `firewall-arp-target-address-rejected` | `test/parse/firewall-arp-target-address-rejected.ci` | AC-7 message for `0.0.0.0` | |
| `firewall-nd-target-in-ip-rejected` | `test/parse/firewall-nd-target-in-ip-rejected.ci` | AC-10 message | |
| `firewall-nd-target-type-rejected` | `test/parse/firewall-nd-target-type-rejected.ci` | AC-11 message for `icmpv6-type echo-request` beside `nd-target-address` | |
| `firewall-arp-source-address-rejected` | `test/parse/firewall-arp-source-address-rejected.ci` | AC-12 message | |
| `firewall-arp-leaf-vpp-rejected` | `test/parse/firewall-arp-leaf-vpp-rejected.ci` | AC-20 | |
| `firewall-arp-kernelcap-absent` | `test/parse/firewall-arp-kernelcap-absent.ci` | AC-14 with the probe forced Absent through the kernelcap test override | |
| `firewall-table-name-reserved-rejected` | `test/parse/firewall-table-name-reserved-rejected.ci` | AC-15 message naming `vrrp-owner` for a table `vrrp_owner_nd` | |
| `firewall-cli-show` (extended) | `test/firewall/firewall-cli-show.ci` | AC-19 | |
| `TestNftIntegrationARPAddressMatchesCountOnlyTheirPackets` | `internal/plugins/firewall/nft/counter_integration_linux_test.go` (integration, linux) | AC-17 with real ARP frames in `withNftNetNS` | |
| `TestNftIntegrationNDTargetIgnoresOtherICMPv6Types` | `internal/plugins/firewall/nft/counter_integration_linux_test.go` (integration, linux) | AC-18 with a real NA and a crafted echo request | |
| `TestVRRPOwnerSurvivesOperatorARPTable` | QEMU suite beside `TestVRRPOwnerAnswersWithVirtualMACOnly` | AC-16: operator accept table present, physical-MAC reply still dropped | |

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | A local packet filter with no protocol peer; the Linux kernel is the only other party, and AC-17, AC-18 and the QEMU test exercise it with real frames | |

## Files to Modify
- `internal/component/firewall/yang/ze-firewall-conf.yang` - six leaves in `from-block`, each `ze:backend "nft"`; `family` leaf description names what arp admits
- `internal/component/firewall/config.go` - `parseFromBlock` reads the six leaves; ARP opcode parse through a name index; MAC text to 6 octets
- `internal/component/firewall/model.go` - three new match types (below); ARP opcode value-to-name map; remove "Daemon-only" sentences from the three existing match comments; `MatchNDTargetAddress` comment says the lowering restricts the type; new types added to the `matchMarker` list
- `internal/component/firewall/validate.go` - cases for the three new types; arp-family refusals (AC-12); ND type rule (AC-11); reserved-name table check (AC-15); comment on `validateSetFamilyCompat` no longer calls the literal gap untracked
- `internal/component/firewall/registry.go` - name reservation beside `RegisterTables`, a lookup for verify, and the `RegisterTables` refusal (AC-22); the `ApplyAll` comment says merging is for contributions into a table no other owner reserved
- `internal/plugins/firewall/nft/lower_linux.go` - `lowerMatch` cases and three new lowerings; ICMPv6 type restriction inside `lowerNDTargetAddressMatch` and its comment updated; layout comment above `lowerARPOperationMatch` names the four address lowerings that rely on it
- `internal/component/firewall/cmd/show.go` - `formatMatch` and `matchTypeName` cases for the three new types
- `internal/component/web/page_firewall.go` - `matchSummary` cases for all six matches with the words `show firewall` prints
- `docs/architecture/web-workbench-pages.md` - the `matchSummary` paragraph claims an exhaustive switch over "15 match types", which is already wrong (the model has more, and the ARP and ND types fall to the `%T` arm); phase 4 drops the count and says the switch names every match type, with the six added here
- Every table producer reserves its names from its own `init`, using the constant it already declares: `internal/plugins/vrrp/` (ownerfilter and acceptfilter), `internal/plugins/flowspec-firewall/`, `internal/plugins/copp/`, `internal/plugins/policyroute/`, `internal/plugins/ddos/local/`, `internal/component/gtsm/`, `internal/component/firewall/plugins/irr/` (for `ze_irr_iface` only)
- `internal/core/diagnostic/codes.go` - absent and unknown codes for the arp family capability
- `plan/journal/two-owners-share-one-name.md` - the 2026-09-28 row's status becomes fixed, naming the commit
- `docs/guide/firewall.md`, wiki `firewall.md` (`~/Code/github.com/ze-software/ze/wiki/firewall.md`), `docs/architecture/firewall/table-ownership-and-shutdown-flush.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/guide/vrrp.md` (operator table interaction), `docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md`, `docs/features.md`
- `internal/test/fixture/netfilter_fixture.go` - two table snapshot registrations

New model types (all family arp only, lowered with the Ethernet/IPv4 layout guard):

| Type | Field | Type of field | Description |
|------|-------|---------------|-------------|
| `MatchARPTargetAddress` | Addr | `netip.Addr` | ARP target protocol address; a specified IPv4 address, validated like the sender |
| `MatchARPSenderHardwareAddress` | Addr | 6-octet array | ARP sender hardware address; the fixed length makes a wrong-length value unrepresentable |
| `MatchARPTargetHardwareAddress` | Addr | 6-octet array | ARP target hardware address; same shape |

## Files to Create
- `internal/plugins/firewall/nft/kernelcap_linux.go` - `kernelcap.MustRegister` for CONFIG_NF_TABLES_ARP, InUse and Probe
- `internal/plugins/firewall/nft/kernelcap_linux_test.go`
- `test/firewall/firewall-arp-match.ci`, `test/firewall/firewall-nd-target-match.ci`
- `test/parse/firewall-arp-matches-accepted.ci`, `firewall-arp-in-inet-rejected.ci`, `firewall-arp-target-address-rejected.ci`, `firewall-nd-target-in-ip-rejected.ci`, `firewall-nd-target-type-rejected.ci`, `firewall-arp-source-address-rejected.ci`, `firewall-arp-leaf-vpp-rejected.ci`, `firewall-arp-kernelcap-absent.ci`, `firewall-table-name-reserved-rejected.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/firewall/yang/ze-firewall-conf.yang`, grouping `from-block` |
| YANG validation constraints | Yes | enum for `arp-operation`; `zt:ipv4-address` for the ARP protocol-address leaves; `zt:ipv6-address` for `nd-target-address`; MAC pattern for the hardware-address leaves |
| YANG custom validators | Yes | the existing `ze:validate "mac-address"` on the two hardware-address leaves; family admissibility needs the table's family, which `validateMatch` already has, so no new validator |
| CLI commands/flags | No | No command added; `show firewall` output gains three match strings |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | Yes | Automatic from the enum; the `mac-address` complete function serves the hardware leaves; `ze:backend "nft"` filters the leaves off under vpp |
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
| 2 | Config syntax changed? | Yes | `docs/guide/firewall.md` match table, families and reserved table names; `docs/guide/configuration.md` only if it lists `from` leaves (check) |
| 3 | CLI command added/changed? | No | `show firewall` output unchanged in shape |
| 4 | API/RPC added/changed? | No | No RPC |
| 5 | Plugin added/changed? | No | Producers gain a reservation call and the nft backend an enrolment, not a plugin surface |
| 6 | Has a user guide page? | Yes | `docs/guide/firewall.md`, wiki `firewall.md`, `docs/guide/vrrp.md` (operator table beside the owner tables) |
| 7 | Wire format changed? | No | No Ze wire format |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | No | Filtering ARP and ND is operator policy, not an RFC obligation; the VRRP rows are unchanged |
| 10 | Test infrastructure changed? | No | Two snapshot registrations use the existing helper |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if it has a firewall ARP row (check) |
| 12 | Internal architecture changed? | Yes | `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` (VRRP producers, the name reservation), `docs/architecture/doctor-and-health-checks.md` (third enrolment) |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | kernel capability inventory in `docs/architecture/doctor-and-health-checks.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Declared by `// Design:` headers of files this spec changes: `docs/architecture/core-design.md` (declared by `internal/component/firewall/config.go`, `model.go`, `registry.go` and the nft package) is unaffected, because it describes registration and composition in general and the reservation follows that pattern without changing it; `docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md` (declared by `internal/plugins/vrrp/ownerfilter.go`) is updated in phase 5 with the operator-table interaction and the reservation; `docs/features/ai-first.md` (declared by `internal/core/diagnostic/codes.go`) is unaffected, because it describes `ze explain` generically and lists no individual code; `docs/architecture/web-workbench-pages.md` (declared by `internal/component/web/page_firewall.go`) is updated in phase 4, because its `matchSummary` paragraph counts 15 match types and this change makes that more wrong than it already is. Producer packages gaining a reservation call (copp, flowspec-firewall, policyroute, ddos local, gtsm, irr, vrrp acceptfilter) and `internal/component/web/page_firewall.go` are listed by the anchor command at implementation; each doc it names is judged there. Advisory mentions: `docs/architecture/firewall/firewall-irr.md` (mentions `registry.go`, `config.go`, `model.go`, `validate.go`) is re-read in phase 5, because the IRR sets rely on the same-name merge the reservation must leave working; `docs/architecture/ddos/cp-survival-5-detect-5-characterization.md`, `docs/architecture/traffic/cp-survival-2-copp-port179.md`, `docs/architecture/bgp/as112-coordination.md`, `docs/guide/redistribution.md` and `docs/guide/vpp.md` mention the changed files for matches, codes and parsing this spec does not alter. Re-run `./le spec citation anchors spec plan/spec-firewall-arp-nd-matches.md` at implementation and name each doc it lists |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | guide and wiki firewall examples; verify against YANG after the change |

## Implementation Steps

1. **Phase: Wiring** - YANG leaves, `parseFromBlock` reads, the three new model types with markers, fixture registrations, the two `test/firewall` `.ci` files and the accepted parse test; they fail until the leaves parse and lower
2. **Phase: Validation** - family admissibility for the five ARP types (AC-8), target address validity (AC-7), arp-family refusals (AC-12, AC-13), ND type rule (AC-11), `ze:backend` (AC-20), parse `.ci` refusals; `TestValidate*` red then green
3. **Phase: Lowering** - three new ARP lowerings with the layout guard, the ND type restriction; `TestLower*` red then green, then AC-17 and AC-18 integration tests; rerun `TestVRRPOwnerAnswersWithVirtualMACOnly` (R-6)
4. **Phase: Display** - `formatMatch`, `matchTypeName`, web `matchSummary` (AC-19), `docs/architecture/web-workbench-pages.md`
5. **Phase: Table name reservation** - reservation registry, verify check, `RegisterTables` refusal, one reservation per producer, AC-15, AC-16, AC-22 tests, `docs/architecture/firewall/table-ownership-and-shutdown-flush.md`, `docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md`, journal row status
6. **Phase: Kernel capability** - enrolment, probe, codes, doctor, AC-14 tests
7. **Phase: Docs** - each page is edited in the phase whose change makes it wrong; this phase verifies examples against YANG (row 17)

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | No `from` leaf reaches the nft lowering in an arp table unless it reads the ARP header or packet meta |
| Correctness | Every ARP address compare is preceded by the Ethernet/IPv4 layout compare in the same rule |
| Correctness | An ND target compare never reaches the kernel without the 135-to-136 restriction in the same rule |
| Correctness | The reservation refuses on name alone, in every family; IRR and domain set contributions into operator tables still merge |
| Naming | Leaf names match the words `show firewall` prints and the messages `validateMatch` emits |
| Data flow | The firewall component names no producer or VRRP symbol; reserved names come only from producers' registrations |
| Rule: principles, silent wrong value | Every misread path is refused, not narrowed silently; the web page never prints a Go type name for these matches |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Six YANG leaves with help text | `./le doc yang-contract help-shape` |
| Arp-family refusals | `test/parse/firewall-arp-source-address-rejected.ci` |
| Kernel enrolment | `ze doctor` output in `TestFirewallARPFamilyCapabilityInUse` |
| Real traffic match | `TestNftIntegrationARPAddressMatchesCountOnlyTheirPackets`, `TestNftIntegrationNDTargetIgnoresOtherICMPv6Types` |
| Reserved table names | `test/parse/firewall-table-name-reserved-rejected.ci`, `TestRegisterTablesRefusesAnotherOwnersReservedName` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Protocol-address leaves refuse unspecified and wrong-family addresses; hardware leaves take exactly 6 octets; opcode limited to the enum |
| Fail-open | An operator table cannot remove a VRRP drop (AC-16) or merge into any producer's table (AC-15, AC-22) |
| Resource exhaustion | None new: one match per leaf per term; the reservation holds one entry per producer table |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood, back to RESEARCH |
| Lint failure | Fix inline. If architectural, back to DESIGN |
| Functional test fails | Check the AC: wrong AC to DESIGN, correct AC to IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| A-8 broken (a producer contributes into another producer's table) | Back to DESIGN: the reservation must admit named contributors |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- nftables runs every base chain registered at a hook in priority order; accept ends only that chain, drop is final. This is why an operator table can never cancel the VRRP owner's drop but can drop traffic VRRP needs.
- Each ARP address match carries its own layout guard, so a term naming several address leaves repeats the 6-octet compare once per leaf. That costs one extra load and compare per leaf, and it keeps each match correct on its own, which is what lets validation and lowering treat the matches independently.
- `mergeSameNameTables` serves two different things today: contributions into a table another owner declared (IRR, domain) and accidental collisions. The reservation separates them by what a producer declares, not by changing the merge.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Leaf names `arp-operation`, `arp-sender-address`, `arp-target-address`, `arp-sender-hardware-address`, `arp-target-hardware-address`, `nd-target-address` | nft spellings `arp-saddr-ip`, `arp-daddr-ether`, `icmpv6-taddr`; `source`/`destination` instead of `sender`/`target`; `mac` instead of `hardware-address` | No vendor has an ARP field match to copy; Ze's `from` leaves spell words in full and end in `-address`; sender and target are RFC 826's words and the ones `show firewall` and `validateMatch` already use; the `arp-` prefix names the family the leaf needs |
| Reuse `family arp`, no new family | A dedicated `arp { }` container | The family exists in YANG, model and lowering; a second surface would declare the same fact twice |
| Bare address, not a prefix, for every address leaf | Prefix like `source-address` | The model holds one address and the VRRP use needs one; nftables' arp expression compares a value; a prefix is new model work nobody asked for |
| Hardware address held as a fixed 6-octet array | `net.HardwareAddr` slice | A fixed length makes a wrong-length value unrepresentable, so validation needs only the family check and the lowering needs no length check |
| htype, ptype, hlen, plen not exposed | Four more leaves | They are the layout guard every address match already carries; exposing them adds leaves whose only useful value is the one the guard already demands |
| D-1 resolved: expose every ARP and ND field (see Open Decisions) | Only the three existing matches | Owner decision 2026-09-28 |
| D-2 resolved: the ND lowering restricts the type itself (see Open Decisions) | Verify demands `icmpv6-type` beside the leaf | Owner decision 2026-09-28 |
| D-3 resolved: reserve producer table names and refuse collisions here (see Open Decisions) | Journal row only | Owner decision 2026-09-28 |
| Reservations are static, made from each producer's `init` | Verify reads the live table registry | The live registry holds VRRP's tables only while VRRP has addresses, so a colliding config would validate today and break later; a static reservation refuses it whatever runs |
| Reservation refuses on name alone, in every family | Refuse only equal name and family | The decision names the kernel name as the owned thing; an operator `ze_vrrp_owner_nd` in inet beside VRRP's in ip6 reads as VRRP's table in `nft list ruleset` and in every page that names it |
| `RegisterTables` also refuses a reserved name | Verify only | Verify covers the operator path; the registry covers every other path, including one that bypassed verify, with one fact |

### Open Decisions for the Owner

All three were decided by Thomas on 2026-09-28. None remains open.

| ID | Question | Decision | Date | Owner |
|----|----------|----------|------|-------|
| D-1 | Which ARP and ND fields to expose | Expose everything the firewall can match for ARP and ND: ARP operation, sender IP address, target IP address, sender hardware address, target hardware address, and ND target address. The three new ones get new model match types (model, validation, show formatting) and new nft lowering built the way the existing three are | 2026-09-28 | Thomas |
| D-2 | Where the ND type restriction lives | `lowerNDTargetAddressMatch` adds the ICMPv6 type compare (135 or 136) itself. A term naming `nd-target-address` with an `icmpv6-type` other than 135 or 136 is refused because it can never match | 2026-09-28 | Thomas |
| D-3 | Operator table name collides with a producer's table | Fixed in this spec: each producer registers the table names it owns, and verify refuses an operator table whose kernel name another owner holds, naming that owner. The VRRP owner tables keep their terms (AC-15) | 2026-09-28 | Thomas |

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
- [ ] AC-1..AC-22 all demonstrated
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
