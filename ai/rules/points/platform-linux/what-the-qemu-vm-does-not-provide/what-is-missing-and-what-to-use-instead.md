---
kind: directive
level:
stage:
---
- systemd (use Alpine's OpenRC or skip systemd-specific tests)
- Physical serial ports (use PTY pairs)
- Multiple physical NICs (use veth pairs)
- GPU or display (tests are headless)
- Persistent state between runs (boots fresh from ISO each time)
