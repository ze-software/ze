# Simplified Technical English (ASD-STE100 Issue 9)

**When:** writing or reviewing any prose in this repository: docs, comments, error messages, CLI output, YANG descriptions, specs, or commit messages
**Severity:** blocking
**Related:** language-and-spelling, documentation, error-messages, comparison-honesty

## Directives

Ze writes in ASD-STE100 Simplified Technical English, Issue 9 (2025-01-15). This is rule one of the repository. The working guide is `docs/contributing/writing-style.md`, which is committed and complete on its own.

- **Write every sentence in STE.** One topic per sentence, active voice, the imperative form for instructions, and an approved verb for every action.
- **The six habits in the next section are the operative list.** A numbered STE rule bans each one. Learn the six first, then read the standard for the remainder.
- **A short, plain, repetitive sentence is correct.** Elegant variation is a defect, not a style.
- **Maximum 20 words in a procedural sentence** (STE Rule 5.1). This covers guide steps, runbooks, remediation lines, and commit messages.
- **Maximum 25 words in a descriptive sentence** (STE Rule 6.3), and maximum 6 sentences in a paragraph (STE Rule 6.6).
- **No semicolons** (STE Rule 8.1). Write two sentences, or use a vertical list (STE Rule 4.3).
- **No contractions, and do not omit words** (STE Rule 4.2). Write "do not", never "don't".
- **US English spelling** (STE Rule 1.14). This agrees with `language-and-spelling.md` for all project text.
- **When you cannot say it in STE, the text is unclear, not the rule.** Change the sentence construction (STE Rule 9.1).
- **One word, one meaning, one part of speech.** Do not verb a noun, and do not carry two senses of a word through one document (STE Rules 1.2, 1.3, 1.7).
- **No slang, no jargon, no regional words.** `brick the router`, `bounce the daemon`, and `footgun` are not English a reader can look up (STE Rule 1.10).
- **Three words in a noun stack, no more.** Break a longer one with a preposition (STE Rules 2.1, 2.2). Expand an abbreviation on first use.
- **A replacement must keep the meaning.** `should` becomes MUST only for a real obligation, and `must` never sits in front of an imperative (STE Rules 1.2, 5.3).
- **Splitting a sentence is half the job. Connect the pieces.** Give information gradually, repeat the key word, open a paragraph with its topic sentence, and use a connector. `And` and `But` are correct sentence openers (STE Rules 4.4, 6.1, 6.2, 6.4, 6.5).
- **Keep the conjunction `that`,** and never drop an article to shorten a line (STE Rules 4.5, GR-1).
- **No definite article before a noun that carries an alphanumeric identifier.** Write `RFC 4271` and `peer 10.0.0.1` (STE Rule 4.5).
- **No Latin abbreviations.** Write `for example`, `that is`, and `and so on` (STE GR-6). Use gender-neutral language (GR-7).
- **The `-ing` form is permitted as a technical noun or its modifier, and nowhere else.** `routing table` is correct, and `before installing` is not (STE Rule 3.5).
- **Formatting, units, and abbreviation policy are Ze's own.** The standard sets no rules for them, so the uppercase in a safety block is our choice, not a requirement.

## The six habits to avoid

