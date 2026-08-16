---
kind: note
level:
stage:
---
There is no `pytest` and no `unittest discover` in this repo. A Python test that
nothing invokes never runs, and reads as coverage while providing none. Use one
of the two wired conventions, never a bare test file plus hope:
