# Serving a PKI Certificate on a TLS Listener

A TLS listener can serve an operator certificate from the PKI store together
with its full chain. The web and API HTTPS listener, the looking glass, the managed fleet listener,
and the DoT and DoH listeners of the as112 and geodns plugins use it.
The store itself is [`pki-store.md`](pki-store.md).

Each consumer reads its own name:

| Consumer | Leaf |
|----------|------|
| Web and API | `environment.web.certificate` |
| Looking glass | `environment.looking-glass.certificate` |
| Managed fleet | `bus hub server <block> certificate` |
| DoT and DoH | the `certificate` leaf of the endpoint's own `tls {}` container |

One shared name would make an operator choose which surface gets the right
certificate.

## Design decisions

| Decision | It forbids | Source | Date |
|----------|------------|--------|------|
| The looking glass may serve plain HTTP, because what it publishes is public, and one looking glass serves HTTP and HTTPS at the same time. Thomas's words, as voice-transcribed: "We should allow the looking glass to be HTTP. The information is public." and "We should be able to serve on both HTTP and HTTPS, not either." Asked where the choice lives and what happens to `ze.looking-glass.tls`, he answered "1a 2a": the choice is the `tls` leaf of each `server` entry, default true, and the environment variable is removed. The no-store rule applies to each entry: an entry that left `tls` unset serves HTTP with a warning on a daemon with no `database/` store and no named certificate, an entry that says `tls true` there is refused at start and at reload, and a reload does what a restart with the same config does | A `tls` leaf on the `looking-glass` container, or any other setting that gives every listener of the looking glass one protocol. An environment variable that chooses the looking glass's protocol. A fallback that still reads either. A reload that leaves a listener on a protocol a restart with the same config would not serve. Serving the RFC 8522 API on a connection that is not TLS | plan/records/ruling-looking-glass-http-and-https.md | 2026-11-23 |
| The web and looking glass use the shared `listenerTLSMaterial` precedence. The approved Fleet reload design shares `managedCertificate` with startup because the daemon authority issues its unnamed leaf. | Independent certificate selection at startup and reload | plan/release/features/spec-fleet-1-device-registry.md | 2026-11-23 |
| A certificate that cannot be presented never stops ze from starting. A web server whose certificate name does not resolve is not started. The looking glass is judged one `server` entry at a time: an entry that says `tls true` with no certificate to present stays down, every entry that asks for TLS stays down when the named certificate does not resolve, and the other entries serve. Each is reported with its fix by an error line in the log under a doctor code and by `ze doctor`. A reload whose config cannot be served stays refused. This supersedes the start half of the row of 2026-11-23 above, which says a `tls true` entry "is refused at start": at start that entry stays down and the others serve, and at reload the config is still refused. Thomas's words, as voice-transcribed: "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; "reload stay refused"; "Should only that endpoint stay down while the others serve: yes" | A daemon that exits over a web or looking-glass certificate. A looking glass that does not start because one entry cannot be served. A self-signed certificate or plain HTTP served in place of a named certificate. A reload accepted with an entry that cannot be served | plan/records/ruling-ze-always-starts.md | 2026-11-26 |

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

With an empty name, the web and looking glass use their self-signed pair at
startup and after a reload. The managed fleet listener uses the leaf issued
by the daemon authority, at a reload too. The hub declares each of its two
precedences once: `listenerTLSMaterial` for the web and looking glass listeners,
and `managedCertificate` for the managed listener. Each is called by its
listener's start and by its reload, so neither two surfaces nor a start and a
reload can disagree about which certificate a resolvable config asks a listener
for (the unresolved-name startup/reload difference for the managed listener is
stated under "Decision: rotation does not rebind the hub listeners").

The self-signed branch alone needs the `database/` store, because that is where
the generated pair is persisted. The looking glass therefore applies its
store rule to an EMPTY name only, and applies it to each `server` entry, because
each entry chooses HTTPS or HTTP in its own `tls` leaf.

| Looking glass entry, no store, no named certificate | Result |
|------------------------------------------------------|--------|
| `tls true` written by the operator | at start that entry stays down, is reported, and the other entries serve; a reload is refused |
| `tls` left unset, so the default, true | a warning, and plain HTTP on that entry |
| `tls false` | plain HTTP, with no warning |

