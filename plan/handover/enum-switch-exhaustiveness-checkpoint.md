# Enum switch exhaustiveness checkpoint

Implementation is paused at the owner's request so it can continue on another Linux machine. The enum conversion and later defect repairs are implemented, but acceptance remains open. This document records the continuation boundary; it does not certify a clean tree, completed spec, or publication.

- Spec: `plan/spec-enum-switch-exhaustiveness.md`, in-progress, Phase 4/11.
- Branch at capture: `main`.
- Implementation checkpoint: `d9844ae724` (1,080 files). The commit succeeded. Its shared-index synchronization initially hit a stale lock; the owner authorized guarded removal, and replaying only the generated post-commit synchronization succeeded. Do not repeat the commit.
- Inventory: `plan/enum-switch-exhaustiveness-inventory.json`.
- The owner authorized pushing this checkpoint. The native push attempt refused publication because 150 verification-debt rows were open. The checkpoint has not been published; a normal pull from the existing remote cannot retrieve it yet. Do not bypass the gate in `internal/le/commit/prepare.go`.
- The owner chose to checkpoint all current checkout changes, including overlapping BGP/RFC work from other sessions. Preserve that dependency-complete state when resuming.
- The earlier session-state file contains obsolete phase, no-push, and pending-job statements. This checkpoint supersedes those statements. The spec remains the acceptance authority.

## Scope and implemented changes

Every first-party enum switch needs one disposition: C for closed internal values, O for open external values, or P for an intentional subset. C/O switches name every accessible distinct value. A retained C default asserts `BUG:` only after producer provenance proves the value internal; O defaults preserve safe unknown-input handling and explain the open set. P uses a reasoned switch-local `//exhaustive:ignore`. Valid zeros, sentinels, aliases, masked values, and registry fallbacks retain their behavior.

The seven package batches and parent setup sites are implemented. The root exhaustive setting is false and the style guide states global enforcement. The supported golangci-lint/exhaustive upgrade is committed as `d0bf51e9af43`; Commands is committed as `93bb60878ad8`. Construction rules and a design-only follow-up spec are in `f11f3077b2`. The historical VRRP recorder was preserved as non-executable Markdown in `dc3c3cd998`; do not repeat that archive commit.

The owner's expanded scope includes fixing all identified lint/runtime defects, renewed RFC evidence, and VPP SR-policy/local binding-SID lifecycle with real forwarding proof. Implemented repairs cover directional Extended Message permissions, EAP discard behavior, protocol transitions, malformed AS_PATH bounds, RIB ownership and shutdown races, callback shutdown, tooling input fingerprints, fixtures, and platform contracts. Implementation reports describe edits; they do not establish execution.

The VPP deployment proof is implemented in eight `internal/le/test/deployment/vpp_srv6_*test.go` files. It exercises the BGP/connected producer path, normal Ze restart, managed VPP restart and replay, tenant tables 10/20, shared policy ownership, replacement, withdrawal, and reinstallation. It retains an independent packet oracle. Its prepared coverage remains unproven on the native ARM image.

## Observed evidence and limits

The original full race run was red: 637 packages passed, 218 had no tests, and 11 failed. Later scoped results below do not turn that run into a full-suite pass.

