# Learned: RFC work is ordered by a graph the texts already hold, and a figure never reads complete while anything is unmeasured

`spec-rfc-priority-tree` gave the owner rule "RFC work is ordered by use-case
profile and by the RFC dependency tree" (`ai/META-DECISIONS.md`, Goal and
scope) its tooling: a dependency graph derived from the RFC texts, profiles
over it, a mandatory and an optional coverage figure per RFC, profile and
aggregate, `./le rfc tree` and `./le rfc next`, one website page per profile
under `/quality/rfc-compliance/profiles/`, and an `RFC gaps` spec field that
the roadmap maps to profiles. The reference page is
`docs/contributing/rfc-conformance-gates.md` ("Profiles and aggregates",
"Coverage figures", "The dependency graph", "The updated-by marker", "The
priority tree").

## What outlives the spec

1. **Edges are read, never authored.** Normative references (or a combined
   References section, accepting over-reach) and the header `Updates:` and
   `Obsoletes:` lines are the graph (`graph.Parse`, `graph.Build`,
   `internal/le/rfc/graph/`). The summaries' Meta links are checked against
   the headers (`checkHeaderLinks`); the header wins. Citation cycles collapse
   to one level.
2. **Membership is declared by the member.** Each summary's `Support` cell
   names its aggregate; a profile file under `rfc/profiles/` lists only its
   roots. The old `statusSections` literal was a central enumeration and was
   deleted, not kept beside the files.
3. **Proven means a verified discrimination record, and the denominator is the
   2026-09-02 ruling** (not-applicable and lower-layer rows stay in, `{rollup}`
   rows are out). A `{gap}` or `{partial}` row is never proven; an annotated row
   is proven only through a verified `{single-polarity}` record (`rowCounts`).
4. **An unmeasured RFC is named in words, never as a number.** No extraction
   sign-off reads "not walked"; a reached text with no summary reads "no
   summary"; both count in the rollup, which is never complete while either
   exists.
5. **Owner decision (2026-10-25): an RFC a profile lists as `Absent` renders 0%
   mandatory and 0% optional**, never "not walked" and never blank, and the
   rollup counts it, so a profile cannot read complete while one exists
   (`absentFigures`, `Rollup.mandatoryComplete`). This holds for the fttx-edge
   gaps (LAN DNS resolver, DHCPv6 server, NAT, NTP) until Thomas decides on
   their specs.
6. **`{updated-by: <ID>; why}` moves a rewritten row to its updater's level.**
   One updater id per row, naming the governing row where several rewrite one;
   a row the updater leaves "unchanged", or that reads the same under it, is
   not marked. Only rfc4271/rfc7606 is marked; the rest is
   `spec-rfc-updated-by-markers`.
7. **A package under `internal/le` must register a command.** A reader-only
   package breaks `internal/le/group_test.go` and the bootstraps check; the
   roadmap's corpus reader registers from `internal/le/rfc`'s `init()`
   (`readRoadmapCorpus`) so `roadmap` itself links none of the RFC parsers.

## Traps met

- A summary syntax the committed parser does not know makes the shared `./le`
  (rebuilt from HEAD) refuse every session's Bash calls naming `rfc/`: a
  migration that changes the cell and its parser lands as one commit.
- Deleting a package widens `./le verify changed` to every package (journal
  row in `plan/journal/gate-fires-outside-its-population.md`).
