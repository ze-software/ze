# Language and Spelling

**When:** writing any project text, or any prose in Thomas's voice
**Severity:** blocking

## Directives

The project language is **US English**. Every artifact that is part
of Ze -- code, docs, and user-facing text -- uses US English spelling, wording,
and date/number conventions. The single exception is prose authored in Thomas's
own voice, which uses **UK (British) English**. See "The one exception" below.

## Why two variants

Software convention is US English: identifiers, standard-library names, RFC text,
and the wider Go ecosystem all spell it `color`, `initialize`, `serialize`,
`behavior`. Ze follows that so its surface reads like every other tool an operator
or plugin author already knows. Thomas, however, writes in UK English, so anything
that speaks as *him* keeps his spelling. The boundary is authorship, not medium:
who the text speaks as decides the variant.

## US English -- everything that is the project

Use US English for all of the following. This list is illustrative, not exhaustive:

| Surface | Examples |
|---------|----------|
| Go identifiers, types, functions, fields | `color`, `Normalize`, `Analyzer`, `licenseKey` |
| Code comments (`// ...`) and godoc | "normalize the value", "this behavior" |
| Error messages, log lines, diagnostic codes | see `ai/rules/error-messages.md` |
| CLI output, help text, completions, TUI labels | `ze` command help, dashboards |
| YANG leaf descriptions and config help text | schema `description` strings |
| `docs/` (user + architecture documentation) | guides, references, comparisons |
| `ai/` rules, patterns, digests, indexes | this file included |
| `plan/` specs, learned summaries | spec bodies, ACs |
| Commit messages and PR text | subject and body |

Common divergences to get right (US -- avoid the UK form):

`color` (not colour), `behavior` (not behaviour), `initialize` / `normalize` /
`serialize` / `organize` / `optimize` / `recognize` (not the `-ise` forms),
`license` as both noun and verb (not licence), `catalog` (not catalogue),
`center` (not centre), `canceled` / `canceling` (one `l`), `analyze` (not
analyse), `fiber` (not fibre), `gray` (not grey), `defense` (not defence).

## The one exception -- Thomas's authored prose is UK English

Prose written in Thomas's voice keeps UK (British) English: `colour`, `behaviour`,
`organise`, `licence` (noun), `centre`, and so on. This covers everything produced
under the `/write` skill and anything that reaches a reader as Thomas himself:

- Blog posts, articles, essays.
- Emails and letters sent from Thomas.
- The Ze weekly update prose (`/ze-weekly-update`), which is Thomas's public
  voice even though the surrounding tooling and code are US English.

The `/write` skill (`~/.claude/commands/write.md`) is where this variant is
applied; it also carries Thomas's no-em-dash and voice guidance. When in doubt:
if the text is *about* Ze it is US English; if the text *is* Thomas talking, it is
UK English.

## Current state and drift

Ze is already de-facto US English (the large majority of files use the US forms).
A small amount of UK spelling has leaked into `docs/` over time. Do not run a blind
global find/replace: some occurrences are quoted RFC/BIRD text or proper nouns that
must stay verbatim. Fix drift opportunistically when you touch a file, matching the
surrounding US convention, and leave quoted external text untouched.

## Mechanical check

Before writing or reviewing any project text, ask: is this text the project
speaking (US English) or Thomas speaking (UK English)? Spell to match. New
identifiers, comments, docs, and error strings must be US English with no
exceptions.
