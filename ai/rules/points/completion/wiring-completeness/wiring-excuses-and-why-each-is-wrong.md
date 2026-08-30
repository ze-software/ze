---
kind: directive
level: MUST NOT
stage:
---
**These MUST NOT be offered for an unwired feature:**

| Pattern | Why it's wrong |
|---------|----------------|
| "The caller will wire it later" | Later never comes. Other sessions see it as done. |
| "It's available for callers" | Available is not wired. No caller means no effect. |
| "The review said NOTE" | A review flags unwired code as BLOCKER. |
| "The web UI doesn't need it" | If the feature produces data a UI page renders, the UI has to show it. |
