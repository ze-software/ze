# DNS Secure Transports and DNSSEC Stub Validation

DNS-over-TLS (RFC 7858) and DNS-over-HTTPS (RFC 8484) extend the shared harness
in `internal/core/dnsserver`, not each DNS plugin. DoT is a TLS-wrapped TCP
`dns.Server`. DoH is a `net/http` handler that drives the same `dns.Handler`
through an in-memory `dns.ResponseWriter`.

<!-- source: internal/core/dnsserver/secure.go -- DoT and DoH listeners -->
<!-- source: internal/core/dnsserver/manager.go -- Manager, Apply -->

## One answer policy across four transports

Per-plugin DoT and DoH would give each plugin its own copy of the answer
policy. With one handler behind all four transports, the `allow-from` decision,
the client-IP selection and the authoritative and recursion guard are identical
on cleartext UDP, cleartext TCP, DoT and DoH.

A denied query is not the same on every transport. Cleartext DNS drops
silently. DoH is request and response, so a denial returns HTTP 403. A hung
connection or a fabricated DNS reply would both be dishonest answers to a
request that was received and refused.

## The listener signature carries the certificate fingerprint

The endpoint signature folds in the leaf certificate fingerprint, so a rotated
certificate rebinds the listener. The self-signed fallback is generated once
and cached. Regenerating it per apply would change the fingerprint and rebind
on every no-op config reload.

<!-- source: internal/core/dnsserver/tlsmaterial.go -- LoadTLSMaterial -->
<!-- source: internal/core/dnsserver/certcheck.go -- CheckCertMaterial -->

as112 and geodns share `LoadTLSMaterial` and the `CheckCertMaterial` doctor
check, which reuses the registered `doctor-tls-*` codes. A third authoritative
DNS plugin gets secure transports and the certificate doctor check with no new
code.

## Port defaults

A secure listener port that binds the plugin's existing addresses is a scalar
`leaf listen-port { type zt:port; default N }`. Only a service that uses
`uses zt:listener` registers in `internal/component/config/listener_defaults.go`,
so the scalar form stays outside the port-defaults lint. Config parse does not
materialize a YANG default for an absent leaf, so the Go parser applies the
same defaults the YANG states.

<!-- source: internal/core/dnsserver/secure.go -- DefaultSecureConfig, ParseSecureLeaves -->

## DNSSEC is the RFC 4035 stub model

The ze resolver sets the EDNS0 DO bit with CD=0 and relies on a validating
upstream to answer SERVFAIL for a broken chain. It builds no in-process RRSIG
validator, which is outside the role of a stub resolver.

| Mode | Behavior on a failed chain | Unsigned answer |
|------|----------------------------|-----------------|
| `off` (default) | DO bit not set, byte-identical query to before | accepted |
| `permissive` | logged | accepted |
| `strict` | rejected | accepted |

An insecure or unsigned answer with AD=0 is always accepted. The toggle lives
in `system { dns { dnssec-validation } }`, the existing resolver container, not
in a new `service { resolve }` surface.

<!-- source: internal/component/resolve/dns/resolver.go -- DNSSEC stub validation modes -->
<!-- source: internal/component/config/system/system.go -- dnssec-validation leaf -->
