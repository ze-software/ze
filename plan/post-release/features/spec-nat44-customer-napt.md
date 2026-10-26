# Spec: nat44-customer-napt

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P3 |
| Release reason | Adds a capability the `fttx-edge` profile lists as Absent (RFC 3022, RFC 4787, RFC 5382); the shipped firewall `snat`/`masquerade` primitives and their obligations remain required and are not changed in meaning. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Status is `skeleton` only because four owner questions remain (section "Open
Owner Questions"). The research is complete: every reading entry carries its
decision or constraint, and the design below is the recommended answer to each
question. When the owner answers, the spec moves to `design` and then `ready`
without new research.

## Task

The `fttx-edge` use-case profile (`rfc/profiles/fttx-edge.md`) puts customers on
the internet over L2TP, and lists RFC 3022, RFC 4787 and RFC 5382 as Absent. Ze
can already translate addresses: the firewall carries `snat`, `dnat` and
`masquerade` actions lowered to nftables, and a VPP backend lowered to NAT44-ED.
What Ze does not have is a NAPT44 that behaves the way the BEHAVE RFCs require of
a NAT that customer applications (VoIP, gaming, WebRTC, STUN/ICE) depend on.

Measured against the RFC text, the stack today fails these mandatory
requirements:

| Requirement | What Ze plus Linux does today | Evidence |
|-------------|-------------------------------|----------|
| RFC 4787 REQ-1, RFC 5382 REQ-1: Endpoint-Independent Mapping | Linux keys a NAT binding on the whole conntrack reply tuple. It reuses an existing mapping for a new destination only while the resulting tuple is unique, and it hands the same external port to a second internal endpoint toward a different destination, so the first endpoint's next flow to that destination gets another port. VPP NAT44-ED is documented as address and port-dependent mapping | `get_unique_tuple` and `find_appropriate_src` in Linux `net/netfilter/nf_nat_core.c` (6.12 stable, read 2026-10-26); VPP `src/plugins/nat/nat44-ed/nat44_ed_doc.rst`: "The component implements an address and port-dependent mapping and address and port-dependent filtering NAT" |
| RFC 4787 REQ-9, RFC 5382 REQ-8: hairpinning with external source | No rule translates traffic from an inside host to another inside host's external address and port. Conntrack only matches an inbound packet whose tuple equals an existing reply tuple, so a packet from inside host X1 to X2':x2' finds no binding and reaches the router's own stack | same `nf_nat_core.c` lookup; no hairpin rule anywhere in `internal/plugins/firewall/nft/lower_linux.go` |
| RFC 4787 REQ-5: UDP mapping timer not below two minutes | `nf_conntrack_udp_timeout` is 30 s for a flow seen in one direction only; Ze sets no conntrack timer | `/proc/sys/net/netfilter/nf_conntrack_udp_timeout` = 30 on kernel `7.2.9-ze`, read 2026-10-26; `grep` finds no writer of a `nf_conntrack_*_timeout` key in `internal/` (`internal/component/support/collect_linux.go` only reads them) |
| RFC 5382 REQ-5: established idle-timeout not below 2 h 4 min, transitory not below 4 min | FIN_WAIT 120 s and CLOSE_WAIT 60 s, which RFC 5382 Section 5 places in the established phase; SYN_SENT 120 s, SYN_RECV 60 s and LAST_ACK 30 s, which are transitory | the same `/proc/sys/net/netfilter/` keys on `7.2.9-ze` |
| RFC 5382 REQ-4: no response to an unsolicited inbound SYN for 6 s | A SYN to the external address on a port no local socket holds is answered with a RST at once by the local TCP stack, unless an operator's input rule drops it | assumption A-4, proved by test |
| RFC 4787 REQ-7: dynamically configured external interface | Nothing checks a WAN address learned by DHCP or PPPoE against the inside prefixes | no such check under `internal/` |

The goal: an operator writes one `nat44` block naming the customer prefixes and
the uplink, and Ze installs a NAPT44 that meets every MUST and MUST NOT of RFC
4787 and RFC 5382, proven through the real config entry point, a QEMU
integration test on Ze's own kernel, and an interop scenario where a third-party
STUN implementation (RFC 5780 behaviour discovery) classifies Ze's NAT.

RFC 3022 is Informational and carries no MUST (`grep -c '\bMUST\b'
rfc/full/rfc3022.txt` = 0); it is the definition of NAPT this spec builds, not a
source of requirements.

RFC enrolment is frozen by the owner, so this spec writes no `rfc/short/`
summary. The RFC texts are in `rfc/full/rfc3022.txt`, `rfc/full/rfc4787.txt`,
`rfc/full/rfc5382.txt` and `rfc/full/rfc5508.txt` (the BEHAVE-ICMP document RFC
5382 REQ-9 and REQ-10 reiterate). The `RFC gaps` cell stays `-` because no
summary exists to name.

## Open Owner Questions

| # | Question | Evidence | What each answer changes | Recommendation |
|---|----------|----------|--------------------------|----------------|
| Q1 | How does the nft backend get Endpoint-Independent Mapping: a Ze-authored netfilter patch carried in `gokrazy/kernel/patches/`, or VPP only? | Mainline Linux has no EIM mode (`nf_nat_core.c`, row 1 of the Task table). The one public nftables full-cone module, `fullcone-nat-nftables/nft-fullcone`, is archived (GitHub API: `"archived": true`, last push 2023-05-17) and declares no licence, so it is not a dependency. Ze already carries kernel patches (`gokrazy/kernel/patches/series`: `0001-nct6683.patch`, `0002-mpls-ip-mtu.patch`) | Patch: Ze owns C in netfilter (mapping reservation keyed on the external tuple, an inbound lookup for filtering and hairpin) and rebases it at every kernel bump; the default backend becomes conformant. VPP only: the default `nft` backend refuses a `nat44` block and fttx-edge requires VPP; no kernel C. Claiming a Linux limit is not open, because Ze builds the kernel | Patch. Without it the default backend of the release profile cannot meet a MUST |
| Q2 | Default filtering behaviour | RFC 4787 Section 12 REQ-8 and RFC 5382 Section 8 REQ-3 recommend Endpoint-Independent Filtering "if application transparency is most important" and Address-Dependent Filtering "if a more stringent filtering behavior is most important", and let it be configurable. Linux conntrack natively gives address and port-dependent filtering, which neither recommends | EIF default: maximum application transparency, any external host can reach a live mapping. ADF default: needs a per-mapping record of contacted addresses in the kernel patch. APDF default: no extra kernel code, meets every MUST, misses a RECOMMENDED | Configurable `filtering` leaf offering `endpoint-independent` and `address-and-port-dependent` in this spec, default `endpoint-independent` for an FTTx customer edge; `address-dependent` is a SHOULD-level follow-up |
| Q3 | Which branch of RFC 4787 REQ-7 Ze takes | "A NAT device whose external IP interface can be configured dynamically MUST either (1) Automatically ensure that its internal network uses IP addresses that do not conflict with its external network, or (2) Be able to translate and forward traffic between all internal nodes and all external nodes whose IP addresses numerically conflict with the internal network." The fttx-edge WAN can be a PPPoE or DHCP client (`test/interop-pppoe/`, Ze as client). Ze is the LAN's DHCP server in the same profile (roots `rfc2131`, `rfc2132`) | (1): when the learned WAN prefix overlaps an inside prefix, Ze stops leasing the overlapping range and reports it; customers already holding such a lease keep it until renew, so (1) is "ensure" only over a lease period. (2): twice-NAT of the conflicting external range, a second translation table | (1), with the overlap reported by `ze doctor` and a log line, because (2) is a second NAT design for a case an ISP uplink rarely produces |
| Q4 | Does this spec carry the VPP backend (NAT44-EI), or does VPP refuse `nat44` here with a later spec owning it? | Ze vendors only `nat44_ed` binapi (`vendor/go.fd.io/govpp/binapi/`: `nat44_ed`, `nat_types`). NAT44-ED is APDM by its own documentation. VPP ships a separate `nat44-ei` plugin (`src/plugins/nat/nat44-ei/`). `plan/proposals/spec-cgnat.md` (unapproved) already plans CGNAT on NAT44-ED | In this spec: vendor `nat44_ei` binapi, a VPP lowering, a VPP integration proof; the existing VPP masquerade path keeps NAT44-ED, and running both plugins at once is assumption A-7. Separate spec: VPP refuses a `nat44` block with an error naming the backend and the fix | Separate spec, written in the same commit as this one's implementation start, so this spec stays one dataplane |

## Required Reading

### Architecture Docs
- [ ] `docs/guide/firewall.md` - the operator surface for tables, chains, `snat`, `dnat`, `masquerade`, `exclude`, backends
  → Constraint: `nft` is the default backend on Linux, `vpp` is opt-in (Backend table). A `nat44` block must work on the default backend, or the profile has no conformant NAT out of the box
  → Constraint: `masquerade` already accepts `port-range`, `random`, `random full` and `persistent`, mutually exclusive with `port-range`. The `nat44` design must not change what those leaves mean
- [ ] `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` - seven in-process producers register nft tables through `firewall.RegisterTables`
  → Decision: the NAPT44 producer registers its own table `ze_nat44` through `firewall.RegisterTables("nat44", ...)` and reconciles through `firewall.ApplyAll`, exactly as copp does in `applyCoppPolicy` (`internal/plugins/copp/register.go`). It never writes the operator's `firewall` tables
  → Constraint: a producer's table name MUST carry the `ze_` prefix; `RegisterTables` refuses a bare name (Rule 2)
  → Constraint: `ApplyAll` loads the OS-default backend on demand when a producer registers tables and no `firewall {}` block exists (Rule 1), so `nat44` without a `firewall` block still applies
- [ ] `docs/architecture/firewall/fw-6-firewall-vpp.md` - VPP backend, NAT44-ED integration
  → Constraint: the VPP masquerade path marks every VPP interface as a NAT44 output interface (`applyMasquerade` in `internal/plugins/firewall/vpp/nat_linux.go`); that is NAT44-ED, APDM, and cannot back a `nat44` block (Q4)
- [ ] `docs/architecture/testing/interop.md` - suites, scenario naming, typed checkers, catalog registration
  → Constraint: a scenario directory is named, never numbered; its checker is a typed Go function in a lab package under `internal/le/interoplab/`, registered in the lab's `register_catalog.go`
  → Constraint: no NAT suite exists (the suite table, "The suites, one per protocol area"); this spec creates one
- [ ] `ai/rules/platform-linux.md` - kernel config, QEMU proof, labs that depend on host-kernel features
  → Constraint: a Docker lab that depends on host-kernel features MUST also ship a QEMU-runnable path, both native actions in the same change, and a row in `docs/architecture/testing/qemu-integration.md`
  → Constraint: a kernel-touching test carries `integration && linux` and joins `integrationPackages` in `internal/le/test/qemu/alltests.go`; every QEMU proof boots Ze's runtime kernel
- [ ] `ai/rules/rfc-compliance.md` - whole-stack conformance
  → Constraint: a requirement Ze meets by configuring the kernel counts as met only when a test asserts what Ze installed (the ct timeout policy, the mapping flag) and observes the behaviour; delegation to netfilter is `mixed`, and the implementer is named as "Linux netfilter NAT (nf_nat, nf_conntrack) with Ze's EIM patch"
  → Constraint: every enforcing statement carries `// RFC 4787 Section 12: "REQ-N ..."` or `// RFC 5382 Section 8: "REQ-N ..."` with the quote byte-exact

### RFC Texts (Scope: protocol; no `rfc/short/` summary, enrolment frozen)
- [ ] `rfc/full/rfc4787.txt` Section 12 - consolidated UDP requirements
  → Constraint: REQ-1 "A NAT MUST have an "Endpoint-Independent Mapping" behavior."; Section 4.1 defines it: "X1':x1' equals X2':x2' for all values of Y2:y2."
  → Constraint: REQ-3 "A NAT MUST NOT have a "Port assignment" behavior of "Port overloading"." Linux changes the port on a true tuple collision, so it does not overload; the EIM patch must keep that when it reserves ports
  → Constraint: REQ-5 "A NAT UDP mapping timer MUST NOT expire in less than two minutes, unless REQ-5a applies." REQ-5c recommends a five-minute default
  → Constraint: REQ-6 "The NAT mapping Refresh Direction MUST have a "NAT Outbound refresh behavior" of "True"." Conntrack refreshes on every packet in either direction, which also gives the MAY of REQ-6a
  → Constraint: REQ-7 (Q3), REQ-9 "A NAT MUST support "Hairpinning"." with 9a "A NAT Hairpinning behavior MUST be "External source IP address and port"."
  → Constraint: REQ-11 "A NAT MUST have deterministic behavior, i.e., it MUST NOT change the NAT translation (Section 4) or the Filtering (Section 5) Behavior at any point in time, or under any particular conditions." so port exhaustion drops a new mapping; it never falls back to address and port-dependent mapping
  → Constraint: REQ-12 "Receipt of any sort of ICMP message MUST NOT terminate the NAT mapping."
  → Constraint: REQ-13 "If the packet received on an internal IP address has DF=1, the NAT MUST send back an ICMP message "Fragmentation needed and DF set" to the host, as described in [RFC0792]." and 13a "If the packet has DF=0, the NAT MUST fragment the packet and SHOULD send the fragments in order."
  → Constraint: REQ-14 "A NAT MUST support receiving in-order and out-of-order fragments, so it MUST have "Received Fragment Out of Order" behavior." and 14a on fragment DoS
  → Decision: REQ-10 (SHOULD turn UDP ALGs off) is already met: every `CONFIG_NF_CONNTRACK_<helper>` is `=n` in `gokrazy/kernel/kernel.config`, and `/proc/sys/net/netfilter/nf_conntrack_helper` is empty
- [ ] `rfc/full/rfc5382.txt` Sections 5 and 8 - TCP requirements and timer phases
  → Constraint: REQ-1 "A NAT MUST have an "Endpoint-Independent Mapping" behavior for TCP."
  → Constraint: REQ-2 "A NAT MUST support all valid sequences of TCP packets (defined in [RFC0793]) for connections initiated both internally as well as externally when the connection is permitted by the NAT." with 2a simultaneous open
  → Constraint: REQ-4 "A NAT MUST NOT respond to an unsolicited inbound SYN packet for at least 6 seconds after the packet is received." and, within that window after an outbound SYN for the connection, "the NAT MUST silently drop the original unsolicited inbound SYN packet." Silently dropping always satisfies both MUSTs; the ICMP Port Unreachable after 6 s is a SHOULD
  → Constraint: REQ-5 "the value of the "established connection idle-timeout" MUST NOT be less than 2 hours 4 minutes. The value of the "transitory connection idle-timeout" MUST NOT be less than 4 minutes." Section 5 places ESTABLISHED, FIN_WAIT_1, FIN_WAIT_2 and CLOSE_WAIT in the established phase, SYN_SENT and SYN_RCVD in partially open, CLOSING and LAST_ACK in closing, and says TIME_WAIT is not affected by the transitory timeout
  → Decision: the Linux conntrack TCP states map to the RFC phases as: `established`, `fin_wait`, `close_wait`, `unacknowledged`, `retrans` take the established floor (7440 s); `syn_sent`, `syn_recv`, `syn_sent2`, `last_ack` take the transitory floor (240 s); `time_wait` and `close` (after RST) are outside both floors and keep the kernel defaults
  → Constraint: REQ-7 no port overloading, REQ-8 hairpinning with external source, REQ-10 "Receipt of any sort of ICMP message MUST NOT terminate the NAT mapping or TCP connection for which the ICMP was generated."
- [ ] `rfc/full/rfc3022.txt` - Traditional NAT, Informational, zero MUSTs
  → Decision: cited for the NAPT definition only; it adds no requirement row

**Key insights:**
- Ze already ships the translation primitives; what is absent is BEHAVE conformance: EIM, hairpin, RFC timers, unsolicited-SYN silence, WAN-overlap handling
- Mainline Linux cannot do EIM or EIM-based hairpin; Ze's own kernel can carry the patch (Q1)
- Timers are scoped to NAT flows with an nft `ct timeout` object, which needs `CONFIG_NF_CONNTRACK_TIMEOUT=y` (currently "is not set" in the runtime kernel config)
- The feature is a firewall-table producer (`ze_nat44`), like copp, not a change to the operator's `firewall` tables

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/model.go` - abstract model: `SNAT`, `DNAT` (`Address`, `AddressEnd`, `Port`, `PortEnd`, `Flags`), `Masquerade` (`Port`, `PortEnd`, `Flags`), `Table` with chains, sets and flowtables; no conntrack-timeout object, no EIM flag
- [ ] `internal/component/firewall/config.go` - parses `snat { to }`, `dnat { to }`, `masquerade` (`parseMasquerade`) in a term's then-block
- [ ] `internal/component/firewall/yang/ze-firewall-conf.yang` - `container masquerade` with `port-range`, `random { full }`, `persistent`
- [ ] `internal/plugins/firewall/nft/lower_linux.go` - `lowerNAT` refuses any non-zero NAT flag ("NAT flags not yet supported"); `lowerMasquerade` maps port range and random/fully-random/persistent to `expr.Masq`
- [ ] `internal/plugins/firewall/vpp/nat_linux.go` - NAT44-ED: `nat44Enable`, output-interface feature for masquerade, address ranges for SNAT, static mappings for DNAT
- [ ] `internal/component/firewall/registry.go` - `RegisterTables(owner, tables)`, `ApplyAll()`
- [ ] `internal/plugins/copp/register.go`, `internal/plugins/copp/yang/ze-copp-conf.yang` - the producer pattern this spec follows: own top-level YANG container (`control-plane-protection`), own `ze_` table, `RegisterTables` then `ApplyAll`, withdraw by registering nil
- [ ] `internal/component/firewall/cmd_show_conntrack.go` - `show system conntrack`: entry count, table size, timeouts
- [ ] `vendor/github.com/google/nftables/obj.go`, `expr/ct.go` - `ObjTypeCtTimeout` and `expr.CtTimeout` exist in the vendored library
- [ ] `gokrazy/kernel/kernel.config`, `gokrazy/kernel/runtime.config`, `gokrazy/kernel/patches/series` - `NF_NAT`, `NFT_NAT`, `NFT_MASQ`, `NFT_CT`, `NF_CT_NETLINK` built in; every conntrack helper `=n`; `NF_CONNTRACK_TIMEOUT` not set (cached runtime config `~/.cache/ze/runtime-kernel/7.2.9-runtime-amd64-*/config`)
- [ ] `test/firewall/firewall-masquerade-flags.ci`, `test/firewall/firewall-masquerade-ports.ci`, `test/firewall/firewall-snat-addr-range.ci` - existing NAT functional coverage, config to ruleset only

**Behavior to preserve:**
- Every existing `snat`, `dnat`, `masquerade`, `exclude` term keeps its YANG, its parse and its lowering; their `.ci` tests stay byte-identical
- `lowerNAT`'s refusal of unknown flags stays: the new EIM flag is a named field, never a raw `Flags` bit a caller can set unseen
- VPP NAT44-ED masquerade, SNAT and DNAT keep working
- Global conntrack sysctls stay at kernel defaults; the NAPT44 timers apply only to flows the `ze_nat44` table marks

**Behavior to change:**
- None in existing surfaces. A new `nat44` config block, producer, kernel option and kernel patch are added

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: a `nat44 { translation <name> { ... } }` block in the YANG-modelled config, at boot and on every commit
- Packets: customer traffic from an inside prefix (often a PPP interface of an L2TP session) leaving by the outside interface; inbound packets to the external address

### Transformation Path
1. Config tree to the `nat44` plugin's `OnConfigure` / `OnConfigApply`, parsed into a typed translation (inside prefixes, outside interface or address range, port range, filtering, timers)
2. The plugin builds one `firewall.Table` named `ze_nat44` (family `ip`): a ct timeout policy object per protocol, a prerouting nat chain (inbound EIM lookup per filtering mode, hairpin), a postrouting nat chain (EIM source translation for inside sources, including hairpinned flows), an input filter chain dropping unsolicited TCP SYN to the external address's NAT port range
3. `firewall.RegisterTables("nat44", ...)` then `firewall.ApplyAll()`; the nft backend lowers the new model pieces (ct timeout object, ct timeout assignment, EIM mapping flag, inbound lookup) to netlink
4. In the kernel: Ze's netfilter patch reserves each external `(proto, X', x')` for one internal `(X, x)` while any conntrack of that internal endpoint lives, and answers the inbound lookup from that reservation
5. WAN address change (iface event bus) triggers the REQ-7 overlap check (Q3) and a reapply when the outside address is the masquerade source

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to plugin | YANG section delivered through the 5-stage protocol, `OnConfigure` | No |
| Plugin to firewall component | in-process `firewall.RegisterTables` / `ApplyAll`, same as copp | No |
| Firewall backend to kernel | nftables netlink through the vendored `google/nftables`, plus a diff for the EIM attribute (Q1) | No |
| iface events to plugin | event bus subscription for address add and delete on the outside interface | No |

### Integration Points
- `firewall.Table`, `firewall.RegisterTables`, `firewall.ApplyAll` - the producer contract
- `lowerMasquerade` / `lowerNAT` in `internal/plugins/firewall/nft/lower_linux.go` - gain the named EIM mapping field
- `show system conntrack` - its timeout rows gain the NAT policy values so an operator sees what applies
- `ze doctor` - a check that the running kernel supports the EIM attribute and ct timeout objects

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes (design) | the plugin produces a `firewall.Table`; only the nft backend talks netlink |
| No unintended coupling (components stay isolated) | Yes (design) | same in-process producer contract copp uses; no import of the operator firewall engine |
| No duplicated functionality (extends existing, does not recreate) | Yes (design) | reuses the masquerade/SNAT lowering, adds one named field and one object type |
| Zero-copy preserved where applicable (refs, not copies) | N-A | control-plane config path, no per-packet Go code |
| Registration over hardcoding, outbound | Yes (design) | the plugin registers via `init()` in `register.go`; generated `internal/component/plugin/all/all.go` |
| Registration over hardcoding, inbound | Open | the lists to search at implementation: the firewall model `Action` switch in `lower_linux.go` (the arm `case firewall.Masquerade`) learns the new object and action types, which is the backend's job, not a central enumeration of features; the table-owner table in `table-ownership-and-shutdown-flush.md` is prose and gains a row |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Linux NAT reuses an existing source mapping for a new destination unless the tuple collides, so the EIM patch only has to add a reservation and an inbound lookup, not replace port selection | `get_unique_tuple` step 1 calls `find_appropriate_src` before `find_best_ips_proto` (`nf_nat_core.c`, 6.12 stable) | patch grows into a port allocator | Phase 3 reading of the Ze kernel version's `nf_nat_core.c`, then `TestNAT44EndpointIndependentMapping` | unvalidated |
| A-2 | `CONFIG_NF_CONNTRACK_TIMEOUT=y` plus the vendored `expr.CtTimeout` let a nat-table rule bind a timeout policy to NAT flows only | `ObjTypeCtTimeout` in `vendor/github.com/google/nftables/obj.go`, `CtTimeout` in `expr/ct.go` | timers fall back to global sysctls through the sysctl plugin's default layer, affecting all conntrack | `TestNAT44CtTimeoutPolicyApplied` reads the timeout of a live ct entry | unvalidated |
| A-3 | Conntrack handles TCP simultaneous open (`SYN_SENT2`) once the inbound SYN is permitted by filtering | Linux `nf_conntrack_proto_tcp.c` state table | REQ-2a gap | `TestNAT44TCPSimultaneousOpen` | unvalidated |
| A-4 | An unsolicited SYN to the external address on an unheld port is answered by a RST from the local stack today | standard Linux TCP behaviour | the input drop rule is unnecessary but harmless | `TestNAT44UnsolicitedSYNSilent` with the rule removed goes red | unvalidated |
| A-5 | ICMP errors are RELATED in conntrack and never delete a ct entry | `nf_conntrack_proto_icmp.c` | REQ-12 / 5382 REQ-10 gap | `TestNAT44ICMPDoesNotTerminateMapping` | unvalidated |
| A-6 | Linux forwarding sends ICMP Fragmentation Needed for DF=1 oversize and fragments DF=0; `nf_defrag_ipv4` reassembles out-of-order fragments within `ipfrag_high_thresh` | `net/ipv4/ip_forward.c`, `nf_defrag_ipv4` | REQ-13 / REQ-14 gap | `TestNAT44FragmentationBehaviour` | unvalidated |
| A-7 | (Q4, VPP in scope only) NAT44-EI and NAT44-ED cannot run at once in one VPP | VPP plugin layout | VPP backend must choose one per config | VPP docs and a probe | unvalidated |
| A-8 | stuntman (`stunclient --mode behavior`) reports mapping and filtering classes per RFC 5780 against a two-address `stunserver` | stuntman documentation | interop needs another RFC 5780 client | first lab run | unvalidated |
| A-9 | The Docker host for labs runs Ze's runtime kernel, so it carries the patch | `plan/release/delivery/spec-docker-hosts-run-the-ze-kernel.md` | the Docker lab cannot pass; the QEMU path remains the proof | lab kernel check (`docs/architecture/testing/interop.md`, "The Docker host kernel check") | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The kernel patch fails to rebase at a kernel bump | the kernel build step fails on the patch | the patch touches `nf_nat_core.c` and adds one file; its own QEMU test runs on every bump |
| R-2 | Port reservation exhausts the pool: one internal endpoint pins an external port for every live flow | `ze_nat44_mapping_failures_total` rises | REQ-11 forbids falling back; the counter and a doctor warning name the pool size, and an address range widens it |
| R-3 | The unsolicited-SYN drop hides a local service on the external address | operator's SSH or BGP on the WAN address stops answering | the drop covers only the NAT port range and excludes ports of configured local listeners; a `.ci` asserts SSH still answers |
| R-4 | Longer NAT timers grow the conntrack table | `show system conntrack` count near `nf_conntrack_max` | doctor check compares count to max; timers stay configurable above the RFC floors |
| R-5 | EIF exposes customer endpoints to any external host while a mapping lives | security review | `filtering address-and-port-dependent` available; Q2 sets the default |
| R-6 | Hairpinned flow is SNATed twice or not at all when the operator's own `firewall` NAT table also matches | ruleset shows two nat chains at the same hook | `ze_nat44` chains run at a fixed priority and the guide states the ordering; a coexistence `.ci` covers both tables |
| R-7 | WAN address changes under live mappings | mappings to the old address stay in conntrack | masquerade semantics already flush on address loss (`nf_nat_masquerade.c` device and inet notifiers); the integration test changes the address |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Customer traffic through the new `nat44` block; existing firewall NAT is untouched. A faulty kernel patch can break all NAT on the box, including `masquerade` terms |
| How is it reverted? | Remove the `nat44` block (the producer registers nil and `ApplyAll` deletes `ze_nat44`); a kernel patch defect needs a kernel rebuild without the patch |
| Who else touches this path? | the firewall engine, copp, policy-routes, ddos-local, firewall-irr, flowspec bridge, anomaly-shape (shared `ApplyAll`); `plan/proposals/spec-cgnat.md` (VPP NAT44-ED, unapproved) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `nat44` config block at boot | → | nat44 plugin `OnConfigure`, `RegisterTables("nat44")`, nft lowering | `test/firewall/nat44-apply.ci` |
| `nat44` block removed on reload | → | withdraw, `ApplyAll` deletes `ze_nat44` | `test/firewall/nat44-withdraw.ci` |
| config with a timer below the RFC floor | → | YANG range refusal | `test/firewall/nat44-timer-below-floor.ci` |
| customer UDP and TCP through Ze's kernel | → | `ze_nat44` table plus the EIM patch | `TestNAT44EndpointIndependentMapping` (`internal/plugins/nat44/nat44_integration_linux_test.go`) |
| third-party STUN client behind Ze | → | the whole stack | interop scenario `nat44-behavior-stuntman` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `nat44 translation cust { inside-prefix 100.64.0.0/10; outside interface eth1; }` committed | `show firewall ruleset` lists table `ze_nat44` with its prerouting, postrouting and input chains and its ct timeout policies; no operator `firewall` table changes |
| AC-2 | Inside host X:x sends UDP to Y1:y1 then Y2:y2 (two outside hosts) | Both observe the same source X':x' (RFC 4787 REQ-1) |
| AC-3 | Same as AC-2 over TCP | Both observe the same source X':x' (RFC 5382 REQ-1) |
| AC-4 | Inside host B:x (same port as A) sends to Y1:y1 while A's mapping to Y2 lives | B gets an external port different from A's x'; A's next flow to Y1 still uses x' (EIM held, REQ-3 no overloading) |
| AC-5 | Inside host X1 sends to X2':x2' (X2's live mapping) over UDP and TCP | X2 receives the packet with source X1':x1', the external pair (RFC 4787 REQ-9, 9a; RFC 5382 REQ-8) |
| AC-6 | A UDP mapping receives one outbound packet and then nothing | The ct entry's remaining timeout is at least 120 s after the packet, and the default is 300 s (REQ-5, 5c) |
| AC-7 | TCP connections in ESTABLISHED, FIN_WAIT, CLOSE_WAIT | Their ct timeouts are at least 7440 s; in SYN_SENT, SYN_RECV, LAST_ACK at least 240 s (RFC 5382 REQ-5) |
| AC-8 | Config sets `timeout udp 60` or `timeout tcp-established 3600` | Commit refused; the error names the leaf, the RFC floor and the requirement, and says which value is accepted |
| AC-9 | Outside host sends an unsolicited SYN to X':p for p in the NAT port range and not a local listener | No packet is sent back within 6 s; if an outbound SYN for that connection is translated in that window, the original SYN is dropped and the connection completes (RFC 5382 REQ-4, REQ-2a) |
| AC-10 | Ze's own SSH listens on the external address | An SSH connection to it still succeeds with `nat44` active (R-3) |
| AC-11 | `filtering endpoint-independent`: an outside host Y3 never contacted sends to X':x' | X:x receives it. With `filtering address-and-port-dependent` it is dropped |
| AC-12 | An ICMP Destination Unreachable for a live UDP mapping and for a live TCP connection arrives | Both mappings remain and the next packet uses the same X':x' (RFC 4787 REQ-12, RFC 5382 REQ-10) |
| AC-13 | Inside host sends DF=1 larger than the outside MTU; then DF=0 | DF=1: the host receives ICMP Fragmentation Needed; DF=0: the outside host receives fragments that reassemble (REQ-13, 13a) |
| AC-14 | Outside host sends a fragmented UDP reply out of order | The inside host receives the whole datagram (REQ-14) |
| AC-15 | Port pool exhausted by reservations | A new mapping fails, `ze_nat44_mapping_failures_total` increments, and no existing mapping changes behaviour (REQ-11) |
| AC-16 | WAN address learned by DHCP or PPPoE overlaps an inside prefix | Behaviour per Q3, reported by `ze doctor` and a log line naming the prefix, the WAN address and the fix |
| AC-17 | Running kernel lacks the EIM attribute or ct timeout support | Commit of a `nat44` block is refused with an error naming the missing kernel feature, and `ze doctor` reports it; nothing is installed half-way |
| AC-18 | `nat44` block on the `vpp` backend | Per Q4: either NAT44-EI lowering meeting AC-2..AC-15, or a refusal naming the backend and the `nft` backend as the fix |
| AC-19 | `nat44` block removed | `ze_nat44` disappears from the kernel; existing `masquerade` terms are unaffected |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator configures NAT for subscriber pool | config → nat44 plugin → `ze_nat44` → nft backend → kernel | `test/firewall/nat44-apply.ci` |
| 2 | Subscriber's STUN client classifies the NAT | inside client → Ze kernel NAT → two-address STUN server | `nat44-behavior-stuntman` |
| 3 | Two subscribers reach each other by public address (hairpin) | X1 → prerouting lookup → postrouting SNAT → X2 | `TestNAT44Hairpin` |
| 4 | Operator inspects NAT timers | `show system conntrack` | `test/firewall/nat44-show-conntrack.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNAT44ParseConfig` | `internal/plugins/nat44/config_test.go` | prefixes, outside choice, port range, filtering, timers parse; mutual exclusion of outside interface and address range | |
| `TestNAT44BuildTable` | `internal/plugins/nat44/translate_test.go` | the produced `firewall.Table`: name `ze_nat44`, chains, hooks, priorities, ct timeout objects, SYN drop excluding local listener ports | |
| `TestLowerCtTimeoutObject` | `internal/plugins/firewall/nft/lower_linux_test.go` | timeout policy lowers to `ObjTypeCtTimeout` with the per-state values | |
| `TestLowerMasqueradeEndpointIndependent` | `internal/plugins/firewall/nft/lower_linux_test.go` | the named EIM field lowers to the patch's attribute; a raw `Flags` bit is still refused | |
| `TestNAT44OverlapCheck` | `internal/plugins/nat44/overlap_test.go` | REQ-7 check fires on overlap, stays silent otherwise | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `timeout udp` (s) | 120-86400 | 120 / 86400 | 119 | 86401 |
| `timeout tcp-established` (s) | 7440-2592000 | 7440 / 2592000 | 7439 | 2592001 |
| `timeout tcp-transitory` (s) | 240-86400 | 240 / 86400 | 239 | 86401 |
| `timeout icmp` (s) | 30-3600 | 30 / 3600 | 29 | 3601 |
| `port-range` | 1024-65535 | 1024-65535 | 1023 | 65536 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `nat44-apply` | `test/firewall/nat44-apply.ci` | config applies, ruleset shows `ze_nat44` (`option=needs-linux`) | |
| `nat44-withdraw` | `test/firewall/nat44-withdraw.ci` | removing the block removes the table, masquerade terms untouched | |
| `nat44-timer-below-floor` | `test/firewall/nat44-timer-below-floor.ci` | commit refused with the RFC-naming error | |
| `nat44-show-conntrack` | `test/firewall/nat44-show-conntrack.ci` | `show system conntrack` shows the NAT policy timers | |
| `nat44-kernel-missing` | `test/firewall/nat44-kernel-missing.ci` | refusal when the kernel lacks the feature (AC-17) | |

Integration (QEMU, `integration && linux`, Ze runtime kernel, added to `integrationPackages`): `internal/plugins/nat44/nat44_integration_linux_test.go` with network namespaces for two inside hosts, the Ze NAT, and three outside hosts: `TestNAT44EndpointIndependentMapping` (AC-2, AC-3, AC-4), `TestNAT44Hairpin` (AC-5), `TestNAT44CtTimeoutPolicyApplied` (AC-6, AC-7), `TestNAT44UnsolicitedSYNSilent` (AC-9, AC-10), `TestNAT44TCPSimultaneousOpen` (AC-9), `TestNAT44Filtering` (AC-11), `TestNAT44ICMPDoesNotTerminateMapping` (AC-12), `TestNAT44FragmentationBehaviour` (AC-13, AC-14), `TestNAT44PoolExhaustion` (AC-15), `TestNAT44WANAddressChange` (R-7). Each is discriminated by reverting its producer (the patch flag, the ct timeout binding, the hairpin rule, the SYN drop) and recording red.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `nat44-behavior-stuntman` | `test/interop-nat/scenarios/` | stuntman `stunclient` inside, `stunserver` with two outside addresses (latest stable) | the third-party RFC 5780 client classifies Ze as Endpoint-Independent Mapping and, per the configured mode, Endpoint-Independent or Address-and-Port-Dependent filtering | |
| `nat44-tcp-mapping-stuntman` | `test/interop-nat/scenarios/` | stuntman over TCP, fixed local port, both server addresses | the two TCP bindings report the same mapped address and port | |

The new lab lives in `internal/le/interoplab/nat/` with `register_catalog.go`, a
native Docker action and a QEMU action, and a row in
`docs/architecture/testing/qemu-integration.md`. Its scenarios are proven
discriminating by building the kernel without the EIM patch flag and recording
the red classification.

## Files to Modify
- `internal/component/firewall/model.go` - ct timeout policy object on `Table`, an action binding it, a named endpoint-independent field on `Masquerade` and `SNAT`
- `internal/plugins/firewall/nft/lower_linux.go` - lower the new object, action and field; keep `lowerNAT`'s raw-flag refusal
- `internal/plugins/firewall/vpp/translate.go` - refuse or lower per Q4
- `internal/component/firewall/cmd_show_conntrack.go` - show the NAT policy timers
- `gokrazy/kernel/kernel.config`, `gokrazy/kernel/runtime.config` - `CONFIG_NF_CONNTRACK_TIMEOUT=y`
- `gokrazy/kernel/patches/series` - the EIM patch (Q1)
- vendored `google/nftables` - a diff against the release for the EIM attribute, applied at build time per the third-party module rule
- `internal/le/test/qemu/alltests.go` - add the nat44 integration package
- `docs/guide/firewall.md`, `docs/architecture/firewall/table-ownership-and-shutdown-flush.md`, `docs/architecture/testing/interop.md`, `docs/architecture/testing/qemu-integration.md` - see the documentation checklist

## Files to Create
- `internal/plugins/nat44/register.go`, `nat44.go`, `config.go`, `translate.go`, `overlap.go`, `doctor.go`, `metrics.go` - the producer plugin
- `internal/plugins/nat44/yang/ze-nat44-conf.yang` - the `nat44` container
- `internal/plugins/nat44/nat44_integration_linux_test.go` - QEMU proof
- `gokrazy/kernel/patches/0003-netfilter-nat-endpoint-independent-mapping.patch` - Q1
- `test/firewall/nat44-*.ci` - the five functional tests above
- `internal/le/interoplab/nat/` and `test/interop-nat/scenarios/nat44-behavior-stuntman/`, `nat44-tcp-mapping-stuntman/`, `test/interop-nat/Dockerfile.stuntman`
- `docs/guide/nat44.md`, `features/nat44.md`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/plugins/nat44/yang/ze-nat44-conf.yang` |
| YANG validation constraints | Yes | `range` floors on every timer (the RFC MUST becomes the schema), `ze-types` prefix and interface types, port-range pattern |
| YANG custom validators | Yes | overlap of inside prefixes between translations; outside interface existence through `CompleteFn` |
| CLI commands/flags | No | no new command; `show system conntrack` gains rows |
| CLI grammar (keyword before value) | Yes | every leaf is keyword-then-value (`inside-prefix 100.64.0.0/10`) per `ai/rules/cli.md` |
| Editor autocomplete | Yes | enum `filtering` automatic; outside interface via `CompleteFn` |
| Functional test for new RPC/API | N-A | no new RPC |
| Pipe completeness | N-A | no new command output |
| Env var registration | N-A | no `environment/` leaf |
| Doctor check for runtime dependencies | Yes | kernel EIM attribute and `NF_CONNTRACK_TIMEOUT` support, conntrack count against max, WAN overlap; codes in `internal/core/diagnostic/codes.go` |
| Prometheus counters/metrics | Yes | `ze_nat44_mapping_failures_total{translation}`, `ze_nat44_wan_overlap{translation}` |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `features/nat44.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` (pointer), `docs/guide/nat44.md` |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (`show system conntrack` rows) |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/nat44.md`, cross-link from `docs/guide/firewall.md` |
| 7 | Wire format changed? | No | no protocol wire format; the nftables netlink attribute is documented in the patch |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented? | Yes | no `rfc/short/` summary (enrolment frozen); the `rfc/profiles/fttx-edge.md` Absent row is the owner's to change once enrolment reopens; `docs/guide/nat44.md` lists each REQ with its test |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` (new suite row), `docs/architecture/testing/qemu-integration.md` (lab row) |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` NAT row |
| 12 | Internal architecture changed? | Yes | `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` (eighth producer) |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | Yes | `docs/guide/nat44.md` metrics section |
| 15 | Registered plugin or inventory changed? | Yes | `docs/plugin-overview.md`, `docs/features/plugins.md` |
| 16 | Changed source files referenced by doc anchors? | Yes | derived at implementation with `./le spec citation anchors spec plan/post-release/features/spec-nat44-customer-napt.md` |
| 17 | Existing docs show examples for this area? | Yes | `docs/guide/firewall.md` masquerade examples stay valid; add a pointer to `nat44` for customer NAT |

## Implementation Steps

1. **Phase: Wiring** - plugin skeleton, YANG, `RegisterTables("nat44")` with an empty table; `nat44-apply.ci` and `nat44-withdraw.ci` fail on missing chains
2. **Phase: Timers** - kernel config `NF_CONNTRACK_TIMEOUT`, ct timeout object and binding in model and nft lowering, YANG floors; AC-6, AC-7, AC-8
3. **Phase: Kernel EIM patch** - reservation keyed on the external tuple, inbound lookup, uapi attribute; patch QEMU test against a bare nft ruleset first
4. **Phase: Mapping, filtering, hairpin** - lowering of the EIM field and lookup, postrouting hairpin SNAT; AC-2..AC-5, AC-11, AC-15
5. **Phase: Unsolicited SYN, ICMP, fragments** - input drop with listener exclusion; AC-9, AC-10, AC-12..AC-14
6. **Phase: WAN overlap and doctor** - Q3 behaviour, doctor checks, metrics; AC-16, AC-17
7. **Phase: VPP** - per Q4; AC-18
8. **Phase: Interop lab** - `internal/le/interoplab/nat/`, Docker and QEMU actions, both scenarios, discrimination records
9. **Phase: Docs** - each page edited in the phase that changes it; this step only verifies

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every MUST of RFC 4787 Section 12 and RFC 5382 Section 8 maps to an AC and a test |
| Correctness | hairpinned packets carry the external source pair, never the internal one |
| Correctness | no fallback to APDM under exhaustion (REQ-11) |
| Naming | YANG leaves kebab-case; metric names `ze_nat44_*` |
| Data flow | the plugin never edits operator `firewall` tables; only the nft backend speaks netlink |
| Rule: rfc-compliance | each enforcing rule-builder carries the byte-exact quote |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `ze_nat44` table installed | `nat44-apply.ci` |
| EIM and hairpin on Ze's kernel | QEMU integration package green, reds recorded |
| third-party classification | `nat44-behavior-stuntman` green with recorded run |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | prefixes, port range, timers bounded by YANG |
| Resource exhaustion | reservation per internal endpoint bounded by the port range; conntrack count doctor check |
| Fail open | a refused or half-applied table never leaves inside traffic forwarded untranslated to the outside; the postrouting chain is installed before forwarding is permitted |
| Exposure | EIF default documented with its risk (R-5) |

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

- The firewall primitives are not the gap; behaviour is. A NAT that compiles every `masquerade` correctly still fails REQ-1 because the mapping key lives in the kernel
- RFC 5382 Section 5 puts FIN_WAIT and CLOSE_WAIT in the established phase, so the 2 h 4 min floor applies to them, not the 4 min one

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A dedicated `nat44` producer with its own `ze_nat44` table | New leaves on the firewall `masquerade` action | timers, the SYN drop, hairpin and the overlap check span several chains and hooks; an operator term cannot express them, and existing terms keep their meaning |
| EIM in Ze's kernel (Q1) | Userspace pinning from ctnetlink events into nft maps | events arrive after the kernel picked the port, so a fast second flow gets another port and another internal endpoint can take the port meanwhile: not deterministic (REQ-11) |
| Timers as an nft ct timeout policy bound to NAT flows | Global `nf_conntrack_*` sysctls through the sysctl default layer | global timers change every flow the box tracks, and an operator `sysctl` override would silently break conformance; a scoped policy is checkable by doctor |
| Timer floors as YANG `range` minimums | Runtime validator | the RFC floor becomes the schema; no value below it can be committed |
| Silent drop of unsolicited SYN | ICMP Port Unreachable after 6 s | dropping satisfies both MUSTs of RFC 5382 REQ-4; the ICMP is a SHOULD with a timer state machine per SYN |

## Known Limitations
- Address-Dependent Filtering (RFC 4787 REQ-8 RECOMMENDED option) is not offered; it is SHOULD-level work for a later spec, named when this one closes
- Port parity (REQ-4 RECOMMENDED) and paired pooling (REQ-2 RECOMMENDED) are reported as optional figures, not guaranteed
- CGNAT port-block allocation and NAT logging stay with `plan/proposals/spec-cgnat.md`
- IPv6 is out of scope: NAPT44 only

## RFC Documentation (Scope: protocol)

Add `// RFC 4787 Section 12: "<quoted requirement>"` and `// RFC 5382 Section 8:
"<quoted requirement>"` above each rule-builder and lowering statement that
enforces a requirement, byte-exact.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced (not applicable: enrolment frozen by the owner)
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-19 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches, and `./le verify changed` over the closing commits (`/ze-close` step 6f). The full verification is Thomas's release check: no agent runs it, and one that is owed is handed to him as its exact command. A red there gets a fix commit
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
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
