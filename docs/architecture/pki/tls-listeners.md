# Serving a PKI Certificate on a TLS Listener

A TLS listener can serve an operator certificate from the PKI store together
with its full chain. Two consumers use it: the web and API HTTPS listener, and
the DoT and DoH listeners of the as112 and geodns plugins. The store itself is
[`pki-store.md`](pki-store.md).

<!-- source: internal/component/pki/tls.go -- ServerTLSMaterial, chainPEM, CheckCertReference -->
<!-- source: cmd/ze/hub/service_web.go -- webTLSMaterial -->
<!-- source: internal/core/dnsserver/secure.go -- the tls certificate reference -->

## Decision: the leaf comes first, then the intermediates

`ServerTLSMaterial` returns one PEM document holding the leaf CERTIFICATE block
followed by one block per stored intermediate, plus the PKCS#8 private key.

The block order is load-bearing. `tls.X509KeyPair` parses every CERTIFICATE
block into `tls.Certificate.Certificate`, and TLS requires the sender's own
certificate at index 0. The same shape is what
`show pki certificate name <n> bundle pem` prints, so the served chain and the
displayed chain cannot disagree.

## Decision: a named certificate fails closed

A configured name that does not resolve returns an error and no material. There
is no fallback to the self-signed certificate. A listener that quietly serves a
self-signed certificate while the config names a real one looks like a working
deployment until a client rejects it.

An EMPTY name is a different case: it means the operator configured no
reference, and the established self-signed path is used unchanged.

The reload path applies the same rule twice. The commit is refused when the web
certificate reference does not resolve against the just-installed store, before
any consumer applies, so the reload fails as a whole rather than leaving the
listener on a certificate the config no longer describes. The prior store is
restored on failure.

<!-- source: cmd/ze/hub/main_reload.go -- the web certificate reference check -->

## Decision: rotation does not rebind the listener

`UpdateWebCertificate` re-resolves the name in the new store and installs the
chain on the running server through `UpdateTLSCertificate`, which stores the
parsed pair in an atomic pointer the handshake reads. The listeners keep their
sockets, and the next handshake serves the new chain.

<!-- source: cmd/ze/hub/listener_migrate.go -- UpdateWebCertificate -->
<!-- source: internal/component/web/server.go -- UpdateTLSCertificate -->

## Decision: dnsserver takes an injected resolver

`internal/core` may not import `internal/component`, and the PKI store is a
component. The DNS server harness therefore holds a
`TLSMaterialResolver func(name string) (certPEM, keyPEM []byte, err error)`
option, and each consumer plugin injects `pki.ServerTLSMaterial`.

A nil resolver means the consumer supports no store references. A secure config
that names one is then an error, never a silent fallback. A `certificate` name
and a `cert-file` / `key-file` pair are mutually exclusive, so there is one
source of TLS material per endpoint.

<!-- source: internal/core/dnsserver/manager.go -- Options.TLSMaterialResolver -->
<!-- source: internal/plugins/as112/server.go -- the injected resolver -->
<!-- source: internal/plugins/geodns/server.go -- the injected resolver -->

## Decision: doctor reads the parsed config, not the live store

`CheckCertReference` validates a configured name against a config parsed
offline, so `ze doctor` reports a broken reference before the config is
committed. It emits `doctor-tls-reference` for a name the pki block does not
define and `doctor-tls-expired` for an expired certificate, and it warns 30 days
ahead of `NotAfter`. That window and the expired code match the file-based DoT
and DoH check, because the operator fix is the same whatever the source of the
certificate.

<!-- source: internal/component/web/doctor.go -- the web certificate reference check -->
<!-- source: internal/core/dnsserver/certcheck.go -- the file-based equivalent -->
