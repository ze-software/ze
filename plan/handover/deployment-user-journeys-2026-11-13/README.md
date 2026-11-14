# Deployment user journeys: handover

**Spec:** [deployment-user-journeys](../../release/delivery/spec-deployment-user-journeys.md), in progress, not closed.
**Branch:** main. Wiki checkout: `../wiki`, local branch `wiki` tracking `wiki/master`.
**Goal:** Reduce friction across Linux installation and removal, Docker, VM and ISO, containerlab and netlab, and gokrazy, with accurate documentation and website guidance.
**Written:** 2026-11-13, at the end of the first session after the machine migration, and updated on 2026-11-14 by the session that ran step 0 on the Mac. Resume from this page. Nothing was pushed or published.

Start a new session with a new claim (`./le spec claim plan/release/delivery/spec-deployment-user-journeys.md`). Read `AGENTS.md` and its rule files first. Work in the main checkout, with native `./le` actions and session-private named outputs. No worktree, no test export, no `./le verify`.

## Where the spec stands

The spec's own evidence table (the rows headed `Criterion`, `Current
evidence`, `Remaining proof or limit`) is the authority. This is its state by
acceptance criterion. "Open" means the spec states it as not proven. Whether
an open item is proven or accepted as a stated limit is decided at the Review
Gate, never by leaving it out.

| Criterion | Proven | Open in the spec |
|-----------|--------|------------------|
| AC-1 native installation and removal | Real distribution and systemd evidence | Nothing stated |
| AC-2 upgrade, retention and refusal | Trust, unit, conflict, ownership and busy-store controls. A changed-version upgrade through `ze install`, by `test/ui/native-install-upgrade.ci` in the arm64 Linux guest: pass, 80 of 80 under load, red when the binary copy is skipped | Nothing: the owner ruled the arm64 run sufficient |
| AC-3 source build and named executables | All five flavors compile at recorded revisions | The source-bound artifact bundle, see AC-10 |
| AC-4 supplied two-peer quickstart | The control-plane tutorial | No production forwarding evidence. The whole-UI run was not green |
| AC-5 deployment container persistence | Authenticated edit and recreation on a source-unbound image | **Recreation on a source-bound image** |
| AC-6 live containerlab and netlab | All six sourced pings on a patched tree, then on the committed template at `92ca4ce6e4` on arm64 under HVF, each at the first attempt | No amd64 run from the committed template is recorded |
| AC-7 installer ISO and installed disk | amd64 under KVM. arm64 under HVF, after three fixes | Ventoy on arm64 was not run. The arm64 ISO was assembled by a Linux GRUB in the lab Docker, see the Mac facts |
| AC-8 appliance image update and recovery | amd64 under KVM. arm64 under HVF at `6b8656ae9f` | No routing or VPN traffic |
| AC-9 website deployment journeys | The owner reviewed the pages in `../gh-pages` on 2026-11-14; committed as `01d7ee17ef` and regenerated from it | **A clean site check**, and publication on the owner's order |
| AC-10 identified preview delivery | A private bundle at an older revision | **A bundle from the final pushed commit**, both architectures |
| AC-11 production-evaluator guidance | The chooser and update guidance | "Outstanding actual deployment evidence remains required" |
| AC-12 named appliance configuration push | amd64 under KVM and arm64 under HVF, this session | **A run from the release commit** on each host |

Also done this session, outside that table: the FRR topology with its
counterfactual, the affected reactor repetitions (one failure in 20, traced and
owned elsewhere), the RFC check for this spec's two next-hop requirements, and
the zefs registry test.

Still to come after every row above: source-review reconciliation, the Review
Gate and the two closure commits.

The arm64 rows (AC-7, AC-8) and AC-2's timeout were settled on 2026-11-13 and
14. The spec's evidence rows and its arm64 verification table hold the detail.

## What to do next, in order

Step 0 needs nothing from the owner. Each later step waits on him for its
first line.

0. **Done on the Mac, 2026-11-13 and 14.** AC-8 and AC-7 passed on arm64 under
   HVF. AC-2's timeout predates `9e3c0ab9c0`, which removed the store-lock
   folder fsync the strace had located; the script that timed out was never
   committed, so a committed test replaces it. Nothing is left of this step:
   the owner ruled on 2026-11-14 that the arm64 run of that test is
   sufficient, because nothing in it depends on the architecture (file copy,
   rename, lock and a configuration read through the standard library).
1. **Pull on the server.** The push happened on 2026-11-14: commits up to `6ad32587d3` are on origin, and `01d7ee17ef` and `4423922ca1` landed after it and are not. The `claude` server checkout was still at `990f785bca` at 02:45 that day, with 87 uncommitted files from other sessions. It only has what the owner pulls there. Once it has the commits, go to step 2. An agent never pushes on its own: the owner orders it, through `./le commit push "<owner authorisation>"`.
2. **Preview artifacts**, from the pushed commit. `./le help build preview` names the action. The earlier preparation recorded in the spec's AC-10 row passed for an older source. The final one covers static Linux amd64 and arm64 binaries and archives, the deployment and lab exports, the manifest and checksums. Build amd64 on the server and arm64 on the Mac: a kernel is never cross-compiled or emulated. Rerun `./le test qemu appliance-config-push` from that same commit on each host, so the proof is bound to the release source.
3. **Website and docs.** The owner reviewed the deployment pages in `../gh-pages` on 2026-11-14 and the work is committed as `01d7ee17ef`; `../gh-pages` was regenerated from it and is neither committed nor pushed. What is left: a clean `./le site check` (it exits 1 on `/guides/ai-agent/`, a page left by a working-tree build that included another session's uncommitted manifest row, and on `tests.fuzz_display` plain figures on four pages), then publication on the owner's order. The site is always generated into `../gh-pages`, which the owner serves locally; never into a scratch directory. `./le doc check all` was red on 2026-11-13 for reasons outside this spec: six stale source anchors in `docs/guide/configuration.md` and `docs/guide/operational-reports.md`, a `show system ntp` command with no declared answer shape, and 36 drift findings because the generated site was older than the live command catalogue. The pages this spec edited had no finding. Review the news draft in `evidence/deployment-user-journeys-news-draft.md` only after its claims are true.
4. **Source review, Review Gate, closure.** Reconcile the review with the retained population (below) and every later commit that intersects it, run the formal Review Gate, then the two closure commits. Do this last: each later commit makes a review stale.

A spec edit is gated on reading about a dozen doors and nine source files, so batch spec bookkeeping into one agent. Give that agent the facts and let it check them against the file: in this session it twice refused an edit that would have written something the spec contradicted.

The spec validator reports five structure errors after every edit of that file: no `### Unit Tests` heading, no `## Checklist`, and the checklist items "Tests written", "Tests FAIL" and "Tests PASS". They predate this session and no edit here caused them. Closure will need them resolved.

