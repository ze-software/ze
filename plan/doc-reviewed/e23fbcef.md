# Reviewed claims: session e23fbcef

Each row records a documentation claim compared with a change that preserves
the behavior the claim states. The ledger contract is in
`docs/contributing/documentation-testing.md`.

| Doc | Source | Symbol | Commit | Reason |
|-----|--------|--------|--------|--------|
| `docs/contributing/terminal-demos.md` | `internal/test/runner/server_process.go` | `ServerProcess.Stop` | `12afa215f6` | Stop delegates to the shared stop helper with wholeGroup false. The daemon still receives leader TERM, the caller's ten-second grace, owned group cleanup and a sole-reaper join before store removal. Cleanup errors remain joined with the browser failure. |
| `docs/guide/appliance.md` | `internal/le/test/qemu/dockerlab.go` | `runDockerLab` | `934e7d8c03` | The VPN proof still supplies only lab and timeout, so mode validation accepts it and it retains the native image, hardware-only Docker branch and compiled boot preparation. The added command branch is selected only by a command argument absent from this proof. |
| `docs/guide/appliance.md` | `internal/le/test/qemu/dockerlab.go` | `dockerLabGuest.cidataFiles` | `934e7d8c03` | The VPN proof has no explicit command kernel, so command mode is false. It retains the original Docker boot script with vpnGuestPreparation prepended before services, including the disposable guest module restrictions; UID and no-Docker changes apply only to command mode. |
