# Handoff: CLI work remaining after the pipe-operator spec

Written 2026-08-23 by session `ca1b4c65` (`7e4e9f00`), at the point the web
build hit its limit. Everything here was measured against the built product,
not read off an audit. Where I did not verify something, the row says so.

The operator LANGUAGE is built and verified. What remains is publishing it in
one place, one boundary defect it exposed, and the conversions it makes
possible.

## 1. The website catalog: changed, uncommitted, UNVERIFIED

This is the item the owner asked about last and it is the one with a real
unknown in it.

| Fact | State |
|------|-------|
| `website/tools/render-cli-catalog.py` consumes per-command `operators` and groups them by `available` (`with-rows` renders as "With rows") | YES, in the WORKING TREE |
| The same file in HEAD | ZERO occurrences of `operators` or `with-rows`. The support is entirely uncommitted: 217 insertions, 9 deletions |
| Does it actually render correctly against today's JSON | **NOT VERIFIED.** Rendering refuses a `zetest` binary by design, so it needs `go build -o bin/ze ./cmd/ze` first, and that is where we stopped |
| `internal/le/wikicatalog/render.go` (the wiki half) | DONE and committed in `a1507c0c2`. It holds no operator list and derives each entry from the catalog |

**First thing to do on restart:**

```
go build -o bin/ze ./cmd/ze
cd website/tools && python3 render-cli-catalog.py
```

Then read the rendered page and confirm the modifiers appear per command, with
`always` / `with-rows` / `when-streaming` distinguished. The generator reads
`pipe_items(command, "pipes")` and `"pipe-aliases"`, and **the product emits
neither key today** — measured across all 199 published commands, the keys are
`path`, `description`, `mode`, `wire-method`, `operators`, `args`,
`answer-shape`, `subcommands`, `backend`, `task-support`. Those two reads are
expected to be no-ops rather than errors, but that is an inference and it is
exactly the kind I was wrong about earlier tonight, so RUN IT.

The website tree carries a lot of other uncommitted work by another agent
(blog posts, CSS, JS, faq). The catalog generator change is one file among
them. Do not sweep the rest into a commit for it — that is how the journal
rows in `87645d0db` ended up carrying four other sessions' work.

## 2. The argument-versus-pipe boundary: two confirmed defects, one cause

Nothing owns the line between a command's arguments and its pipe chain. It
fails in both directions and both are reproducible:

```
show host              -> answers
show host cpu | json   -> answers            (the chain splits correctly)
show host all | count  -> answers            (the chain splits correctly)
show host | count      -> error: unknown section "|"
```

A command with an OPTIONAL positional argument binds `|` as that argument when
the argument is absent, and the chain is never split. Meanwhile
`ze help command --json` publishes **15 operators for `show host`**, `display`
and `match` and `count` among them. That is a published contract the runtime
does not honor, which is the precise falsehood the whole surface exists to end.

The other direction, recorded 2026-08-23 in
`plan/audit-command-pipe-vs-subcommand.md` under "an unknown trailing argument
is ignored":

```
ze show env list --nosuchflag        -> 96 rows, exit 0
ze cli -c "show env list --nosuchflag"   -> the same 96 rows
```

Neither is mine. `show host` is not a local-data command, so it never enters
the `ServeLocal` path this spec added; the `|` is swallowed during plugin
argument binding, which this spec did not touch. Both predate it.

Fix the boundary once, for every command, rather than per family. Today no
layer owns "this command takes no arguments" or "this token ends the arguments
and begins the chain".

## 3. The 51 conversions the language now makes possible

`plan/audit-command-pipe-vs-subcommand.md` holds 51 PIPE-verdict leaves:
subcommands that should collapse into operator chains now that the operators
exist. The `show host` family is 10 commands, of which 8 are section
subcommands the audit says `| display` expresses, and `show host` /
`show host all` are ONE command spelled twice — which the audit names as "the
`show bgp` / `show bgp summary` defect, unfixed, in another family". That is
the same defect `spec-cli-show-bgp-is-the-command` fixed for BGP.

**Sequencing matters: item 2 BLOCKS this one.** You cannot replace
`show host cpu` with `show host | display cpu` while `show host` eats the `|`.

## 4. `spec-cli-pipe-operator-coverage` closure

