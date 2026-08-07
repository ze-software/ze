---
kind: directive
level:
stage:
---
**A NARROW FAILURE GETS A NARROW RE-RUN, NEVER A SECOND FULL PASS.** When the
end-of-development run comes back with one or two failing stages and you fix
exactly those, re-run the GATE THAT FAILED and the tests of the package you
touched. Twenty-three green stages that nothing has touched since do not become
more green by being run again.
