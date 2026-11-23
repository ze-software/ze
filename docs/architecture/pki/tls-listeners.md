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
reference, and the established self-signed path is used unchanged. That holds
at a reload too. A commit that removes the leaf returns the listener to its
self-signed pair, the one a restart with that config would serve, rather than
leaving it on the chain the config dropped.

The hub declares that precedence once, in `listenerTLSMaterial`. Both hub
listeners call it at start, and both reload hooks call it, so neither two
surfaces nor a start and a reload can disagree about which certificate one
config asks for. It is always-on code, because the reload hooks are. A second
copy of the rule would be a future disagreement with nothing to arbitrate it.

The self-signed branch alone needs the `database/` store, because that is where
the generated pair is persisted. The looking glass therefore applies its
store rule to an EMPTY name only.

| Looking glass, no store | Result |
|-------------------------|--------|
| `tls true` written by the operator | an error |
| the TLS default, inherited | a warning, and plaintext |
| a named certificate | the named chain, over TLS |

The web server refuses to start without the store for the whole listener earlier still.
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

A reload can also fail after a listener took the reloaded config's
certificate: the looking glass refuses its install, its next-hop ranges are
refused, or the candidate cannot be promoted. Putting the prior store back is
not enough then, because the listener holds the pair it was handed and never
re-reads a name. Before it changes anything, the reload reads the name each
listener has under the running config, through the same two readers
(`runningListenerCertificates`). Once `rollbackReload` has put the prior store
back, it reinstalls those names on every listener whose install succeeded, so
the listener serves the certificate of the config that stays in force. A
listener whose install failed kept what it was serving, so nothing is
reinstalled on it. A reinstall that fails is reported with the refusal as a
failed `certificate restore`.

<!-- source: cmd/ze/hub/main_reload.go -- the web and environment.looking-glass.certificate reference checks, restorePKIAfter -->
<!-- source: cmd/ze/hub/main_reload.go -- lgCertificateName -->
<!-- source: cmd/ze/hub/main_reload.go -- runningListenerCertificates, restoreOn, the restoreCertificates undo -->

## Decision: a listener that serves no TLS makes the leaf inert

A looking glass that serves no TLS presents no certificate, so a reload resolves
nothing and rotates nothing on it. A commit refused over a name that no listener
reads would reject a config the same daemon starts without a complaint.

Two configs reach that state, and each is caught at its own seam.

| Config | What makes the leaf inert |
|--------|---------------------------|
| `tls false`, written by the operator | `lgCertificateName` reports no name, so neither the startup gate nor the reload reads the leaf |
| `tls` inherited, no name written, no store, so Ze dropped the listener to plaintext at start | the registration installs no rotation handle, because `ServesTLS` reports that the running server serves plaintext |

The second config states that TLS is on, so `lgCertificateName` does report the
name and the startup gate and the reload refusal both read it. Only the running
server knows it was downgraded, which is why the second row is decided there and
not in the config.

An operator who adds a name to that deployment gets an accepted reload and a
looking glass that keeps serving plaintext. A restart then serves the named
chain over TLS, because the named path reads the `pki {}` container and needs no
store.

This is not a fallback. A looking glass that does serve TLS still fails closed
on a name the store does not hold.

<!-- source: cmd/ze/hub/main_reload.go -- lgCertificateName, the plaintext looking glass reports no name -->
<!-- source: cmd/ze/hub/service_lg.go -- buildLGService, the plaintext downgrade -->
<!-- source: cmd/ze/hub/register_lg.go -- the rotation handle is installed only for a TLS-serving looking glass -->
<!-- source: internal/component/lg/server.go -- ServesTLS -->

## Decision: rotation does not rebind the hub listeners

On every reload a hub listener resolves the certificate the reloaded config
describes through `listenerTLSMaterial`: the named chain from the new store, or
the self-signed pair when the config names none. Each listener's handle carries
its own self-signed store and SAN hint (`selfSignedSource`), the ones it was
built with. It installs the result on the running server through
`UpdateTLSCertificate`. That method parses the material first, then stores the
parsed pair in an `atomic.Pointer[tls.Certificate]` the handshake reads.
Unparseable material leaves the previous certificate serving. The listeners
keep their sockets, and the next handshake serves the new certificate.

A reload never rebinds a listener to plaintext. A looking glass that started on
a named certificate with no `database/` store has no self-signed pair to return
to, so a reload that drops its name is refused, and the listener keeps the
chain of the config that stays in force.

| Reloaded config | What the next handshake presents |
|-----------------|----------------------------------|
| A name the new store defines | that chain |
| A name the new store does not define | the reload is refused, the previous certificate stays |
| No name, a store | the self-signed pair, loaded from the store or generated into it |
| No name, no store (looking glass only) | the reload is refused, the previous certificate stays |

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

<!-- source: cmd/ze/hub/listener_migrate.go -- updateWebCertificate, updateLGCertificate, installListenerCertificate -->
<!-- source: cmd/ze/hub/service_tls.go -- listenerTLSMaterial, errSelfSignedNeedsStore -->
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
