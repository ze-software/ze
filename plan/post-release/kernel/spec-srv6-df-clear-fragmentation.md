# Spec: srv6-df-clear-fragmentation

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P3 |
| Release reason | Kernel work: DF-clear IPv4 fragmentation across Linux seg6 `lwtunnel_output` requires a Linux kernel datapath change, which Thomas placed after the first release |
| RFC gaps | - |
| Depends | - (inner TTL decrement and DF-set PMTU ICMP landed in `spec-srv6-headend-decrements-inner-ttl`) |
| Phase | 4/4 |
| Handoff | - |
| Updated | 2026-11-22 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Handle forwarded DF-clear IPv4 packets whose length exceeds the effective tunnel
MTU (egress link or route MTU minus the seg6 lwtunnel headroom: 64 bytes for a
single-SID SRH on a 1500-byte link, effective MTU 1436) on Ze's Linux runtime
kernel.

Measured during `spec-srv6-headend-decrements-inner-ttl` (A-8) on patched Linux
7.3.0-ze in QEMU (`TestFIBServiceSIDOversizedDFAnswersFragmentationNeeded`): a
1460-byte DF-clear IPv4 packet crossing a 1500-byte egress link with 64-byte
SRv6 headroom produced `unfragmented-srv6=0 ipv6-outer-fragments=0
icmp-errors=0`. `ip_forward` skips `ip_exceeds_mtu` when DF is clear, and
`dst->output` points to `lwtunnel_output` (`seg6_output_core`) instead of
`ip_output` / `ip_finish_output`, so the inner IPv4 packet is never fragmented
before encapsulation. `seg6_output_core` encapsulates the 1460-byte packet into
a 1524-byte outer IPv6+SRH packet with `skb->ignore_df == 0`, and `ip6_fragment`
(`net/ipv6/ip6_output.c`) rejects it at `fail_toobig`, dropping the packet and
sending ICMPv6 Packet Too Big to the headend's own outer IPv6 source address
rather than forwarding fragments or signalling the IPv4 sender.

Goal: on Ze's runtime kernel, a forwarded DF-clear IPv4 packet larger than the
seg6 route's effective MTU (MTU minus seg6 headroom) is fragmented at the inner
IPv4 layer before SRv6 encapsulation into valid IPv4 fragments, each encapsulated
in its own unfragmented outer IPv6+SRH packet within the egress MTU so Linux
`seg6_local` (`End.DT4` / `End.DX4`), VPP, and hardware SRv6 egress routers
decapsulate and deliver the complete payload. A behavioural `kernelcap` probe
(extending the shared SRv6 throwaway-namespace TUN probe) reports stock kernels
without this patch in `ze doctor` as a warning and never refuses startup.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/fib/fib-depth-4-srv6.md` - "Kernel backend": how Ze programs `SEG6Encap` routes and documents patch 0014 (`CONFIG_IPV6_SEG6_FORWARD_TTL`) and the SRv6 doctor capabilities
  → Decision: Ze's netlink route encoding (`buildSEG6Encap`) stays unchanged; this spec adds patch `0015-seg6-forward-df-clear-frag.patch` (`CONFIG_IPV6_SEG6_FORWARD_FRAG`) and extends the shared SRv6 behavioural probe and capability enrolments.
  → Constraint: every factual paragraph added to this page carries a `<!-- source: -->` anchor and updates in the same phase as the code it describes.
- [ ] `docs/architecture/doctor-and-health-checks.md` - "Kernel capabilities: one enrolment, three callers": how compiled-in boolean SRv6 capabilities enroll, memoize definitive verdicts, and reach `ze doctor`
  → Decision: enroll `srv6-forward-frag` (`Degrades`, order 742) alongside `srv6-encap` and `srv6-forward-ttl` using the existing `SRv6InUse` predicate and shared throwaway-namespace TUN probe.
  → Constraint: definitive (`present`/`absent`) verdicts memoize for process life while `unknown`/`denied` are never cached; no new sysctl or AppArmor rule is needed because the probe reuses the same TUN device and sysctls.
- [ ] `docs/architecture/linux-limitations.md` - `LL-SEG6-HEADEND-NO-TTL-DECREMENT` and `LL-MPLS-FORWARD-NO-PMTU`: entry schema and `TestLinuxLimitsCheckedAtShippedVersions` pin
  → Constraint: record stock Linux 7.3's DF-clear seg6 drop as verified entry `LL-SEG6-HEADEND-NO-DF-CLEAR-FRAG` with Kind `kernel`, Effect `constrains design`, Status `verified`, Checked `Linux 7.3, 2026-11-22`, quoted kernel source evidence, Alternatives (including why outer IPv6 fragmentation fails Linux `seg6_local`), Re-check, and Ze meanwhile.
- [ ] `docs/architecture/mpls/mpls-kernel.md` - "Path MTU and label overhead": how Ze's kernel MPLS MTU patch handles DF-clear IPv4 fragmentation, IPv4 options, and sub-quantum MTU guards (`mtu < hlen + 8`)
  → Decision: mirror the proven IPv4 pre-encapsulation fragmentation contract (preserve IPv4 options on first fragment, strip non-copied options on later fragments, guard sub-quantum MTU without livelock) by reusing the kernel's exported `ip_do_fragment`.
- [ ] `docs/architecture/appliance/kernel-profiles.md` - "Go owns the guarantee" and "Runtime requirements": patch series rules (`patch --fuzz=0`), `runtime.config`, and `runtime.require`
  → Constraint: `0015-seg6-forward-df-clear-frag.patch` applies with `--fuzz=0` after `0014-seg6-forward-ttl.patch` on Linux v7.3, and `CONFIG_IPV6_SEG6_FORWARD_FRAG` is set in `gokrazy/kernel/runtime.config` and pinned in `gokrazy/kernel/runtime.require`.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/full/rfc8986.txt` Section 5.1 (H.Encaps) and Section 5.2 (H.Encaps.Red)
  → Constraint: H.Encaps S05 decrements inner IPv4 TTL before encapsulation and submits the encapsulated packet to IPv6 transmission; RFC 8986 is in no release profile (`ai/META-DECISIONS.md` forbids enrolling a summary for an RFC no profile lists).
- [ ] `rfc/short/rfc2473.md` and `rfc/full/rfc2473.txt` Sections 7 and 7.2
  → Decision: RFC 2473 Section 7.2(b) describes outer IPv6 fragmentation for generic IPv6 tunnels while Section 7 warns that outer IPv6 fragments break reassembly at anycast exit points; furthermore Linux's own `seg6_local` (`net/ipv6/seg6_local.c` `seg6_local_build_state` sets `LWTUNNEL_STATE_INPUT_REDIRECT` and `decap_and_validate` runs before `ipv6_frag_rcv`) drops outer IPv6 fragments unconditionally. Pre-encapsulation inner IPv4 fragmentation produces unfragmented IPv6+SRH packets that interoperate with Linux `seg6_local`, VPP, hardware SRv6 egress routers, and anycast SIDs.

**Key insights:**
- Root cause verified in Linux v7.3 source (`net/ipv4/ip_forward.c`, `net/ipv4/ip_output.c`, `net/core/lwtunnel.c`, `net/ipv6/seg6_iptunnel.c`, `net/ipv6/ip6_output.c`): `ip_forward` skips `ip_exceeds_mtu` when `IP_DF` is clear, then calls `dst_output`, which `lwtunnel_set_redirect` replaced with `lwtunnel_output` -> `seg6_output_core` (skipping `ip_output` / `ip_finish_output` / `ip_fragment`). `seg6_output_core` encapsulates the oversized IPv4 packet into an outer IPv6+SRH skb with `skb->ignore_df == 0`, and `ip6_fragment` drops it at `fail_toobig` while sending ICMPv6 Packet Too Big to the headend's own IPv6 source address.
- Outer IPv6 fragmentation (`skb->ignore_df = 1`) fails against Linux's own SRv6 egress decapsulator (`seg6_local` `End.DT4`/`End.DX4`): `seg6_local_build_state` sets `LWTUNNEL_STATE_INPUT_REDIRECT`, so `decap_and_validate` runs before `ip6_local_deliver` / `ipv6_frag_rcv` and drops fragmented outer IPv6 packets.
- `ip_do_fragment` (`net/ipv4/ip_output.c`) is `EXPORT_SYMBOL` and already computes `ip_skb_dst_mtu(sk, skb)` (which subtracts `lwtunnel_headroom` via `ip_dst_mtu_maybe_forward`), checks `mtu < hlen + 8` (preventing livelock), handles `CHECKSUM_PARTIAL`, `IPCB(skb)->frag_max_size`, `ip_options_fragment`, `frag_list`, and `IPSTATS_MIB_FRAG*`, and invokes a caller-supplied `output(net, sk, frag)` callback on every fragment while keeping `orig_dst` (`lwtstate`) attached to each fragment.
- In the shared throwaway-namespace TUN probe (`internal/component/kernelcap/probe_srv6_linux.go`), writing a 1460-byte DF-clear probe packet followed immediately by a small 44-byte sentinel packet on the same TUN fd distinguishes present (two inner IPv4 fragments received) from absent (sentinel received with zero fragments) in under 1 ms with zero timeout penalty on stock kernels and zero new AppArmor rules.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `gokrazy/kernel/patches/0014-seg6-forward-ttl.patch` - sets `LWTUNNEL_STATE_OUTPUT_REDIRECT` only on L3 seg6 modes (`encap`, `encap.red`, `inline`) and defines `CONFIG_IPV6_SEG6_FORWARD_TTL`
- [ ] `gokrazy/kernel/patches/0002-mpls-ip-mtu.patch` - precedent for kernel IPv4 fragmentation on a Ze runtime kernel patch and Kconfig marker pinning
- [ ] `gokrazy/kernel/patches/series`, `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require` - patch ordering (`0001`..`0014`) and `CONFIG_IPV6_SEG6_*` pins
- [ ] `Linux v7.3 net/ipv6/seg6_iptunnel.c` (`seg6_lwt_headroom`, `__seg6_do_srh_encap`, `seg6_do_srh`, `seg6_output_core`, `seg6_output`, `seg6_build_state`) - encapsulates `skb` directly in `seg6_output_core` without checking IPv4 MTU or calling `ip_do_fragment`
- [ ] `Linux v7.3 net/ipv4/ip_forward.c` (`ip_exceeds_mtu`, `ip_forward`, `ip_forward_finish`) - skips MTU rejection when `IP_DF` is clear and calls `dst_output`
- [ ] `Linux v7.3 net/ipv4/ip_output.c` (`ip_finish_output_gso`, `__ip_finish_output`, `ip_fragment`, `ip_do_fragment`, `ip_frag_init`, `ip_frag_next`, `ip_fraglist_prepare`, `ip_copy_metadata`) - exported `ip_do_fragment` computes `ip_skb_dst_mtu`, guards `mtu < hlen + 8`, handles options/checksums/stats, and passes each fragment to `output`
- [ ] `Linux v7.3 net/ipv6/ip6_output.c` (`__ip6_finish_output`, `ip6_fragment`) - rejects oversized outer IPv6 packet when `!skb->ignore_df`, sending ICMPv6 PTB to outer `saddr` and dropping the skb
- [ ] `Linux v7.3 net/ipv6/seg6_local.c` (`seg6_local_build_state`, `decap_and_validate`, `input_action_end_dt4`) - hooks `LWTUNNEL_STATE_INPUT_REDIRECT`, bypassing IPv6 fragment reassembly
- [ ] `Linux v7.3 net/core/lwtunnel.c` & `include/net/lwtunnel.h` (`lwtunnel_output`, `lwtunnel_headroom`, `lwtunnel_set_redirect`) - `lwtunnel_headroom` subtracts `lwtstate->headroom` for `OUTPUT_REDIRECT` routes
- [ ] `Linux v7.3 include/net/ip.h` (`ip_dst_mtu_maybe_forward`, `ip_skb_dst_mtu`) - both subtract `lwtunnel_headroom(dst->lwtstate, mtu)`
- [ ] `internal/component/kernelcap/probe_srv6_linux.go` & `probe_other.go` - shared memoized throwaway-namespace TUN probe answering `SRv6Encap()` and `SRv6ForwardTTL()`
- [ ] `internal/plugins/fib/kernel/kernelcap_linux.go`, `register_doctor_codes.go`, `kernelcap_codes_linux_test.go` - `srv6-encap` and `srv6-forward-ttl` enrolments and their four doctor codes
- [ ] `internal/plugins/fib/kernel/srv6_integration_linux_test.go`, `srv6probe_integration_linux_test.go` - existing SRv6 datapath and namespace probe integration tests including A-8 observation
- [ ] `test/ui/doctor-srv6-kernelcap.ci` - functional doctor test forcing `srv6-encap` and `srv6-forward-ttl`

**Behavior to preserve:**
- Ze's netlink route encoding (`buildSEG6Encap` and `buildRichRoute` in `internal/plugins/fib/kernel/nexthop_linux.go`) stays byte-identical.
- All patch 0014 behavior stays intact: forwarded IPv4/IPv6 inner TTL/Hop Limit decrements once, TTL/Hop Limit 1 returns ICMP/ICMPv6 Time Exceeded, oversized DF-set IPv4 returns ICMP Fragmentation Needed with next-hop MTU equal to link/route MTU minus seg6 headroom, oversized IPv6 returns ICMPv6 Packet Too Big, L2 modes keep `LWTUNNEL_STATE_INPUT_REDIRECT`, and locally originated packets keep their existing path.
- Fast path for non-GSO IPv4 packets whose length fits within `ip_skb_dst_mtu(sk, skb)` (with `IPCB(skb)->frag_max_size == 0`) incurs no fragmentation allocation or copy.
- `ze-kernel-probe` AppArmor profile and Docker `--device /dev/net/tun` setup stay unchanged (no new sysctls or devices).
- `srv6-encap`, `srv6-forward-ttl`, and the new `srv6-forward-frag` all degrade (warnings) and never refuse startup.

