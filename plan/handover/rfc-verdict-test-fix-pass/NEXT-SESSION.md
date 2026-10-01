# Next session: spec-rfc-verdict-test-fix-pass

Everything needed is in this directory (plan/handover/rfc-verdict-test-fix-pass/),
committed. Nothing under tmp/ is needed, on this machine or another.

## 1. Paste this as the first message of the new session

Replace N with the number of agents to run at once.

```
Continue spec-rfc-verdict-test-fix-pass. Read
plan/handover/rfc-verdict-test-fix-pass/HANDOFF.md
first, then RULINGS.md and the last 40 lines of QUEUE.md in the same directory.
Claim the parent spec with
./le spec claim spec plan/pre-release/spec-rfc-verdict-test-fix-pass.md
Run once before any agent records or stamps (the briefs name these lock paths):
mkdir -p tmp/session/2026-09-28-869df689-cc8f-4d78-9161-1d7c87434c8e/scratch/children
Use N agents at a time. Start at step 1 of the handoff's "Next queue".
```

## 2. Run this yourself, from the repository root, in a normal terminal (needs CAP_NET_ADMIN)

It records the one access test no agent can record (RFC2866-4.1-1, test/l2tp/radius-acct-wire.ci).
Run it while no agent is recording RFC 2866.

```
sudo -E env "PATH=$PATH" "HOME=$HOME" ./le rfc discriminate-record id RFC2866-4.1-1 polarity positive unit test/l2tp/radius-acct-wire.ci route revert producer internal/component/l2tp/plugins/authradius/acct.go::onSessionIPAssigned; sudo chown -R "$(id -u):$(id -g)" rfc/discrimination/rfc2866.json tmp ~/.cache/go-build
```

Expected: the recorder says the test went red under the break and writes one record to
rfc/discrimination/rfc2866.json. If it says "stayed GREEN", tell the next session so the
claim or producer is corrected.

## 3. Where it stands (2026-10-01, end of session 869df689)

| Child | State |
|---|---|
| vrrp | Closed |
| bfd | Listing empty. Owes AC-C2 record sweep (incl. stale rfc5880 producer-changed records), verify, /ze-close |
| routing | Only ids named in other specs. AC-C2 sweep, verify, /ze-close |
| services | Only ids named in other specs. AC-C2 sweep, verify, /ze-close |
| access | Judged and committed (9af0ecb46b). Left: the RFC2866-4.1-1 recording above, then close |
| ike-eap | Ruling-5 ids judged (b57b47161d). Owes AC-C2 sweep (incl. 7 stale RFC9190-5.10-1 records), close |
| ospf | 57 unblocked weak/wrong rows (untouched this session) |
| bgp | 131 listing rows incl. Blocked-by. RFC 4271 done except 3 Blocked-by ids. Next: RFC 9552 (13 left), then rfc4724, rfc9494, linklocal rest, ... |

After all children close: the parent's /ze-review gate, the AC-11 check, a full
`./le verify worktree` on a quiet machine.

Full detail and the ordered queue: HANDOFF.md. Binding rulings: RULINGS.md (R1..R58).
