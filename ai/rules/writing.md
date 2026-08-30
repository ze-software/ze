# Writing

**When:** writing or reviewing any prose in this repository: docs, code comments, error messages, CLI output, YANG descriptions, specs, commit messages, or a product comparison
**Severity:** blocking
**Related:** cli, evidence, rule-format, repo-maintenance, completion

## Directives

**Project text MUST be US English AND Simplified Technical English (ASD-STE100 Issue 9). Write what changes the reader's next action, and write nothing else.** The habits, the word choices, the restricted meanings, the words that never change, and the US spellings are published in `docs/contributing/writing-style.md`.

- **The project language is US English.** Every artifact that is part of Ze, code, docs, and user-facing text, uses US English spelling, wording, and date/number conventions. The single exception is prose authored in Thomas's own voice, which uses UK (British) English.
- **Ze writes in ASD-STE100 Simplified Technical English, Issue 9 (2025-01-15). This is rule one of the repository.** The six habits apply to all project text.
- **STE itself is a GUIDELINE. It is not a law, and it is not a gate.** The English variant, the documentation obligations, and the source anchors in this file are enforced. The STE checker reports and lets the work through.
- **A change MUST update documentation when it changes user or agent behavior, changes an architecture contract, invariant, or documented data flow, makes existing documentation stale, or adds a surface users or agents MUST discover.** A private implementation change that meets none of these triggers requires no prose.
- **Every factual claim in `docs/` MUST be verified against actual code before you write it.** Read the source first, then add the anchor.
- **A product comparison is advice, not marketing.** Every claim MUST help the reader choose the right tool, and MUST NOT make Ze look better.

## Language and Spelling

**This section decides the VARIANT; `docs/contributing/writing-style.md` decides the style.** Before writing or reviewing any project text you MUST ask: is this the project speaking, or Thomas speaking? Spell to match.
**A new identifier, comment, doc, or error string MUST be US English, with no exceptions.**

**Every surface below MUST be US English:**

| Surface | Examples |
|---------|----------|
| Go identifiers, types, functions, fields | `color`, `Normalize`, `Analyzer`, `licenseKey` |
| Code comments and godoc | "normalize the value", "this behavior" |
| Error messages, log lines, diagnostic codes | see `ai/rules/cli.md` |
| CLI output, help text, completions, TUI labels | `ze` command help, dashboards |
| YANG leaf descriptions and config help text | schema `description` strings |
| `docs/` user and architecture documentation | guides, references, comparisons |
| `ai/` rules, patterns, digests, indexes | this file included |
| `plan/` specs and journal rows | spec bodies, acceptance criteria |
| Commit messages and PR text | subject and body |

**Prose written in Thomas's voice MUST keep UK English:** `colour`, `behaviour`, `organise`, `licence` as a noun, `centre`. This covers everything produced under the `/write` skill and anything that reaches a reader as Thomas himself.
**UK spelling that has drifted into `docs/` MUST be fixed opportunistically when you touch a file, never by a global find-and-replace.** Some occurrences are quoted RFC or vendor text, or proper nouns, and quoted external text stays verbatim.

These categories MUST use UK (British) English:
- Blog posts, articles, essays.
- Emails and letters sent from Thomas.
- The Ze weekly update prose (`/ze-weekly-update`), which is Thomas's public voice even though the surrounding tooling and code are US English.

**When in doubt, apply this test: text ABOUT Ze MUST be US English, and text that IS Thomas talking MUST be UK English.** The `/write` skill applies that variant and carries his voice guidance.

## Simplified Technical English (ASD-STE100 Issue 9)

Prose SHOULD follow STE. **STE is a GUIDELINE. It is not a law, and it is not a gate.** It exists to make
text clearer for a reader. Owner directive, 2026-07-31.

