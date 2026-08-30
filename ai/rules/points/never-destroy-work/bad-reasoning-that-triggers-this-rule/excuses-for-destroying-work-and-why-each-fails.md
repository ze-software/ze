---
kind: directive
level: MUST NOT
stage:
---
**You MUST NOT destroy work on any of this reasoning:**

| Excuse | Why it fails |
|--------|-------------|
| "Lint is blocking" | Hooks are advisory. The file's contents are orthogonal to whether a hook passes. |
| "It'll be rewritten anyway" | The user might want to diff the current version, or have a different plan than you assume. |
| "I just wrote it one turn ago" | It does not matter. You wrote it at their direction; it is theirs now. |
| "It's clearly an error, dead code, or scaffolding" | Ask. "Clearly" is often wrong. |
| "Reverting my own mistake" | When deletion is the correct fix and the file is user-visible, ask for permission instead of leaving it behind. |
| "The tree is in a broken state" | Tree-broken is preferable to work-lost. Tell the user; let them decide. |
