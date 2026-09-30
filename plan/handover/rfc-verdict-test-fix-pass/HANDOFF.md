# Handoff: spec-rfc-verdict-test-fix-pass (2026-09-30, from session 869df689)

Goal (owner, unchanged): close plan/pre-release/spec-rfc-verdict-test-fix-pass.md and its
eight children. Done = parent AC-11: every child closed, the derived weak/wrong listing holds
only verdicts named in another spec's acceptance criteria, /ze-review gate clean,
`./le verify worktree` has run.

Orchestration files (read in this order), all in this directory
(plan/handover/rfc-verdict-test-fix-pass/, copied from session 869df689's scratch on the
owner's order; session-state.md is that session's per-spec state file). The briefs and
older handoffs still name the old scratch path `tmp/session/2026-09-28-869df689-.../scratch/children/`:
read it as this directory, and put new ledger lock files and logs in the new session's scratch.
Nothing under tmp/ is needed: every file the next session needs is in this directory.
data/ holds the three data files the handoffs cite: triage-c17.txt (OSPF listing triage),
vrrp-closure-records.sh (guest recording script, SKIP_HOST=1), rc12-recorded.tsv (access
AC-C2 sweep list). Any other tmp/session/... path in these files names a log or script that
is regenerable and was deliberately not kept. Derived listings: rerun each child spec's jq.
Owner rule: all handoff data lives here in the repo, never only in tmp/.