With a named certificate every entry but a `tls false` one serves the named
chain over TLS, store or no store. When that name does not resolve, every entry
but a `tls false` one stays down at start, store or no store: the self-signed
pair and plain HTTP are never served in place of a named certificate.

`planLGEndpoints` is the one place this rule is written. It answers a plan: the
protocol of each entry that is served, the entries that serve HTTP for want of a
certificate, and the entries that cannot be served, each with the operator
error that names it and its fix. The start path, the reload path and `ze doctor`
all read that plan, and they differ only in what they do with an entry that
cannot be served:

| Reader | An entry that cannot be served |
|--------|--------------------------------|
| Start (`startLGService`) | stays down and is reported; the other entries are bound and serve. When no entry can be served, no looking glass is built |
| Reload (`checkLGTransport`, through `planLGTransports`) | refuses the reload as a whole; the running listeners keep what they serve |
| `ze doctor` (`lgListenerDiagnostics`) | one `doctor-lg-tls-unservable` finding for each such entry |

The reload gate reads the store of the running looking glass. When no looking
glass runs, which is what a start leaves when it could serve no entry, the gate
reads the store the daemon was started with (`setDaemonStore`), so a reload of
the config that start could not serve is still refused.

The web server is not started without the store, for the whole listener, earlier still.
Its credentials and its config live there whatever it serves.

<!-- source: cmd/ze/hub/lg_transport.go -- planLGEndpoints, planLGTransports, lgTLSNeedsCertificate, lgCertificateUnpresentable, resolveLGListeners -->
<!-- source: cmd/ze/hub/service_lg.go -- startLGService, the plan applied at start -->
<!-- source: cmd/ze/hub/listener_migrate.go -- checkLGTransport, setDaemonStore -->

## Decision: a certificate that cannot be presented never stops ze

Ze always starts. A daemon that exits brings up no networking, and the operator
then has no route to the box to correct the name that stopped it. So a
certificate that cannot be presented leaves its listener unstarted, and ze runs
with its other services.

| At start | What ze does | Reported under |
|----------|--------------|----------------|
| The web certificate name does not resolve | The web server is not started | `doctor-tls-reference` |
| The looking-glass certificate name does not resolve | Every entry that asks for TLS stays down, and the `tls false` entries serve | `doctor-lg-tls-unservable` for each entry in the log, `doctor-tls-reference` for the name in `ze doctor` |
| A looking-glass entry says `tls true` with no store and no name | That entry stays down, and the other entries serve | `doctor-lg-tls-unservable` |
| `ze.looking-glass.listen` does not parse | The looking glass is not started | `doctor-lg-listen-unreadable` |

`runYANGConfig` resolves the web name against the loaded store before it builds
any service (`webCertificateProblem`). The looking glass resolves its own name
when it starts (`lgCertificateProblem`). Each one writes an error line in the
log under the code above, with the fix, and the same text on the terminal, and
says that ze keeps running. Ze never serves a self-signed certificate in place
of a named one: such a listener would look healthy while it presents the wrong
identity to every client.

A listener that was left down takes a corrected configuration at the next
restart of ze. A looking glass that runs with one entry down binds that entry at
the reload that corrects it, because a reload binds an address the running
looking glass does not hold yet.

<!-- source: cmd/ze/hub/main.go -- runYANGConfig, the web certificate startup gate and the looking-glass listener read -->
<!-- source: cmd/ze/hub/service_tls.go -- webCertificateProblem, reportWebNotStarted -->
<!-- source: cmd/ze/hub/lg_transport.go -- lgCertificateProblem, reportLGServerDown, reportLGNotStarted -->

## Decision: a reload refuses what a start leaves down

The reload path refuses the same two names against the just-installed store and
asks the managed server to prepare its certificate (`prepareManagedReload`) at
the same gate. The check runs before any consumer applies, so the reload fails
as a whole rather than leaving a listener on a certificate the config no longer
describes. `restorePKIAfter` puts the prior store back on failure. The looking
glass reads its listeners and its name through `lgListenerConfigFrom`, which
the start path calls too, so a restart and a reload cannot disagree about
which certificate one config asks for. The same gate refuses, through
`checkLGTransport`, a `tls true` entry that has no certificate to present.
A start leaves a listener down where a reload refuses the config: the daemon
that reloads is already running and reachable, and keeps what it serves.