**Behavior to change:**
- On Ze's runtime kernel (`0015-seg6-forward-df-clear-frag.patch`, `CONFIG_IPV6_SEG6_FORWARD_FRAG=y`), `seg6_output_core` fragments oversized DF-clear IPv4 packets (and DF-clear IPv4 GSO skbs whose segment size exceeds effective MTU, and DF-set skbs with `ignore_df` when permitted) at the inner IPv4 layer via `ip_do_fragment` before calling `seg6_output_encap`, producing unfragmented outer IPv6+SRH packets each within the egress MTU.
- The shared SRv6 behavioural probe (`internal/component/kernelcap/probe_srv6_linux.go`) also probes DF-clear fragmentation in the same throwaway namespace and exposes `kernelcap.SRv6ForwardFrag()`, enrolled by `fib-kernel` as degrading capability `srv6-forward-frag` with doctor codes `doctor-srv6-forward-frag-unavailable` and `doctor-srv6-forward-frag-unknown`.
- `docs/architecture/linux-limitations.md` records `LL-SEG6-HEADEND-NO-DF-CLEAR-FRAG`, and the SRv6 architecture, doctor, and feature pages document patch 0015 and the new capability.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Kernel patch series and runtime require manifest | `gokrazy/kernel/patches/series`, `gokrazy/kernel/patches/0014-seg6-forward-ttl.patch`, `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require`, `internal/appliance/kernelreq_enrolled_linux_test.go` | Applies patches `0001`..`0014` on Linux v7.3 (`patch --fuzz=0`), pins required `=y` Kconfig symbols before compile, and checks that every enrolled `kernelcap` symbol is pinned | extends: appends `0015-seg6-forward-df-clear-frag.patch` and pins its marker `CONFIG_IPV6_SEG6_FORWARD_FRAG` in `runtime.config` and `runtime.require` |
| Kernel IPv4 fragmentation engine | Linux v7.3 `net/ipv4/ip_output.c` (`ip_do_fragment`, `ip_skb_dst_mtu`) | Exported IPv4 fragmentation routine handling `CHECKSUM_PARTIAL`, `lwtunnel_headroom` subtraction, `mtu < hlen + 8` guard, `frag_max_size`, IPv4 options (`ip_options_fragment`), `frag_list`, and MIB stats, invoking a caller `output` callback per fragment | extends: `seg6_output_core` invokes `ip_do_fragment(net, sk, skb, seg6_output_encap)` instead of reimplementing IPv4 fragmentation |
| Shared SRv6 throwaway-namespace behavioural probe | `internal/component/kernelcap/probe_srv6_linux.go` (`SRv6Encap`, `SRv6ForwardTTL`, `runSRv6ProbeMemoized`, `askSRv6InThrowawayNamespace`) and `probe_other.go` | Runs one throwaway-namespace `/dev/net/tun` seg6 forwarding probe per process, memoizing definitive (`present`/`absent`) verdicts for compiled-in boolean SRv6 features | extends: sends one 1460-byte DF-clear packet plus one 44-byte sentinel packet on the same TUN fd and memoizes a third verdict `SRv6ForwardFrag()` in the same single run |
| Kernel capability enrolment and owning-package doctor codes | `internal/plugins/fib/kernel/kernelcap_linux.go` (`srv6EncapCapability`, `srv6ForwardTTLCapability`), `register_doctor_codes.go` (`doctorDiagnosticCodes`), `internal/component/kernelcap/predicate.go` (`SRv6InUse`) | Enrols degrading SRv6 capabilities once in `fib-kernel`'s `init()` and registers diagnostic codes via `diagnostic.Register` | extends: adds `srv6ForwardFragCapability` (`srv6-forward-frag`, order 742, `Degrades`) and its two diagnostic codes `doctor-srv6-forward-frag-unavailable` / `doctor-srv6-forward-frag-unknown` |
| Docker kernel-probe AppArmor profile and TUN grant | `internal/component/kernelcap/apparmor.go` (`ProbeAppArmorProfile`), `internal/le/interoplab/kernelcheck.go` (`kernelProbeContainer`) | Grants `/dev/net/tun` and the three IPv4/IPv6 forwarding sysctls to the Docker kernel check container | does not apply (reused unchanged): the extended probe uses the exact same TUN device and sysctls and discovers `srv6-forward-frag` automatically from `ze doctor --json kernel-capabilities` |
| SRv6 QEMU integration testbed | `internal/plugins/fib/kernel/srv6_integration_linux_test.go` (`setupSRv6Testbed`, `srv6ForwardTTLPatched`) and `srv6probe_integration_linux_test.go` | Drives `fibKernel.processEvent` over a veth testbed in QEMU with strict `-ze` assertions and probe-based stock-kernel branching | extends: adds `srv6ForwardFragPatched` and dedicated DF-clear fragmentation, IPv4 options, and sub-quantum MTU integration tests |
| Linux limitations register | `docs/architecture/linux-limitations.md` (`LL-SEG6-HEADEND-NO-TTL-DECREMENT`) | Records verified stock Linux 7.3 kernel behaviors checked by `TestLinuxLimitsCheckedAtShippedVersions` | extends: adds verified entry `LL-SEG6-HEADEND-NO-DF-CLEAR-FRAG` at Linux 7.3 |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | The core knows nothing, and everything registers | the door every change reads; the door covering `internal/component/kernelcap/probe_srv6_linux.go`, `probe_other.go`, `probe_srv6_linux_test.go` | "It never needs an edit to a switch, a factory, a field list, a help string or a completion table elsewhere (`ai/rules/principles.md`, `ai/patterns/registration.md`)." | agrees | `fib-kernel` registers `srv6-forward-frag` via `kernelcap.MustRegister` in `kernelcap_linux.go` and its two doctor codes via `diagnostic.Register` in `register_doctor_codes.go`; no central switch or enumeration is edited |
| `docs/architecture/doctor-and-health-checks.md` | Kernel capabilities: one enrolment, three callers | `// Design:` and anchors of `internal/component/kernelcap/probe_srv6_linux.go`, `probe_other.go`, `probe_srv6_linux_test.go`, `internal/plugins/fib/kernel/kernelcap_linux.go`, `register_doctor_codes.go`, `srv6probe_integration_linux_test.go`; page this spec modifies | "Probes for compiled-in boolean kernel features (`srv6-encap`, `srv6-forward-ttl`) memoize a definitive `present` or `absent` verdict for the process life (`runSRv6ProbeMemoized`), because only a reboot can change it. `unknown` and `denied` verdicts are never memoized." | agrees | `srv6-forward-frag` (`CONFIG_IPV6_SEG6_FORWARD_FRAG`) is a compiled-in boolean kernel feature sharing `runSRv6ProbeMemoized` and `SRv6InUse`, memoizing only definitive `present`/`absent` verdicts and degrading as a warning; this page is updated in Phase 1 to list `srv6-forward-frag` alongside `srv6-encap` and `srv6-forward-ttl` |
| `docs/architecture/fib/fib-depth-4-srv6.md` | Kernel backend | anchors of `probe_srv6_linux.go`, `kernelcap_linux.go`, `register_doctor_codes.go`; `// Design:` of `srv6_integration_linux_test.go`; page this spec modifies | "Ze's runtime kernel carries `gokrazy/kernel/patches/0014-seg6-forward-ttl.patch` (marker `CONFIG_IPV6_SEG6_FORWARD_TTL=y`), which sets only `LWTUNNEL_STATE_OUTPUT_REDIRECT` on L3 modes (`encap`, `encap.red`, `inline`) and keeps `LWTUNNEL_STATE_INPUT_REDIRECT` on L2 modes." | agrees | Builds directly on patch 0014's `LWTUNNEL_STATE_OUTPUT_REDIRECT` forward path by adding pre-encapsulation IPv4 fragmentation in `seg6_output_core` (patch 0015, `CONFIG_IPV6_SEG6_FORWARD_FRAG`) and extending this section with patch 0015 and `srv6-forward-frag` |
| `docs/architecture/linux-limitations.md` | How an entry is written | anchors of `probe_srv6_linux.go`, `kernelcap_linux.go`; page this spec modifies | "Each entry is a `###` heading that holds its id, followed by these fields:" | agrees | Adds verified entry `LL-SEG6-HEADEND-NO-DF-CLEAR-FRAG` with all required schema fields (`Kind: kernel`, `Effect: constrains design`, `Status: verified`, `Checked: Linux 7.3, 2026-11-22`, Limitation, Blocks, Evidence, Alternatives, Re-check, Ze meanwhile) and source anchors |
| `docs/architecture/mpls/mpls-kernel.md` | On a kernel without the patch | anchored to `internal/plugins/fib/kernel/kernelcap_linux.go` | "Ze's `ze-kernel-probe` profile grants the private `/proc/sys` mounts and the exact sysctl writes that Ze's namespace probes need (MPLS `platform_labels` and the three SRv6 forwarding sysctls)." | agrees | The extended SRv6 TUN probe reuses the exact same throwaway network/mount namespace, TUN device, and three IPv4/IPv6 forwarding sysctls, requiring zero changes to `ze-kernel-probe` or `mpls-kernel.md` |
| `ai/doors/forwarding.md` | Who depends on it from outside | the door covering `internal/plugins/fib/kernel/*` | "Ze owns only what it marked. It owns routes by `rtm_protocol` (`internal/core/rtproto`, `FIBKernel` 250, `Static` 251)" | agrees | Route installation and `rtprotZE` ownership in `fib-kernel` are untouched; the probe runs strictly inside a throwaway network namespace |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `test/ui/doctor-srv6-kernelcap.ci` | "**A `.ci` that needs a Linux kernel feature carries `option=needs-linux`** and runs in the QEMU VM (`ai/rules/platform-linux.md`)." | agrees | `test/ui/doctor-srv6-kernelcap.ci` retains `option=needs-linux`, and real kernel datapath and namespace probe tests stay in `*_integration_linux_test.go` (`//go:build integration && linux`) run in QEMU |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Datapath: a forwarded (or locally originated) DF-clear IPv4 packet whose length exceeds the seg6 route's effective MTU (`mtu - lwtunnel_headroom`, e.g. 1436 bytes for a single-SID SRH on a 1500-byte link) enters a Linux seg6 L3 encapsulation route (`SEG6_IPTUN_MODE_ENCAP` or `SEG6_IPTUN_MODE_ENCAP_RED`) installed by `buildSEG6Encap`.
- Detection: `ze` start, reload, `ze config validate`, and `ze doctor` with `fib { kernel { } }` and a BGP peer or group; `ze doctor kernel-capabilities` with no config.

### Transformation Path
1. Install (unchanged): sysrib best change with `SRv6SID` -> `fibKernel.processEvent` -> `addRichRoute` / `replaceRichRoute` -> `buildSEG6Encap` -> `RTM_NEWROUTE` with `LWTUNNEL_ENCAP_SEG6` (`mode encap`).
2. IPv4 forward path (from patch 0014): `ip_rcv` -> route lookup -> `ip_forward` (TTL > 1 check, `ip_exceeds_mtu` returns false for DF-clear, `ip_decrease_ttl` decrements inner TTL once, `NF_INET_FORWARD`, `ip_forward_finish` runs `ip_forward_options` if present) -> `dst_output` -> `lwtunnel_output` -> `seg6_output` -> `seg6_output_core`.
3. Pre-encapsulation IPv4 fragmentation (new, patch 0015): in `seg6_output_core`, when `skb->protocol == htons(ETH_P_IP)` and mode is `SEG6_IPTUN_MODE_ENCAP` or `SEG6_IPTUN_MODE_ENCAP_RED`:
   - Pull the IPv4 header (`pskb_may_pull(skb, sizeof(struct iphdr))`), read `mtu = ip_skb_dst_mtu(sk, skb)` (which subtracts `lwtunnel_headroom`).
   - Fast path: if `!skb_is_gso(skb)` and `skb->len <= mtu` and `!IPCB(skb)->frag_max_size`, call `seg6_output_encap(net, sk, skb)` directly.
   - GSO path: if `skb_is_gso(skb)`: if `skb_gso_validate_network_len(skb, mtu)` holds, call `seg6_output_encap` directly; otherwise segment with `skb_gso_segment(skb, netif_skb_features(skb) & ~NETIF_F_GSO_MASK)` and run the per-packet IPv4 MTU/fragment check on each segment.
   - Non-GSO oversized or `frag_max_size` path: if DF is clear (`(ip_hdr(skb)->frag_off & htons(IP_DF)) == 0`) or (`skb->ignore_df` and `(!IPCB(skb)->frag_max_size || IPCB(skb)->frag_max_size <= mtu)`), call `ip_do_fragment(net, sk, skb, seg6_output_encap)`. Otherwise (DF set and `!skb->ignore_df` reaching output after hook expansion or local raw send), increment `IPSTATS_MIB_FRAGFAILS`, send `icmp_send(skb, ICMP_DEST_UNREACH, ICMP_FRAG_NEEDED, htonl(mtu))`, free `skb`, and return `-EMSGSIZE`.
