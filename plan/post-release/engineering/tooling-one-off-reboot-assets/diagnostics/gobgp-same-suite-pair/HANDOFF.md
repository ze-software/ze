# Same-Suite secondary GoBGP diagnostic: private inputs

Revised source-only handoff, November 6, 2026. No formatter, compiler, Go tool,
test, Docker command, script, or runtime probe was executed. Native execution
requires a fresh independent review of the actual four-file delta.

## Rejected draft and diagnosis

The original draft and review are preserved under `rejected-shared-patch/`.
`DIAGNOSIS.md` records the five-part diagnosis before this repair. A final read
followed by replacement rename was not compare-and-swap; it could discard an
unknown concurrent edit. This revision removes shared-patch selection and
restoration entirely. It never opens the project's canonical patch or any
first-party source for writing, even during error cleanup or process timeout.

## Files changed in this package

`overlay.json` selects these existing files in the main checkout:

| Canonical file under `internal/le/interoplab/bgp/` | Private after file | Change |
|---|---|---|
| `selected_images_test.go` | `after/selected_images_test.go.txt` | Original root preserved; additive same-Suite caller and private-input helpers |
| `check_gobgp_reload.go` | `after/check_gobgp_reload.go.txt` | Default wrapper and selected-input helper share the original assertion body |
| `check_gobgp_reload_patch.go` | `after/check_gobgp_reload_patch.go.txt` | Immutable expected-patch input; original runtime-byte/SHA algorithm retained |
| `check_gobgp_reload_patch_test.go` | `after/check_gobgp_reload_patch_test.go.txt` | Original tests preserved; three additive selected-input roots |

`before/` contains exact canonical snapshots. No canonical source, patch,
Dockerfile, scenario or existing test body was edited. MAIN computes source
hashes after formatting and checks every before snapshot against its live file.

## Input binding and preserved proof

Each expected-patch value copies its constructor input into an immutable
string. Empty input refuses construction. A deliberately uninitialized explicit
value still refuses in the native provenance path; it cannot become a canonical
default. The private checker helper accepts a value and gives its own copy to
the existing proof object.

The registered `checkGoBGPExportReload` default still chooses the canonical
source. Its expected-file read remains inside identification, after image and
version observations and before runtime patch/config reads. The default's
missing-checkout, unreadable-file and empty-file refusal order is unchanged.
The selected input joins at that same point. Both paths execute the original
exact runtime bytes, named runtime path, digest shape/hex and computed SHA256
checks. No version-only identification or provenance bypass is introduced.

The selected helper runs the original common assertion body. All initial,
reload, repeated-read, route, source attribute, session, daemon, capture and
wire checks remain unchanged. Diagnostic plans use the original
`specialChecker` error/log wrapper around that selected-input helper; they do
not substitute another route or wire oracle.

The additive root creates a fresh private directory, with two contexts:

- `gobgp-cut-input/`: byte-exact canonical `Dockerfile.gobgp` and reviewed cut patch.
- `gobgp-full-input/`: the same exact Dockerfile and reviewed full patch.

Each context contains only those two explicit third-party build inputs. Files
are created with exclusive-create, mode 0400, then the directory becomes mode
0500. The Dockerfile's existing relative COPY name is preserved. No first-party
tree, export, alternate checkout or copied Go development tree is created.
Contexts are checked for exact contents before each plan and after the suite.

One actual `Suite.Run` and one Docker owner retain the Ze/BIRD/FRR image
references across both plans. Only the two GoBGP logical image declarations
have different private contexts. Both keep the native build settings and cache
tag. The full plan builds only its new GoBGP logical declaration. Actual
consumed Ze/BIRD/FRR/sink IDs must match across both retained proofs; the two
GoBGP images and observed producer executables must differ.

Both scenario cleanup lists and Suite image-retention cleanup must be empty.
The first result must be the intended simultaneous stale-prefix semantic error,
and the second must be a clean native pass. Setup/build failure, timeout alone,
skip, no execution, another wrong route, changed source/session fence or changed
nonproducer image fails the outer root. The original Suite's one-fail/one-pass
Code 1 is retained in its private report.

The negative proof reconstructs one complete initial attempt and one complete
simultaneous attempt. Existing parsers and comparison helpers check source
attributes, capability multisets, counters, generations and daemon identities.
Every receiver route uses the original expected/strict route contract, except
that the one intended stale secondary prefix 6 must retain its exact prior
useful path. The independent sink must satisfy absence. Actual sent bookkeeping
must describe only that extra survivor. Both PCAPs pass through the existing
complete-capture and wire checks again.

Fresh proof-directory observations bracket the serial plans in this session.
Exactly one new completed proof is required at each boundary; ambiguity refuses.
Raw proof maps and suite reports remain private. Only the small safe report
may be printed.

