---
kind: directive
level: MUST
stage:
rationale: plan/journal/green-that-could-not-have-been-red.md
---
**A Python test file MUST print unittest's `Ran N tests` line, and that N MUST
equal the number of `def test...` it declares.** `TestPythonUnitTests` compares the
two counts and fails the file by name when they differ, in either direction, and
when the run prints no count at all. It compares counts, never names, so a file that
declares two cases and runs one of them twice passes. An exit code alone cannot tell
"every test passed" from "no test ran".

**A file the glob matched that declares no case fails too**, so a helper named
`<name>_test.py` or `test_<name>.py` MUST be renamed or MUST offer a case. A file
whose run legitimately reaches MORE cases than it declares, a mixin base class or a
case built at run time, MUST say so with a `# python-tests: generated-cases: N`
comment line, where N MUST equal the count the run reports. The count keeps the
marker a statement about one run: a marker that states none, or one no integer can
hold, fails the file rather than permitting every raised count. N is checked
whenever the marker is present, so a marker on a file whose run matches its
declared count fails when N disagrees with both.

**pytest is not installed here, so a pytest fixture in a signature is a case that
never runs.** Write unittest cases and end the file with
`if __name__ == "__main__": unittest.main()`. Running fewer cases than declared
also catches two classes with one name, two methods with one name, and a case
outside any `TestCase`.
