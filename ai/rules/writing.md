# Writing

**When:** writing or reviewing any prose in this repository: docs, code comments, error messages, CLI output, YANG descriptions, specs, commit messages, or a product comparison
**Severity:** blocking
**Related:** cli, evidence, rule-format, repo-maintenance, completion

## Directives

**Project text MUST be US English AND Simplified Technical English (ASD-STE100 Issue 9), and prose written in Thomas's voice MUST be UK English.** Text ABOUT Ze is US English: Go identifiers, comments and godoc, error messages, CLI output, YANG descriptions, `docs/`, `ai/`, `plan/`, and commit messages. Text that IS Thomas speaking is UK English (`colour`, `behaviour`, `licence` as a noun): blog posts, articles, essays, emails, and the weekly update, all produced under the `/write` skill. A new identifier, comment, doc or error string MUST be US English with no exceptions, and UK spelling that has drifted into `docs/` MUST be fixed opportunistically in a file you already touch, never by a global replace, because some occurrences are quoted external text.
**STE is a GUIDELINE. It is not a law, and it is not a gate.** The checker reports and lets the work through, so apply a finding when it makes the text clearer and never rewrite a sentence only to satisfy a count. The six habits, the word choices, the sentence limits, the detail budget, the comparison rules and the config-example rules are in `docs/contributing/writing-style.md`. Read that page before documentation work, a deep prose review, or resolving an STE finding.

## Detail Budget

- **You MUST write what changes the reader's next action, and nothing else.** Detail is a cost the reader pays, not proof that you did the work, so a fact the reader can recover in seconds by opening the code MUST NOT be written down.
- **You MUST cite a location so the reader can NAVIGATE, never to show that you looked, and you MUST name the file and the symbol: `session.go` `Session.Run`.** A line number is correct only when the line IS the fact, or when a gate or a generator pins it: a stack frame, a generated ledger row, a gate's own message, a `file:line -> sha` audit entry, an `ai/digests/` anchor, a handoff edit range, a `<!-- source: -->` anchor. Everywhere else it rots at the next edit.
- **The artifact budgets, the banned detail patterns, and how to ask a question in plain English are in `docs/contributing/writing-style.md`.** Over budget means CUT, never split into two documents.

## Documentation

**A change MUST update the document its row names when it changes user or agent behavior, changes an architecture contract, invariant or documented data flow, makes existing documentation stale, or adds a surface users or agents have to discover.** A private implementation change that meets none of those triggers needs no prose. Name the file and the section and describe the change: "update documentation" is not an instruction. Every spec carries the Documentation Update Checklist from `plan/TEMPLATE.md`, each row answered Yes or No, and each Yes naming the file and what to add.

| # | Category | Location | When to update |
|---|----------|----------|----------------|
| 1 | Feature list | `docs/features.md` | New user-facing feature |
| 2 | User guide | `docs/guide/<topic>.md` | Feature with usage instructions |
| 3 | Config syntax | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` | Config format changes |
| 4 | CLI reference | `docs/guide/command-reference.md` | New or changed CLI commands |
| 5 | API and RPC docs | `docs/architecture/api/commands.md`, `docs/architecture/api/architecture.md` | New or changed RPCs or event types |
| 6 | Plugin guide | `docs/guide/plugins.md`, `docs/plugin-development/` | Plugin SDK or lifecycle changes |
| 7 | Wire format | `docs/architecture/wire/*.md` | Encoding or decoding changes |
| 8 | Plugin SDK rules | `ai/rules/plugins.md` | Registration fields, protocol changes |
| 9 | RFC compliance | `rfc/short/rfcNNNN.md` | New RFC implementation |
| 10 | Test infrastructure | `docs/functional-tests.md`, `docs/architecture/testing/` | New test tools or patterns |
| 11 | Comparison | `docs/comparison.md` | Feature parity with other daemons |
| 12 | Architecture | `docs/architecture/core-design.md` or the subsystem doc | Structural design changes |
| 13 | Route metadata | `docs/architecture/meta/README.md` and `docs/architecture/meta/<plugin>.md` | A plugin sets or reads route metadata keys |
| 14 | Prometheus counters | `docs/plugin-development/metrics.md` or the subsystem telemetry doc | Counters or gauges added or changed |
| 15 | Agent discovery | `ai/rules/repo-maintenance.md`, `ai/INDEX.md` | New features, tools, self-checks, verification gates, test infrastructure, or agent workflows |

**Every paragraph or table in `docs/` that carries a factual claim about syntax, field names, behavior or data structures MUST carry a source anchor, written `<!-- source: <relative-path> -- <symbol-or-topic> -->` after that paragraph or row.** One anchor per factual paragraph or table, never per sentence and never per file. It MUST NOT sit inside a fenced code block, because an HTML comment renders as visible text there; put it after the closing fence.
**When you edit a page you MUST verify its existing anchors and fix a stale one, and when you change code you MUST check whether a doc anchors the changed file and update the claim that is now wrong.** Run `./le doc check verify` after editing any file under `docs/`, after adding or removing a plugin, and after touching a YANG `ze:command` declaration. It is not part of `./le verify current mode full`, so you MUST run it on demand.

**A path inside a `plan/` record MUST NOT be repointed when the file it names moves.** A spec, a journal row, a deferral, a debt row and a known-failure shard each describe what was true when they were written, so the path is a fact about that moment rather than a claim about the tree today. `citationExcludePrefixes` (`internal/le/doc/check/links.go`) keeps those trees out of `./le doc check links`, and it keeps the live instruction files IN by name: `plan/README.md`, the two templates, and the learned indexes.
**Everywhere else the rule is FIX ON TOUCH: you MUST repair a stale path in a file you are already editing for another reason, and you MUST leave the rest alone.** A rename owes the record trees nothing.
