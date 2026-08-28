---
kind: table
level:
stage:
---
| Change | Why agents need it |
|--------|--------------------|
| Changed user-facing or agent-facing behavior | Agents must know the behavior exists and where users or agents configure or invoke it |
| CLI command, RPC, MCP tool, YANG command, or API contract | Agents must discover the command shape, JSON contract, and wiring |
| Architecture contract, invariant, or documented data flow | Agents must find the current contract before changing or relying on it |
| Developer tool, native action, generator, or inventory command | Agents must know the tool exists before reimplementing it |
| Self-check, verification gate, hook, lint, or doc validator | Agents must run the right check and understand failures |
| Test runner, test format, fixture pattern, or required test category | Agents must place tests in the right suite and run the right target |
| Runtime dependency or readiness condition | Agents must verify the host with `ze doctor` before starting Ze |
| Recurring trap | Agents must find its journal record first, then any rule or gate that prevents it |
| New BGP family, SAFI, or capability | Agents must update migration schema, route converter, bridge, and compat tests (`ai/patterns/bgp-family.md`) |
| RFC-level protocol behavior added, changed, or newly proven | The standards ledger drives user and design decisions; a stale RFC status misleads both |
| Existing documentation made stale by the change | Agents must not discover an obsolete claim |
