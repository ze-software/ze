# 23 -- fix CI testing: work complete, blocked at the review gate

## Rationale

- The twelve unexplained CI reds (learned 1275 left them) are all diagnosed and
  FIXED, each with a different root cause. Three more reds nobody had listed were
  found and fixed too, including the entire 87-test web suite.
- Every fix is verified by RUNNING it, not by inspection: full suites on macOS,
  the Linux-only set in QEMU, and the load-sensitive pair under `stress-repro`.
- The work is uncommitted for one reason only: `commit_helper.py`'s review gate
  requires an INDEPENDENT critical review (reviewer subagents over the diff), and
  the session that did the work was instructed not to spawn agents unasked. Its
  own inline reasoning does not satisfy `ai/rules/critical-review.md`, and
  `--review-override` is labelled an owner override.
- `git push` is absolutely forbidden, so this session could not make CI itself
  run, nor re-run `qemu-nightly` against the fixes. That part is necessarily the
  owner's.

## State

Working tree carries 40 changed/new files, all from this work; a concurrent
session's BMP work was committed separately and is not entangled.

| Verification | Result |
|--------------|--------|
| `make ze-plugin-test` (macOS) | 495/495 |
| `make ze-ui-test` (macOS) | 157/157 |
| `make ze-web-test` (macOS) | 87/87 |
| the eight Linux-only tests in QEMU, selected BY NAME (`--pattern`) | 8/8 (+ `bgp-gtsm-reject`) |
| `stress-repro.py "bgp plugin" --test "37 145"` | 0 reproductions in 60 (was 2/60) |
| unit tests for every changed package | green |

Known red and NOT from this work (proven by removing every change and
re-running -- they fail identically):

- 4 tests in `internal/component/doctor`: `TestCheckListeners_PortInUse`,
  `TestCheckListeners_API`, `TestCollectSchemaListeners_SSHDefault`,
  `TestCollectSchemaListeners_SSHExplicit`. They fail on macOS AND Linux.
- 13 lint issues, all in `internal/component/bgp/plugins/bmp/`, a package this
  work does not touch.
- `verify-status.sh` therefore reads red from a run at 10:13Z made by another
  session.

## Remaining steps

1. **Independent review** (the gate's requirement). Either:

   - authorise reviewer subagents over the diff, fix findings, loop to zero, then

     ```
     python3 scripts/dev/review_gate.py record --spec fixit-ci-red-classes \
       --verdict clean --files <the code files>
     ```

   - or take the owner override by adding to the `create` call below:

     ```
     --review-override "<reason>"
     ```

2. **Prepare and run the commit.** The exact `commit_helper.py create` invocation
   is in the session transcript; it lists all 40 files explicitly and carries
   this `--unverified` reason (the red is attributed, per
   `ai/rules/git-safety.md` "Known-Red Full Verify: Scope to Changed"):

   > verify record is red from a concurrent session's run at 10:13Z, not this
   > work: the 4 doctor listener unit failures reproduce identically with every
   > change here removed, and all 13 lint issues are in
   > internal/component/bgp/plugins/bmp which this commit does not touch.

   Then `bash tmp/commit-<SESSION>.sh`.

3. **Push (owner only)**, then watch `verify`. That is the first run in which CI
   can go green; it has never passed before.

4. **After the push, re-run `qemu-nightly`** (`workflow_dispatch`). Two things it
   still has to prove:
   - the inner budget raise 3600s -> 5400s holds (the first run measured 3269s,
     91% of the old cap);
   - the 300s boot budget under TCG remains UNEXERCISED, because KVM was
     available on the first run. It stays as insurance, and
     `.github/workflows/qemu-nightly.yml` says so rather than implying it is
     proven.

## Two defects found but deliberately NOT in this commit

Both are in developer tooling, not CI, and adding unreviewed changes would
invalidate the review this commit is gated on. File them as follow-ups.

1. **`make ze-qemu-debug` symlinks a binary it never builds.**
   `mk/test-integration.mk` builds only `$(ZE_QEMU_BIN)` and
   `$(ZE_QEMU_TEST_BIN)` under `ifneq ($(NOBUILD),1)`, then symlinks all THREE
   names including `ze-stripped` into the VM's PATH shim. The `ze-stripped`
   link therefore dangles, so any suite that execs it (the `ui` suite does --
   `ZE_TEST_DEPS_STRIPPED`) cannot be debugged with this target. Unverified
   whether it explains anything else; it is simply wrong.

2. **Verifying `.ci` tests by numeric id is unsafe across sessions.** A
   concurrent session added `.ci` files mid-session and renumbered the plugin
   suite: id 373 moved from `resolve-ping` to `remove-private-as-replace-peer`
   and 522 from `vpp-doctor-hugepages` to `trafficusage-external-refuses`, so an
   id-based verification script silently ran the wrong tests. Use
   `ze-test bgp plugin -p <name>`. `tmp/qemu-verify-byname.sh` is the corrected
   form.

## Owner decision already taken

`checkBGPPeerConfig` (`internal/component/doctor/checks_config.go`) reports at
**ERROR**: `ready=false` and `ze doctor` exits 1 for a config the engine refuses.
Decided 2026-07-27. Three shipped `test/ui` fixtures turned out to carry
engine-invalid configs while asserting exit 0 (`doctor-bgp-listen`,
`doctor-bgp-md5`, `doctor-config-reference`); all three were the defect and all
three are fixed rather than accommodated.

Delete this file in the commit that completes its last item.