The startup gate of `runYANGConfig` does not read the managed name: at startup
an unresolved managed certificate leaves the managed server built with nothing
bound and each affected address reported unbound in doctor, so the operator can
commit a fix without restarting into a dead daemon. Every later reload that
still carries the unresolved name is refused at the reload gate until the name
is defined in `pki` (`edit pki certificate <name>`) or removed from the serving
blocks.

<!-- source: cmd/ze/hub/managed_server.go -- startManagedServer, prepareManagedReload -->
<!-- source: cmd/ze/hub/main_reload.go -- runReloadContext, prepareManagedReload call -->

A reload can also fail after the web or looking-glass listener took the reloaded config's
certificate: the looking glass refuses its install, a listener cannot move to
its new address, the next-hop ranges are refused, or the candidate cannot be
promoted. Putting the prior store back is
not enough then, because the listener holds the pair it was handed and never
re-reads a name. Before it changes anything, the reload reads the name each
listener has under the running config, through the same two readers
(`runningListenerCertificates`), and for the looking glass the protocol of each
of its listeners with it. Once `rollbackReload` has put the prior store
back, it reinstalls those names on every listener whose install succeeded, so
the listener serves the certificate of the config that stays in force, and each
looking-glass listener serves HTTPS or HTTP as that config says
(`restoreLGTransport`). An entry of that config that the start left down stays
down: the restore applies the entries that serve and refuses nothing. A
listener whose install failed kept what it was serving, so nothing is
reinstalled on it. A reinstall that fails is reported with the refusal as a
failed `certificate restore`. The managed listener needs no reinstall: its
certificate is prepared at the gate and installed after promotion
(`ApplyManagedListeners`), so it never presents the certificate of a config
that was rolled back.

<!-- source: cmd/ze/hub/main_reload.go -- the web and environment.looking-glass.certificate reference checks, restorePKIAfter -->
<!-- source: cmd/ze/hub/lg_transport.go -- lgListenerConfigFrom -->
<!-- source: cmd/ze/hub/main_reload.go -- lgCertificateName -->
<!-- source: cmd/ze/hub/listener_migrate.go -- checkLGTransport -->
<!-- source: cmd/ze/hub/main_reload.go -- runningListenerCertificates, restoreOn, the restoreCertificates undo -->
<!-- source: cmd/ze/hub/listener_migrate.go -- restoreLGTransport, applyLGPlan -->

## Decision: each looking-glass listener serves HTTPS or HTTP

The looking glass has no protocol of its own. Each `server` entry has a `tls`
leaf, default true, and the listener of that entry serves HTTPS or plain HTTP.
One looking glass therefore serves both at once:

```
environment {
    looking-glass {
        enabled true
        server public { ip 0.0.0.0; port 80; tls false; }
        server secure { ip 0.0.0.0; port 8443; }
    }
}
```

The server holds one bound listener per entry, and each listener carries its
protocol in an atomic flag that its `Accept` reads: a TLS connection or the bare
one. All TLS listeners share one `tls.Config`, whose certificate is read per
handshake. Nothing in the server is true of every listener, so no setting can
give them all one protocol.

An address that has no entry takes the default, HTTPS: the addresses of
`ze.looking-glass.listen`, and the `0.0.0.0:8443` endpoint of a looking glass
that `ze.looking-glass.enabled` starts with no block. No environment variable
chooses the protocol. A deployment that wants HTTP writes the `server` entry
with `tls false`.

The RFC 8522 API under `/.well-known/looking-glass/` is refused on a connection
that is not TLS, whatever else the same looking glass serves. `http.Server` sets
`Request.TLS` from the connection it was handed, so the check is per connection.

<!-- source: internal/component/lg/yang/ze-lg-conf.yang -- leaf tls of list server -->
<!-- source: internal/component/config/loader_extract.go -- LGServerTLS, extractLGServerList -->
<!-- source: internal/component/lg/server.go -- ListenAddr, transportListener, UpdateListenerTLS -->
<!-- source: internal/component/lg/handler_rfc8522.go -- handleRFC8522, the plaintext refusal -->

## Decision: a looking glass with no TLS entry makes the leaf inert

