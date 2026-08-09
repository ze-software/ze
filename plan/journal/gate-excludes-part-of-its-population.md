| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-09 | fixit-dead-design-pointers-in-tests | doc gates | `go_files()` excluded `_test.go`, so 133 dead `// Design:` spec pointers accumulated where no gate looked. Non-test files held zero, which is the control | widened the list, and refused the pointer class that closure deletes |
