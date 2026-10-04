# Handoff, session ff3776cb (2026-10-03 / 2026-10-04)

**Branch:** main, working tree clean, 195 commits ahead of origin, nothing pushed.
**Claimed spec:** none (the naming spec was closed and released).
**Goal of the next session:** close the four immediate fixes this session opened, re-judge the RFC verdicts they staled, run full verification, then push on the owner's order.

## Status

### Done
- **spec-rfc-test-file-naming: CLOSED** (5abd02fd97, 6c35c09952; lesson plan/learned/028-one-predicate-for-every-refusal.md). Ten review rounds, last fix 49039f53bb reviewed clean.
- **git-safety pull rule rewritten** (7f4917cb8a): never edit behind upstream; a dirty tree is no blocker unless a pull is owed.
- **Startup race** (spec-bgp-reactor-startup-race, Status in-progress): fixed 3c0cfa5fe6, owner approvals on two tagged tests. NOT independently reviewed, NOT closed.
- **Prefix-SID duplicate TLV (D6 of spec-bgp-prefix-sid-rfc-defects)**: a41bf6a186, interop 0c6ae07e69, review fixes 465af38f1d, RFC8669-6-3 re-judged enforced 96dbfd4da5. Reviewed. D1-D5 of that spec stay open (spec Status skeleton).
- **ADD-PATH best path per prefix, all families** (spec-bgp-addpath-best-path-per-prefix, Status in-progress): phases 1-4 and review rounds 1-3 fixed; commits e059baccb8, 1875c5472a, bdce67b4de, 28679bbd73, f05bb38cfa, 0f9b07ec87, a92135c816, b28b95a85d, b2b1cb73aa, d029f58129, c6271f93e5, 1b574db1e7, 460b5bcd36, e5fae02369, df36922d65, 14fc101e1d, 312da72376, 3fd196d06e, 196261ed14, e27607c82b.
- **Link-local next hop (D6 of spec-bgp-update-propagation-rfc-defects) + withdraw at every withhold gate**: 41eab66e74, f93a30fbfc, 5874676150, 9f5e1453e5, 834f7d26e2, a5b200523c, 8bd6194ac0, 28ab406700, 94c9a3d3c6, ddbe894668, 2987e2c517, a829eb68d1; re-judge 9ac8bd76fb. Review round 1 fixes landed.
- **Skeleton specs**: spec-bgp-next-hop-auto-rewrites-for-ebgp (c2af7e1b3f), spec-bgp-withdraw-only-exported-routes (c5219d54b0).

### Remaining, in order
1. **ADD-PATH review round 4** (independent, /ze-review): scope = round 3 fixes 312da72376, 3fd196d06e, 196261ed14, e27607c82b plus sibling call sites. Then fix and loop to 0 BLOCKER / 0 ISSUE.
2. **ADD-PATH RFC re-judge** (independent /ze-rfc-audit): RFC8277 STALE 2.4-1, 2.5-3, 3.1-1; RFC4271 STALE 9.1.2.2-1; RFC2918 4-3; RFC5575 4-8; RFC9117 4.1-1, 4.2-1; RFC9252 5-1; RFC8955 6-1 (list in the ADD-PATH state file). Then `./le rfc reseal` ONLY for this spec's SHIFTED rows (do not reseal other sessions' rows).
3. **ADD-PATH closure** via /ze-close.
4. **Link-local review round 2** (independent): scope = 28ab406700, 94c9a3d3c6, ddbe894668, 2987e2c517 plus sibling call sites. Then fix/loop, re-judge any stale link-local rows, close D6 (the spec's other defects stay open; spec Status skeleton, so record D6 closure inside it rather than deleting the spec).
5. **Startup race**: independent review of 3c0cfa5fe6, then /ze-close.
6. **Full verification** on Linux with -race: `./le verify worktree` (verify-status STALE, never run this session; debt rows in plan/verification-debt/1d41415e.md). This is the first real run of the Linux-networking tests (fd00::2, lo, 127.0.0.2).
7. **Push**: only on the owner's order, through `./le commit create ... push "<owner authorisation>"`.

### Owner decisions already given (do not re-ask)
- Naming: full convention; rounds 7-10 authorised.
- Link-local: withhold under `next-hop unchanged`; withdraw at every withhold gate; `auto` rewrites to self for eBGP (own spec); extend the netns fixture; Label-Index single only on labelled unicast (Prefix-SID); "Track exported routes" (own spec; unconditional withdrawal stays until it lands).
- Approvals recorded for every tagged-test change listed in the commit bodies and specs.

### Housekeeping owed
- `tmp/commit-rfc-approved-1d41415e.md` holds 18 approval rows no commit consumed (7 ADD-PATH retroactive, 11 link-local cap-77). Each is documented in a commit body or spec. Clear them before the next commit in session namespace 1d41415e, so they are not attached to an unrelated later edit of those tests. The owner's words are the ledger; ask before deleting if unsure.
- Scratch exports tmp/session/2026-10-03-ff3776cb-*/scratch/aigp-head and aigp-pre (~329M each), rev3-clone, base/: disposable build trees, safe to delete.

### Not done, and who owns it
- Prefix-SID D1-D5: plan/immediate/spec-bgp-prefix-sid-rfc-defects.md.
- Link-local spec's other defects: plan/immediate/spec-bgp-update-propagation-rfc-defects.md.
- `next-hop auto` eBGP rewrite: plan/immediate/spec-bgp-next-hop-auto-rewrites-for-ebgp.md (design question R-5 on cross-family next hop open; ~34 .ci and ~14 interop scenarios to review).
- Withdraw only exported routes: plan/immediate/spec-bgp-withdraw-only-exported-routes.md (assumptions A-1, A-2 for its design gate).
- Journal rows added this session (fix later in journal passes): stale MRT RFC 8050 comment and false TestIsAddPathHelpers claim; RFC8050-x-4 stays weak (subtype vs OPEN policy); BMP Loc-RIB sends only ORIGIN/AS_PATH/next hop; six labelled-withdrawal sibling parsers (rib JSON rail, reactor prefix count, ze bgp decode, nlritype.Retain, RR non-CIDR keys, VPN decoder); adj-rib-in labelled key collision; Process.Registration unsynchronised; peerOnLink scope; AIGP test flake (~3/500); approval gate cannot see fixture-only changes; weakened audit cannot read RFC-approved trailers; two IKE MTU .ci share one config store.

## Files already handled (per-spec state files hold digests)
- tmp/session/2026-10-03-ff3776cb-ce05-4491-9d8f-ba31bcc36641/state/session-state-spec-bgp-addpath-best-path-per-prefix-*.md
- tmp/session/2026-10-03-ff3776cb-ce05-4491-9d8f-ba31bcc36641/state/session-state-spec-bgp-update-propagation-rfc-defects-*.md (and the misnamed session-state-bgp-update-propagation-rfc-defects-*.md)
- tmp/session/2026-10-03-ff3776cb-ce05-4491-9d8f-ba31bcc36641/state/session-state-spec-bgp-reactor-startup-race-*.md
- tmp/session/2026-10-03-ff3776cb-ce05-4491-9d8f-ba31bcc36641/state/session-state-spec-bgp-prefix-sid-rfc-defects-*.md

## Edits
None pending: the working tree is clean. All remaining work is review, re-judge, closure and verification, each run by its ze-* skill in a subagent.

## Then
`./le verify worktree > "$(./le session scratch ensure)/verify.log" 2>&1` (long; read the log), after items 1-5.
