# Authoring an Agent Rule

Agent rules live in `ai/rules/`. An agent READS `ai/rules/<rule>.md`. An author
EDITS the point files that generate it. This page is the author's walk-through.
The obligations are in `ai/rules/rule-format.md`, which is short on purpose.

## The layout

One rule is one directory, and inside it one directory per `##` section. The
depth is fixed at two, so a point id is always `<rule>/<section>/<slug>`.

| Path | Holds | Who writes it |
|------|-------|---------------|
| `ai/rules/points/<rule>/manifest.md` | The title, the metadata block, the sections and the reading order | You |
| `ai/rules/points/<rule>/<section>/<slug>.md` | One point: a frontmatter header, then the body verbatim | You |
| `ai/rules/<rule>.md` | The rendered rule an agent reads | `./le rules render-update` |
| `ai/rules/TRIGGERS.md`, `ai/rules/CORE.md` | The session payload | `./le rules condensed-update` |
| `ai/rules/INDEX.md` | The dispatch index | `./le rules index-update` |

An Edit to any generated file is refused by `writeRenderedRule` in
`internal/le/hookruntime/writeedit.go`. The refusal names the point directory.

A `##` heading is a section DIRECTORY, not a point file. A `###` or `####`
heading stays a point inside its section: it is sub-structure within a section
rather than a section of its own, and that is what keeps the depth at two.

## The five things an author does

| Task | Steps |
|------|-------|
| Change one instruction | Edit the point file. Run `./le rules render-update` |
| Add an instruction | Pick a slug no file in that SECTION uses, write the point, add the slug under that section in the manifest, render |
| Add a section | Add its line to the manifest, create the directory, and put at least one point in it. An empty section is refused |
| Remove an instruction | Delete the point file AND its manifest line. Either one alone is a hard error |
| Reorder | Move the slug, or the whole section block, in the manifest. Never rename anything: the path is the id and a gate can be bound to it |
| Change the title, trigger, severity, or related list | Edit the manifest frontmatter |

A point body is copied through verbatim. The renderer joins bodies with one
blank line and rewrites nothing inside one. That is what lets
`./le rules points-roundtrip-check` prove no byte was lost.

## Pick a slug that is free

The slug is a bare lowercase kebab-case component, and the path it forms is the
point's permanent id. A section directory's name is a slug on the same terms.
Before you create a point, check the name:

    ls ai/rules/points/<rule>/<section>/<slug>.md

A Write to a name that already exists is refused by `writePointOverwrite` in
`internal/le/hookruntime/writeedit.go`. The Write replaces the whole file, so the
instruction in it is gone before any gate runs. The refusal names both routes:
edit the point that is there, or pick a slug no file uses. An Edit is targeted
and stays permitted, and so does a Write to a free slug.

## The frontmatter fields

Every point declares three fields, and two are usually empty. Two more are
optional: they are written only when they carry a value, because the split
cannot derive either one and an empty line would claim the point was examined.

| Field | Values | Notes |
|-------|--------|-------|
| `kind` | `directive`, `table`, `note`, `heading`, `fence` | Describes the block. `heading` and `fence` are structural, so `./le rules gate-map-report` leaves them out of its counts |
| `level` | `MUST`, `MUST NOT`, `SHOULD`, `SHOULD NOT`, `MAY` | The strongest RFC 2119 level the body states. Required on a `directive`, empty on every other kind. See "Every directive states a level" |
| `stage` | empty | Reserved. It will let a design-phase agent skip implementation directives. Leave it empty |
| `rationale` | a repo-relative path, or the line absent | Where the record of WHY this instruction exists lives: a `plan/learned/NNNN-*.md` summary, or an `ai/rationale/*.md` file |
| `excepted-by` | one or more point ids, comma-separated, or the line absent | The point or points that carve an exception out of this one |

Both optional fields are HEADER fields, so neither reaches the body and the
rendered rule is byte-identical whether a point carries one or not. Both are
also links, so both fail the gate map when they name nothing:

    ---
    kind: directive
    level: MUST
    stage:
    excepted-by: writing/language-and-spelling/prose-written-in-thomas-s-voice-keeps-uk-british-english
    ---
    - **The project language is US English.** ...

`excepted-by` is declared on the GENERAL point and names the EXCEPTION, never
the other way round. A general instruction must carry its own exception, or a
reader who stops after the general statement is misled, and the repetition that
prevents that is invisible: `ai/rules/writing.md` states the UK English
exception at three levels, and a dedup pass can delete one copy with every gate
staying green. The link closes that. Deleting the exception point leaves the
general point naming nothing, and `./le rules gate-map-report` fails.

Declare it when the exception lives in a DIFFERENT point. An instruction that
states its own carve-out in the same block needs no link, and neither does one
whose exception is a co-equal branch of a decision table. An invented link is
worse than an absent one: coverage is a measurement and never a red.