## Environment facts the next session needs

**The `claude` server (ssh host `claude`, Linux amd64, Ze's kernel, KVM).**

- Its root disk aborts writes when a QEMU guest writes into a fresh image stored on it. The kernel log says "Aborted Command" and "potential data loss". It happened on most days from 2026-11-10 and again twelve minutes after a restart on 2026-11-13. A proof whose scratch is on that disk fails at the guest's first write to its persistent partition.
- The second disk is mounted at `~/.cache` and logged no error. Put VM and lab scratch there: `TMPDIR=$HOME/.cache/<name>`. Docker's data already lives there.
- A non-interactive ssh has no Go on `PATH`: prefix commands with `export PATH=/usr/local/go/bin:$HOME/go/bin:$PATH GOPATH=$HOME/go`.
- The Mac's Bash hook refuses any command whose text names `/tmp`, on the server too. A proof run with its default scratch is therefore unreadable from the Mac. Set `TMPDIR` as above.
- netlab is not installed on the host, only inside the lab's tool image. For `./le test netlab check` and `update`, create a virtual environment with the version pinned in `contrib/netlab/Dockerfile.tools` and pass `NETLAB=<venv>/bin/netlab`.
- Other sessions run guest and Docker tests there from uncommitted files, and mutate files for red proofs. Tell them before a mutation and when it is restored. A tree that is clean at the start of a run can be dirty before it ends.

**The Mac (arm64, HVF).**

- The lab Docker is the `ze` Colima profile, which boots Ze's Docker-host image. That image keeps all of Docker's data on its own partition, sized when the image is built. Colima's 100 GiB data disk is not used by it.
- That partition was 14.9 GB, too small for a kernel build beside the lab images. Fixed: `e2627d3e7b` makes the image default 64 GiB (the file is sparse) and `995c46014d` makes `start` pass Colima a root disk of that size. Docker there now has 61.6 GB and was emptied on 2026-11-13, so lab images rebuild on first use.
- The arm64 runtime kernel for the current sources is cached under `~/.cache/ze/runtime-kernel/`.
- An agent is not permitted to delete Docker images or volumes there. The owner ran those two removals.
- The plain `./le` follows HEAD in the background, so right after a commit it
  can still be the previous build: the first ISO run after `be52bcb019` printed
  the old amd64 default. When a proof must include a commit just made, use the
  named build.
- An arm64 installer ISO run on the Mac needs four things. An arm64 installer
  kernel: `./ze-host appliance kernel --target installer --arch arm64 --builder docker`
  (about 17 minutes, cached, and copied to `build/kernel/Image`). A GRUB with
  arm64-efi modules, which macOS does not have: the session-private prefix
  `~/.cache/ze-deployment-journeys/grub` holds a `grub-mkstandalone` that runs
  Debian's GRUB 2.12 in the lab Docker (image
  `ze-deployment-journeys-grub:f96ab875`) and a copy of its modules; put its
  `bin` first on `PATH`. Scratch on a path with no symlink in it, because Ze's
  store refuses one and `~/.cache` is a symlink here:
  `TMPDIR=/Users/thomas/Unix/cache/ze-deployment-journeys/work`. And
  `ZE_INSTALL_KERNEL=$PWD/build/kernel/Image`. Remove the prefix and the Docker
  image when no further arm64 ISO run is owed.
