# Writing

**When:** writing or reviewing any prose in this repository: docs, code comments, error messages, CLI output, YANG descriptions, specs, commit messages, agent reports, or a product comparison
**Severity:** blocking
**Related:** cli, evidence, rule-format, repo-maintenance, completion

## Directives

Project text is US English AND Simplified Technical English (ASD-STE100 Issue 9). Write what changes the reader's next action, and write nothing else.

- **The project language is US English.** Every artifact that is part of Ze, code, docs, and user-facing text, uses US English spelling, wording, and date/number conventions. The single exception is prose authored in Thomas's own voice, which uses UK (British) English.
- **Ze writes in ASD-STE100 Simplified Technical English, Issue 9 (2025-01-15). This is rule one of the repository.** The working guide is `docs/contributing/writing-style.md`, which is committed and complete on its own.
- **STE itself is a GUIDELINE. It is not a law, and it is not a gate.** The English variant, the documentation obligations, and the source anchors in this file are enforced. The STE checker reports and lets the work through.
- **Every feature change MUST update the specific documentation it affects.** Name the file, name the section, describe the change.
- **Every factual claim in `docs/` MUST be verified against actual code before you write it.** Read the source first, then add the anchor.
- **A product comparison is advice, not marketing.** Every claim MUST help the reader choose the right tool, and MUST NOT make Ze look better.

## Language and Spelling

**You MUST use this section to decide the English VARIANT. You MUST use the Simplified Technical English section to decide the STYLE, and it is rule one.** STE Rule 1.14 also requires American English spelling, so the two agree. Thomas's authored prose is the exception to both: UK English, and no STE.

### Why two variants

Software convention is US English: identifiers, standard-library names, RFC text,
and the wider Go ecosystem all spell it `color`, `initialize`, `serialize`,
`behavior`. Ze follows that so its surface reads like every other tool an operator
or plugin author already knows. Thomas, however, writes in UK English, so anything
that speaks as *him* keeps his spelling. The boundary is authorship, not medium:
who the text speaks as decides the variant.

### US English -- everything that is the project

Use US English for all of the following. This list is illustrative, not exhaustive:

| Surface | Examples |
|---------|----------|
| Go identifiers, types, functions, fields | `color`, `Normalize`, `Analyzer`, `licenseKey` |
| Code comments (`// ...`) and godoc | "normalize the value", "this behavior" |
| Error messages, log lines, diagnostic codes | see `ai/rules/cli.md` |
| CLI output, help text, completions, TUI labels | `ze` command help, dashboards |
| YANG leaf descriptions and config help text | schema `description` strings |
| `docs/` (user + architecture documentation) | guides, references, comparisons |
| `ai/` rules, patterns, digests, indexes | this file included |
| `plan/` specs, journal rows | spec bodies, ACs |
| Commit messages and PR text | subject and body |

Common divergences to get right (US -- avoid the UK form):

`color` (not colour), `behavior` (not behaviour), `initialize` / `normalize` /
`serialize` / `organize` / `optimize` / `recognize` (not the `-ise` forms),
`license` as both noun and verb (not licence), `catalog` (not catalogue),
`center` (not centre), `canceled` / `canceling` (one `l`), `analyze` (not
analyse), `fiber` (not fibre), `gray` (not grey), `defense` (not defence).

### The one exception -- Thomas's authored prose is UK English

Prose written in Thomas's voice keeps UK (British) English: `colour`, `behaviour`,
`organise`, `licence` (noun), `centre`, and so on. This covers everything produced
under the `/write` skill and anything that reaches a reader as Thomas himself:

These categories MUST use UK (British) English:
- Blog posts, articles, essays.
- Emails and letters sent from Thomas.
- The Ze weekly update prose (`/ze-weekly-update`), which is Thomas's public voice even though the surrounding tooling and code are US English.

The `/write` skill (`~/.claude/commands/write.md`) is where this variant is
applied; it also carries Thomas's no-em-dash and voice guidance. When in doubt:
if the text is *about* Ze it is US English; if the text *is* Thomas talking, it is
UK English.

### Current state and drift

Ze is already de-facto US English (the large majority of files use the US forms).
A small amount of UK spelling has leaked into `docs/` over time. Do not run a blind
global find/replace: some occurrences are quoted RFC/BIRD text or proper nouns that
must stay verbatim. Fix drift opportunistically when you touch a file, matching the
surrounding US convention, and leave quoted external text untouched.

