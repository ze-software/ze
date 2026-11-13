# RFC new-carrier move: independent source review

November 13, 2026. Source CLEAR for the bounded frozen delta; no concrete new
blocker. This is source preparation, not a formal Review Gate or closure.

Read `internal/le/rfc/rename.go` scoped changes and reached plan/admission/apply
producers, the whole new `rename_new_carrier_test.go`, carrier classification
and tag-stem resolution, and only the own Moving-a-tagged-test-file paragraphs
in `docs/contributing/rfc-conformance-gates.md`. Applied runtime/security,
tests/wiring/edges and style/docs lenses. Development tooling/system door
contracts retained/read; no product registration or shipped linkage changes.

Frozen SHA-256 identities:

- rename.go: 792260e2121d40e495d6a9eabd0925f18d77f8c45bd8484889cb45b74e6999d1
- rename_new_carrier_test.go: 0e7a987da4331740cc459509c67e0ffaea209c2613d2dcc241230808740cd178

The new admission path requires a unit carrier and a recognized RFC tag stem.
A failed rev-parse is insufficient: successful ls-tree must establish HEAD
absence. Exact NUL-delimited ls-files --others --exclude-standard output must
identify this source, excluding staged and ignored files; Lstat then requires
the source entry be regular. Errors do not silently grant admission. The
tracked source path continues comparing its content hash with HEAD, preventing
an edited/recreated HEAD source from taking the new-carrier exception. Existing
target existence, directory, clean path, suffix/platform, carrier and RFC naming
checks remain shared across move/propose/repair admission.

Planning snapshots every source's bytes. Applying first compares sources,
evidence and cited files against their snapshots, refusing before its own
writes if any differs. Existing non-overwriting hard links, partial-write
reporting and path-only evidence rewrites remain intact. This is optimistic
checking rather than filesystem-wide writer exclusion: mutation during
collection/admission or after the pre-apply comparison is not atomically
excluded. The guide explicitly states the concurrent-filesystem limitation,
so this review does not claim arbitrary-writer safety. The new-carrier change
does not grant permission to bypass tracked-file commit-owned relocation.

The six new parent tests preserve a useful successful move and exact file
body/tag identity; verify moved discrimination/audit fingerprints with actual
verifiers; retain target/path/platform/naming/batch refusals; reject an edited
recreated HEAD source; reject staged, ignored, symlink and no-HEAD sources; and
refuse a body change after planning without touching proof keys or destination.
The fixture builds a real committed repository baseline, then adds the new
carrier outside HEAD/index. Existing tracked-source tests are unchanged.

Read ROOT's `rfc-new-carrier-move-red.log`: old admission rejects the useful
new-carrier move/evidence/planning tests because the file is not tracked at
HEAD. Read actual `job-rfc-new-carrier-move-green-coverage-ca7ff413.log`: all
six new parents and selected existing Rename/BuildSuffixMoves/EvidenceStrings
tests pass, with 33.2% scoped statement coverage. Expected refusal diagnostics
and the existing partial-write fixture's permission error are PASS controls,
not unexamined failures. This reviewer executed none of those actions.

ROOT still owns actual native relocation of the real new LL carrier, final
record binding and any subsequent immutable delta admission. Retained 431-path
deployment context remains unchanged. No tests/builds/formatter, tracked edit,
commit, metadata approval, formal CLEAN or closure by this reviewer.