Status `verification`. Implementation complete, all 16 ACs verified against the
built product, both Review Gate defects fixed and committed (`30eb1a1d9`,
`d73f9694e`), references already rehomed to `docs/architecture/api/commands.md`
so the eventual deletion strands nothing.

It owes exactly one thing: **a review pass by a context that did not write the
code.** `commit_helper.py` refuses the closure commit without
`tmp/review/cli-pipe-operator-coverage-<session>.md`, and `review_gate.py`'s
own contract says the artifact must come from subagents or a fresh session,
never the author's inline reasoning. My session was instructed not to spawn
agents, so it could not produce one, and recording my self-review there would
have been the falsification the gate exists to prevent.

Why the gate is worth respecting here rather than routed around: six phases, a
full self-review of the diff, and a 194-case suite all returned clean while
AC-11 was false. Running the product falsified it on the first command.

When the review returns clean, closure is two commits: `create --replace` for A
(spec + journal), then `create --append --remove plan/immediate/spec-cli-pipe-operator-coverage.md`
for B. The deferral shard this handoff also named went with the whole
`plan/deferrals/` tree on 2026-09-05, so closure removes the spec alone. <!-- doc-links: ignore (the tree was deleted on purpose; this line records that it was) -->

## 5. Not left over, recorded so nobody re-derives it

- Live deferral rows in `cli-order-pipe` (2), `cli-pipe-aliases` (1),
  `cli-root-namespace-grammar` (4), `plugin-registers-pipe-operations` (2) and
  `cli-show-bgp-is-the-command` (1) are all status `deferred` with a named
  destination spec that exists. Parked correctly, not stranded.
- Other open CLI specs are not this line of work:
  `spec-ipsec-dataplane-inspection` (in-progress),
  `spec-bgp-decode-render` / `spec-bgp-pcap-decode` / `spec-support-export`
  (ready, unstarted), `spec-ze-website-0-umbrella` (design), and three `future`
  CLI specs.

## 6. Blockers, and one claim of mine that was wrong

- **`./le doc check verify` and `./le changed scope` are RED for reasons outside this
  work.** A source anchor in `docs/guide/web-interface.md` broken by another
  session's uncommitted edit to `cmd/ze/hub/aaa_authenticator_web.go`;
  `ai/PACKAGE-MAP.md` stale from their untracked `internal/core/configorder`
  and `internal/core/configvalue`; 9 misspell findings in their untracked
  `internal/component/config/toplugin_order_test.go`; and 7 doc-links
  references, 5 of them stranded by the `8f3a80bf9` closure. Regenerating
  PACKAGE-MAP would carry their packages into your commit. Leave it.
- **Verification debt: 654 open, 272 cleared. It IS clearable — run
  the retired `ze-verify-debt-clear` (current: `./le commit debt-clear`).** I first reported this as "648 open, zero ever
  cleared, structurally unclearable". That was WRONG in both halves and the
  correction is worth more than the original claim, so it stays here in full.

  The number came from `grep -c "| closed |"`, and the schema's token is
  `cleared`. A word that appears nowhere in the corpus returns zero from every
  shard, so I read a vocabulary mistake as a fact about the mechanism. The rule
  that catches exactly this, `prove a search can find before you report a
  zero`, was already in `ai/rules/evidence.md` (`5cf32fb90`, committed hours
  earlier the same day).

  I then built a structural explanation that FIT the false zero: debt is
  attributed per session, the clearing gate is tree-global, therefore a session
  can never discharge its debt while other sessions hold untracked files. Every
  premise was true and checked. The conclusion was not, because `047f64f53`
  ("judge the commit in a worktree, not the shared tree", 2026-08-22) already
  made `clear_debt` run every gate inside a throwaway worktree at HEAD, which
  carries no untracked files from anyone. Two of the three reds I cited — the
  misspell findings and the PACKAGE-MAP staleness — do not exist there. 248 of
  the 272 cleared rows went through that path.

  The lesson is the one I had just handed another session and then walked into:
  **a true premise chain does not make a conclusion measured.** I diagnosed the
  tree I was standing in rather than the gate the clearing pass actually runs,
  and `./le commit debt-clear` would have settled it in under a minute.

  What survives: debt is attributed per session and NOTHING SCHEDULES A
  CLEARING RUN. 28 shards holding 654 open rows is what the pre-`047f64f53`
  behaviour left behind. Running the pass is a real, available task for the
  next session, and it is probably worth doing before anything else here.