### Mechanical check: variant

Before writing or reviewing any project text, ask: is this text the project
speaking (US English) or Thomas speaking (UK English)? Spell to match. New
identifiers, comments, docs, and error strings must be US English with no
exceptions.

## Simplified Technical English (ASD-STE100 Issue 9)

Prose SHOULD follow STE. **STE is a GUIDELINE. It is not a law, and it is not a gate.** It exists to make
text clearer for a reader. Owner directive, 2026-07-31.

- **Never rewrite a sentence only to satisfy a count.** An edit that changes no meaning for a reader is pure overhead, and it is the thing this guideline exists to remove. A sentence two words over the limit is not a defect.
- **The checker reports. It does not refuse.** `make ze-ste-check` and the commit-time check print findings and let the work through. Apply a finding when it makes the text clearer, and ignore it when it does not.
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

### The six habits to avoid

| # | Habit | What it looks like | Banned by | Write instead |
|---|-------|--------------------|-----------|---------------|
| 1 | **Synonym rotation** | one concept with three names on one page: `peer`, then `neighbor`, then `session`. Also the formal word where a plain one exists: `initiate`, `terminate`, `obtain`, `utilize`, `ascertain` | STE Rules 1.3, 1.11, 9.4, and the controlled vocabulary itself | Give each concept one name, and make that name the plain one: `start`, `stop`, `get`, `use`, `make sure`. Keep a domain verb that no plainer word replaces (`implement`, `negotiate`, `withdraw`) |
| 2 | **Hedging** | `may`, `might`, `should probably`, `generally`, `typically`, `in most cases`, `we believe`, `it seems` | STE Rule 1.1 and the dictionary: `may` -> CAN, `should` -> MUST, `generally` and `normally` -> USUALLY. `might`, `could`, and `typically` have no entry at all | State the fact. CAN for a possibility, MUST for an obligation, WILL for a future event. If you do not know, write that you do not know |
| 3 | **Frozen verbs** | `do the installation of the plugin`, `performs validation of the config`, `before the removal of the unit` | STE Rule 3.7, with Rules 1.13 and 3.5 | Use the verb: `install the plugin`, `validates the config`, `before you remove the unit` |
| 4 | **Marketing adjectives** | `powerful`, `seamless`, `robust`, `blazing fast`, `feature-rich`, `effortless`, `battle-tested`, `best-in-class` | STE Rule 1.1: these words have no dictionary entry and they are not technical nouns | Give the number, the limit, or the mechanism. Then delete the adjective (see "Comparison Honesty" below) |
| 5 | **Run-ons** | one sentence with three clauses, a semicolon splice, a paragraph of eight sentences | STE Rules 4.1, 5.1, 6.3, 6.6, 8.1 | Write one topic in each sentence. Split the sentence. Use a vertical list for complex text (STE Rule 4.3) |
| 6 | **Phrasal verbs** | `spin up the daemon`, `kick off the run`, `figure out the cause`, `get rid of the route`, `put out the fire` | STE Rule 9.3 | Use one verb: `start the daemon`, `start the run`, `find the cause`, `delete the route`, `extinguish the fire` |

### Words that stay verbatim

| Surface | Why it does not change |
|---------|------------------------|
| RFC 2119 keywords (MUST, MUST NOT, SHOULD, MAY) when they name an RFC's obligation level | The keyword IS the requirement. `rfc-compliance.md` and every `RFC requirement:` test tag read the exact word. The dictionary substitution `should -> MUST` never applies to a quoted normative term |
| Quoted external text: RFC prose, vendor output, peer daemon log lines, third-party documentation | A quotation is evidence. A changed quotation is false evidence (`evidence.md`) |
| Go identifiers, YANG leaf names, JSON keys, env var keys, CLI tokens, command grammar | `go-standards.md`, `config.md`, `config.md`, and `cli.md` own these. STE governs prose, never identifiers |
| Technical nouns and technical verbs of this subject field: `peer`, `prefix`, `NLRI`, `teardown`, `netlink`, `deenergize` | STE Rules 1.5, 1.6, and 1.12 permit them. `request peer teardown` is a technical noun and is correct. `tear down the peer` is a phrasal verb and is not |
| Test fixture data, hex dumps, config samples, and fenced code blocks | These are data, not prose |