- **Never rewrite a sentence only to satisfy a count.** An edit that changes no meaning for a reader is pure overhead, and it is the thing this guideline exists to remove. A sentence two words over the limit is not a defect.
- **The checker reports. It does not refuse.** `./le ste check` and the commit-time check print findings and let the work through. Apply a finding when it makes the text clearer, and ignore it when it does not.
- **Aim at the six habits, not at the arithmetic.** The word and sentence counts below are a smell test for a run-on, never a target to hit.
- **Write every sentence in STE.** One topic per sentence, active voice, the imperative form for instructions, and an approved verb for every action.
- **The six habits in the next section are the operative list.** A numbered STE rule bans each one. Learn the six first, then read the standard for the remainder.
- **A short, plain, repetitive sentence is correct.** Elegant variation is a defect, not a style.
- **Maximum 20 words in a procedural sentence** (STE Rule 5.1). This covers guide steps, runbooks, remediation lines, and commit messages.
- **Maximum 25 words in a descriptive sentence** (STE Rule 6.3), and maximum 6 sentences in a paragraph (STE Rule 6.6).
- **No semicolons** (STE Rule 8.1). Write two sentences, or use a vertical list (STE Rule 4.3).
- **No contractions, and do not omit words** (STE Rule 4.2). Write "do not", never "don't".
- **When you cannot say it in STE, the text is unclear, not the rule.** Change the sentence construction (STE Rule 9.1).
- **One word, one meaning, one part of speech.** Do not verb a noun, and do not carry two senses of a word through one document (STE Rules 1.2, 1.3, 1.7).
- **No slang, no jargon, no regional words.** `brick the router`, `bounce the daemon`, and `footgun` are not English a reader can look up (STE Rule 1.10).
- **Use the plain word unless the technical one earns its place.** Write for a capable reader who knows computing but not this repository. A good teacher keeps the idea and simplifies the words around it. A technical term is right when it names something exactly. Everywhere else it costs the reader and returns nothing. The test: would you say this sentence out loud to a colleague who has not read our code? `gated` is the standing example of a word to cut. It says that something is restricted, but not by what. Write `N requirements the check enforces`, or `compiled out unless ze_bgp is set`. The opposite mistake is equally real. Terms the code defines (`carrier`, `polarity`, `disposition`) are exact, so keep them and expand them on first use. Owner directive, 2026-08-01, and no checker enforces it.
- **Three words in a noun stack, no more.** Break a longer one with a preposition (STE Rules 2.1, 2.2). Expand an abbreviation on first use.
- **A replacement MUST keep the meaning.** `should` becomes MUST only for a real obligation, and `must` never sits in front of an imperative (STE Rules 1.2, 5.3).
- **Splitting a sentence is half the job. Connect the pieces.** Give information gradually, repeat the key word, open a paragraph with its topic sentence, and use a connector. `And` and `But` are correct sentence openers (STE Rules 4.4, 6.1, 6.2, 6.4, 6.5).
- **Keep the conjunction `that`,** and never drop an article to shorten a line (STE Rules 4.5, GR-1).
- **No definite article before a noun that carries an alphanumeric identifier.** Write `RFC 4271` and `peer 10.0.0.1` (STE Rule 4.5).
- **No Latin abbreviations.** Write `for example`, `that is`, and `and so on` (STE GR-6). Use gender-neutral language (GR-7).
- **The `-ing` form is permitted as a technical noun or its modifier, and nowhere else.** `routing table` is correct, and `before installing` is not (STE Rule 3.5).
- **Formatting, units, and abbreviation policy are Ze's own.** The standard sets no rules for them, so the uppercase in a safety block is our choice, not a requirement.

**STE MUST be applied exactly where this table says, and MUST NOT be applied where it says No:**

