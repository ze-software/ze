# Configuration Reload

Ze supports live configuration reload without restarting the daemon. Changed peers are updated, new peers are added, and removed peers are disconnected.

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
| BGP globals (`router-id`, `local-as`) | Affects all peers, requires full restart |
| Hub listen address/port | Listener cannot be changed at runtime |
| SSH server settings | Server cannot be reconfigured live |

## Error Handling

If the new configuration fails to parse:
- The daemon continues running with the previous configuration
- An error is logged with details about the parse failure
- No peers are affected

If a plugin reload fails:
- The daemon logs the error
- In-process BGP continues with the old plugin state

## Signals

| Signal | Effect |
|--------|--------|
| `SIGHUP` | Reload configuration (add/remove/update peers) |
| `SIGTERM` / `SIGINT` | Graceful shutdown (NOTIFICATION Cease to all peers) |
| `SIGUSR1` | Dump status to stderr |

## Reload Workflow

1. Ze reads the config file from disk
2. Parses and validates against YANG schemas
3. Computes the diff between old and new config
4. For each removed peer: sends NOTIFICATION Cease, closes session
5. For each new peer: creates session, initiates connection
6. For each changed peer: tears down old session, starts new one
7. Plugins receive config-verify then config-apply callbacks

## Best Practices

- Always validate before reload: `ze config validate config.conf`
- Use `ze config diff old.conf new.conf` to preview changes
- Monitor `ze cli bgp monitor event state` during reload to watch sessions
