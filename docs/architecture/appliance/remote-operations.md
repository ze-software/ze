# Remote fleet operations

The bastion manages a fleet of gokrazy devices. After an image is built, the
operator pushes images and config changes without physical access.

<!-- source: internal/appliance/cmd_config.go -- config preview of the merged base and overlay -->
<!-- source: internal/appliance/cmd_config_push.go -- config push over SSH -->
<!-- source: internal/appliance/parallel.go -- bounded worker pool for --parallel N -->
<!-- source: internal/appliance/cmd_push.go -- listAddressedAppliances, shared by every --all command -->

## Decisions

| Decision | Reason |
|----------|--------|
| Config preview calls the same seed-config resolution as assemble | the preview cannot diverge from what the build produces |
| Config push uses the operator's ssh-agent, not the Ze passphrase agent | SSH key management belongs to the operator |
| `--all` requires the passphrase agent and refuses an interactive prompt | one prompt per device is error-prone in a batch |
| `--parallel N` is bounded 1 to 64 and defaults to 1 | sequential is the safe default |
| Batch init with a generated password prints it once to stdout | it is never stored in plaintext, and each device gets its own |
| The SSH upload transport is a replaceable function variable | command tests isolate transport, while transport tests use a real agent and SSH server |

<!-- source: internal/appliance/cmd_config_push.go -- sshExecFunc, configPushOne -->
<!-- source: internal/appliance/cmd_config_push_transport_test.go -- TestConfigPushSSHUsesRealAgentAndUploadSubsystem -->

## Live configuration upload

The host requests the `ze-config-upload` SSH subsystem after key authentication
and pinned host-key verification. The hub registers the subsystem on both daemon
startup paths. Before reading its body, the receiver authorizes the connection's
accepted identity for `api.ConfigAuthCommit`, with `readOnly=false`.
<!-- source: internal/appliance/cmd_config_push.go -- sshExecReal, applianceHostKeyCallback -->
<!-- source: cmd/ze/hub/service_ssh.go -- sshBuildImpl, sshBuildStandaloneImpl -->
<!-- source: cmd/ze/hub/config_upload_ssh.go -- registerSSHConfigUpload -->

The request starts with a four-byte unsigned big-endian length N, followed by
exactly N configuration bytes and SSH EOF. N must be between 1 and 4,194,304.
Short headers, truncated bodies, and trailing bytes are refused before validation
or publication. Success requires exact stdout `configuration applied\n` and SSH
exit status zero. Refusals do not echo configuration or validator input lines.
<!-- source: internal/core/configupload/configupload.go -- Read, Write, Receipt -->
<!-- source: internal/appliance/cmd_config_push.go -- sshExecReal -->
<!-- source: cmd/ze/hub/config_upload.go -- configMutationOwner.Upload -->

One daemon mutation owner admits SSH uploads and web source commits before store
access. Its starting state refuses operations until the runtime binds. Validation
and publication use the existing source-aware candidate, confirmed-window exclusion,
reload, rollback, and source promotion. HTTP retains its body adapter and validation
responses. SSH does not depend on the optional web service.
<!-- source: cmd/ze/hub/config_upload.go -- configMutationOwner, configMutationRuntime.commit -->
<!-- source: cmd/ze/hub/config_source.go -- commitRuntimeConfig, writeRuntimeConfig -->
<!-- source: cmd/ze/hub/service_web.go -- startWebServer -->

Runtime binding retains the daemon's confirmed-commit window through shutdown.
Withdrawing the public window does not remove this exclusion. A stopped window
refuses uploads and source commits while its pending rollback remains recorded.
The window can still use the admitted reload operation while its worker stops.
<!-- source: cmd/ze/hub/config_upload.go -- configMutationRuntime.window, configMutationRuntime.commit -->
<!-- source: cmd/ze/hub/confirm_window.go -- startConfirmWindow, stopConfirmWindow -->
<!-- source: internal/component/config/confirm/confirm.go -- Window.Stop, WriteOutside -->

Shutdown closes the mutation owner's admission and joins admitted operations before
store release. Each registered subsystem explicitly shares SSH session limits and
panic containment, because upstream subsystem dispatch bypasses Wish middleware.
A joined timeout worker interrupts blocked intake and output after 45 seconds by
closing the connection. Listener migration binds replacements and retains current
connections, so an upload can move its own listener without joining itself.
<!-- source: cmd/ze/hub/main.go -- runYANGConfig mutation owner and shutdown -->
<!-- source: internal/component/ssh/subsystem.go -- subsystemOptions, serveSubsystem, subsystemChannelOwner.run -->
<!-- source: internal/component/ssh/ssh.go -- Server.Reconfigure -->

The host bounds agent operations to ten seconds and the SSH exchange to sixty
seconds. Raw channel request and diagnostic readers are joined before returning.
Losing a receipt after delivery starts reports an uncertain outcome and prints a
command to inspect the running configuration. A live upload has no automatic
health-window rollback. The pushed-file boot monitor is a separate operation.
<!-- source: internal/appliance/cmd_config_push.go -- sshExecReal, sshUploadReceipt, uncertainConfigUpload -->
<!-- source: cmd/ze/health_revert.go -- HealthRevert.Start -->

## Constraint the code does not state

**Error messages differentiate an unreachable device from a protocol error.** A
`protocolError` type carries the second case, so a 401 does not report as
"unreachable". The first version labelled every failure "unreachable", and an
authentication failure read as a network problem.

<!-- source: internal/appliance/cmd_push.go -- protocolError, isProtocolError, mapUpdaterError -->

## Related

- `ota-push.md` for the image push protocol itself
- `device-config.md` for the device side of a config push