| Surface | STE applies |
|---------|-------------|
| `docs/` guides, references, comparisons, architecture pages | Yes |
| Code comments and godoc | Yes |
| Error messages, log lines, diagnostic remediation text | Yes, together with `ai/rules/cli.md` |
| CLI output, help text, completions, TUI labels | Yes |
| YANG `description` strings | Yes |
| `ai/` rules, patterns and digests, plus the durable half of `plan/`: journal rows, learned summaries, the template | Yes |
| Commit messages and PR text | Yes |
| A `plan/` document deleted at closure: `plan/spec-*.md`, a deferral shard, a known-failure shard | No. It is removed when the work closes, so nobody reads the edit |
| Chat replies, reports, and analysis for the user | No. Answer the person who asked |
| Thomas's authored prose: blog posts, articles, emails, the weekly update | No. That prose is his voice and it stays UK English |
| Identifiers, keys, tokens, quoted external text, and fixture data | No. `docs/contributing/writing-style.md`, "Words that never change" |

- **You MUST read `docs/contributing/writing-style.md` before documentation work, a deep prose review, or resolving an STE finding.** For all other project text, apply US English and the six habits. Do not open the full guide.
- That page carries every operative point. It covers the six habits with Ze examples, the sentence and paragraph limits, and verbs and voice. It also covers conditions, warnings, punctuation, the word-count convention, and the per-surface notes.
- The published standard stays the authority for a question that page does not answer. ASD gives Issue 9 at no cost: `https://www.asd-ste100.org`. The direct file is `https://www.asd-ste100.org/assets/files/ASD-STE100_ISSUE9.pdf`.
- Issue 9 has 53 writing rules in 9 sections, approximately 900 approved words, and approximately 1200 unapproved words with their alternatives.
- **The document is copyright (c) ASD 2025 and Ze has no reproduction right.** The special usage rights cover ASD, AIA, and ICCAIA members and their customers. They also cover ministries of defense, airworthiness authorities, and universities. Ze is in none of these categories.
- **You MUST keep the PDF, its text, and its dictionary out of the tracked repo.** You MUST put the local copy in `tmp/asd-ste100/`, which `.gitignore` excludes. Naming the standard is free, and copying its text is not.
- A converted copy at `tmp/asd-ste100/ASD-STE100_ISSUE9.md` is local and uncommitted. When that file is absent, you MUST download the PDF again.

- **HEAD is the baseline, and the comparison is per file.** A document nobody touched can never fail the gate, so legacy prose stays until someone rewrites it. The sentence you just wrote is what goes red.
- **There is no baseline file, and nothing to re-bless.** Rewriting a number cannot silence this gate, so the one way to green is to fix the prose (`ai/rules/completion.md`).
- **The gate is at commit time, not in `./le doc check verify`.** Several sessions share this checkout. A tree-wide prose gate reports a sibling session's in-flight sentences, and a gate that reddens for a colleague's typing gets switched off.
- **`./le ste check` still reads the whole working tree**, so it can name a file another session is editing. Read the path before you read the habit.
- **The checker holds our own word lists, not the ASD dictionary.** It cannot see every violation, so the six habits stay a review checklist as well as a gate. Report a violation as an ISSUE against its habit number.
- **When the tool is wrong, fix the tool and add the case to `internal/le/ste/ste_test.go`.** A checker that flags `setup`, an RFC 2119 MUST, or a code span gets switched off, and then it protects nothing.
- **Escape hatch for a document that MUST quote non-STE text at length:** `<!-- ste: ignore-file <reason> -->`, or `<!-- ste: ignore -->` above one line. The reason is mandatory.
- **Surfaces the tool reads:** Markdown in `docs/`, `ai/`, the durable half of `plan/`, and the repository root. Prose comments in `.go`. The `description` strings in `.yang`. Piped text on stdin. It never reads `rfc/`, which stays verbatim.
- **A document that is DELETED when the work closes is out of scope, and editing its prose is banned work (owner directive, 2026-08-10).** A spec `git rm`s itself in commit B, and a deferral or known-failure shard goes when its rows resolve, so a sentence rewritten there is read once by the session that wrote it. `plan/spec-*.md`, `plan/deferrals/` and `plan/known-failures/` are excluded in `internal/le/ste/ste.go`. `plan/journal/`, `plan/learned/` and `plan/TEMPLATE.md` stay in: they outlive every spec and are read by sessions that were not there.

