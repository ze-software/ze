# 22 Fix the two largest shipped-code root causes: refuse-by-default, then YANG as the single source

Owner order (2026-10-23): implement these two specs, in this order, each in its
own fresh session:

1. `plan/release/fixes/spec-refuse-instead-of-defaulting.md` (P1, 51 owned journal rows)
2. `plan/release/fixes/spec-yang-single-source-of-truth.md` (P1, 51 owned journal rows)

## Why these two

On 2026-10-23 the 1,350 open journal rows were analysed for shared causes and
homed in about 30 specs (commits 1b41ab9fbb through 9204d040ee). About 1,240
rows stay open, and every one is listed in an `## Owned journal rows` table.
These two specs are the largest families in shipped Ze code, and both affect the
release:

- Refuse-by-default: code that cannot answer returns a plausible value. Examples
  verified at the producer: RADIUS accepts a reply with no Message-Authenticator
  (`verifyResponseMessageAuthenticator`), LDP hands out a label already in use
  (`allocateLabelLocked`), ADD-PATH path identifier 0 is read as "no path id"
  (`internal/component/bgp/rib/route.go`), health checks answer a constant
  (`ebpfSupported`, l2tp `checkHealth`).
- YANG as the single source: Go re-declares what the schema says. YANG `uint64`
  is read as 32-bit and `decimal64` as a string (`yangTypeToValueType`),
  `config validate` walks a hand list of 21 sections (`validatedSections`),
  leaves such as `auto-commit-delay` have no reader, and command handlers read
  untyped positional arguments (`injectRoute`).

Each spec carries its own Current Behavior, fix design, ACs and the full row
table. Read the spec, not this summary.

## How to start each session

| Step | Command or action |
|------|-------------------|
| 1 | `git fetch`; `git rev-list --count HEAD..@{u}` must be 0 |
| 2 | `./le spec wip` (10 of 12 on 2026-10-23, so there is room) |
| 3 | `./le spec claim spec <spec path>`, then run `/ze-implement` through a subagent (`ai/rules/planning.md`) |
| 4 | Before writing code, re-verify each family's producer at HEAD: line numbers in the specs drift |

Dependencies:

- Refuse-by-default depends on `plan/release/fixes/spec-plugin-optional-dependency-refusal.md`
  (status `ready`). That spec owns the "feature not compiled in reads as not
  configured" rows. Reuse its refusal; do not duplicate it.
- YANG source coordinates with `spec-config-yang-when-unique-choice.md` and
  `spec-config-yang-rpc-mandatory-defaults.md`, which own the RFC 7950 rules.
- One row overlaps: the union list key that `yangTypeToValueType` turns into a
  string sits in refuse-by-default (F2), but its fix is the YANG derivation.
  Whichever spec runs second checks it is already gone.

## Things that bit this session

- `./le verify current fast` widens to the whole tree when the last verify was
  interrupted, and prints nothing on the terminal for its whole run (45 minutes
  on 2026-10-23; owned by `spec-gates-report-cause-and-progress`). Check
  `./le verify status check` first.
- No whole-tree testing on this machine (owner order, later on 2026-10-23): no
  verify of any kind, no staticcheck matrix piece, no full lint, no
  `./le test unit all`, no repo-wide compile. Prove the work with scoped package
  tests under `./le job run` and scoped lint; the full verify runs on another
  host.
- The disk fills fast when several sessions build at once. On ENOSPC run
  `./le scratch store-trim`, then `./le scratch cache-clean`.
- The carried "staticcheck part 6/6 red" was an interrupted run, not a finding.
  Since 305a92cb6d the gate reports an interrupted stage as never judged and
  charges nothing, so no override is needed. Refresh `bin/le` with
  `./le --update` if it is older than the gate code. A real red still blocks.
- Scoped lint refuses with "lint inputs changed during planning" while other
  sessions edit the tree. Retry when it is quiet, and say in the commit body if
  it never ran.
- A stale `.git/index.lock` (empty, no owning process) happened twice. When a
  commit script says the shared index could not be pointed at the commit, it
  prints the repair; agents are refused it, so ask Thomas to run it.

## New journal rule (in force from 987bd0983a)

Every open journal row is listed in the owned journal rows table of the spec
that will fix it, and a commit adding or changing an open row with no owner is
refused (`plan/journal/README.md`, "Owned rows"). When these specs fix a row,
set its Fix cell to `fixed: <evidence>`. A defect found along the way gets a row
and a table entry in the spec that fits.

The review findings on that check are fixed (110ed77106, a605b367e4,
70cfbdba38). A two-commit closure is judged against the script's earlier
blocks: when you close a spec, every row in its owned table must be
`fixed: <evidence>` in commit A or re-homed to another spec's table, or
commit B is refused.

## Other owed items from session 895e52ca

- `docs/contributing/committing.md` owes a one-line pointer to the new
  ownership refusal. Another session's uncommitted edits held the file.
- A full verify over the 2026-10-23 commits has not run; it needs another host.
- About 46 open rows cite deleted scripts (`scripts/dev/`, `mk/`,
  `.claude/hooks/*.py`). Each belongs to a spec, which judges it on
  implementation.
- The root-cause analysis is in
  `tmp/session/2026-10-23-895e52ca-087b-4d00-9f7b-24de74193df6/scratch/journal-root-causes.md`
  (session scratch, not durable). The specs carry everything that matters.
