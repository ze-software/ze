---
kind: directive
level: MUST NOT
stage:
---
**A numeric id is a position, not an identity (BLOCKING for anything you keep).**
`ze-test <suite> N` resolves `N` as a one-based ordinal over the sorted `.ci` glob,
so adding, renaming, or deleting an EARLIER file silently renumbers every test
after it. Ids are fine while you iterate inside one turn. They MUST NOT persist in
anything that outlives the turn -- a verification script, a gate subset, a
handover, a commit message claiming "8/8 green". A concurrent session added `.ci`
files mid-session and id 373 moved from `resolve-ping` to
`remove-private-as-replace-peer` while an id-driven script reported green for
tests it never ran.
