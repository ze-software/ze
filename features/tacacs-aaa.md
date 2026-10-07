# TACACS+ AAA

## Meta

| Field | Value |
|-------|-------|
| Name | TACACS+ AAA |
| Page | docs/guide/tacacs.md |
| Kind | protocol |
| Scope | partial |
| Scope gaps | PAP is the only authentication type, no interop against a third-party TACACS+ server |
| Level | experimental |
| Components | internal/component/tacacs |
| Real-path tests | test/plugin/tacacs-auth.ci, test/plugin/tacacs-acct.ci, test/plugin/tacacs-author.ci, test/plugin/tacacs-fallback.ci, test/plugin/tacacs-local-only.ci, test/plugin/tacacs-readonly.ci, test/plugin/tacacs-show.ci, test/plugin/tacacs-singleconnect.ci |
| RFCs | rfc8907 |
| Docs | docs/guide/tacacs.md |
| Doc review | 2026-10-07: every source anchor resolves; START and STOP records are sent by internal/component/tacacs/accounting.go, strict-fallback is a YANG leaf, and the chain priority 100 is in register.go |
| Defect review | 2026-10-07: audit found no open immediate spec against internal/component/tacacs; journal rows naming a Component, not each re-verified here: helper-bypassed-by-an-open-coded-copy.md:28, reference-checked-claim-unchecked.md:42, silent-fall-through.md:23 |
| Extra criteria | supported: interop against tac_plus = test/interop-tacacs; supported: each journal row naming a Component re-verified as fixed or not a defect = none yet |

## Description

RFC 8907 TACACS+ client for SSH login: PAP authentication, ordered server failover with per-server timeout, MD5 pseudo-pad body encryption, priv-lvl-to-profile mapping, command accounting (START/STOP records on every dispatched CLI command), and explicit-reject vs unreachable distinction so wrong-password TACACS+ replies do NOT silently fall through to local bcrypt. Runs as a pluggable `aaa.Authenticator` so local bcrypt remains the fallback when every TACACS+ server is unreachable (default). Configurable `strict-fallback` mode denies authorization when TACACS+ infrastructure is unavailable instead of falling back to local RBAC. <!-- source: internal/component/tacacs/client.go -- TacacsClient, server failover --> <!-- source: internal/component/tacacs/authenticator.go -- tacacsAuthenticator -->
