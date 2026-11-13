# Deployment migration handover — 2026-11-13

**Spec:** [deployment-user-journeys](../../release/delivery/spec-deployment-user-journeys.md), in progress.
**Branch:** main. Wiki checkout: ../wiki, branch github.
**Goal:** Reduce friction across Linux installation/removal, Docker, VM/ISO, containerlab/netlab and gokrazy, with accurate documentation and website guidance.

This is the owner's requested migration checkpoint. It intentionally preserves unfinished work. Resume from this document; do not resume the older scratch notes as if their active jobs or temporary mutations still existed. Nothing was pushed or published for this checkpoint.

## Status

Done and previously committed:

- Linux install/uninstall/purge lifecycle, native amd64/KVM ISO selected-disk installation with the other disk preserved and installed SSH access, and appliance OTA update/recovery checks are recorded in the spec. Their previous successful runs remain evidence for their recorded source, not for every later change.
- Public two-peer quickstart, IS-IS transaction/rollback repairs, image/archive delivery helpers and deployment website source improvements landed earlier. A prior Compose persistence image was not tied to the final source; it is not final release-artifact evidence.
- Interface address replay: 0ec5666e2e942a26e871ead6789b3a51bf44cd4e. Connected startup/retirement and doctor: c5baed81d002c3ccd0a9cb442b121165a5c9eab6.
- Netlab tool compatibility: 994c2c115247423afd308245adc77179d606d419. Bootstrap/golden fixture: 4644c95058 (full SHA resolves in git); draft baseline, all eighty Ze-kernel stress invocations and promoted live path passed. Full topology traffic is still owed.
- BGP Default next-hop handling: 7bf418baaa5c564a9b01163a48bbdf135c683fbf. RFC carrier-move tooling: 19086169f5d9775c8a49f15c51492d7709cee3ee. Interop diagnostics and actual TCP emission: 089f7edaaafc3bad57234d2fc4a41aad5e297ebc.
- Complete set-format line reading, including actual four-MiB receiver boundary: 9b475acf09556b5b8ecc581ed7fe2fe4a0beb6b8. Correct carriage-return output expectation: 750362b574160f89c75186171227b62a2375830c. Every one of these Go commits passed all five committed build flavors.

Implemented in this migration checkpoint:

- Real authenticated, host-key-verified SSH configuration upload subsystem and canonical daemon transaction, replacing invented remote shell commands. Shared mutation ownership survives listener changes and shutdown; a stopped confirmed window refuses further uploads and source commits.
- Named appliance bootstrap keys use the local credential namespace and accepted admin identity, respect disabled admin and configured-user precedence, and reject malformed/restricted/certificate keys. Doctor reports unusable bootstrap credentials and clears the finding after repair.
- Corrected FRR OSPF-to-BGP fixture uses an ordinary dummy interface. The previous self-host prefix and loopback network-type fixtures were invalid; their failures do not prove the desired consumer behavior.
- Native appliance-config-push action, one current image, three fresh processes using the same disk, private agent, out-of-band disk host pin and canonical selected-source inspection. **This runner has known findings below and is not proven.**
- Appliance, authentication, storage and architecture documentation updated to the actual producer behavior. Website source chooser/tutorial improvements are already committed; final generated-site checks remain open.
- Two next-hop audit records rejudged and stamped without widening their claims. This does not resolve the shared tree's other RFC failures.
- Doctor functional test promoted at exact SHA256 bdb94769a89c4b0f0d0843b0809373e3ffea66d49916a48584823aaca2111d14.

## First work on the new machine

Read AGENTS.md and its included rule files. Use the main checkout, native owning actions and session-private named outputs. No worktree, test export or whole-tree le verify. Check upstream before edits and claim the deployment spec with the new session identity. Do not reuse the old session ID to perform new work.

The checkpoint commit manifest is [checkpoint-paths.txt](checkpoint-paths.txt). The source-review reports in evidence/ preserve completed reads; they are preparation, **not a CLEAN Review Gate**. The earlier reviewed population includes 120 IKE paths carried by historical Spec-work commits. Retain that population and reconcile later intersecting commits; do not silently exclude it or restart a broad audit.

### Edit 1 — repair the two subsystem client lifecycles

Files: internal/le/test/qemu/install_config_push_ssh.go, configPushMalformed, around line 148; internal/le/test/qemu/install_config_push_protocol_test.go, productPush, around line 378.

Current sequence includes these exact calls:

```go
session.RequestSubsystem(configupload.Subsystem)
session.Wait()
```

Replace that session-based subsystem exchange at both sites with the supported owned raw SSH session-channel exchange used by internal/appliance/cmd_config_push.go:sshExecReal: OpenChannel("session"), subsystem request with reply, framed input, CloseWrite, bounded stdout/stderr readers, explicit exit-status consumption and joined stream/request workers. Read the actual product helper before choosing the smallest shared helper. RequestSubsystem does not start Session's workers, so Wait currently returns "ssh: session not started"; accepting this local error as a remote refusal would be false evidence. Retain full framing, remote nonzero status, success-receipt and private-payload assertions. First reproduce using the existing sequence tests; no semantic RED is claimed yet for this final runner version.

