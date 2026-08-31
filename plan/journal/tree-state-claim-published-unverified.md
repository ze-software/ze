# A tree-state claim is published without being checked at its producer

`ai/rules/evidence.md` governs claims about what CODE DOES, and it works: a
session about to say what a function returns reaches for the function, because
the rule says so in as many words and because reading it is the obvious move.

It does not govern claims about the STATE OF THE TREE. Who owns this file. Is
this red at HEAD. Is this gate in that stage set. Was this defect reachable
before my change. Does this string exist in the repository. Those feel like
OBSERVATIONS rather than claims, so nobody treats them as needing a producer,
and in a checkout several sessions share they are the least stable facts
available: a `git status` mixes every session's work, a gate reads the working
tree rather than HEAD, a binary is built from a tree that has moved, and a
stack trace is evidence about an agent's tree rather than about HEAD.

There are two tells, and the weak one is the floor rather than the test.

The FLOOR: a sentence about the repository that names no command you ran and no
file you opened. That catches the cheapest instance, an ownership guess assembled
from who was working nearby, and it should not take a subtler test to catch one.

The TEST, because the floor lets through the two instances that travelled
furthest: **a sentence about the repository whose evidence does not name the TREE
it was measured against.** Both of the costly ones below carried real evidence. A
`./le` invocation answered `unknown command`, from a binary built out of a tree
nobody ships. A stack trace named four functions and a line, and was a
measurement of one agent's uncommitted tree, published as a fact about HEAD.
Naming a command is not enough, because the question is never "did you run
something" but "which tree answered".

A stale binary measures a tree that no longer exists. A stack trace measures the
tree that produced it. `git status` measures every session's tree at once. A gate
that walks the working tree measures whoever's unstaged edits are present. HEAD
is a different tree from the working tree, and neither of them is "the
repository".

So the question to ask before publishing: **would this sentence still be true in
a clean checkout at HEAD?** If yes, say so. If no, name the tree IN the sentence,
because a reader who is not told will take the safest reading, and the safest
reading is always HEAD. That assumption is what made two of the rows below
alarming rather than merely wrong.

Not one instance below was caught by a gate. Every one was caught by a person
re-measuring and saying so out loud.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-31 | yang-short-and-long-command-help | ownership of a broken package, reported across sessions | `internal/le/rfc/` did not compile mid-rename. This session read `git status`, saw eleven dirty files, knew which session had worked that package all night, and named it on the bus. A second session had already made the same inference and passed it on, so the guess arrived pre-confirmed. It was wrong: the re-seal comment in `native_fixture_test.go` reads "Re-sealed 2026-08-31, for phase 1 of spec-rfc-tag-claim-discrimination" and names the real owner, in the file, the whole time. The accused session had never edited `selftest_core.go` in any commit | corrected on the bus to both sessions. The general practice: a shared checkout has no "who is editing this path" query, so ownership is read from a marker the file itself carries or from the owner's own testimony, never inferred from who was working nearby. Sessions that write their spec name into a file they touch are the only reason the answer was recoverable, and nothing requires them to |
| 2026-08-31 | yang-short-and-long-command-help | `./le cli-grammar`, recorded as a spec baseline | This session ran it, got `cli-grammar: OK` exit 0, and wrote it into a spec as a property of HEAD. A peer ran it an hour later, got exit 1 with twelve R9 findings, and wrote THAT into a journal row as a property of HEAD. Both were wrong the same way: `Check` walks the tree on disk with `filepath.WalkDir` (`cligrammar.go`, `flags.go`), so it judges whatever every session has uncommitted at that instant. The whole difference was a third session holding two `-cmd.yang` modules mid-rewrite | both records corrected to name the measurement time and the tree read. The general practice: a tool that walks the working tree answers about a moment, never about a commit, so its result is recorded with a timestamp and never as a baseline. Compare the deltas a known change PREDICTS rather than totals, because a total moves under both observers |
| 2026-08-31 | yang-short-and-long-command-help | membership of `doc check verify` in the pre-commit stage set | This session grepped `internal/le/verify/` for a doc stage, read an empty-looking result, and told the owner the new gate did NOT make `./le verify worktree` red. It does: the fixed pre-commit stage table in `internal/le/verify/engine/stages.go` names `stage("doc check", "verify")`. The match was present and `// Design:` header lines had crowded it out of a truncated view | corrected to the owner in the next message. The general practice: a NEGATIVE claim from a search is the easiest one to get wrong, because absence of output and absence of the thing look identical. A grep whose output was truncated has answered about the truncation |
| 2026-08-31 | - | a security finding's reachability at HEAD, found by the rfc-drain session | A session read an agent's panic stack trace and reported to this one that an unauthenticated PPP peer could crash the session goroutine, as a live shipped defect. The trace was evidence about that agent's WORKING TREE. At HEAD the `LCPOptMagic` arm of `negotiatePeerOption` answered with two echoed octets, 700 of which fit the frame; the session's own in-flight change made each reply six octets, and the bound landed in the same tree | corrected by its author unprompted, after this session had already cited it in a spec's Security Review Checklist where a reviewer would have read "remote panic" as shipped. The general practice: a stack trace, a red test and a gate verdict are each evidence about the tree that produced them. Reachability at HEAD is a separate claim needing its own measurement |
| 2026-08-31 | - | a YANG description collision, reported across sessions | A session reported seeing `WARN YANG command description mismatch node=peer` several hundred times and quoted both sides. One side, "Scope a command to the peers a selector matches.", exists nowhere in the repository. A clean binary captured the warning set either side of an unrelated merge change: one line each time, zero mismatch warnings, `ze help command --json` byte-identical at 554684 bytes | not reproduced and not fixed, because there was nothing to fix. Recorded because the report was specific, quoted, and repeated a number of times that made it feel measured. A quoted string is not evidence that the string exists |
| 2026-08-31 | - | a journal row naming a command, written by the rfc-drain session | A session filed a row against a `./le` command it believed did not exist. It did | corrected by its author. Same class: a claim about what the repository CONTAINS, published without running the one command that would have settled it |
