---
name: Do not touch files outside git control without authorization
description: Never overwrite, redirect into, or modify files that are not tracked by git without explicit user authorization
type: feedback
originSessionId: a4a7adb3-5066-4636-936f-6c5f9b766c22
---
Do not modify files that are not under git control without explicit authorization.

**Why:** Overwrote `docs/comparison.html` (gitignored, hand-crafted) with auto-generated output. The file was not tracked by git, which means it was deliberately excluded. Modifying it without authorization destroyed work with no way to recover via git.

**How to apply:** If a file is not tracked by git (gitignored or untracked), do not write to it, redirect into it, or modify it without asking first. Untracked files are outside the safety net of version control.
