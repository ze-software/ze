# Session handover: the CLI operator work

**Session `api`** (`ca1b4c65` / `7e4e9f00`), 2026-08-23.

Goal given: *complete cli-show-bgp and before that its dependencies.* That goal
is met in the product. One spec is implemented and verified but **not closed**,
for a reason that is not a judgement call — see [section 2](#2-the-one-spec-that-did-not-close-and-why).

The detailed CLI worklist is committed at `plan/handoff-cli-remaining.md`, with
the commands to run. This file is the session-level view: what to do first,
what I got wrong, and what is not yours to touch.

Standing owner instructions still in force:

- *"I do not want an half-existing. I want the feature so that the website
  documentation can be improved with what each command supports. And make sure
  ALL the command support ALL the modifier which make them useful."*
- *"when written implement it, do not ask, start."*
- *"make sure we commit work in logical chunk as we go along"*
- `show bgp rib` gets FLAT ROWS: one envelope, one row per route, `peer` and
  `direction` as fields. Owner ruling, already implemented.

## 1. Do these first, cheapest to dearest

**1. Clear the verification debt.**

```
make ze-verify-debt-clear
```

Under a minute to start. 654 open debt rows across 28 shards, and *nothing
schedules a run*. Since `047f64f53` the pass judges each commit in a throwaway
worktree at HEAD, so other sessions' untracked files do not redden it. 272 rows
have already been cleared that way. See [section 4](#4-where-i-was-wrong-because-it-is-more-useful-than-where-i-was-right)
for why I first reported this as impossible.

**2. Render the website catalog.**

```
make ze-build
cd website/tools && python3 render-cli-catalog.py
```

**The thing the owner asked about last, and the only item with a real unknown
in it.** `website/tools/render-cli-catalog.py` has been changed to publish
per-command modifiers grouped by availability — but that change is
**uncommitted** (217 insertions; HEAD holds zero occurrences of `operators` or
`with-rows`), and it has never been run end to end. Rendering refuses a
`zetest` binary by design, which is where we stopped. The wiki half *is* done
and committed (`a1507c0c2`).

When it renders, confirm each command shows its modifiers with `always` /
`with-rows` / `when-streaming` distinguished. The generator reads a `pipes` and
a `pipe-aliases` key that the product emits for **none** of its 199 commands. I
expect harmless no-ops. That is an inference, and inference is what I got wrong
twice today, so run it rather than trust it.

> **Caution.** The website tree carries a lot of other uncommitted work by
> another agent — blog posts, CSS, JS, faq. Commit the generator alone.
> Sweeping siblings is exactly how `87645d0db` ended up carrying four other
> sessions' journal rows.

**3. Fix the argument-versus-pipe boundary.** Two confirmed defects, one cause,
and it blocks the 51 conversions. Section 2 of `plan/handoff-cli-remaining.md`.

## 2. The one spec that did not close, and why

`plan/spec-cli-pipe-operator-coverage.md`, Status `verification`.

Implementation complete. All 16 ACs verified against the **built product**, not
the diff. Both Review Gate defects fixed and committed. References already
rehomed to `docs/architecture/api/commands.md`, so deleting the spec strands
nothing — which is not hypothetical: five references are stranded in the tree
right now from the `8f3a80bf9` closure doing the opposite.

It owes exactly one thing: **a review pass by a context that did not write the
code.** `scripts/dev/commit_helper.py` refuses the closure commit without a
review artifact under `tmp/review/`, and `scripts/dev/review_gate.py`'s own
contract says that artifact comes from subagents or a fresh session, "never the
author's own inline reasoning". This session was instructed not to spawn
agents, so it could not produce one. Recording my self-review there would have
been the precise falsification the gate exists to prevent.

This was not caution for its own sake. The self-review of the whole diff
returned 0 outstanding issues. Pre-Commit Verification then ran the product and
**AC-11 was false on the first command**: `ze show env list` and
`ze cli -c "show env list"` did not answer the same bytes, because the CLI
client called `fmt.Println` over an already-newline-terminated rendering. Six
phases, a full self-review and a 194-case suite all passed while that was true.
Nothing in the suite compared two surfaces of one command *to each other*. That
is the argument for the gate, made by the gate.

When the review returns clean, closure is two commits:

| Commit | Command |
|--------|---------|
| A | `create --replace` with the spec + the journal row |
| B | `create --append --remove plan/spec-cli-pipe-operator-coverage.md` and `--remove plan/deferrals/cli-pipe-operator-coverage.md` |

That shard holds zero rows; do not leave it behind as an empty file.

> **Note.** The closure trigger is the journal row's **Spec column**. A row
> naming the spec declares a closure whether you meant one or not — it refused
> two of my commits today. While the spec stays open the row carries `-` and
> names the spec in its text instead.

## 3. What landed

25 commits, session `ca1b4c65`.

Specs closed: `record-answers-1`, `record-answers-2`,
`record-answers-3-zero-alloc`, `cli-show-bgp-is-the-command`.

The operator language, built from nothing:

| File | What it does |
|------|--------------|
| `internal/component/command/pipe_catalog.go` | 17 operators with class, argument kind, repetition rule, shapes, description. **The** single source. Five hand-copied lists deleted |
| `internal/component/command/answer_shape.go` | `RegisterShape`, and the row extraction the row operators act through |
| `internal/component/command/pipe_save.go` | `\| save <path>`, atomic, mode 0600 |
| `internal/component/command/local_data.go` | `ServeLocal`, the path by which a command served in the client's own process reaches the pipe layer at all. 38 commands reached none before |
| `cmd/ze/help_command.go` | Publishes per command what its shape derives. The `global-pipes` boolean, which claimed the same thing for 381 commands, is gone |

Two refusal paths: from the **declared** shape before dispatch, and from the
**answer** at apply time. The second is universal, so an undeclared command is
refused too. `show bgp rib` streams flat rows in one deterministic order on
both paths.

Behaviour, measured at closure: 95 rows bare, 7 after `| match PATH`, 1 after a
second `| match GO`. `| fill alpha | fill alpha` and `| json | yaml` refused by
name with a reason. `| resolve` refused where no field is declared to hold an
address. `show schema protocol | first 1` refused before it runs.

**Deviation worth knowing** (D-1 in the spec): AC-2 said an undeclared command
defaults to `doc` and refuses row operators. Built instead as `with-rows`,
decided from the answer. The spec's rule would have bought honesty with a
regression — 232 of 252 commands declare nothing and most do answer rows.

## 4. Where I was wrong, because it is more useful than where I was right

I made one claim to Thomas that was wrong in both halves, and it is worth
reading before you trust any other number in my notes.

I reported verification debt as *"648 open, zero ever cleared, structurally
unclearable"*. Real figure: **654 open, 272 cleared.**

- The zero came from `grep -c "| closed |"`. The schema's token is `cleared`. A
  word that appears nowhere returns zero from every shard, and I read a
  vocabulary mistake as a fact about the mechanism. `ai/rules/evidence.md`
  carries the rule that catches this — *prove a search can find before you
  report a zero* — and it landed in `5cf32fb90` hours before I made the error.
- I then built a structural explanation that **fit** the false zero: debt is
  per-session, the clearing gate is tree-global, therefore nobody can ever
  discharge it. Every premise was true and independently checked. The
  conclusion was not, because `047f64f53` already made the pass run in a
  worktree at HEAD.

The lesson, which I had handed another session an hour earlier and then walked
into myself: **a true premise chain does not make a conclusion measured.** I
diagnosed the tree I was standing in rather than the gate the clearing pass
actually runs. One command would have settled it.

The correction is committed in full in `plan/handoff-cli-remaining.md` section
6 rather than silently edited out.

Four other mistakes are in the spec's Mistake Log (M-1 to M-10). The two most
transferable: a `strings.Contains` assertion that passed for `raw` while `raw`
was absent, because `json` is a substring of `ndjson`; and changing
`show bgp rib` to flat rows without sweeping for consumers, which broke
`lg-graph-lab` for real.

## 5. Not yours to touch

Another session (`fix`) holds **untracked** work in this checkout:
`internal/core/configorder/`, `internal/core/configvalue/`,
`tomap_shape_test.go`, `toplugin_order_test.go`,
`caps_declaration_lint_test.go`, `lab_docker_rm_test.py`.
<!-- doc-links: ignore (these are another session's UNTRACKED files; naming them as resolvable paths would be latent-green here and dead in a fresh clone, which is the very trap this section documents) -->

Consequences you will meet and should **not** "fix":

- `ze-lint-changed` is red: 9 misspell findings, all in their
  `toplugin_order_test.go`.
- `ze-doc-verify` is red: `ai/PACKAGE-MAP.md` stale from their untracked
  packages, plus a source anchor in `docs/guide/web-interface.md` broken by a
  third session's uncommitted edit to `cmd/ze/hub/aaa_authenticator_web.go`.
- **Regenerating `ai/PACKAGE-MAP.md` would carry their packages into your
  commit.** I did that once already today and had to recover with
  `git show HEAD:<path> > <path>`. Leave it.
- `ze-doc-links-check` is red on 7 references, 5 of them stranded by the
  `8f3a80bf9` closure. None is ours.

Four journal rows in my own `87645d0db` cite files from that untracked set. My
commit carried them; I did not write them. It added nine rows and exactly one
was mine. **If a doc gate reddens on a citation, attribute by the row's Spec
cell and subject, not by the commit that carried it** — the carrier is nearly
always innocent, and `git log -S` cannot tell you the difference.

Related and unbuilt: `path_resolves` in `scripts/dev/check_doc_links.py` uses
`os.path.exists`, so it measures the *checkout*, not the tracked corpus. A row
citing untracked code is therefore not red, it is **latent**: green on every
machine carrying the work, broken only in a fresh clone or CI. That is a
2026-08-14 row in `plan/journal/reference-checked-claim-unchecked.md` against
the same function, still with nothing built for it.

## 6. Cross-session state

The `system` session is **out of tokens** and has cancelled the hourly
check-in cron. Its handover is `for-later.txt` in this directory — read it, it
holds a `.ci` predicate checker never started and a
`TestPrefixExceedTeardown` assertion waiting on Thomas. **Do not wait on
`system` for the review pass this spec is owed.**

Restart its cron with:

```
/loop 1h Ask api and fix whether they hit SYSTEM/tooling problems since last
time, and whether they are stuck on a question to Thomas while idling.
```

## 7. Two systemic things for Thomas, not fixable in passing

1. **A session instructed not to spawn agents cannot close any spec.** Not
   hypothetical — it stopped this session. The only exits are faking the review
   artifact or not closing, and I chose not closing. Either the gate needs a
   route for such a session, or such sessions should not be given specs to
   close.
2. **Nothing schedules a debt-clearing run.** The mechanism works and is
   correctly scoped since `047f64f53`; it simply never fires unless someone
   types it. 654 rows is what that looks like.
