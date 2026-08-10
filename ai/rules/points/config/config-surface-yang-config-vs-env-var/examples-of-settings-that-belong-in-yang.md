---
kind: directive
level: SHOULD
stage:
---
**Settings in these categories SHOULD live in YANG config:**
- Queue depths, buffer sizes, batch limits, pool budgets
- Timers that affect convergence or session behavior
- Feature toggles that change observable routing behavior
- Capacity knobs (max peers, max prefixes, max routes)
- Any setting an operator would document in a change ticket
