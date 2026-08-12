# Configuration Reload

Ze supports live configuration reload without restarting the daemon. Changed peers are updated, new peers are added, and removed peers are disconnected.
<!-- source: cmd/ze/hub/main_reload.go -- handleSIGHUPReload, doReload -->

## Triggering a Reload

```bash
ze signal reload                    # Via SSH command
kill -HUP $(pidof ze)               # Direct signal
```

## What Can Change Live

| Change | Effect |
|--------|--------|
| New peer added | Session initiated |
| Peer removed | Session torn down with NOTIFICATION |
| Peer settings changed | Session restarted with new config |
| Plugin config changed | Plugin reloaded |
| Static routes changed | New routes announced, old withdrawn |
| Capability changes | Session restarted to renegotiate |

## What Requires Restart

| Change | Why |
|--------|-----|
| BGP globals (`router-id`, `local { as }`) | Affects all peers, requires full restart |
| Hub listen address/port | Listener cannot be changed at runtime |
| SSH server settings | Server cannot be reconfigured live |

## Error Handling

If the new configuration fails to parse:
- The daemon continues running with the previous configuration
- An error is logged with details about the parse failure
- No peers are affected

The same applies when the configuration parses but a registered value validator
refuses one of its values (an IS-IS hostname outside 7-bit ASCII, an invalid
NET, an unregistered internal plugin name). The reload fails on the same branch,
the staged candidate is cleared, and the daemon keeps the configuration it is
already running. There is no override and no force flag. The message is
`reload error: reload: parse config: config validation failed: ...` and it names
the section, the leaf and the rule. See "Upgrading from a release that validated
only on demand" in `docs/guide/configuration.md`.
<!-- source: internal/component/config/loader.go -- LoadConfig refuses through ValidateCustomSections -->
<!-- source: cmd/ze/hub/main_reload.go -- runReload loadErr branch, before ReloadConfig -->

If a plugin reload fails:
- The daemon logs the error
- In-process BGP continues with the old plugin state

## Signals

| Signal | Effect |
|--------|--------|
| `SIGHUP` | Reload configuration (add/remove/update peers) |
| `SIGTERM` / `SIGINT` | Graceful shutdown (NOTIFICATION Cease to all peers) |
| `SIGUSR1` | Dump status to stderr |
<!-- source: cmd/ze/hub/main.go -- signal.Notify for SIGINT/SIGTERM/SIGHUP -->

A SIGTERM that arrives while a reload is running does not cut it short. Shutdown
waits up to 3 seconds for the reload to report `sighup reload complete` or
`reload error: ...`, so the answer to a SIGHUP is never lost with the process. A
reload still running after those 3 seconds is left behind, and the daemon prints
`shutdown: config reload still running after 3s, stopping without its result`.
<!-- source: cmd/ze/hub/main_reload.go -- awaitReloadWorker, reloadShutdownGrace -->
<!-- source: internal/component/bgp/reactor/signal.go -- SignalHandler, SIGUSR1 status dump -->

## Reload Workflow

1. Ze reads the config file from disk
2. Parses and validates against YANG schemas, then applies the registered
   `ze:validate` value validators. A refusal stops the reload here, before any
   diff is computed
3. Computes the diff between old and new config
4. For each removed peer: sends NOTIFICATION Cease, closes session
5. For each new peer: creates session, initiates connection
6. For each changed peer: tears down old session, starts new one
7. Plugins receive config-verify then config-apply callbacks
<!-- source: internal/component/bgp/config/resolve.go -- ResolveBGPTree -->
<!-- source: internal/component/bgp/reactor/config.go -- PeersFromTree, config diff -->

## Best Practices

- Always validate before reload: `ze config validate config.conf`
- Use `ze config diff old.conf new.conf` to preview changes
- Monitor `ze cli bgp monitor event state` during reload to watch sessions
