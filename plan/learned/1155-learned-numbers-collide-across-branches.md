# 1155 -- learned-next cannot prevent duplicate numbers across branches

## Context

A `git pull --rebase` of 12 local commits onto 25 upstream ones conflicted on
`plan/learned/.counter` (local 1125, upstream 1129). Resolving the counter was
trivial, but the conflict was a symptom: the local VRRP branch and the upstream
rib-arch branch had each allocated 1120, 1121, 1122, 1123 and 1124 for entirely
different summaries. The rebase merged both sets, so five numbers each named two
files. A sweep found 22 such numbers across the corpus, 26 excess files in all.

## The trap

`ai/rules/git-safety.md` claimed `learned-next` meant "concurrent sessions cannot
allocate the same number". That is true only within one working tree.
`learned_next` (`scripts/dev/commit_helper.py:1120`) computes
`number = max(highest + 1, counter)` where `highest` comes from
`learned_dir.glob("[0-9]*-*.md")` -- the LOCAL filesystem -- then creates the file
with `O_EXCL` (`:1122`). The exclusive create is what defeats a racing session in
the same tree. It says nothing about a branch whose files are not checked out.

So two branches allocate the same number by construction, and the duplicate
surfaces only when they meet. Nothing detected it: `learned_index.py` globs and
renders whatever it finds, so `ai/LEARNED-FULL-INDEX.md` simply grew two rows
with the same `#`, and no check asserted uniqueness.

Note what the counter is NOT: a defence. Because allocation takes
`max(highest + 1, counter)`, a stale low counter cannot cause a collision -- the
file-prefix scan backstops it. Resolving the `.counter` conflict to 1129 rather
than 1125 was right (it is the true high-water mark) but would not have prevented
anything on its own. The counter is bookkeeping; the file prefixes are the truth.

## Resolution

`scripts/dev/learned_numbers.py` now owns the invariants: unique numbers, H1
number matching the filename, `.counter` above the highest. `--check` runs inside
`make ze-doc-test` and `make ze-regen-check`; `--fix` resolves collisions.

The `--fix` policy is keep-the-most-referenced: in each colliding group the
summary cited most often elsewhere in the tree keeps the contested number, ties
break by earliest add-commit. Applied to the 22 groups this touched ZERO `.go`
files, because every summary cited by a `// Design:` header (631-host-0-inventory
with 27 references, 1124-vrrp-first-hop-redundancy with 43, 962-ospf-8-spf-rib
with 10) was the most-referenced in its group and kept its number. Renumbering by
"newest loses" or "upstream wins" would have rewritten those headers instead. The
cheap rule and the safe rule coincided; only 3 external reference sites moved.

## Reusable

- A uniqueness guarantee that reads the working tree is a guarantee about ONE
  tree. Any allocator scanning local files (numbered summaries, fixture IDs, port
  assignments) collides across branches, and merge is where you find out. If two
  branches may allocate from one namespace, a check at merge is the only defence.
- When renumbering or renaming within a corpus, rank by inbound references and
  keep the most-referenced fixed. Churn concentrates in the few heavily-cited
  items; leaving those alone is both the smallest diff and the lowest risk.
- Generated indexes launder duplicates into something that looks fine. Two rows
  with the same number render perfectly. Uniqueness has to be asserted, not
  assumed from a generator that never complained.

## Files

None recorded.
