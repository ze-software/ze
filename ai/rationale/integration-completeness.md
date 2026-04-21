# Integration Completeness Rationale

Why: `ai/rules/integration-completeness.md`

## The "Bridge to Nowhere" Pattern

1. Spec builds new infrastructure
2. Spec tests infrastructure in isolation (compiles, unit tests pass)
3. Spec defers integration as "consumer responsibility"
4. Result: feature exists but nothing proves it works when used

## Examples

| Feature Type | Isolation (insufficient alone) | Integration (required) |
|-------------|-------------------------------|----------------------|
| Interface | `var _ Clock = RealClock{}` compiles | Inject fake, verify component uses it |
| CLI flag | Flag parses | Flag changes behavior |
| Config option | Config loads | Option affects runtime |
| API/RPC | Handler returns response | Caller reaches handler through transport |
| Event/hook | Event serializes | Event fires, subscriber receives |

## Deferral Boundary

Deferrable: full virtual clock, comprehensive mock network, property-based testing, benchmarks.
NOT deferrable: one test proving the wiring works.