The manifest frontmatter carries `title`, `when`, `severity` and an optional
`related`. `internal/le/rules.Answer` validates what those produce in the
rendered file, so the metadata contract did not change when the format did.

The manifest BODY is the rule's structural spine, and it has exactly two line
shapes. A section line is the directory name, one space, then the `##` heading
line VERBATIM. A point line is that section's slug, indented by two spaces.

    directives ## Directives
      write-the-test-first-and-never-weaken-it
      use-project-tmp-for-scratch-files

The heading lives here because a directory name cannot carry capitalisation,
punctuation or the marker, and the rendered rule must come back byte for byte.
A body line matching neither shape is a hard error, never a skipped line.

## Every directive states a level

A rule exists to settle what an agent owes. An instruction whose weight a reader
infers from tone is an instruction two readers weigh differently, so every point
whose `kind` is `directive` states its obligation in RFC 2119 language:

| You mean | Write | `level:` |
|----------|-------|----------|
| An obligation, or a ban | MUST, MUST NOT | `MUST`, `MUST NOT` |
| A strong default a reader can depart from with a stated reason | SHOULD, SHOULD NOT | `SHOULD`, `SHOULD NOT` |
| A permission | MAY | `MAY` |

SHALL, SHALL NOT, REQUIRED, RECOMMENDED, NOT RECOMMENDED and OPTIONAL are
accepted in the body and collapse onto the level they name, so `level:` carries
one spelling per level. When a body states several, `level:` names the strongest
TIER: MAY, then SHOULD with SHOULD NOT, then MUST with MUST NOT. RFC 2119 does
not rank MUST against MUST NOT, so a point stating both declares whichever its
central clause carries.

The lowercase spellings `must`, `shall`, `should` and `may` are refused in a
directive body. They read as the obligation word and carry none of its force,
and `ai/rules/writing.md` bans the hedging spelling outright. Capitalise the
keyword, or rewrite the sentence so it carries no modal at all.

Text inside a code span or a fenced block is quoted, never stated, so neither
gate reads it. A shell snippet or a reproduced error message keeps its own
spelling, and a rule that NAMES a banned word as an example puts it in
backticks.

A block that states no obligation is `kind: note` or `kind: table`, never
`kind: directive`. The gate is scoped to directives on purpose: a two-column
lookup gains a word and no obligation from being made to say MUST.

Two gates enforce this. `writePointLanguage` in
`internal/le/hookruntime/writeedit.go` refuses the write, and `./le rules lint`
refuses the finished tree. A Write carries the whole point, so a missing keyword
is refused there; an Edit carries a fragment, so only the lowercase modal it
introduces is decidable at write time. Run the pass alone with:

    ./le rules lint

## What the renderer refuses

`internal/le/rules.Answer` fails closed rather than rendering a partial rule.

| Refused | Why |
|---------|-----|
| A point file the manifest does not list | The manifest is the reading order, so the point would vanish from the rule with nothing going red |
| A `*.md` sitting directly in a rule directory | It is outside every section, so it has no three-part id and nothing renders it |
| A section directory the manifest does not list | The same silent drop, one level up: a whole section leaves the rule |
| A manifest slug or section with no file or directory | Half a rule is worse than a failed render |
| The same slug twice in one section, or the same section twice | The body would be emitted twice |
| A section listing no point | An empty directory carries no instruction and does not survive a clone |
| A slug carrying a path separator, a leading dot, or a parent reference | A manifest MUST NOT read outside its own rule directory |
| A `*.md` below its section directory, or a directory inside one | The tree is at a fixed depth of two and nothing reads deeper, so the instruction renders into nothing while every gate exits 0 |
| A `##` heading inside a point BODY | A `##` opens a section, and a section is a directory. The rendered bytes are identical either way, so this is the one loss no other gate can see: every point after that heading carries an id naming a section no reader ever sees |

A `##` heading with no blank line above it is still a section. The manifest
records the missing blank line with a leading `^` on the section line, so the
rendered rule keeps the bytes the source had.

## Retiring a point

A point IS an instruction. Delete the file and its manifest line together and
every gate stays green, because the points and the rendered rule then agree on
the smaller corpus. Git history says what left and when, so no ledger file
records the removal.

Moving a point between sections is a rename, not a retirement: the point's
`// ze point:` binding is repointed at the new id.

## What the session payload artifacts hold

Two files load into every session, and one generator emits both from one parse.

| Artifact | Holds |
|----------|-------|
| `ai/rules/TRIGGERS.md` | One routing line per rule: its path, its severity, and its `**When:**` trigger. Every rule appears, so none is ever invisible |
| `ai/rules/CORE.md` | The condensed directives of the always-on rules only |

<!-- source: internal/le/rules/artifacts.go -- core_members -->
Core membership is derived, never listed. A rule is always-on when the ladder in
`ai/rules/rule-precedence.md` names it on rung 1 or 2, when it IS that ladder,
when it has no routable trigger, or when no past task description in `plan/`
would surface it. `./le rules router-report` prints that last set and the corpus
it read, and `./le rules payload-report` measures what a session loads against a
40,000-token budget.

