# Same-Suite diagnostic preservation failure

Source diagnosis before the repair, November 6, 2026. The independent reviewer
refused native execution of the first draft; that draft has not run.

1. **Symptom.** The draft claimed unknown concurrent patch edits were preserved,
   but its final equality check and replacement rename were separate operations.
   A writer in that interval could lose its new bytes at the canonical path.
   The same race existed in deferred restoration.
2. **Root cause.** `goBGPSameSuitePair.replacePatch` selected a runtime input by
   mutating `test/interop/gobgp-v4.11.0-export-reload.patch`. A read followed by
   `os.Rename` is not compare-and-swap. Atomic replacement protects against a
   partial write, not against overwriting a concurrent owner's edit.
3. **Owning layer.** The private diagnostic caller owns its image inputs. The
   production checker's fixed expected-input path made the draft share a file
   that the proof did not own exclusively. The native image/checker algorithms
   need a narrow per-plan input binding, not a stronger shared-file overwrite.
4. **Two fixes.** **[workaround]** Add more reads, an advisory lock, or an exchange
   and recovery protocol around publication. **[source]** Eliminate canonical
   patch mutation. Bind each case's exact immutable expected bytes to the same
   strict provenance algorithm and to a private two-file GoBGP build context.
5. **Why the source fix.** Other sessions do not promise to honor a new lock,
   and more reads leave another race. Exchange would still move unknown work
   before ownership was established. Private inputs remove that action entirely;
   no public patch selection or restoration remains to race.

## Concrete repair plan

Preserve the refused overlay, handoff and review under `rejected-shared-patch/`
before changing the draft. Keep every canonical file untouched.

Use selected-file Go overlays for the existing checker and provenance files:

- Add one private expected-patch value containing an owned immutable string.
  Its validating constructor rejects empty bytes and copies caller-owned input.
- Factor the existing checker body into a common private runner. The registered
  function retains its canonical default; a private diagnostic helper passes
  an explicit patch value. Image/version observations, the expected-patch load
  point, early refusal order, config reads, reload stimulus, routes, session
  fences, capture and finish logic remain unchanged.
- Factor only the expected-byte acquisition inside `identifyPatch`. Both
  sources feed the original exact runtime-byte comparison, named runtime path,
  digest parsing, and computed SHA256 equality. The default still reads this
  checkout's canonical file at the same point. A zero selected value refuses;
  it must not silently select the canonical input.

The additive root creates two private Docker contexts under its own fresh
directory. Each contains exactly the byte-for-byte canonical `Dockerfile.gobgp`
and its selected small patch under the Dockerfile's existing relative COPY name.
It does not copy first-party source, make an export or create another checkout.
Both GoBGP image declarations keep the native build settings and use those
private contexts. One native Suite still owns the shared Ze/BIRD/FRR images.

Bind each plan's expected input through the private checker helper while
retaining the native special-checker error/log wrapper. Preserve every existing
test body. Add narrow source-input seam cases beside the existing provenance
tests: owned-byte copying, explicit-input binding, zero/empty refusal and strict
runtime-byte/digest refusal before config reads. All execution remains MAIN's.

Remove `selectPatch`, `replacePatch`, `restoreFull`, shared-patch file mode state
and all rename/deferred-restoration code. Keep read-only canonical/full and
frozen-control identity checks. Preserve all previous semantic, wire, same-image,
cleanup and private-report assertions. Unknown external edits may make an
identity check fail; the diagnostic never replaces or restores those edits.

The revised delta requires a fresh independent review before MAIN formats or
runs it. No source-only check is a native or paired-proof verdict.
