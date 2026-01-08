# RFC Implementation Summary

## Instructions
1. Use ULTRATHINK for deep analysis
2. READ: rfc/$ARGUMENTS.txt
3. WRITE: .claude/zebgp/rfc/$ARGUMENTS.md
4. CHECK errata: https://www.rfc-editor.org/errata/rfcNNNN
5. VERIFY: Re-read RFC and summary, check:
   - ALL wire formats captured with ASCII diagrams?
   - ALL MUST requirements listed?
   - ALL error conditions documented?
   - Key constants/type codes present?
   - Quoted requirements match RFC exactly?
   - Section references correct?
   - Field sizes/offsets accurate?

## Structure

### Meta
- RFC title, number, status, date
- Obsoletes / Updates / Obsoleted-by
- Purpose (1-2 sentences)
- Scope: AFI/SAFI if BGP extension

### Wire Formats
For EACH format defined (message/attribute/capability/NLRI):

```
#### <Name> Format
Type code: X (if applicable)
Flags: O=? T=? P=? E=? (for attributes)

<ASCII diagram verbatim>

| Field | Offset | Size | Type | Constraints |
|-------|--------|------|------|-------------|

Length calculation: <formula>
Parse order: <if non-obvious>
```

### Encoding Rules
- Byte ordering (exceptions to network order)
- Variable-length field encoding
- Optional field presence rules
- Padding requirements

### Decoding Rules
- Parse sequence
- How to determine field boundaries
- Unknown/unsupported value handling

### Validation
| Check | Valid | Invalid Action |
|-------|-------|----------------|

### MUST Requirements
Group by:
- **Tx**: What sender MUST do
- **Rx**: What receiver MUST do
- **Validation**: What MUST be checked
- **Errors**: How MUST respond
- **State**: State machine MUSTs
- **Timers**: Timing MUSTs

### SHOULD/MAY
- [SHOULD] requirement - <consequence if not>
- [MAY] option - <when to use>

### Error Handling
| Condition | Detect How | Response | Code/Subcode |
|-----------|------------|----------|--------------|

### State Machine
| State | Event | Guard | Action | Next |
|-------|-------|-------|--------|------|

### Timers
| Name | Default | Range | Behavior |
|------|---------|-------|----------|

### Constants
| Name | Value | Usage |
|------|-------|-------|

### Algorithms
Step-by-step, pseudocode if RFC provides it.

### Pitfalls
- **Edge cases**: <specific scenarios>
- **Interop**: <known issues with other implementations>
- **Security**: <attack vectors, mitigations>

### Compatibility
- Behavior with non-supporting peers
- Feature negotiation fallback

## Rules
- MUST read file, never from memory
- ASCII diagrams: copy EXACTLY (spacing matters for field boundaries)
- Requirements: quote verbatim, cite section number
- Tables: prefer over prose for structured data
- Skip sections that don't apply (no empty sections)
- Skip: abstract, introduction (unless defines terms), acknowledgments, full IANA section (keep just the values)
- Errata: note any that affect implementation
