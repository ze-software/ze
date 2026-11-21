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
| 7 | Design approval | Judged approve (digest 1632cd34d203); `./le spec design check` says ready to present | Thomas runs, from his own terminal in the worktree: `./le spec design approve spec plan/release/validation/spec-host-resource-trend-alerts.md words "<what he said>"` |
| 8 | Claim and status | Not claimed: the WIP cap is 12 and 18 specs are in progress | Thomas decides: close specs first, or authorise this claim above the cap. Then the session sets Status `ready`, runs `./le spec claim`, and the commit carrying the spec takes the `Design-approved:` trailer from the local record |
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
