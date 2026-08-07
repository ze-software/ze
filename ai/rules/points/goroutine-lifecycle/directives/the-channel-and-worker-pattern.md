---
kind: note
level:
stage:
---
Pattern: channel + worker. Start → create chan + start worker. Hot path → enqueue. Stop → close chan.
