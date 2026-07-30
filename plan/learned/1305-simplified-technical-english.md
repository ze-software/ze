# 1305 -- Simplified Technical English as rule one

## Context

Ze had no writing standard. `ai/rules/language-and-spelling.md` picked a spelling
variant and stopped there, so every other choice was the author's taste. Ze is
read by network operators whose English is often a second language. A sentence a
reader misunderstands becomes a misconfigured router. The owner chose ASD-STE100
Simplified Technical English, Issue 9, as the first rule of the repository. He
also asked for code that reviews a document against it.

## Decisions

- **Rule one is a rule file plus a committed guide, never the standard itself.** The standard is free to download and copyright (c) ASD 2025, and Ze holds no reproduction right. The PDF, its text, and its controlled dictionary stay in gitignored `tmp/asd-ste100/`. `docs/contributing/writing-style.md` is Ze's own restatement, reorganised around Ze's surfaces, and quotes nothing.
- **Naming the standard is free. Copying its text is not.** Dropping the citation would buy no safety and would turn a reference into an unattributed copy, so the guide keeps a lineage paragraph.
- **Six habits over 53 rules**, chosen by the owner: synonym rotation, hedging, frozen verbs, marketing adjectives, run-ons, phrasal verbs. Each maps to numbered rules, and the guide carries the material the six do not reach.
- **The gate compares each file against its own HEAD version, per file.** A committed baseline file was built first and was wrong twice. Its failures named no file, and it went red because a sibling session was editing `internal/component/mcp/*.go`. Several sessions share this checkout, so the working tree is the wrong unit of attribution.
- **The BLOCKING gate lives in `commit_helper.py`, not in `ze-doc-test`.** The files of one commit are the only place where prose has a single author. `make ze-ste-check` stays available for the whole tree.
- **The checker holds Ze's own word lists.** The controlled dictionary cannot be embedded, so recall is bounded by what we listed. Precision was chosen over recall: a checker that flags `setup`, an RFC 2119 MUST, or a code span gets switched off, and then it guards nothing.

## Consequences

- Legacy prose costs nothing until someone touches its file. No baseline file exists, so no number can be rewritten to reach green.
- `make ze-ste-review` reports about 44000 findings tree-wide. That is the visible debt, and it does not gate.
- Domain verbs are exempt by design. `implement`, `negotiate`, `withdraw`, and `advertise` are never flagged, because no plainer word means the same thing.
- The guide cannot reproduce the standard's mechanism. The standard is a closed allowlist of approved words, and a blocklist can only reject what somebody listed. The guide names the discipline so a writer knows the question to ask.

## Gotchas

- **A checker that documents its own escape hatch will exempt itself.** The `ste: ignore-file` pattern matched the rule file's own description of it, and switched off the one document that defines the rule. The pattern is now anchored to the start of a line.
- **A document that names a banned word trips its own gate.** Put every cited word in a code span. Fenced blocks are skipped whole, which makes them the right place for before-and-after examples.
- **`DO NOT EDIT` is a bad generated-file marker.** `ai/INSTRUCTIONS.md` opens with the banner it writes into its own outputs, so that string skipped the one document every session reads.
- **Sentence splitting must allow for markdown emphasis.** A bolded lead-in glues its whole bullet into one long sentence unless the splitter accepts `*` between the full stop and the space.
- **Rule 6.6 caps sentences in a PARAGRAPH.** A reference-table cell holding eight short facts is a table, and capping it pushes authors toward fewer, longer sentences.
- **The standard is permissive where the first draft was not.** Two instructions in one sentence are correct when the actions happen at the same time (`Remove and discard the seal`). One topic per sentence is a descriptive rule. A vertical list is for complex text, and an inline series stays correct. Being stricter than the standard is its own infidelity.
- **The `-ing` form is permitted as a technical noun or its modifier.** `routing table` and the heading `Troubleshooting` are correct. The first draft inverted this and banned them.
- **Do not conjugate inside a fix message.** Stripping `ing` from `writing` produced "after you writ".
- **Verify a converted PDF against a second extraction before you trust an example.** The Markdown conversion dropped an `Incorrect:` label, so `Tag the circuit breaker 36L7` reads as the form to follow when it is the form to avoid.
- **A rule digest regenerated from a shared working tree publishes another session's unlanded rule text.** Rebuild it from HEAD plus your own rule edits (`ai/rules/rule-format.md`).
- **Audit a restatement against the source, and verify the auditor.** Three independent audits found an inverted rule, three over-strict statements, two inventions, and a whole missing section. One of their findings was wrong, because the text that refuted it sat outside the range that agent was given.

## Files

- `ai/rules/simplified-technical-english.md` (rule one), `docs/contributing/writing-style.md` (the working guide)
- `scripts/dev/ste_check.py`, `scripts/dev/ste_check_test.py` (60 tests)
- `scripts/dev/commit_helper.py` (`ste_problems`, a BLOCK gate), `scripts/dev/commit_helper_test.py`
- `mk/inventory.mk` (`ze-ste-check`, `ze-ste-review`, `ze-ste-review-changed`, `ze-ste-review-json`)
- `ai/INSTRUCTIONS.md` (RULE ONE), `ai/INDEX.md`, `ai/rules/hook-mapping.md`, `ai/rules/language-and-spelling.md`
- `docs/contributing/README.md`, `docs/contributing/documentation-testing.md`
