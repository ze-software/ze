# 021 - A gate that narrows its scope must not believe the narrowing

**Spec:** spec-a-literal-restates-a-registry, closed 2026-09-14
**Class:** `plan/journal/zero-value-as-valid-answer.md`

## What the work built

`./le enumeration` refuses a Go literal that enumerates what a live registry
already holds. Its five key corpora are read from the registries in the tool's
own process, behind a blank import of the product composition root, so the gate
carries no copy of any key set. `check` judges the change set and blocks;
`report` covers the tree and measures. The same work taught
`./le plugin imports` a fifth registrar kind, the CLI command owner, which took
`cmd/ze/ze_core_dispatch.go` from 24 module-internal blank imports to 6.

## Decisions

**A literal that FEEDS a registry is a declaration; only one that READS the same
key space is a copy.** That distinction is worth more than any threshold. It
removed the plugin registrations, the doctor checks and the per-plugin
diagnostic code blocks from the finding set without a single exemption, and it
is why `declarationRanges` exists rather than a longer marker list.

**Whether a const block can BE a declaration depends on the registry, not on the
syntax.** A diagnostic code enters the registry as the const itself, so the
const is where it comes from. A family name is composed by the registrar from an
AFI name and a SAFI name, so every Go const holding the joined string is a
second spelling and none of them is the declaration. `Corpus.WrittenWhole`
carries that difference.

**The key-share ratio separates only the tail.** Measured over the checkout:
0.6 loses `IsReadOnlyPath` and nine of the 27 family rows; 0.1 loses three
literals that are copies; 0.08 loses none and still drops fourteen rows. Above
8 percent no ratio separates `validatedSections`, which is a copy, from
`rtProtoNames`, which is the kernel's namespace, at the same five strings of
fifteen. The residue is not reachable by any ratio and is not pursued.

**A gate over a whole tree and a gate over a change set are different products.**
This one had to become the second: 230 genuine findings cannot be marked before
one is fixed, and a first state that is mostly suppression teaches every later
reader that the markers are noise.

## The trap

**A fact published to a file loses every column the file format does not carry,
and the reader cannot tell the loss from an answer.**

`changed.WriteScopePackages` publishes a verify run's change set as packages,
one per line. `ScopeReport.Widened` and `ScopeReport.Reason` are not in it. On a
checkout with no green baseline the selector widens, publishes `./...`, and
`Scope.fromFile` hands that back with `Widened` false. The enumeration stage
read the flag, took the narrow route, matched every finding through an empty
prefix, and blocked every verify run with 230 rows while printing a scope line
that said it had judged one package.

Nothing was wrong with the publication for any earlier reader: every other
consumer wants "test everything" from `./...`, which is what it gets. The loss
only bites a consumer that distinguishes *everything changed* from *I cannot
tell*, and this gate is the first one.

**The repair generalises.** The fact was still there, in the packages rather than
in the flag, because `./...` is written by `widen` and `failOpen` and by nothing
else. Reading it off the answer that survives the round trip beats adding a
column, which would be a second declaration of what the packages already state,
in a spec whose whole subject is second declarations.

## The rejected alternative

Adding a `widened` header line to the published file. It was rejected for the
reason the gate itself exists: the selection already says it, and two places
stating one fact is the defect. The cost of the choice is that a HUMAN reading
the artifact after a red run still cannot see WHY the scope is as wide as it is,
because the reason goes to stderr and nowhere else. That is recorded in the
journal row rather than fixed here.

## What to check next time

A threshold that decides a verdict and has no test that fails when it moves is
not a threshold, it is a number. Three of the four constants here had one;
`closedGroupKeysMin` did not, and lowering it from 3 to 2 left every test green
while turning `true`/`false` in any literal into a YANG enumeration copy. Break
each constant and watch which test goes red. If none does, the constant is
undefended.