### Edit 2 — prove observer authentication before edit denial

File: internal/le/test/qemu/install_config_push.go:configPushRefusals, around line 354.

Before the existing configPushTarget(namedPath, guest.sshPort, configPushObserver) and denied command, add a fresh pinned observer key login using guest.configPushSSH with configPushObserver, ssh.PublicKeys(signer), verify and HostnameQuery. Require its parsed hostname to equal configPushAcceptedHost. Then keep the existing nonzero/no-receipt, complete live-configuration and selected persistent-source comparisons. Otherwise a broken observer login can masquerade as an edit-authorization refusal.

### Edit 3 — record native evidence after the repairs

The exact existing documentation status is "The runner is an implementation checkpoint, not a completed native proof." Keep it until the nine intended action/helper/sequence parent tests and affected unchanged ISO/OTA tests pass, followed by the real native action. Record actual image/driver identities, all three joined boots, publication, persistence, both login methods and each refusal. Only then replace that status with the observed scope and update AC-12. Do not claim automatic live rollback or ISO installation from this configuration proof.

Phase 2 is needed after these edits: remaining labs, artifacts, documentation generation, immutable review and eventual closure are listed below.

### Then: scoped verification

Use the native unit action, serialized in the main thread:

```sh
./le --name deployment-user-journeys test unit pkg path ./internal/le/test/qemu/ run '^TestApplianceConfigPush'
```

TestApplianceConfigPushFixtureProcess is a child-process fixture, not a tenth acceptance parent. The intended nine parents are the three action/admission tests, three host-command tests, agent lifecycle test and two sequence/cleanup tests. Also run the existing affected OTA/ISO tests before the VM proof. Use the existing job admission route for long runs and record actual invocation, output and exit status.

On native amd64 with /dev/kvm available (or native arm64/HVF on the appropriate host), with room for built image and disk copies:

```sh
ZE_INSTALL_QEMU_ACCEL=kvm ZE_INSTALL_IMAGE_SIZE=1610612736 ZE_INSTALL_KEEP=1 ./le --name deployment-user-journeys test qemu appliance-config-push
```

Do not set ZE_INSTALL_IMAGE; this action must build the matching current native image. Init JSON must supply named authorized keys before secrets are created. Editing appliance.json afterwards does not regenerate secrets. Preserve the complete bootstrap-selected configuration, including discovered interfaces; change only hostname. The source uses one allocated SSH port before build for the guest listener and every host forward across all boots. Older scratch suggestions of fresh per-boot ports are superseded.

## Verification already observed

The identifiers below are suffixes/labels of the original native job logs. They describe actual runs; raw logs and generated coverage profiles remain optional scratch transfers.

| Scope | Observed result and limits |
|---|---|
| Hub, appliance, SSH, framing and configuration packages | Full race runs passed before the final shutdown/key/doctor repairs. The wider CLI run failed a CR spelling expectation (now corrected) and an unrelated LDP formatter round-trip case (still open). Do not report the whole CLI run green. |
| Confirm-window shutdown | f1e44291 reproduced upload and source-commit acceptance after stop; focused 16077554 passed after stable-window repair, including no-pending/revert-reload controls. |
| Bootstrap key login | Corrected e7e47c81 reproduced actual login failure; f57bc1e2 passed the key/assembly parents after repair. Initial fixture-path failure was not authentication evidence. |
| Restricted certificates | 7e0c86f7 reproduced unrestricted actual SSH acceptance of four certificate cases; 03acd7d2 passed certificate/assembly parents and plain-key control after parser refusal. That job also had separate doctor failures, so was not an overall green run. |
| Credential diagnostic | Actual missing-registration/report failures in 6398878a/03acd7d2; focused typed check, deduplication/clear and nil-store parents passed 6bc651c7. |
| Shipped doctor fixture | Draft baseline 2b3d8004 passed. Stress job a5bec302 produced eighty actual exit=0 invocations, parallel 4, burners 2. Native stress exit 1 means NOT REPRODUCED in this harness, not eighty failures. Exact promoted live path passed job-migration-doctor-promoted-0178ac2d.log. |
| Native proof helper frontier | Missing-action/helper semantic reds were observed before implementation; agent ownership/sign/verify/cleanup test passed 33bd91e2. Final full runner formatted e8ea6b6a, but the complete post-implementation nine-parent run and VM run have not happened. |
| RFC check | b7f48cf9 failed 186 shared-tree findings. The two owned next-hop judgments were subsequently stamped; a fresh final check is owed. No claim that the rest are fixed. |

Previous coverage profiles predate the final repairs and are not final coverage. Allocation/coverage facts can be retained, but this development VM is not a source of publishable timing/throughput figures.

## Remaining acceptance and ownership

