# Feature Integration Completeness

**BLOCKING:** Every new feature MUST be proven to work integrated into the system — not just in isolation.

## Why This Exists

This rule prevents the "bridge to nowhere" pattern:

1. Spec builds new infrastructure (interfaces, APIs, config options, CLI flags, etc.)
2. Spec tests the infrastructure in isolation (compiles, unit tests pass)
3. Spec defers integration proof as "consumer responsibility" or "out of scope"
4. **Result:** Feature exists but nothing proves it actually works when used

This has happened before. It wastes time when the next spec discovers the plumbing is broken, unwired, or unreachable from outside.

## The Rule

**Every new feature MUST have at least one end-to-end test that proves the feature is reachable and functional from its intended usage point.**

"End-to-end" means: enter through the path a real user or consumer would use, exercise the feature, verify the outcome changes.

## Common Instances

| Feature Type | Isolation Test (insufficient alone) | Integration Test (required) |
|-------------|-------------------------------------|----------------------------|
| Injectable interface | `var _ Clock = RealClock{}` compiles | Inject fake, verify component uses it |
| New CLI flag | Flag parses correctly | Flag changes program behavior |
| New config option | Config loads without error | Option affects runtime behavior |
| New API/RPC | Handler returns correct response | Caller can reach handler through real transport |
| New event/hook | Event struct serializes | Event fires and subscriber receives it |
| New plugin capability | Plugin registers successfully | Engine dispatches to plugin correctly |
| Setter/injection point | Setter exists as a method | Something external calls the setter and behavior changes |
| New struct field | Field serializes/deserializes | Field is read somewhere and affects a decision |

## What "Integration" Means Per Feature Type

### Interfaces / Injection Points

At least one test must:
1. Create a fake/stub implementation
2. Inject it via the public API (setter, constructor, config)
3. Exercise the code that uses the interface
4. Verify the fake was called (behavior changed)

### CLI Flags / Config Options

At least one test must:
1. Set the flag/option to a non-default value
2. Run the code that reads the flag
3. Verify the output or behavior differs from default

### APIs / RPCs

At least one test must:
1. Call the API through the real transport (not just the handler function)
2. Verify the response
3. Functional test strongly preferred (`.ci` file)

### Events / Hooks

At least one test must:
1. Register a subscriber/handler
2. Trigger the event through normal operation (not direct call)
3. Verify the subscriber received the event

## How This Affects Spec Writing

### In Acceptance Criteria

At least one AC must test integration, not just isolation:

| Not Sufficient Alone | Required |
|---------------------|----------|
| "Interface compiles" | "Injected mock changes behavior" |
| "Flag parses" | "Flag affects output" |
| "Config loads" | "Config option changes runtime behavior" |
| "RPC handler works" | "RPC reachable through transport" |

**Integration ACs cannot be deferred.** They are the proof that the feature works.

### In TDD Test Plan

At least one test must exercise the feature from outside:

| Isolation (keep, but not enough) | Integration (required) |
|----------------------------------|----------------------|
| `TestRealClockNow` | `TestReactorWithFakeClock` |
| `TestFlagParse` | `TestFlagAffectsOutput` |
| `TestRPCHandler` | Functional test through real socket |

### Self-Check Question

Before marking a spec done, ask:

> "If I deleted all the new code except the tests, would any test fail because it tried to USE the feature through the intended path?"

If no test would fail — the feature is tested in isolation only and this rule is violated.

## When Deferral IS Acceptable

Deferral is acceptable for **advanced behavior**, not for **basic integration proof**:

| Deferrable | Not Deferrable |
|-----------|----------------|
| Full virtual clock with deterministic scheduler | One test injecting a fake clock |
| Comprehensive mock network with fault injection | One test injecting a fake dialer |
| Property-based testing with mock | One smoke test proving the wiring works |
| Performance benchmarks | One test proving the feature is reachable |

## Checklist (Add to Spec Goal Gates)

When a spec creates new features:

```markdown
### Goal Gates (MUST pass)
- [ ] Integration test: feature proven to work from intended usage point (not just isolation)
```

## Enforcement

Before marking a spec as "done":

1. For each new feature, find the test that exercises it from outside
2. If no such test exists, the spec is NOT complete
3. "Infrastructure ready; consumer tests deferred" is NOT acceptable for ALL tests
4. At minimum ONE integration test must exist per feature
