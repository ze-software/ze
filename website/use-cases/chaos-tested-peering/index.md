# Chaos-Tested BGP Peering

Use `ze-chaos` before deployment to prove that a peer configuration converges, survives selected faults, and produces a reproducible failure record when it does not.

## Start from a validated peer

Build the intended peer or group with the [BGP peering guide](../../guides/bgp-peering/). Validate the configuration and run ordinary interop or lab checks first. Chaos testing is useful after the normal path works, not as a substitute for basic configuration validation.

## Run one complete reproducible scenario

Use a fixed seed and keep the generated configuration beside the event log.
This sequence first makes the configuration inspectable, validates it, then
runs the same four-peer shape:

```console
mkdir -p evidence/seed-42
ze-chaos --config-only --seed 42 --peers 4 --config-out evidence/seed-42/ze.conf
ze config validate evidence/seed-42/ze.conf
ze-chaos --event-log evidence/seed-42/run.ndjson --seed 42 --peers 4 --duration 60s
```

For a deterministic in-process run that also varies route activity:

```console
ze-chaos --in-process --event-log evidence/seed-42/in-process.ndjson --seed 42 --duration 60s --chaos-rate 0.1 --route-rate 0.05
```

Replay the captured run before accepting the result:

```console
ze-chaos --replay evidence/seed-42/run.ndjson
```

If replay preserves a failure, reduce it to the smallest event sequence:

```console
ze-chaos --shrink evidence/seed-42/run.ndjson
```

Keep `ze.conf`, the event logs, the seed, the Ze version, and the final command
output as one evidence bundle. See the
[chaos testing guide](../../guides/chaos-testing/) for pipeline, FRR, BIRD,
and dashboard modes.

## State the properties

A useful scenario has explicit properties rather than a general expectation that the daemon remains running. Typical properties include:

- all intended sessions eventually establish;
- every announced route reaches every eligible destination;
- withdrawals remove the route everywhere;
- a restarted session converges before its deadline;
- no route crosses a policy or family boundary;
- the final RIB agrees with the final peer state;
- warning and error output contains no unexplained event.

Choose deadlines from the intended timers and route volume. A deadline copied from a small lab is not evidence for a full table.

## Inject meaningful faults

Select faults that exercise the deployment's real risks:

- connection failure and reconnect;
- partial reads or writes;
- session reset during an update stream;
- route churn during restart;
- slow consumers and backpressure;
- malformed messages on an untrusted peer;
- plugin delay or failure where policy depends on a plugin.

Run one fault class at a time before combining them. This keeps failures attributable and makes shrinking more effective.

## Replay and shrink

When a scenario fails:

```console
ze-chaos --replay run.ndjson
ze-chaos --shrink run.ndjson
```

Replay proves the failure is deterministic. Shrink removes unrelated events until the smallest useful reproduction remains. Keep the seed, event log, configuration, binary version, and final report together.

## Deployment gate

Before accepting the peer configuration:

1. The ordinary session and route checks pass.
2. Every declared chaos property passes for repeated fixed seeds.
3. A deliberately broken configuration or handler makes the relevant property fail.
4. Replay reproduces a captured failure.
5. The final route and peer state is clean after recovery.
6. The test records the Ze version and scenario inputs.

A scenario that remains green when the protected behaviour is broken is observation, not proof.
