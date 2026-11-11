# Inventory traversal review

The source inventory still has no complete result for the actual repository.
Its reflection analysis can revisit a shared type through many paths, even when
those visits add the same method to the answer. The pending decision concerns
how to avoid proven duplicate work while preserving the analysis that prevents
an incomplete inventory from being published as complete.

The approved input boundary remains unchanged. The analyzer inspects retained
Git source and checks whether external code can reach registration functions or
alter registration metadata through accessible values. Ordinary reflection over
those values is part of that analysis. The external-effects decision, removed
from the tree when `spec-inventory-fast-registry-checks` replaced the proof
(`plan/learned/DESIGN-HISTORY.md`, "Plugin system: architecture"), recorded
its bounded trust contract. Neither traversal proposal adds library
source or treats familiar libraries as harmless.

The last retained test of the actual repository,
`TestAnswerAnswersTheInventory`, reached its unchanged ten-minute limit without
a report. That run followed three approaches: cheaper binding and output
construction, reuse of completed receiver queries, and an ordered method census.
Each reduced work inside a visit, but the complete inventory still failed to
finish. The matching binary and profiles survive outside managed scratch, as
recorded in the [recovery record](plan/post-release/engineering/tooling-one-off-evidence-recovery-2026-11-06.md#inventory-traversal-decision).
This timeout predates later correctness repairs. There is no fresh actual-root
result establishing the current evaluator's completion or correctness.

The CPU profile places most sampled work in the descriptor traversal. It does
not count equivalent visits or prove that avoidable duplication dominates the
actual repository. The model under review is therefore whether making each
visit cheaper is sufficient when the number of visits can multiply.

In `externalreflectiontypes.go` (deleted with the proof; read it in git history),
`externalReflectionWalk.collectNamed` keeps a list of named types on the active
recursive path. This prevents a cycle from continuing indefinitely. It removes
the entry when the call returns, so a later sibling can traverse the completed
subgraph again. The output map retains one entry per method, while
`collectMethods` still traverses its signature on each matching visit.

```mermaid
flowchart TD
    Root --> B
    Root --> C
    B --> D
    C --> D
```

After the traversal returns from B, it reaches D again through C. Repeating this
shape across levels multiplies paths through a small set of type declarations.
The existing depth-128 guard limits each path's length. It does not prevent
shared subgraphs from being visited through many paths.

A diagnostic overlay observed this behavior without changing the evaluator's
answers. All nine cases passed with race instrumentation. At synthetic depth
12, the results were:

| Observation | Single-child chain | Branching diamond |
|-------------|--------------------|-------------------|
| Collection calls | 29 | 24,575 |
| Distinct structural keys | 29 | 41 |
| Method-signature visits | 1 | 4,096 |

Both returned the same `Touch` method, with no external origins or uncertainty.
Recursive, distinct-generic-binding, and diagnostic-storage-overflow controls
also passed. The [recorded observation](plan/post-release/engineering/tooling-one-off-evidence-recovery-2026-11-06.md#inventory-traversal-decision)
contains the exact test command and retained log identity.

Those structural keys omit generic substitutions, lexical bindings, and active
ancestors. They demonstrate repeated traversal in the synthetic graph, without
proving that those visits are interchangeable. The counts support no speedup
promise. The next measurement must establish how often the current actual-root
analysis repeats contributions that can safely be skipped.

A global “already visited this type” set can lose facts. A different incoming
depth changes whether the depth refusal occurs, and a generic type can expose
different capabilities for different arguments. Traversal also depends on which
methods are visible and whether the starting point is a value or a type
descriptor. Active ancestors matter even without generics:
if D also refers to B, the traversal through B prunes that return edge, while
the traversal through C can enter B. A completed visit under one ancestry does
not establish what another ancestry contributes.

Route A remembers completed contributions within one query and its same output
union. That union only accumulates methods, external origins, and uncertainty.
If an equivalent contribution is already represented there, repeating it adds
nothing. The proposal stores proof of coverage rather than cached answer maps
or borrowed lexical scopes, and every new query retains its own fresh output.

Reuse requires exact incoming depth, substitutions, and traversal flags. It also
requires proof that the old and new ancestor paths prune equivalent work, or
that the contribution is independent of both paths. That proof must include
targets pruned during the original visit. A completed visit alone is
insufficient. Whenever proof is unavailable or the coverage table is full, the
original traversal runs. Route A is the recommended incremental choice only if
measurement shows that its provable subset covers the repeated actual work.

Route B replaces recursive execution with an explicit graph of analysis states
and ordered continuations that record which operation comes next. States retain
the relevant semantic context, including active ancestors. This larger rewrite
makes pending work and completion explicit, which may make broader reuse easier
to prove. It still needs that proof before merging states. Distinct contexts can
multiply, so an explicit graph carries no automatic polynomial-time guarantee.

Either repair must preserve the Git-only trust contract, exact depth boundary,
refusal order, and independently owned outputs. Proof must cover completion
before reuse, different depths and generic arguments, scope ownership, cycles,
ancestor collisions, and capacity fallback. The actual inventory must complete,
its consumers must accept the result, and tests for unsafe capabilities exposed
through type information must still refuse unsafe inventories. Separate
correctness defects and remaining signed
fixture failures remain owed: faster traversal alone cannot establish a correct,
complete inventory.

Thomas decided on 2026-11-06: measure first, then Route A. Route B is not
pursued. The measurement counts, on the actual repository root, how many visits
repeat a state whose contribution is provably the same (equal frame, incoming
depth, substitutions and flags, under an equivalent active ancestry), beside the
looser count that ignores ancestry. Route A is built only if its provable subset
covers the repeated work found. Neither route is implemented yet, and no count
is a claim about speed.

## Actual-root measurement, 2026-11-06

A diagnostic overlay counted the actual root's traversal for ten minutes without
changing any answer; its synthetic controls compared every observed method union
with an unobserved run. The inventory still did not finish. 29,150 queries
completed with 603,282 named visits in total. One query was still running: it
had made 1,416,890,951 named visits over only 1,774 distinct states.

That query starts at the `Editor` callback parameter of
`internal/component/cli/model_commands_edit.go` and reaches goyang's recursive
`yang.Statement` and `yang.Node` syntax tree, which it walks again on every
path that reaches it.

| Rule for reusing a completed visit | Repeats it proves skippable in that query |
|------------------------------------|-------------------------------------------|
| Same state and identical ancestry, or nothing pruned below the earlier visit | 13,761,877 of 326,489,532 (4 %) |
| Same state, and every frame the earlier visit pruned against is also on the current path | 326,465,059 of 326,489,532 (99.99 %) |

The second rule is sound because the later visit then prunes at least the edges
the earlier one did, while every frame active inside the subtree is the same, so
its contribution is contained in what the shared output union already holds.
The 24,473 repeats it does not prove are mostly ones whose earlier record the
measurement's size caps dropped. Route A is therefore built on the second rule,
with the full traversal kept as the fallback whenever the proof is unavailable.

## Depth decision, 2026-11-07

Once Route A, the recursive-type fixpoint and cross-query summaries let the
inventory reach protobuf's reflection interfaces, the exact depth boundary
became the limit. The rule "refuse if any path through the type graph exceeds
depth 128" makes the walk enumerate paths through a large strongly connected
interface graph: a bounded 240-second run still made 10.7 million expansions
over about 37,000 distinct states and did not finish.

Thomas decided that the inventory treats the type graph by reachability. Each
exact type (definition, exact arguments, flags) is walked once per analysis and
a cycle is a cycle, so a finite graph costs one walk. The depth bound remains
only for a generic instantiation that grows without end, such as
`grow[[]T]`, which is the case the guard exists to stop. This replaces the
earlier constraint that every repair preserve the exact depth boundary. A
refusal caused only by a long path through a finite cycle no longer occurs, and
the site of a first refusal can move; the reasons that remain are unchanged.
