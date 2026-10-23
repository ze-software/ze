# Handover: close spec-ike-eap-rfc-defects

Written 2026-10-21 by the agent that drove `plan/immediate/spec-ike-eap-rfc-defects.md`
toward closure. The code is complete and reviewed; what is left needs the one heavy
slot on the Mac (a QEMU docker-lab), which another session's `./le verify worktree`
held for over three hours.

## Done this session (evidence a machine can reach)

| Item | Evidence |
|------|----------|
| Independent `/ze-review`, four rounds, converged 0 BLOCKER 0 ISSUE | round 1 found a BLOCKER (a rekey after a NAT rebind kept the retired pair's peer ADDRESS) and two ISSUEs; fixed in `5754eac4f4`; round 2's four ISSUEs fixed in the same commit; round 3's two ISSUEs fixed in `25938b3000`; round 4 clean (three NOTEs: the "ze defect" refusal in `installChildSA` names no next step, the lost-tunnel row has no configured-remote qualifier, a constant comment says "the peer declined it" which fits only the initiator) |
| Red then green for every new unit | eight mutations in a `git archive` export, each reddening its test (`rebind_address_test.go`), green restored; `./internal/component/ike/...` passes |
| RFC4301-5.1-7 re-judged by an independent auditor after `installChildSA` changed | `rfc/audit/rfc4301.json` in `5754eac4f4`: `unimplemented`, gap stands |
| A-5, A-7, A-9 confirmed from strongSwan 5.9.14 source | spec "Assumptions" rows (uncommitted spec edit, see below) |
| CI runs that carry the patched-tree steps | qemu-nightly run 39402173269 (jobs needs-linux and runtime-kernel-labs, step "Select the patched Go standard library": success), CodeQL Advanced 39576850399 (Analyze go: success), govulncheck 39407444997 (success) |
| QEMU guest under the patched tree | `f0493deda2` body: a real `./le test qemu` run passed with the guest tree in its tmpfs |
| D2 unit evidence | `go test` of `internal/core/eap`, `internal/le/go/stdpatch/...`, `internal/le/go/patchstamps`, `internal/le/go/toolchain`, `internal/appliance`: all ok on 2026-10-21 |

## Still owed before `/ze-close` (in this order, one heavy run at a time)

| # | Item | How |
|---|------|-----|
| 1 | Green run of the four new interop scenarios and the three existing real-nat ones | `./le test qemu docker-lab lab "test integration interop-ipsec" env "IPSEC_INTEROP_SCENARIO=<name>"` for `real-nat-port-translated-ze-responder`, `real-nat-port-translated-transport-ze-responder`, `real-nat-port-translated-eap-ze-responder`, `responder-eap-tls12`, `real-nat-tunnel-control`, `real-nat-transport-ze-responder`, `real-nat-transport-ze-initiator` |
| 2 | Revert-rebuild-red walk per new scenario | in a `git archive HEAD` export (never the shared tree): ports (`childEncapPorts` answering `transport.NATTPort` twice) reddens phase one of the tunnel and transport scenarios; a no-op `rebindChildSAs` reddens both rebind steps; the D1b adoption order reverted in `handleResponderEAP` reddens the EAP scenario; `responder-eap-tls12` has no reachable red (A-5 confirmed: charon never logs the id), record that |
| 3 | `TestRFC3948InstalledStateCarriesTheIKEPeerPort` (integration, linux, root) | `./le test qemu run kernel ~/.cache/ze/runtime-kernel/<entry>/vmlinuz command "<go test -tags integration -run ... ./internal/component/ike/engine/>"` |
| 4 | `docker/Dockerfile` proof build under the patched tree | `docker build -f docker/Dockerfile -t ze-d2-stdpatch-probe:<id> .`, then remove that tag by name |
| 5 | Full `./le verify worktree` once, attribute any red in the commit body | skip if `./le verify status check` is FRESH |
| 6 | `/ze-close` | append `plan/TEMPLATE-CLOSURE.md`, add the missing Deliverables Checklist, fill Goal Validation with the runs above, `./le spec review record ... rounds 4` (findings files of rounds 1 to 4 were scratch: their content is summarised in the row above), commit A then commit B, `./le spec release` |

The spec edits of 2026-10-21 (Assumptions A-5, A-7, A-9) and this handover are
committed together.

## Progress, 2026-10-21 afternoon (closure agent, session 0b4a8e87)

- Spec claimed. The Deliverables Checklist is added and `plan/TEMPLATE-CLOSURE.md` is
  appended and filled except the cells marked in capitals (GOAL-D1, GOAL-D2,
  INTEROP-TABLE, LINUX-INT, FULL-VERIFY, REVIEW-ARTIFACT, REVIEW-CHECK, A1/A4/A6/A8
  evidence): each waits for the run that answers it.
- The interop runs go from a `git archive` export of `5785852cbb`, not the shared tree,
  so the greens and the mutant reds are over committed code. The heavy slot was held
  all afternoon by two other sessions' `./le verify worktree` and a sibling's interop
  loop; the first attempt failed in setup (QEMU host port 2222 taken by a VM started
  the same minute), which is no verdict.
- Mutants for the walk: (1) `childEncapPorts` returns `transport.NATTPort` twice
  (tunnel and transport phase one); (2) `rebindChildSAs` returns `ownedOutcome{}` first
  (both rebind steps); (3) the `sa.adoptAuthenticatedEndpoint(...)` line before
  `buildAuthResponse` in `handleResponderEAP` removed (EAP scenario).
- The Linux integration test binary cross-builds with
  `./le job run label ike-int-build command env GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go test -c -tags "integration ze_core <feature tags>" -o <out> ./internal/component/ike/engine/`
  (13 s); run it in the guest with `-test.v -test.run TestRFC3948InstalledStateCarriesTheIKEPeerPort`
  and check it says PASS, not SKIP (it skips without CLONE_NEWNET).

## Runs reached a verdict, 2026-10-21 18:45 to 19:31 (disk back to ~10G free)

Full table in the spec, "Interop Tests", "Runs of 2026-10-21". Two blockers keep the
spec open; neither is a lab flake:

| Blocker | Evidence | Decision owed |
|---------|----------|---------------|
| AC-14 on the wire: Ze's transport-mode Child SA rekey after a NAT rebind is refused by charon with TS_UNACCEPTABLE | Ze's CREATE_CHILD_SA names the peer's public 172.28.0.7 (the RFC 7296 Section 2.23.1 substituted selector it installed); charon behind the NAT holds 172.28.0.3. RFC 7296 Section 2.23.1 describes only the client-behind-NAT initiator; the server-initiated rekey is not spelled out there | product fix in Ze (send the peer's original selector address, stored at creation as the section's "original source address", or let the NATed peer drive the rekey); read strongSwan `child_create.c` before choosing |
| `responder-eap-tls12` cannot pass: strongSwan 5.9.14 has no RFC 7627 EMS, Go refuses the TLS 1.2 MSK export | Ze WARN "cannot export the RFC 5216 Section 2.3 MSK ... on TLS 1.2" | which way: (a) the scenario asserts the TLS 1.2 handshake completes through Finished and Ze's RFC 7627 refusal, or (b) a TLS 1.2 peer with EMS (e.g. eapol_test) |

Also found: the D1b mutant stays green on the wire (earlier EAP rounds adopt the
endpoint in `handleOwnedInbound`), recorded in the spec. Fixture fixes made this
session: `ze-env` (`ZE_LOG_IKE=info`) in both rebind scenarios, `pfs disable` in the
transport one. Still owed after the blockers: transport walk (M1, M2), Docker proof
build, one full `./le verify worktree`, `/ze-close`.

## Stopped, 2026-10-21 18:30: the Mac disk is full

- No interop, walk, Linux integration, Docker or verify run reached a verdict.
  Attempts, all setup failures: 13:40 and 17:59 lost QEMU host port 2222 to a VM
  another agent started the same minute; 17:50 the export's `cache` symlink (to
  `~/.cache/ze`) broke the guest's docker build context ("too many levels of
  symbolic links"; never link `cache` into an export); from 18:14 the root volume
  was full (`no space left on device`, 117M to 474M free of 927G).
  `./le scratch store-trim` and `./le scratch cache-clean` freed nothing (every entry
  younger than 3h); le's caches and `tmp/` are about 120G, so the rest is outside le.
  Journal row: `plan/journal/full-disk-false-red.md`, 2026-10-21 ike-eap row.