| Surface | Latest observed result | Remaining limit |
|---|---|---|
| Committed tree `d9844ae724e6` | `env CGO_ENABLED=0 ./le --name enum-checkpoint repo compiles check` passed all five flavors: distro, appliance, setup, host, installer | Compilation only; not full lint, tests, runtime acceptance, or debt clearance |
| Strict exhaustive contract | Native scoped direct/alias omission, explicit-case, new-member, subset, and adjacent-unmarked controls passed | This is a scoped enforcement proof, not full lint |
| Full native lint | Last completed matrix reported 136 distinct diagnostics, none exhaustive; repairs followed | The latest attempt failed before linters because the input Git scan was killed; the complete post-repair matrix is owed |
| Reactor/RIB/capability/context/format | Full-feature scoped race passed; changed reactor paths passed 20 repetitions | `TestInProcessScale20` remains open; no whole-tree race claim |
| RIB lifecycle | Missing peer-slot regression failed before repair; seven targeted lifecycle tests passed race; independent follow-up review cleared | Preserve peer-reference reservation before releasing `peerMu`, retained storage ownership, and same-best reference balance |
| Tunnel withdrawal fixture | Corrected valid fixture failed against original producer and passed corrected producer | Earlier runs without `nextHopScope` were not discriminating; retain interface `192.0.2.2/24` |
| VPP lifecycle | Durable flush retry regression failed before repair; full VPP package race passed; independent review cleared | Ownership maps remain until `RemoveKey` succeeds; package tests do not prove forwarding |
| Tooling/hooks | Broken enclosing-Git regression failed before repair; repo-compiles selftest and package race passed; hooks race and actual CLI hooks 30/30 passed | Shared `goEnv` owns `GOFLAGS=-buildvcs=false`; do not add a second override |
| EAP and Extended Message | EAP/IKE engine race and public API smoke passed: wrong-role discard, 21 undefined Codes then valid Identity, four directional combinations, 4097-byte UPDATE, fixed OPEN/KEEPALIVE | Named-peer Extended Message runs and refreshed RFC proofs are owed |
| strongSwan | `eap-mschapv2`, `responder-eap-mschapv2`, `eap-tls13`, and `responder-eap-tls13` passed | No malformed-Code peer-injection claim; parallel attempts collided on a fixed subnet |
| Portability | Earlier FreeBSD cross-build passed; DragonFly exposed signed `Blocks` conversion | Both consumers now use existing `UsableBlocks`; post-fix DragonFly and js/wasm compile/runtime have not run. Latest wrapper failed before build during tracked-change fingerprinting |
| VPP runtime | Translated amd64 VPP 26.06 created real policy and steering; native ARM image build passed | AF_PACKET creation failed under translation (`SIOCETHTOOL` ENOSYS, `PACKET_VERSION` errno 92). No forwarding proof and no packet run on the native image |

The final pre-repair frozen census retained 825 typed identities: 459 C, 202 O, and 164 P. Its raw status retains two excluded lexical candidates, both source/LSP-resolved built-in string switches. Later guards and edits are not reconciled. The earlier RFC campaign recorded 281 canonical records, but later repairs invalidate affected evidence. Three weak RFC8654/RFC3748 audit rows remain Partial until current discriminating proof and independent rejudgment justify a change.

## Evidence portability

The tracked archive is `plan/handover/enum-switch-exhaustiveness-evidence.tar.gz`; its adjacent `.json` manifest records every member's SHA-256. All 448 files were decompressed and compared with their original artifacts before commit. It contains reports, logs, pre-fix source snapshots, census data, and non-executable JSON copies of scratch recipes. Restore the original relative paths from the repository root:

```sh
tar -xzf plan/handover/enum-switch-exhaustiveness-evidence.tar.gz
```

The archive does not contain Docker images, VM state, caches, or compiled tools. Original paths starting with `artifact://` remain host-only. The restored scratch root is:

```text
tmp/session/2026-10-04-01a10694-3795-71f2-8250-25e3a877f4cf/scratch/
```

Important originals, relative to that root:

- `enum-implementation/review/manifest.json` and `enum-followthrough/review-manifest-01.json` describe scope. Concurrent BGP RFC edits existed; do not treat every dirty path as enum-owned.
- `enum-followthrough/Repair*.json`, `ReviewRetainedRIBCutover-followup.json`, `ReviewVPPSRv6Lifecycle-final.json`, `ReactorInterop.json`, and `prove-vpp-srv6.json` contain repair/review details and proof recipes. The VPP report includes the native image build result and exact next command.
- `job-enum-full-native-lint-after-repairs-a6dafd5a.log` records the pre-linter failure; `enum-lint-diagnostics-02.json` records the prior 136 diagnostics.
- `job-enum-reactor-rib-final-race-20debfaa.log`, `job-enum-reactor-changed-race20-b1b44791.log`, and `job-enum-hook-fixtures-after-facd36bc.log` record the latest scoped race results.
- `job-enum-vpp-real-forwarding-5ebc196e.log` and `job-enum-vpp-interface-diagnosis-5ebc196e.log` record the translated VPP failure. `job-enum-vpp-native-image-6d42e940.log` records the native build.
- The frozen census's `output/census.json` and status, strict alias-probe transcripts, census collector sources, and RFC proof plans are included. Full frozen source trees and compiler caches are excluded.

If the package omits a scratch runner, pre-fix snapshot, packet capture, or log needed for a remaining proof, reconstruct it from tracked producers and the report before execution. An inaccessible original artifact cannot support a new success claim.

## Resume on Linux

Use the registered `le` actions and job admission. A single coordinator owns verification after edits; do not start competing lint/build jobs. Docker IPsec scenarios must run serially because they share a fixed subnet. Keep other fixed-resource Docker proofs serial as well.

1. Read this checkpoint, the spec acceptance criteria, and the packaged report manifest. Check the destination's actual source state and publication commit before modifying anything. Keep the spec in progress.
2. Diagnose the killed tracked-input/Git fingerprint scan first. Both lint and the portability wrapper stopped before their intended checks, so repeating those commands without resolving admission does not supply missing evidence. Do not suppress fingerprint errors or weaken coherence checks.
3. Prepare native Docker and full-feature Go execution. For linked schema/decoder tests, `ze_core` alone is insufficient. Use `ze_core` plus every unique `ze_` feature in `feature-gates.txt`. The native unit actions derive that set; an admitted manual Go test must pass it explicitly.
4. Execute the VPP proof below and the pending peer/platform checks. Preserve failures and exact input identities. Resume RFC recordings only after the affected source is stable.
5. Reconcile the final census and inventory, then finish the full native lint matrix, docs/indexes, journal validation, and independent final review. Phase 2 is the remaining acceptance and closure work; no speculative source replacement is prescribed before the failed proof or admission cause is known.

Native ARM VPP build recipe, using the same official image recipe that built on the original host:

```sh
./le job run label enum-vpp-native-image command docker build \
  --platform linux/arm64 --build-arg VPP_VERSION=26.06-release \
  --tag ze-vpp-enum-de830ef0:26.06 \
  'https://github.com/ligato/vpp-base.git#671a3aa978e1d558c1c07499fd871b92a7c6c840'

ZE_VPP_DOCKER_IMAGE=ze-vpp-enum-de830ef0:26.06 \
ZE_VPP_DOCKER_PLATFORM=linux/arm64 ZE_VPP_DOCKER_GOARCH=arm64 \
./le job run label enum-vpp-srv6 command go test \
  -tags integration -count=1 -timeout 30m ./internal/le/test/deployment \
  -run '^TestVPPSRv6ServiceRoute$' -v

./le test deployment vpp-test
```

The original local image manifest was `sha256:41f1845545318353df81874473c8d8a80b4f6290688e31d8cd27548cc6b8f500`. It is host-local and is not an image distribution address. The recipe above pins the observed upstream commit; record the destination image identity because package repositories can change. On amd64, use matching native image/platform/GOARCH settings instead of ARM emulation.

Prerequisites are privileged Docker, checkout bind mounts, working veth/AF_PACKET, matching VPP SRv6 API support, `/usr/bin/vpp`, writable `/etc/vpp`, sufficient existing hugepage resources, and VPP core packet-generator support for managed mode. Do not change global hugepage reservations as a fixture workaround. External mode must retain raw-packet forwarding proof; managed mode uses actual VPP packet-generator graph ingress/egress. Preserve state dumps, managed transcript, and packet captures from the printed scratch directory.

