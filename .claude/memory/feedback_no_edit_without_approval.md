---
name: No edits without approval during design discussion
description: When discussing design alternatives, present the proposal and wait. Never edit files until the user explicitly approves.
type: feedback
---

Do not edit files during a design discussion. When the user asks "what keyword could we use?" or similar design questions, present options and wait for explicit approval before touching any files.

**Why:** The user was exploring alternatives for the `add` keyword in `set bgp peer`. I proposed dropping it and immediately edited the YANG file without waiting. The user said "do not change things without my authorisation."

**How to apply:** During any conversation about alternatives, naming, or design: present a table of options, explain trade-offs, and STOP. Only edit after the user says "do that" or equivalent.