- The spec now carries the Deliverables Checklist and the appended closure sections,
  committed with this handover; cells in capitals still wait for their runs.
- Next machine or session: once the disk has room, recreate the export
  (`git archive HEAD | tar -x -C <scratch>/walk`, no `cache` link), then run
  `<scratch>/series.sh`-style one scenario at a time, waiting only for no other
  `qemu-system` and no listener on TCP 2222 (a full verify held the wider slot for
  over 3 hours twice today while other agents ran VMs beside it). Then the three
  mutants above, the Linux integration test, the Docker proof build, one full
  `./le verify worktree`, and `/ze-close` (review artifact: `./le spec review record
  spec spec-ike-eap-rfc-defects.md verdict CLEAN rounds 4 file <the code files of
  5754eac4f4 and 25938b3000>`).

## Blockers decided, 2026-10-21 evening (closure agent, session 0b4a8e87)

| Blocker | Outcome |
|---------|---------|
| AC-14 transport rekey | Cause found in strongSwan 5.9.14 `child_create.c`: ze's Child SA rekey carried no USE_TRANSPORT_MODE, so charon took tunnel mode and skipped its own NAT substitution (`narrow_ts`). Fix: the rekey request carries the notify for a transport SA, the responder echoes it, the initiator refuses an answer without it. Spec Design Insight "AC-14 TS_UNACCEPTABLE" carries the RFC 7296 Section 1.3.1, 1.3.3 and 2.23.1 quotes and the rejected "send the original address" option. Unit tests `rfc7296_child_rekey_transport_test.go`, red before the fix. Committed `9d60c1d841`. Docker lab over it: `real-nat-port-translated-transport-ze-responder` GREEN 20:55; walk M4 (request hunk disabled) RED 20:59 with charon's "no acceptable traffic selectors found". Review round 5: 0 BLOCKER, 2 ISSUE (no rekey hold on the refusal, a false test comment), both fixed in the next commit with a red-proven test |
| `responder-eap-tls12` | Stopped on this item as the brief allows: no IKEv2 EAP-TLS initiator with RFC 7627 EMS exists. strongSwan up to 6.1.0 and master only defines the extension constant (`tls.h`), its `tls_peer.c` never sends it and `tls_crypto.c` `derive_master` uses the plain RFC 5246 seed; Libreswan's EAP-TLS is responder only (`ikev2_eap.c`: "there is no initiator code"); OpenIKED supports MSCHAP-V2 only; `eapol_test` speaks EAP over RADIUS, not IKEv2. Needs Thomas: keep the EMS requirement and re-scope the scenario's assertions to Ze's refusal, or drop the requirement |

