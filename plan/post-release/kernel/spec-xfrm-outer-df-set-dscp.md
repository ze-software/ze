# Spec: xfrm-outer-df-set-dscp

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P3 |
| Release reason | Kernel work: it patches the Linux kernel Ze runs, which Thomas placed after the first release. Until it lands, Ze refuses `df-bit set` and `dscp-map` 1 to 63 on Linux at commit and at install (owner decision, recorded in `plan/release/fixes/spec-rfc4301-architecture-gaps.md`) |
| RFC gaps | RFC4301-8.1-2, RFC4301-5.1.2.1-1 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

RFC 4301 Section 8.1: "This means that it MUST be possible to configure the
implementation's treatment of the DF bit (set, clear, copy from inner header)
for each SA." RFC 4301 Section 5.1.2.1, note 5: "If the packet will immediately
enter a domain for which the DSCP value in the outer header is not appropriate,
that value MUST be mapped to an appropriate value for the domain".

Ze offers `df-bit copy|clear|set` and `dscp-map copy|0..63` per site-to-site
peer. On the Linux XFRM dataplane only `copy` and `clear`, and `copy` and `0`,
can be written: `xfrm4_tunnel_encap_add` (linux-7.3 `net/xfrm/xfrm_output.c`)
builds the outer IPv4 header with
`top_iph->frag_off = (flags & XFRM_STATE_NOPMTUDISC) || small_ipv6 ? 0 : (XFRM_MODE_SKB_CB(skb)->frag_off & htons(IP_DF));`
and
`if (x->props.extra_flags & XFRM_SA_XFLAG_DONT_ENCAP_DSCP) top_iph->tos = 0; else top_iph->tos = XFRM_MODE_SKB_CB(skb)->tos;`.
It can copy or clear DF, and copy or zero the DSCP; no flag or attribute writes
DF set or another DSCP value (`XFRMA_IPTFS_DONT_FRAG` is IP-TFS only).
`xfrm6_tunnel_encap_add` has the same DSCP choice. Ze therefore refuses both at
commit (`verifyPeerSAOptions`, `engine/sa_options.go`) and at install
(`xfrmDFFlag`, `xfrmDSCPFlag`, `dataplane/xfrm_linux.go`).

The VPP dataplane writes any fixed outer DSCP, but DF set is refused there too:
VPP's ESP encrypt path honours `ENCAP_COPY_DF` and never reads `ENCAP_SET_DF`
(`vppDFError`, `dataplane/vpp.go`). This spec covers the Linux kernel. Whether
VPP gains DF set is a separate question, recorded here so it is not lost.

Goal: Ze's kernel gains an XFRM tunnel-mode state option that writes the outer
DF bit set, and one that writes a fixed outer DSCP value, in
`xfrm4_tunnel_encap_add` (and the DSCP one in `xfrm6_tunnel_encap_add`). Ze
programs them for `df-bit set` and `dscp-map 1..63`, and the commit and install
refusals lift on a kernel that carries the patch, and only there.

Owner decision (2026-11-12): keep the refusal for the first release, and write
the kernel feature after it. An nftables rewrite of the outer ESP header (a
`postrouting` rule per SA setting the DF bit or the DSCP) was weighed in
`plan/release/fixes/spec-rfc4301-architecture-gaps.md` as a way Ze could meet
both rows with state it installs on a stock kernel; the owner chose the kernel
feature.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

