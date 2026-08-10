---
kind: directive
level: MUST
stage:
---
**A NARROW FAILURE MUST GET A NARROW RE-RUN; IT MUST NOT GET A SECOND FULL PASS.** When the
end-of-development run comes back with one or two failing stages and you fix
exactly those, you MUST re-run the GATE THAT FAILED and the tests of the package you
touched. Twenty-three green stages that nothing has touched since do not become
more green by being run again.