A looking glass whose every `server` entry says `tls false` presents no
certificate. `lgCertificateName` reports no name for it, so neither the startup
gate nor the reload reads the `certificate` leaf. A commit refused over a name
that no listener reads would reject a config the same daemon starts without a
complaint.

One entry that asks for TLS is enough for the name to be read and resolved.
When it does not resolve, a start leaves the entries that ask for TLS down, and
a reload is refused.

A listener that serves HTTP because its entry left `tls` unset, on a daemon with
no store and no named certificate, is not that case: the config asks for TLS
there. An operator who adds a name to that deployment gets HTTPS on that
listener from its next connection, with no restart, because that is what a
restart with the same config serves.

<!-- source: cmd/ze/hub/main_reload.go -- lgCertificateName, the looking glass with no TLS entry reports no name -->
<!-- source: cmd/ze/hub/lg_transport.go -- lgAsksForTLS -->
<!-- source: cmd/ze/hub/register_lg.go -- the protocol and certificate handle is installed whatever the looking glass serves -->

## Decision: rotation does not rebind the hub listeners

On every reload the web and looking-glass listeners resolve the certificate the reloaded config
describes through `listenerTLSMaterial`: the named chain from the new store, or
the self-signed pair when the config names none. Each listener's handle carries
its own self-signed store and SAN hint (`selfSignedSource`), the ones it was
built with. It installs the result on the running server through
`UpdateTLSCertificate`. That method parses the material first, then stores the
parsed pair in an `atomic.Pointer[tls.Certificate]` the handshake reads.
Unparseable material leaves the previous certificate serving. The listeners
keep their sockets, and the next handshake serves the new certificate.

Web and looking-glass rotation keeps the bound sockets and serves what a
restart with the same config would serve. For the looking glass that covers the
protocol too: `updateLGTransport` installs the certificate first, then hands the
running server the protocol of each listener (`UpdateListenerTLS`), before any
listener moves. A listener
whose protocol changes keeps its socket. Its open connections finish on the
protocol they started with, and its next connection takes the new one.

| Web or looking-glass configuration | What the next handshake presents |
|-----------------|----------------------------------|
| A name the new store defines | that chain |
| A name the new store does not define | the reload is refused, the previous certificate stays |
| No name, a store | the self-signed pair, loaded from the store or generated into it |
| No name, no store, a looking-glass entry that left `tls` unset | nothing: that listener serves plain HTTP from its next connection, and Ze logs a warning that names the entry and the fix |
| No name, no store, a looking-glass entry that says `tls true` | the reload is refused, the previous certificate and protocol stay |
| A name added, on a looking-glass entry that served HTTP for want of a certificate | that chain, over TLS, from the listener's next connection |
| `tls` changed on a looking-glass entry | the listener serves the new protocol from its next connection |

A reload can also add an entry. `updateLGTransport` runs before the listeners
move, and `UpdateListenerTLS` records the protocol of an address that is not
bound yet. `Reconfigure` then binds the new address with that record, so a
`tls false` entry answers plain HTTP from its first connection and a TLS entry
presents the reloaded config's certificate from its first handshake. The record
also holds the protocol of an address `Reconfigure` removed, which is what a
rollback reads when it puts that address back. A rollback hands the looking
glass the running config's protocols before it moves the listeners back, for
the same reason. `Reconfigure` refuses an address that has no record, before it
binds anything: the server never chooses a protocol that no config gave it.

A reloaded config that starts no looking glass describes no listener. The
running listeners stay, with their protocol, and the ones that serve TLS take
the certificate that config names. With no name and no store they have no
self-signed pair to return to, and that reload is refused
(`errSelfSignedNeedsStore`).

On every reload the managed listener prepares its certificate at the certificate
gate through `managedCertificate` (`prepareManagedReload`,
`PrepareManagedListeners`) and installs it after promotion
(`ApplyManagedListeners`) behind the source `getCertificate` reads, before
binding any added addresses. All three listeners keep their sockets during
rotation, no managed session is closed for a certificate change, and the next
handshake presents the prepared chain. An accepted managed reload matches a
restart with the same configuration, whereas an unresolved managed certificate
name differs: at startup the daemon keeps running with its managed addresses
unbound and reports them in doctor, while a reload naming an unresolved managed
certificate is refused and keeps the previous configuration.