### Scope

| Surface | STE applies |
|---------|-------------|
| `docs/` guides, references, comparisons, architecture pages | Yes |
| Code comments and godoc | Yes |
| Error messages, log lines, diagnostic remediation text | Yes, together with `cli.md` |
| CLI output, help text, completions, TUI labels | Yes |
| YANG `description` strings | Yes |
| `ai/` rules, patterns, and digests, plus the durable half of `plan/`: journal rows, learned summaries, the template | Yes |
| A `plan/` document deleted at closure: `plan/spec-*.md`, a deferral shard, a known-failure shard | No. It is removed when the work closes, so nobody reads the edit |
| Commit messages and PR text | Yes |
| Chat replies, reports, and analysis for the user | No. Answer the person who asked |
| Thomas's authored prose: blog posts, articles, emails, the weekly update (`/write`, `/ze-weekly-update`) | No. That prose is his voice and it stays UK English (see "Language and Spelling" above) |
| Identifiers, keys, tokens, quoted external text, and fixture data | No. Refer to "Words that stay verbatim" |

### Where to read the detail

- **You MUST read `docs/contributing/writing-style.md` first, and expect to need nothing else.** It is committed, and it is Ze's own text.
- That page carries every operative point. It covers the six habits with Ze examples, the sentence and paragraph limits, and verbs and voice. It also covers conditions, warnings, punctuation, the word-count convention, and the per-surface notes.
- The published standard stays the authority for a question that page does not answer. ASD gives Issue 9 at no cost: `https://www.asd-ste100.org`. The direct file is `https://www.asd-ste100.org/assets/files/ASD-STE100_ISSUE9.pdf`.
- Issue 9 has 53 writing rules in 9 sections, approximately 900 approved words, and approximately 1200 unapproved words with their alternatives.
- **The document is copyright (c) ASD 2025 and Ze has no reproduction right.** The special usage rights cover ASD, AIA, and ICCAIA members and their customers. They also cover ministries of defense, airworthiness authorities, and universities. Ze is in none of these categories.
- **You MUST keep the PDF, its text, and its dictionary out of the tracked repo.** You MUST put the local copy in `tmp/asd-ste100/`, which `.gitignore` excludes. Naming the standard is free, and copying its text is not.
- A converted copy at `tmp/asd-ste100/ASD-STE100_ISSUE9.md` is local and uncommitted. When that file is absent, you MUST download the PDF again.

### Enforcement

| Command | What it does |
|---------|--------------|
| `scripts/dev/commit_helper.py create` | **Advisory.** `ste_problems` PRINTS findings for a commit's own `.md`, `.go`, or `.yang` files. It never refuses the commit. It runs on the files of that commit, so the prose it judges has one author |
| `make ze-ste-check` | The same gate over every changed file in the working tree. Run it before you prepare a commit. About 2 seconds |
| `make ze-ste-review` | The whole-tree report. Every finding with its `file:line`, its habit number, and the replacement to use |
| `make ze-ste-review-changed` | The same report for changed files only |
| `python3 scripts/dev/ste_check.py <file>...` | Review named documents |
| `git log -1 --format=%B \| python3 scripts/dev/ste_check.py -` | Review a commit message or a PR body before you send it |

- **HEAD is the baseline, and the comparison is per file.** A document nobody touched can never fail the gate, so legacy prose stays until someone rewrites it. The sentence you just wrote is what goes red.
- **There is no baseline file, and nothing to re-bless.** Rewriting a number cannot silence this gate, so the one way to green is to fix the prose (`ai/rules/completion.md`).
- **The gate is at commit time, not in `ze-doc-test`.** Several sessions share this checkout. A tree-wide prose gate reports a sibling session's in-flight sentences, and a gate that reddens for a colleague's typing gets switched off.
- **`make ze-ste-check` still reads the whole working tree**, so it can name a file another session is editing. Read the path before you read the habit.
- **The checker holds our own word lists, not the ASD dictionary.** It cannot see every violation, so the six habits stay a review checklist as well as a gate. Report a violation as an ISSUE against its habit number.
- **When the tool is wrong, fix the tool and add the case to `scripts/dev/ste_check_test.py`.** A checker that flags `setup`, an RFC 2119 MUST, or a code span gets switched off, and then it protects nothing.
- **Escape hatch for a document that MUST quote non-STE text at length:** `<!-- ste: ignore-file <reason> -->`, or `<!-- ste: ignore -->` above one line. The reason is mandatory.
- **Surfaces the tool reads:** Markdown in `docs/`, `ai/`, the durable half of `plan/`, and the repository root. Prose comments in `.go`. The `description` strings in `.yang`. Piped text on stdin. It never reads `rfc/`, which stays verbatim.
- **A document that is DELETED when the work closes is out of scope, and editing its prose is banned work (owner directive, 2026-08-10).** A spec `git rm`s itself in commit B, and a deferral or known-failure shard goes when its rows resolve, so a sentence rewritten there is read once by the session that wrote it. `plan/spec-*.md`, `plan/deferrals/` and `plan/known-failures/` are excluded in `scripts/dev/ste_check.py`. `plan/journal/`, `plan/learned/` and `plan/TEMPLATE.md` stay in: they outlive every spec and are read by sessions that were not there.