Last `./le verify worktree` (2026-09-30T22:57Z, tree busy with in-flight agent edits): FAILED,
full red at rfc/check, arch iface-resolution, test sensitivity/check, go staticcheck part 1/6
(deadline), doc wiring, doc check/links, doc check/retired-commands, repo/tree-check,
arch compound-guard/check, test health/check, site facts/check, verify deps/unit-cached,
test functional/gating. The functional BGP api suite passed 40/40. Not triaged: which reds
are this work's and which are other sessions'. Triage on a quiet tree is owed before any close.
- RULINGS.md: R1..R46, OWNER RULING 2..5, binding on every agent.
- AUTHOR-BRIEF.md / JUDGE-BRIEF.md: the prompts' shared rules (addendums binding).
- QUEUE.md: full chronological log; the last ~40 lines are current.
- <child>/*.md: per-package author handoffs (continuations).
- Per-child derived listing: the jq in each child spec's "Derived listing" section.

## Child state

| Child | State | Next |
|---|---|---|
| vrrp | CLOSED (875eb64337, 3b30602d2e) | post-hoc AC-C2 record check only |
| bfd | listing 0; interop fixed (uncommitted, see below) | AC-C2 sweep, verify, /ze-close |
| routing | listing = 2 Blocked-by | AC-C2 sweep, verify, /ze-close |
| services | listing = 12 Blocked-by (dbdde9fb4f) | AC-C2 sweep, verify, /ze-close |
| access | listing = 3 Blocked-by + RFC5176-3-1 (fixed, uncommitted) | judge c12, RFC2866-4.1-1 .ci record (owner, sudo), close |
| ike-eap | 5 Blocked-by + 6 ruling-5 ids authored (uncommitted) | judge ruling 5 (+R46), AC-C2 sweep, close |
| ospf | 57 unblocked weak/wrong (last 7708c6b240) | author/judge rounds |
| bgp | ~127 unblocked (last 9aad86e4fc) | c8 author first (see below), then packages |

## Uncommitted work in the tree (owner: this orchestration)

| Files | What | Who commits |
|---|---|---|
| internal/core/eap/eap_tls.go, eap_tls_fragment_guard_test.go, peer_test.go, rfc5216_reassembly_refusal_test.go (new), rfc/discrimination/rfc5216.json, rfc7296.json, docs/architecture/ike/ipsec-11-interop-eap.md | ike-eap ruling-5 author (RFC5216-3-1 D-8 fix, 2.1.5-1 negatives, 7296-3.15.1-2 records) | ike-eap judge (handoff ike-eap/ruling5-author.md; apply R46 to RFC7296-2.23-2) |
| authradius/rfc5176_section_rows_test.go, rfc/short/rfc5176.md, rfc/discrimination/{rfc1332,1334,1661,1994,2516,2661,2865,2866,2869,5072,5176,8907}.json, plan/journal/green-that-could-not-have-been-red.md | access c12: RFC5176-3-1 fix + AC-C2 sweep (135 records) | access judge (handoff access/radius-author.md "Continuation 12"); RFC-approved trailers for both authradius units |
| (committed 2276df3a66) | L2TP env-key fix | done |
| test/interop/scenarios/ospf-bfd-frr/ze.conf, ospfv3-bfd-frr/ze.conf, docs/architecture/ospf/bfd-client.md | OSPF-BFD interop scenarios fixed (top-level bfd container; bind-v6); all 3 FRR BFD scenarios PASS | commit with the BFD close |
| rfc/audit/{draft-ietf-idr-linklocal-capability,rfc4659,rfc6793,rfc8669,rfc9086}.json, test/weakened/*.md | corpus reseal side effects / other sessions | leave unless a judge owns the stem |
| validator_mandatory_rfc7950_red_test.go, loader_rfc7950_structural_red_test.go, rfc5216_resumption_defect_test.go | intentional red tests owned by other specs | never commit here |

## Standing decisions (all in RULINGS.md / QUEUE.md)
- AC-C2 strict: every tagged unit of a verdict moved to enforced carries a discrimination record. Sweep each child before close (access done; bfd, routing, services, ike-eap, and post-hoc vrrp owed).
- AC-C3: a {gap} row stamped unimplemented / {not-applicable} stamped not-applicable counts as a verdict; planned retire/merge done or corrected.
- Never `./le spec release` from a subagent (shared session id drops the parent claim).
- A commit script hitting another session's index.lock: never re-run parts; write an owner script `tmp/delete-<sid>-*.sh`.
- Long gates (`./le verify worktree`, full lint, functional suites, .ci recordings) run from the main thread or a fresh agent. Last verify attempt: staticcheck exceeded its 10m30s deadline under load; re-run when the machine is quiet.

## Next queue (owner asked for ONE agent at a time in session 869df689; ask the owner the cap for the new session)
1. ike-eap judge (ruling 5 + R46), then ike-eap AC-C2 sweep and /ze-close.
2. access judge (c12), then owner re-runs the RFC2866-4.1-1 recorder from a normal terminal (command in QUEUE.md; needs CAP_NET_ADMIN), then access close.
3. BGP c8 author: FIRST correct the false public claim on RFC4271-6.1-4 {gap} + rfc4271 Support text (Ze sends 1/3 since fc9c8bcaae); restore the full 6.1-3 sentence and tag the header checks to 6.1-1/6.1-3; retire 8.2.2-18 as a duplicate of 8.2.2-8 (move signal-stop-cease.ci tag, long functional record); R45 Event-3 gap split for 8.2.2-7; 8.2.2-12/14/15; clean the garbled tag comment at internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go:155 and re-record. Then BGP packages (reactor ~45, rib 18, nlri/ls 10, gr 10, ls_export 8, message 7, mrt 8, rib/pool 6, small rest).
4. OSPF c18 onward: 57 unblocked ids (scratch/triage-c17.txt has triage notes).
5. AC-C2 sweeps for bfd, routing, services, ike-eap; post-hoc check vrrp.
6. `./le verify worktree` (quiet machine), then /ze-close for bfd, routing, services, access, ike-eap.
7. Parent: /ze-review gate, AC-11 check, final verify, close.

## Owner-facing open items
- None blocking beyond the RFC2866-4.1-1 recording (step 2) and the agent cap.
- Journal-only item flagged for the owner: an OSPF interface with `bfd` but no top-level `bfd` container only Warns (BFD plugin never loads); recommended a commit-time error (not done).
