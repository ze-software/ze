---
kind: note
level:
stage:
---
First run downloads Alpine ISO + Go toolchain (~1 min). Subsequent runs
reuse the cache in `tmp/qemu/` (~30s boot + test time).
