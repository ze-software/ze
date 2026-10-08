# Root (`sudo`) in agent sessions

Owner decision, 2026-10-08.

An agent MAY run `sudo` only for the stress harness
(`./le test integration stress`, which needs root for network namespaces).
Every other use of `sudo` stays forbidden.

| Condition | Why |
|-----------|-----|
| One root run at a time, never alongside another test run, VM or Docker lab | A root run shares ports, namespaces and lock directories with every other run |
| The run removes what it created (namespaces, temp files) and reports anything it could not remove | Leftover namespaces broke later runs |
| Anything it creates in a shared location (`/tmp/le-port-locks`, `tmp/`) stays usable by the owner's user | A root-created `/tmp/le-port-locks` locked every later run out (2026-10-07) |
| Measurement runs only over committed code with no foreign uncommitted changes under `internal/component/bgp` or `internal/core` | A profile of in-flight code is not evidence |
