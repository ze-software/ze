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

1. Prove which retained packages external code can import. Paths from both
   registration sinks to the external package exclude a static return path by
   the Go import-cycle rule. A second source certificate uses a closed module
   namespace and Go's internal-package rule: outside code must first enter
   through a retained public package. Both routes require analysis of the
   remaining accessible code and initialization; neither exempts a library name.
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

Evidence availability changed on November 6. The old session's raw logs,
profiles, omission overlays and reviewed scratch proposals linked below became
unavailable during an automatic cleanup run. Its log does not identify every
removed directory. Earlier results remain recorded observations, while their original
bytes are unavailable. The
[recovery record](plan/post-release/engineering/tooling-one-off-evidence-recovery-2026-11-06.md)
identifies surviving source snapshots and the private archive for new retained
proof. The approved Git-only decision and trust contract remain recorded here.

The retained-source inventory foundation was committed as `5a6bd238c1`. Its value
and pointer ownership tests passed, but the actual-root run still refused at the
external receiver described above.

An external-effect evaluator is now present in the working tree. It tracks
external result ownership, callable and writable capabilities, accessible package
state, and their fixed point over retained source. Parameter provenance includes
dormant retained callers and distinguishes a proved nil or inert argument from
missing argument evidence. Recursive forwarding needs a grounded source for every
dependent obligation; unresolved indirect callers refuse. The new forwarding and
unbound-parameter regressions now pass with their ordinary inventory controls.

The escaped registration-function, exported-constant method and nested-module
identity regressions pass, as do the retained constructor and receiver controls.
The public-entry certificate and its initialization, private-reference and
new-origin regressions now pass. Targeted tests also pass for positional fields,
unknown callable unions, range execution and assignments, and exact field owners.
The per-reader mutation index and completed-query cache preserve declaration
identity and mutation policy in their targeted tests.

The actual repository run has not produced a report. The last observed
pre-terminal run of `TestAnswerAnswersTheInventory` reached its ten-minute limit.
After the allocation repair, the native `repo inventory` command also ended at its
fifteen-minute diagnostic deadline with exit 124 and no report. That attempt
predates the terminal-refusal guards. Three later actual-root attempts timed out
at the unchanged ten-minute bound under normal `CGO_ENABLED=0` execution. The
latest attempt includes the completed receiver-query repair as well as the
reflected-signature, package-binding and descriptor allocation repairs. Thirty-five
affected source roots pass. Separate fallback tests also preserve raw resolution
for synthetic or unindexed receiver sources, with every block in the new wrapper
executed. The latest full-run profile still points to repeated method collection
and receiver-table lookups. A method-census repair is under source-only design,
with no implementation released yet.
The [evidence record](plan/post-release/engineering/tooling-one-off-evidence.md#git-only-evaluator-continuation-on-2026-11-06)
retains the exact command and validation scope.

The allocation repair passes eleven source-selected controls for value merging,
flattening, lexical and type identity, and received-field ownership. The terminal
guards now stop further expansion once a nonempty unknown in the monotone ambient
pool guarantees refusal. They preserve the current preferred issue and admit no
operations on that path. Independent source review found the change clean, and
all sixteen selected roots pass with race instrumentation and full feature tags:
the earlier eleven controls and all five `TestExternalTerminalUnknown*` roots.
Those five include the two valid survivors and the three restored refusal cases.
The tests are in `internal/le/repo/inventory/`; the evidence record gives their
exact selection command. This result does not establish a complete inventory.

The exposed-signature repair separates reflected type authority from executable
callable ownership. MAIN restored all five paired scenarios in
`TestExternalSignatureTypeCapabilitiesSolver` and all thirty-two cases in
`TestExternalSignatureAuthoritySourceSolver`: the twenty-two previously unsafe
cases now refuse, and twenty controls retain their expected outcomes. Six
reflection-domain roots passed, followed by the corrected generic nil-interface
precision cases. The earlier sixteen source controls also passed after the
signature repair. Independent review found the revised implementation clean
within that scope.

A return-value baseline found seven unsafe admissions. A retained call can
return a typed nil function or pointer, or a named scalar constant, whose
declared result type carries reflected methods. Those failures are in
`TestExternalSignatureReturnedValueAuthoritySolver`. Its controls and the
companion `TestExternalSignatureReturnedNilCallableKeepsOwnership` retain fifteen
expected outcomes.
The repair now carries declared Results type authority on actual retained call
results after the original ownership and assembly checks. All seven unsafe cases
are restored, and independent review found the actual change clean within that
scope. The full affected source selection passes all twenty-six roots: four
signature roots with twenty-nine restored unsafe cases and thirty-five controls,
six reflection-domain roots, and the sixteen prior source controls. Existing
tests were unchanged. The source-reviewed closure and assembly-sibling tests now
pass. Their old-hook overlay baseline preserved both scalar controls and exposed
both unsafe returned-value cases. The unchanged live tests now refuse both unsafe
cases. The assembly sibling proves composition with the retained source
certificate, without executing a native assembly call.

The package-binding index and descriptor collector passed all thirty-two
affected source roots, including those two siblings and four new cost and
compatibility roots. The collector gives each query one fresh output instead of
copying intermediate method maps through each descriptor ancestor. Its allocation
test measures three allocations for both shallow and nested descriptors, after
reproducing nine and thirty-one before the repair. Exact binding identity,
ambiguity, independent outputs and the existing depth boundary remain covered.
These source tests do not establish a complete inventory report.

The analyzer also lost uncertainty when a channel receive replaced a struct
with an earlier inert callback field.
The direct solver regression reproduced that defect and passes after the repair.
After Thomas unlocked signing, its committed fixtures passed for both plugin and
YANG registrations. The signed public-entry capability, certain-initializer and
both-sink-witness roots also pass. Separate omission runs expose unsafe report
publication when either accessible exports or possible initializers are removed
from the analysis. Restoration after those omissions is still in progress.

The actual-root proof, remaining discrimination tests, full signed-fixture
regression, measured coverage, final review and publication remain owed. All
tests and builds run in the main session, as Thomas requested.
Approval does not guarantee that every initializer can be certified, and the
owning spec remains open.

The detailed decision and reviewed design are retained in the session artifacts:

- [Original source-boundary proposal](tmp/session/2026-11-05-001121/scratch/inventory-source-boundary-decision.md)
- [Git-only external-effect design](tmp/session/2026-11-05-001121/scratch/inventory-git-only-external-effects-design.md)

Those scratch documents preserve the proposals as reviewed. Their requests for
approval are historical: Thomas subsequently selected Git-only inputs and accepted
the explicit bounded trust contract recorded here.
