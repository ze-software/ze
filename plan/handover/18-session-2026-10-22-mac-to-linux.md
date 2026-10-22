# Handover: session 0b4a8e87 moves from the Mac to Linux (2026-10-22)

Written for the next agent, on Linux. Session 0b4a8e87 drove four specs at once; each
has its own handover, and this file is the index plus what the move itself changes.
Closed this session: spec-bgp-sr-policy-rfc-defects (`ba5304d558`).

Thomas, 2026-10-22: "commit all the files". So this session's uncommitted work was
committed for real where that was truthful, and kept as a patch where committing it
would publish a false statement.

## What landed before the move, and what is a patch

**Update 2026-10-22:** Thomas ordered the patch content committed before closure
("the plans and docs are not commited"). It landed in the commit that removed both
`18-mac-to-linux-*.patch` files, so at closure there is nothing to apply: the learned
file, the doc status and the repoints are already in the tree, and they read as
closed while the two specs are still open.

| Spec | Committed | Kept as patch, and why |
|------|-----------|------------------------|
| `spec-bgp-rs-replacement-on-withdrawal` | `e306f08858`: the Phase 3a draft (six reactor files) and the Phase 4 `departureOweFraming` fix with `TestRecoveryDepartureWithdrawsMultiLabelRouteWithCompatibility`. A WIP commit: the reactor package is red, its body names the 12 red tests | nothing |
| `spec-ike-eap-rfc-defects` | nothing new (the `ai/INDEX.md` and `interop.md` hunks already rode along with `c0b69d7a53`) | `18-mac-to-linux-ike-eap-rfc-defects.patch`: `plan/learned/032-ike-eap-rfc-defects.md` and the citer repoints in `docs/contributing/patching-the-standard-library.md`, `plan/pre-release/spec-rfc-verdict-fix-ike-eap.md`, `plan/pre-release/spec-rfc-verdict-test-fix-pass.md`. Committing a learned file is a closure, and the repoints say "closed 2026-10-21" or point at the learned file; the spec is not closed |
| `spec-bgp-update-propagation-rfc-defects` | `10e2fbaed6`: the spec's closure edits (Review Gate rounds 4 and 5, R4-I1, A-3 broken, Goal Validation greens) | `18-mac-to-linux-bgp-update-propagation-rfc-defects.patch`: the four citer repoints in `spec-bgp-graceful-restart-rfc-defects.md`, `spec-bgp-link-local-only-next-hop.md`, `spec-bgp-next-hop-auto-rewrites-for-ebgp.md`, `spec-plugin-reload-applies-or-refuses.md`. Each says the spec is "closed 2026-10-21", which is false until it closes |
| `spec-rfc-verdict-fix-ospf` | nothing new: the `interop.md` paragraphs for `ospf-self-lsa-flush-frr`, `ospf-sr-anycast-nexthop-frr` and `ze-sysctls` already landed in `c0b69d7a53` | nothing |

Both patches passed `git apply --check -R` against the Mac working tree when written,
so they hold exactly what was there. They are closure text: apply each one only in the
closing commit of its spec (ze-close commit A), never before.

## Resume order on Linux

1. `git pull --rebase --no-autostash`, then `git log -1 e306f08858` to confirm the WIP
   commit is there. `go test ./internal/component/bgp/reactor/` is red for everyone
   until 3a is finished; that is expected and named in the commit body.
2. update-propagation first: it is closest to closing and needs only the lab.
3. OSPF next: lab work, no code expected.
4. ike-eap: discrimination re-records, then close.
5. rs-replacement last: the longest, and every review round needs Thomas.

To apply a patch at closure: `git apply plan/handover/18-mac-to-linux-<slug>.patch`,
then commit the files it touched with the closing commit and `remove` the patch file
in the same commit. If `git apply` refuses because a citing spec moved on, apply with
`git apply --3way` or redo the one-line repoint by hand; each hunk is a single line.

## One row per spec

| Spec | Own handover | Patch | Next step | What blocks it |
|------|--------------|-------|-----------|----------------|
| `spec-bgp-rs-replacement-on-withdrawal` | `12-rs-replacement-phase-3-remaining.md` | none (all in `e306f08858`) | finish Phase 3a: 12 tests red, 8 of them "timed out waiting for recipient TCP causal marker"; start with the instrumentation in `recoveryAdmission.admit` that handover 12's top section describes (key 0's outcome, candidate source and message, entry owner), then rewrite the four Q-3 expectations, then the 3a tests, the `initial-source-message` chain, docs | review round 12 over `e306f08858` and the rest of 3a needs Thomas |
| `spec-ike-eap-rfc-defects` | `14-ike-eap-rfc-defects-closure.md` | `18-mac-to-linux-ike-eap-rfc-defects.patch` | re-record the 39 stale discrimination records (`rfc/discrimination/rfc{3948,4301,4303,7296}.json`, `producer-changed`) with `./le rfc discriminate-record`, one at a time; fix the 3 other `rfc/check` rows (untagged `rfc7296_child_rekey_transport_test.go`, RFC3948-2.1-2 SHIFTED verdict via `./le rfc reseal`, RFC7296-1.4.1-1 STALE verdict re-judged by `ze-rfc-audit`); `./le verify worktree` once; `/ze-close` | nothing on Thomas's side: review is clean at round 9. If the re-records change code, round 10 is pre-authorised. The review artifact sat in `tmp/review/` on the Mac and is lost: re-record it with `rounds 9` |
| `spec-bgp-update-propagation-rfc-defects` | `15-update-propagation-interop.md` | `18-mac-to-linux-bgp-update-propagation-rfc-defects.patch` | the five red walks (the `RED-TREE` edits listed in handover 15), each red at its own assertion; record them in Goal Validation (`INTEROP-RED-Dn` placeholders); `./le verify worktree`; ze-close. Also repoint `plan/pre-release/spec-rfc-verdict-test-fix-pass.md` line 628 at closure. The review artifact was in `tmp/review/` on the Mac: re-record it with `rounds 5` over the files the spec's Review Gate names | nothing: 5 of 5 interop scenarios are green |
| `spec-rfc-verdict-fix-ospf` | `12-rfc-verdict-fix-ospf.md` | none | run the 14 interop greens still owed: of the 12 original scenarios only `ospf-p2p-frr` and `ospf-broadcast-frr` passed, so 10 remain (rerun `ospf-opaque-frr` with the 60s wait fix), plus the four new ones `ospf-sr-anycast-nexthop-frr`, `ospf-self-lsa-flush-frr`, `ospf-accept-lifetime-last-key-frr`, `ospf-gr-helper-broadcast-frr`, then the 3 interop reds (`ospf-sr-anycast-nexthop-frr`, `ospf-self-lsa-flush-frr`, `ospf-accept-lifetime-last-key-frr`, with the reverts handover 12-ospf names); review round 3; verify; close | nothing but lab time |

