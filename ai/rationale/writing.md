# Why the writing rule exists

## Why a controlled language

Ze speaks to network operators in many countries, and English is a second
language for many of them. A router that a reader misunderstands is a router
that a reader misconfigures. The aerospace industry measured that cost in
maintenance errors, then removed the ambiguity from the language instead of
asking readers to work harder.

Agents read this text too. A controlled vocabulary and one name per concept make
the documentation searchable and quotable. Synonym rotation defeats grep, hedging
defeats a decision, and a marketing adjective defeats a comparison.

## Why two English variants

Software convention is US English: identifiers, standard-library names, RFC text
and the wider Go ecosystem all spell it `color`, `initialize`, `serialize`,
`behavior`. Ze follows that so its surface reads like every other tool an
operator or plugin author already knows. Thomas writes in UK English, so anything
that speaks as HIM keeps his spelling. The boundary is authorship, not medium.

## Why detail has a budget

Detail feels like rigor, so it grows without anyone deciding to add it. Its cost
is invisible at the moment of writing and paid on every read afterwards. A record
earns its length from what the reader must DO.

Narrating a gate's internals is the version of this that looks most useful. A
rule written just after a mechanism changed reads as an explanation, and the
explanation is what the author has in mind. Guard order, exit codes and line
offsets live in the code and its fixtures; a rule that copies them holds a stale
second copy.

## Why the citation rule has a second cost

The `file:line` demand was minted independently by nine rules and repeated by
seven `ze-*` skills. A line number pinned in prose goes stale on the next edit of
the file it points into. That is why an RFC audit has to tell a real verdict
change from a pure `file:line` refresh caused by somebody else's unrelated test
edit.

## Where durable lessons go

`plan/journal/`, one file per problem class and one row per occurrence. Learned
summaries and index entries grew past their budget when they were used for this
instead.

Rule: `ai/rules/writing.md`. The published style guide is
`docs/contributing/writing-style.md`.
