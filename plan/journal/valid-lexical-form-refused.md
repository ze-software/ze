# A valid lexical form is refused

A value has more than one representation permitted by its specification, but
validation or canonical comparison accepts only a subset. Tests of the accepted
forms can leave that missing representation invisible.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-12 | spec-config-enum-refusal-proposes-valid-values (found) | YANG `validateUnsigned`/`canonicalValue` | Source-read, unexecuted: RFC 7950 9.2.1 permits either sign, but unsigned `-0` reaches `ParseUint` unchanged; 9.1-1 tags include declaration loading and missing-target checks rather than whole lexical proof | open: post-release spec-config-yang-reference-types reproduces and normalizes minus-zero, retains negative-nonzero refusal, adds direct lexical proofs and rejudges 9.1-1 |
