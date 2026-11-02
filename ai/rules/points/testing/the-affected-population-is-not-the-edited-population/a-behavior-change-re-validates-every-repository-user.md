---
kind: directive
level: MUST
stage:
---
- **A change to what Ze does (a default, an enforcement, a refusal, an output format) MUST convert or re-validate, in the same work, every user of that behavior the repository carries: configs, `.ci` bodies, fixtures, benchmarks, interop scenarios, pinned digests and docs.** A user left on the old contract fails in one of two ways: it goes red where nobody looks, or it stays green because the new behavior removed what it checked, such as a test that passes only because its routes are now dropped, or a benchmark that times a selection of zero.
- **For configs, `TestRepositoryConfigsValidate` (`internal/component/config/cli/validate_corpus_test.go`) is the mechanism: it validates every Ze config the repository carries, and it MUST be run when validation tightens.** It does not read what a test expects, so the `.ci` expectations, benchmarks, `needs-linux` suites, interop checkers and pinned digests that use the behavior MUST be found by search and run.
