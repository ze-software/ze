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
| The SSH exec is a replaceable function variable | network operations stay testable with no real SSH |

## Constraint the code does not state

**Error messages differentiate an unreachable device from a protocol error.** A
`protocolError` type carries the second case, so a 401 does not report as
"unreachable". The first version labelled every failure "unreachable", and an
authentication failure read as a network problem.

<!-- source: internal/appliance/cmd_push.go -- protocolError, isProtocolError, mapUpdaterError -->

## Related

- `ota-push.md` for the image push protocol itself
- `device-config.md` for the device side of a config push