### Mechanical check: STE

Before you publish a sentence, answer six questions:

You MUST answer these questions of each sentence:
1. Does each concept in this file have exactly one name? (habit 1)
2. Did I write a fact, or did I hedge? (habit 2)
3. Is each action a verb? (habit 3)
4. Did I praise the product instead of measuring it? (habit 4)
5. Is the sentence shorter than 20 words in a procedure, or 25 words in a description? (habit 5)
6. Is each verb a single word? (habit 6)

## Reporting to the Owner

### Reporting to the Owner

- **A report to the owner MUST open with three things and nothing before them: what is blocked, why it matters, and what you need from him.** Everything else is detail he can ask for.
- **You MUST assume he will read the first ten lines and stop.** Put the decision there or it does not exist.
- **A decision you need MUST be a table, one row per decision**, with the choice, what it unblocks, and what it costs him to answer. He picks a row; he does not parse a paragraph to find the question.
- **You MUST NOT lead with narrative.** What you did, in what order, and what each agent found is the story of your work, not his input to it. It goes last or it goes unsaid.
- **You MUST NOT pre-empt questions he has not asked.** Caveats, alternatives considered, and evidence chains are answers waiting for a question. Hold them.
- **Status that changes no decision MUST be one line or absent.** A count, a pass, a green gate: one line. A reader who wants the breakdown will ask.

- **An agent's report is written for the agent that commissioned it. The owner's report is written for a person. You MUST NOT relay the first as the second.** A subagent report is a working artifact: exhaustive, hedged, full of file paths and gate names, because its reader has to verify it. Passing that register on to the owner is the failure this section exists to prevent, and it happens most often when a phase agent's findings are good and the temptation is to forward them intact.
- **You MUST re-write, never forward.** Take the conclusion, drop the derivation, and say it in the words a colleague would use at a desk.
- **Formatting carries the message and you MUST use it.** A table for choices, a short list for state, bold for the single thing that matters. A wall of paragraphs is unreadable to a person however correct each sentence is.
- **You MUST NOT pad a report to show effort.** Length reads as thoroughness to a machine and as noise to a person. The owner measures the work by what changed, never by how much you wrote about it.
- **The tell is a reader who has to hunt for the ask.** A question that is not findable in one glance means the report has failed, whatever else it got right.

## Detail Budget

Write what changes the reader's next action. Write nothing else.

