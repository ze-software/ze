# Spec: srv6-df-clear-fragmentation

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P3 |
| Release reason | Kernel work: DF-clear IPv4 fragmentation across Linux seg6 `lwtunnel_output` requires a Linux kernel datapath change, which Thomas placed after the first release |
| RFC gaps | - |
| Depends | - (inner TTL decrement and DF-set PMTU ICMP landed in `spec-srv6-headend-decrements-inner-ttl`) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-22 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Handle forwarded DF-clear IPv4 packets whose length exceeds the egress link MTU
minus the seg6 lwtunnel headroom (64 bytes for a single-SID SRH on a 1500-byte
link, effective MTU 1436) on Ze's Linux runtime kernel.

Measured during `spec-srv6-headend-decrements-inner-ttl` (A-8) on patched Linux
7.3.0-ze in QEMU (`TestFIBServiceSIDOversizedDFAnswersFragmentationNeeded`): a
1460-byte DF-clear IPv4 packet crossing a 1500-byte egress link with 64-byte
SRv6 headroom produced `unfragmented-srv6=0 ipv6-outer-fragments=0
icmp-errors=0`. `ip_forward` skips `ip_exceeds_mtu` when DF is clear, and
`dst->output` points to `lwtunnel_output` (`seg6_output_core`) instead of
`ip_finish_output`, so the inner IPv4 packet is never fragmented before
encapsulation and the resulting oversized outer IPv6 packet is dropped.

This skeleton records the observed defect so the open journal row has a home;
no kernel patch design is selected or approved yet.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `kernel-refuses-what-the-installer-sends.md` | 2026-11-22 | SRv6 headend DF-clear IPv4 > MTU minus headroom (`seg6_iptunnel.c`, `lwtunnel_output`) |