| Reloaded managed listener configuration | What the next handshake presents |
|--------------------------------|----------------------------------|
| A name the new store defines | That chain, including a renewal under the same name |
| A name the new store does not define, or material that does not parse | The reload is refused, the previous certificate stays |
| No name | The leaf the daemon's authority issues, naming `127.0.0.1` and the host of every served address |
| No name, and the authority cannot issue, on a listener that presents a named certificate | The reload is refused, the named certificate stays. Restore its name on each candidate serving block (for multiple blocks the refusal supplies the set lines for an absolute merge) |
| No name, and the authority cannot issue, on a listener that presents the issued leaf | The leaf in place stays, and an address the reload added is left unbound and reported in doctor |

| Consumer | Reload hook | Running server |
|----------|-------------|----------------|
| Web and API | `updateWebCertificate` | `web.(*WebServer).UpdateTLSCertificate`, read per handshake by `getCertificate` |
| Looking glass | `updateLGTransport` | `lg.(*LGServer).UpdateTLSCertificate`, read per handshake by `getCertificate`, then `lg.(*LGServer).UpdateListenerTLS`, read per connection by `transportListener.Accept` |
| Managed fleet | `prepareManagedReload` (`PrepareManagedListeners`) at the certificate gate; `ApplyManagedListeners` after promotion | `ManagedServer` certificate source, read per handshake by `getCertificate` |
| DoT and DoH | none | as112 and geodns rebind, because the listener signature folds in the leaf fingerprint |

The web and looking-glass servers clear `tls.Config.Certificates` at the statement that sets
`GetCertificate`. `crypto/tls` serves the fixed list ahead of the callback when
the client sends no SNI. A populated list would let the atomic pointer and the
wire disagree. The managed listener uses `StartListeners` with the callback alone.

Each hook takes its own server handle, so rotating one listener cannot install
the other listener's chain.

<!-- source: cmd/ze/hub/listener_migrate.go -- updateWebCertificate, updateLGTransport, installListenerCertificate -->
<!-- source: cmd/ze/hub/main_reload.go -- runReloadContext, the certificates and protocols before reloadListeners -->
<!-- source: internal/component/lg/server.go -- UpdateListenerTLS, Reconfigure, errLgServerNoRecordedProtocol, transportListener -->
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
leaf it read, so `ze doctor` names the listener the operator must fix. The web
and looking-glass findings of `doctor-tls-reference` also say what ze does with
the name, which is to start without that listener, and carry the fix.

The looking glass has a second check, `lg-listener-protocol`, which reports
what each listener serves on this box. It asks the functions the start and
reload paths ask (`lgListenerConfigFrom`, `planLGEndpoints`), with the store
doctor opened standing for the daemon's store, so it cannot disagree with what
ze does with the same config. It takes the named certificate as presentable:
the reference check above reports a name the pki block does not define, and
its finding says which listeners stay down for it and gives the fix.

| Code | Severity | When |
|------|----------|------|
| `doctor-lg-plain-http` | warning | an entry left `tls` unset, no certificate is named and there is no `database/` store, so it serves plain HTTP. The start path prints the same warning, and a reload logs it under this code |
| `doctor-lg-tls-unservable` | error | an entry says `tls true` with no certificate to present, one finding for each such entry: ze starts and leaves that entry down, the other entries serve, and a reload is refused. The start path logs each entry it leaves down under this code, also when the cause is a named certificate that does not resolve |
| `doctor-lg-listen-unreadable` | error | `ze.looking-glass.listen` does not parse: ze starts without the looking glass, and a reload is refused |

<!-- source: internal/component/web/doctor.go -- the web certificate reference check -->
<!-- source: internal/component/lg/doctor.go -- the looking-glass certificate reference check -->
<!-- source: cmd/ze/hub/lg_doctor.go -- lgListenerDiagnostics, the looking-glass listener protocol report -->
<!-- source: cmd/ze/hub/register_lg_doctor.go -- checkLGListeners, the lg-listener-protocol registration -->
<!-- source: internal/core/dnsserver/certcheck.go -- the file-based equivalent -->

<!-- source: internal/component/plugin/server/managed_serve.go -- managedCertificate -->
<!-- source: internal/component/plugin/server/managed_apply.go -- PrepareManagedListeners, ApplyManagedListeners, getCertificate -->