- **Detail is a cost the reader pays, not proof that you did the work.** A fact the reader can recover in seconds by opening the code MUST NOT be written down.
- **You MUST cite a location so the reader can NAVIGATE, never to show that you looked.** Verification is an action you take (read the producing function). The citation is a pointer for the reader, and it is a separate decision.
- **You MUST name the file and the symbol: `session.go` `Session.Run`.** A line number is correct when the line IS the fact, or when a gate or generator pins it. Examples: a stack frame, a generated ledger row, a gate's own message, a `file:line -> sha` audit entry, a `ai/digests/` anchor that `make ze-digest-check` validates, a handoff edit range, and a `<!-- source: -->` anchor. Everywhere else the number rots at the next edit, and a reader who has the symbol never needs it.
- **One example for one point.** A second example earns its place only when it shows a DIFFERENT reading. A second instance of the same reading teaches nothing and costs every future session.
- **When a directive can be read two ways, you MUST write both readings and name the one that governs.** More examples hide an ambiguity. Naming the readings ends it.
- **You MUST NOT make the same cut twice.** When a table and a paragraph draw the same distinction, keep the table and delete the paragraph.
- **You MUST state the obligation, name the gate, and stop.** How a gate is implemented (flags, exit codes, guard order, retry bounds, byte offsets) belongs in the script and its fixtures. A rule that narrates its own enforcement code is a second, stale copy of that code.
- **You MUST report the conclusion, not the search.** What you tried, in what order, and how long it took are yours. The reader needs the answer, the evidence that would overturn it, and what is still open.
- **You MUST give a count plus the exceptions, not a row per item.** "12 call sites updated, 2 refused and are listed below" is complete. Twelve identical rows are not more complete.
- **A directive line in an always-on rule enters EVERY session through `CORE.md`, and every rule's `**When:**` line enters it through `TRIGGERS.md`.** Before you add one, you MUST ask whether it changes an action. When it does not, you MUST put it under `## Rationale` or `## Examples`. The digest drops both.
- **A pointer line points. It MUST NOT summarise.** An entry in `ai/INDEX.md` or any other index MUST say what the target answers, then stop, staying under 120 characters after the link. A reader who wants the content opens the target.

### Write like a person

Explanations, questions, and requests for a decision go in plain English. Nobody
talks the way a rule file reads, and a reader should not have to decode a sentence
to answer a simple question.

- **You MUST ask for a decision the way you would ask a colleague.** "Do you want the IKE work in this commit, or kept separate?" beats a paragraph about commit ownership and verification scope.
- **You MUST say the thing, then the reason.** Not the reason, the qualifier, the caveat, and then the thing.
- **You MUST drop the machinery from the sentence.** File names, gate names, and rule ids belong in the report where a reader needs to act on them, never inside a sentence explaining what happened.
- **You MUST NOT stack qualifiers.** One sentence, one claim. If a sentence needs three commas to survive, it is two sentences.
- **This is not a license to be vague.** Plain is not loose. You MUST say exactly what happened, in words a person would use.

### Budgets

A record earns its length from what the reader must DO. Over budget means cut, never split into two documents.

| Artifact | Contains | Budget |
|----------|----------|--------|
| Reply to the user | what changed, what proves it, what is not done | under 15 lines, tables before prose |
| Subagent report to the main thread | the conclusion, the evidence that would overturn it, open questions | under 40 lines |
| Review finding | the claim, where it lives, how it fails | 3 lines |
| Commit subject | what changed, imperative | one line |
| Commit body | the defect, its cause, what the fix does | under 15 lines |
| Known-failure shard | the failing output, the repro command, the next step | under 20 lines |
| Learned summary | what the code cannot tell a future reader | 25 to 35 lines (`ai/rules/planning.md`) |
| Index or pointer line | what the target answers | under 120 characters after the link |
| Rule file | trigger, directives, one example for each | under 150 lines. Above that, move reference tables to `docs/` and link |

No gate measures these yet. They are the standard a review applies, and the number to quote when a document is over.

### Banned

| Banned | Why |
|--------|-----|
| Recounting dead ends, wrong hypotheses, or the order you tried things | The reader needs the answer, not the route to it |
| Any sentence about the difficulty or size of the work | It changes no action |
| Restating a fact in the next paragraph, or "as noted above" | Say it once, in the place the reader looks first |
| A line number for a claim the symbol name already locates | It rots at the next edit and forces a re-index |
| Pasting a whole file, table, or log when the answer is one row | Quote the row |
| A third example to settle an ambiguity two readings would settle | The ambiguity survives, now hidden |

## Documentation

Every feature change MUST update the specific documentation it affects.

### Principle

Name the file, name the section, describe the change. Never say "update documentation" generically.

### Documentation Categories

