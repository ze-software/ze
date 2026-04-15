---
paths:
  - "plan/**"
  - "tmp/session/selected-spec"
---

# Data Flow Tracing

**BLOCKING:** Trace full data flow before writing or reviewing specs.
Rationale: `.claude/rationale/data-flow-tracing.md`

## Checklist

```
[ ] 1. Entry points — where does data enter? (wire, API, config, plugin) What format?
[ ] 2. Transformations — parse → validate → store → process → encode
[ ] 3. Boundary crossings:
      Engine ↔ Plugin (JSON over pipes)
      FSM ↔ Reactor (event types)
      WireUpdate ↔ RIB (attribute refs)
      Caps ↔ PackContext (encoding context)
[ ] 4. Violations? Bypassed layers? Unintended coupling? Duplicated functionality? Broken zero-copy?
[ ] 5. Integration points exist? Signatures match? Unrelated code needs changes?
```

## Reference Flows

- **Wire → RIB:** TCP → message parse → UPDATE (WireUpdate, lazy iterator) → attribute extraction → pool dedup → RIB entry (NLRI → attr refs)
- **API → Wire:** command parse → attribute building → WireUpdate → PackContext → wire bytes
- **Plugin ↔ Engine:** event → JSON encode → write stdin → plugin processes → write stdout command → engine parses → execute

## Must Answer Before Approving Spec

1. Where does data come from?
2. What happens at each stage?
3. Where does it go and in what format?
4. Which boundaries does it cross?
5. What existing code does it interact with?
