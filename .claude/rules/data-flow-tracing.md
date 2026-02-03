# Data Flow Tracing

**BLOCKING:** When reviewing or editing specs, ALWAYS trace the full data flow through the system.

## Why This Matters

Ze's architecture has distinct layers and boundaries:
- Engine ↔ Plugin communication (JSON over pipes)
- Wire bytes ↔ Parsed structures ↔ RIB storage
- Negotiated capabilities affect encoding/decoding

A change that looks correct in isolation may violate architectural boundaries or create impossible data paths.

## When to Trace Data Flow

| Action | Trace Required |
|--------|----------------|
| Writing a new spec | Yes |
| Reviewing a spec | Yes |
| Modifying existing spec | Yes |
| Adding "Files to Modify" | Yes |
| Proposing implementation approach | Yes |

## Data Flow Tracing Checklist

**BLOCKING:** Complete before approving any spec change.

```
[ ] 1. Identify entry points
      → Where does data enter? (wire bytes, API command, config, plugin message)
      → What format is it in at entry?

[ ] 2. Trace each transformation
      → Parsing/decoding: wire → struct
      → Validation: what checks occur?
      → Storage: how is it stored in pools/RIB?
      → Processing: policy, filtering, forwarding decisions
      → Encoding: struct → wire (for outbound)

[ ] 3. Verify boundary crossings
      → Engine ↔ Plugin: JSON format correct? (see `rules/json-format.md`)
      → FSM ↔ Reactor: event types match?
      → WireUpdate ↔ RIB: attribute references valid?
      → Negotiated caps ↔ PackContext: encoding context correct?

[ ] 4. Check for architectural violations
      → Does the change bypass intended layers?
      → Does it create coupling that shouldn't exist?
      → Does it duplicate existing functionality?
      → Does it break zero-copy semantics?
      → Does it violate pool/memory patterns?

[ ] 5. Verify integration points exist
      → Are all required functions/types already defined?
      → Do the signatures match what the change needs?
      → Will the change require modifying unrelated code?
```

## Ze Data Flow Reference

### Wire Bytes → RIB Storage

```
Wire bytes (TCP)
    ↓
BGP Message parsing (header → type dispatch)
    ↓
UPDATE: WireUpdate (lazy iterator, keeps wire refs)
    ↓
Attribute extraction (per-type iterators)
    ↓
Pool storage (dedup, ref-counted)
    ↓
RIB entry (NLRI → attribute refs)
```

### API Command → Wire Bytes

```
API command (text or hex)
    ↓
Command parsing (update/forward/withdraw)
    ↓
Attribute building (from text or raw hex)
    ↓
WireUpdate construction
    ↓
PackContext (negotiated caps for encoding)
    ↓
Wire bytes (to peer)
```

### Plugin ↔ Engine Communication

```
Engine event (BGP message received)
    ↓
JSON encoding (ze-bgp format, kebab-case)
    ↓
Write to plugin stdin
    ↓
Plugin processes, decides action
    ↓
Plugin writes command to stdout
    ↓
Engine parses command
    ↓
Engine executes (forward/withdraw/etc.)
```

## Questions to Answer

Before approving a spec, you MUST be able to answer:

1. **Entry:** "Where does this data come from?"
2. **Path:** "What happens to it at each stage?"
3. **Exit:** "Where does it go and in what format?"
4. **Boundaries:** "Which architectural boundaries does it cross?"
5. **Dependencies:** "What existing code does this interact with?"

## Red Flags

Stop and investigate if:

- The spec doesn't mention how data enters the system
- The spec skips transformation steps ("it just gets stored")
- The spec proposes direct access across layer boundaries
- The spec requires modifying core types for a specific feature
- The spec creates new JSON fields without checking `rules/json-format.md`
- The spec adds plugin communication without following the protocol

## Example: Good vs Bad Spec Review

### Bad (No Data Flow)

```markdown
## Files to Modify
- internal/rib/rib.go - add new field
```

**Problem:** Doesn't explain how data gets to RIB, what format, or how it's used.

### Good (Data Flow Traced)

```markdown
## Data Flow

1. **Entry:** UPDATE message with new attribute (wire bytes via TCP)
2. **Parsing:** Attribute iterator in `internal/bgp/attribute/` extracts type
3. **Storage:** Attribute pool deduplicates and stores with ref-counting
4. **RIB:** Entry stores reference to pooled attribute (not copy)
5. **Output:** JSON event to plugin includes attribute in `attr` object (kebab-case)

## Files to Modify
- internal/bgp/attribute/<type>.go - parse new attribute type
- internal/bgp/attribute/pool.go - add pool entry for new type
- internal/bgp/message/json.go - include in JSON output
```

## Integration with Spec Template

The spec template includes a "Data Flow" section. When reviewing:

1. Check that the section exists and is complete
2. Verify each step is accurate (read the source files)
3. Confirm boundaries are respected
4. Validate that integration points exist

## Common Architectural Boundaries

| Boundary | Allowed | Not Allowed |
|----------|---------|-------------|
| Engine → Plugin | JSON events | Direct function calls |
| Plugin → Engine | Text commands | Direct memory access |
| Wire → Storage | Via iterators/pools | Direct struct copy |
| RIB → Wire | Via PackContext | Ignoring negotiated caps |
| Config → Runtime | Via loader | Global state mutation |