Journal rows written: `plan/journal/test-shares-a-process-global.md` (race in
`TestImplicitDiscardReadsWhatTheLastApplyInstalled`), `plan/journal/silent-fall-through.md`
(a peer rekey of a transport SA without the notify; the two other rekey-refusal branches
with no hold).

Stopped here: a sixth review round (over the round-5 fixes) needs Thomas, and so does the
EAP-TLS 1.2 scenario. Still owed after both: the Docker proof build, one full
`./le verify worktree`, `/ze-close`. Lab traps met today: a `git archive` export needs the
ignored `gokrazy/docker-host/builddir/github.com/ze-software/ze/go.sum` copied in, and its
path must be short enough for the forwarded Docker socket (run it through a short symlink
such as `~/.zt/ikw`). A stale `.git/index.lock` (0 bytes, 20:23, no holder) left the
shared index pointing at pre-`9d60c1d841` content for its paths; the repair is in
`tmp/delete-0b4a8e87.sh` and the commit script log, for Thomas to run.

## Round 6 and the TLS 1.2 re-scope, 2026-10-21 night (Thomas: "ok to both")

- `responder-eap-tls12` re-scoped: `checkResponderEAPTLS12` (`internal/le/interoplab/ipsec/natport.go`)
  now asserts, numbered, charon `negotiated TLS 1.2`, `received TLS Finished handshake`,
  `received EAP_FAILURE, EAP authentication failed`, Ze's refusal facts (from
  `eapTLS12ExportRefused`), and no XFRM state on either peer. GREEN 22:07 in a `git archive`
  export of `a48a721a4c` plus the checker. RED 22:10 with a second stdpatch patch in the
  export that deletes crypto/tls's `noEKMBecauseNoEMS` branch: charon established the IKE SA,
  so assertion 3 timed out. The spec runs table carries Ze's exact refusal line. Committed with
  this entry.
