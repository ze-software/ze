# plan/pre-release/ -- The Release Cannot Go Out Without It

A spec here breaks nothing an operator can reach. It still holds the release,
because the release is more than the daemon: it is a package somebody installs, an
appliance that boots, a document somebody reads, and the evidence Ze publishes
about what it conforms to.

## What goes here

- Packaging and distribution: signed artifacts, repositories, version identity.
- Install and appliance boot, and the QEMU proof that both work.
- Onboarding documentation, and the audit that says every user-visible surface
  has one.
- RFC conformance evidence Ze owes a reader outside this repository: extraction
  sign-off, non-unit proof, and the ledger that publishes the verdict.
- The release evidence gate itself.

## What does NOT go here

A defect an operator meets goes to `plan/immediate/`, however small. Test tooling
that only makes development faster goes to `plan/`: the test is whether the
release can ship without it, not whether a developer wants it.

## Lifecycle

Same header table, same statuses, and the same two-commit closure as `plan/`
(`plan/README.md`).
