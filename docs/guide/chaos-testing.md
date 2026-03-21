# Chaos Testing

Ze includes a chaos testing mode that injects faults during operation to verify the daemon handles failures correctly. This is useful for validating configuration changes, testing plugin resilience, and finding edge cases before production.

## Quick Start

```bash
# Run all chaos tests
make ze-chaos-test

# Start ze with chaos mode enabled
ze --chaos-seed 42 --chaos-rate 0.1 config.conf
```

## Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--chaos-seed <N>` | PRNG seed for reproducible faults. `-1` = time-based. `0` = disabled. | 0 (off) |
| `--chaos-rate <f>` | Probability of fault per operation (0.0 to 1.0) | 0.1 |

## ze-chaos Tool

The `ze-chaos` tool is a chaos simulator that runs multiple BGP peers against a ze route server, validates route propagation, and injects faults.

```bash
# Pipeline mode: config on stdout, diagnostics on stderr
ze-chaos --ze ./bin/ze --seed 42 --peers 8 --duration 60s | ./bin/ze -

# Write config to file
ze-chaos --config-out chaos.conf --seed 42 --peers 8
ze chaos.conf

# In-process mode: mock network + virtual clock (fully deterministic)
ze-chaos --in-process --seed 42 --duration 30s

# Multi-family
ze-chaos --families ipv4/unicast,ipv6/unicast --chaos-rate 0.2 | ./bin/ze -
```

### Event Logging and Replay

```bash
# Record events
ze-chaos --event-log run.ndjson --seed 42 | ./bin/ze -

# Replay a recorded failure
ze-chaos --replay run.ndjson

# Shrink to minimal reproduction
ze-chaos --shrink run.ndjson
```

### Property Validation

```bash
ze-chaos --properties all --convergence-deadline 5s | ./bin/ze -
ze-chaos --properties list    # Show available properties
```

Properties validated:
- **Convergence:** All routes reach all peers despite faults
- **State consistency:** RIB matches peer state after recovery
- **No data loss:** Updates eventually delivered
- **Graceful degradation:** Sessions restart on critical faults

## Fault Types

| Category | Examples |
|----------|---------|
| Network I/O | Connection failures, partial writes, read timeouts |
| Wire parsing | Malformed messages, truncated packets |
| FSM | State machine violations, invalid transitions |
| Route processing | Dropped updates, corrupted NLRI |
| Event delivery | Lost messages, out-of-order events |

## Deterministic Replay

With a fixed seed, chaos testing is fully reproducible:

```bash
# These two runs produce identical fault sequences
ze --chaos-seed 42 --chaos-rate 0.1 config.conf
ze --chaos-seed 42 --chaos-rate 0.1 config.conf
```

Seed `0` disables chaos entirely (zero overhead). Seed `-1` uses the current time (non-reproducible).

## Make Targets

| Target | Description |
|--------|-------------|
| `make ze-chaos-test` | Run chaos unit + functional tests |
| `make ze-chaos` | Build ze-chaos binary |

## When to Use

- **Before deployment:** Validate your config handles peer failures
- **CI pipeline:** Catch race conditions and edge cases
- **Debugging:** Reproduce intermittent failures with a fixed seed
- **Benchmarking:** Measure convergence time under fault conditions
