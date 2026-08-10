# AI code-navigation audit: what Ze has, and the three gaps that slow rediscovery

*Audit date: 2026-07-05. Author: Claude (Opus 4.8), from a three-way sub-audit of the AI-assistance layer. Evidence is cited as `file:line` throughout.*

## The short version

You asked what would help me find answers faster when I keep rediscovering how the
code works. I audited the whole AI-assistance layer. The honest answer is that you
do not have too few references. Ze carries one of the deepest AI-navigation layers I
have worked in:

| Layer | Count |
|-------|-------|
| Behaviour rules (`ai/rules/`) | 80 |
| Rule rationales (`ai/rationale/`) | 44 |
| Pattern cookbooks (`ai/patterns/`) | 8 |
| Skills / workflows (`ai/skills/`) | 26 |
| Learned summaries (`plan/learned/`) | 1082 |
| Architecture docs (`docs/architecture/`) | 100 |
| Total docs (`docs/`) | 273 |
| Generated code-to-docs edges (`ai/CODE-TO-DOCS.md`) | 1186 |
| Per-file `// Design:` comments across `.go` | ~5717 |

Adding more of the same will slow me down, not speed me up. The friction is not
volume. It is two structural gaps.

The whole layer is tuned for **writing new code correctly**: rules, gates, patterns,
"before you..." tables, prohibitions. It is thin on **orienting in existing code**:
what a package is, what it does, how data moves through it. And the knowledge I build
while working (I traced this flow, it goes A to B to C) is thrown away when the
session ends, so the next session re-traces the same flows from source.

Three artifacts close most of the gap. In priority order, all buildable from data you
already keep:

1. **A generated package map.** One line per package saying what it does. This is the
   direct answer to "what does what". Roughly 90% auto-generatable.
2. **A generated doc-to-code reverse index.** Given a design doc, the files that
   implement it. Inverts the 5717 `// Design:` comments you already maintain.
3. **A living per-subsystem digest.** The current flow and key files per subsystem,
   git-tracked and kept fresh. This is the real meaning of "more lessons learned": not
   more history (you have 1082 summaries), a living current-state map.

Then a piece of hygiene: retire or regenerate the hand-maintained indexes that have
gone stale, and stop hand-writing what a script can emit.

## What already exists (so we build on it, not beside it)

Before proposing anything, here is the current discovery surface. The risk with this
request is answering "you are missing X" by adding a seventh index, which makes the
navigation problem worse. Most of what an agent needs is already here:

| Surface | Answers the question | Form |
|---------|----------------------|------|
| `CLAUDE.md` / `ai/INSTRUCTIONS.md` | prohibitions, arch summary, source layout, highest-stakes "before you" | generated |
| `ai/INDEX.md` | task to docs, keyword to doc, keyword to RFC, dev tools, arch doc list | hand |
| `ai/rules/` (80) + `ai/rules/INDEX.md` | how to behave, how to add code correctly | hand |
| `ai/patterns/` (8) | mechanical recipes for CLI, plugin, config, family, test | hand |
| `ai/rationale/` (44) | why each rule exists | hand |
| `ai/skills/` (26) | end-to-end workflows (implement, review, debug, audit) | hand |
| `ai/CODE-TO-DOCS.md` | code path to the docs that describe it | generated (`make ze-doc-index`) |
| `ai/LEARNED-INDEX.md` | a curated slice of learned summaries by topic | hand |
| `plan/learned/` (1082) + `DESIGN-HISTORY` / `RECURRING-PATTERNS` / `HOOK-FRICTION` | history: decisions, recurring traps, hook false positives | hand + append |
| `docs/architecture/` (100) | canonical design of each subsystem | hand |
| `// Design:` + `// Detail/Overview/Related` per file | which doc governs a file, which siblings relate | hand, hook-enforced |

This is genuinely strong. `ai/INDEX.md` alone maps hundreds of keywords to docs and
RFCs, and `CODE-TO-DOCS.md` is a real generated reverse index. The gaps below are not
"you have nothing", they are "the last mile of orientation is missing".

