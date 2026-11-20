# Handover: RFC 4301 architecture gaps, D-3, D-4 and forwarding (2026-11-20)

Spec: `plan/release/fixes/spec-rfc4301-architecture-gaps.md` (in-progress). Its
top section, "Current continuation checkpoint, 2026-11-19 (evening)", carries the
full status and evidence; this page is the short form. Nothing is pushed.

## Landed (local main, not pushed)

| Commit | What |
|---|---|
| `e700fb7392` | D-4: node-wide UDP 500/4500 SPD bypass removed; red seen before, green after, on Ze's K1 kernel |
| `ce5c276457` | Policy-based gateway under `unmatched discard` forwarded nothing; per-Child FWD and delivery allows |
| `50b89fb51b` | Reserved SPD priority band removed; ranks 1 to 2147483647 |
| `e626cbdf17` | Summary row RFC4301-5.2-11; strongSwan prose replaced; RFC4301-5.2-4 records |
| `22a4740af7` | Interop: DF judged on the wire first; scenarios `spd-discard-entry`, `pad-subtree-identity` |
| `072ef5bf1f`, `0c283fffc6`, `cb6d05616a` | D-3: operator entries decide forwarded traffic; its packet test; 7.4-2 population |
| `516671704b` | Forwarded ICMP errors mapped to the return SA under `unmatched discard` |
| `721d67d3c4` | 12 stale rfc4301 verdicts re-judged, 5.2-11 first verdict |
| `a5528d0863` | le: a lab boots either backend's cached kernel of the same inputs |

## Remaining, in order

1. **`mobike-initiator` interop red (Mac, docker-host lab, K1 kernel):** "MOBIKE
   replaced an IKE or Child SA instead of migrating it" (new IKE and ESP SPIs
   after the move). The 40 MOBIKE engine tests pass in the guest. Find whether
   today's work caused it: run the same scenario at `6cf8d9289d` (before D-4)
   and at `e700fb7392`, `ce5c276457` with
   `./le test qemu docker-lab lab "test integration interop-ipsec" env "IPSEC_INTEROP_SCENARIO=mobike-initiator" timeout 1800s`
   (run from a detached worktree only on a machine where the lab socket path
   fits; on the Mac the main checkout's path fits). Suspects: the per-socket
   bypass on the shared wildcard NAT-T socket MOBIKE uses, and the new Child FWD
   and delivery allows MOBIKE does not migrate. Log:
   `tmp/session/2026-11-19-205719/scratch/interop-mobike-initiator.log`.
2. AC-8 red proofs on `tunnel-df-treatment` (it passes): break `xfrmDFFlag`
   for `DFClear` (expect assertions 1 and 3 red), then for `DFCopy` (4 and 6).
3. Red proofs for `spd-discard-entry` (break `spdPolicyHasDirection` out, then
   in: assertions 4, 5) and `pad-subtree-identity` (`textIdentityMatches`
   always true, then false: assertions 7, 1). Both pass clean.
4. `.ci` drafts: the two parse drafts pass; stress with
   `./le test stress-repro run suite "parse --draft" test <name> any-failure iterations 40`
   (the first try ended exit 1 unread: read its log first), promote. The two
   `test/draft/ipsec/` drafts need `./le test qemu ci test test/draft/ipsec/<name>.ci`.
5. AC-13 records: RFC7296-3.13.1-1 and -2, positive on
   `TestTSWireAnyPortsForEveryProtocol` (break `ipsec/traffic_selector.go::Wire`),
   negative on `TestTSWireRefusesUndefinedPortValues` (break
   `engine/ts_narrow.go::selectorsToWire`).
6. AC-4 remainder and the owner decisions below; then the five disagreeing
   summary rows, the independent review, and closure.

## Decisions for Thomas (no RFC, design or rule answers them)

| # | Decision | Recommendation |
|---|---|---|
| 1 | Map a forwarded ICMP error an operator out/both DISCARD covers: (a) allow at order-1 plus order refusals, (b) refuse at commit, (c) extend K1 so `__xfrm_route_forward` tries `xfrm_out_fwd_icmp` on `-EPERM` | (c): it also replaces today's allow entries and nftables table |
| 2 | A commit-time notice on a port-scoped operator BYPASS until K3's capability bit exists | yes, a notice, not a refusal |
| 3 | The 13 phase-4 Mobility Header walks: their list was lost; 15 untagged candidates exist | rebuild as one surviving-mutant check per producer guard |
| 4 | `bypass-fragment-forgery`: its target (the node-wide IKE bypass) is gone | retarget to an operator port-scoped BYPASS, in a K1/K3 lab |
| 5 | Carried: VPP SPD entry interface binding; RFC 4301 sign-off scope in rfcgate-6; decision 3 at a boot catch-all failure | as in handover 50 |

## Environment notes

- The claude server's host kernel lacks K1 bit 5; packet tests need the guest
  (`./le test qemu docker-lab kernel <docker-1562c4ae vmlinuz> root command ...`).
  A docker-lab lab cannot run from a server worktree: the forwarded socket path
  is 121 bytes, over the 103 limit. Run labs on the Mac.
- The le launcher builds le from HEAD; a vanished Go cache entry made two of its
  builds fail tonight (`bin/builds/.failed-*`), and it then serves the previous le.
- A Bash hook refuses any command naming `docs/` or `rfc/` while another session's
  `rfc/short/rfc2181.md` does not parse.
- Post-commit approval cleanup failed after several commits (le rebuild broken at
  the time); the rows are in `tmp/commit-*-approved-34458f33.md`, commands in
  `tmp/session/2026-11-19-205719/scratch/approval-clean.sh`.