- A needs-linux `.ci` runs on the Mac in Ze's guest:
  `./le --name <name> test qemu ci test test/ui/<name>.ci`. Its load run is
  `./le --name <name> test qemu docker-lab kernel <runtime vmlinuz> harness command '<the in-guest line the ci run prints> stress 80 stress-parallel 4'`.
- A named build (`./le --name <name> ...`) compiles the whole working tree, so another session's half-finished edit breaks it. The plain `./le` runs a build of the committed tree and is not affected. When a named build fails on a package this work does not touch, say whose edit it is and wait, or use the committed build if the change under test is committed.

## What landed in this session

| Commit | What it does |
|--------|--------------|
| `ca9571e0e2` | The config-push proof counts only a refusal the device answered. Three defects: the generated seed wrote list keys as `name` fields and was unparseable; both upload clients read a local error as a device refusal; a broken observer login passed as an edit refusal |
| `92b0b49dcc` | The proof asks for a machine reset (`kexec=off`). gokrazy's plain reboot is a kexec, so QEMU's `-no-reboot` never exited |
| `e4bdb55d1e` | The proof reads the configuration under the name the daemon's resolver gives it. A named appliance stores `<name>.conf`, not `ze.conf` |
| `929b1aec14` | Four operator messages named `show config`, a command group that answers nothing. They name `show config running`, and a test holds the host driver's hint |
| `8793b59939` | The live configuration is compared answer against answer of `show config running`. The two refusal controls that go through the host driver must fail for their own printed cause |
| `00d8e62202` | The zefs registry test expects the local admin authorized-keys key. An independent test judge approved it |
| `ef1a3cf33d` | The netlab render attaches `bgp-rs` and `bgp-adj-rib-in` and sets `next-hop self` toward external peers, so a node passes routes between its BGP peers |
| `e2627d3e7b`, `995c46014d` | The Docker-host image default and Colima root disk, described above |
| `10d888bda3` | Journal row `parked-work-returns-out-of-order`, owned by `spec-bgp-one-off-defects` |

