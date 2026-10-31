# Spec: srv6-headend-decrements-inner-ttl

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P3 |
| Release reason | Kernel work: it patches and reconfigures the Linux kernel Ze runs, which Thomas placed after the first release |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-31 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

RFC 8986 Section 5.1, H.Encaps: "S05.   Decrement inner IPv6 Hop Limit or
IPv4 TTL", and "The received packet is encapsulated unmodified (with the
exception of the IPv4 TTL or IPv6 Hop Limit that is decremented as described in
[RFC2473])."

Ze programs a Service SID as a seg6 lwtunnel in encap mode (`buildSEG6Encap`,
`internal/plugins/fib/kernel/nexthop_linux.go`). Linux redirects input for
seg6 (`seg6_build_state` sets `LWTUNNEL_STATE_INPUT_REDIRECT`), so a forwarded
IPv4 packet reaches `seg6_input` and never `ip_forward`: the inner TTL is
neither checked nor decremented (`__seg6_do_srh_encap`, linux-7.3
`net/ipv6/seg6_iptunnel.c`). `TestFIBServiceSIDEncapsulationOnWire` read inner
TTL 0x40 for a sent 0x40 on the hosted runner (evidence-nightly run
41038783931).

Goal: a packet Ze's headend encapsulates leaves with its inner TTL or Hop Limit
decremented once, and one arriving with TTL 1 is not encapsulated, on Ze's
kernel; the test runs where that kernel runs. Thomas settled the design at the
design gate: Ze's kernel gains `CONFIG_IPV6_SEG6_LWTUNNEL`; patch 0003 gives the
seg6 L3 modes output redirect only, the MPLS lwtunnel model; a behavioural
`kernelcap` probe in the shape of `kernelcap.MPLSIPMTU` tells `ze doctor` what a
stock kernel does; and a refused install names the missing kernel feature (Key
Design Decisions).

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/fib/fib-depth-4-srv6.md` - "Kernel backend": how a Service SID becomes a seg6 encap on every ECMP member
  → Decision: the kernel backend owns the seg6 lwtunnel and selects it ahead of any label field; this spec changes no Ze route encoding.
  → Constraint: the page states nothing about the inner TTL or about which kernels carry seg6; the page gains that paragraph in this work.
- [ ] `docs/architecture/doctor-and-health-checks.md` - "Kernel capabilities: one enrolment, three callers"
  → Decision: a capability is enrolled once by its owning package (`kernelcap.MustRegister`) and answered by the startup gate (`kernelcap.Refuse` in `cmd/ze/hub/main.go`, reload in `cmd/ze/hub/main_reload.go`), `ze config validate` (`cmd_validate.go`) and `ze doctor` (`kernelcap.ProbeAll`, `internal/component/doctor/kernel_capabilities.go`).
  → Constraint: no result cache exists (`probe` in `internal/component/kernelcap/probe.go` calls `Capability.Probe` every time), so a probe runs at each start, reload, validate and doctor run whose config makes `InUse` true.
- [ ] `docs/architecture/linux-limitations.md` - entry LL-MPLS-FORWARD-NO-PMTU is the shape of the new entry
  → Constraint: an entry carries Kind, Effect, Status, Checked, Limitation, Blocks, Evidence (quoted kernel source), Re-check and "Ze meanwhile", plus a source anchor.
- [ ] `docs/architecture/mpls/mpls-kernel.md` - how patch 0002 and `CONFIG_MPLS_IP_MTU` are documented
  → Decision: a Ze kernel patch adds a marker Kconfig symbol that the runtime require manifest pins, so a build without the patch fails before the compile; runtime detection is a behavioural probe, never a read of the kernel config.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9252.md` - the SRv6 service routes whose SID Ze installs as H.Encaps
  → Constraint: RFC 9252 sends the forwarding behaviour to RFC 8986; no `rfc/short/rfc8986.md` exists.
- [ ] `rfc/full/rfc8986.txt` Section 5.1 (read in full; no summary exists)
  → Constraint: "S05.   Decrement inner IPv6 Hop Limit or IPv4 TTL" and "The received packet is encapsulated unmodified (with the exception of the IPv4 TTL or IPv6 Hop Limit that is decremented as described in [RFC2473])."
  → Constraint: S03 reads "Set outer Payload Length, Traffic Class, Hop Limit, and Flow Label fields" with the note "S03: As described in [RFC2473] and [RFC6437]." S03 fixes no outer Hop Limit value, so the outer value stays a RFC 2473 matter.
- [ ] `rfc/full/rfc2473.txt` Section 6.3
  → Constraint: "The "single-hop" mechanism should be implemented by having the tunnel entry point node set a tunnel IPv6 header hop limit independently of the hop limit of the original header." Lowercase should: a Known Limitation here, not work.
- [ ] RFC profiles: `grep -rl 8986 rfc/profiles/` returns nothing, and RFC 8986 is in neither `rfc/enrolled.txt` nor `rfc/not-enrolled.txt`.
  → Decision: no release profile lists RFC 8986, so no summary is written (`ai/META-DECISIONS.md`, "RFC validation scope does not grow"). The comments in the patch and in Ze quote `rfc/full/rfc8986.txt` directly.

**Key insights:** (minimal context to resume after compaction)
- Ze's own runtime kernel has no seg6 at all: the arm64 runtime build config (`tmp/kernel/build/config`, 7.3) reads `# CONFIG_IPV6_SEG6_LWTUNNEL is not set`, and no `SEG6` symbol appears in `gokrazy/kernel/` or `kernelcap`. H.Encaps cannot be installed on Ze's kernel today, so the TTL test only ever ran on the stock Ubuntu runner kernel.
  → Constraint: this spec must enable `CONFIG_IPV6_SEG6_LWTUNNEL` in the runtime config and require manifest before any patch has anything to fix.
- Cause, read in linux-7.3 `net/ipv6/seg6_iptunnel.c`: `seg6_build_state` sets `LWTUNNEL_STATE_INPUT_REDIRECT` for every mode and `OUTPUT_REDIRECT` for every non-L2 mode. Input redirect replaces `ip_forward` with `lwtunnel_input`, so `seg6_input` -> `seg6_input_core` -> `seg6_do_srh` -> `__seg6_do_srh_encap` runs and nothing checks or decrements the inner TTL. IPv6 inner: `__seg6_do_srh_encap` copies the outer hop limit from the inner and only the outer is decremented by `ip6_forward`; its comment claiming decapsulation overwrites the inner hop limit is false (`seg6_local.c` has no hop-limit code). IPv4 inner: outer hop limit from `ip6_dst_hoplimit`, inner TTL untouched, TTL 1 forwarded.
- MPLS lwtunnel sets only `XMIT_REDIRECT` (`mpls_build_state`, `net/mpls/mpls_iptunnel.c`), so forwarded traffic passes `ip_forward` first.
  → Decision (proposed): for the L3 modes (encap, encap.red, inline) drop `INPUT_REDIRECT` and keep `OUTPUT_REDIRECT`, so forwarded packets go through `ip_forward`/`ip6_forward` (one decrement, Time Exceeded to the real sender, FORWARD hook) and then `lwtunnel_output` -> `seg6_output`. L2 modes keep input redirect (no TTL in an Ethernet frame). Upstream master has no change here (fetched 2026-10-30).