## Owner authorisations already given

Quoted from the handovers; do not ask again.

| Spec | Round or approval | Thomas's words | Where it is recorded |
|------|-------------------|----------------|----------------------|
| rs-replacement | round 6 | "round 6" (relayed by the main session) | handover 12; spec Review Gate |
| rs-replacement | round 7 | "ok round 7" | handover 12; spec Review Gate, rounds 7 |
| rs-replacement | round 8 | "ok round 8" | handover 12; spec Review Gate, rounds 8 |
| rs-replacement | round 9 | "and you can continue with what you're suggesting" | handover 12; spec Review Gate, rounds 9 |
| rs-replacement | round 10 | "ok" | handover 12; spec Review Gate, rounds 10 |
| rs-replacement | round 11 | "I approve round 11." | handover 12; spec Review Gate, rounds 11 |
| rs-replacement | the two RFC 7311 tagged tests | "I approve round 11. and the changes to the test." | `3205ea7c37`; owner row of `plan/verification-debt/6d6a3fdc.md` discharged (kind owner) |
| ike-eap | round 6 and the TLS 1.2 re-scope | "ok to both" | handover 14; spec Review Gate |
| ike-eap | round 7 | "and you can continue with what you're suggesting" | handover 14; spec Review Gate |
| ike-eap | round 8 | "Review round 8 for IKE EAP" | handover 14; spec Review Gate |
| ike-eap | round 9 | "ok" | handover 14; the review artifact's `owner-authorised` (machine-local, re-record it) |
| ike-eap | round 10, pre-authorised | "If not, I run round 10." | handover 14, last section |
| all | commit the uncommitted work before the move | "commit all the files" | this file; `e306f08858`, `10e2fbaed6` |

Round 12 of rs-replacement is not authorised yet.

## Traps met on the Mac, and whether they follow you

| Trap | What happened | On Linux |
|------|---------------|----------|
| Colima refuses AppArmor | the plain Docker route refuses on the Mac's Colima kernel, so every interop ran through the darwin route `./le test qemu docker-lab lab "test integration interop" env "INTEROP_SCENARIO=<name>"` (13 minutes a run, mostly guest boot) | does not apply. Run `./le test integration interop` (or `interop-ipsec`) with the scenario env directly against the host Docker daemon. If kernelcheck refuses, its message names the fix (`./le setup docker-kernel install`) |
| Full disk | the root volume fell to a few hundred MB several times, filled by other sessions' verify and lab runs; runs died in setup with no verdict (journal `plan/journal/full-disk-false-red.md`) | may recur on any machine. Check `df -h` before a heavy run; `./le scratch cache-clean` frees le's caches |
| Stale `.git/index.lock` | a 0-byte lock from 20:23 on 2026-10-21 with no holder: commits landed but the scripts could not repoint the shared index, so `git diff --cached` showed reverse-staged paths. The lock was gone by 2026-10-22 morning | a fresh clone or pull on Linux has its own `.git`, so the stale state does not travel. If a script prints `git ls-tree HEAD -- <paths> \| git update-index --index-info` as a repair, tell Thomas |
| QEMU port 2222 | only one QEMU guest fits; a run whose guest lost port 2222 to another session's VM failed setup ("sshd did not answer within 5m0s") | applies only if you use a QEMU route on Linux (the IKE Linux integration test under `./le test qemu run`). Interop on Linux needs no guest |
| 103-byte socket path | an export under `tmp/session/<id>/scratch/` made the forwarded Docker socket path exceed `unixSocketPathMax` in `dockerlab.go`; exports went to `~/.cache/ze/iop/<name>` or a short symlink | does not apply to the native Docker route. Keep `git archive` exports at a short path anyway |

Two export traps apply everywhere: a `git archive` export lacks `gokrazy/modcache`
(link the checkout's entries in, as `sharedModcacheLink` does) and the ignored
`gokrazy/docker-host/builddir/github.com/ze-software/ze/go.sum` (copy it in); and never
symlink `cache` into an export that a docker build context reads.