## Main-thread review and source checks owed

Obtain a fresh independent actual-delta review before formatting or execution.
The previous refusal remains recorded as history; it approves nothing here.

MAIN formats only the four `after/*.txt` Go sources through native admission,
then computes/retains hashes for every before/after source, overlay, diagnosis,
reviewed patch input, canonical Dockerfile and frozen-control manifest. Verify
all original test bodies are unchanged. The original default provenance tests
must run beside the new selected-input tests.

The canonical patch is read-only to this diagnostic. It must initially equal
the reviewed full SHA
`35eed21c951868c3846465c8bbf4fc5c507ffedd93f9c35b4c4e06d03a09b047`.
The cut input must equal
`1b7b90213a0875f9a1ef8e8e5cfadb5537bde56a641174651cbb5050871bcc67`.
The root verifies all frozen controls and additionally checks the canonical
full patch. A foreign change fails a read-only identity check and is preserved.
No same-filesystem or source-restoration operation is needed.

## MAIN execution owed

All commands below are future instructions, not results. Run serially after
the current MAIN CPU job finishes, with the established all-feature tags and
`integration docker`. The explicit selectors prevent the native pair from
running during the pure source-input cohort.

First run the unchanged default and new selected-input source controls:

```sh
CLAUDE_CODE_SESSION_ID=01a1a907-3278-7971-98dc-dbc41d4a54a5 CGO_ENABLED=1 \
  bin/le-tooling-resume-01a1a907/le job run \
  label gobgp-private-patch-input-controls-20261106 quiet command \
  go test -race -tags "$tooling_test_tags" -p=1 \
  -overlay /home/thomas/Code/github.com/ze-software/ze/main/tmp/session/2026-11-06-080923/scratch/gobgp-same-suite-pair/overlay.json \
  -run '^TestGoBGPReload(RetainsAppliedPatch|RefusesUnknownAppliedPatch|SelectedPatchInput.*)$' \
  -count=1 -timeout=3m -v ./internal/le/interoplab/bgp
```

Then run only the new native root with NO_BUILD unset:

```sh
env -u NO_BUILD \
  CLAUDE_CODE_SESSION_ID=01a1a907-3278-7971-98dc-dbc41d4a54a5 CGO_ENABLED=1 \
  bin/le-tooling-resume-01a1a907/le job run \
  label gobgp-secondary-same-suite-private-inputs-20261106 quiet command \
  go test -race -tags "$tooling_test_tags" -p=1 \
  -overlay /home/thomas/Code/github.com/ze-software/ze/main/tmp/session/2026-11-06-080923/scratch/gobgp-same-suite-pair/overlay.json \
  -run '^TestNativeGoBGPSecondarySameSuitePair$' -count=1 -timeout=25m -v \
  ./internal/le/interoplab/bgp
```

`stageBinaries` uses `gotoolchain.Environment` with CGO false by default.
Its overrides append CGO_ENABLED=0 after inherited environment. Thus outer
test race support/CGO 1 does not reach the first-party native image binaries.
The exact canonical GoBGP Dockerfile explicitly uses CGO 0 for its builds.
This caller adds no independent Go build subprocess or worker.

The existing per-build, readiness, reload, capture and cleanup bounds remain
unchanged. The outer root requires an explicit timeout. File reads are bounded:
1 MiB for patch/manifest input, 16 MiB per frozen source/context input, 512 MiB
for raw proof JSON, and the declared 4,096-packet capture maximum at the native
snapshot length. No loop or worker is added.

## Results and retention owed

No execution result or accepted pair is claimed by this source-only handoff.
MAIN reads each exact root's PASS/FAIL and skip lines. The first inner scenario
is expected semantic RED; the outer root passes only if all its guards and the
second native PASS succeed. A successful `pair-safe.json` is an oracle report,
not a replacement for the final Go test verdict or retained raw outcomes.

The root logs its private `gobgp-same-suite-pair-run-*` directory and retains
`suite-report.private.json`, the frozen-control manifest, both private
contexts, and, on successful oracles, `pair-safe.json`. The original native
proof directories remain under this session's
`bgp-export-reload-secondary-gobgp-*` paths. MAIN immediately archives both
actual proof directories/PCAPs, reports, source identities and job log outside
managed scratch. Never print the full raw proof or private suite report.

This proves only the bounded stable-session non-ADD-PATH secondary producer
case if MAIN observes the result. Historical four-peer evidence, fifteen
checker obligations, the original cleanup stall and whole Phase 9 remain open.
Production documentation is unchanged because only private diagnostic overlays
were authored; no production behavior or canonical input changed.
