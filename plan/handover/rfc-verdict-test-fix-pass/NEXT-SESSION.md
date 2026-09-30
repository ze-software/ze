# Next session: spec-rfc-verdict-test-fix-pass

## 1. Paste this as the first message of the new session

Replace N with the number of agents to run at once.

```
Continue spec-rfc-verdict-test-fix-pass. Read
plan/handover/rfc-verdict-test-fix-pass/HANDOFF.md
first, then RULINGS.md and the last 40 lines of QUEUE.md in the same directory (plan/handover/rfc-verdict-test-fix-pass/).
Claim the parent spec with
./le spec claim spec plan/pre-release/spec-rfc-verdict-test-fix-pass.md
Use N agents at a time. Start at step 1 of the handoff's "Next queue".
```

## 2. Run this yourself, in a normal terminal (needs CAP_NET_ADMIN)

It records the one access test no agent can record (RFC2866-4.1-1, test/l2tp/radius-acct-wire.ci).
The L2TP fix it depended on is committed (2276df3a66).

```
cd /home/thomas/Code/github.com/ze-software/ze/main && sudo -E env "PATH=$PATH" "HOME=$HOME" ./le rfc discriminate-record id RFC2866-4.1-1 polarity positive unit test/l2tp/radius-acct-wire.ci route revert producer internal/component/l2tp/plugins/authradius/acct.go::onSessionIPAssigned; sudo chown -R "$(id -u):$(id -g)" rfc/discrimination/rfc2866.json tmp ~/.cache/go-build
```

Expected: the recorder says the test went red under the break and writes one record to
rfc/discrimination/rfc2866.json. If it says "stayed GREEN", the test ran but does not
discriminate that producer; tell the next session so the claim or producer is corrected.

## 3. Where it stands (2026-09-30, end of session 869df689)

| Child | State |
|---|---|
| vrrp | Closed |
| bfd | Listing empty. Owes AC-C2 record sweep, `./le verify worktree`, /ze-close |
| routing | Only ids named in other specs. Same three steps |
| services | Only ids named in other specs. Same three steps |
| access | One judge away (Continuation 12 uncommitted), plus the recording above |
| ike-eap | One judge away (ruling 5 + R46, uncommitted) |
| ospf | 57 unblocked weak/wrong rows |
| bgp | About 127 unblocked rows; start with the false-claim correction on RFC4271-6.1-4 |

After all children close: the parent's /ze-review gate, the AC-11 check, and a full
`./le verify worktree` on a quiet machine (the last attempt timed out in staticcheck under load).

Full detail, uncommitted-file ownership and the ordered queue: HANDOFF.md in this directory.
Binding rulings: RULINGS.md in this directory.
