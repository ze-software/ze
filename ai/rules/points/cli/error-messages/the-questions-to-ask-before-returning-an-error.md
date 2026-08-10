---
kind: directive
level: MUST
stage:
---
MUST ask these questions before returning an error:

1. Does it name the specific subject (path/key/field/value), not just the operation?
2. Could a reader who has never seen this code take the next step from this line alone?
3. If the next step needs more than one line, is there a diagnostic code carrying it?
4. Is the leading phrase stable and greppable, or did I reword a shared failure?