The plan commits that carry the evidence and this page are not listed.

Landed by the step 0 session, 2026-11-13 and 14:

| Commit | What it does |
|--------|--------------|
| `1d8017e6fd` | The qemu and hardware installer kernel profiles build virtio SCSI in. An arm64 VM presents the ISO as a SCSI CD-ROM, and the installer had failed with "ISO media not found" |
| `be52bcb019` | The ISO and Ventoy proofs target the host architecture, and the proof finds the UEFI variable store from QEMU's firmware descriptors. A test judge approved replacing the test that pinned amd64 |
| `ece5c2a80d` | Three appliance build tests pin the host architecture. They had been red on every arm64 host since `d5ac4d3249` |
| `7de6a5d319` | `TestISOCaptureWaitsForActualPoweroff` no longer depends on host load |
| `0e4bdbe95b` | The harness builds `ze-upgrade`, the DUT under release 99.12.31, and `test/ui/native-install-upgrade.ci` proves a changed-version upgrade and a fresh bootstrap |
| `92ca4ce6e4` | Docs and spec: what is proven on arm64, and that a stock macOS host cannot build the ISO |
| `6ad32587d3` | Owner rule: the website is always generated into `../gh-pages`, review included |
| `01d7ee17ef` | Website: upgrade and removal guidance on every deployment journey, chooser links, the Start menu led by installation, netlab split into a guide and a lab page, same-site links to hand-written pages, the "Self-contained" hero point and the figures row |
| `4423922ca1` | `request wireguard keypair` makes its keys in process; it had run the `wg` tool, which the appliance image does not ship |

## Evidence observed in this session

**Appliance configuration push**, `./le test qemu appliance-config-push`. Recorded in `docs/architecture/testing/qemu-integration.md`, "Appliance SSH configuration publication", and in the spec's AC-12 row.

| Host | Source | Host driver | Image |
|------|--------|-------------|-------|
| amd64 Linux, KVM, QEMU 8.2.2 | `990f785bca`, tree clean at the start | `0973af0618ed` | `5a5a86c2d672` |
| arm64 macOS, HVF, QEMU 11.1.3 | `9bf21a753a`, other sessions' files in the tree | `08000cc46d7f` | `cdcd2df65929` |

Two earlier amd64 passes ran on a patched tree and are superseded by the first row. Neither row is release-artifact evidence. Each real run exposed one assumption the unit fixture shared with the runner, which is why every fix above changed the fixture as well.

**FRR topology**, `INTEROP_SCENARIO=bgp-ospf-redistribute-default-frr ./le test integration interop`. It passes at `990f785bca` in three minutes. With `internal/component/bgp/redistribute/consumer.go` sending the explicit next hop again, it fails at the assertion that waits for FRR to accept the route, and Ze logs that it withholds a route whose next hop is the peer's own address. The file was restored byte-exact (sha256 `80e43ceedf67...`).

**netlab topology**, `./le test qemu docker-lab lab "test netlab run"`. On the committed render three of six sourced pings passed: a node passed on only its own prefixes. With `ef1a3cf33d` all six pass at the first attempt, `./le test netlab check` passes against goldens regenerated with netlab 26.10, and `test/plugin/netlab-lab-profile.ci` passes in the QEMU guest. The live pass ran with the template change applied to `990f785bca` as an uncommitted patch identical to the commit.

**Reactor repetitions**, the script in `evidence/deployment-bgp-affected-checks.sh`: 414 tests, 20 repetitions under the race detector on darwin/arm64, no race report. One test failed in one repetition, `TestPolicyFinalReplayPreservesPreAdmittedBatchOrder`: a withdraw then an announce reached the wire reversed. The cause is traced to `takeOverflowReleased` reading the ordering hold again for each parked item. It is outside the next-hop change and owned by `spec-bgp-one-off-defects`, which still owes it an acceptance criterion.

