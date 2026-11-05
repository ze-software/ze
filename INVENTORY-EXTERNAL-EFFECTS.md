# Why inventory needed a library-source decision

Recorded on November 5, 2026, for the
[tooling one-off defects spec](plan/post-release/engineering/spec-tooling-one-off-defects.md).

The inventory analyzer reached a method whose implementation was outside its
approved input: one committed Git snapshot of this repository. That limitation
led me to propose retaining a version-pinned copy of the Go standard library.
Thomas instead chose to keep Git-only inputs and approved an explicit trust
contract for analyzing external effects.

The decision is approved. Implementation and validation of the new external-effect
analysis are still in progress. This note does not claim a complete inventory run.

## Root cause

Inventory must identify plugin and YANG registrations reached during package
initialization. It must refuse when an unresolved call could hide a registration
or change its metadata. Otherwise it could publish an incomplete report as complete.

The actual repository run reached `brewPrefixes` in
[internal/appliance/homebrew.go](internal/appliance/homebrew.go):

```go
if info, err := os.Stat(p); err == nil && info.IsDir() {
    prefixes = append(prefixes, p)
}
```

The initialization chain runs through `ResolveE2FSTool`, `e2fsSearchDirs`,
`brewKegDirs`, and `brewPrefixes`. The analyzer had support for retained value
and pointer method owners. Here, however, `os.Stat` returns two values, and the
receiver of `IsDir` comes from a library outside the retained repository source.
The analyzer could not establish that receiver's implementation and registration
effects from the available source.

This was an analysis boundary, not evidence of a defect in `os.Stat` or `IsDir`.
The retained run `job-inventory-root-after-pointer-0ff56994.log` refused at this
call. Treating familiar library names as harmless would have concealed the gap.

## Why I asked about library sources

One possible solution was to retain exact standard-library sources, selected by
a committed manifest with version, patch, and content identities. The analyzer
could then inspect those bodies and trace returned owners. A local library copy
alone would not establish that identity or prove the effects.

That proposal expanded the agreed Git-only input boundary. Access to the cache
did not authorize changing which sources constituted inventory evidence. I asked
for a design decision because the alternative required a different proof, rather
than merely another filesystem path.

## The change Thomas approved

The selected design keeps repository source as the only inspected input. It asks
whether external execution can reach a registration or alter registration metadata,
without claiming to know the library's implementation or return values.

The design has three parts:

1. Prove an import path from each registration sink to the external package.
   In a valid Go program, an ordinary static import back to a sink would create
   an import cycle. This proof comes from source, not a library-name whitelist.
2. Track all other ways external code can obtain registration capabilities:
   callbacks, returned interfaces, captured objects, shared writable values, and
   accessible package globals. A neutral package can expose a callback even when
   no call explicitly passes it to the library.
3. Revisit earlier observations until the effect analysis stabilizes. Refuse the
   whole report if a possible callback or mutation makes membership or metadata
   uncertain. Never publish a partial population as complete.

Thomas approved the required bounded trust contract: external code cannot fabricate
references to retained registration functions or memory outside those access paths.
Unsafe pointer fabrication, `go:linkname`, runtime symbol tricks, dynamic loading,
and arbitrary native capability fabrication are outside that contract. Reflection
over reachable values remains part of the conservative analysis.

This assumption is explicit trust in uninspected external code. It is not a theorem
about unrestricted Go, and a repository digest does not prove library compliance.
The analysis concerns registration effects, not filesystem safety, termination,
or the actual result of `IsDir`. Both branches remain subject to analysis.

## Implementation and evidence status

The retained-source inventory foundation was committed as `5a6bd238c1`. Its value
and pointer ownership tests passed, but the actual-root run still refused at the
external receiver described above.

At this record's creation, the new contract documentation and adversarial fixtures
are being prepared. The external-effect evaluator has not yet been implemented or
proven. Tests must cover both registration sinks, multiple results, interface
methods, escaped callbacks, neutral globals, mutation, and stale cached summaries.
All tests and builds run in the main session, as Thomas requested.

The approved design must still pass those controls and a fresh actual-root run.
Approval does not guarantee that every remaining initializer can be certified.
The owning spec stays open until its required implementation and proofs succeed.

The detailed decision and reviewed design are retained in the session artifacts:

- [Original source-boundary proposal](tmp/session/2026-11-05-001121/scratch/inventory-source-boundary-decision.md)
- [Git-only external-effect design](tmp/session/2026-11-05-001121/scratch/inventory-git-only-external-effects-design.md)

Those scratch documents preserve the proposals as reviewed. Their requests for
approval are historical: Thomas subsequently selected Git-only inputs and accepted
the explicit bounded trust contract recorded here.
