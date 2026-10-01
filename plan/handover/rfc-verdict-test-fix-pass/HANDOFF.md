# Handoff: spec-rfc-verdict-test-fix-pass (2026-10-01, end of session 869df689)

Goal (owner, unchanged): close plan/pre-release/spec-rfc-verdict-test-fix-pass.md and its
eight children. Done = parent AC-11: every child closed, the derived weak/wrong listing holds
only verdicts named in another spec's acceptance criteria, /ze-review gate clean,
`./le verify worktree` has run.

Every file the next session needs is in this directory (plan/handover/rfc-verdict-test-fix-pass/),
committed, so the work resumes on any machine. Paths under tmp/session/2026-09-28-869df689-.../
named in the briefs and older handoffs are lock files and logs only: create the directory
(NEXT-SESSION.md step 1) and let flock create the locks. Logs named there were not kept.

Read in this order:
- NEXT-SESSION.md: the prompt to paste and the owner's sudo command.
- RULINGS.md: R1..R58 and owner rulings 2..5, binding on every agent.
- AUTHOR-BRIEF.md / JUDGE-BRIEF.md: the shared agent rules (addendums binding).
- QUEUE.md: chronological log; the last ~40 lines are current.
- <child>/*.md: per-package author handoffs. BGP: bgp/fsm-peer-author.md (continuations 8..21,
  RFC 4271 + link-local + RFC 2545), bgp/rfc9552-author.md (BGP-LS, in progress).
- data/: triage-c17.txt (OSPF), vrrp-closure-records.sh, rc12-recorded.tsv (access sweep),
  rfc4271_replacement_red_test.go.txt (the route-server probe, for spec-bgp-rs-replacement-on-withdrawal).
- session-state.md: session 869df689's per-spec state file at the first handover (historical).

## Child state

| Child | State | Next |
|---|---|---|
| vrrp | CLOSED (875eb64337, 3b30602d2e) | post-hoc AC-C2 record check only |
| bfd | listing 0; OSPF-BFD interop fixes committed (1c21de3153) | AC-C2 sweep incl. rfc5880 producer-changed records, verify, /ze-close |
| routing | listing = 2 Blocked-by | AC-C2 sweep, verify, /ze-close |
| services | listing = 12 Blocked-by (dbdde9fb4f) | AC-C2 sweep, verify, /ze-close |
| access | judged (9af0ecb46b); RFC1332-2.1-1 negative weak (absence claim, needs mutant route) | owner RFC2866-4.1-1 recording, close |
| ike-eap | ruling-5 ids enforced (b57b47161d) | AC-C2 sweep incl. 7 stale RFC9190-5.10-1 records on startTLSClient, close |
| ospf | 57 unblocked weak/wrong (last 7708c6b240) | author/judge rounds (data/triage-c17.txt) |
| bgp | 131 listing rows incl. Blocked-by; RFC 4271 left = 4.3-3, 4.3-4, 9.2-7 (all Blocked-by) | RFC 9552 (13 left), then rfc4724 9, rfc9494 8, rfc8277 7, rfc9252 6, rfc8669 6, linklocal 6, rfc2385 5, ... |

## Session 869df689, 2026-10-01: what landed

Commits (oldest first): b57b47161d ike-eap judge; 9af0ecb46b access judge; 16a01204c4 R47;
07bdc7ca76 c8; 841553bffd c9; 64e4b99bd0 c10; 6ab45a1542 c11 (FSM Error NOTIFICATION);
1e5c829890 c12 (D-9 version error = Event 24); 36435ca55c c13 (IBGP export AS_PATH,
AS_SET-first MED); 1e54dbbc7d c14 (RFC 7705 migration-aware export guard); 7101dc6f37 c15
(6.3-2 WARN log, DelayOpen gap); 5380077d80 c16 (next-hop self from the connected endpoint);
db6e42abe2 c17 (link-local on forward rails); 6da51925b2 interop bgp-nexthop-self-local-auto-frr;
4755e4d1f6 c18; 771fc01602 c19 (own link-local only with own global, RFC2545-3-6 gap,
skeleton spec-bgp-rs-replacement-on-withdrawal); b926ed0894 c20 (capability 77 needs a
link-local address); 1c21de3153 OSPF-BFD interop configs; c21 judge commit (see QUEUE.md).

Defects fixed this session (each failing test first): unknown message type claim; FSM Error
NOTIFICATION in OpenSent/OpenConfirm; version-error NOTIFICATION as Event 24; export
prepend/remove-private toward IBGP (incl. RFC 7705 migration-internal) and dry-run agreement;
AS_SET-first neighbour AS in MED comparison; next-hop-self NEXT_HOP log at default level;
next-hop self under local ip auto on the forward rails; link-local-only next hop refusal on
forward rails; own link-local under local ip auto; third-party global never paired with Ze's
link-local; capability 77 refused without a link-local address; global+link-local relayed to
multihop EBGP stripped to the global.

## Owner decisions this session
- Agent cap: 1 (this session only).
- spec-bgp-rs-replacement-on-withdrawal (RFC4271-9.2-7, route-server bare withdrawal while
  another source still has the prefix): runs LATER. The red probe
  internal/component/bgp/plugins/rs/rfc4271_replacement_red_test.go is untracked (red on
  purpose); a copy is kept as data/rfc4271_replacement_red_test.go.txt for that spec.
- Duplicate red test rfc4271_third_party_nexthop_red_test.go: removed.
- R58(a) chose config-load refusal for capability 77 without link-local; the owner was told
  interface-derived link-local is the alternative (no reply: refusal stands).

## Uncommitted in the tree at handover (not this work: leave)
rfc/audit/{rfc4659,rfc6793,rfc8669,rfc9086}.json and test/weakened/{7e722ae0,916ee49e}.md
(corpus reseal side effects / other sessions); config *_red_test.go and
internal/core/eap/rfc5216_resumption_defect_test.go (intentional reds owned by other specs).

## Standing decisions (RULINGS.md / QUEUE.md)
- AC-C2 strict: every tagged unit of a verdict moved to enforced carries a discrimination record.
- AC-C3: {gap}+unimplemented and {not-applicable}+not-applicable count as verdicts.
- A row Ze implements with no row gets one (R47b). Optional absent features: {gap} +
  unimplemented (R53), never hand-deleted audit keys (R52).
- Never `./le spec release` from a subagent. Long gates from the main thread.
- Commit-script index.lock failure after the commit landed: never re-run parts; write an owner
  script tmp/delete-<sid>-*.sh (done once this session, 1c21de3153).
- Handoff rows go in plan/handover files, never only in the session state file.

## Next queue
1. BGP: RFC 9552 (bgp/rfc9552-author.md; 5.2.1.4-1 looks like a sender D-8), then the next
   stems by size. Author/judge pairs, one stem at a time.
2. OSPF c18 onward: 57 unblocked ids.
3. Owner: RFC2866-4.1-1 recording (NEXT-SESSION.md step 2); then access close.
4. AC-C2 sweeps: bfd (rfc5880 stale records), routing, services, ike-eap (RFC9190); vrrp post-hoc.
5. `./le verify worktree` on a quiet machine; triage; /ze-close bfd, routing, services, access, ike-eap.
6. Parent: /ze-review gate, AC-11, final verify, close.

## Known reds that are not this work
- BGP plugin functional 524/523 path-asn-filter-export-reject and 641/640
  redistribute-export-modify: wait-file fixture regression, journaled in
  plan/journal/option-set-for-one-caller-changes-another.md.
- Load-only timeouts seen and passing alone: plugin 401, 687; encode 10-13, 16, 17.