You MUST answer these questions of each sentence:
1. Does each concept in this file have exactly one name? (habit 1)
2. Did I write a fact, or did I hedge? (habit 2)
3. Is each action a verb? (habit 3)
4. Did I praise the product instead of measuring it? (habit 4)
5. Is the sentence shorter than 20 words in a procedure, or 25 words in a description? (habit 5)
6. Is each verb a single word? (habit 6)

## Detail Budget

**You MUST write what changes the reader's next action, and you MUST write nothing else.**

- **Detail is a cost the reader pays, not proof that you did the work.** A fact the reader can recover in seconds by opening the code MUST NOT be written down.
- **You MUST cite a location so the reader can NAVIGATE, never to show that you looked.** Verification is an action you take (read the producing function). The citation is a pointer for the reader, and it is a separate decision.
- **You MUST name the file and the symbol: `session.go` `Session.Run`.** A line number is correct when the line IS the fact, or when a gate or generator pins it. Examples: a stack frame, a generated ledger row, a gate's own message, a `file:line -> sha` audit entry, a `ai/digests/` anchor that `./le digest` validates, a handoff edit range, and a `<!-- source: -->` anchor. Everywhere else the number rots at the next edit, and a reader who has the symbol never needs it.
- **One example for one point.** A second example earns its place only when it shows a DIFFERENT reading. A second instance of the same reading teaches nothing and costs every future session.
- **When a directive can be read two ways, you MUST write both readings and name the one that governs.** More examples hide an ambiguity. Naming the readings ends it.
- **You MUST NOT make the same cut twice.** When a table and a paragraph draw the same distinction, keep the table and delete the paragraph.
- **You MUST state the obligation, name the gate, and stop.** How a gate is implemented (flags, exit codes, guard order, retry bounds, byte offsets) belongs in the script and its fixtures. A rule that narrates its own enforcement code is a second, stale copy of that code.
- **You MUST report the conclusion, not the search.** What you tried, in what order, and how long it took are yours. The reader needs the answer, the evidence that would overturn it, and what is still open.
- **You MUST give a count plus the exceptions, not a row per item.** "12 call sites updated, 2 refused and are listed below" is complete. Twelve identical rows are not more complete.
- **A directive line in an always-on rule enters EVERY session through `CORE.md`, and every rule's `**When:**` line enters it through `TRIGGERS.md`.** Before you add one, you MUST ask whether it changes an action. When it does not, you MUST put it under `## Rationale` or `## Examples`. The digest drops both.
- **A pointer line points. It MUST NOT summarise.** An entry in `ai/INDEX.md` or any other index MUST say what the target answers, then stop, staying under 120 characters after the link. A reader who wants the content opens the target.

**An explanation, a question, or a request for a decision MUST be written in plain English.** Nobody talks the way a rule file reads, and a reader MUST NOT have to decode a sentence to answer a simple question.

- **You MUST ask for a decision the way you would ask a colleague.** "Do you want the IKE work in this commit, or kept separate?" beats a paragraph about commit ownership and verification scope.
- **You MUST say the thing, then the reason.** Not the reason, the qualifier, the caveat, and then the thing.
- **You MUST drop the machinery from the sentence.** File names, gate names, and rule ids belong in the report where a reader needs to act on them, never inside a sentence explaining what happened.
- **You MUST NOT stack qualifiers.** One sentence, one claim. If a sentence needs three commas to survive, it is two sentences.
- **This is not a license to be vague.** Plain is not loose. You MUST say exactly what happened, in words a person would use.

**A record earns its length from what the reader has to DO. Over budget means CUT, and it MUST NOT be split into two documents.**

**Each artifact SHOULD stay within its budget. No gate measures these: they are the standard a review applies, and the number to quote when a document is over:**

