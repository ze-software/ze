---
kind: note
level:
stage:
---
Do not add a `*_test.py` outside a directory covered by one of the above without
wiring it, and do not "fix" a discovery glob by replacing it with a hardcoded list:
the glob is what stops the next file from rotting.