| # | Category | Location | When to update |
|---|----------|----------|----------------|
| 1 | Feature list | `docs/features.md` | New user-facing feature |
| 2 | User guide | `docs/guide/<topic>.md` | Feature with usage instructions |
| 3 | Config syntax | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` | Config format changes |
| 4 | CLI reference | `docs/guide/command-reference.md` | New/changed CLI commands |
| 5 | API/RPC docs | `docs/architecture/api/commands.md`, `docs/architecture/api/architecture.md` | New/changed RPCs or event types |
| 6 | Plugin guide | `docs/guide/plugins.md`, `docs/plugin-development/` | Plugin SDK or lifecycle changes |
| 7 | Wire format | `docs/architecture/wire/*.md` | Encoding/decoding changes |
| 8 | Plugin SDK rules | `ai/rules/plugins.md` | Registration fields, protocol changes |
| 9 | RFC compliance | `rfc/short/rfcNNNN.md` | New RFC implementation |
| 10 | Test infrastructure | `docs/functional-tests.md`, `docs/architecture/testing/` | New test tools or patterns |
| 11 | Comparison | `docs/comparison.md` | Feature parity with other daemons |
| 12 | Architecture | `docs/architecture/core-design.md` or subsystem doc | Structural design changes |
| 13 | Route metadata | `docs/architecture/meta/README.md` + `docs/architecture/meta/<plugin>.md` | Plugin sets or reads route metadata keys |
| 14 | Prometheus counters | `docs/plugin-development/metrics.md` or subsystem telemetry doc | Counters/gauges added or changed |
| 15 | Agent discovery | `ai/rules/repo-maintenance.md`, `ai/INDEX.md` | New features, tools, self-checks, verification gates, test infrastructure, or agent workflows |

### In Specs

Every spec MUST have a **Documentation Update Checklist** (see `plan/TEMPLATE.md`).
Each row answered Yes/No. Each Yes names the file and what to add.

Every direct implementation that changes a feature, tool, self-check,
verification gate, or test infrastructure MUST also satisfy
`ai/rules/repo-maintenance.md`. Documentation is incomplete until future
agents can find the new surface from `ai/INDEX.md` or a
specific rule.

### Source Anchors (BLOCKING)

**Every factual claim** in `docs/` MUST be verified against actual code before writing.
You MUST NOT describe what you *think* the code does. You MUST read the source first.

Add HTML comment anchors tying claims to code locations:

```
<!-- source: internal/component/bgp/reactor/forward_pool.go -- ForwardPool -->
```

These are invisible in rendered markdown but let future sessions verify accuracy.

| Rule | Detail |
|------|--------|
| When to add | Every paragraph with a factual claim (syntax, field names, behavior, data structures) |
| Format | `<!-- source: <relative-path> -- <symbol-or-topic> -->` |
| Placement | After the paragraph or table row containing the claim. **NEVER inside fenced code blocks** (between ` ``` ` delimiters) -- place after the closing fence. Inside code blocks, HTML comments render as visible text. |
| When editing docs | Verify existing anchors still match reality. Fix stale ones |
| When changing code | Check if any doc has an anchor pointing to the changed file. Update if claim is now wrong |
| Granularity | One anchor per factual paragraph or table. Not every sentence, not every file |

**Before writing any documentation:** you MUST read the actual source file. After writing, you MUST add the anchor.
**Before editing existing documentation:** you MUST grep for `<!-- source:` anchors and verify each one.

**Every config example in `docs/` MUST be written one statement per line, and MUST NOT be collapsed onto one line.** The multi-line form is the house style, and an agent that reflows an example to one line, followed by an agent that reflows it back, costs two diffs and tells the reader nothing.
**The one-line form is also a syntax error unless the last statement carries its semicolon.** Automatic semicolon insertion fires at a newline, so a closing brace on the same line as the statement it closes ends the block before the statement ends. Measured with a built `ze config validate`: `attach process bgp-rr { receive [ update ] }` is refused with "expected ';' after receive, got RBRACE", while `attach process bgp-rr { receive [ update ]; }` is accepted. An operator copies what a guide shows, so a guide MUST NOT show the refused form.
**An inline mention inside a sentence or a table cell MAY stay on one line, and MUST then carry the semicolon** (`internal rib { use bgp-rib; }`). Reflowing a phrase into a block would break the sentence around it.

**Every config example in `docs/` MUST parse, and an excerpt MUST parse inside the smallest complete config that carries it.** Build the binary and run `ze config validate` over that config before you publish the example. The one-line form above is one way an example is refused; a retired keyword, a leaf that moved, and a peer named by address are the others.

**Nothing in this repository parses a config example, so a refused one survives until a reader tries it.** `docs/guide/rpki.md` opens with a peer written `peer peer1 { remote { ... } local { ... } }`, and `remote` is not a peer field: `ze-bgp-conf.yang` models it under `container connection` in `grouping peer-fields`, so the parser answers `unknown field in peer: remote`. That example has never parsed, at `origin/main` too, and `route-reflection.md` and `graceful-restart.md` carry the same retired shape.

The second reader of a refused example is not an operator, it is a review. A review lens cited invalid guide lines as evidence that a config shape was live in the tree, and those lines had never parsed. A document that shows config the parser refuses is not only a bad instruction to an operator: inside this repository it is false evidence about the product, and it is quoted as a producer by whoever is reasoning quickly.

**No gate reads a config example today, and the gate that lands here MUST gate what ze's own parser recognizes as a config attempt, with an opt-OUT that states its reason on the block.** Gating every fenced block in `docs/` would fire mostly on deliberate excerpts, an estimated four in five, because they start mid-tree or carry a placeholder. An opt-IN marker inverts the failure: every example already refused stays unmarked and uncaught, which is the `rpki.md` case exactly.

**MUST state that cost when proposing it, rather than sell the gate.** Somebody annotates the excerpts once, and each new excerpt pays one line. `ai/rules/repo-maintenance.md` owns registering the gate and its row in the hook mapping. `plan/spec-ze-config-fmt.md` owns how an example is RENDERED, so cross-reference it: a formatter decides the shape, and this decides whether the shape parses.

### Validation

Run `make ze-doc-test` after editing any file under `docs/`, after adding or removing a plugin, or after touching a YANG `ze:command` declaration. The umbrella target runs `check-doc-drift` (validates doc counts/lists and narrow stale-claim checks), `validate-commands` (validates YANG `ze:command` <-> RPC handler contract), and the source-anchor stale-path check. These fail the make target on drift and report all issues found.

Not part of `ze-verify` today because of a pre-existing drift backlog. Run on demand. See `docs/contributing/documentation-testing.md` for the full workflow and how to interpret output.

### NOT Documentation

Non-documentation text MUST follow its own rule instead of this one:
- Code comments (`// Design:`, `// Related:`) -- covered by `go-standards.md` and `go-standards.md`
- Journal rows (`plan/journal/`) -- covered by `planning.md`
- Memory entries -- covered by `memory.md`

## Comparison Honesty

### Principle

Product comparisons are advice, not marketing. They can create tension between projects, so every claim must help the reader choose the right tool rather than make Ze look better.

### Requirements

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

### Writing pattern

Use this shape near the top of public comparisons:

```
Scope: inspected <projects/versions/paths>. Claims cite code or official docs. Integrated products cite their integration surface and the integrated implementation when relevant. `Not found` means not found in this inspected scope.
```

### Final check

Before publishing or handing off a comparison:

- Every row MUST have source evidence, a link, or an explicit `Unclear`/`Not found in inspected scope` caveat.
- Product columns MAY be hidden when the table is too wide.
- The prose MUST NOT imply Ze is better without evidence that would convince a maintainer from the other project.

## Rationale

Code without matching docs is incomplete. "Update the docs" is not actionable.

Ze speaks to network operators in many countries, and English is a second language for many of them. A router that a reader misunderstands is a router that a reader misconfigures. The aerospace industry measured that cost in maintenance errors. It then removed the ambiguity from the language, instead of asking readers to work harder.

Agents read this text too. A controlled vocabulary and one name for each concept make our documentation searchable and quotable. Synonym rotation defeats grep, hedging defeats a decision, and a marketing adjective defeats a comparison.

Detail feels like rigor, so it grows without anyone deciding to add it. The cost is invisible at the moment of writing and paid on every read after it.

Two measurements, 2026-07-31. `CONDENSED.md` reached 99k tokens. `ai/INSTRUCTIONS.md` imports it into every session before any work starts. One table row in `repo-maintenance.md` reached 1,327 tokens. It narrates a hook's guard order, its exit codes, and its line offsets. The script and its 35 fixtures already state all three.

The drift was measurable in the old learned corpus too. Summaries averaged 27 lines in the first hundred and 93 lines in the last hundred, against a stated budget of 25 to 35. The corpus was replaced by `plan/journal/` (one file per problem class, one row per occurrence).

The citation rule has a second cost. Nine rules mint the `file:line` demand independently. Seven `ze-*` skills repeat it for each claim. A line number pinned in prose goes stale on the next edit of the file it points into. This is why `/ze-rfc-audit` must tell a real verdict change from "a pure `file:line` refresh from someone else's un-regenerated test edit".
