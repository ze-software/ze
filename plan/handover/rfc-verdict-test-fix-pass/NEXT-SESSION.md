# Next session: spec-rfc-verdict-test-fix-pass

Everything needed is in this directory (plan/handover/rfc-verdict-test-fix-pass/),
committed. Nothing under tmp/ is needed, on this machine or another. Four intentionally red,
untracked tests live only in the tree of the machine that wrote them (listed in HANDOFF.md
owner items); copies of the older ones are in data/.

## 1. Paste this as the first message of the new session

Replace N with the number of agents to run at once.

```
Continue spec-rfc-verdict-test-fix-pass. Read
plan/handover/rfc-verdict-test-fix-pass/HANDOFF.md
first, then RULINGS.md and the last 20 lines of QUEUE.md in the same directory.
Claim the parent spec with
./le spec claim spec plan/pre-release/spec-rfc-verdict-test-fix-pass.md
Run once before any agent records or stamps (the briefs name these lock paths):
mkdir -p tmp/session/2026-09-28-869df689-cc8f-4d78-9161-1d7c87434c8e/scratch/children
Commit with an explicit `tag <name>` (session 6912077e used every automatic tag).
Use N agents at a time. Start at the handoff's "Step 1", then present the owner items.
```

## 2. Run this yourself on a Linux host, from the repository root (needs CAP_NET_ADMIN)

It records the one access test no agent can record (RFC2866-4.1-1, test/l2tp/radius-acct-wire.ci).
Then commit rfc/discrimination/rfc2866.json from that host (or copy it over).

```
sudo -E env "PATH=$PATH" "HOME=$HOME" ./le rfc discriminate-record id RFC2866-4.1-1 polarity positive unit test/l2tp/radius-acct-wire.ci route revert producer internal/component/l2tp/plugins/authradius/acct.go::onSessionIPAssigned; sudo chown -R "$(id -u):$(id -g)" rfc/discrimination/rfc2866.json tmp ~/.cache/go-build
```

The same Linux host also owes: RFC5880-9-2 +, RFC5881-5-1 +, RFC5881-5-3 + (bfd),
RFC4555-4.2.5-1 + (ike-eap), RFC4302-3.4.3-1 and RFC2328-4.4-1 (ospf). The command for each
is in the child's handoff file.

## 3. Where it stands (2026-10-02, end of session e08980d7)

| Child | State |
|---|---|
| vrrp | Closed |
| bfd | AC-C2 done; 3 Linux records owed; verify, /ze-close |
| routing | AC-C2 done; 22 LDP records stale after the LDP fix (re-record); verify, /ze-close |
| services | AC-C2 done; verify, /ze-close |
| access | RFC2866-4.1-1 Linux recording, then close |
| ike-eap | AC-C2 done; RFC4555-4.2.5-1 Linux record; verify, /ze-close |
| ospf | 3 rows (2 Linux-only, 1 blocked); 2 stale rfc2328 records |
| bgp | ~55 unblocked rows; 13 owner items in HANDOFF.md first |

After all children close: the parent's /ze-review gate, the AC-11 check, a full
`./le verify worktree` on a quiet machine (never run this session).

Full detail: HANDOFF.md. Binding rulings: RULINGS.md (R1..R59, owner rulings 2..8).
