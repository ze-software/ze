# Lifecycle, Rollback, and Recovery

Use this guide for routine daemon control and configuration recovery. Ze separates configuration reload, process restart, and host reboot because they have different routing consequences.

## Before first start

```console
ze init
ze config validate ze.conf
ze doctor
ze start ze.conf
```

`ze init` creates the local credential and blob database. Validation checks the candidate configuration without applying it. `ze doctor` checks host capabilities and dependencies. See [Operations](operations.md) and [Installation](ze-install.md) for systemd and appliance-specific setup.

## Choose the right action

| Intent | Action | Routing effect |
|--------|--------|----------------|
| Apply a validated configuration change | `ze signal reload` | Transactional subsystem reload, with the current config retained on failure |
| Stop normally | `ze signal stop` | Graceful shutdown, peers receive a normal teardown |
| Restart with BGP GR signalling | `ze signal restart` | Writes the restart marker used by the Graceful Restart path |
| Inspect process state | `ze signal status` | No configuration or routing change |
| Capture goroutines and exit | `ze signal quit` | Diagnostic termination, not a routine stop |
| Check liveness | `ze status` | TCP liveness check only |

A failed reload clears the candidate and keeps the previous active configuration. Do not replace reload with a process restart merely to apply a normal edit.

## Review and commit configuration

The CLI and web editor maintain a candidate separate from the active configuration:

1. Enter an edit session.
2. Make the smallest change.
3. Validate and review the diff.
4. Commit.
5. Verify live state and health.

Use `show config dump` for the fully resolved configuration and `show config diff` before rollback or commit.

## Archive and rollback

Ze stores configuration revisions in the configured storage backend. The archive guide covers revision pointers, named archive destinations, schema evolution, and recovery.

Useful commands include:

Copy the snapshot key returned by `show config history` before running `cat`:

```console
ze cli -c "show config history"
ze cli -c "show config diff"
ze cli -c "show config cat <snapshot-key>"
ze cli -c "request config archive"
```

See [Configuration archive](config-archive.md) for the current archive and rollback workflow. Review the selected revision before moving the active pointer. A syntactically valid historical revision can still be unsuitable after a schema or topology change.

## Logging and reports

Start with bounded operational views:

```console
ze cli -c "show warnings"
ze cli -c "show errors"
ze cli -c "show event recent"
ze cli -c "show health"
```

Raise a subsystem's log level only while investigating:

```console
ze cli -c "request log level bgp.fsm debug"
```

Persistent log configuration and destinations are described in [Logging](logging.md). Use structured command output for automation rather than scraping human log messages.

## Software updates

The update mechanism is platform-aware. Standard Linux, systemd, and appliance deployments do not all use the same backend. Follow [Self update](self-update.md), then decide whether the BGP restart path is required for the maintenance window.

Before updating:

1. Archive the active configuration.
2. Confirm the current version and health.
3. Confirm peers negotiated the resilience mechanisms the maintenance plan assumes.
4. Record peer and RIB counts.
5. Keep the previous binary or appliance slot available for rollback.

After updating:

1. Confirm the process is running.
2. Check warnings, errors, and component health.
3. Compare peer and RIB counts with the pre-change snapshot.
4. Confirm stale routes were purged and no replay remains in progress.
5. Exercise one representative management path.

## Recovery order

When a change fails, recover in this order:

1. Preserve warnings, errors, events, and crash output.
2. Determine whether the failure is configuration, one subsystem, or the process.
3. Discard or roll back the candidate when configuration caused the failure.
4. Reload if the process remains healthy.
5. Restart only when process state must be rebuilt.
6. Reboot the host only for kernel, dataplane, or appliance failures that require it.

See [Production diagnostics](production-diagnostics.md) for symptom-led checks and support bundle collection.

## Related guides

- [Operations](operations.md)
- [Configuration archive](config-archive.md)
- [Logging](logging.md)
- [Self update](self-update.md)
- [BGP resilience](bgp-resilience.md)
- [Production diagnostics](production-diagnostics.md)
