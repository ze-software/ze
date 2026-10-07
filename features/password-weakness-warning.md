# Password Weakness Warning

## Meta

| Field | Value |
|-------|-------|
| Name | Password Weakness Warning |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/config/password_strength.go, internal/component/config/password_hash.go, internal/plugins/passwd |
| Real-path tests | test/parse/password-weakness-warning.ci |
| Docs | docs/guide/authentication.md |
| Doc review | 2026-10-07: passwordDenylist in password_strength.go holds exactly the eight named passwords |
| Defect review | 2026-10-07: plan/spec-password-weakness-warning.md in progress (review gate empty, the .ci never ran) |

## Description

Every password-set path judges the plaintext before it hashes it and warns when the password is shorter than 8 characters or matches one of eight common passwords (`password`, `123456`, `12345678`, `qwerty`, `admin`, `letmein`, `root`, `changeme`, compared whole and case-insensitively). The warning is advisory: the password is hashed and set, the commit succeeds, and the exit code stays 0, so a config Ze accepted before the check existed still commits. One policy serves `ze config set`, `ze config deactivate`, the CLI editor commit, the daemon load path, the REST/gRPC/gNMI commit path and `ze passwd`; the CLI surfaces name the leaf on stderr or in the commit status, and the daemon paths log at WARN. The reason names the rule and never the password. <!-- source: internal/component/config/password_strength.go -- PasswordWeakness, PasswordMinLength, passwordDenylist --> <!-- source: internal/component/config/password_hash.go -- HashedPassword, PasswordWeaknessWarnings --> <!-- source: internal/plugins/passwd/main.go -- runImpl -->