| Artifact | Contains | Budget |
|----------|----------|--------|
| Subagent report to the main thread | the conclusion, the evidence that would overturn it, open questions | under 40 lines |
| Review finding | the claim, where it lives, how it fails | 3 lines |
| Commit subject | what changed, imperative | one line |
| Commit body | the defect, its cause, what the fix does | under 15 lines |
| Known-failure shard | the failing output, the repro command, the next step | under 20 lines |
| Learned summary | what the code cannot tell a future reader | 25 to 35 lines (`ai/rules/planning.md`) |
| Index or pointer line | what the target answers | under 120 characters after the link |
| Rule file | trigger, directives, one example for each | under 150 lines. Above that, move the reference tables to `docs/` and link |

**These MUST NOT be written:**

| Banned | Why |
|--------|-----|
| Recounting dead ends, wrong hypotheses, or the order you tried things | The reader needs the answer, not the route to it |
| Any sentence about the difficulty or size of the work | It changes no action |
| Restating a fact in the next paragraph, or "as noted above" | Say it once, in the place the reader looks first |
| A line number for a claim the symbol name already locates | It rots at the next edit and forces a re-index |
| Pasting a whole file, table, or log when the answer is one row | Quote the row |
| A third example to settle an ambiguity two readings would settle | The ambiguity survives, now hidden |

## Documentation

- **Documentation MUST be updated when a change changes user or agent behavior, changes an architecture contract, invariant, or documented data flow, makes existing documentation stale, or adds a surface users or agents MUST discover.**
- **A private implementation change that meets none of these triggers requires no prose.**

**You MUST name the file, name the section, and describe the change. "Update documentation" MUST NOT be written as an instruction: it is not actionable.**

**A change MUST update the document its row names:**

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

**Every spec MUST carry a Documentation Update Checklist (`plan/TEMPLATE.md`). Each row MUST be answered Yes or No, and each Yes MUST name the file and what to add.**

**Every implementation that changes a feature, tool, self-check, verification gate, or test infrastructure MUST also satisfy `ai/rules/repo-maintenance.md`. Documentation is incomplete until a future agent can find the new surface from `ai/INDEX.md` or a named rule.**

**Every factual claim** in `docs/` MUST be verified against actual code before writing.
You MUST NOT describe what you *think* the code does. You MUST read the source first.

**A source anchor ties a factual claim to the code that produces it, and every rule below MUST be met:**

| Rule | Detail |
|------|--------|
| When to add | Every paragraph with a factual claim: syntax, field names, behavior, data structures |
| Format | `<!-- source: <relative-path> -- <symbol-or-topic> -->`, for example `<!-- source: internal/component/bgp/reactor/forward_pool.go -- ForwardPool -->` |
| Placement | After the paragraph or table row carrying the claim. NEVER inside a fenced code block: place it after the closing fence, because an HTML comment renders as visible text inside one |
| When editing docs | Verify the existing anchors still match reality, and fix a stale one |
| When changing code | Check whether any doc has an anchor pointing at the changed file, and update it when the claim is now wrong |
| Granularity | One anchor per factual paragraph or table. Not every sentence, and not every file |

Anchors are invisible in rendered markdown, and they are what lets a later session verify the page.

**Before writing any documentation:** you MUST read the actual source file. After writing, you MUST add the anchor.
**Before editing existing documentation:** you MUST grep for `<!-- source:` anchors and verify each one.

**Every config example in `docs/` MUST be written one statement per line, and MUST NOT be collapsed onto one line.** The multi-line form is the house style, and an agent that reflows an example to one line, followed by an agent that reflows it back, costs two diffs and tells the reader nothing.
**The one-line form is also a syntax error unless the last statement carries its semicolon.** Automatic semicolon insertion fires at a newline, so a closing brace on the same line as the statement it closes ends the block before the statement ends. Measured with a built `ze config validate`: `attach process bgp-rr { receive [ update ] }` is refused with "expected ';' after receive, got RBRACE", while `attach process bgp-rr { receive [ update ]; }` is accepted. An operator copies what a guide shows, so a guide MUST NOT show the refused form.
**An inline mention inside a sentence or a table cell MAY stay on one line, and MUST then carry the semicolon** (`internal rib { use bgp-rib; }`). Reflowing a phrase into a block would break the sentence around it.