### Architecture Docs
- [ ] `docs/architecture/linux-limitations.md` - entry LL-XFRM-OUTER-DF-SET-DSCP, which this spec lifts on Ze's kernel
- [ ] `docs/architecture/mpls/mpls-kernel.md` - how patch `0002-mpls-ip-mtu.patch` is carried, pinned by a marker Kconfig symbol, and detected at runtime by a behavioural probe
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - the SA options and their exact-or-refuse projection
- [ ] `docs/architecture/doctor-and-health-checks.md` - "Kernel capabilities: one enrolment, three callers"

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4301.md` - RFC4301-8.1-2 and RFC4301-5.1.2.1-1

## Current Behavior (MANDATORY)

**Source files read:** (read for the skeleton on 2026-11-12; the kernel side is
re-read when the spec moves to design)
- [ ] `internal/component/ike/dataplane/xfrm_linux.go` - `xfrmDFFlag` refuses `DFSet`, `xfrmDSCPFlag` refuses a fixed value other than 0, and `SAOptionsSupported` answers commit verify from the same two predicates
- [ ] `internal/component/ike/dataplane/vpp.go` - `vppDFError` refuses `DFSet` because VPP never reads `ENCAP_SET_DF`; any fixed DSCP is written
- [ ] `internal/component/ike/engine/sa_options.go` - `verifyPeerSAOptions` refuses at commit what the loaded backend's checker refuses

Still to read at design: `outer_header.go`, the kernel's
`xfrm4_tunnel_encap_add` and `xfrm6_tunnel_encap_add`, the uapi `XFRM_STATE_*`,
`XFRM_SA_XFLAG_*` and `XFRMA_*` lists, and `verify_newsa_info` in
`net/xfrm/xfrm_user.c`.

**Behavior to preserve:**
- `df-bit copy|clear` and `dscp-map copy|0` keep their current projection on every kernel.
- A kernel without the patch refuses `set` and 1 to 63 exactly as today.

**Behavior to change:**
- On Ze's kernel, `df-bit set` and `dscp-map 1..63` are programmed rather than refused.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Per-SA outer header options with exact-or-refuse | `dataplane.SAParams` (`DF`, `OuterDSCP`), `dataplane.SAOptionChecker`, `xfrmDFFlag`, `xfrmDSCPFlag`, `SAOptionsSupported` (`dataplane/xfrm_linux.go`); `verifyPeerSAOptions` (`engine/sa_options.go`) | Programs copy and clear, copy and 0, and refuses set and 1 to 63 at commit and at install with `ErrSAOptionRefused` | Extends: the two predicates answer the new state options when the kernel carries them, from one capability answer, so the commit and the install refusals cannot disagree |
| Ze's carried kernel patches | `gokrazy/kernel/patches/` with `series` (`0002-mpls-ip-mtu.patch`), `docs/architecture/mpls/mpls-kernel.md` | A Ze patch adds a marker Kconfig symbol the runtime require manifest pins | Extends: a third patch in the same series, same marker and pinning pattern |
| Kernel capability probes | `internal/component/kernelcap` (`kernelcap.MustRegister`, `kernelcap.MPLSIPMTU`) | A behavioural probe tells startup, `ze config validate` and `ze doctor` what the running kernel does | Extends: one capability for each option, so a stock kernel keeps the refusal and `ze doctor` names it |
| Netlink XFRM vendor diff | `internal/le/vendorpatch/patches/netlink-xfrm-fixes.patch` | Carries Ze's changes to `vishvananda/netlink` XFRM encoding | Extends: the new state attribute or flag is encoded here if the library lacks it |
| Linux limitations register | `docs/architecture/linux-limitations.md`, LL-XFRM-OUTER-DF-SET-DSCP | Records that a stock kernel cannot write DF set or a DSCP value | Extends: the entry gains the Ze-kernel answer; it stays for stock kernels |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [Where data enters: the `df-bit` and `dscp-map` leaves of a site-to-site peer]

### Transformation Path
1. `peerSAOptions` to `dataplane.SAOptions`, then `dataplane.SAParams` for each Child SA.
2. `xfrmStateFromParams` writes the new state option when the capability is present.
3. The patched `xfrm4_tunnel_encap_add` / `xfrm6_tunnel_encap_add` write the outer header.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine to dataplane | `dataplane.SAParams` | No |
| Dataplane to kernel | netlink XFRM state attribute or flag | No |

### Integration Points
- `dataplane.SAOptionChecker` and the two predicates in `xfrm_linux.go`.
- `kernelcap` for the runtime answer.

## Risks & Assumptions

| ID | Assumption or risk | If wrong |
|----|--------------------|----------|
| A-1 | A new `XFRM_SA_XFLAG_*` bit (or a new attribute) is accepted by `verify_newsa_info` without a uapi change upstream rejects | the patch needs a new attribute instead; read `xfrm_user.c` at design |
| A-2 | RFC 6040 ECN handling (`INET_ECN_encapsulate`) is independent of the DSCP value written | AC-2's ECN clause needs its own design |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `set vpn ipsec site-to-site peer <p> df-bit set` committed on Ze's kernel | → | the set-DF state option in `xfrmStateFromParams` | [to be named at design] |
| `set vpn ipsec site-to-site peer <p> dscp-map 46` committed on Ze's kernel | → | the fixed-DSCP state option in `xfrmStateFromParams` | [to be named at design] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `df-bit set` on a tunnel-mode IPv4 peer, Ze's kernel with the patch | the commit is accepted, the installed SA carries the set-DF option, and an emitted outer IPv4 header carries DF set for an inner packet whose DF is clear, observed on the wire |
| AC-2 | `dscp-map 46` on a tunnel-mode peer, Ze's kernel with the patch, IPv4 and IPv6 outer | the outer header carries DSCP 46 whatever the inner DSCP, with the ECN bits still derived from the inner packet as RFC 6040 asks, observed on the wire |
| AC-3 | either option on a kernel without the patch (a stock distribution kernel) | the commit is refused naming the missing kernel feature and the fix, and `ze doctor` reports the capability absent; no SA is installed with another treatment |
| AC-4 | the interop scenario `tunnel-df-treatment` extended with a `set` phase, and a DSCP phase | strongSwan's counters read DF set and the configured DSCP on the outer header; each phase proven red by breaking the producer |
| AC-5 | `rfc/short/rfc4301.md` read after the change | RFC4301-8.1-2 and RFC4301-5.1.2.1-1 carry tagged tests for set and for a fixed value, with discrimination records |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [to be named at design] | `internal/component/ike/dataplane/` | each predicate answers the capability, and refuses on a kernel without it | |

## Files to Modify
- `internal/component/ike/dataplane/xfrm_linux.go` - the two predicates and `xfrmStateFromParams`
- `gokrazy/kernel/patches/series` - the new patch
- `docs/architecture/linux-limitations.md` - LL-XFRM-OUTER-DF-SET-DSCP gains the Ze-kernel answer
- `rfc/short/rfc4301.md` - RFC4301-8.1-2 and RFC4301-5.1.2.1-1

## Implementation Steps

1. Design: read the kernel sources named in Current Behavior and choose flag or attribute.
2. The kernel patch and its marker symbol; the capability probe.
3. The dataplane projection, the predicates, the tests, the interop phases.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `./le test functional ipsec` and `./le test integration interop-ipsec` for the scenarios named in AC-4

## Known Limitations

- VPP's ESP encrypt path never reads `ENCAP_SET_DF`, so DF set stays refused on
  VPP after this spec. It needs its own spec if it is wanted.
