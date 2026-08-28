---
kind: directive
level: MUST
stage:
---
- **A bare `git push` from a Bash call stays forbidden; the hook enforces it.**
- **You MUST push only with `./le commit create ... push "<owner authorisation>"`; the generated script performs it after every commit succeeds.**
- **The owner orders a push; you MUST NOT add `push` on your own initiative.**
- **`git push --force` and `-f` stay forbidden; the native route never forces.**
