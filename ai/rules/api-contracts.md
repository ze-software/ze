# API Contracts in Comments

**BLOCKING:** When authoring functions with caller obligations, document them in the godoc.
Rationale: `ai/rationale/api-contracts.md`

## When to Document

Any function where skipping a step causes a resource leak, deadlock, panic, or silent misbehavior.

| Pattern | Required Comment |
|---------|-----------------|
| Start/Stop/Wait lifecycle | Type doc: full sequence. Stop: "MUST call Wait after". Wait: "Must be called after Stop". |
| Close/cleanup required | "Caller MUST call Close when done" on the constructor |
| Init before use | "MUST call Init before first use" on the type or constructor |
| Call ordering | "MUST be called before/after X" on the dependent function |
| Concurrency safety | "Safe for concurrent use" or "NOT safe for concurrent use" |
| Paired operations (Lock/Unlock, Acquire/Release) | "Caller MUST call Y after X" on X |

## Format

Use "MUST" (not "should") for obligations that cause bugs when violated.
Place the obligation on both sides of the pair: the function that creates the obligation AND the function that fulfills it.

## Checklist (before merging new API)

```
[ ] Every resource-acquiring function names how to release it
[ ] Every multi-step lifecycle is documented on the type
[ ] Every "call B after A" appears in both A's and B's comments
```
