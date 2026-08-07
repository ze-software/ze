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
| `ai/rules/<rule>.md` | The rendered rule an agent reads | `make ze-rules-render` |
| `ai/rules/TRIGGERS.md`, `ai/rules/CORE.md` | The session payload | `make ze-rules-condensed` |
| `ai/rules/INDEX.md` | The dispatch index | `make ze-rules-index` |

An Edit to any generated file is refused by `c_rendered_rules` in
`.claude/hooks/pretool-writeedit.py`. The refusal names the point directory.

A `##` heading is a section DIRECTORY, not a point file. A `###` or `####`
heading stays a point inside its section: it is sub-structure within a section
rather than a section of its own, and that is what keeps the depth at two.

## The five things an author does

| Task | Steps |
|------|-------|
| Change one instruction | Edit the point file. Run `make ze-rules-render` |
| Add an instruction | Pick a slug no file in that SECTION uses, write the point, add the slug under that section in the manifest, render |
| Add a section | Add its line to the manifest, create the directory, and put at least one point in it. An empty section is refused |
| Remove an instruction | Delete the point file AND its manifest line. Either one alone is a hard error |
| Reorder | Move the slug, or the whole section block, in the manifest. Never rename anything: the path is the id and a gate can be bound to it |
| Change the title, trigger, severity, or related list | Edit the manifest frontmatter |

A point body is copied through verbatim. The renderer joins bodies with one
blank line and rewrites nothing inside one. That is what lets
`make ze-rules-points-roundtrip` prove no byte was lost.

## Pick a slug that is free

The slug is a bare lowercase kebab-case component, and the path it forms is the
point's permanent id. A section directory's name is a slug on the same terms.
Before you create a point, check the name:

    ls ai/rules/points/<rule>/<section>/<slug>.md

A Write to a name that already exists is REFUSED by `c_point_overwrite` in
`.claude/hooks/pretool-writeedit.py`. The Write replaces the whole file, so the
instruction in it is gone before any gate runs. The refusal names both routes:
edit the point that is there, or pick a slug no file uses. An Edit is targeted
and stays permitted, and so does a Write to a free slug.

## The frontmatter fields

Every point declares three fields, and two are usually empty. Two more are
optional: they are written only when they carry a value, because the split
cannot derive either one and an empty line would claim the point was examined.

| Field | Values | Notes |
|-------|--------|-------|
| `kind` | `directive`, `table`, `note`, `heading`, `fence` | Describes the block. `heading` and `fence` are structural, so `make ze-rules-gate-map` leaves them out of its counts |
| `level` | `MUST`, `MUST NOT`, `SHOULD`, `MAY`, or empty | The strongest RFC 2119 level the body states. About 95% of the corpus states none |
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
general point naming nothing, and `make ze-rules-gate-map` fails.

Declare it when the exception lives in a DIFFERENT point. An instruction that
states its own carve-out in the same block needs no link, and neither does one
whose exception is a co-equal branch of a decision table. An invented link is
worse than an absent one: coverage is a measurement and never a red.

The manifest frontmatter carries `title`, `when`, `severity` and an optional
`related`. `scripts/dev/rules_lint.py` validates what those produce in the
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

## What the renderer refuses

`scripts/dev/rules_points.py` fails closed rather than rendering a partial rule.

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
the smaller corpus.

So a removal is declared: one row in `ai/rules/points/RETIRED.md`, naming the id
and saying what happened to the instruction. `make ze-rules-gate-map` compares
each rule's point count against git HEAD and fails when a drop is not covered by
a row added since HEAD. A row stops counting once it is committed, so nothing
there pre-approves a future deletion.

Moving a point between sections is a rename, not a retirement: the count does
not change, and the point's `# ze point:` binding is repointed at the new id.

## Binding a hook check to a point

A check declares what it enforces with one comment directly above its `def`.
Only a blank line or another comment can sit between the two.

    # ze point: performance/directives/write-wire-encoding-into-pooled-bounded-buffers
    def c_encoding_alloc(ctx):

A check that enforces nothing written in `ai/rules/` says so, with a reason:

    # ze point: none -- build hygiene, and no rule states where a Go binary lands
    def check_root_build(ctx):

`make ze-rules-gate-map` joins those comments against the points on disk, and
joins the two optional link fields the same way. It reports the gated points,
the dangling bindings, the points that regressed, the checks that declare
`none`, the two sets of links naming nothing, the ungated count, and the two
coverage counts.

Six of those sets fail. Dangling is a binding naming a point that does not
exist. Regressed is a point that carried a binding at HEAD and carries none now,
which is the one route from gated to ungated that leaves every other gate green.
Declared-none is the same route with one more step: rename the point, then
rewrite the dangling binding as `none -- <why>`. Shrunk is a rule that lost
points no `RETIRED.md` row accounts for. The last two are a `rationale` naming
no record and an `excepted-by` naming no point: the same defect one direction
out, where the explanation or the exception moved out from under the
instruction.

Every count is a measurement and exits 0 whatever its value.

The files it reads come from the PreToolUse entries in `.claude/settings.json`.
A fourth dispatcher joins the map by being wired up. A dispatcher that no entry
runs is reported rather than skipped.

The same target compares the Hook-to-Rule Mapping table in
`ai/rules/repo-maintenance.md` against those comments. A new check therefore
owes a row in that table, and a deleted check's row cannot survive it.

## The order the generators run in

`make ze-rules-condensed` and `make ze-rules-index` parse the RENDERED rules, so
a render that has not run yet feeds them the previous text.

    make ze-rules-render
    make ze-rules-condensed
    make ze-rules-index
    make ze-rules-lint

`make ze-doc-test` runs all of them, plus `ze-rules-render-check`,
`ze-rules-points-roundtrip` and `ze-rules-gate-map`. Run it before you commit.

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

    git archive HEAD ai/rules scripts/dev/rules_points.py scripts/dev/rules_condensed.py \
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
`ai/rules/INDEX.md` can be derived instead of hand-written. The measurements are
in `plan/learned/1228-rule-format-condensed-eager-load.md`.

## Why one instruction is one file

The unit of enforcement used to be one rule file, and a rule file holds hundreds
of instructions. Nothing named the sentence a check enforced. Nothing counted
what a refusal had prevented. Nothing said whether a reworded instruction still
had a gate behind it.

The path is now the id, and a machine answers the first two. `gate_map` in
`scripts/dev/rules_points.py` names the point each check enforces, and a refusal
cites that point rather than a 900-line file.

The third is answered in REVIEW, not by a machine. `Binding` carries the point's
path and no digest of its body, so rewording a body under the same slug leaves
the binding green. A content hash as the id was considered and rejected: it
changes the id on every edit, and every gate bound to it breaks on a typo fix.
What the path buys here is that a reword is a one-file diff, under a name that
did not move, which a reviewer sees and `git log` dates.

Reading order lives in the manifest, not in a numeric filename prefix. That is
what lets you reorder a rule and keep every binding into it.