**RFC check** on the Mac: red with 244 findings, 64 of them from missing entries in the Mac's Go build cache at the time. Neither `RFC4271-5.1.3-1` nor `RFC8950-4-1` is among them.

## Owner rulings from this session

Both were made on 2026-11-13 and each needs a design conversation with the owner in a session of its own.

- **Ze passes its best route between BGP peers by default**, as an ordinary speaker does, without a route-server plugin attached. Recorded in `docs/architecture/rib-transition.md`. The work is the skeleton `plan/release/fixes/spec-bgp-best-route-advertised-to-peers.md`, which lists nine design questions. Whether a redistributed route and a forwarded one for the same prefix reach the existing owner election is untraced and is one of them.
- **Run `spec-bgp-next-hop-auto-rewrites-for-ebgp`.** Under the default setting a forwarded route keeps its next hop toward an external peer, against the YANG text and RFC 4271 Section 5.1.3. That spec records the go-ahead. The netlab render's explicit `next-hop self` is a workaround it should make unnecessary without breaking.

## Owned elsewhere

- Dedicated QEMU and Xen kernel profiles: `plan/proposals/spec-virtual-machine-kernel-profiles.md`, a skeleton. No implementation or Xen proof exists.
- Unexpected redistribution producer loss: `plan/release/fixes/spec-redistribution-producer-exit-withdrawal.md`.
- External-orchestrator nil bus: `plan/release/fixes/spec-plugin-event-bus-across-the-process-boundary.md`.
- Recovery packing recurrence and the parked-batch reorder: `plan/release/fixes/spec-bgp-one-off-defects.md`. The refresh-policy recurrence stays with its refresh spec.
- LDP formatter round trip: accepted nested `environment log ldp` input serializes to a flat form that reparsing refuses. The `ldp-backoff-clear-frr` work owns it.
- The legacy boot inbox and `HealthRevert` write a legacy mirror while the canonical selected pointer takes precedence. The docs state the limit. The header of `cmd/ze/health_revert.go` is stale. No automatic live rollback is proven.
- Historical IKE findings keep their owner. Deployment review preparation is not IKE acceptance.
- `ze-host appliance iso` cannot build an ISO on a stock macOS host: no GRUB there has arm64-efi modules, and Homebrew's x86 GRUB keeps its modules where the build does not look. The guide says so, and the owner decided on 2026-11-14 that the ISO build gets no Docker path for GRUB: an ISO is built on a Linux host.
- Site tests. The eight tests the website commit owns pass. The `internal/le/site` package is red for other reasons: five tests wait on another session's uncommitted ai-agent guide wiring (manifest counts, a route row, an eyebrow row, a sidebar golden); `TestTheHomepageProofStripCarriesItsStatSpans`, `TestAFactTheSnapshotLostStopsTheHomepage` and `TestHomeHeroDerivesFromTheManifest` were red at HEAD from earlier hero commits; `TestTheSharedHeaderReadsAsThePublishedHeader` compares against a hand-kept golden that disagreed with HEAD's own `nav.json` before this work, and the Start menu changes widen it. A proposed golden for HEAD's menu is in the session scratch but was derived by hand and never run; whoever lands the ai-agent menu entry should regenerate it once, with the test judge.
- The site build compiles `ze` and `le` from the working tree, so another session's half-finished edit stops it. On 2026-11-14 that happened four times (doctor, skills, the BGP reactor, a help string in `internal/le/test/weakened`). A build that fails after staging leaves hand-written pages in `../gh-pages` without their generated shell until the next good build.
- `test/ui/pipe-local-command.ci` fails on macOS at "doctor kernel-capabilities answered no capability row", after the WireGuard step passes. Five parse tests failed in the same suite run as the WireGuard test passed: `ac1-native-orphan-refusals`, `iface-disable-flag-refuses-value`, `ipsec-dual-family-selectors-validate`, `ipsec-dscp-map`, `sysctl-mpls-ttl-propagate-refused`. None was investigated.
- Lint of `internal/le/test/qemu`, `internal/test/runner` and `internal/test/fixture` is red in files the step 0 session did not touch (errcheck and goconst in the config-push and OTA proofs, errorlint in the runner tests).

