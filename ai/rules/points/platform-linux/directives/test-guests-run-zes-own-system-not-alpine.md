---
kind: directive
level: MUST
stage:
---
**Every VM and test guest MUST run Ze's own system, its kernel and base as the appliance builds them, and MUST NOT be built on Alpine (owner decision M1, Thomas, 2026-10-20, `ai/META-DECISIONS.md`).** Where a guest still boots the Alpine image, a problem that exists only because of Alpine is out of scope: its fix is moving that guest to Ze's system. It MUST NOT be put to the owner as a question, and MUST NOT be worked around in a test, a harness or Ze's code. Until a guest moves, the runtime-kernel directive below still applies to it.