## Binding a hook check to a point

A native check declares what it enforces with a `// ze point:` comment directly
above its Go function. Only another comment may sit between the binding and the
declaration.

    // ze point: performance/directives/write-wire-encoding-into-pooled-bounded-buffers
    func writeGoPatterns(ctx context) *verdict {

A check that enforces nothing written in `ai/rules/` says so, with a reason:

    // ze point: none -- build hygiene; no rule states where a Go binary must be written
    func bashRootBuild(ctx context) *verdict {

<!-- source: internal/le/rules/coverage.go -- bindingLine, noPointLine -->
`./le rules gate-map-report` joins those comments against the points on disk, and
joins the two optional link fields the same way. It reports the gated points,
the dangling bindings, the points that regressed, the checks that declare
`none`, the two sets of links naming nothing, the ungated count, and the two
coverage counts.

Six of those sets fail. Dangling is a binding naming a point that does not
exist. Regressed is a point that carried a binding at HEAD and carries none now,
which is the one route from gated to ungated that leaves every other gate green.
Declared-none is the same route with one more step: rename the point, then
rewrite the dangling binding as `none -- <why>`. The last two are a `rationale`
naming no record and an `excepted-by` naming no point: the same defect one
direction out, where the explanation or the exception moved out from under the
instruction.

Every count is a measurement and exits 0 whatever its value.

The files it reads come from the PreToolUse entries in `.claude/settings.json`.
A fourth dispatcher joins the map by being wired up. A dispatcher that no entry
runs is reported rather than skipped.

The same target compares the check tables in `.claude/hooks/README.md` against
those comments. Each table sits under a heading naming its Go source and opens
with the `Check` and `Enforces` columns. A new check therefore owes a row, and
a deleted check's row cannot survive it. What each check triggers on and what
it does is documentation, and it sits in the remaining column of the same
table.

## The order the generators run in

`./le rules condensed-update` and `./le rules index-update` parse the RENDERED rules, so
a render that has not run yet feeds them the previous text.

    ./le rules render-update
    ./le rules condensed-update
    ./le rules index-update
    ./le rules lint

`./le doc check verify` runs all of them, plus `./le rules render-check`,
`./le rules points-roundtrip-check` and `./le rules gate-map-report`. Run it before you commit.

After a trigger edit, READ your rule's row in the regenerated `TRIGGERS.md`. A
trigger that lost half its clause is not visible from the point file alone.

## Committing

Commit the points and all four generated artifacts together: the rendered rule,
`TRIGGERS.md`, `CORE.md` and `ai/rules/INDEX.md`. The freshness gates regenerate
from the WORKING TREE and compare against it. A commit that carries the point
and not the artifact is therefore green on your machine. CI regenerates from
HEAD, and it fails.

Several sessions share this checkout, so your working tree can hold another
author's unlanded rule text. When it does, generate the artifacts from HEAD plus
your own points rather than from the shared tree:

    git archive HEAD ai/rules internal/le/rules.Answer internal/le/rules.Answer \
        | tar -x -C <scratch>
    # copy your edited point files into <scratch>/ai/rules/points/
    # run the generators there, and commit what they produce

## Why the digest exists

Rules used to load into every session in full. That cost about 99,600 tokens on
every turn. Every session and every subagent paid it, whether or not any of it
applied. A session editing one markdown file paid what a session rewriting the
BGP wire encoder paid. No single rule exceeded 6.4% of the total, and the top 20
were only 55% of it. Trimming files therefore cannot fix a cost shaped like
that.

`TRIGGERS.md` plus `CORE.md` replaced that import. They measure about 21,100
tokens, an 80% reduction. The safety property is awareness rather than
inclusion. Every rule keeps a line in `TRIGGERS.md`. A rule whose body is not
loaded is therefore still named in every session, one Read away.

That is what makes the `**When:**` trigger load-bearing. It is also why
`ai/rules/INDEX.md` can be derived instead of hand-written.

## Why one instruction is one file

The unit of enforcement used to be one rule file, and a rule file holds hundreds
of instructions. Nothing named the sentence a check enforced. Nothing counted
what a refusal had prevented. Nothing said whether a reworded instruction still
had a gate behind it.

The path is now the id, and a machine answers the first two. `gate_map` in
`internal/le/rules.Answer` names the point each check enforces, and a refusal
cites that point rather than a 900-line file.

The third is answered in REVIEW, not by a machine. `Binding` carries the point's
path and no digest of its body, so rewording a body under the same slug leaves
the binding green. A content hash as the id was considered and rejected: it
changes the id on every edit, and every gate bound to it breaks on a typo fix.
What the path buys here is that a reword is a one-file diff, under a name that
did not move, which a reviewer sees and `git log` dates.

Reading order lives in the manifest, not in a numeric filename prefix. That is
what lets you reorder a rule and keep every binding into it.