## Review population to keep

The source-review reports in `evidence/` are completed reads and preparation, **not a clean Review Gate**. The checkpoint commit manifest is [checkpoint-paths.txt](checkpoint-paths.txt). The earlier reviewed population includes 120 IKE paths carried by historical spec-work commits. Keep that population and reconcile later intersecting commits. Do not silently exclude it and do not restart a broad audit. `evidence/deployment-complete-source-population-5797.txt` and its ledgers hold the exact-source reads to reuse.

The two findings of `evidence/deployment-native-config-push-runner-independent-source-review.md` are closed by `ca9571e0e2`.

## Earlier history, before this session

Done and committed before the migration:

- Linux install, uninstall and purge lifecycle, native amd64/KVM ISO selected-disk installation with the other disk preserved and installed SSH access, and appliance OTA update and recovery checks are recorded in the spec. Those runs are evidence for their recorded source, not for every later change.
- Public two-peer quickstart, IS-IS transaction and rollback repairs, image and archive delivery helpers and deployment website source improvements. A prior Compose persistence image was not tied to the final source and is not release-artifact evidence.
- Interface address replay `0ec5666e2e`. Connected startup, retirement and doctor `c5baed81d0`.
- Netlab tool compatibility `994c2c1152`. Bootstrap and golden fixture `4644c95058`.
- BGP Default next-hop handling `7bf418baaa`. RFC carrier-move tooling `19086169f5`. Interop diagnostics and TCP emission `089f7edaaa`.
- Complete set-format line reading, including the four-MiB receiver boundary, `9b475acf09`. Carriage-return output expectation `750362b574`.
- The migration checkpoint `6f11891cab`: the authenticated, host-key-verified SSH configuration upload subsystem and daemon transaction; named appliance bootstrap keys in the local credential namespace with doctor reporting; the dummy-interface FRR fixture; the native config-push action; the documentation of the real producer behaviour; two next-hop audit records rejudged and stamped; the doctor functional test promoted.

Verification observed before the migration. The identifiers are suffixes of the original native job logs, which were not transferred.

| Scope | Observed result and limits |
|---|---|
| Hub, appliance, SSH, framing and configuration packages | Full race runs passed before the final shutdown, key and doctor repairs. The wider CLI run failed a carriage-return expectation (since corrected) and the LDP formatter round trip (still open). Do not report the whole CLI run green |
| Confirm-window shutdown | `f1e44291` reproduced upload and source-commit acceptance after stop. `16077554` passed after the stable-window repair |
| Bootstrap key login | `e7e47c81` reproduced the login failure. `f57bc1e2` passed after repair |
| Restricted certificates | `7e0c86f7` reproduced unrestricted acceptance of four certificate cases. `03acd7d2` passed the certificate and plain-key parents after parser refusal |
| Credential diagnostic | Failures in `6398878a` and `03acd7d2`. Passed in `6bc651c7` |
| Shipped doctor fixture | Draft baseline `2b3d8004` passed. Stress job `a5bec302` produced eighty exit-0 invocations. The promoted live path passed |

Coverage profiles from before the repairs are not final coverage. The Linux development VM is not a source of publishable timing or throughput figures.

## Notes carried from the migration

- The wiki's `.source-commit` deliberately stays at `31f16fc038ab9e63dee4d35b1b592b370f7f5073`, because the documentation sync was partial. Do not advance it as though every source change were documented.
- No private SSH identity, appliance secret store, disk image, GPG material, socket or generated binary is committed. A machine rebuilds from committed sources.
- The migration archive on the server (`tmp/deployment-migration-2026-11-13.tar`) was never needed: all three checkouts on the Mac were already at its commits.
