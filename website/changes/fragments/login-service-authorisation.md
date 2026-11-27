---
spec: login-service-authorisation
topic: Security
---

## Before

Ze authenticated an account and authorized each command, but nothing checked
which management surface the login arrived on. Any valid password, SSH key,
TACACS+ privilege mapping, RADIUS login, or named MCP token reached every
surface that accepted that credential type.

## After

Operators can define named `system authorization login-service` sets listing
allowed doors (`ssh`, `web`, `rest`, `grpc`, `mcp`). They can assign a set
through an optional `login-service` leafref on `system authentication user`,
`system authentication tacacs-profile`, and `system authentication radius`.
Each surface self-registers its name in `internal/component/config/loginservice`
at `init()`, so compiled-out surfaces vanish from validation and editor
completion.

A valid credential presented on an excluded door fails terminally at
authentication (`aaa.ErrAuthRejected`) with the surface's normal
wrong-credential response. Each refusal records an audit failure and increments
`ze_aaa_login_service_refused_total{service}`. Omitting `login-service` keeps
all doors open for backward compatibility, while an empty `service` list denies
every checked door. The unshadowed stored recovery account stays exempt.

`ze doctor` warns under `doctor-aaa-all-accounts-excluded` when every configured
account is excluded from every registered door. Reloads apply with no restart.
Open SSH sessions, gRPC streams, and web SSE streams survive until close. Web
`ze-session` cookies and subsequent MCP POST requests are refused at once.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