4. Per-fragment encapsulation (existing `seg6_output_core` body extracted as `seg6_output_encap`): for each fragment skb (size `<= mtu`, inner TTL already decremented, valid IPv4 header checksum, matching IPv4 ID, proper MF/offset, options stripped after first fragment), `seg6_do_srh` prepends Outer IPv6 (40B) + SRH (24B, `NH=IPIP`), resolves/reuses `slwt->cache_output`, replaces `skb_dst`, and calls `dst_output` -> `ip6_output` -> egress link (`<= 1500B`, unfragmented IPv6+SRH).
5. Detection: `kernelcap.Evaluate` -> `SRv6InUse` -> `kernelcap.SRv6ForwardFrag` -> `runSRv6ProbeMemoized` -> `askSRv6InThrowawayNamespace` sends (a) 44B TTL packet, then when `encap` is present (b) 1460B DF-clear packet + 44B sentinel packet on the same TUN fd -> classifies 2+ valid inner IPv4 fragments (`StatePresent`) vs sentinel before fragments (`StateAbsent`) -> memoizes 3-tuple -> `evaluateOne` emits `doctor-srv6-forward-frag-unavailable` or `doctor-srv6-forward-frag-unknown` (WARNING).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire -> kernel datapath -> wire | Oversized DF-clear IPv4 packet in; 2+ unfragmented IPv6+SRH packets (`NH=IPIP`, each `<=` egress MTU) carrying inner IPv4 fragments out | Yes |
| Component -> kernel (probe) | Existing throwaway network & mount namespace, existing TUN fd (`zesrv6p0`, MTU 1500); adds 1460B DF-clear packet + 44B sentinel packet | Yes |
| Kernel builder -> kernel tree | `applyPatches` (`internal/appliance/kernelbuilder/worker.go`) applies `0015-seg6-forward-df-clear-frag.patch` with `--fuzz=0`; `runtime.require` checks `CONFIG_IPV6_SEG6_FORWARD_FRAG=y` before compile | Yes |

### Changed-Wire Census

No BGP wire format, plugin RPC wire format, or rtnetlink route-encoding wire format changes: `buildSEG6Encap` continues sending the exact same `LWTUNNEL_ENCAP_SEG6` (`mode encap`) netlink attribute, and no BGP UPDATE bytes change. The only observable wire change is on the forwarded data plane when a DF-clear IPv4 packet exceeds the seg6 route's effective MTU (egress MTU minus seg6 headroom): instead of being silently dropped after encapsulation inside `ip6_fragment`, the patched headend emits two or more unfragmented outer IPv6+SRH packets (`NH=IPIP`, each within egress MTU) carrying standard RFC 791 inner IPv4 fragments with matching IPv4 ID, decremented inner TTL, valid IPv4 header checksums, and reassemblable payload.

**Search evidence:** searched `SEG6`, `seg6`, `SRv6ForwardTTL`, and `srv6-forward-ttl` across `internal/`, `gokrazy/`, `test/`, and `docs/` (`grep -rn "TestFIBServiceSIDOversizedDFAnswersFragmentationNeeded\|SRv6ForwardTTL\|0014-seg6" internal/ gokrazy/ docs/` and `grep -rn "LL-SEG6" docs/ internal/ test/ plan/`). Positive control: `internal/plugins/fib/kernel/nexthop_linux.go` (`buildSEG6Encap`), `internal/component/kernelcap/probe_srv6_linux.go` (`SRv6Encap`, `SRv6ForwardTTL`), `internal/plugins/fib/kernel/kernelcap_linux.go`, `test/ui/doctor-srv6-kernelcap.ci`, and `gokrazy/kernel/patches/0014-seg6-forward-ttl.patch`. No shell or Python producer/consumer exists for seg6 encapsulation.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| rtnetlink `RTM_NEWROUTE` (`LWTUNNEL_ENCAP_SEG6`) | writer | `internal/plugins/fib/kernel/nexthop_linux.go` (`buildSEG6Encap`, `buildRichRoute`) | Only Ze netlink SEG6 writer; unchanged by this spec | preserve byte-identical encoding |
| rtnetlink `RTM_NEWROUTE` (`LWTUNNEL_ENCAP_SEG6`) | writer (probe) | `internal/component/kernelcap/probe_srv6_linux.go` (`askSRv6InThrowawayNamespace`) | Installs the throwaway-namespace encap route matching `buildSEG6Encap` | preserve route install; reuse same TUN fd for second DF-clear probe packet |
| rtnetlink `RTM_NEWROUTE` (`LWTUNNEL_ENCAP_SEG6`) | reader | Linux kernel `net/ipv6/seg6_iptunnel.c` (`seg6_build_state`) | Unchanged by patch 0015 (patch 0014 already set `OUTPUT_REDIRECT` on L3 modes) | preserve netlink ABI |
| Forwarded IPv6+SRH+IPv4 wire packets (oversized DF-clear IPv4) | writer | Linux kernel `net/ipv6/seg6_iptunnel.c` (`seg6_output_core` -> `ip_do_fragment` -> `seg6_output_encap`), patched by `gokrazy/kernel/patches/0015-seg6-forward-df-clear-frag.patch` | Today drops oversized DF-clear IPv4 inside `ip6_fragment`; patched path emits unfragmented IPv6+SRH packets each carrying one inner IPv4 fragment | change on Ze's runtime kernel; stock kernel drop recorded in `LL-SEG6-HEADEND-NO-DF-CLEAR-FRAG` |
| Forwarded IPv6+SRH+IPv4 wire packets (oversized DF-clear IPv4) | reader | External SRv6 egress routers (Linux `seg6_local` `End.DT4`/`End.DX4`, VPP, hardware PEs), `internal/component/kernelcap/probe_srv6_linux.go`, and `internal/plugins/fib/kernel/srv6_integration_linux_test.go` | Unfragmented outer IPv6+SRH packets decapsulate cleanly in Linux `seg6_local` (`decap_and_validate`), and IPv4 destination hosts reassemble the inner IPv4 fragments | extend probe and integration tests to validate reassembly, options, and sub-quantum MTU drop |

