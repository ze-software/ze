# No Layering

**BLOCKING:** When asked to replace X with Y:
1. DELETE X first
2. Then implement Y
3. Never keep both

"Integration" means REPLACEMENT, not ADDITION.

## Forbidden patterns
- "Keep old system, add new alongside"
- "Hybrid approach for safety"
- "Gradual migration"
- "Fallback to old system"

## Required behavior
- Delete old code BEFORE writing new
- If deletion breaks things, fix the breakage
- No "temporary" compatibility code
- Ask "am I adding or replacing?" before every change

## Call out
If Claude proposes keeping both systems, user should say:
"You're layering. Delete the old one."
