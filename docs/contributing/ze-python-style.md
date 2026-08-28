# Python References

Python remains relevant when Ze documents or interoperates with an external
Python program, including ExaBGP and vendor tools. Keep those facts and examples
accurate to the external project.

Repository development commands, generators, hooks, fixture drivers, and test
runners are compiled Go. New repository automation belongs in
`internal/le/<area>/` and is exposed through `./le <area> <verb>`. Go fixture
drivers live under `internal/test/fixture`, and package-local fixtures live
under the owning package's `testdata/` directory.

Do not add a Python launch instruction for first-party development or testing.