### Integration Points
- `gokrazy/kernel/patches/series` - append `0015-seg6-forward-df-clear-frag.patch` after `0014-seg6-forward-ttl.patch`.
- `gokrazy/kernel/runtime.config` & `gokrazy/kernel/runtime.require` - set and pin `CONFIG_IPV6_SEG6_FORWARD_FRAG`.
- `internal/component/kernelcap/probe_srv6_linux.go` & `probe_other.go` - extend the shared probe and memo struct to return `(encap, forwardTTL, forwardFrag)` and export `SRv6ForwardFrag()`.
- `internal/plugins/fib/kernel/kernelcap_linux.go` - enroll `srv6ForwardFragCapability` (`srv6-forward-frag`, order 742, `Degrades`) using `kernelcap.SRv6InUse` and `kernelcap.SRv6ForwardFrag`.
- `internal/plugins/fib/kernel/register_doctor_codes.go` - register `doctor-srv6-forward-frag-unavailable` and `doctor-srv6-forward-frag-unknown`.
- `test/ui/doctor-srv6-kernelcap.ci` - force and assert `srv6-forward-frag` alongside `srv6-encap` and `srv6-forward-ttl`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Inner IPv4 packets still pass `ip_forward` (TTL decrement, FORWARD hook, XFRM forward policy, IPv4 option forwarding), then fragment via the kernel's own `ip_do_fragment` before `seg6_output_encap` |
| No unintended coupling (components stay isolated) | Yes | Capability enrolment and doctor codes stay in `internal/plugins/fib/kernel`; `internal/component/kernelcap` holds only the probe, matching `srv6-encap` and `srv6-forward-ttl` |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses Linux's exported `ip_do_fragment`, reuses Ze's existing throwaway-namespace TUN probe and `SRv6InUse` predicate, and reuses `ze-kernel-probe` AppArmor profile with zero edits |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Non-oversized packets take the fast path directly to `seg6_output_encap` with zero extra copies; the Go probe runs once per process via memoization |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | One `kernelcap.MustRegister(srv6ForwardFragCapability)` call in `fib/kernel/kernelcap_linux.go` and two `diagnostic.Register` entries in `register_doctor_codes.go`; no core switch or list is edited |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Searched `kernelcap.Enrolled`, `ProbeAll`, `KernelSymbols`, `internal/le/interoplab/kernelcheck.go`, `internal/appliance/kernelreq_enrolled_linux_test.go`, and `diagnostic.Lookup`: all derive dynamically from the registries |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `ip_do_fragment(net, sk, skb, seg6_output_encap)` safely executes inside `lwtunnel_output` (`local_bh_disable` + `rcu_read_lock` + `dev_xmit_recursion_inc`) and preserves `orig_dst` (`lwtstate`) across all fragments on both the `frag_list` fast path (`ip_fraglist_prepare` runs before `output`) and the `slow_path` (`ip_frag_next` copies `skb_dst`) | Linux v7.3 `net/ipv4/ip_output.c` lines 760-937 and `net/core/lwtunnel.c` lines 320-373 (read for this spec) | Refcount leak, use-after-free, or crash on fragment 2+ | QEMU integration tests `TestFIBServiceSIDOversizedDFClearFragmentsInnerIPv4` and `TestFIBServiceSIDOversizedDFClearPreservesIPv4Options` on patched 7.3.0-ze | confirmed |
| A-2 | `ip_do_fragment` is exported (`EXPORT_SYMBOL`) and callable from `net/ipv6/seg6_iptunnel.c` without symbol or header errors (including with `CONFIG_IPV6=y` and `CONFIG_IPV6=m`) | Linux v7.3 `net/ipv4/ip_output.c` line 937 (`EXPORT_SYMBOL(ip_do_fragment)`) and `net/ipv6/seg6_iptunnel.c` line 13 (`#include <net/ip.h>`) | Kernel build fails to compile or link | Runtime kernel build (`./ze-host appliance kernel --builder qemu --target runtime --arch amd64 --profile runtime`) | confirmed |
| A-3 | Sending a 1460-byte DF-clear IPv4 probe packet followed immediately by a 44-byte sentinel packet on the throwaway TUN fd (`zesrv6p0`, default MTU 1500) delivers the fragments before the sentinel on patched kernels and delivers the sentinel with zero fragments on stock/0014-only kernels in under 1 ms (no 1-second timeout penalty) | Linux TUN `tun_get_user` / `netif_rx` processes sequential `write(fd, ...)` calls in order inside one throwaway namespace | Probe either misclassifies present as absent (if sentinel overtakes fragments) or hits 1-second timeout on stock kernels | `TestSRv6Integration_ProbeAnswersFromAFreshNamespace` on both unpatched/0014-only 7.3.0-ze (expects `absent` fast) and patched 7.3.0-ze (expects `present`) | confirmed |
| A-4 | On a seg6 route whose IPv4 `orig_dst->dev` is the underlay egress link (Ze's route shape from `buildRichRoute`), `ip_skb_dst_mtu(sk, skb)` equals `min(dev->mtu, RTAX_MTU) - lwtunnel_headroom`, so every encapsulated inner IPv4 fragment fits within the outer egress link MTU without triggering `ip6_fragment` | Linux v7.3 `include/net/ip.h` (`ip_dst_mtu_maybe_forward`, `ip_skb_dst_mtu`) and `internal/plugins/fib/kernel/nexthop_linux.go` (`buildRichRoute`) | Encapsulated fragments exceed egress MTU and drop in `ip6_fragment` | `TestFIBServiceSIDOversizedDFClearFragmentsInnerIPv4` asserting every captured outer frame length `<= 1500` and exact reassembly | confirmed |
| A-5 | `ip_do_fragment`'s existing guard `if (mtu < hlen + 8) { err = -EMSGSIZE; goto fail; }` (Linux v7.3 `net/ipv4/ip_output.c` line 797) drops sub-quantum DF-clear packets cleanly without livelocking when an explicit `RTAX_MTU` leaves fewer than `hlen + 8` bytes after seg6 headroom | Linux v7.3 `net/ipv4/ip_output.c` lines 796-800 | Forwarding CPU spins in infinite zero-byte fragment loop | `TestFIBServiceSIDOversizedDFClearSubQuantumDropsCleanly` injecting a sub-quantum DF-clear packet followed by a normal marker packet on the same veth queue | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Split-interface or reduced underlay PMTU: if a route's IPv4 `orig_dst->dev` has a larger MTU (e.g. 1500 or `lo` 65536) than the outer IPv6 `dst` to the first SID (e.g. a second interface with MTU 1400 or learned IPv6 PMTU 1400), `ip_skb_dst_mtu(sk, skb)` reflects `orig_dst`, not `outer_dst`, so encapsulated fragments still exceed `outer_dst` MTU | Encapsulated fragments dropped by `ip6_fragment` when `orig_dst->dev->mtu > dst_mtu(outer_dst)` | Same boundary already applies to DF-set PMTU in `ip_forward`; documented explicitly in Known Limitations and `fib-depth-4-srv6.md`. Ze's `buildRichRoute` resolves the IPv4 route's next-hop onto the underlay egress interface |
| R-2 | `SEG6_IPTUN_MODE_ENCAP_RED` headroom over-estimate: `seg6_lwt_headroom` returns unreduced SRH length (`24 + 40 = 64` for 1 SID instead of `40`), so reduced-encap inner fragments are 24 bytes smaller than the link could theoretically carry | Slightly smaller fragments in `encap.red` mode | Safe and conservative (never overflows egress MTU); Ze uses `SEG6_IPTUN_MODE_ENCAP` (exact 64-byte headroom) |
| R-3 | Sentinel ordering in TUN probe (A-3): if fragments were queued asynchronously behind the sentinel, the probe could read the sentinel before fragments arrive and falsely return `StateAbsent` | `TestSRv6Integration_ProbeAnswersFromAFreshNamespace` fails with `srv6-forward-frag = absent` on patched 7.3.0-ze | Verify in QEMU on patched 7.3.0-ze; `tun_net_xmit` appends synchronously to `tun_file->tx_ring` inside softirq before the next `write(fd)` executes, so fragment frames always precede the sentinel in the ring |
| R-4 | Adding `srv6-forward-frag` causes existing `test/ui/doctor-srv6-kernelcap.ci` or `internal/plugins/fib/kernel/kernelcap_codes_linux_test.go` to observe an unforced `srv6-forward-frag` row on stock CI hosts | `doctor-srv6-kernelcap.ci` sees unexpected real probe output on stock Linux | Update `doctor-srv6-kernelcap.ci` in Phase 1 to include `srv6-forward-frag` in `ze.test.kernelcap.force` and assert both its forced diagnostic and its omission when BGP is absent |
| R-5 | Patch `0015-seg6-forward-df-clear-frag.patch` stops applying at a future kernel bump | `applyPatches` (`--fuzz=0`) fails the kernel build | Patch touches only `net/ipv6/Kconfig` and `net/ipv6/seg6_iptunnel.c`; `LL-SEG6-HEADEND-NO-DF-CLEAR-FRAG`'s Re-check names both files and functions |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Oversized DF-clear IPv4 packets crossing an SRv6 headend could be corrupted, dropped, or livelock (kernel patch defect), or `ze doctor` could report a false warning (probe defect). `srv6-forward-frag` degrades only and never refuses daemon startup |
| How is it reverted? | Remove `0015-seg6-forward-df-clear-frag.patch` from `series` and its symbol from `runtime.config`/`runtime.require` (requires kernel rebuild), and revert the probe/capability commit. No config migration, no BGP wire change |
| Who else touches this path? | `gokrazy/kernel/` (kernel bumps), `internal/component/kernelcap/probe_srv6_linux.go`, `internal/plugins/fib/kernel/` (SRv6/MPLS capabilities and doctor codes). Check `git status` before committing |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| sysrib best change with `SRv6SID`, oversized DF-clear IPv4 packet in | → | `buildSEG6Encap` route -> `ip_forward` -> `seg6_output_core` -> `ip_do_fragment` -> `seg6_output_encap` | `TestFIBServiceSIDOversizedDFClearFragmentsInnerIPv4` |
| `ze doctor --json config <file>` with `fib { kernel { } }` and BGP peer | → | `srv6ForwardFragCapability` -> `SRv6InUse` -> `kernelcap.SRv6ForwardFrag` -> `doctor-srv6-forward-frag-unavailable` | `test/ui/doctor-srv6-kernelcap.ci` |
| `kernelcap.SRv6ForwardFrag()` in a fresh network namespace with forwarding off | → | `runSRv6ProbeMemoized` -> `askSRv6InThrowawayNamespace` -> TUN DF-clear probe + sentinel | `TestSRv6Integration_ProbeAnswersFromAFreshNamespace` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Forwarded DF-clear IPv4 UDP packets of 1436 bytes (exact fit), 1437 bytes (effective MTU + 1), and 1460 bytes crossing a 1500-byte egress link with single-SID SRv6 encap (64-byte headroom, effective MTU 1436) on Ze's patched 7.3.0-ze kernel | The 1436-byte packet leaves as 1 unfragmented 1500-byte outer IPv6+SRH (`NH=IPIP`) packet with inner TTL 63 and unchanged payload. Both the 1437-byte and 1460-byte packets leave as 2 unfragmented outer IPv6+SRH (`NH=IPIP`) packets each `<= 1500` bytes carrying valid inner IPv4 fragments (inner TTL 63, matching IPv4 ID, valid header checksums, 8-byte aligned first-fragment payload with MF=1 and offset=0, second fragment with MF=0 and exact offset) that reassemble byte-for-byte to the original IPv4 payload, with zero ICMP errors and zero outer IPv6 Fragment Headers (`NH=44`) | None on Ze's patched kernel: every oversized DF-clear packet must fragment into unfragmented IPv6+SRH frames and reassemble cleanly. On stock kernels without patch 0015 (`srv6-forward-frag` absent), the packet is dropped as documented in `LL-SEG6-HEADEND-NO-DF-CLEAR-FRAG` |
| AC-2 | Forwarded oversized DF-clear IPv4 packet (1460 bytes) carrying both a copied IPv4 option (e.g. Router Alert or Security/NOP) and a non-copied IPv4 option (e.g. Record Route or Timestamp) crossing the patched SRv6 headend | Emits 2 unfragmented outer IPv6+SRH packets each `<= 1500` bytes: the first inner IPv4 fragment carries all original options (with Record Route updated by `ip_forward_options`), the second inner IPv4 fragment retains copied options and omits non-copied options, both carry valid header checksums and inner TTL 63, and the inner payloads reassemble byte-for-byte | None: RFC 791 Section 3.1 requires non-copied options to appear only in the first fragment |
| AC-3 | Forwarded DF-clear IPv4 packet crossing an SRv6 route whose explicit route `PathMTU` (`RTAX_MTU`) leaves fewer than `hlen + 8` bytes after 64-byte seg6 headroom (e.g. `PathMTU=84` -> 20 bytes < 28 bytes), followed immediately by a normal forwarded marker packet on the same veth queue | The sub-quantum packet is dropped cleanly without encapsulating or livelocking, and the subsequent marker packet arrives intact on the egress capture link | Dropping the sub-quantum packet is the required RFC-safe refusal because no 8-byte IPv4 fragment quantum fits; zero encapsulated fragments may be emitted for it |
| AC-4 | Existing DF-set PMTU (`TestFIBServiceSIDOversizedDFAnswersFragmentationNeeded`), IPv4/IPv6 inner TTL/Hop Limit decrement, and TTL/Hop Limit 1 Time Exceeded tests on Ze's patched 7.3.0-ze kernel | All 5 existing `TestFIBServiceSID*` datapath tests continue to pass unchanged: DF-set 1437-byte IPv4 still returns ICMP Fragmentation Needed (MTU 1436) with zero SRv6 egress frames, and IPv4/IPv6 TTL/Hop Limit decrement and expiry behavior remain intact | None: patch 0015 must not regress DF-set PMTU or TTL/Hop Limit enforcement |
| AC-5 | Live throwaway-namespace probe `TestSRv6Integration_ProbeAnswersFromAFreshNamespace` on patched 7.3.0-ze (with patch 0015) and on 0014-only / stock Linux kernels | On patched 7.3.0-ze (`-ze` release), `kernelcap.SRv6ForwardFrag()` answers `StatePresent` alongside `SRv6Encap()` and `SRv6ForwardTTL()`; on a kernel without patch 0015, `SRv6ForwardFrag()` answers `StateAbsent` without waiting for the 1-second timeout (sentinel terminates immediately); caller's `/proc/sys/net/ipv4/ip_forward` stays unchanged | Off Linux (`!linux`), `SRv6ForwardFrag()` returns `StateUnknown` (`errNotLinux`), matching `SRv6Encap()` and `SRv6ForwardTTL()`; unprivileged/no-TUN hosts return `StateUnknown` or `StateDenied` |
| AC-6 | `ze doctor --json config` with `fib { kernel { } }` and a BGP peer when `srv6-forward-frag` is forced absent or unknown, and with no BGP block | With BGP present, `ze doctor --json` exits 0 (degrading WARNING), reports `doctor-srv6-forward-frag-unavailable` (naming `CONFIG_IPV6_SEG6_FORWARD_FRAG`, `LL-SEG6-HEADEND-NO-DF-CLEAR-FRAG`, and `ze doctor`) or `doctor-srv6-forward-frag-unknown`, and `kernelcap.Refuse` returns nil; without BGP, `ze doctor --json` reports `"ready": true` with no `srv6-forward-frag` row | Degrading warning (exit 0) is intentional so stock-kernel BGP routers that carry no SRv6 routes still boot, matching `srv6-encap` and `srv6-forward-ttl` |
| AC-7 | Kernel build manifest and enrolled capability check (`TestEnrolledKernelSymbolsAreInRuntimeRequire`, `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require`, `gokrazy/kernel/patches/series`, and native runtime kernel build) | `0015-seg6-forward-df-clear-frag.patch` applies with `--fuzz=0` to Linux v7.3, `CONFIG_IPV6_SEG6_FORWARD_FRAG=y` is resolved in the built kernel config and pinned in `runtime.require`, and `TestEnrolledKernelSymbolsAreInRuntimeRequire` passes | None: a kernel build missing patch 0015 must fail before compile |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Forwards an oversized DF-clear IPv4 packet (e.g. 1460B UDP/IPv4) through a BGP SRv6 Service SID route on Ze's appliance kernel | sysrib best-change with `SRv6SID` -> `fibKernel.processEvent` -> `buildSEG6Encap` -> `ip_forward` (TTL-1) -> `seg6_output_core` -> `ip_do_fragment` -> `seg6_output_encap` -> 2 unfragmented IPv6+SRH packets `<= 1500B` on egress link | `TestFIBServiceSIDOversizedDFClearFragmentsInnerIPv4` |
| 2 | Runs `ze doctor --json` on a stock Linux kernel with `fib { kernel { } }` and BGP configured | `ze doctor` -> `kernelcap.Evaluate` -> `SRv6InUse` -> `SRv6ForwardFrag` TUN probe -> `doctor-srv6-forward-frag-unavailable` WARNING naming `CONFIG_IPV6_SEG6_FORWARD_FRAG` and `LL-SEG6-HEADEND-NO-DF-CLEAR-FRAG` | `test/ui/doctor-srv6-kernelcap.ci`, `TestSRv6Integration_ProbeAnswersFromAFreshNamespace` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSRv6ProbeDFClearFragClassification` | `internal/component/kernelcap/probe_srv6_linux_test.go` | Classifier recognizes 2+ valid inner IPv4 fragments (`StatePresent`), sentinel before fragments (`StateAbsent`), truncated/non-matching frames ignored, single fragment without completion (`StateUnknown`), and 3-tuple memoization | |
| `TestSRv6CapabilityCodesRegistered` (extended) | `internal/plugins/fib/kernel/kernelcap_codes_linux_test.go` | `srv6-forward-frag` is enrolled and `doctor-srv6-forward-frag-unavailable` / `doctor-srv6-forward-frag-unknown` are registered with Title, Description, and Remedy | |
| `TestSRv6CapabilitiesDegrade` (extended) | `internal/plugins/fib/kernel/kernelcap_codes_linux_test.go` | `srv6-forward-frag` declares `Degrades`, names `CONFIG_IPV6_SEG6_FORWARD_FRAG`, never refuses startup, and emits WARNING when forced absent | |
| `TestEnrolledKernelSymbolsAreInRuntimeRequire` | `internal/appliance/kernelreq_enrolled_linux_test.go` | `CONFIG_IPV6_SEG6_FORWARD_FRAG` is pinned in `gokrazy/kernel/runtime.require` | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Inner IPv4 DF-clear datagram length on 1500B link with 64B SRv6 headroom | 28..65535 (unfragmented up to 1436; fragmented at 1437+) | 1436 (unfragmented exact fit), 1437 (smallest fragmented: 2 frags, 1416B + 1B payload) | N/A (smaller packets pass fast path) | 1460 (2 frags: 1416B + 24B payload) |
| Route `PathMTU` (`RTAX_MTU`) minus 64B headroom for 20B IPv4 header | `>= 92` (`92 - 64 = 28 = 20 + 8`, one fragment quantum) | 92 (effective MTU 28, fragments cleanly) | 84 (`84 - 64 = 20 < 28`: `ip_do_fragment` drops cleanly with `-EMSGSIZE`, no livelock) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `doctor-srv6-kernelcap` | `test/ui/doctor-srv6-kernelcap.ci` | `ze doctor --json` with BGP reports forced `doctor-srv6-forward-frag-unavailable` (WARNING, exit 0, naming `CONFIG_IPV6_SEG6_FORWARD_FRAG` and `LL-SEG6-HEADEND-NO-DF-CLEAR-FRAG`), and omits `srv6-forward-frag` when BGP is absent | |
| `TestFIBServiceSIDOversizedDFClearFragmentsInnerIPv4` | `internal/plugins/fib/kernel/srv6_integration_linux_test.go` | QEMU on patched 7.3.0-ze (and RED on 0014-only 7.3.0-ze): 1436B exact fit, 1437B boundary+1, and 1460B DF-clear IPv4 fragment into unfragmented IPv6+SRH packets `<= 1500B` that reassemble byte-for-byte | |
| `TestFIBServiceSIDOversizedDFClearPreservesIPv4Options` | `internal/plugins/fib/kernel/srv6_integration_linux_test.go` | QEMU on patched 7.3.0-ze: oversized DF-clear IPv4 packet with copied and non-copied IPv4 options preserves all options in fragment 1 and omits non-copied options in fragment 2 | |
| `TestFIBServiceSIDOversizedDFClearSubQuantumDropsCleanly` | `internal/plugins/fib/kernel/srv6_integration_linux_test.go` | QEMU on patched 7.3.0-ze: sub-quantum `PathMTU=84` drops DF-clear packet cleanly without livelock; subsequent marker packet arrives | |
| `TestSRv6Integration_ProbeAnswersFromAFreshNamespace` (extended) | `internal/plugins/fib/kernel/srv6probe_integration_linux_test.go` | QEMU on patched 7.3.0-ze: `SRv6ForwardFrag()` answers `StatePresent`; on 0014-only 7.3.0-ze answers `StateAbsent` rapidly | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A (no BGP wire change; Linux `seg6_local` egress decapsulation proven in kernel integration) | N-A | N-A | No BGP control-plane wire format changes. Inner IPv4 fragmentation produces standard unfragmented IPv6+SRH (`NH=IPIP`) packets carrying RFC 791 IPv4 fragments, proven end-to-end on Ze's runtime kernel in QEMU (with RED/GREEN discrimination against the 0014-only 7.3.0-ze kernel) | |

## Files to Modify
- `gokrazy/kernel/patches/series` - append `0015-seg6-forward-df-clear-frag.patch`
- `gokrazy/kernel/runtime.config` - enable `CONFIG_IPV6_SEG6_FORWARD_FRAG=y`
- `gokrazy/kernel/runtime.require` - pin `CONFIG_IPV6_SEG6_FORWARD_FRAG`
- `internal/component/kernelcap/probe_srv6_linux.go` - extend shared TUN probe and memoization with DF-clear fragmentation packet + sentinel and `SRv6ForwardFrag()`
- `internal/component/kernelcap/probe_other.go` - add `SRv6ForwardFrag()` non-Linux stub
- `internal/component/kernelcap/probe_srv6_linux_test.go` - unit tests for DF-clear fragmentation probe classification and 3-tuple memoization
- `internal/plugins/fib/kernel/kernelcap_linux.go` - enroll `srv6ForwardFragCapability` (`srv6-forward-frag`, order 742, `Degrades`)
- `internal/plugins/fib/kernel/register_doctor_codes.go` - register `doctor-srv6-forward-frag-unavailable` and `doctor-srv6-forward-frag-unknown`
- `internal/plugins/fib/kernel/kernelcap_codes_linux_test.go` - extend registration and degrading tests for `srv6-forward-frag`
- `internal/plugins/fib/kernel/srv6_integration_linux_test.go` - add `srv6ForwardFragPatched` and three DF-clear fragmentation integration tests (leaving existing 5 tests intact)
- `internal/plugins/fib/kernel/srv6probe_integration_linux_test.go` - assert `kernelcap.SRv6ForwardFrag()` on `-ze` and non-`-ze` kernels
- `test/ui/doctor-srv6-kernelcap.ci` - force and assert `srv6-forward-frag` alongside `srv6-encap` and `srv6-forward-ttl`
- `docs/architecture/linux-limitations.md` - add verified entry `LL-SEG6-HEADEND-NO-DF-CLEAR-FRAG`
- `docs/architecture/fib/fib-depth-4-srv6.md` - document patch 0015, pre-encapsulation DF-clear IPv4 fragmentation, and `srv6-forward-frag`
- `docs/architecture/doctor-and-health-checks.md` - add `srv6-forward-frag` to the shared memoized SRv6 probe paragraph
- `docs/features/srv6.md` - document patch 0015 and `doctor-srv6-forward-frag-unavailable`
- `plan/journal/kernel-refuses-what-the-installer-sends.md` - mark the 2026-11-22 SRv6 DF-clear row `fixed:` once proven

## Files to Create
- `gokrazy/kernel/patches/0015-seg6-forward-df-clear-frag.patch` - Linux v7.3 patch adding `CONFIG_IPV6_SEG6_FORWARD_FRAG` and IPv4 pre-encapsulation fragmentation in `seg6_output_core`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Kernel datapath fix and degrading `kernelcap` probe; no new config leaf or RPC |
| YANG validation constraints | N-A | No new YANG leaf |
| YANG custom validators | N-A | No new YANG leaf |
| CLI commands/flags | N-A | Existing `ze doctor` and `ze doctor kernel-capabilities` discover the enrolled capability automatically |
| CLI grammar (keyword before value) | N-A | No CLI command added or changed |
| Editor autocomplete | N-A | No config or CLI addition |
| Functional test for new RPC/API | Yes | `test/ui/doctor-srv6-kernelcap.ci` (forced capability doctor check) |
| Pipe completeness | N-A | No new command output |
| Env var registration | N-A | Reuses existing `ze.test.kernelcap.force` |
| Doctor check for runtime dependencies | Yes | `internal/plugins/fib/kernel/kernelcap_linux.go` (`srv6ForwardFragCapability`), `internal/plugins/fib/kernel/register_doctor_codes.go`, `internal/component/kernelcap/probe_srv6_linux.go` |
| Doctor code per problem class | Yes | `doctor-srv6-forward-frag-unavailable` (stock kernel without patch 0015, WARNING, pre-start/doctor `kernelcap` phase) and `doctor-srv6-forward-frag-unknown` (probe could not run, WARNING, pre-start/doctor `kernelcap` phase) |
| Prometheus counters/metrics | N-A | Linux kernel's existing `IPSTATS_MIB_FRAGCREATES` / `FRAGOKS` / `FRAGFAILS` counters track fragmentation via `ip_do_fragment`; no new Ze daemon metric |
| BGP family surface (new SAFI / capability / attribute) | N-A | No new BGP SAFI, capability, or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `docs/features/srv6.md` (Linux Kernel section: add patch 0015 and `doctor-srv6-forward-frag-unavailable`) |
| 2 | Config syntax changed? | N-A | No config syntax change |
| 3 | CLI command added/changed? | N-A | No CLI command change |
| 4 | API/RPC added/changed? | N-A | No API/RPC change |
| 5 | Plugin added/changed? | No | `fib-kernel` plugin metadata and transport unchanged |
| 6 | Has a user guide page? | N-A | No separate SRv6 user guide page; covered by `docs/features/srv6.md` |
| 7 | Wire format changed? | N-A | Standard RFC 791 IPv4 fragments inside standard RFC 8986 IPv6+SRH; no BGP or Ze wire format changed |
| 8 | Plugin SDK/protocol changed? | N-A | No SDK or plugin protocol change |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC 8986 is not enrolled (no release profile lists it); RFC 2473 summary covers `ip6tnl`/`ipip6`, not seg6 |
| 10 | Test infrastructure changed? | N-A | Reuses existing QEMU integration harness and `.ci` runner unchanged |
| 11 | Affects daemon comparison? | N-A | No daemon comparison row for SRv6 DF-clear fragmentation |
| 12 | Internal architecture changed? | Yes | `docs/architecture/fib/fib-depth-4-srv6.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/architecture/linux-limitations.md` |
| 13 | Route metadata keys added/changed? | N-A | No route metadata change |
| 14 | Prometheus counters added/changed? | N-A | No new Ze Prometheus counter |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | `kernelcap` enrolments are documented in `docs/architecture/doctor-and-health-checks.md` and `docs/architecture/fib/fib-depth-4-srv6.md`, not in `docs/plugin-overview.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived via `./le spec citation anchors`: `docs/architecture/doctor-and-health-checks.md`, `docs/architecture/fib/fib-depth-4-srv6.md`, `docs/architecture/linux-limitations.md`, and `docs/features/srv6.md` are updated by this spec; `docs/architecture/mpls/mpls-kernel.md` (anchored to `kernelcap_linux.go` for `transitMTUCapability`) is unaffected because `ze-kernel-probe` and MPLS capability enrolments do not change |
| 17 | Existing docs show config/CLI/API examples for this area? | N-A | Existing SRv6 route install examples (`ip route add ... encap seg6 mode encap segs ...`) remain accurate |

## Design presentation

### What changes

On Ze's runtime kernel, patch `0015-seg6-forward-df-clear-frag.patch` (`CONFIG_IPV6_SEG6_FORWARD_FRAG=y`, pinned in `gokrazy/kernel/runtime.require`) makes `seg6_output_core` fragment oversized DF-clear IPv4 packets at the inner IPv4 layer via Linux's exported `ip_do_fragment` before SRv6 encapsulation, emitting two or more unfragmented outer IPv6+SRH packets each within the egress MTU. Non-oversized packets take the fast path unchanged, DF-clear GSO packets whose segment size exceeds effective MTU are segmented and fragmented, IPv4 options follow RFC 791 fragmentation rules, and sub-quantum MTUs (`mtu < hlen + 8`) drop cleanly without livelock. Ze's shared SRv6 throwaway-namespace TUN probe (`internal/component/kernelcap/probe_srv6_linux.go`) adds one 1460-byte DF-clear probe packet plus a 44-byte sentinel packet on the same TUN fd to expose `kernelcap.SRv6ForwardFrag()`, enrolled by `fib-kernel` as degrading capability `srv6-forward-frag` (codes `doctor-srv6-forward-frag-unavailable` and `doctor-srv6-forward-frag-unknown`), with stock-kernel behavior recorded in `LL-SEG6-HEADEND-NO-DF-CLEAR-FRAG`.

### Before and after

1. **Oversized DF-clear IPv4 packet (1460 bytes, TTL 64) crossing a 1500-byte egress link with single-SID SRv6 encap (64-byte headroom, effective MTU 1436):**
   - Before (patched 7.3.0-ze with patch 0014 only): `ip_forward` skips `ip_exceeds_mtu` (DF clear), `lwtunnel_output` skips `ip_finish_output`, `seg6_output_core` builds a 1524-byte outer IPv6+SRH packet with `ignore_df == 0`, and `ip6_fragment` drops it while sending ICMPv6 Packet Too Big to the headend itself (`unfragmented-srv6=0 ipv6-outer-fragments=0 icmp-errors=0`).
   - After (Ze runtime kernel with patch 0015): `seg6_output_core` calls `ip_do_fragment` at effective MTU 1436, then encapsulates each fragment via `seg6_output_encap`:
     - Egress frame 1: 1500-byte unfragmented IPv6+SRH (`NH=IPIP`) carrying 1436-byte inner IPv4 fragment 1 (`TTL=63`, `MF=1`, `offset=0`, 1416B payload).
     - Egress frame 2: 108-byte unfragmented IPv6+SRH (`NH=IPIP`) carrying 44-byte inner IPv4 fragment 2 (`TTL=63`, `MF=0`, `offset=1416`, 24B payload).
     - Both packets decapsulate cleanly on Linux `seg6_local` (`End.DT4` / `End.DX4`) and reassemble byte-for-byte at the IPv4 destination.
2. **`ze doctor --json` on a stock kernel (or 0014-only kernel) with `fib { kernel { } }` and a BGP peer:**
   - Before: `ze doctor` reports `srv6-encap` and `srv6-forward-ttl` only; silent DF-clear IPv4 drops are unflagged.
   - After: `ze doctor` emits a WARNING row with code `doctor-srv6-forward-frag-unavailable` naming `CONFIG_IPV6_SEG6_FORWARD_FRAG`, `LL-SEG6-HEADEND-NO-DF-CLEAR-FRAG`, and the remedy (run Ze's appliance kernel or build with patch `0015-seg6-forward-df-clear-frag.patch`), still exiting 0.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Outer IPv6 fragmentation after encapsulation (`skb->ignore_df = 1` in `seg6_output_core`, literal RFC 2473 Section 7.2(b)) | Verified broken against Linux's own SRv6 egress decapsulator (`net/ipv6/seg6_local.c`): `seg6_local_build_state` sets `LWTUNNEL_STATE_INPUT_REDIRECT`, so `End.DT4`/`End.DX4` runs `decap_and_validate` BEFORE `ip6_local_deliver` / `ipv6_frag_rcv` and drops outer IPv6 fragments unconditionally. Outer IPv6 fragments also break reassembly on anycast SIDs (noted in RFC 2473 Section 7 itself) and are commonly dropped/punted by hardware SRv6 ASICs |
| Switch seg6 L3 modes from `LWTUNNEL_STATE_OUTPUT_REDIRECT` to `LWTUNNEL_STATE_XMIT_REDIRECT` (the MPLS lwtunnel hook) | Would run `NF_INET_POST_ROUTING` and `BPF_CGROUP_RUN_PROG_INET_EGRESS` unconditionally on the inner packet before encapsulation (changing `nf_hooks_lwtunnel_enabled` semantics and double-running POST_ROUTING on outer IPv6), drop `sk` from `lwtunnel_xmit(skb)` for local sockets, and touch the IPv6-inner path (`ip6_output` rejects routes whose `orig_dst->dev` has `disable_ipv6=1`). Calling `ip_do_fragment` directly from `seg6_output_core` keeps `OUTPUT_REDIRECT` intact and scopes the change strictly to oversized IPv4 packets |
| No `kernelcap` enrolment (patch and QEMU test only, no `ze doctor` warning) | Leaves operators on stock Linux kernels (or a kernel with only patch 0014) with silent DF-clear IPv4 blackholing and no `ze doctor` diagnostic; also leaves integration tests without a clean capability probe to distinguish 0014-only kernels from 0015 kernels. Extending the existing shared TUN probe adds <1 ms via the sentinel packet and zero new namespaces or AppArmor rules |

### Documents departed from

none

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| C-1 | Fragmentation layer for oversized DF-clear IPv4 at the SRv6 headend | (a) Pre-encapsulation inner IPv4 fragmentation via `ip_do_fragment` in `seg6_output_core` — emits unfragmented IPv6+SRH packets that work with Linux `seg6_local` (`End.DT4`/`End.DX4`), VPP, hardware ASICs, and anycast SIDs (Recommended); (b) Post-encapsulation outer IPv6 fragmentation via `skb->ignore_df = 1` — follows RFC 2473 Section 7.2(b) literally, but Linux `seg6_local` drops outer IPv6 fragments without reassembly and anycast SIDs fail reassembly | (a) adopted 2026-11-22 |
| C-2 | Runtime capability probe and `ze doctor` enrolment | (a) Extend the shared SRv6 TUN probe with one 1460B DF-clear packet + 44B sentinel, enroll degrading `srv6-forward-frag` (`CONFIG_IPV6_SEG6_FORWARD_FRAG`, `doctor-srv6-forward-frag-*`), and pin the marker in `runtime.require` (Recommended); (b) Keep patch 0015 + `CONFIG_IPV6_SEG6_FORWARD_FRAG` in `runtime.require` only, with no new `kernelcap` enrolment or `ze doctor` warning | (a) adopted 2026-11-22 |
| C-3 | One-off WIP cap exception to start implementation (`18 -> 19` in flight) | Owner granted one-off `ZE_SPEC_WIP_CAP=19` exception on 2026-11-22 | granted 2026-11-22 |

## Implementation Steps

1. **Phase 1: Wiring (MANDATORY FIRST)** -- enroll `srv6-forward-frag` with a stub returning `StateUnknown`, register the two `doctor-srv6-forward-frag-*` codes, extend unit tests and `test/ui/doctor-srv6-kernelcap.ci`, and update `docs/architecture/doctor-and-health-checks.md`
   - Tests: `TestSRv6CapabilityCodesRegistered`, `TestSRv6CapabilitiesDegrade`, `test/ui/doctor-srv6-kernelcap.ci`, `TestEnrolledKernelSymbolsAreInRuntimeRequire` (expected red until Phase 2 pins the symbol)
   - Files: `internal/component/kernelcap/probe_srv6_linux.go` (stub `SRv6ForwardFrag`), `probe_other.go`, `internal/plugins/fib/kernel/kernelcap_linux.go`, `register_doctor_codes.go`, `kernelcap_codes_linux_test.go`, `test/ui/doctor-srv6-kernelcap.ci`, `docs/architecture/doctor-and-health-checks.md`
   - Verify: `ze doctor` exposes `srv6-forward-frag` via forced answers; `TestEnrolledKernelSymbolsAreInRuntimeRequire` fails until Phase 2
2. **Phase 2: Kernel patch and runtime manifest** -- write `gokrazy/kernel/patches/0015-seg6-forward-df-clear-frag.patch`, append to `series`, set and pin `CONFIG_IPV6_SEG6_FORWARD_FRAG` in `runtime.config` and `runtime.require`, and build the amd64 runtime kernel
   - Tests: `TestEnrolledKernelSymbolsAreInRuntimeRequire`, `./ze-host appliance kernel --builder qemu --target runtime --arch amd64 --profile runtime`
   - Files: `gokrazy/kernel/patches/0015-seg6-forward-df-clear-frag.patch`, `gokrazy/kernel/patches/series`, `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require`
   - Verify: patch applies with `--fuzz=0` on Linux v7.3, built kernel config resolves `CONFIG_IPV6_SEG6_FORWARD_FRAG=y`, commit this phase on its own
3. **Phase 3: Behavioural probe** -- extend `askSRv6InThrowawayNamespace` and `runSRv6ProbeMemoized` to inject the 1460B DF-clear packet + 44B sentinel on the same TUN fd, parse inner IPv4 fragments, classify present/absent/unknown, and memoize the 3-tuple
   - Tests: `TestSRv6ProbeDFClearFragClassification`, `TestSRv6ProbeMemoizesDefinitiveVerdict`, `TestSRv6Integration_ProbeAnswersFromAFreshNamespace`
   - Files: `internal/component/kernelcap/probe_srv6_linux.go`, `probe_srv6_linux_test.go`, `internal/plugins/fib/kernel/srv6probe_integration_linux_test.go`
   - Verify: unit tests pass; live probe answers `absent` on 0014-only 7.3.0-ze in QEMU (fast, sentinel-terminated) and `present` on 0015-patched 7.3.0-ze in QEMU
4. **Phase 4: QEMU datapath proofs & RED/GREEN discrimination** -- add `srv6ForwardFragPatched` and the three new integration tests (`TestFIBServiceSIDOversizedDFClearFragmentsInnerIPv4`, `TestFIBServiceSIDOversizedDFClearPreservesIPv4Options`, `TestFIBServiceSIDOversizedDFClearSubQuantumDropsCleanly`), and update docs (`linux-limitations.md`, `fib-depth-4-srv6.md`, `docs/features/srv6.md`, `plan/journal/kernel-refuses-what-the-installer-sends.md`)
   - Tests: all `TestFIBServiceSID*` and `TestSRv6Integration_*` in QEMU on both 0014-only 7.3.0-ze (RED) and 0015-patched 7.3.0-ze (GREEN), plus `./le doc check all`
   - Files: `internal/plugins/fib/kernel/srv6_integration_linux_test.go`, `docs/architecture/linux-limitations.md`, `docs/architecture/fib/fib-depth-4-srv6.md`, `docs/features/srv6.md`, `plan/journal/kernel-refuses-what-the-installer-sends.md`
   - Verify: RED recorded on 0014-only 7.3.0-ze, GREEN recorded on 0015-patched 7.3.0-ze for all new and existing SRv6 tests; all A-1..A-5 confirmed; `./le doc check all` passes

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-7 has an implementation and fresh evidence at file:line |
| Feature completeness | Both user stories work end-to-end with passing tests |
| Correctness | Patch 0015 applies only to `ETH_P_IP` on `ENCAP`/`ENCAP_RED`, leaves `INLINE`/`L2ENCAP`/`ETH_P_IPV6` untouched, preserves DF-set ICMP Frag Needed, handles GSO via `skb_gso_segment`, reuses `ip_do_fragment`, and probe validates complete inner IPv4 fragment reassembly/offsets rather than counting any two frames |
| Naming | Subsystem `srv6-forward-frag` (lower-kebab); codes `doctor-srv6-forward-frag-unavailable` and `doctor-srv6-forward-frag-unknown`; Kconfig marker `IPV6_SEG6_FORWARD_FRAG`; limitation `LL-SEG6-HEADEND-NO-DF-CLEAR-FRAG` |
| Data flow | `fib-kernel` owns enrolment and doctor codes; `kernelcap` owns only the probe; no new AppArmor rule or central list edit |
| Operator errors and doctor codes | Both doctor code descriptions state what is wrong, why (stock/0014-only kernel without patch 0015 or missing privilege/TUN), and exact Remedy ending in `ze doctor` (`ai/rules/user-facing-errors.md`) |
| Rule: `ai/rules/interop-and-goal-validation.md` | RED run on 0014-only 7.3.0-ze and GREEN run on 0015-patched 7.3.0-ze both recorded |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Patch `0015-seg6-forward-df-clear-frag.patch` in `series` | `grep 0015-seg6-forward-df-clear-frag.patch gokrazy/kernel/patches/series` |
| `CONFIG_IPV6_SEG6_FORWARD_FRAG` set and pinned | `grep -n IPV6_SEG6_FORWARD_FRAG gokrazy/kernel/runtime.config gokrazy/kernel/runtime.require` |
| `srv6-forward-frag` enrolment and two doctor codes | `grep -n "srv6-forward-frag\|doctor-srv6-forward-frag-" internal/plugins/fib/kernel/kernelcap_linux.go internal/plugins/fib/kernel/register_doctor_codes.go` |
| Unit tests green | `./le test unit pkg path ./internal/component/kernelcap/` and `./internal/plugins/fib/kernel/` and `./internal/appliance/` |
| Doctor `.ci` test green | `./le test ui doctor-srv6-kernelcap` |
| QEMU RED/GREEN datapath & probe proof | `./le test qemu run` on 0014-only 7.3.0-ze (RED) and 0015-patched 7.3.0-ze (GREEN) running `TestFIBServiceSID\|TestSRv6Integration` |
| Limitation entry and docs | `grep -n LL-SEG6-HEADEND-NO-DF-CLEAR-FRAG docs/architecture/linux-limitations.md` and `./le doc check all` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation (kernel) | `seg6_output_core` calls `pskb_may_pull(skb, sizeof(struct iphdr))` before reading `ip_hdr(skb)->frag_off`, and `ip_do_fragment` guards `mtu < hlen + 8` before fragmentation so crafted small MTUs or 60-byte IPv4 option headers cannot livelock the forwarding CPU |
| Input validation (Go probe) | Every frame read from the throwaway TUN fd bounds-checks every header offset (`ipv6`, `srh`, `inner ipv4`, `ihl`, `udp`, `marker`) before slicing and ignores unsolicited frames |
| Resource exhaustion | Memoized single probe per process, bounded by 1-second deadline and immediate sentinel termination; throwaway namespace thread exits cleanly |
| Fail-open | `unknown` and `denied` are never cached and never reported as `present`; `srv6ForwardFragPatched` fails on `unknown`/`denied` and strictly requires `present` on `-ze` kernels |

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

- Verified at Linux v7.3 `net/ipv6/seg6_local.c`: `seg6_local_build_state` sets `LWTUNNEL_STATE_INPUT_REDIRECT`, so Linux's own SRv6 egress actions (`End.DT4`, `End.DX4`, `End.DT46`) run `decap_and_validate` before `ip6_local_deliver` / `ipv6_frag_rcv` and cannot reassemble outer IPv6 fragments. Inner IPv4 fragmentation before encapsulation is therefore required for Linux SRv6 interoperability as well as anycast SIDs and hardware ASICs.
- Verified at Linux v7.3 `net/ipv4/ip_output.c`: `ip_do_fragment` is `EXPORT_SYMBOL`, computes `ip_skb_dst_mtu(sk, skb)` (which already subtracts `lwtunnel_headroom`), checks `mtu < hlen + 8`, handles `CHECKSUM_PARTIAL`, `ip_options_fragment`, `frag_max_size`, and `frag_list` (copying `skb_dst` to `iter.frag` before calling `output`), making direct reuse from `seg6_output_core` clean and minimal.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `kernel-refuses-what-the-installer-sends.md` | 2026-11-22 | SRv6 headend DF-clear IPv4 > MTU minus headroom (`seg6_iptunnel.c`, `lwtunnel_output`) |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Pre-encapsulation inner IPv4 fragmentation via `ip_do_fragment` in `seg6_output_core` | (a) Post-encapsulation outer IPv6 fragmentation (`skb->ignore_df = 1`); (b) Switching seg6 L3 modes to `LWTUNNEL_STATE_XMIT_REDIRECT` | Outer IPv6 fragments are dropped by Linux's own `seg6_local` (`LWTUNNEL_STATE_INPUT_REDIRECT` bypasses IPv6 reassembly) and break anycast SIDs; `XMIT_REDIRECT` alters `nf_hooks_lwtunnel_enabled`, cgroup BPF, and IPv6-inner paths. Calling `ip_do_fragment` from `seg6_output_core` reuses the kernel's exact IPv4 fragmentation engine and emits standard unfragmented IPv6+SRH packets |
| Separate patch `0015-seg6-forward-df-clear-frag.patch` with marker `CONFIG_IPV6_SEG6_FORWARD_FRAG` | Extending `0014-seg6-forward-ttl.patch` in place | Keeps patch 0014 single-purpose and independently revertible, and lets `runtime.require` and `ze doctor` distinguish a 0014-only kernel from a 0015 kernel |
| Extend the shared SRv6 TUN probe with 1460B DF-clear + 44B sentinel packet and enroll degrading `srv6-forward-frag` | No runtime probe / no doctor warning | Gives operators visibility on stock/0014-only kernels and gives integration tests an accurate runtime probe; sentinel avoids any 1-second timeout penalty on unpatched kernels and reuses the existing namespace, TUN fd, and AppArmor profile |

## Known Limitations

- Split-interface or reduced underlay PMTU (`orig_dst->dev->mtu > dst_mtu(outer_dst)`, e.g. a seg6 route pinned to `dev lo` with MTU 65536 pointing to a SID over a 1500-byte interface, or an underlay link whose learned IPv6 PMTU shrinks below `orig_dst->dev->mtu`): `ip_skb_dst_mtu(sk, skb)` reflects `orig_dst`, matching `ip_forward`'s DF-set PMTU check. Ze installs SRv6 routes via the underlay next-hop device (`buildRichRoute`), where `orig_dst->dev` is the egress link.
- Forwarded IPv6 packets `<= 1280` bytes crossing an egress link whose MTU minus SRv6 headroom falls below 1280 bytes (RFC 2473 Section 7.1(b)): IPv6 routers cannot fragment the inner IPv6 packet, and outer IPv6 fragmentation is incompatible with Linux `seg6_local`; SRv6 underlay links must carry MTU `>= 1280 + SRH_headroom`.
- Stock Linux kernels (and 0014-only kernels) without patch 0015 continue to drop oversized DF-clear IPv4 packets; Ze surfaces the gap via `doctor-srv6-forward-frag-unavailable` and documents it in `LL-SEG6-HEADEND-NO-DF-CLEAR-FRAG`.

## RFC Documentation (Scope: protocol)

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| RFC 791 Section 3.1 / RFC 1812 Section 4.2.2.7 (IPv4 fragmentation of DF-clear datagrams) & RFC 8986 Section 5.1 H.Encaps S05 | Default `fib { kernel { } }` with BGP SRv6 Service SID route: forwarded DF-clear IPv4 packets exceeding effective MTU (`mtu - seg6_headroom`) decrement inner TTL once, fragment into valid 8-byte-aligned inner IPv4 fragments (preserving options on fragment 1, omitting non-copied options on fragment 2+), and encapsulate each fragment in an unfragmented IPv6+SRH packet `<=` egress MTU | DF-set IPv4 continues returning ICMP Fragmentation Needed (MTU minus headroom); explicit route `PathMTU` (`RTAX_MTU`) continues bounding effective MTU | `TestFIBServiceSIDOversizedDFClearFragmentsInnerIPv4`, `TestFIBServiceSIDOversizedDFClearPreservesIPv4Options`, `TestFIBServiceSIDOversizedDFClearSubQuantumDropsCleanly` | `TestFIBServiceSIDOversizedDFAnswersFragmentationNeeded`, `TestFIBServiceSIDEncapsulationOnWire`, `TestFIBServiceSIDEncapsulationIPv6InnerOnWire` |

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented
- **Kernel patch and build manifest:** created `gokrazy/kernel/patches/0015-seg6-forward-df-clear-frag.patch` against Linux v7.3 (after `0014-seg6-forward-ttl.patch`), defining `CONFIG_IPV6_SEG6_FORWARD_FRAG` (`def_bool y`, `depends on IPV6_SEG6_LWTUNNEL && IPV6_SEG6_FORWARD_TTL`), extracting `seg6_output_encap`, and adding `seg6_output_ipv4_fragment` and `seg6_output_ipv4_gso` in `net/ipv6/seg6_iptunnel.c` to fragment oversized DF-clear IPv4 packets (and oversized DF-clear IPv4 GSO segments) at the inner IPv4 layer via `ip_do_fragment(net, sk, skb, seg6_output_encap)` before SRv6 encapsulation. Appended the patch to `gokrazy/kernel/patches/series`, enabled `CONFIG_IPV6_SEG6_FORWARD_FRAG=y` in `gokrazy/kernel/runtime.config`, and pinned `CONFIG_IPV6_SEG6_FORWARD_FRAG` in `gokrazy/kernel/runtime.require`.
- **Shared SRv6 behavioural probe:** extended `internal/component/kernelcap/probe_srv6_linux.go` (`SRv6ForwardFrag`, `runSRv6ProbeMemoized`, `isDefinitiveSRv6Verdict`, `askSRv6InThrowawayNamespace`, `buildSRv6FragProbePacket`, `pollSRv6FragProbe`, `extractSRv6InnerIPv4Fragment`) to inject a 1460-byte DF-clear IPv4 UDP probe packet plus a 44-byte sentinel packet on the existing throwaway-namespace TUN fd and memoize the 3-tuple `(encap, forwardTTL, forwardFrag)` only on definitive (`present`/`absent`) verdicts. Added the non-Linux stub `SRv6ForwardFrag` in `internal/component/kernelcap/probe_other.go`.
- **Kernel capability enrolment and doctor codes:** enrolled degrading capability `srv6ForwardFragCapability` (`srv6-forward-frag`, `CONFIG_IPV6_SEG6_FORWARD_FRAG`, order 742, `Degrades`) in `internal/plugins/fib/kernel/kernelcap_linux.go`, and registered `doctor-srv6-forward-frag-unavailable` and `doctor-srv6-forward-frag-unknown` in `internal/plugins/fib/kernel/register_doctor_codes.go`.
- **Tests:** added `TestSRv6ProbeDFClearFragClassification` and extended `TestSRv6ProbeMemoizesDefinitiveVerdict` and `TestSRv6ProbeClassification` in `internal/component/kernelcap/probe_srv6_linux_test.go`; extended `TestSRv6CapabilityCodesRegistered` and `TestSRv6CapabilitiesDegrade` in `internal/plugins/fib/kernel/kernelcap_codes_linux_test.go`; extended `TestSRv6Integration_ProbeAnswersFromAFreshNamespace` in `internal/plugins/fib/kernel/srv6probe_integration_linux_test.go`; added `srv6ForwardFragPatched`, `TestFIBServiceSIDOversizedDFClearFragmentsInnerIPv4`, `TestFIBServiceSIDOversizedDFClearPreservesIPv4Options`, and `TestFIBServiceSIDOversizedDFClearSubQuantumDropsCleanly` in `internal/plugins/fib/kernel/srv6_integration_linux_test.go`; and extended `test/ui/doctor-srv6-kernelcap.ci`.
- **Operator news fragment:** created `website/changes/fragments/srv6-df-clear-fragmentation.md`.

### Bugs Found/Fixed
- **GSO segmentation with `skb->dev == NULL` on local output:** `seg6_output_ipv4_gso` calls `skb_gso_segment(skb, 0)` rather than `netif_skb_features(skb)` because locally originated packets reaching `lwtunnel_output()` bypass `ip_output()` so `skb->dev` may be `NULL`, matching `0002-mpls-ip-mtu.patch`.
- **Closure documentation/journal/citer fixes:** added missing `srv6-forward-frag` to the source anchor at `docs/architecture/doctor-and-health-checks.md:472`, shortened the 2026-11-22 row in `plan/journal/kernel-refuses-what-the-installer-sends.md:13` from 698 chars to under the 600-char cap (`./le spec journal check` valid), and cleared the closing spec path citation in `test/ui/doctor-srv6-kernelcap.ci:2` to bare stems.

### Documentation Updates
- `docs/architecture/doctor-and-health-checks.md` -- added `srv6-forward-frag` (`CONFIG_IPV6_SEG6_FORWARD_FRAG`, `kernelcap.SRv6ForwardFrag`) to the SRv6 capability enrolment and memoization paragraphs and updated source anchors (`kernelcap_linux.go` lines 472 and 553, `probe_srv6_linux.go` lines 555 and 600).
- `docs/architecture/fib/fib-depth-4-srv6.md` -- documented patch `0015-seg6-forward-df-clear-frag.patch`, inner IPv4 pre-encapsulation fragmentation via `ip_do_fragment`, split-interface MTU boundary, `srv6-forward-frag` enrolment, and `doctor-srv6-forward-frag-*` codes with source anchors (`runtime.require`, `0015-seg6-forward-df-clear-frag.patch`, `kernelcap_linux.go`, `register_doctor_codes.go`, `probe_srv6_linux.go`).
- `docs/architecture/linux-limitations.md` -- added verified entry `LL-SEG6-HEADEND-NO-DF-CLEAR-FRAG` (`Checked: Linux 7.3, 2026-11-22`) with 5 source anchors (`0015-seg6-forward-df-clear-frag.patch`, `runtime.require`, `kernelcap_linux.go`, `register_doctor_codes.go`, `probe_srv6_linux.go`).
- `docs/features/srv6.md` -- added patch 0015, `LL-SEG6-HEADEND-NO-DF-CLEAR-FRAG`, `doctor-srv6-forward-frag-unavailable`, and source anchors in the Linux Kernel section.
- `plan/journal/kernel-refuses-what-the-installer-sends.md` -- marked the 2026-11-22 `spec-srv6-df-clear-fragmentation` row `fixed:`.
- `./le doc check all`: passed (`Documentation tests PASSED`) before a concurrent session's unrelated edit in `internal/component/l2tp/ppp/kernel/` broke `cmd/ze` compilation during the second run; source anchors, links, and journal check all pass on our files.

### Deviations from Plan
- `seg6_output_ipv4_gso` passes `features = 0` to `skb_gso_segment(skb, 0)` instead of `netif_skb_features(skb) & ~NETIF_F_GSO_MASK` (as sketched in Data Flow step 3) because `skb->dev` can be `NULL` for locally originated packets entering `lwtunnel_output()` and holds the ingress device for forwarded packets, matching `0002-mpls-ip-mtu.patch`.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Phase 1 updated lines 536-600 of `docs/architecture/doctor-and-health-checks.md` but left the earlier source anchor on line 472 listing only `mpls, mpls-transit-mtu, srv6-encap, srv6-forward-ttl` | Line 472 also anchors `internal/plugins/fib/kernel/kernelcap_linux.go` and needed `srv6-forward-frag` | Closure step 4 grep of `docs/` for `source: .*kernelcap_linux` | Updated line 472 in `docs/architecture/doctor-and-health-checks.md` |
| approach | Phase 4 updated the 2026-11-22 row in `plan/journal/kernel-refuses-what-the-installer-sends.md` to 698 characters | `./le spec journal check` enforces a 600-character row cap | Closure pre-commit journal check (`./le spec journal check`) | Shortened row 13 to fit under 600 characters while preserving all proof facts |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Fragment oversized DF-clear IPv4 packets before SRv6 encapsulation on Ze's runtime kernel | Done | `gokrazy/kernel/patches/0015-seg6-forward-df-clear-frag.patch` (`seg6_output_core`, `seg6_output_ipv4_fragment`, `seg6_output_ipv4_gso`, `seg6_output_encap`) | Applies `--fuzz=0` on Linux v7.3; `CONFIG_IPV6_SEG6_FORWARD_FRAG=y` |
| Emit unfragmented outer IPv6+SRH packets within egress MTU carrying valid inner IPv4 fragments | Done | `gokrazy/kernel/patches/0015-seg6-forward-df-clear-frag.patch` (`ip_do_fragment(..., seg6_output_encap)`) | Proven 1500B+85B (1437B) and 1500B+108B (1460B) with exact reassembly in QEMU |
| Preserve RFC 791 IPv4 options across fragments and guard sub-quantum MTU without livelock | Done | `gokrazy/kernel/patches/0015-seg6-forward-df-clear-frag.patch` (`ip_do_fragment`) | Proven by `TestFIBServiceSIDOversizedDFClearPreservesIPv4Options` and `TestFIBServiceSIDOversizedDFClearSubQuantumDropsCleanly` |
| Extend shared SRv6 throwaway-namespace TUN probe and enroll degrading `srv6-forward-frag` in `ze doctor` | Done | `internal/component/kernelcap/probe_srv6_linux.go` (`SRv6ForwardFrag`, `pollSRv6FragProbe`), `internal/plugins/fib/kernel/kernelcap_linux.go` (`srv6ForwardFragCapability`), `register_doctor_codes.go` | Memoizes definitive verdicts; warns at exit 0, never refuses startup |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestFIBServiceSIDOversizedDFClearFragmentsInnerIPv4` (`internal/plugins/fib/kernel/srv6_integration_linux_test.go:548`) | 1436B exact fit (1500B), 1437B (1500B+85B), 1460B (1500B+108B) PASS on 0015-patched 7.3.0-ze; RED on 0014-only 7.3.0-ze |
| AC-2 | Done | `TestFIBServiceSIDOversizedDFClearPreservesIPv4Options` (`internal/plugins/fib/kernel/srv6_integration_linux_test.go:630`) | Router Alert + Record Route updated on frag 1, Record Route replaced by `IPOPT_NOOP` on frag 2 (1496B+124B) PASS on 0015-patched 7.3.0-ze; RED on 0014-only |
| AC-3 | Done | `TestFIBServiceSIDOversizedDFClearSubQuantumDropsCleanly` (`internal/plugins/fib/kernel/srv6_integration_linux_test.go:689`) | `PathMTU=84` (20B < 28B) drops cleanly without livelock, subsequent marker packet delivered intact; PASS on 0015-patched, RED on 0014-only |
| AC-4 | Done | `TestFIBServiceSIDEncapsulationOnWire`, `TestFIBServiceSIDEncapsulationIPv6InnerOnWire`, `TestFIBServiceSIDExpiringTTLAnswersTimeExceeded`, `TestFIBServiceSIDExpiringHopLimitAnswersTimeExceeded`, `TestFIBServiceSIDOversizedDFAnswersFragmentationNeeded` | All 5 existing datapath tests PASS unchanged on 0015-patched 7.3.0-ze (A-8 log now shows `unfragmented-srv6=2 frame-lengths=[1500 108]`) |
| AC-5 | Done | `TestSRv6Integration_ProbeAnswersFromAFreshNamespace` (`internal/plugins/fib/kernel/srv6probe_integration_linux_test.go:37`), `internal/component/kernelcap/probe_other.go:50` | `StatePresent` on 0015-patched 7.3.0-ze; `StateAbsent` (sentinel-terminated) on 0014-only 7.3.0-ze; `StateUnknown` (`errNotLinux`) off Linux |
| AC-6 | Done | `test/ui/doctor-srv6-kernelcap.ci`, `TestSRv6CapabilityCodesRegistered`, `TestSRv6CapabilitiesDegrade` (`internal/plugins/fib/kernel/kernelcap_codes_linux_test.go:66,110`) | Exits 0 (WARNING), reports `doctor-srv6-forward-frag-unavailable` naming `CONFIG_IPV6_SEG6_FORWARD_FRAG` and `LL-SEG6-HEADEND-NO-DF-CLEAR-FRAG`, `kernelcap.Refuse` returns nil, omitted without BGP |
| AC-7 | Done | Runtime kernel build (`7.3-runtime-amd64-runtime-qemu-6db1b4aa-5d7f6651`), `TestEnrolledKernelSymbolsAreInRuntimeRequire` (`internal/appliance/kernelreq_enrolled_linux_test.go`) | Patch 0015 applied `--fuzz=0`, `CC net/ipv6/seg6_iptunnel.o` zero warnings, `CONFIG_IPV6_SEG6_FORWARD_FRAG=y` resolved and pinned |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestSRv6ProbeDFClearFragClassification` | Done | `internal/component/kernelcap/probe_srv6_linux_test.go:668` | Exact name; passes in `./le test unit pkg path ./internal/component/kernelcap/` |
| `TestSRv6ProbeMemoizesDefinitiveVerdict` | Done | `internal/component/kernelcap/probe_srv6_linux_test.go:439` | Extended for 3-tuple memoization |
| `TestSRv6CapabilityCodesRegistered` | Done | `internal/plugins/fib/kernel/kernelcap_codes_linux_test.go:66` | Extended for `srv6-forward-frag` and both codes |
| `TestSRv6CapabilitiesDegrade` | Done | `internal/plugins/fib/kernel/kernelcap_codes_linux_test.go:110` | Extended for `srv6-forward-frag` |
| `TestEnrolledKernelSymbolsAreInRuntimeRequire` | Done | `internal/appliance/kernelreq_enrolled_linux_test.go` | Passes with `CONFIG_IPV6_SEG6_FORWARD_FRAG` pinned |
| `doctor-srv6-kernelcap` | Done | `test/ui/doctor-srv6-kernelcap.ci` | Passes `./le test ui doctor-srv6-kernelcap` (re-verify after comment edit in owed commands) |
| `TestFIBServiceSIDOversizedDFClearFragmentsInnerIPv4` | Done | `internal/plugins/fib/kernel/srv6_integration_linux_test.go:548` | RED on 0014-only, PASS (1.03s) on 0015-patched 7.3.0-ze |
| `TestFIBServiceSIDOversizedDFClearPreservesIPv4Options` | Done | `internal/plugins/fib/kernel/srv6_integration_linux_test.go:630` | RED on 0014-only, PASS (0.36s) on 0015-patched 7.3.0-ze |
| `TestFIBServiceSIDOversizedDFClearSubQuantumDropsCleanly` | Done | `internal/plugins/fib/kernel/srv6_integration_linux_test.go:689` | RED on 0014-only, PASS (0.33s) on 0015-patched 7.3.0-ze |
| `TestSRv6Integration_ProbeAnswersFromAFreshNamespace` | Done | `internal/plugins/fib/kernel/srv6probe_integration_linux_test.go:37` | RED on 0014-only (`absent`), PASS (0.00s) on 0015-patched 7.3.0-ze (`present`) |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `gokrazy/kernel/patches/0015-seg6-forward-df-clear-frag.patch` | Done | Created; 171 lines; applies `--fuzz=0` |
| `gokrazy/kernel/patches/series` | Done | Appended line 23 |
| `gokrazy/kernel/runtime.config` | Done | Added `CONFIG_IPV6_SEG6_FORWARD_FRAG=y` |
| `gokrazy/kernel/runtime.require` | Done | Pinned `CONFIG_IPV6_SEG6_FORWARD_FRAG` |
| `internal/component/kernelcap/probe_srv6_linux.go` | Done | Extended TUN probe, classifier, and 3-tuple memoization |
| `internal/component/kernelcap/probe_other.go` | Done | Added `SRv6ForwardFrag()` non-Linux stub |
| `internal/component/kernelcap/probe_srv6_linux_test.go` | Done | Added `TestSRv6ProbeDFClearFragClassification` and updated existing tests |
| `internal/plugins/fib/kernel/kernelcap_linux.go` | Done | Enrolled `srv6ForwardFragCapability` |
| `internal/plugins/fib/kernel/register_doctor_codes.go` | Done | Registered both `doctor-srv6-forward-frag-*` codes |
| `internal/plugins/fib/kernel/kernelcap_codes_linux_test.go` | Done | Extended registration and degrading tests |
| `internal/plugins/fib/kernel/srv6_integration_linux_test.go` | Done | Added `srv6ForwardFragPatched` and 3 DF-clear tests; existing 5 tests untouched |
| `internal/plugins/fib/kernel/srv6probe_integration_linux_test.go` | Done | Extended for `SRv6ForwardFrag()` |
| `test/ui/doctor-srv6-kernelcap.ci` | Done | Extended forced assertions and cleared spec path citation |
| `docs/architecture/linux-limitations.md` | Done | Added `LL-SEG6-HEADEND-NO-DF-CLEAR-FRAG` |
| `docs/architecture/fib/fib-depth-4-srv6.md` | Done | Documented patch 0015 and `srv6-forward-frag` |
| `docs/architecture/doctor-and-health-checks.md` | Done | Updated SRv6 enrolment, memoization, and anchors |
| `docs/features/srv6.md` | Done | Updated Linux Kernel section and anchors |
| `plan/journal/kernel-refuses-what-the-installer-sends.md` | Done | Marked 2026-11-22 row `fixed:` (under 600 chars) |
| `website/changes/fragments/srv6-df-clear-fragmentation.md` | Done | Operator news fragment created for closure |

### Audit Summary
- **Total items:** 4 Task requirements + 7 ACs + 10 TDD tests + 19 planned/closure files = 40 items
- **Done:** 40
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (`seg6_output_ipv4_gso` uses `skb_gso_segment(skb, 0)`, recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Forwarded DF-clear IPv4 packet larger than seg6 effective MTU fragments at inner IPv4 layer into valid fragments, each in an unfragmented outer IPv6+SRH packet within egress MTU | QEMU kernel integration test (RED/GREEN discrimination) | RED on 0014-only `7.3.0-ze` (`vmlinuz` `b427ccda`): `A-8 ... unfragmented-srv6=0 ipv6-outer-fragments=0 icmp-errors=0`, 3 new DF-clear tests FAIL (`srv6-forward-frag absent`). GREEN on 0015-patched `7.3.0-ze` (`vmlinuz` `6db1b4aa`): 9/9 PASS (`TestFIBServiceSIDOversizedDFClearFragmentsInnerIPv4` 1.03s: 1436B->1500B, 1437B->[1500 85], 1460B->[1500 108] with exact reassembly; `TestFIBServiceSIDOversizedDFClearPreservesIPv4Options` 0.36s: [1496 124]; `TestFIBServiceSIDOversizedDFClearSubQuantumDropsCleanly` 0.33s) |
| Shared behavioural `kernelcap` probe reports stock/0014-only kernels in `ze doctor` as a warning and never refuses startup | QEMU namespace probe test + `.ci` functional test + unit tests | `TestSRv6Integration_ProbeAnswersFromAFreshNamespace`: FAIL (`srv6-forward-frag absent`, sentinel-terminated) on 0014-only 7.3.0-ze, PASS (`present`, memoized 812ns) on 0015-patched 7.3.0-ze; `test/ui/doctor-srv6-kernelcap.ci` PASS 1/1 (exit 0, `doctor-srv6-forward-frag-unavailable`); `TestSRv6CapabilitiesDegrade` PASS (`kernelcap.Refuse` nil) |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | All in-scope deliverables and acceptance criteria (AC-1..AC-7) completed. | none |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/srv6-df-clear-fragmentation-816634a8-9a83-434b-a892-75c183d9ed4e.md` |
| `./le spec review check` | `review_gate: OK (9 code files, clean, hashes match tmp/review/srv6-df-clear-fragmentation-816634a8-9a83-434b-a892-75c183d9ed4e.md)` |
| Rounds | 1 |
| Reviewer lenses used | logic+wiring, security+edge-cases, kernel-patch+style+docs+spec |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | NOTE (doc drift) | Source anchor on line 472 omitted `srv6-forward-frag` | `docs/architecture/doctor-and-health-checks.md:472` | Added `srv6-forward-frag` to anchor comment |
| 2 | NOTE (journal gate) | Updated journal row was 698 chars (cap 600) | `plan/journal/kernel-refuses-what-the-installer-sends.md:13` | Tightened wording under 600 chars (`./le spec journal check` valid) |
| 3 | NOTE (citer cleanup) | `# Spec:` header cited closing spec path | `test/ui/doctor-srv6-kernelcap.ci:2` | Replaced paths with bare stems (`spec-srv6-headend-decrements-inner-ttl`, `spec-srv6-df-clear-fragmentation`) |
| 4 | NOTE (weakening gate) | File-level `t.Skip` count rose 2 -> 5 from 3 new functions while all 5 existing `TestFIBServiceSID*` tests stayed unchanged | `internal/plugins/fib/kernel/srv6_integration_linux_test.go` | Independent `ze-test-judge` approval recorded (`./le test weakened approve unit kernel.srv6_integration_linux_test`, `tmp/commit-weakening-approved-c949ab01.md`) |

**Final clean run summary (Round 1):** 0 BLOCKER, 0 ISSUE in product code, kernel patch, and tests. Style pass (`docs/contributing/ze-go-style.md` 7 questions + `./le go style check path` on all 8 changed Go files: `go style: OK`): no peer-reachable `panic()`, all loops bounded (`srv6ProbeDeadline`, fixed 1440B payload), clean names and types, lifecycle obligations documented (`Safe for concurrent use`). Design documents lens (`./le spec design documents spec`): 7/7 derived pages covered, 0 decision rows violated.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `gokrazy/kernel/patches/0015-seg6-forward-df-clear-frag.patch` | Yes | `-rw-rw-r-- 1 thomas thomas 5683 Nov 22 17:39 gokrazy/kernel/patches/0015-seg6-forward-df-clear-frag.patch` |
| `test/ui/doctor-srv6-kernelcap.ci` | Yes | `-rw-rw-r-- 1 thomas thomas 3308 Nov 22 18:58 test/ui/doctor-srv6-kernelcap.ci` |
| `website/changes/fragments/srv6-df-clear-fragmentation.md` | Yes | `-rw-rw-r-- 1 thomas thomas 1321 Nov 22 18:58 website/changes/fragments/srv6-df-clear-fragmentation.md` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | 1436B exact fit, 1437B and 1460B DF-clear IPv4 fragment into unfragmented IPv6+SRH <= 1500B with exact reassembly | `TestFIBServiceSIDOversizedDFClearFragmentsInnerIPv4` PASS (1.03s) on 0015-patched 7.3.0-ze; FAIL on 0014-only |
| AC-2 | Copied Router Alert kept on frag 2, non-copied Record Route replaced with `IPOPT_NOOP` on frag 2 | `TestFIBServiceSIDOversizedDFClearPreservesIPv4Options` PASS (0.36s) on 0015-patched 7.3.0-ze; FAIL on 0014-only |
| AC-3 | Sub-quantum `PathMTU=84` drops cleanly without livelock and following marker packet arrives | `TestFIBServiceSIDOversizedDFClearSubQuantumDropsCleanly` PASS (0.33s) on 0015-patched 7.3.0-ze; FAIL on 0014-only |
| AC-4 | All 5 existing `TestFIBServiceSID*` tests pass unchanged | 5/5 PASS on 0015-patched 7.3.0-ze; `git diff HEAD -- internal/plugins/fib/kernel/srv6_integration_linux_test.go` confirms 0 lines changed in existing 5 tests |
| AC-5 | Throwaway-namespace probe returns `present` on 0015-patched, `absent` promptly on 0014-only, `unknown` off Linux | `TestSRv6Integration_ProbeAnswersFromAFreshNamespace` PASS (0.00s, `present`) on 0015-patched, FAIL (`absent`, sentinel-terminated) on 0014-only; `probe_other.go:50` returns `StateUnknown` (`errNotLinux`) |
| AC-6 | `ze doctor --json` exits 0 (WARNING) naming `doctor-srv6-forward-frag-unavailable`, `CONFIG_IPV6_SEG6_FORWARD_FRAG`, `LL-SEG6-HEADEND-NO-DF-CLEAR-FRAG`, omitted without BGP | `grep -n "srv6-forward-frag\|doctor-srv6-forward-frag-"` hits `kernelcap_linux.go:105` and `register_doctor_codes.go:16-17,110,117`; `TestSRv6CapabilityCodesRegistered`, `TestSRv6CapabilitiesDegrade`, and `test/ui/doctor-srv6-kernelcap.ci` PASS |
| AC-7 | Patch 0015 applies `--fuzz=0`, `CONFIG_IPV6_SEG6_FORWARD_FRAG=y` resolved and pinned | `grep` hits `series:23`, `runtime.config:552`, `runtime.require:126`; kernel build `6db1b4aa` exited 0; `TestEnrolledKernelSymbolsAreInRuntimeRequire` PASS |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze doctor --json config srv6.conf` (`fib { kernel { } }` + BGP peer) and `no-bgp.conf` | `test/ui/doctor-srv6-kernelcap.ci` | Yes -- read lines 1-107: forces `srv6-forward-frag=absent`, asserts exit 0 + `doctor-srv6-forward-frag-unavailable` + `CONFIG_IPV6_SEG6_FORWARD_FRAG` + `LL-SEG6-HEADEND-NO-DF-CLEAR-FRAG`, rejects `srv6-forward-frag` when BGP is absent |
| sysrib best-change with `SRv6SID` -> `buildSEG6Encap` -> `seg6_output_core` -> `ip_do_fragment` -> `seg6_output_encap` | `internal/plugins/fib/kernel/srv6_integration_linux_test.go` (`TestFIBServiceSIDOversizedDFClearFragmentsInnerIPv4`) | Yes -- QEMU integration test (kernel datapath requires booted 7.3.0-ze kernel) |
| `kernelcap.SRv6ForwardFrag()` in fresh netns with `ip_forward=0` | `internal/plugins/fib/kernel/srv6probe_integration_linux_test.go` (`TestSRv6Integration_ProbeAnswersFromAFreshNamespace`) | Yes -- QEMU integration test |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestFIBServiceSIDOversizedDFClearFragmentsInnerIPv4` and `TestFIBServiceSIDOversizedDFClearPreservesIPv4Options` PASS on 0015-patched 7.3.0-ze with zero refcount leaks or crashes across multi-fragment `ip_do_fragment` delivery |
| A-2 | confirmed | `./ze-host appliance kernel --builder qemu --target runtime --arch amd64 --profile runtime` exited 0 with `CC net/ipv6/seg6_iptunnel.o` zero warnings and `CONFIG_IPV6_SEG6_FORWARD_FRAG=y` |
| A-3 | confirmed | `TestSRv6Integration_ProbeAnswersFromAFreshNamespace` returned `absent` immediately via sentinel on 0014-only 7.3.0-ze and `present` (memoized 812ns) on 0015-patched 7.3.0-ze |
| A-4 | confirmed | `TestFIBServiceSIDOversizedDFClearFragmentsInnerIPv4` verified exact outer frame lengths `[1500 85]` (1437B) and `[1500 108]` (1460B), all `<= 1500` with zero outer IPv6 fragments |
| A-5 | confirmed | `TestFIBServiceSIDOversizedDFClearSubQuantumDropsCleanly` verified `PathMTU=84` drops 68B DF-clear packet cleanly and delivers the subsequent 44B marker packet intact |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Category 1 (Feature page): `docs/features/srv6.md` Linux Kernel section | Verified against `0015-seg6-forward-df-clear-frag.patch`, `runtime.config:552`, `runtime.require:126`, `kernelcap_linux.go:105`, `register_doctor_codes.go:107` | Yes |
| Category 12 (Architecture): `docs/architecture/fib/fib-depth-4-srv6.md`, `doctor-and-health-checks.md`, `linux-limitations.md` | Verified all paragraphs and `<!-- source: -->` anchors against `0015-seg6-forward-df-clear-frag.patch`, `probe_srv6_linux.go`, `kernelcap_linux.go`, `register_doctor_codes.go`; `grep -n LL-SEG6-HEADEND-NO-DF-CLEAR-FRAG docs/architecture/linux-limitations.md` hits line 807; `TestLinuxLimitsCheckedAtShippedVersions` PASS | Yes |
| Category 16 (Existing doc anchors): `git grep -n "source: .*probe_srv6_linux\|kernelcap_linux\|register_doctor_codes\|0015-seg6\|runtime.require"` across `docs/` | All anchored claims updated (`doctor-and-health-checks.md:472,553,555,600`, `fib-depth-4-srv6.md:48,97,115,116,118`, `linux-limitations.md:876-880`, `features/srv6.md:341-347`); `mpls-kernel.md:265` anchors only `transitMTUCapability` (unchanged) | Yes |
| Categories 2-11, 13-15, 17 (No / N-A) | No YANG, CLI, API/RPC, plugin transport, BGP wire format, RFC enrollment, or metric changes (`git diff --stat` covers only `gokrazy/kernel/`, `kernelcap`, `fib/kernel`, `test/ui/doctor-srv6-kernelcap.ci`, and docs) | Yes |

