# Deferrals: fixit-ipsec-verify-siblings

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/deferral-tracking.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-23 | spec-fixit-ipsec-verify-siblings | Validate the `remote-access` gateway certificate references (`ra.Auth.ca-certificate`, `ra.Auth.certificate`) against the candidate PKI, and require a `ca-certificate` for `mode eap-tls` per RFC 5216 Section 5.3 (was AC-3..AC-7 of the source spec) | Tracing the item showed the whole `remote-access` block is inert (`engine/register.go:372` discards the IP pool; `ra.Auth` and `ra.Users` have no consumer; `matchResponderPeer` admits only configured site-to-site peers). Validating references on config that does nothing would polish an operator trap. Owner chose 2026-07-23 to implement the feature instead, so the validation belongs with the implementation that gives it meaning | `plan/spec-ipsec-remote-access.md` | deferred |
| 2026-07-23 | spec-fixit-ipsec-verify-siblings | Resolve `eap-user/*/certificate` against the PKI store (was AC-6) | Dropped, not moved: `EAPUser.Certificate` has no runtime consumer, and a *client* certificate would not normally live in the gateway's own PKI store. The check was an invented requirement. The field's real meaning is decided by the remote-access implementation | `plan/spec-ipsec-remote-access.md` | deferred |
