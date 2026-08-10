---
kind: directive
level: MUST
stage:
---
- systemd (you MUST use Alpine's OpenRC or skip systemd-specific tests)
- Physical serial ports (you MUST use PTY pairs)
- Multiple physical NICs (you MUST use veth pairs)
- GPU or display (tests are headless)
- Persistent state between runs (boots fresh from ISO each time)
