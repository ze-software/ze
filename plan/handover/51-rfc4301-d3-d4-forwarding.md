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

1. **`mobike-initiator` interop red: not caused by this spec's work, owned by
   `plan/release/fixes/spec-ipsec-strongswan-interop-reds.md`**, which records
   the same failure from evidence-nightly 43586166002 on 2026-11-14. Cause, read
   at the producer: in `vpn ipsec interface` mode the address worker
   (`followAddress`, engine/dynamic_wan.go, since 67700fd37e) rebinds the
   listeners, and `rebindListeners` stops every session on the old address
   (`stopSessionsOn`, engine/apply.go), so a fresh IKE_SA_INIT from the new
   address replaces the SA before `serviceMobike` can send UPDATE_SA_ADDRESSES.
   Only route-chosen mode exempts an SA with `mobikeUp`. The strongSwan log shows
   no UPDATE_SA_ADDRESSES and an INITIAL_CONTACT replacement. Decided by Thomas
   2026-11-20: (A), a MOBIKE SA survives an interface-address move (re-point the
   session's socket pair, skip the restart and the reconcile restart for an
   interface fill-in change; recorded on the design page ipsec-7-ikev2-engine.md
   beside the rebind paragraph). The work belongs to
   spec-ipsec-strongswan-interop-reds: add it there, with an engine test that an
   interface-mode address move with `mobikeUp` keeps the SPIs and queues the
   update, and the mobike-initiator lab as the interop proof.
2. **K1 extension for forwarded ICMP errors (owner decision 2026-11-20, (c)):**
   extend `gokrazy/kernel/patches/0003-xfrm-state-scope.patch` so the forwarded
   path, `__xfrm_route_forward` included, tries `xfrm_out_fwd_icmp` when the OUT
   lookup answers BLOCK (`-EPERM`); advertise it by a K1 feature bit; make the
   engine require that bit as it requires bit 5; remove the stock-kernel OUT
   allows at `dataplane.PriorityICMPErrorReturn` and the nft table
   `ze_ipsec_icmp_error_return` (no layering); add a namespace packet test where
   an operator out DISCARD covers a forwarded mappable error and it is mapped
   into the return SA, with the negative that a non-mappable error under the
   same discard is dropped.
3. **K1 bit 7, forwarded ICMP error past a gate (owner decision 2026-11-20, (a)):**
   a BYPASS's FWD transit gate (`spdTransitPolicies`) matches a forwarded ICMP
   error that arrived on a Child SA, so stock `__xfrm_policy_check` rejects the
   consistent error (`XfrmInTmplMismatch`; `TestRFC4301ForwardedICMPUsesPayloadPolicy`
   red at if_id 0 on `qemu-c860ed8b`). Write the K1 hunk: in the FWD check, an ICMP
   error with a secpath whose own-header policy is a template-free ALLOW carrying
   `XFRM_POLICY_ICMP` takes `xfrm_in_fwd_icmp`'s payload policy. Set
   `XFRM_POLICY_ICMP` on every transit gate, require bit 7 like bits 5 and 6, move
   the scope tests' complete receipt to 255 (or as the bit list says), run in the
   guest. The `40cf3cfaec` patch is not defective (fixture shared a kernel
   selector; the test now runs `verifySPDOrder`). Open: why the spec records the
   test passing at `516671704b`, which already held D-3.
4. AC-8 red proofs on `tunnel-df-treatment` (it passes): break `xfrmDFFlag`
   for `DFClear` (expect assertions 1 and 3 red), then for `DFCopy` (4 and 6).
5. Red proofs for `spd-discard-entry` (break `spdPolicyHasDirection` out, then
   in: assertions 4, 5) and `pad-subtree-identity` (`textIdentityMatches`
   always true, then false: assertions 7, 1). Both pass clean.
6. `.ci` drafts: the two parse drafts pass; stress with
   `./le test stress-repro run suite "parse --draft" test <name> any-failure iterations 40`
   (the first try ended exit 1 unread: read its log first), promote. The two
   `test/draft/ipsec/` drafts need `./le test qemu ci test test/draft/ipsec/<name>.ci`.
7. AC-13 records: RFC7296-3.13.1-1 and -2, positive on
   `TestTSWireAnyPortsForEveryProtocol` (break `ipsec/traffic_selector.go::Wire`),
   negative on `TestTSWireRefusesUndefinedPortValues` (break
   `engine/ts_narrow.go::selectorsToWire`).
8. AC-4 remainder and the owner decisions below; then the five disagreeing
   summary rows, the independent review, and closure.

## Decisions for Thomas (no RFC, design or rule answers them)

| # | Decision | Recommendation |
|---|---|---|
| 1 | Map a forwarded ICMP error an operator out/both DISCARD covers: (a) allow at order-1 plus order refusals, (b) refuse at commit, (c) extend K1 so `__xfrm_route_forward` tries `xfrm_out_fwd_icmp` on `-EPERM` | Decided 2026-11-20: (c) |
| 2 | A commit-time notice on a port-scoped operator BYPASS until K3's capability bit exists | yes, a notice, not a refusal |
| 3 | The 13 phase-4 Mobility Header walks: their list was lost; 15 untagged candidates exist | rebuild as one surviving-mutant check per producer guard |
| 4 | `bypass-fragment-forgery`: its target (the node-wide IKE bypass) is gone | retarget to an operator port-scoped BYPASS, in a K1/K3 lab |
| 5 | Carried: VPP SPD entry interface binding; RFC 4301 sign-off scope in rfcgate-6; decision 3 at a boot catch-all failure | as in handover 50 |
| 6 | A BYPASS's FWD transit gate rejects a consistent forwarded ICMP error from a Child SA: (a) K1 bit 7, the FWD check takes `xfrm_in_fwd_icmp`'s payload policy past a template-free ALLOW carrying `XFRM_POLICY_ICMP`, Ze sets that flag on every gate; (b) no FWD gate where a BYPASS would cover Child SA traffic; (c) a template on the gate | Decided 2026-11-20: (a) |

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