## The diagnosis: two root causes

**Root cause 1: the layer describes rules and history better than it describes the
code itself.** When I need "which package owns route selection" or "what does
`internal/component/managed` actually do", there is no compact source. The generated
arch map lists 134 directory names with zero descriptions each
(`scripts/dev/arch_map.py` emits `", ".join(names)` and nothing more). The
keyword table in `ai/INDEX.md` covers popular topics but is not an enumeration of
packages. So I open packages one at a time, and 42% of them have no package doc
comment to read (356 of 610 package directories carry `// Package ...`; the rest do
not).

**Root cause 2: session knowledge is not persisted.** A fresh session auto-loads
exactly two things: `CLAUDE.md` (198 lines) and about 15 lines of stdout from
`session-start.sh`. Both are pointers, not knowledge. When a session ends, the only
per-session state written (`tmp/session/session-state-*.md`) is a git-status file
list, it is gitignored, and it is deleted after 24 hours (`session-start.sh`,
`.gitignore:12,51-55`). The digest format that `post-compaction.md` describes
(`peer.go (380L): Peer struct, FSM transitions...`) is exactly the right shape, but
nothing enforces it and its sink is the same ephemeral file. So the code-flow tracing
that `architecture.md` mandates before every buffer or call-site change
is discovered, used once, and discarded. The next session starts cold.

Everything below follows from these two.

## Gap 1: no "what does what" package map (highest leverage)

**Symptom.** To learn what a package does, I open it. There is no single artifact that
lists every package or plugin with a one-line responsibility.

**Evidence.**
- `scripts/dev/arch_map.py` generates only comma-separated directory-name lists
  (the `arch-components` / `arch-system-plugins` / `arch-bgp-plugins` blocks in
  `CLAUDE.md`). Names, no descriptions.
- Package doc coverage is ~58% (356 of 610 dirs). Confirmed missing on real packages:
  `internal/plugins/dhcpserver`, `internal/plugins/static`, `internal/component/ike`,
  `internal/core/textbuf`, `internal/core/observation`.
- The descriptions **already exist** for plugins but are surfaced only at runtime. The
  registry carries `Description string` (`registry.go`), and 106 `register.go`
  files set a real one-liner, for example `internal/plugins/ospf/register.go`
  ("Open Shortest Path First v2 (RFC 2328): native link-state IPv4 IGP"). The only
  consumers are help text and the TUI (`registry.go`, `cmd/ze/help_command.go`).
  An agent sees them only by running `bin/ze help`.

**Fix.** Generate `ai/PACKAGE-MAP.md` (or root `PACKAGES.md`): one row per package and
plugin, `path | one-line responsibility | key type / entrypoint`. Populate it by
joining two sources you already keep:
- Plugins and components: `reg.Description` (the registry already exposes it via
  `registry.All()`; `inventory.go` already reads it).
- Documented packages: the first sentence of the `// Package ...` comment.
- Undocumented packages (~254): emit as `TODO`. The artifact then doubles as a
  doc-coverage backfill driver, which is a feature, not a defect.

**Cost.** Low. Extend `scripts/dev/arch_map.py` (already walks the trees) or
`scripts/inventory/inventory.go` (already imports the registry and renders a
Description column, `inventory.go`), and wire it into the existing
`ze-doc-test` / `ze-regen` freshness gate (`mk/inventory.mk`) so it cannot rot.

**Why first.** It is the literal answer to your complaint, it is ~90% generatable from
metadata you maintain, and the freshness gate keeps it honest. This is the single best
return on effort in the whole audit.

## Gap 2: no doc-to-code reverse index, and the ADR index is dead

**Symptom.** When the map says "subsystem X is governed by `docs/architecture/Y.md`,
go change or verify the code", I have to grep for the implementing files every time.