- Review round 6 over `a1cbd95e05` and the re-scope: 0 BLOCKER, 1 ISSUE, 5 NOTEs. The ISSUE was a
  false sentence in `docs/architecture/testing/interop.md` (it claimed walks the spec did not
  yet record); fixed in this commit. The NOTEs are in the spec's Review Gate.
- Stopped here, per the brief: round 6 found an ISSUE, so round 7 (over this commit's page and
  checker) needs Thomas. Still owed after it: the `docker/Dockerfile` proof build, one full
  `./le verify worktree`, `/ze-close` (record with `rounds 7`, `rounds-reason` naming the rekey
  retry storm round 5 found, `owner-authorised` carrying his words for rounds 6 and 7).

## Round 7, 2026-10-21 late (Thomas: "and you can continue with what you're suggesting")

- Round 7 over `09333fb1a1`: 0 BLOCKER, 1 ISSUE, 6 NOTEs (spec Review Gate, rows 12 and 13 and
  the round 7 paragraph). The ISSUE: `interop.md` said the EAP port scenario has no reachable
  red; only the D1b line has none. Reworded in the working tree, and the N4 survey wording too.
  (Correction from round 8: that edit DID land, in another session's `f5ea135dd3`.) It was
  believed uncommitted because the file also holds another session's uncommitted
  hunks naming `dockerKernelRouteFor`, which HEAD lacks, and stripping foreign hunks is banned
  (owner rule, 2026-10-21). It lands when that session commits its kernelcheck work, or with
  the closure once their referent is in HEAD. Round 8 needs Thomas.
- `docker build -f docker/Dockerfile` over `54cbbc8dfa` in a `git archive` export: built, and
  the image's ze ran (`ze dev`); tag removed, export deleted. This closes the Docker proof item.
- Full `./le verify worktree` over `54cbbc8dfa`: started 22:46, STOPPED by me at ~23:05 with no
  verdict because the root volume fell below 5 GB (4.7G, then 3.5G; other sessions' runs).
  Still owed, once.
- Closure prep left in the working tree, uncommitted (committing a `plan/learned/` file reads
  as a closure): `plan/learned/032-ike-eap-rfc-defects.md`; citer repoints in
  `docs/contributing/patching-the-standard-library.md` (status now "built", points at 032),
  `ai/INDEX.md` line 611, `plan/pre-release/spec-rfc-verdict-fix-ike-eap.md` (3 lines),
  `plan/pre-release/spec-rfc-verdict-test-fix-pass.md` (1 line). The spec's closure placeholders
  are filled except FULL-VERIFY and REVIEW-ARTIFACT / REVIEW-CHECK. Next: round 8 (Thomas),
  then the verify, then `/ze-close` (`rounds 8`).

## Round 8, 2026-10-21 23:30 (Thomas: "Review round 8 for IKE EAP")

- Round 8 over `ee07c2b2da` and the page: 0 BLOCKER, 1 ISSUE, 4 NOTEs (spec rows 13, 14 and
  the round 8 paragraph). The ISSUE was real test debt: D1b had no interop proof, because
  strongSwan negotiated MOBIKE and `handleAuthRequest` records the endpoint at the first
  IKE_AUTH (RFC 4555 Section 3.3). Fixed: `mobike = no` in
  `real-nat-port-translated-eap-ze-responder/swanctl.conf`. Green 23:25, walk M3 red 23:26
  (Ze refuses the IKE_AUTH Child SA: no authenticated IKE endpoint), committed with this entry.
  Runs used an export of `09333fb1a1` (ike engine, EAP and ipsec interop code identical to HEAD; other `internal/le/interoplab` harness files differ) because HEAD's
  docker-host image needs a gokrazy module the cache lacks (`GOPROXY=off`, another session's
  `2b98d538dc`/`a02b362081` change).
- The matching `interop.md` rewording is in the working tree, NOT committed: the file holds
  another session's hunk naming `dockerhost.ColimaHome`, which HEAD lacks. It lands with their
  commit or with the closure.
- Round 9 needs Thomas. Then the full verify (disk was 24 GB at 23:20), then `/ze-close`
  (`rounds 9`), with the closure prep listed above.

## Round 9 clean; closure blocked by rfc/check, 2026-10-22 01:50

- Round 9 (Thomas: "ok") over `b2ff0fbbfc`: CLEAN, 0 BLOCKER, 0 ISSUE, 4 NOTEs. Artifact
  recorded: `./le spec review record ... rounds 9` with the rekey retry storm as rounds-reason and
  Thomas's words for rounds 6 to 9; `./le spec review check` OK over 10 code files. The artifact
  lives in `tmp/review/` (machine-local); re-record it if the closure runs elsewhere.
- Full `./le verify worktree` over `40e5a5ea23` (log
  `tmp/verify-worktree-logs/20261021T232548.747872000Z-p71703-r1-40e5a5ea239c`): stopped at stage
  49 of 55 when the disk fell to 2.9 GB. Its reds are attributed in the spec (Pre-Commit
  Verification, "all unit ACs"). Most are other sessions' work. **`rfc/check` is red on 42 rows
  this spec caused**: 39 discrimination records under `rfc/discrimination/rfc{3948,4301,4303,7296}.json`
  no longer verify (`producer-changed`, because this spec changed `respondChildRekey`,
  `applyChildRekeyResponse`, `rebindChildSAs`, `installChildSA`, `newChildSA`,
  `handleCreateChildSAOwned`). Also `rfc7296_child_rekey_transport_test.go` is named for rfc7296
  but carries no tag (`./le rfc rename`, a tag, or `// RFC naming: untagged -- <reason>`),
  RFC3948-2.1-2 has a SHIFTED audit verdict (`./le rfc reseal`), and RFC7296-1.4.1-1 has a STALE
  one (re-judge with `ze-rfc-audit`, `inbound.go::handleCreateChildSAOwned`). The full list is in
  the verify log's rfc/check stage.
- ai/rules/rfc-compliance.md says a tagged unit whose producer a change alters owes a fresh
  discrimination record in the same change, so these are inside this spec. They are not done,
  and the spec stays open.
- Next: free disk, then re-record each with `./le rfc discriminate-record` (one at a time), fix
  the file-name refusal and the two verdicts, rerun `./le rfc check`, then `/ze-close`. If the
  re-recorded code changes, a review round covers it (round 10 is pre-authorised: "If not, I run
  round 10."). The closure prep (`plan/learned/032-ike-eap-rfc-defects.md`, the citer repoints in
  `docs/contributing/patching-the-standard-library.md`, `ai/INDEX.md` and two `plan/pre-release/`
  specs) is still uncommitted in the working tree.