| # | Habit | What it looks like | Banned by | Write instead |
|---|-------|--------------------|-----------|---------------|
| 1 | **Synonym rotation** | one concept with three names on one page: `peer`, then `neighbor`, then `session`. Also the formal word where a plain one exists: `initiate`, `terminate`, `obtain`, `utilize`, `ascertain` | STE Rules 1.3, 1.11, 9.4, and the controlled vocabulary itself | Give each concept one name, and make that name the plain one: `start`, `stop`, `get`, `use`, `make sure`. Keep a domain verb that no plainer word replaces (`implement`, `negotiate`, `withdraw`) |
| 2 | **Hedging** | `may`, `might`, `should probably`, `generally`, `typically`, `in most cases`, `we believe`, `it seems` | STE Rule 1.1 and the dictionary: `may` -> CAN, `should` -> MUST, `generally` and `normally` -> USUALLY. `might`, `could`, and `typically` have no entry at all | State the fact. CAN for a possibility, MUST for an obligation, WILL for a future event. If you do not know, write that you do not know |
| 3 | **Frozen verbs** | `do the installation of the plugin`, `performs validation of the config`, `before the removal of the unit` | STE Rule 3.7, with Rules 1.13 and 3.5 | Use the verb: `install the plugin`, `validates the config`, `before you remove the unit` |
| 4 | **Marketing adjectives** | `powerful`, `seamless`, `robust`, `blazing fast`, `feature-rich`, `effortless`, `battle-tested`, `best-in-class` | STE Rule 1.1: these words have no dictionary entry and they are not technical nouns | Give the number, the limit, or the mechanism. Then delete the adjective (`comparison-honesty.md`) |
| 5 | **Run-ons** | one sentence with three clauses, a semicolon splice, a paragraph of eight sentences | STE Rules 4.1, 5.1, 6.3, 6.6, 8.1 | Write one topic in each sentence. Split the sentence. Use a vertical list for complex text (STE Rule 4.3) |
| 6 | **Phrasal verbs** | `spin up the daemon`, `kick off the run`, `figure out the cause`, `get rid of the route`, `put out the fire` | STE Rule 9.3 | Use one verb: `start the daemon`, `start the run`, `find the cause`, `delete the route`, `extinguish the fire` |

## Words that stay verbatim

| Surface | Why it does not change |
|---------|------------------------|
| RFC 2119 keywords (MUST, MUST NOT, SHOULD, MAY) when they name an RFC's obligation level | The keyword IS the requirement. `rfc-compliance.md` and every `RFC requirement:` test tag read the exact word. The dictionary substitution `should -> MUST` never applies to a quoted normative term |
| Quoted external text: RFC prose, vendor output, peer daemon log lines, third-party documentation | A quotation is evidence. A changed quotation is false evidence (`no-fabrication.md`) |
| Go identifiers, YANG leaf names, JSON keys, env var keys, CLI tokens, command grammar | `naming.md`, `config-naming.md`, `yang-structure.md`, and `cli-grammar.md` own these. STE governs prose, never identifiers |
| Technical nouns and technical verbs of this subject field: `peer`, `prefix`, `NLRI`, `teardown`, `netlink`, `deenergize` | STE Rules 1.5, 1.6, and 1.12 permit them. `request peer teardown` is a technical noun and is correct. `tear down the peer` is a phrasal verb and is not |
| Test fixture data, hex dumps, config samples, and fenced code blocks | These are data, not prose |

## Scope

| Surface | STE applies |
|---------|-------------|
| `docs/` guides, references, comparisons, architecture pages | Yes |
| Code comments and godoc | Yes |
| Error messages, log lines, diagnostic remediation text | Yes, together with `error-messages.md` |
| CLI output, help text, completions, TUI labels | Yes |
| YANG `description` strings | Yes |
| `ai/` rules, patterns, and digests, plus `plan/` specs and learned summaries | Yes |
| Commit messages and PR text | Yes |
| Chat replies, reports, and analysis for the user | No. Answer the person who asked |
| Thomas's authored prose: blog posts, articles, emails, the weekly update (`/write`, `/ze-weekly-update`) | No. That prose is his voice and it stays UK English (`language-and-spelling.md`) |
| Identifiers, keys, tokens, quoted external text, and fixture data | No. Refer to "Words that stay verbatim" |

## Where to read the detail