- RFC 8986 Section 4: "An implementation of the pseudocode is compliant as long as the externally observable wire protocol is as described by the pseudocode." Section 5.1 S05 "Decrement inner IPv6 Hop Limit or IPv4 TTL". Section 5.2 is an optimisation of 5.1 and inherits S05. RFC 2473 Section 3.1 ties the decrement to forwarding, so locally originated packets owe none.
- FRR (`zebra/rt_netlink.c` `fill_seg6ipt_encap`) relies entirely on the kernel and has the same defect; BIRD 3.1.9 installs no seg6 encap; freeRtr encapsulates before its TTL decrement; VPP decrements the inner header itself. No implementation offers a fix to copy.
- Precedent for the patch, probe and doctor shape: `gokrazy/kernel/patches/0002-mpls-ip-mtu.patch` (+ `series`, applied by `applyPatches` in `internal/appliance/kernelbuilder/worker.go`), Kconfig symbol `CONFIG_MPLS_IP_MTU` in `runtime.require`/`runtime.config`, the `kernelcap.Capability` enrolment in `internal/plugins/fib/kernel/kernelcap_linux.go`, the probe `kernelcap.MPLSIPMTU` in `internal/component/kernelcap/probe_linux.go`, diagnostic codes in `internal/core/diagnostic/codes.go`, limitation entry LL-MPLS-FORWARD-NO-PMTU in `docs/architecture/linux-limitations.md`.
- The test runs on the stock kernel in `evidence-nightly.yml` job `integration`, and on Ze's amd64 kernel in `qemu-nightly.yml` (the qemu all-tests list includes `./internal/plugins/fib/kernel/...`). It has no IPv6-inner variant and no TTL 1 case. Ze's arm64 kernel builds natively on the Mac with `./ze-host appliance kernel --target runtime --arch arm64`.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/plugins/fib/kernel/nexthop_linux.go` - `buildRouteEncap` gives the Service SID precedence and `buildSEG6Encap` installs a seg6 encap-mode lwtunnel
- [ ] `internal/plugins/fib/kernel/srv6_integration_linux_test.go` - `TestFIBServiceSIDEncapsulationOnWire` asserts the inner TTL is decremented once
- [ ] `internal/plugins/fib/kernel/fibkernel.go` - `addChange` / `replaceChange` reach `addRichRoute` / `replaceRichRoute`; on error `processEvent` logs `fib-kernel: add route failed` and raises `reportCodeFIBSyncFailure` with the text "FIB add failed: " plus the netlink error
- [ ] `internal/plugins/fib/kernel/kernelcap_linux.go` - the `mpls` and `mpls-transit-mtu` enrolments (`transitMTUCapability`, `Degrades` set)
- [ ] `internal/component/kernelcap/kernelcap.go` - `Capability`, `Evaluate`, `evaluateOne` (message "the kernel holds no <Kernel>, which <ConfigLeaf> requires"), `ProbeAll`, `Refuse`, `KernelSymbols`
- [ ] `internal/component/kernelcap/probe_linux.go` - `MPLSIPMTU`, `askMPLSIPMTUInThrowawayNamespace`, `remountSysctlWritable`, `classifyMPLSIPMTU`, `stepFailed`
- [ ] `internal/component/kernelcap/predicate.go` - `MPLSInUse` (`fib { kernel { } }` first, then the config shape)
- [ ] `internal/plugins/fib/kernel/mplsipmtuprobe_integration_linux_test.go` - `TestMPLSIntegration_ProbeAnswersFromAFreshNamespace` wants present on a `-ze` release, absent elsewhere
- [ ] `internal/plugins/fib/kernel/mplstransitmtu_integration_linux_test.go` - `TestMPLSIntegration_TransitPathMTUFollowsTheProbe` branches its expectation on the probe
- [ ] `internal/appliance/kernelreq_enrolled_linux_test.go` - `TestEnrolledKernelSymbolsAreInRuntimeRequire` holds every enrolled `Kernel` symbol to `gokrazy/kernel/runtime.require`
- [ ] `test/ui/doctor-l2tp-kernelcap.ci` - forced answers through `ze.test.kernelcap.force` reach `ze doctor --json`
- [ ] `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require`, `gokrazy/kernel/patches/series`, `gokrazy/kernel/patches/0002-mpls-ip-mtu.patch` - the MPLS block, the require pins, the patch header
- [ ] `tmp/kernel-7.3-dl/linux-7.3/net/ipv6/seg6_iptunnel.c` - `seg6_build_state`, `seg6_input_core`, `seg6_input_route`, `seg6_output_core`, `seg6_output_dst_lookup`, `__seg6_do_srh_encap`, `__seg6_do_srh_inline`
- [ ] `tmp/kernel-7.3-dl/linux-7.3/include/net/lwtunnel.h` - `lwtunnel_set_redirect`; `net/core/lwtunnel.c` `lwtunnel_output`, `lwtunnel_valid_encap_type`
- [ ] `tmp/kernel-7.3-dl/linux-7.3/net/ipv4/ip_forward.c` and `net/ipv6/ip6_output.c` - `ip_forward`, `ip6_forward`: TTL check and ICMP, MTU check with `ip_dst_mtu_maybe_forward` / `ip6_dst_mtu_maybe_forward`, FORWARD hook, XFRM forward policy

**Behavior to preserve:** (unless the user explicitly said to change it)
- Ze's route encoding: `buildRouteEncap` and `buildSEG6Encap` send the same netlink request (encap mode, one segment, no table, no source).
- `TestFIBServiceSIDEncapsulationOnWire` keeps every assertion: SRH present, next header IPIP, inner TTL sent minus one, inner addresses and payload unchanged, withdrawal removes the route.
- The seg6 L2 modes (`l2encap`, `l2encap.red`) keep input redirect: an Ethernet frame has no TTL.
- Locally originated traffic over a seg6 route already takes `seg6_output` and gets no decrement (RFC 2473 Section 3.1 ties the decrement to forwarding).
- The SRH layout, outer source selection (`set_tun_src`) and the IPv6-inner outer Hop Limit value: before the patch the outer copies the inner value H and `ip6_forward` lowers the outer to H-1; after it the inner is lowered to H-1 first and the outer copies H-1.

**Behavior to change:** (only what the user asked for)
- On Ze's kernel, a forwarded IPv4 or IPv6 packet that matches a seg6 encap route leaves with its inner TTL or Hop Limit lowered by one, and one that arrives with TTL or Hop Limit 1 is not encapsulated: the sender gets ICMP Time Exceeded.
- Consequences of the forward path that come with it, each asserted or recorded: the FORWARD netfilter hook and the XFRM forward policy now see the inner packet; a DF-set IPv4 packet (or any IPv6 packet) larger than the route MTU minus the seg6 headroom is answered with ICMP Fragmentation Needed or Packet Too Big to the real sender; the outer Hop Limit of an IPv4-inner packet is `ip6_dst_hoplimit` of the IPv4 route without the old extra decrement (64, was 63); the first-SID lookup moves from `ip6_route_input` (with the ingress interface) to `ip6_route_output` (no interface).
- Ze's runtime kernel gains `CONFIG_IPV6_SEG6_LWTUNNEL` (today absent, so no Service SID installs on Ze's kernel).
- A kernel without seg6, or with seg6 but without patch 0003, is named by `ze doctor`, the startup gate, `ze config validate`, and the FIB install error.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Install: a BGP RFC 9252 service route with a Service SID reaches the fib kernel plugin, which installs a route with a seg6 encap-mode lwtunnel (`buildSEG6Encap`, `internal/plugins/fib/kernel/nexthop_linux.go`) over netlink.
- Datapath: an IPv4 or IPv6 packet received on any interface whose destination matches that route; inner TTL / Hop Limit as received.
- Detection: `ze` start, reload, `ze config validate` and `ze doctor` with a config that holds `fib { kernel { } }` and a `bgp` peer or group; `ze doctor kernel-capabilities` with no config.

### Transformation Path
1. Install (unchanged): sysrib best change with `SRv6SID` -> `fibKernel.processEvent` -> `addChange` -> `addRichRoute` -> `buildRichRoute` -> `buildRouteEncap` -> `buildSEG6Encap` -> `RTM_NEWROUTE` with `RTA_ENCAP_TYPE` SEG6.
2. Kernel route build: `lwtunnel_build_state` -> `seg6_build_state`. Patched: L3 modes set `LWTUNNEL_STATE_OUTPUT_REDIRECT` only; `lwtunnel_set_redirect` then replaces `dst->output` with `lwtunnel_output` and leaves `dst->input` at `ip_forward` / `ip6_forward`.
3. Datapath, patched: `ip_rcv` / `ipv6_rcv` -> route input -> `ip_forward` / `ip6_forward` (XFRM forward policy, TTL or Hop Limit check with ICMP Time Exceeded, MTU check that subtracts `lwtunnel_headroom`, decrement, FORWARD hook) -> `dst_output` -> `lwtunnel_output` -> `seg6_output` (POST_ROUTING on the inner when `nf_hooks_lwtunnel` is on) -> `seg6_output_core` -> `seg6_do_srh` -> `__seg6_do_srh_encap` -> first-SID lookup `seg6_output_dst_lookup` (`ip6_route_output`, or `seg6_table_lookup` when a table is set) -> LOCAL_OUT on the outer when `nf_hooks_lwtunnel` is on -> `dst_output` -> `ip6_output`.
4. Datapath, unpatched (today): route input -> `lwtunnel_input` -> `seg6_input` -> `seg6_input_core` -> encap with the inner untouched -> `seg6_input_route` (`ip6_route_input`) -> `dst_input` -> `ip6_forward` on the outer only.
5. Detection: `kernelcap.Evaluate` -> `SRv6InUse` -> `probe` -> `kernelcap.SRv6Encap` / `kernelcap.SRv6ForwardTTL` -> one shared behavioural probe in a throwaway network namespace -> `evaluateOne` -> diagnostic with the new codes.
6. Install refusal: `addRichRoute` / `replaceRichRoute` -> netlink `EOPNOTSUPP` (`lwtunnel_valid_encap_type`, extack "lwt encapsulation type not supported") -> wrapped error naming the kernel feature and the fix -> `processEvent` log and `report.RaiseError`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin -> kernel (route) | rtnetlink `RTM_NEWROUTE`, `RTA_ENCAP_TYPE` = `LWTUNNEL_ENCAP_SEG6`, `SEG6_IPTUNNEL_SRH` in encap mode | No |
| Wire -> kernel datapath | IPv4 or IPv6 packet in; IPv6 + SRH + inner packet out, or ICMP back to the sender | No |
| Component -> kernel (probe) | throwaway network and mount namespace, sysctl writes, rtnetlink, one packet through a TUN device | No |
| Kernel builder -> kernel tree | `applyPatches` (`internal/appliance/kernelbuilder/worker.go`) applies `series`; the require manifest is checked against the resolved `.config` before the compile | No |

### Integration Points
- `kernelcap.MustRegister` from `internal/plugins/fib/kernel/kernelcap_linux.go` - two new `kernelcap.Capability` values beside `transitMTUCapability`.
- `internal/component/kernelcap/predicate.go` - new `SRv6InUse`, built on the same `fib { kernel { } }` check as `MPLSInUse`.
- `internal/component/kernelcap/probe_linux.go` and `probe_other.go` - the new probe beside `MPLSIPMTU`, reusing `InThrowawayNetworkNamespace`, `remountSysctlWritable` and `stepFailed`.
- `internal/core/diagnostic/codes.go` - four codes and their registry entries.
- `addRichRoute` / `replaceRichRoute` in `internal/plugins/fib/kernel/nexthop_linux.go` - the refusal wrap.
- `gokrazy/kernel/patches/series` - patch 0003 after 0002.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The patch moves forwarded seg6 traffic back onto `ip_forward` / `ip6_forward`, the path the MPLS lwtunnel already takes (`mpls_build_state` sets only `LWTUNNEL_STATE_XMIT_REDIRECT`) |
| No unintended coupling (components stay isolated) | Yes | The enrolment lives in the fib kernel plugin that installs seg6; `kernelcap` holds only the probe and predicate, as for MPLS |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `InThrowawayNetworkNamespace`, `remountSysctlWritable`, `stepFailed`, the `evaluateOne` messages and the `ze.test.kernelcap.force` override |
| Zero-copy preserved where applicable (refs, not copies) | N-A | No Ze datapath changes; the probe builds one packet once |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | Two `kernelcap.MustRegister` calls in the owning plugin's `init()`; no shared enumeration is edited |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Searched: `kernelcap.Enrolled`, `ProbeAll` and `KernelSymbols` (derive from the registry), `internal/le/interoplab/kernelcheck.go` (reads `ProbeAll` rows), `internal/core/diagnostic/codes.go` (each code declared once with its entry). The implementer repeats the search for `srv6-encap` and `srv6-forward-ttl` |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
<!-- Every row needs a validation method. `unvalidated` is not a valid final
     status: closure re-checks each one. A broken assumption also gets a
     Mistake Log row and a Deviations entry. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `seg6_output_core` handles a forwarded skb the way it handles a locally sent one: it reads only `skb_dst(skb)->lwtstate`, takes `sk` NULL, and `__seg6_do_srh_encap` rewrites `IP6CB` for an IPv4 inner and keeps it for an IPv6 inner, exactly as on the input path | linux-7.3 `seg6_output_core`, `__seg6_do_srh_encap`, `lwtunnel_output` (read for this spec) | the patched kernel drops or corrupts forwarded packets | AC-1, AC-2 on Ze's kernel in qemu-nightly | unvalidated |
| A-2 | `inline` mode on a forwarded IPv6 packet still lowers the Hop Limit exactly once: `ip6_forward` lowers it, `__seg6_do_srh_inline` keeps the same header (`memmove`), and `seg6_output_core` ends in `dst_output`, not a second `ip6_forward` | linux-7.3 `__seg6_do_srh_inline`, `seg6_output_core` | inline goes from one decrement to zero or two | Ze installs no inline route, so validated by reading at the patch review, and by a kernel-tree selftest run only if one exists for inline (`tools/testing/selftests/net/srv6_*`) | unvalidated |
| A-3 | `ip_dst_mtu_maybe_forward` / `ip6_dst_mtu_maybe_forward` subtract `lwtunnel_headroom` (`include/net/ip.h`, `include/net/ip6_route.h`), so the forward path now answers an oversized DF-set IPv4 or IPv6 packet with ICMP to the real sender | linux-7.3 headers above, `ip_forward` `ip_exceeds_mtu`, `ip6_forward` | no PMTU signal, or a wrong MTU value in the ICMP | AC-6 | unvalidated |
| A-4 | A Service SID can reach the kernel FIB from any BGP peer: IBGP always accepts the SRv6 Prefix-SID (`accept-srv6-prefix-sid` description, `ze-bgp-conf.yang`), so `SRv6InUse` cannot narrow below "kernel FIB and a BGP peer or group" | `internal/component/bgp/yang/ze-bgp-conf.yang`, `internal/core/rib/routeinstall/sink.go` carries `SRv6SID` | a narrower predicate would hide the warning on a router that installs SRv6 | read `routeinstall/sink.go` and the family set that carries a Service SID during Phase 3; `TestSRv6InUse` | unvalidated |
| A-5 | A TUN device can be created in the throwaway namespace (`CONFIG_TUN=y` in `gokrazy/kernel/runtime.config`; `/dev/net/tun` present on the appliance and on the CI runners) and needs only CAP_NET_ADMIN | `runtime.config`, `kernelcap` probe precedent | the probe answers unknown on hosts without `/dev/net/tun` (some containers) | the probe maps a missing `/dev/net/tun` to unknown with that reason; `TestSRv6ProbeClassification` | unvalidated |
| A-6 | One probe packet is enough: a definitive verdict (present or absent) is a property of the running kernel and does not change while the process lives, because `IPV6_SEG6_LWTUNNEL` is a bool built into `ipv6` and patch 0003 is compiled in | `net/ipv6/Kconfig` `config IPV6_SEG6_LWTUNNEL` is `bool` | a cached absent survives a kernel change, which needs a reboot anyway | `TestSRv6ProbeMemoizesDefinitiveVerdict` | unvalidated |
| A-7 | Ze's runtime kernel builds with `CONFIG_IPV6_SEG6_LWTUNNEL=y`: it depends on `IPV6` and selects `LWTUNNEL`, `DST_CACHE`, `IPV6_MULTIPLE_TABLES`, all already `=y` | `net/ipv6/Kconfig`; `runtime.config` has `CONFIG_LWTUNNEL=y`, `CONFIG_IPV6_MULTIPLE_TABLES=y` | the require manifest refuses the build | `./ze-host appliance kernel --target runtime --arch arm64` on the Mac; the manifest check before the compile | unvalidated |
| A-8 | A DF-clear IPv4 packet larger than the route MTU minus the headroom is dropped after encapsulation both before and after the patch, because `dst->output` is `lwtunnel_output`, which skips `ip_finish_output` and its fragmentation | linux-7.3 `lwtunnel_set_redirect`, `ip_forward` (`ip_exceeds_mtu` returns false for DF clear) | the patch changes DF-clear handling and an AC is missing | measured once by the AC-6 test with DF clear, result written into Design Insights | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The first-SID lookup moves from `ip6_route_input` (uses the ingress interface, so a VRF ingress resolves the SID in the VRF table, and `ip rule iif` applies) to `ip6_route_output` with no interface (main table, `skb->mark` only) | a Service SID route in a VRF stops reaching its SID, or starts reaching it; the AC-1 test passes but a VRF test does not | the underlay lookup in the default table is what SRv6 L3VPN needs; Ze sets no `SEG6_IPTUNNEL_TABLE`. Recorded in the patch header and `fib-depth-4-srv6.md`; a VRF case is named in Work Not Done if no VRF SRv6 install path exists to test |
| R-2 | Forwarded seg6 traffic now passes the FORWARD hook and the XFRM forward policy: a firewall that drops forwarded traffic, or an IPsec policy, now applies to it | packets that encapsulated before are dropped on a router with a forward-drop policy | this is the correct semantics of a forwarding router; documented in the patch header and `fib-depth-4-srv6.md` |
| R-3 | The patch stops applying at a kernel bump | `applyPatches` fails the kernel build | the patch is a two-line flag change plus a comment; the limitation entry's Re-check names it |
| R-4 | The behavioural probe costs time at every start, reload, validate and doctor run | start latency, a probe that waits for a packet that never comes | one probe per process (A-6), bounded by a one-second read deadline that answers unknown; the throwaway namespace is torn down when its thread exits |
| R-5 | The probe runs where the host forbids namespaces (no CAP_SYS_ADMIN, AppArmor, SELinux) | unknown or denied rows | same route as `MPLSIPMTU`: `stepFailed` maps a policy refusal to denied, other failures to unknown; both are warnings and ze starts |
| R-6 | Adding two enrolments changes `internal/le/interoplab/kernelcheck.go` `DockerKernelRoute`: a stock-kernel Docker host now reads `srv6-forward-ttl` absent | interop runs move from Docker to Ze's VM | expected: META requires lab hosts to run Ze's kernel; named in the commit body |
| R-7 | Enabling `IPV6_SEG6_LWTUNNEL` also builds `seg6_local` and the seg6 sysctls into Ze's kernel | new attack surface for SRH packets | `net.ipv6.conf.*.seg6_enabled` defaults to 0, so SRH packets addressed to the router are still dropped; Ze installs no seg6local route |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | SRv6 service traffic through Ze's kernel is dropped or mis-encapsulated (a kernel patch defect), or a start, reload or `ze doctor` run reports a false warning (a probe defect). The probe never refuses a start: both new capabilities degrade |
| How is it reverted? | One revert per commit: the kernel config and patch commit (needs a kernel rebuild and a reboot of the appliance), the probe commit, the test commit. No config migration, nothing on the BGP wire |
| Who else touches this path? | `gokrazy/kernel/` (any kernel bump or config spec), `internal/plugins/fib/kernel/` (MPLS transit MTU, ownership), `internal/component/kernelcap/` (every enrolment), `internal/core/diagnostic/codes.go` (every component). Check `git status` for foreign hunks in these files before each commit |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| sysrib best change with `SRv6SID`, IPv4 packet in | → | `buildSEG6Encap` route, then the patched `ip_forward` -> `seg6_output_core` | `TestFIBServiceSIDEncapsulationOnWire` |
| sysrib best change with `SRv6SID`, IPv6 packet in | → | `buildSEG6Encap` route, then the patched `ip6_forward` -> `seg6_output_core` | `TestFIBServiceSIDEncapsulationIPv6InnerOnWire` |
| sysrib best change with `SRv6SID`, TTL 1 or Hop Limit 1 packet in | → | `ip_forward` / `ip6_forward` ICMP Time Exceeded | `TestFIBServiceSIDExpiringTTLAnswersTimeExceeded`, `TestFIBServiceSIDExpiringHopLimitAnswersTimeExceeded` |
| `ze doctor --json config <file>` with `fib { kernel { } }` and a BGP peer | → | `kernelcap.Evaluate` -> `SRv6InUse` -> the two enrolments in `kernelcap_linux.go` | `test/ui/doctor-srv6-kernelcap.ci` |
| enrolled probe called with no config on the running kernel | → | `kernelcap.SRv6ForwardTTL` / `kernelcap.SRv6Encap` | `TestSRv6Integration_ProbeAnswersFromAFreshNamespace` |
| sysrib best change with `SRv6SID` on a kernel without seg6 | → | `addRichRoute` refusal wrap | `TestSEG6InstallRefusalNamesKernelFeature` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Ze's kernel, a Service SID route for 192.0.2.0/24, an IPv4 packet with TTL 64 forwarded to 192.0.2.20 | The packet on the egress link is IPv6 + SRH + IPv4 with inner TTL 63 and inner addresses and payload unchanged; the outer Hop Limit is the egress interface hop limit (64 by default) |
| AC-2 | Ze's kernel, a Service SID route for an IPv6 prefix, an IPv6 packet with Hop Limit 64 forwarded into it | The egress packet is IPv6 + SRH (next header IPv6) + IPv6 with inner Hop Limit 63, inner addresses and payload unchanged; the outer Hop Limit is 63 |
| AC-3 | Ze's kernel, the AC-1 route, an IPv4 packet with TTL 1 | The sender receives ICMP Time Exceeded (type 11, code 0) quoting its packet; no encapsulated packet appears on the egress link within the capture window |
| AC-4 | Ze's kernel, the AC-2 route, an IPv6 packet with Hop Limit 1 | The sender receives ICMPv6 Time Exceeded (type 3, code 0); no encapsulated packet appears on the egress link |
| AC-5 | A stock kernel with seg6 but without patch 0003 (the Ubuntu runner in evidence-nightly) | AC-1 to AC-4 tests read the probe, assert the unpatched behaviour (inner TTL unchanged, TTL 1 encapsulated), log the verdict, and pass; on a release ending in `-ze` they assert AC-1 to AC-4 and never take the stock branch |
| AC-6 | Ze's kernel, the AC-1 route on a 1500-byte link, a DF-set IPv4 packet that fits 1500 but not 1500 minus the seg6 headroom | The sender receives ICMP Fragmentation Needed whose next-hop MTU is 1500 minus the headroom; nothing is encapsulated. The same test with DF clear records what the kernel does (A-8) |
| AC-7 | The kernelcap probe in a fresh network namespace | On a `-ze` release both `srv6-encap` and `srv6-forward-ttl` answer present; on a stock kernel with seg6, `srv6-encap` present and `srv6-forward-ttl` absent; on a kernel without seg6, both absent; never unknown where the test has CAP_SYS_ADMIN and `/dev/net/tun` |
| AC-8 | `ze doctor --json config <file>` with `fib { kernel { } }`, a BGP peer, and forced `srv6-forward-ttl=absent,srv6-encap=unknown` | Output holds `doctor-srv6-forward-ttl-not-decremented` naming the kernel feature, what is lost and the remedy, and `doctor-srv6-encap-unknown`; exit code 0 (both are warnings); the forced override is named |
| AC-9 | A config without `fib { kernel { } }` or without any BGP peer | No `srv6-*` row, and the probe does not run |
| AC-10 | A Service SID route installed on a kernel without seg6 | The FIB sync report and the log say the kernel has no SRv6 encapsulation (`CONFIG_IPV6_SEG6_LWTUNNEL`), that the route was not installed, and name the fix (run Ze's kernel or a kernel with that option, and `ze doctor` to confirm) |
| AC-11 | Ze's kernel build | `runtime.require` pins `CONFIG_IPV6_SEG6_LWTUNNEL` and `CONFIG_IPV6_SEG6_FORWARD_TTL`; the resolved config holds both `=y`; `TestEnrolledKernelSymbolsAreInRuntimeRequire` passes with the two new enrolments |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs Ze as an SRv6 headend; customer traffic crosses it | BGP UPDATE with SRv6 Prefix-SID -> RIB -> sysrib -> `fibKernel.processEvent` -> `buildSEG6Encap` -> kernel route -> patched `ip_forward` -> `seg6_output_core` -> wire | `TestFIBServiceSIDEncapsulationOnWire`, `TestFIBServiceSIDEncapsulationIPv6InnerOnWire` |
| 2 | Runs traceroute from a customer site through the headend | TTL 1 packet -> `ip_forward` / `ip6_forward` -> ICMP Time Exceeded from the headend | `TestFIBServiceSIDExpiringTTLAnswersTimeExceeded`, `TestFIBServiceSIDExpiringHopLimitAnswersTimeExceeded` |
| 3 | Runs `ze doctor` on a stock distribution kernel | config -> `kernelcap.Evaluate` -> `SRv6InUse` -> probe -> warning row naming the patch and the remedy | `test/ui/doctor-srv6-kernelcap.ci`, `TestSRv6Integration_ProbeAnswersFromAFreshNamespace` |
| 4 | Starts Ze with SRv6 routes on a kernel without seg6 | start gate warning, then each refused install says why and how to fix it | `TestSEG6InstallRefusalNamesKernelFeature`, `test/ui/doctor-srv6-kernelcap.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSRv6ProbeClassification` | `internal/component/kernelcap/probe_srv6_linux_test.go` | Table over the classifier: route add refused with `EOPNOTSUPP` -> encap absent and TTL absent; encapsulated packet with inner TTL sent minus one -> both present; inner TTL equal to sent -> encap present, TTL absent; no packet before the deadline -> unknown; `/dev/net/tun` missing -> unknown with that reason; a policy refusal -> denied (`stepFailed`); a reply that is not the probe's own packet (wrong SID, wrong payload marker) is ignored | |
| `TestSRv6ProbeMemoizesDefinitiveVerdict` | `internal/component/kernelcap/probe_srv6_linux_test.go` | The packet step runs once for present and absent, again after unknown or denied; `SRv6Encap` and `SRv6ForwardTTL` share one run | |
| `TestSRv6InUse` | `internal/component/kernelcap/predicate_test.go` | True for `fib { kernel { } }` plus a peer or a group; false without the kernel FIB, without `bgp`, or with an empty `bgp` | |
| `TestSRv6CapabilityCodesRegistered` | `internal/plugins/fib/kernel/kernelcap_codes_linux_test.go` | The four codes are registered with title, description and remedy, as `TestMPLSCapabilityCodesRegistered` checks | |
| `TestSRv6CapabilitiesDegrade` | `internal/plugins/fib/kernel/kernelcap_codes_linux_test.go` | Both enrolments set `Degrades`, so an absent answer is a warning and `kernelcap.Refuse` returns nil | |
| `TestSEG6InstallRefusalNamesKernelFeature` | `internal/plugins/fib/kernel/nexthop_seg6_refusal_linux_test.go` | A backend whose `RouteAdd` or `RouteReplace` returns `EOPNOTSUPP` for a SEG6 encap yields an error that names `CONFIG_IPV6_SEG6_LWTUNNEL`, the prefix and `ze doctor`; an `EOPNOTSUPP` for an MPLS encap is not rewritten | |
| `TestEnrolledKernelSymbolsAreInRuntimeRequire` | `internal/appliance/kernelreq_enrolled_linux_test.go` | Existing; now covers the two new symbols | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Inner IPv4 TTL at the headend | 0-255 | 2 (encapsulated with 1) | 1 (Time Exceeded, AC-3) | N/A |
| Inner IPv6 Hop Limit at the headend | 0-255 | 2 (encapsulated with 1) | 1 (Time Exceeded, AC-4) | N/A |
| Forwarded packet size, DF set | up to route MTU minus seg6 headroom | MTU minus headroom (encapsulated) | N/A | MTU minus headroom plus 1 (Fragmentation Needed, AC-6) |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `doctor-srv6-kernelcap` | `test/ui/doctor-srv6-kernelcap.ci` | An operator with SRv6 on the kernel FIB runs `ze doctor --json config`; forced `srv6-forward-ttl=absent,srv6-encap=unknown` gives both warning rows with their codes and kernel symbols, exit 0, and names the override; a second run with no `bgp` block gives no `srv6-*` row | |
| `TestFIBServiceSIDEncapsulationOnWire` (existing, changed) | `internal/plugins/fib/kernel/srv6_integration_linux_test.go` | AC-1 and AC-5: Ze's kernel asserts inner TTL 63; a stock kernel asserts the probe's branch | |
| `TestFIBServiceSIDEncapsulationIPv6InnerOnWire` | `internal/plugins/fib/kernel/srv6_integration_linux_test.go` | AC-2 | |
| `TestFIBServiceSIDExpiringTTLAnswersTimeExceeded` | `internal/plugins/fib/kernel/srv6_integration_linux_test.go` | AC-3 | |
| `TestFIBServiceSIDExpiringHopLimitAnswersTimeExceeded` | `internal/plugins/fib/kernel/srv6_integration_linux_test.go` | AC-4 | |
| `TestFIBServiceSIDOversizedDFAnswersFragmentationNeeded` | `internal/plugins/fib/kernel/srv6_integration_linux_test.go` | AC-6, and the DF-clear measurement for A-8 | |
| `TestSRv6Integration_ProbeAnswersFromAFreshNamespace` | `internal/plugins/fib/kernel/srv6probe_integration_linux_test.go` | AC-7, in the shape of `TestMPLSIntegration_ProbeAnswersFromAFreshNamespace` (`-ze` release wants present) | |

The scoped run is `./le job run label srv6-ttl command go test` with the feature tag set plus `integration` (`docs/contributing/running-commands.md`, "A bare `go test` is not `./le test unit`") over `./internal/plugins/fib/kernel/`, `./internal/component/kernelcap/` and `./internal/appliance/`; `./le verify changed` judges the closing commits. The integration tests run on the stock kernel in `evidence-nightly.yml` job `integration` and on Ze's amd64 kernel in `qemu-nightly.yml`. Ze's arm64 kernel builds natively on the Mac with `./ze-host appliance kernel --target runtime --arch arm64`. The discrimination walk for AC-1 to AC-4 is: build Ze's kernel without patch 0003, see the four tests fail on the `-ze` branch, restore the patch, see them pass, and record both runs in the spec.

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | `test/interop/scenarios/bgp-srv6-frr/` checked | FRR | The SRv6 scenarios (`bgp-srv6-frr`, `bgp-srv6-invalid-sid-ineligible-frr`, `bgp-srv6-next-hop-identity-frr`, `bgp-srv6-next-hop-identity-unchanged-frr`) prove the BGP control plane and carry no data plane (`bgp-srv6-frr` holds only `frr.conf` and `ze.conf`). The change is in the Linux forwarding path, invisible to a BGP peer; the inner TTL is observed only by a capture on the headend's egress link, which the integration tests do. FRR as a headend uses the same kernel path (`fill_seg6ipt_encap`), so an FRR peer adds no independent evidence | |

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `gokrazy/kernel/runtime.config` - `CONFIG_IPV6_SEG6_LWTUNNEL=y` and `CONFIG_IPV6_SEG6_FORWARD_TTL=y` in a block that states why (Service SID install, the 2026-10-30 finding that Ze's kernel had no seg6)
- `gokrazy/kernel/runtime.require` - pin both symbols
- `gokrazy/kernel/patches/series` - add `0003-seg6-forward-ttl.patch` after `0002-mpls-ip-mtu.patch`
- `internal/plugins/fib/kernel/kernelcap_linux.go` - enrol `srv6-encap` and `srv6-forward-ttl`
- `internal/plugins/fib/kernel/nexthop_linux.go` - `addRichRoute` / `replaceRichRoute` wrap an `EOPNOTSUPP` for a SEG6 encap with the user-facing error (AC-10); comment on `buildSEG6Encap` names the kernel requirement
- `internal/component/kernelcap/predicate.go` - `SRv6InUse`
- `internal/component/kernelcap/probe_other.go` - non-Linux `SRv6Encap` / `SRv6ForwardTTL` answering unknown, as `MPLSIPMTU` does there
- `internal/core/diagnostic/codes.go` - `CodeDoctorSRv6EncapUnavailable`, `CodeDoctorSRv6EncapUnknown`, `CodeDoctorSRv6ForwardTTLNotDecremented`, `CodeDoctorSRv6ForwardTTLUnknown` and their entries (title, description naming LL-SEG6-HEADEND-NO-TTL-DECREMENT, remedy)
- `internal/plugins/fib/kernel/srv6_integration_linux_test.go` - probe branch in the existing test, four new tests
- `internal/plugins/fib/kernel/kernelcap_codes_linux_test.go` - two new tests
- `internal/component/kernelcap/predicate_test.go` - `TestSRv6InUse`
- `docs/architecture/linux-limitations.md` - entry LL-SEG6-HEADEND-NO-TTL-DECREMENT
- `docs/architecture/fib/fib-depth-4-srv6.md` - "Kernel backend": kernel requirement, inner TTL, forward-path consequences, doctor codes
- `docs/architecture/doctor-and-health-checks.md` - the two new rows if the page lists enrolments
- `docs/features/srv6.md` - "Linux Kernel": kernel requirement, inner TTL on Ze's kernel versus a stock kernel, the doctor codes

## Files to Create
- `gokrazy/kernel/patches/0003-seg6-forward-ttl.patch` - the `seg6_build_state` flag change, the corrected comment in `__seg6_do_srh_encap`, and the `config IPV6_SEG6_FORWARD_TTL` marker (`def_bool y`, `depends on IPV6_SEG6_LWTUNNEL`) in `net/ipv6/Kconfig`; header in the 0002 shape: RFC 8986 Section 5.1 S05 quoted, RFC 2473 Section 3.1, the MPLS precedent, the forward-path consequences (R-1, R-2, A-3), "Based on Linux v7.3"
- `internal/component/kernelcap/probe_srv6_linux.go` - the probe
- `internal/component/kernelcap/probe_srv6_linux_test.go` - classification and memo tests
- `internal/plugins/fib/kernel/srv6probe_integration_linux_test.go` - AC-7
- `internal/plugins/fib/kernel/nexthop_seg6_refusal_linux_test.go` - AC-10
- `test/ui/doctor-srv6-kernelcap.ci` - AC-8, AC-9

### Probe procedure (for `probe_srv6_linux.go`)
1. Run inside `InThrowawayNetworkNamespace`; call `remountSysctlWritable`; set `net.ipv4.ip_forward=1`, `net.ipv6.conf.all.forwarding=1`, `net.ipv4.conf.all.send_redirects=0`. Each refused step goes through `stepFailed`.
2. Open `/dev/net/tun` as an IFF_TUN, IFF_NO_PI device; bring it up; give it 198.51.100.1/24 and 2001:db8:ffff::1/64.
3. Add an IPv4 route 203.0.113.0/24 dev tun with a seg6 encap (mode encap, one segment 2001:db8:fffe::1), exactly the request `buildSEG6Encap` sends, and a route 2001:db8:fffe::/64 dev tun. `EOPNOTSUPP` here is the encap-absent verdict.
4. Write one IPv4 UDP packet 198.51.100.2 -> 203.0.113.1, TTL 64, carrying a probe marker, into the TUN fd; read with a one-second deadline, skipping any frame that is not IPv6 to the segment with the marker in the inner payload.
5. Inner TTL 63: present. Inner TTL 64: TTL absent (encap present). No frame: unknown. The thread and both namespaces die with the call, which removes everything.
6. Keep the first present or absent verdict for the process life (A-6); unknown and denied are not kept.

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config leaf: the behaviour follows from the kernel |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | N-A | `ze doctor` gains rows through the existing enrolment, no new command or flag |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | N-A | No leaf |
| Functional test for new RPC/API | N-A | No RPC; the doctor rows are covered by `test/ui/doctor-srv6-kernelcap.ci` |
| Pipe completeness | N-A | No new output command |
| Env var registration | N-A | Reuses `ze.test.kernelcap.force`; no new variable |
| Doctor check for runtime dependencies | Yes | Kernel feature: two enrolments in `internal/plugins/fib/kernel/kernelcap_linux.go`, four codes in `internal/core/diagnostic/codes.go`, `TestSRv6ProbeClassification`, `TestSRv6CapabilityCodesRegistered`, `test/ui/doctor-srv6-kernelcap.ci` |
| Prometheus counters/metrics | No | The probe is a one-shot answer shown by `ze doctor`; the install refusal already raises `reportCodeFIBSyncFailure` |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP change |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | No `features/<id>.md` declaration covers SRv6, so no level changes. `docs/features/srv6.md` "Linux Kernel" (anchored to `nexthop_linux.go`) gains: the kernel needs `CONFIG_IPV6_SEG6_LWTUNNEL`, and only Ze's kernel (patch 0003) lowers the inner TTL; a stock kernel forwards with the inner TTL unchanged, and `ze doctor` says so. The other anchors the audit lists (`docs/architecture/core-design.md`, `docs/guide/mpls.md`, `docs/guide/ipsec.md`, `docs/guide/redistribution.md`, `docs/guide/vpp.md`, `docs/architecture/bgp/as112-coordination.md`) mention changed files for other symbols and stay unaffected |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | `ze doctor` output gains rows through the enrolment; the codes are listed by `ze show diagnostic` from `codes.go` |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | The fib kernel plugin gains enrolments, not a surface |
| 6 | Has a user guide page? | No | No SRv6 guide page exists |
| 7 | Wire format changed? | No | Ze's netlink request is unchanged; the forwarded packet's inner TTL is documented in `docs/architecture/fib/fib-depth-4-srv6.md` |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC 8986 is in no release profile, so no `rfc/short/rfc8986.md` (Required Reading); the behaviour is recorded in `fib-depth-4-srv6.md` and the limitation entry |
| 10 | Test infrastructure changed? | No | New tests use existing harnesses |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` makes no SRv6 TTL claim |
| 12 | Internal architecture changed? | Yes | `docs/architecture/fib/fib-depth-4-srv6.md` "Kernel backend" (kernel requirement, TTL, forward-path consequences); `docs/architecture/linux-limitations.md` LL-SEG6-HEADEND-NO-TTL-DECREMENT; `docs/architecture/doctor-and-health-checks.md` if it lists enrolments |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | Two kernel capability enrolments: check `docs/guide/status.md` and `docs/features/plugins.md` for a list of kernel capabilities and add the rows if one exists |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/features/ai-first.md` (declared by `internal/core/diagnostic/codes.go`): unaffected, it describes the code registry, not individual codes. `plan/release/validation/spec-fib-depth.md` (declared by `nexthop_linux.go` and `richroute.go`): a live spec, unaffected, it describes the encap builders whose request this spec keeps unchanged. `docs/architecture/fib/fib-depth-4-srv6.md` anchors `nexthop_linux.go` `buildSEG6Encap`: updated (row 12). Re-run `./le spec citation anchors spec plan/post-release/kernel/spec-srv6-headend-decrements-inner-ttl.md` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No SRv6 example shows the inner TTL |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- the two enrolments with a stub probe that answers unknown, `SRv6InUse`, the four codes, and the failing tests
   - Tests: `TestSRv6CapabilityCodesRegistered`, `TestSRv6CapabilitiesDegrade`, `TestSRv6InUse`, `test/ui/doctor-srv6-kernelcap.ci` (passes on forced answers), `TestEnrolledKernelSymbolsAreInRuntimeRequire` (fails until Phase 2 pins the symbols)
   - Files: `kernelcap_linux.go`, `predicate.go`, `codes.go`, `probe_srv6_linux.go` stub, `probe_other.go`
   - Verify: `ze doctor` shows the rows from the entry point; the stub answers unknown
2. **Phase: Kernel config and patch** -- write `0003-seg6-forward-ttl.patch`, add it to `series`, set and pin both symbols
   - Tests: the require manifest check, then the kernel build `./ze-host appliance kernel --target runtime --arch arm64` on the Mac; `TestEnrolledKernelSymbolsAreInRuntimeRequire` passes
   - Files: `gokrazy/kernel/patches/0003-seg6-forward-ttl.patch`, `series`, `runtime.config`, `runtime.require`
   - Verify: the patch applies to v7.3 through `applyPatches`; the resolved config holds both `=y`. Commit this phase on its own
3. **Phase: Probe and install refusal** -- the behavioural probe (Probe procedure above), the memo, the `EOPNOTSUPP` wrap
   - Tests: `TestSRv6ProbeClassification`, `TestSRv6ProbeMemoizesDefinitiveVerdict`, `TestSEG6InstallRefusalNamesKernelFeature`, `TestSRv6Integration_ProbeAnswersFromAFreshNamespace`
   - Files: `probe_srv6_linux.go`, `probe_srv6_linux_test.go`, `nexthop_linux.go`, `nexthop_seg6_refusal_linux_test.go`, `srv6probe_integration_linux_test.go`
   - Verify: unit tests fail on the stub, pass on the probe; the integration test answers absent on the stock host kernel. Validate A-4, A-5, A-6
4. **Phase: Datapath tests** -- the probe branch in the existing test and the four new tests
   - Tests: `TestFIBServiceSIDEncapsulationOnWire`, `TestFIBServiceSIDEncapsulationIPv6InnerOnWire`, `TestFIBServiceSIDExpiringTTLAnswersTimeExceeded`, `TestFIBServiceSIDExpiringHopLimitAnswersTimeExceeded`, `TestFIBServiceSIDOversizedDFAnswersFragmentationNeeded`
   - Files: `srv6_integration_linux_test.go`
   - Verify: on Ze's kernel in a QEMU guest, red with patch 0003 removed from `series`, green with it; record both runs here (discrimination). Validate A-1, A-3, A-8
5. **Phase: Documentation** -- the limitation entry, `fib-depth-4-srv6.md`, `docs/features/srv6.md`, `doctor-and-health-checks.md`
   - Tests: `./le doc check all`
   - Files: the docs in Files to Modify
   - Verify: every factual paragraph carries a source anchor; the page and the code agree

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | The patch touches only the flag assignment in `seg6_build_state` (plus the comment and the Kconfig marker); L2 modes keep `INPUT_REDIRECT`; the probe compares the inner TTL, never the outer; the probe's route request is the one `buildSEG6Encap` sends |
| Naming | Subsystems `srv6-encap` and `srv6-forward-ttl` (lower-kebab); codes `doctor-srv6-encap-unavailable`, `doctor-srv6-encap-unknown`, `doctor-srv6-forward-ttl-not-decremented`, `doctor-srv6-forward-ttl-unknown`; Kconfig marker `IPV6_SEG6_FORWARD_TTL` |
| Data flow | The fib kernel plugin owns the enrolments; `kernelcap` holds only probe and predicate; the forwarding owner never runs the namespace probe (it holds no CAP_SYS_ADMIN) |
| Rule: `ai/rules/user-facing-errors.md` | Each doctor description and the install refusal say what is wrong, why (the kernel lacks seg6 or patch 0003), and the exact fix (run Ze's kernel, or build with `CONFIG_IPV6_SEG6_LWTUNNEL` and the patch; confirm with `ze doctor`) |
| Rule: `ai/rules/rfc-compliance.md` | The patch header and the Go comments quote RFC 8986 Section 5.1 S05 byte-exact from `rfc/full/rfc8986.txt` |
| Rule: `ai/rules/interop-and-goal-validation.md` | The red run with patch 0003 removed is recorded for AC-1 to AC-4 |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| Patch 0003 in the series | `grep 0003-seg6-forward-ttl.patch gokrazy/kernel/patches/series` |
| Both symbols set and pinned | `grep -n "IPV6_SEG6" gokrazy/kernel/runtime.config gokrazy/kernel/runtime.require` |
| Two enrolments | `grep -n "srv6-encap\|srv6-forward-ttl" internal/plugins/fib/kernel/kernelcap_linux.go` |
| Four codes | `grep -n "doctor-srv6-" internal/core/diagnostic/codes.go` |
| Unit tests pass | scoped `./le job run label srv6-ttl command go test` over `./internal/component/kernelcap/`, `./internal/plugins/fib/kernel/`, `./internal/appliance/` |
| Doctor functional test passes | `./le test functional` with the single test `doctor-srv6-kernelcap` |
| Datapath tests on Ze's kernel | qemu-nightly run, or a local QEMU guest on Ze's kernel, with the red and green runs pasted here |
| Limitation entry | `grep -n "LL-SEG6-HEADEND-NO-TTL-DECREMENT" docs/architecture/linux-limitations.md` |
| Docs agree | `./le doc check all` |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | The probe reads frames from its own TUN device in its own namespace: it bounds every read to the frame length before indexing, and ignores a frame that is not its marked packet |
| Resource exhaustion | One probe per process (memo), one packet, a one-second deadline; the namespace thread exits, so no device, route or file descriptor outlives the call |
| Isolation | Every route, sysctl and device lives in the throwaway network and mount namespaces; the caller's namespace and `/proc/sys` mount stay untouched (asserted in the integration test, as `TestMPLSIntegration_ProbeAnswersFromAFreshNamespace` asserts the label space) |
| Fail-open | Unknown and denied are warnings that never claim present; a forced answer is always named as forced (`forcedPresent`) |
| Firewall bypass closed | Today forwarded seg6 traffic skips the FORWARD hook (input redirect); after the patch the operator's forward filter applies. Named in the patch header and `fib-depth-4-srv6.md` (R-2) |
| New kernel surface | `seg6_local` and the SRH receive path come with `IPV6_SEG6_LWTUNNEL`; `seg6_enabled` stays 0 by default, and Ze sets it nowhere (R-7) |
| Error leakage | The install refusal names the prefix, the kernel feature and the fix; no kernel address or internal state |

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
<!-- LIVE: write immediately when you learn something. Route each lesson to its
     governing surface under ai/rules/planning.md. A problem-class journal row
     is appropriate only when no surface governs the lesson yet; closure alone
     requires no lesson artifact. -->
- The false comment in `__seg6_do_srh_encap` ("decapsulation will overwrite inner hlim with outer hlim") is the root of the upstream defect: `seg6_local.c` holds no hop-limit code, so no decapsulator restores anything. Patch 0003 corrects the comment.
- After the patch the seg6 input functions (`seg6_input`, `seg6_input_core`, `seg6_input_route`, `cache_input`) serve only the L2 modes. The patch leaves them in place: removing them is an upstream refactor, not this fix.
- The forward path brings PMTU signalling to the headend for free (A-3): today an oversized packet is encapsulated, then refused by `ip6_forward` on the outer, whose Packet Too Big goes to the headend's own address, so the sender never learns.
- Because `ze doctor kernel-capabilities` reports not ready when any enrolment is absent, a stock kernel now reports `srv6-forward-ttl` absent there; lab hosts run Ze's kernel (`ai/META-DECISIONS.md`, "Test systems and labs").

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `kernel-refuses-what-the-installer-sends.md` | 2026-10-30 | SRv6 Service SID install, `buildSEG6Encap`, inner TTL not decremented |

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Enable `CONFIG_IPV6_SEG6_LWTUNNEL` in Ze's runtime kernel and pin it (owner, design gate) | Leave it off and refuse SRv6 on Ze's kernel | Ze installs H.Encaps for RFC 9252 service routes; without the option no Service SID installs on the kernel Ze ships |
| Patch 0003 follows the MPLS lwtunnel model: L3 modes (encap, encap.red, inline) set `OUTPUT_REDIRECT` only, L2 modes keep `INPUT_REDIRECT` (owner, design gate) | (a) Decrement and check the inner TTL inside `seg6_input_core` or the encap helpers. Rejected: it re-implements in seg6 what `ip_forward` / `ip6_forward` already do (TTL check, ICMP to the real sender with the right source, MTU check, FORWARD hook, XFRM forward policy, counters), each of which would need its own copy, and the copies drift. (b) A VRF detour (steer the prefix into a VRF whose route carries the encap, so the packet is forwarded once before encapsulation). Rejected: it costs a VRF device and table per headend, changes Ze's install model, and still leaves the default-table install wrong for every other user | One flag change moves forwarded traffic onto the path every other forwarded packet takes; `mpls_build_state` already proves the shape (`LWTUNNEL_STATE_XMIT_REDIRECT` only). RFC 8986 Section 4 judges only the observable wire result |
| Runtime detection is a behavioural probe: encapsulate one forwarded packet in a throwaway namespace and read its inner TTL (owner, design gate) | A Kconfig-symbol probe (read `/proc/config.gz` or a sysfs marker for `IPV6_SEG6_FORWARD_TTL`). Rejected: `/proc/config.gz` needs `CONFIG_IKCONFIG_PROC`, a distribution kernel may carry an equivalent fix under no symbol, and a symbol proves a build option, not the forwarding result | The probe measures the behaviour the RFC requires, on the running kernel, the way `kernelcap.MPLSIPMTU` measures the MTU attribute |
| The patch adds a marker `config IPV6_SEG6_FORWARD_TTL` (`def_bool y`), used only by `runtime.require` and as the `Kernel` name of the `srv6-forward-ttl` enrolment | Use `CONFIG_IPV6_SEG6_LWTUNNEL` as the `Kernel` name of both enrolments | `evaluateOne` prints "the kernel holds no <Kernel>"; with the shared name a stock kernel with seg6 would read "holds no CONFIG_IPV6_SEG6_LWTUNNEL", which is false. The marker also makes a build without the patch fail before the compile, as `CONFIG_MPLS_IP_MTU` does. Runtime never reads it |
| Two enrolments, `srv6-encap` and `srv6-forward-ttl`, answered by one memoized probe run | One enrolment with one verdict | The two faults have different remedies (no seg6 at all, versus seg6 without the patch) and different losses; one run keeps the cost to one namespace and one packet |
| Both enrolments degrade (warning), never refuse a start | Refuse when `srv6-encap` is absent | `SRv6InUse` must be broad (A-4: IBGP always accepts a Service SID), so a refusal would stop every BGP router on the kernel FIB of a kernel without seg6, including Ze's current kernel; the precedent `transitMTUCapability` degrades too. The install refusal (AC-10) names the fault per route |
| No `rfc/short/rfc8986.md` | Write and enrol a summary | No release profile lists RFC 8986 (`rfc/profiles/`), and META forbids enrolling an RFC no profile lists |
| TUN device in the probe | veth pair with a packet socket | TUN is L3: no neighbour resolution, no MAC addresses, one file descriptor both writes the probe packet and reads the result |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     is not a limitation: write it as its own spec, in the bucket that item
     belongs to, and name that spec here (ai/rules/planning.md). -->
- Until this spec lands, `TestFIBServiceSIDEncapsulationOnWire` stays red on the stock-kernel evidence-nightly job (inner TTL 0x40 read for 0x40 sent), and SRv6 H.Encaps cannot be installed on Ze's own kernel, which has no `CONFIG_IPV6_SEG6_LWTUNNEL`.
- Outer Hop Limit of an IPv6-inner packet follows the (now decremented) inner value. RFC 2473 Section 6.3 asks, in lowercase "should", that the tunnel entry point set the outer hop limit "independently of the hop limit of the original header". RFC 8986 S03 says only "Set outer Payload Length, Traffic Class, Hop Limit, and Flow Label fields" with "As described in [RFC2473] and [RFC6437]", so it makes no outer value mandatory. A separate concern, not work in this spec; RFC 8986 is in no release profile, and an optional requirement waits until mandatory coverage is complete (`ai/META-DECISIONS.md`).
- A stock kernel keeps the defect: Ze cannot change the datapath of a kernel it did not build. `ze doctor` and the startup gate name it (AC-8), and LL-SEG6-HEADEND-NO-TTL-DECREMENT records it.
- DF-clear oversized IPv4 (A-8): if the measurement confirms the packet is dropped after encapsulation on both kernels, that is a fragmentation behaviour this patch does not change. The implementation phase then writes one row in `plan/journal/kernel-refuses-what-the-installer-sends.md` and lists it in the owned journal rows of the spec that will fix it.
- A VRF-bound Service SID route (R-1) is tested only if Ze has a VRF install path for Service SIDs; otherwise the lookup change is recorded in the patch header and `fib-depth-4-srv6.md`.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

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
- [ ] The targeted tests for what changed pass: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches, and `./le verify changed` over the closing commits (`/ze-close` step 6f). The full verification is Thomas's release check: no agent runs it, and one that is owed is handed to him as its exact command. A red there gets a fix commit
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