The VPP discrimination recipe in `prove-vpp-srv6.json` requires a temporary, exactly restored mutation at `(*govppSRv6Backend).acquirePolicy`: omit the actual `SrPolicyAdd` request while retaining the local success path. Rebuild the exec-reached daemon without result-cache reuse; the installed-state proof must fail for absent policy. Restore the producer and require non-skipped external and managed forwarding, normal Ze restart identity retention, actual managed PID change/replay, tenant assertions, and final removal. Neither phase is complete.

The next named-peer runs are:

```sh
INTEROP_SCENARIO=bgp-extended-message-asymmetric-frr ./le test integration interop
INTEROP_SCENARIO=bgp-extended-message-bilateral-frr ./le test integration interop
INTEROP_SCENARIO=bgp-extended-message-send-denied-frr ./le test integration interop
INTEROP_SCENARIO=bgp-extended-message-remote-only-reject-frr ./le test integration interop
INTEROP_SCENARIO=bgp-extended-message-neither-reject-frr ./le test integration interop
INTEROP_SCENARIO=bgp-ebgp-ipv4-frr ./le test integration interop
INTEROP_SCENARIO=isis-p2p-frr ./le test integration interop
./le go lint run
```

`ReactorInterop.json` specifies directional mutations and exact packet/FRR route predicates. Its OPEN-only relay gives FRR a bilateral view while preserving Ze's tested directional permissions; it does not establish native directional negotiation in FRR. Also owed: PPP/L2TP/PPPoE real-peer checks, post-fix DragonFly cross-build, js/wasm compile and Node runtime, and full-feature `TestInProcessScale20`. Node existed only on the original host, not its VM; install or locate a destination runtime before claiming js/wasm execution.

## Remaining acceptance obligations

- AC-1: Collect a stable type-aware census from the real lint build populations and reconcile every first-party C/O/P identity, complete switches, new guards, generated sources, coverage exceptions, and all prior suppressions. Preserve raw errors and supplemental string-switch evidence.
- AC-2 through AC-5 and AC-8: Preserve named-value behavior, valid zeros/sentinels/aliases/masks, BUG assertions only for proven internal impossibilities, non-panicking open defaults, and reasoned subset behavior. The identified runtime repairs need their discriminating regressions and lifecycle/race evidence retained.
- AC-6 and AC-7: Retain real-action direct/alias omission proof, default-independent enforcement, the typed SAFI replacement, and removal of old suppressions. No permanent checker, launcher adapter, blanket exemption, or opt-in enforcement mode is authorized.
- AC-9 and AC-12: Finish current documentation, canonical RFC discrimination, independent audit judgments, and producer-derived RFC/docs indexes. Do not upgrade the three weak rows on historical records. Preserve foreign audit scope, including whole-set MED. Validate journal records and evidence links.
- AC-10 and AC-11: Complete the post-repair all-linter native matrix, changed external-input smokes, affected runtime/race proofs, named-peer interoperability, and both VPP forwarding rails. A failed wrapper or successful image build cannot discharge these criteria.
- Final independent review, acceptance reconciliation, and native commit/publication gates remain separate from this checkpoint. Do not close the spec on prepared tests or scoped successes.

## Work not included

`plan/spec-validated-construction-and-state-types.md` owns the separately requested construction/state-type follow-up and remains design-only. No constructor/API migration belongs in this checkpoint. The VPP authorization excludes SRPM, remote decapsulation, and a new BGP tenant-table producer. Absent RFC2545 third-party link-local behavior, RFC4271-8.2.2-13/-20, RFC4301 plural SAD behavior, and RFC7296 protected INFORMATIONAL support remain unauthorized. No other implementation owner is established here for those absent features.

The source already inspected for this effort includes the exhaustive analyzer implementation, `internal/le/go/lint` planning, the enum style rule, and the package classification/review reports. Those conclusions can be reused without repeating broad historical review; remaining producer edits still require inspection of the current owning code. Relevant operating rules are in `docs/contributing/running-commands.md`, `testing.md`, and `committing.md`.
