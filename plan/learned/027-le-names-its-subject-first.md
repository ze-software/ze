# 027 - le names its subject first

**Spec:** spec-le-subject-first-command-tree, closed 2026-09-25

## What the work built

Every le command is `./le <subject> <action>`, the grammar of the `ze` CLI. A
command's package sits at the directory its name predicts
(`TestEveryCommandIsFoundAtThePathItsNamePredicts`). The developer programs
folded into le: `le chaos run`, `le mrt`, `le perf`, `le build gokrazy` and
`le site terminal-demo pty`. The standalone builds and the `ze_chaos`,
`ze_analyze`, `ze_perf` and `ze_test` tags are gone.

The test harness is no longer a binary. It registers no global root, and each
of its 54 commands is `le test <name>`: one forwarding package per command
over `internal/le/test/harnesstool`. The functional runner runs its own
executable for an `le` head. Containers, QEMU guests, VPP evidence and the
terminal demo carry a linux `le` built by `internal/le/linuxle`.

The old names are declared once, in `internal/le/doc/check/retirednames.go`.
`./le doc check retired-commands` reads that map and is a full verify stage,
so a tracked file that names an old form goes red.

## Decisions

- Three phases in one spec: register the new names, move the callers, delete
  the old names. Old and new names lived side by side only while peer
  sessions still called the old names. That was an owner-approved,
  time-bounded exception to no-layering.
- A retired alias rewrote argv and dispatched again. One mechanism covered a
  rename, a merge (`verify lock` into `job`) and a verb split.
- D-8: the harness links into le. Keeping a harness binary would have needed
  a second registration of every handler as a ze root.
- Historical records (journal, learned, audits, published posts) are gate
  exceptions. They are not rewritten, because a record states what ran on its
  day.

## Traps

- A text sweep cannot see an argv that Go builds from split string words
  (`[]string{"repo", "changed"}` written as `"changed"` alone). After the alias
  was deleted, three fixtures still sent the old words and failed only in the
  functional suites.
- A digest seal over HEAD's committed bytes trails its change by one commit.
  A commit that changes a sealed package reddens the seal, and the next commit
  re-seals it.
