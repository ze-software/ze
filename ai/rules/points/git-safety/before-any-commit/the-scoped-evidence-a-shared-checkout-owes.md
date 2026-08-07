---
kind: directive
level:
stage:
---
1. Run the narrow gate owning each surface you changed (the table below).
2. Run the tests of each package you touched, with `make ze-test-pkg`.
3. ATTRIBUTE every red you saw: name the file, and say whether it is yours. `git
   status --porcelain` plus a modification time settles it in seconds. A red in a
   path your diff does not contain is not yours to chase.
4. Prepare the script with `--unverified "<attribution>"`, giving the gates you ran
   and their verdicts, and naming the concurrent session's paths you excluded.
