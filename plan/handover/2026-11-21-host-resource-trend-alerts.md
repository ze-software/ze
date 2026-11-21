# Handover: host-resource-trend-alerts, first version

Written 2026-11-21 by the session that built the first version on the server
(`claude`), in the worktree `.claude/worktrees/host-trend-alerts`, branch
`worktree-host-trend-alerts`, based on upstream `0178ed7e22`. Thomas lands the
branch on `main` from his laptop with `git rebase worktree-host-trend-alerts`.
Nothing is pushed; a push needs his order.

Thomas's order: build a first simple version of
`plan/release/validation/spec-host-resource-trend-alerts.md`, one rule
(kernel-memory) end to end, keep the rest of the spec owed, record his
decisions D-1 (keep it in release/validation), D-3 (every trend finding is a
warning), D-4 (no off switch), move the spec to ready and claim it, implement,
commit each piece, do not close the spec. The proof he asked for: the
monitor's run loop driven by a fake clock over a fixture /proc, a leak raises
the warning and it clears when the growth stops, memory that rises and then
settles raises nothing.

Read first: the spec's `## First version` section (what is built, with its
evidence, and what stays owed), then this table.

## Start here

| # | Work | State | First action |
|---|------|-------|--------------|
| 1 | Level history and growth pattern (AC-1 to AC-3) | Committed 6cf4b12c26 | None |
| 2 | Shared confirm-and-clear machine, ddos-detect converted (AC-5 half) | Committed 2829eaf0da | None |
| 3 | Trend monitor, kernel-memory rule, doctor checks, hub start (AC-6 to AC-8, AC-12, AC-17 to AC-19) | Committed d2376b7e29 | None |
| 4 | `show host trend` and the three wiring `.ci` tests (AC-21 for kernel-memory) | Committed cfe98a9853 | None |
| 5 | Tooling: a design judge in a worktree finds its read records (AC-30) | Committed a2ee440e05 | None |
| 6 | Tooling: every path of a spec bullet owes its design pages, hook seal re-taken (AC-31) | Committed baf7b18b8c | None |
| 7 | Design approval | Approved by Thomas on 2026-11-21 (digest 1632cd34d203, 57 rows: 28 agrees, 1 departs, 28 silent). The record is local to this worktree, `tmp/spec-design/approved/spec-host-resource-trend-alerts.md` | None |
| 8 | Status `ready` and the claim | Status still `design`: the spec-edit gate owes the editing session reads of every cited source and door, and a subagent may never move a spec to ready. The claim waits on the WIP cap (12, with 18 in progress) | A main session IN THIS WORKTREE ON THE SERVER, before the branch is carried to the laptop (the approval record reaches no other checkout; only the `Design-approved:` trailer of the commit that carries the spec does): read what the gate names, set Status `ready`, commit the spec through `./le commit create` (it writes the trailer), then claim once Thomas settles the cap |
| 9 | The rest of the spec | Owed, listed in `## First version`, "Owed after the first version" | A fresh session, after 7 and 8 |

The strongest-tier model was rate-limited until 2026-11-22 06:00 Europe/London,
so every phase ran on the development tier, on the reading that the spec fixed
each design.

## Reds on the base, not this branch's

`./le arch tier check` (a core test importing a component, PPPoE VPP files
without `ze_vpp`); `cmd/ze/hub` (surface parity for the thanks commands and
plugin command builtins, the configured-plugin conformance family, and the
graphical import and web CLI tests inside the full package run);
`internal/component/plugin/all` (a BGP wire method missing from its snapshot,
a stale PPPoE allowlist entry, `environment/backup` unclaimed); `./le cli
dispatch check` (undeclarable command names in the RIB and three fixtures);
`./le doc yang-contract command-contract` (`ze-cos:cleanup`);
`internal/le/spec/roadmap` (the `doc` namespace has no summary). Each was
checked against the base or by link set.

Observation, not investigated: the `| json` pipe prints a one-key answer as
its value while `| yaml` keeps the key.