- **Read `docs/contributing/writing-style.md` first, and expect to need nothing else.** It is committed, and it is Ze's own text.
- That page carries every operative point. It covers the six habits with Ze examples, the sentence and paragraph limits, and verbs and voice. It also covers conditions, warnings, punctuation, the word-count convention, and the per-surface notes.
- The published standard stays the authority for a question that page does not answer. ASD gives Issue 9 at no cost: `https://www.asd-ste100.org`. The direct file is `https://www.asd-ste100.org/assets/files/ASD-STE100_ISSUE9.pdf`.
- Issue 9 has 53 writing rules in 9 sections, approximately 900 approved words, and approximately 1200 unapproved words with their alternatives.
- **The document is copyright (c) ASD 2025 and Ze has no reproduction right.** The special usage rights cover ASD, AIA, and ICCAIA members and their customers. They also cover ministries of defense, airworthiness authorities, and universities. Ze is in none of these categories.
- **Keep the PDF, its text, and its dictionary out of the tracked repo.** Put the local copy in `tmp/asd-ste100/`, which `.gitignore` excludes. Naming the standard is free, and copying its text is not.
- A converted copy at `tmp/asd-ste100/ASD-STE100_ISSUE9.md` is local and uncommitted. When that file is absent, download the PDF again.

## Enforcement

| Command | What it does |
|---------|--------------|
| `scripts/dev/commit_helper.py create` | **The gate.** `ste_problems` BLOCKS a commit whose own `.md`, `.go`, or `.yang` files grew a habit. It runs on the files of that commit, so the prose it judges has one author |
| `make ze-ste-check` | The same gate over every changed file in the working tree. Run it before you prepare a commit. About 2 seconds |
| `make ze-ste-review` | The whole-tree report. Every finding with its `file:line`, its habit number, and the replacement to use |
| `make ze-ste-review-changed` | The same report for changed files only |
| `python3 scripts/dev/ste_check.py <file>...` | Review named documents |
| `git log -1 --format=%B \| python3 scripts/dev/ste_check.py -` | Review a commit message or a PR body before you send it |

- **HEAD is the baseline, and the comparison is per file.** A document nobody touched can never fail the gate, so legacy prose stays until someone rewrites it. The sentence you just wrote is what goes red.
- **There is no baseline file, and nothing to re-bless.** Rewriting a number cannot silence this gate, so the one way to green is to fix the prose (`ai/rules/no-parking.md`).
- **The gate is at commit time, not in `ze-doc-test`.** Several sessions share this checkout. A tree-wide prose gate reports a sibling session's in-flight sentences, and a gate that reddens for a colleague's typing gets switched off.
- **`make ze-ste-check` still reads the whole working tree**, so it can name a file another session is editing. Read the path before you read the habit.
- **The checker holds our own word lists, not the ASD dictionary.** It cannot see every violation, so the six habits stay a review checklist as well as a gate. Report a violation as an ISSUE against its habit number.
- **When the tool is wrong, fix the tool and add the case to `scripts/dev/ste_check_test.py`.** A checker that flags `setup`, an RFC 2119 MUST, or a code span gets switched off, and then it protects nothing.
- **Escape hatch for a document that must quote non-STE text at length:** `<!-- ste: ignore-file <reason> -->`, or `<!-- ste: ignore -->` above one line. The reason is mandatory.
- **Surfaces the tool reads:** Markdown in `docs/`, `ai/`, `plan/`, and the repository root. Prose comments in `.go`. The `description` strings in `.yang`. Piped text on stdin. It never reads `rfc/`, which stays verbatim.

## Mechanical check

Before you publish a sentence, answer six questions:

1. Does each concept in this file have exactly one name? (habit 1)
2. Did I write a fact, or did I hedge? (habit 2)
3. Is each action a verb? (habit 3)
4. Did I praise the product instead of measuring it? (habit 4)
5. Is the sentence shorter than 20 words in a procedure, or 25 words in a description? (habit 5)
6. Is each verb a single word? (habit 6)

## Rationale

Ze speaks to network operators in many countries, and English is a second language for many of them. A router that a reader misunderstands is a router that a reader misconfigures. The aerospace industry measured that cost in maintenance errors. It then removed the ambiguity from the language, instead of asking readers to work harder.

Agents read this text too. A controlled vocabulary and one name for each concept make our documentation searchable and quotable. Synonym rotation defeats grep, hedging defeats a decision, and a marketing adjective defeats a comparison.
