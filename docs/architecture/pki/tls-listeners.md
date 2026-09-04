# Serving a PKI Certificate on a TLS Listener

A TLS listener can serve an operator certificate from the PKI store together
with its full chain. Three consumers use it: the web and API HTTPS listener, the
looking glass, and the DoT and DoH listeners of the as112 and geodns plugins.
The store itself is [`pki-store.md`](pki-store.md).

Each consumer reads its own name:

| Consumer | Leaf |
|----------|------|
| Web and API | `environment.web.certificate` |
| Looking glass | `environment.looking-glass.certificate` |
| DoT and DoH | the `certificate` leaf of the endpoint's own `tls {}` container |

One shared name would make an operator choose which surface gets the right
certificate.

<!-- source: internal/component/pki/tls.go -- ServerTLSMaterial, chainPEM, CheckCertReference -->
<!-- source: cmd/ze/hub/service_tls.go -- listenerTLSMaterial -->
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

The hub declares that precedence once, in `listenerTLSMaterial`, and both hub
listeners call it. Its build constraint is the disjunction `ze_lg || ze_web`,
because either listener alone is a shipped build. A second copy of the rule
would be a future disagreement between the two surfaces with nothing to
arbitrate it.

The self-signed branch alone needs blob storage, because that is where the
generated pair is persisted. The looking glass therefore applies its
blob-storage rule to an EMPTY name only.

| Looking glass, no blob store | Result |
|------------------------------|--------|
| `tls true` written by the operator | an error |
| the TLS default, inherited | a warning, and plaintext |
| a named certificate | the named chain, over TLS |

The web server refuses a non-blob store for the whole listener earlier still.
Its credentials and its config live there whatever it serves.

<!-- source: cmd/ze/hub/service_lg.go -- buildLGService, the blob-storage branch -->

Both hub listeners are checked before the daemon starts either one. `runYANGConfig`
resolves each configured name against the loaded store, and a name that does not
resolve makes the daemon exit non-zero. A daemon that started and served a
self-signed certificate instead would look healthy while it presents the wrong
identity to every client.

<!-- source: cmd/ze/hub/main.go -- runYANGConfig, the web and looking-glass certificate startup gates -->

The reload path refuses the same two names against the just-installed store. The
check runs before any consumer applies, so the reload fails as a whole rather
than leaving a listener on a certificate the config no longer describes.
`restorePKIAfter` puts the prior store back on failure. The looking glass reads
its name through `lgCertificateName`, which the startup gate calls too, so a
restart and a reload cannot disagree about which certificate one config asks
for.

<!-- source: cmd/ze/hub/main_reload.go -- the web and environment.looking-glass.certificate reference checks, restorePKIAfter -->
<!-- source: cmd/ze/hub/main_reload.go -- lgCertificateName -->

## Decision: a listener that serves no TLS makes the leaf inert

A looking glass that serves no TLS presents no certificate, so a reload resolves
nothing and rotates nothing on it. A commit refused over a name that no listener
reads would reject a config the same daemon starts without a complaint.

Two configs reach that state, and each is caught at its own seam.

| Config | What makes the leaf inert |
|--------|---------------------------|
| `tls false`, written by the operator | `lgCertificateName` reports no name, so neither the startup gate nor the reload reads the leaf |
| `tls` inherited, no name written, no blob store, so Ze dropped the listener to plaintext at start | the registration installs no rotation handle, because `ServesTLS` reports that the running server serves plaintext |

The second config states that TLS is on, so `lgCertificateName` does report the
name and the startup gate and the reload refusal both read it. Only the running
server knows it was downgraded, which is why the second row is decided there and
not in the config.

An operator who adds a name to that deployment gets an accepted reload and a
looking glass that keeps serving plaintext. A restart then serves the named
chain over TLS, because the named path reads the `pki {}` container and needs no
blob store.

This is not a fallback. A looking glass that does serve TLS still fails closed
on a name the store does not hold.

<!-- source: cmd/ze/hub/main_reload.go -- lgCertificateName, the plaintext looking glass reports no name -->
<!-- source: cmd/ze/hub/service_lg.go -- buildLGService, the plaintext downgrade -->
<!-- source: cmd/ze/hub/register_lg.go -- the rotation handle is installed only for a TLS-serving looking glass -->
<!-- source: internal/component/lg/server.go -- ServesTLS -->

## Decision: rotation does not rebind the hub listeners

A hub listener re-resolves the name in the new store. It installs the chain on
the running server through `UpdateTLSCertificate`. That method parses the
material first, then stores the parsed pair in an
`atomic.Pointer[tls.Certificate]` the handshake reads. Unparseable material
leaves the previous certificate serving. The listeners keep their sockets, and
the next handshake serves the new chain.

| Consumer | Reload hook | Running server |
|----------|-------------|----------------|
| Web and API | `updateWebCertificate` | `web.(*WebServer).UpdateTLSCertificate`, read per handshake by `getCertificate` |
| Looking glass | `updateLGCertificate` | `lg.(*LGServer).UpdateTLSCertificate`, read per handshake by `getCertificate` |
| DoT and DoH | none | as112 and geodns rebind, because the listener signature folds in the leaf fingerprint |

Both hub servers clear `tls.Config.Certificates` at the statement that sets
`GetCertificate`. `crypto/tls` serves the fixed list ahead of the callback when
the client sends no SNI. A populated list would let the atomic pointer and the
wire disagree.

Each hook takes its own server handle, so rotating one listener cannot install
the other listener's chain.

<!-- source: cmd/ze/hub/listener_migrate.go -- updateWebCertificate, updateLGCertificate -->
<!-- source: internal/component/web/server.go -- UpdateTLSCertificate, getCertificate -->
<!-- source: internal/component/lg/server.go -- UpdateTLSCertificate, getCertificate -->
<!-- source: internal/core/dnsserver/secure.go -- the listener signature folds in the certificate fingerprint -->

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

Each consumer registers its own check, and each prefixes its messages with the
leaf it read, so `ze doctor` names the listener the operator must fix.

<!-- source: internal/component/web/doctor.go -- the web certificate reference check -->
<!-- source: internal/component/lg/doctor.go -- the looking-glass certificate reference check -->
<!-- source: internal/core/dnsserver/certcheck.go -- the file-based equivalent -->