1. Complete the native SSH proof repairs above, affected race/tests and real VM run. The spec remains in progress until actual acceptance evidence and closure.
2. Run corrected dummy FRR topology normally to GREEN first, then fresh old-explicit consumer counterfactual RED and exact source restoration. Run the unchanged full netlab topology with all six sourced pings. Earlier /32 and loopback failures are invalid discrimination. The pristine internal/component/bgp/redistribute/consumer.go SHA256 is 80e43ceedf6727f343765a74222c5af83a814f1f0682bb7b8fb4234b7108d4b0; no mutation remains applied at handover.
3. Finish source-derived affected reactor repetitions. The old broad race/count-20 attempt failed separately owned refresh/packing assertions and the global deadline; it is not evidence for all affected cases. The affected script is retained in evidence/. Complete Default parser/redistribution/types leaf-package race passes need no repeat merely because time elapsed.
4. Produce final source-bound preview artifacts for both Linux binary architectures and both images, verify manifest/checksums/CA, then docs indexes/links, wiki LLMS and the canonical ALL-producer website build/check. Final artifacts/website proof are not complete. Review the six-heading news draft only after its claims are true.
5. Reconcile immutable source review with the retained original population and later intersecting foreign commits, then the formal Review Gate and two closure commits. No push or publication is authorized by this handover.

Not done and separate owners:

- Dedicated QEMU/Xen kernel profiles: plan/proposals/spec-virtual-machine-kernel-profiles.md is a committed SKELETON only. No kernel build-profile implementation or Xen runtime proof exists.
- Unexpected redistribution producer loss: plan/release/fixes/spec-redistribution-producer-exit-withdrawal.md (new skeleton).
- External-orchestrator nil bus: plan/release/fixes/spec-plugin-event-bus-across-the-process-boundary.md.
- Recovery packing recurrence: plan/release/fixes/spec-bgp-one-off-defects.md, AC-5. Refresh-policy recurrence remains with its existing refresh spec/journal.
- LDP formatter round trip: accepted nested environment/log/ldp input serializes to flat "log ldp info" which reparsing refuses. The foreign ldp-backoff-clear-frr fixture/lifecycle work owns this; preserve its assertions.
- Legacy boot inbox checkPushedConfig and HealthRevert write a legacy mirror while the canonical selected pointer takes precedence. Docs now state the limit. cmd/ze/health_revert.go's old auto-revert header remains stale; no automatic 30-second live rollback is proven.
- Source inspection during checkpoint preparation also found pkg/zefs/registry_test.go still expects the old meta/ssh/authorized-keys registry entry while this patch replaces its registration with the local credential key. internal/plugins/connect/main.go still names that old outbound namespace. Check the intended distinct outbound registration and the unchanged registry test before final acceptance; no test verdict or repair is claimed here.
- Historical IKE findings retain their existing owner; deployment review preparation is not IKE acceptance.

## Files already handled

- cmd/ze/hub/config_upload*.go, confirm_window.go, main.go and SSH service wiring: canonical mutation/window ownership and actual receiver.
- internal/appliance/cmd_config_push*.go; internal/component/ssh/subsystem.go; internal/core/configupload/: actual host client, subsystem and bounded frame contract.
- cmd/ze/hub/bootstrap_credentials.go, register_bootstrap_credentials.go; internal/core/authorizedkeys/; pkg/zefs/keys.go; assembly and diagnostic fixture: bootstrap identity and refusal/reporting.
- internal/le/test/qemu/install_config_push*.go plus changed ISO/OTA adapters: final independent source report is evidence/deployment-native-config-push-runner-independent-source-review.md; its two findings are still open.
- internal/le/interoplab/bgp/*ospf_redistribute_default* and scenario: source-reviewed dummy correction; runtime topology evidence remains owed.
- docs/guide/appliance.md and architectural boot/storage pages: distinguish canonical device source, named host assembly source and legacy mirror. Do not restore the old invented shell-command or live rollback claims.
- evidence/deployment-complete-source-population-5797.txt and accompanying ledgers: reuse completed exact-source reads, then review deltas; no formal CLEAN.

## Transfer and resumption

The final user report gives the checkpoint SHAs and self-contained git bundle paths for main, wiki and the committed gh-pages branch. Copy those bundles to the new machine; validate with git bundle verify before importing into the normal checkout. They carry unpushed reachable commits. Dirty generated gh-pages content and unrelated sessions' uncommitted files are deliberately outside this session's checkpoint.

Do not transfer the old session state as live authority. Create a new session claim. On this machine no deployment job, VM or agent-owned build remains active at checkpoint. Signing was working after the owner unlocked the local key; signing-agent sockets and unlocked state do not migrate with git.

The wiki's .source-commit deliberately remains 31f16fc038ab9e63dee4d35b1b592b370f7f5073 because this was a partial documentation sync. Do not advance it as though every source change were documented.

No private SSH identity, appliance secret store, disk image, GPG material, socket, machine-specific cache or generated binary is committed here. A new machine can rebuild from committed sources. If retaining raw proofs is desired, copy the protected old session scratch separately; use this handover's verdicts and review population, not its obsolete ACTIVE notes, to resume.