**Evidence.**
- Every non-test, non-generated `.go` carries a forward `// Design: <doc>` header,
  hook-enforced (`ai/rules/go-standards.md`, `pretool-writeedit.py`),
  about 5717 edges. `ai/CODE-TO-DOCS.md` also runs code to docs. Both are **forward**.
  The inverse (doc to the `.go` files that implement it) is not materialized anywhere.
  There is no `make` target and no `DOCS-TO-CODE.md`.
- The natural by-topic home for decisions, `docs/architecture/decisions/`, holds
  exactly one ADR (`001-pull-model-metrics.md`) plus a `README.md` index. So "what did
  we decide about Y and why" is not lookup-able through the ADR system. It is
  re-derived from `DESIGN-HISTORY.md` prose plus raw summaries.

**Fix.**
- Generate `ai/DOCS-TO-CODE.md` by inverting the `// Design:` edges (the same data
  `code_to_docs.py` already parses). One heading per design doc, the list of files
  under it. Free, because the forward data already exists.
- Either revive the ADR habit (record real decisions as ADRs so they have a topic
  home) or delete the near-empty directory and point decision-hunting at
  `DESIGN-HISTORY.md` explicitly. A one-entry ADR index is worse than none, because it
  signals a lookup path that does not actually work.

**Cost.** Low for the reverse index (one generator, one gate). The ADR question is a
process decision, not code.

## Gap 3: no living per-subsystem understanding (the real "more lessons learned")

**Symptom.** Each session that needs to understand a flow, say how an UPDATE moves
from receive through the reactor and pools to egress, re-traces it from source. The
rules mandate the tracing (`architecture.md`, "Memory Lifecycle Tracing"
and "Sibling Call-Site Audit") but give it no durable sink.

**Evidence.**
- The 1082 learned summaries are per-completed-spec, historical, append-only. They are
  excellent as a record (each had decisions, consequences, gotchas and
  files) but they answer "why did we change this in spec N", not "how does subsystem X
  work right now".
- `docs/architecture/*.md` is curated canonical design, human-gated, prescriptive
  ("All new code MUST follow"). Agents do not, and should not, dump raw trace findings
  there.
- The only mid-session persistence is the ephemeral, gitignored, 24-hour file above.
  Nothing survives to session N+1.

So the one artifact that would kill the re-derivation, a current-state flow digest per
subsystem, is exactly the one that does not exist. This is what you were reaching for
with "more summary of lessons learned". You do not need more history. You need a
living map.

**Fix.** Create `ai/digests/<subsystem>.md` (git-tracked), one per major subsystem
(reactor, pools/wire, config pipeline, plugin transport, RIB, CLI, web). Each holds
the current flow (entry to exit), the 5 to 10 load-bearing files with a one-line role
each, and the current gotchas. Seed them the first time a session traces the flow.
Keep them fresh with a light discipline: when a session finishes non-trivial work in a
subsystem, it updates that subsystem's digest before writing the learned summary.
Optionally back it with the same `// Design:` freshness ideas so a digest that names a
deleted file gets flagged.

**Cost.** Medium, and it needs discipline, not just a script, because the content is
semantic. Start with two or three subsystems where I re-trace most often (reactor and
the buffer/pool path are the obvious first two) rather than all at once.

**Risk to manage.** A living doc that goes stale is worse than no doc. Two mitigations:
keep each digest short (a stale 30-line digest is easy to spot and fix, a stale 300-line
one is not), and prefer naming files and symbols that a gate can verify still exist.

## Cross-cutting: staleness and index sprawl

The hand-maintained indexes are already trailing the corpus, which quietly erodes
trust in every index:

- `ai/LEARNED-INDEX.md` is curated by design and cites 237 of 1078 summaries (~22%).
  The other ~840 are discoverable only by `ls plan/learned/` and reading NNN-slug
  filenames.
- `DESIGN-HISTORY.md` still says it was extracted from 638 summaries; the corpus is
  past 1066.
- The ADR index has one entry.

Two principles worth adopting:

1. **Generate and gate what can be generated.** The generated surfaces
   (`CODE-TO-DOCS.md`) are trustworthy because a gate keeps them fresh. The
   hand-maintained ones decay. Every artifact this report proposes is generated and
   gated for exactly this reason. For the learned corpus, a generated *complete* index
   (all 1082, `id | slug | first line`, grouped by number range) would sit under the
   curated `LEARNED-INDEX.md` and end the `ls`-and-guess fallback. Cheap to emit.
2. **Prefer one entrypoint over seven.** You have `ai/INDEX.md`, `ai/rules/INDEX.md`,
   `LEARNED-INDEX.md`, `CODE-TO-DOCS.md`, `DESIGN-HISTORY.md`, `RECURRING-PATTERNS.md`,
   `HOOK-FRICTION.md`. An agent has to know which index answers which question.
   `ai/INDEX.md` is closest to a front door, but it is task-oriented ("adding a
   feature") rather than discovery-oriented ("understanding existing code"). A short
   "I want to understand, not change" section at the top of `ai/INDEX.md`, pointing at
   the package map, the doc-to-code index, and the digests, would give me one place to
   start every cold session.

## Recommendations, ranked

| # | Action | Payoff | Effort | Generatable |
|---|--------|--------|--------|-------------|
| 1 | Generate `ai/PACKAGE-MAP.md` (path, one-line role, key type) from registry `Description` + package doc first line; add to `ze-regen` gate | High: direct answer to "what does what" | Low | ~90% |
| 2 | Generate `ai/DOCS-TO-CODE.md` by inverting `// Design:` edges; add a `make` target + gate | Medium-High: ends the per-lookup grep | Low | 100% |
| 3 | Generate a *complete* learned-summary index (all 1082, id + slug + first line) under `LEARNED-INDEX.md` | Medium: ends `ls`-and-guess for ~840 summaries | Low | 100% |
| 4 | Add an "I want to understand, not change" front-door section to `ai/INDEX.md` | Medium: one cold-start entrypoint | Low | No |
| 5 | Create `ai/digests/<subsystem>.md`, git-tracked, starting with reactor + buffer/pool path; update-on-touch discipline | High over time: kills flow re-tracing | Medium | Partial |
| 6 | Backfill `// Package` doc comments on the ~254 undocumented packages (driven by the PACKAGE-MAP TODOs) | Medium: raises the floor for #1 and for godoc/LSP | Medium | No |
| 7 | Decide the ADR question: revive it, or delete the one-entry directory and route decisions to `DESIGN-HISTORY.md` | Low-Medium: removes a dead lookup path | Low | No |
| 8 | Refresh the stale hand-maintained headers (`DESIGN-HISTORY` "638", `LEARNED-INDEX` "500+") or regenerate their coverage | Low: restores trust in the indexes | Low | Partial |

If you do only three, do 1, 2, and 3. They are all cheap, all generated, all gated,
and together they cover most of the "what does what" and "where is it" cost. Number 5
is the highest ceiling but the most discipline, so it is worth doing deliberately and
narrowly rather than broadly.

## What not to do

- Do not add more learned summaries as the answer to slow rediscovery. You have 1082.
  The problem is that the current state is not mapped, not that history is thin.
- Do not hand-write the package map or the reverse index. They will rot. Generate them
  and gate them, the way `CODE-TO-DOCS.md` already is.
- Do not add a seventh or eighth top-level index without first deciding it earns its
  place against `ai/INDEX.md`. Sprawl is part of the problem, not the cure.
- Do not let a living digest grow long. A short digest that is slightly stale is
  cheap to fix. A long one that is stale is a trap.

## How I verified this

Three parallel sub-audits, each reading source and citing `file:line`:
design-decision discoverability (`DESIGN-HISTORY`, `LEARNED-INDEX`, `CODE-TO-DOCS`,
the ADR directory, the `// Design:` enforcement), the package-level "what does what"
map (`arch_map.py`, `registry.go`, `inventory.go`, package-doc coverage sampling), and
per-session bootstrap cost (`session-start.sh`, the compaction hooks, the
`tmp/session` state mechanism, `architecture.md`). Their findings agreed and
are folded into the gaps above.
