# 394 -- zefs-socket-locking

## Objective

Replace three coordination mechanisms (Unix socket, flock/.lock files, PID files) with SSH as the single external interface to the daemon. The daemon is the single writer; no cross-process locking is needed.

## Decisions

- SSH replaces Unix socket as the only external interface -- SSH already existed with auth, YANG config, per-session CLI, and command dispatch
- `ze init` bootstraps the zefs database with SSH credentials (username, password, host, port) before any other command works
- CLI tools (config edit, signal) become SSH clients using env vars (`ze_ssh_host`, `ze_ssh_port`) for target resolution
- Editor without daemon starts an ephemeral daemon, connects via SSH, stops on exit (lf pattern)
- SSH YANG `listen` changed from leaf to leaf-list for multiple addresses
- Liveness detection by TCP dial to SSH port (replaces PID file + kill(0))
- In-process RWMutex kept for goroutine safety; flock removed entirely

## Patterns

- Single writer eliminates all locking -- if the daemon is the only process that writes, flock/.lock files solve a non-problem
- "Can this be done by removing something?" is a better first question than "what do we add?"
- The lf pattern is "one server process, clients connect" -- ze already had this with SSH
- SSH on localhost has negligible latency; do not invent alternative transports for local use

## Gotchas

- Spec went through three major revisions before reaching the simple design: v1 was a new RPC protocol with editor-as-server (massively overengineered), v2 added flock to socket file (still solving a non-problem), v3 realized SSH already exists
- The instinct to ADD coordination mechanisms is strong; ask "who writes?" first -- if one process, no locking needed
- Atomic rename (temp+rename flush) changes the inode, which breaks flock on the store file itself -- this is why the old design used a separate .lock file

## Files

- Created: `cmd/ze/init/main.go`, `cmd/ze/init/main_test.go`
- Deleted: `pkg/zefs/flock_unix.go`, `pkg/zefs/flock_other.go`, `internal/core/pidfile/pidfile.go`, `internal/core/pidfile/pidfile_test.go`
- Modified: `pkg/zefs/store.go`, `pkg/zefs/lock.go`, `internal/component/config/storage/storage.go`, `internal/component/plugin/server/server.go`, `internal/component/ssh/ssh.go`, `internal/component/ssh/schema/ze-ssh-conf.yang`, `internal/component/bgp/config/loader.go`, `cmd/ze/config/cmd_edit.go`, `cmd/ze/signal/main.go`, `cmd/ze/hub/main.go`