**Every config example in `docs/` MUST parse, and an excerpt MUST parse inside the smallest complete config that carries it.** Build the binary and run `ze config validate` over that config before you publish the example.

**You MUST run `./le doc check verify` after editing any file under `docs/`, after adding or removing a plugin, and after touching a YANG `ze:command` declaration.** `internal/le/doc/wiring.Verify` runs the documentation drift, command-surface and source-anchor checks and reports every finding.

**`./le doc check verify` is not part of `./le verify current mode full`, so you MUST run it on demand.** `docs/contributing/documentation-testing.md` carries the workflow and how to read its output.

A path inside a `plan/` record MUST NOT be repointed when the file it names
moves. A spec, a journal row, a deferral, a debt row and a known-failure shard
each describe what was true when they were written, so the path is a fact about
that moment rather than a claim about the tree today. Rewriting it makes the
record say something that was never true.

`./le doc check links` does not police those trees, and
`citationExcludePrefixes` (`internal/le/doc/check/links.go`) names them with the
reason. The live instruction files under `plan/` stay in scope and are listed by
name: `plan/README.md`, the two templates, and the learned indexes. Those are
read for what is true NOW, so a dead path in one misleads a reader.

Everywhere else the rule is FIX ON TOUCH: repair a stale path in a file you are
already editing for another reason, and leave the rest alone. A rename owes the
record trees nothing.

The cost of the opposite policy is measured. One package rename left 383
dangling references across `plan/`, which the gate reported as breakage of the
same kind as a live document pointing at a deleted file. Chasing them meant
editing hundreds of historical files, each edit racing another session writing
its own rows, to make records restate the present.

Non-documentation text MUST follow its own rule instead of this one:
- Code comments (`// Design:`, `// Related:`) -- covered by `go-standards.md` and `go-standards.md`
- Journal rows (`plan/journal/`) -- covered by `planning.md`
- Memory entries -- covered by `memory.md`

## Comparison Honesty

**A product comparison is advice, never marketing.** A comparison can create tension between projects, so every claim MUST help the reader choose the right tool rather than make Ze look better.

1. You MUST cite every capability claim with a durable source.
   - You SHOULD prefer upstream source code links for implemented behavior.
   - You MUST use official feature documentation when code is not practical to cite.
   - For integrated products, you MUST cite both the wrapper/integration surface and the integrated project where the runtime behavior lives. Example: for VyOS routing features, cite VyOS config/templates and FRR documentation or source when FRR implements the protocol.
2. You MUST state the comparison scope before the matrix.
   - You MUST name the inspected checkout, release, branch, commit, docs page, or upstream feature page.
   - You MUST say that `not found` means not found in the inspected scope, not a universal absence claim.
3. You MUST label uncertainty instead of turning it into a gap.
   - You MUST use `Unclear` when evidence is incomplete.
   - You MUST use `Partial` when a narrower feature exists but is not equivalent to the compared feature.
   - You MUST separate similarly named features, such as IS-IS L1/L2 route leaking versus cross-VRF route leaking.
4. You MUST NOT cherry-pick categories to favor Ze.
   - You MUST include equivalent strengths from the other products.
   - If Ze is behind, you MUST say where it is behind and cite the evidence.
   - If another product delegates to an integrated daemon or OS facility, you MUST describe that delegation neutrally.
5. You MUST make wide comparison tables user-controllable.
   - Any comparison page with three or more product columns MUST provide controls to hide products the reader does not care about.
   - The controls MUST be keyboard-accessible and MUST NOT delete evidence from the source document.

**Before you publish or hand off a comparison, every row MUST carry its evidence or an explicit caveat, and a row with neither MUST be removed.**
**A public comparison MUST open with its inspected scope, near the top:**

```
Scope: inspected <projects/versions/paths>. Claims cite code or official docs. Integrated products cite their integration surface and the integrated implementation when relevant. `Not found` means not found in this inspected scope.
```
