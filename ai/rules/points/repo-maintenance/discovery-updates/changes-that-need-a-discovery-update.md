---
kind: table
level:
stage:
---
| Change | Why agents need it |
|--------|--------------------|
| User-facing feature | Agents must know the feature exists and where users configure or invoke it |
| CLI command, RPC, MCP tool, YANG command, or API contract | Agents must discover the command shape, JSON contract, and wiring |
| Developer tool, script, make target, generator, or inventory command | Agents must know the tool exists before reimplementing it |
| Self-check, verification gate, hook, lint, or doc validator | Agents must run the right check and understand failures |
| Test runner, test format, fixture pattern, or required test category | Agents must place tests in the right suite and run the right target |
| Runtime dependency or readiness condition | Agents must verify the host with `ze doctor` before starting Ze |
| Structural decision, repeated gotcha, or workflow change | Agents must find it through the learned index or a rule before repeating the mistake |
| New BGP family, SAFI, or capability | Agents must update migration schema, route converter, bridge, and compat tests (`ai/patterns/bgp-family.md`) |
| RFC-level protocol behavior added, changed, or newly proven | The standards ledger drives user and design decisions; a stale RFC status misleads both |
