# Supplement 27: latest producer originals on an equal effective offer

UNAPPLIED proposal. No source edit, compiler or test execution happened in this
preparation. The patch changes only the call to `g.upsert(stored)`: unchanged
effective offers skip it. The remaining insertion pipeline, including locks,
store update, best computation, revision comparison and unlock, stays intact.
The adjacent source marker identifies the active mutation when main applies it.

Source: `internal/core/rib/locrib/manager.go`.
Current SHA256: `f5bd2a647684165ce563de9a2a7803f0ecd07a81d8f54f9a61138b5e7a553c08`.
Patch: `27-unchanged-original-values.patch` in this directory; execution-v4
also contains the same proposal with current metadata.

Target: `TestLocRIBUnchangedEffectivePathKeepsLatestProducerValues`. Its useful
selected explicit route and no-interim-event control precede policy removal.
The mutation leaves the original tag/distance at 1/30 after the second offer,
so clearing policy must fail the assertion expecting the latest originals 2/40.
The other new explicit-group, unchanged-announcement, secondary-notification,
admission and both-reader tests stay in the selection as positive controls.
A build failure, lock hang or loss of the initial route is not this proof.

The five roots actually passed in main's
`job-rp6-complete-explicit-ecmp-unit-green-79665ef0.log`. That observed pass does
not establish that the originals control discriminates. Main still owes the
marked cut, exact semantic RED, guarded restoration and the same selection GREEN.

Before mutation, use a fresh current source copy. This is an execution recipe
for main, not a script run by this author:

```bash
set -e
rp5_plan=tmp/session/2026-11-12-205428/scratch/rp5-proof-execution-plan
rp5_source=internal/core/rib/locrib/manager.go
rp5_expected=f5bd2a647684165ce563de9a2a7803f0ecd07a81d8f54f9a61138b5e7a553c08
rp5_patch="$rp5_plan/execution-v3/27-unchanged-original-values.patch"
rp5_attempt_rel="$(mktemp -d "$rp5_plan/execution-v4/actual-27-XXXXXXXX")"
rp5_attempt="$PWD/$rp5_attempt_rel"
printf '%s  %s\n' "$rp5_expected" "$rp5_source" | sha256sum --check
cp --preserve=mode,timestamps -- "$rp5_source" "$rp5_attempt/source.before"
sha256sum "$rp5_source" > "$rp5_attempt/source.before.sha256"
sha256sum "$rp5_attempt/source.before" > "$rp5_attempt/backup.sha256"
cp -- "$rp5_patch" "$rp5_attempt/cut.patch"
sha256sum "$rp5_attempt/cut.patch" > "$rp5_attempt/cut.sha256"
patch --dry-run --batch --forward --fuzz=0 -p1 < "$rp5_attempt/cut.patch"
sha256sum --check "$rp5_attempt/source.before.sha256"
patch --batch --forward --fuzz=0 -p1 < "$rp5_attempt/cut.patch"
if diff -u --label source.before "$rp5_attempt/source.before" --label "$rp5_source" "$rp5_source" > "$rp5_attempt/mutation.diff"; then
  printf '%s\n' 'REFUSED: cut produced an empty diff' >&2
  exit 1
else
  rp5_diff_status=$?
  test "$rp5_diff_status" -eq 1
fi
sha256sum "$rp5_source" > "$rp5_attempt/source.mutated.sha256"
sha256sum "$rp5_attempt/mutation.diff" > "$rp5_attempt/mutation.sha256"
cat "$rp5_attempt/mutation.diff"
rg -n '// MUTATION-APPLIED' "$rp5_source"
```

Inspect the exact diff before compiling. Do not format mutated source. Run the
same bounded selection for RED and restored GREEN, changing only `rp5_stage`:

```bash
. ./feature-tags
rp5_tags="ze_core,$(feature_tags feature-gates.txt ,)"
rp5_selection='^Test(LocRIB(ForwardingMembersNonwinnerExplicitGroup|NonwinnerExplicitSecondaryChangesNotify|ExplicitMembersRespectAdmissionRankAndDuplicates|UnchangedEffectivePathKeepsLatestProducerValues)|RouteViewNonwinnerExplicitGroupBothReaders)$'
rp5_stage=red
if CGO_ENABLED=1 ./le --name session job run label "rp6-unchanged-original-values-$rp5_stage" quiet command go test -race -p 1 -v -count=1 -tags "$rp5_tags" -run "$rp5_selection" ./internal/core/rib/locrib ./internal/component/sysrib; then
  rp5_unit_exit=0
else
  rp5_unit_exit=$?
fi
printf 'stage=%s actual_exit=%s\n' "$rp5_stage" "$rp5_unit_exit"
```

Read the quiet log: only the originals target should fail at the latest-values
assertion; all other four roots and both reader subcases must pass. Package
source mutation invalidates RED's cache, and `-count=1` also ensures restored
GREEN executes rather than reusing the earlier observed pass.

Restore only the exact bytes saved for this attempt, guarded against another
session's edit, even after an unexpected result:

```bash
sha256sum --check "$rp5_attempt/backup.sha256"
sha256sum --check "$rp5_attempt/source.mutated.sha256"
cp --preserve=mode,timestamps -- "$rp5_attempt/source.before" "$rp5_source"
cmp -- "$rp5_attempt/source.before" "$rp5_source"
sha256sum --check "$rp5_attempt/source.before.sha256"
```

Then repeat the identical unit recipe with `rp5_stage=green`, and require all
five roots plus both reader subcases to pass with no skips. Retain both quiet
logs, exact source/patch/mutation digests and actual outcomes. A guard mismatch
requires reconciliation; never overwrite it with a stale backup. No git
restoration, index operation or commit appears in this recipe.
