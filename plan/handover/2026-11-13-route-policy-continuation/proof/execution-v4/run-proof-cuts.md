# Main-owned execution of all 27 cut pairs

`run-proof-cuts.py` is executable and has only been parsed as source with
`ast.parse`. Its author did not import it, run it, apply a cut, execute a build,
or start a VM. All execution remains owed.

From the shared main checkout, after all source owners are stable and after
checking the final source hashes in `cuts.current.json`, main runs:

```bash
tmp/session/2026-11-12-205428/scratch/rp5-proof-execution-plan/execution-v4/run-proof-cuts.py --session session
```

Do not wrap the entire driver in another heavyweight job slot: it admits each
individual build or native command through `./le --name session job run label
LABEL quiet command ARGV`. Its child commands are synchronous and joined;
it never overlaps source mutation, compiler work, guest work, or restoration.
The script is an execution recipe, not a new project runner or raw tester.

It uses this current own-system kernel, whose identity the native action also
checks against the runtime image:

`/home/thomas/.cache/ze/runtime-kernel/7.3-runtime-amd64-runtime-docker-6727e257-c79de00c/vmlinuz`

The driver refuses if the working directory is not the main checkout, if any
of the 27 producer hash guards is stale, or if an earlier source mutation
marker is already active. It takes a nonblocking lock against a second copy
of this driver. Main must still coordinate other source authors; the lock does
not claim to exclude every editor or unrelated project command.

Each cut receives a fresh directory beneath a unique `actual-all27-*` directory,
with a current source backup, exact proposed patch, exact marked nonempty
mutation diff and their digests. The source must match the recorded current
digest immediately before application, and the applied bytes must match the
proposal's computed digest. Restoration requires both the saved source digest
and the exact mutated source digest; another session's edit is never overwritten.

For cuts 01 through 24, each stage builds the current daemon set with the native
`test qemu guest-binaries` action. The driver parses that build's returned
environment and records the built binary hashes. It then starts one native
`docker-lab kernel ... harness` command, which builds the current guest le and
runs **every exact carrier assigned to that cut** sequentially. Each carrier
uses the existing `test qemu all test PATH save PATH` action. Its actual exit,
complete native runner log and complete saved client capture remain separate.
The native le writes directly into the outer job's existing quiet log. Unique
BEGIN/END markers delimit each carrier; after the VM joins, the host derives
each retained runner segment from that full native log. No project command's
output is redirected to a hand-created runner log inside the guest. The guest
records only each command's explicit exit code in a separate artifact.
An expected RED does not prevent later assigned carriers in that batch from
running. This uses 48 daemon VM stages rather than one VM per carrier; it does
not add a persistent VM control channel or a second test selector.

Each named RED must show the exact selected path, one executed test, no skipped
phase, the real placement marker, its own named FAIL, and a `ZE-OBSERVER-FAIL`
matching the cut-specific source-owned assertion. The explicit `RED` table has
24 different semantic expectations: rejected winners, attribution, explicit
zero presence, reload distances, hidden-cover resolution, dependent withdrawal,
member labels/SID, replay identity, stopped-reader fencing, scoped cleanup,
metric semantics/invalidation, readiness/reconstruction, recovery command,
withholding and exact kernel route action. A native setup/build/permission
failure, an absent capture, a crash, or a different assertion is invalid.
The exact assertion's sequential fixture path establishes which earlier checks
were reached; the evidence records the original per-cut positive-control text.
Main still reads the retained full captures before treating proof as accepted.

After every RED attempt, the driver restores the saved current bytes and builds
the restored stage. Every same carrier must finish with its own named PASS,
the exact executed/skip counts, `ALL PHASES PASSED`, no observer failure, and
all literal `OK:` success/cleanup expectations declared by that carrier.
If RED was invalid, restoration and restored GREEN still run, then the driver
stops before the next cut. A build/setup refusal is never recorded as RED.

Cut 25 runs the ten existing replay roots together under the exact scoped
race/count=1 package command. Only the false/error target may fail under the
cut, and it must show both the forbidden replacement and absent pending
failure. The other nine roots must pass. Restored GREEN runs the identical set.

Cut 26 builds a fresh fixture ELF for each stage. Its native root command mounts
the private executable tmpfs, grants only CAP_KILL to the fixture and runs the
exact test as non-root `ze`, whose actual sudo child becomes root. The driver
requires the stopped-root cancellation assertion under RED and the actual
joined-root confirmation under GREEN, with no root-role or permission skip.
Both ELF hashes and native quiet logs are retained.

Cut 27 runs all five new ECMP roots together, including both reader subcases.
Only the latest-originals target may fail, at its exact restoration assertion;
the other four roots must remain useful. The restored same selection must pass.

Interrupts cancel and join the active native job before guarded restoration.
If a job cannot join cancellation, or if either restoration guard differs,
the script stops and preserves the active source/evidence for main to reconcile.
It never proceeds to another cut while the current source is unresolved.
It uses no git restoration, index, commit or push command.

`jobs.json` retains every exact admitted argv and quiet-log path/digest. The
per-run `outcomes.json` gains a row only after the cut's actual semantic RED and
restored GREEN pass automated checks. Each row retains
`human_full_capture_review_owed: true`. The existing preparation
`execution-v4/outcomes.json` stays empty and never becomes execution evidence.

The driver does not promote drafts, rerun stress, run interoperability, or close
the spec. Those remain separate acceptance obligations. If a later source fix
changes one of the producer hashes, refresh the proposal against that source
before main starts; do not remove a hash guard to make the driver proceed.
