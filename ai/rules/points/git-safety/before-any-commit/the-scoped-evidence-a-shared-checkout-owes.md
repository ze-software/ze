---
kind: directive
level: MUST
stage:
---
1. You MUST run the narrow gate owning each surface you changed (the table below).
2. You MUST run the tests of each package you touched, with `make ze-unit-pkg-test`.
3. You MUST ATTRIBUTE every red you saw: name the file, and say whether it is yours. `git
   status --porcelain` plus a modification time settles it in seconds. A red in a
   path your diff does not contain is not yours to chase.
4. You MUST prepare the script with `--unverified "<attribution>"`, giving the gates you ran
   and their verdicts, and naming the concurrent session's paths you excluded.
