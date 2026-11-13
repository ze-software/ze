# One source cut at a time

These are main-thread execution instructions, not commands run by this author. Use the existing main checkout only. Keep each cut active until every carrier assigned to it has an inspected semantic RED, then restore before another cut. A build/setup failure is not discrimination. Do not run another source-mutating owner on the same file during this interval.

For the first cut, set these literal values from carrier-commands.md; each later cut gets its own fresh attempt directory and current baseline. Do not reuse a predecessor's source copy. If the expected source digest differs, stop this cut and re-read its current producer; never overwrite the file with a stale baseline.

```bash
set -e
rp5_plan=tmp/session/2026-11-12-205428/scratch/rp5-proof-execution-plan
rp5_cut=01-central
rp5_source=internal/core/rib/policy/policy.go
rp5_expected=c0d8e2514799c3b2bb25fe4e4155e7652a8599c25c24b791e7691380c95c8176
rp5_patch="$rp5_plan/current-v2/01-central.current.patch"
rp5_attempt_rel="$(mktemp -d "$rp5_plan/execution-v3/actual-$rp5_cut-XXXXXXXX")"
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

Inspect mutation.diff against exactly the selected proposal. Every cut, including supplemental 25/26, MUST carry its literal `// MUTATION-APPLIED` comment in the mutated source. The supplemental cuts add only that adjacent marker and the specified semantic line change. No formatter runs on a mutated file: it would add unrelated changes and invalidate the retained mutation digest.

Build RED guest binaries through the existing builder. The named host build also makes a fresh guest le from current source when the harness starts, including fixture/server/doctor cuts. Read each quiet job log and retain its path, not only the exit code.

```bash
CGO_ENABLED=0 ./le --name session job run label "rp5-$rp5_cut-build-red" quiet command ./le --name session test qemu guest-binaries
```

Use the exact GuestBinaryEnvironment returned by that build. Current native-amd64 source expands it as shown in carrier-commands.md. Run each selected CI separately with its unique save path; inspect that carrier before going on. A single boot may hold multiple already-approved carrier commands, but an expected RED must not silently skip later commands or count a later unexecuted carrier. Separate boots are the directly runnable default here. The own-system action verifies the kernel against its runtime image.

After RED, restore only this attempt's saved current bytes, and only if source is still exactly the cut digest. Any mismatch means a peer changed it: retain all files and reconcile instead of overwriting. Execute restoration even after a refused build or unexpected test result; do not start a new cut while this one remains active.

```bash
sha256sum --check "$rp5_attempt/backup.sha256"
sha256sum --check "$rp5_attempt/source.mutated.sha256"
cp --preserve=mode,timestamps -- "$rp5_attempt/source.before" "$rp5_source"
cmp -- "$rp5_attempt/source.before" "$rp5_source"
sha256sum --check "$rp5_attempt/source.before.sha256"
CGO_ENABLED=0 ./le --name session job run label "rp5-$rp5_cut-build-restored" quiet command ./le --name session test qemu guest-binaries
```

Rerun every same exact carrier with the restored binaries and fresh GREEN save path. Require its actual named PASS, one executed selected test, zero skips, positive controls, all semantic assertions and joined cleanup. No git restoration/index command appears in this cycle. Preserve every attempt directory and log; a failed attempt is evidence, not a reason to replace it with an empty success record.
