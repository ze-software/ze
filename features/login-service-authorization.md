# Login-Service Authorization

## Meta

| Field | Value |
|-------|-------|
| Name | Login-Service Authorization |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/config/loginservice, internal/component/aaa, internal/component/authz |
| Real-path tests | test/parse/login-service-unregistered-name.ci, test/parse/login-service-undefined-set.ci, test/plugin/login-service-ssh-password.ci, test/plugin/login-service-ssh-publickey.ci, test/plugin/login-service-ze-cli.ci, test/plugin/login-service-web-form.ci, test/plugin/login-service-web-basic.ci, test/plugin/login-service-web-cookie-reload.ci, test/plugin/login-service-rest.ci, test/plugin/login-service-grpc.ci, test/plugin/login-service-mcp.ci, test/plugin/login-service-tacacs.ci, test/plugin/login-service-radius.ci, test/plugin/login-service-reload-tightens.ci, test/plugin/login-service-recovery-user.ci, test/plugin/login-service-shadowed-recovery-name.ci, test/plugin/login-service-absent-section.ci, test/plugin/login-service-empty-set.ci, test/plugin/login-service-chain-terminal.ci, test/plugin/login-service-shared-token.ci |
| Interop | radius/radius-admin-login-service-refused-freeradius |
| Docs | docs/guide/authentication.md, docs/guide/authorization.md |
| Doc review | 2026-11-27: verified registry symbols (Register, MustRegister, Names, Registered, Lookup), YANG paths under system/authorization/login-service and user/tacacs-profile/radius leafrefs, doctor code doctor-aaa-all-accounts-excluded, and metric ze_aaa_login_service_refused_total |
| Defect review | 2026-11-27: implemented and verified across 22 unit tests, 20 functional .ci tests (90/90 stress pass, 16/16 gating discrimination pass), and FreeRADIUS interop |

## Description

Named `system authorization login-service` sets restrict which login doors an account can authenticate through before command authorization runs. Each login surface registers its own name at `init()` in `internal/component/config/loginservice` (`ssh`, `web`, `rest`, `grpc`, `mcp`). Compiling out a surface removes its name from `LoginServiceValidator` validation and editor completion. Local users (`system authentication user`), TACACS+ privilege mappings (`system authentication tacacs-profile`), and RADIUS admin config (`system authentication radius`) each accept an optional `login-service` leafref (`require-instance true`).

Omitting `login-service` admits all doors, while an empty `service` list denies every checked door. Refusals return terminal `aaa.ErrAuthRejected` with the surface's normal wrong-credential response. Each refusal records an audit failure, logs the account and service, and increments `ze_aaa_login_service_refused_total{service}`. The unshadowed stored recovery account stays exempt, and `ze doctor` reports `doctor-aaa-all-accounts-excluded` when every configured account is excluded from every registered door. Reloads apply with no restart. Open SSH sessions, gRPC streams, and web SSE streams survive until close, while web `ze-session` cookies and subsequent MCP POSTs are refused at once. <!-- source: internal/component/config/loginservice/registry.go -- Register, MustRegister, Names, Registered, Lookup --> <!-- source: internal/component/aaa/login_profiles.go -- CheckLoginService, CheckLoginSet, SetAcceptedLoginSets, bindAAAMetrics --> <!-- source: internal/component/aaa/doctor.go -- checkAAAAllAccountsExcluded --> <!-- source: internal/component/config/validators.go -- LoginServiceValidator -->
