# Native configuration-push runner: independent source preparation

Reviewer deployment_complete_source_review, parent SID
c43b4620-2510-44dd-8936-84100b444bb5. No formal CLEAN or native VM pass.

## Actual reviewed scope

Read the new applied runner's complete changed-hunk/reached-producer population:
actions.go, install_iso.go, install_ota.go and all nine install_config_push*.go
files (12 Go paths total). Full new files were read; retained earlier agent/
command frontier reviews were reused and their applied comments/fixture schedule
checked. Applied production/wiring/security, lifetime/test-edges and style/docs
lenses. Read the appliance state handoff's final applied-source section, accepted
private recipe, and own qemu-integration/appliance-guide paragraphs.

Verified reachable native action, actual current host-driver invocation, init
JSON carrying the generated named bootstrap key before secrets, one image and
three fresh processes on the same target disk, one held SSH port on both sides,
old OTA false/zero wrapper retaining guest22, and complete bootstrap-selected
source retained before hostname-only replacements. Preview/live/persistent
comparisons parse and canonically serialize full configuration. Fresh pinned
password/key observations and image/partition identifiers are checked. Actual
port allocator retains per-port process locks, so a long build does not release
the proof's allocation to another le run.

Stopped-disk inspection follows authenticated reboot/process join, clones only
the bounded /perm partition, opens extracted storage read-only, requires the
active pointer and resolves its selected version, and independently decodes
the exact host-key frame with CRC plus end-of-frame check. It does not use the
deliberately stale active-file mirror in the unit fixture. Private agent and
child-only authentication environment avoid the user's HOME/socket/known_hosts;
cleanup restores owned source/target/pin files, stops agents and joins QEMU
before serial close. No alternate live-store writer or automatic rollback claim
is introduced. SSH I/O capture is bounded and phase cancellation closes its
transport through a joined callback.

## Findings sent to ROOT before fixes

1. **Source-bounded correctness blocker: unsupported subsystem session
   lifecycle in two helpers.** configPushMalformed in install_config_push_ssh.go
   calls Session.RequestSubsystem then Session.Wait; actual x/crypto v0.58.0
   RequestSubsystem223–231 never calls start, and Wait399–402 returns session
   not started instead of remote *ExitError. Output copy workers also never
   start. The new protocol fixture's productPush has the identical lifecycle
   and turns that local error into Code1, so the positive sequence fails before
   reaching the malformed control. ROOT must reproduce through existing actual
   sequence/receiver coverage, then use a supported owned raw-channel lifecycle
   with explicit stream/request joins at BOTH helpers. Do not accept a local
   session error as remote refusal or weaken receipt/exit/privacy assertions.

2. **ISSUE: edit-denial proof lacks authenticated-observer positive.**
   configPushRefusals points the named command at the observer and only requires
   nonzero/no-success. If observer-key authentication is broken, the control
   passes without reaching edit authorization. The accepted recipe explicitly
   requires an authenticated read-only observer. Add a fresh pinned observer
   key login performing allowed HostnameQuery and require the expected hostname
   before its denied push. Preserve all later command/refusal/live/persistent
   assertions. This strengthens the new proof rather than adding product scope.

No execution by this reviewer. These are not claims of ROOT-observed semantic
RED; ROOT owns reproduction before repair. Existing new nine-parent frontier
and affected unchanged OTA tests remain owed after the current stress job.

## Read hashes before repairs

| Path (internal/le/test/qemu/) | SHA256 |
|---|---|
| actions.go | 7f962f5e2a3aea2ed2de8980027bca1a48d0bd8727600270acc3b8213942df02 |
| install_iso.go | 26edb97b4bf112d6358d1629b63913b7ae6ccd7aa09a89b02663c4673e4c86e7 |
| install_ota.go | 12ff707a7ae654f48ae3dfcf1516caee0c96c3e6591b8fe4ab564001fe0e5a1e |
| install_config_push.go | 7a4484600405495843b55781deba3a3650077bb3c67f412ba892f6b1359ba69b |
| install_config_push_ssh.go | 3220d1046b702b5228f3579822559ac395fc3ac8d89b47a60716ca4cdbf8cf8c |
| install_config_push_storage.go | d96a055061c9520b2e7191df5160a6b623994ef4cc12183b859391255bc45a39 |
| install_config_push_agent.go | bfea140b60522adbfa4d77a2495ae51d1f777213bf16ca2ec4eac815ca159b96 |
| install_config_push_agent_test.go | 1a8bf67b6b0f9f55f359960dd6ce19eb3982e47374faa212e4a1e68c245bbf1b |
| install_config_push_command.go | 89dcb065f79b432b92d9177adc8c084f95bcd01143aa6a895fdefd56da38a216 |
| install_config_push_command_test.go | c6007a36d9d15a8cbf9afe13356bc0112d3e0da558db5b4e17e46cb337aed027 |
| install_config_push_protocol_test.go | 3be3aa54c188c1c9f36693154f7b1ae9c78516895efaf6a85768caaebcd3f2d3 |
| install_config_push_test.go | 49bdb78fe472a315718bbb0671fd23800b8cad1db1edbad612a37b3778cfebfa |

## Remaining and boundaries

Read only the exact repairs and reached consumers after ROOT's discriminating
RED. Actual native KVM/HVF proof remains owed; unit orchestration never stands
in for it. Later docs phase must correct known core-design.md section21 stale
live30s-flap-rollback/active-file-default text from actual boot producers, and
index the new native action in ai/INDEX.md (no action hit at review time).
The scratch recipe retains historical diagnosis before bootstrap repair and an
old fresh-port suggestion; actual applied source/handoff supersedes those lines.

Retained full historical431 including foreign IKE paths remains in final scope;
later exact native census must union these paths/versions without exclusions.
No tests/builds/formatters, product execution, tracked edits, commits, children,
formal ReviewGate record or closure by this independent reviewer.
