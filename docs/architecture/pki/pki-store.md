# The PKI certificate store

Shared certificate infrastructure: CA certificates and device certificates
loaded from config, validated at load time, and served to any consumer that
needs X.509 material. IPsec is the first consumer; TLS, SSH host certificates
and managed-device authentication can use the same store.

The package has a second half that config does not reach. `internal/component/pki/ca.go`
is a certificate authority: it keeps one root and issues short-lived leaves from
it, for Ze's own components. The store and the authority answer different
questions. The store serves the certificate an operator CONFIGURED. The
authority issues one when no operator configured any.

A TLS listener that serves one of these certificates with its full chain is
[`tls-listeners.md`](tls-listeners.md).

<!-- source: internal/component/pki/store.go -- Load, Validate, GetCA, GetCertificate, CertCN, CAPool, IntermediatePool, ExportPEM, CleanupPEM -->
<!-- source: internal/component/pki/ca.go -- RootStore, Root, LoadOrGenerateRoot, LoadOrGenerateRootFor, Root.IssueLeaf, Root.IssueLeafFor, Root.CertificatePEM, loadedRoot -->
<!-- source: internal/component/pki/config.go -- ParseConfig, parseCACert, parseDeviceCert, parsePrivateKey, certificateDER, privateKeyDER, verifyKeyMatchesCert -->
<!-- source: internal/component/plugin/leaf.go -- ServingLeaf, ServingLeaf.Certificate, renewalDeadline -->
<!-- source: internal/component/pki/types.go -- CACertEntry, CertificateEntry, PKIConfig, CertSummary -->
<!-- source: internal/component/pki/show.go -- handleShowPKICertificates, handleShowPKICertificate, handleShowPKICertificatePEM, handleShowPKICertificateBundlePEM, handleShowPKICertificateFingerprint, handleShowPKILocalCAPEM -->
<!-- source: internal/component/pki/doctor.go -- caRootDoctorCheck, checkCARoot -->
<!-- source: internal/component/pki/register.go -- the doctor check registration -->
<!-- source: internal/component/pki/yang/ze-pki-conf.yang -- the pki config module -->

## Decisions

**Each output form of `show pki certificate name` is its own command.** The
detail, `pem`, `bundle pem` and `fingerprint` forms take structurally different
tails, so one command reading them from its arguments states its grammar in a
handler switch where no operator and no catalog can read it. Four sibling
containers, each with its own `ze:command` and its own handler, put the grammar
in the model instead.

**Certificate values are PEM or base64-encoded DER.** A leaf that opens
`-----BEGIN` is read as PEM and a broken one is refused as PEM, so a truncated
paste is never reported as a base64 error. Base64 DER is the compact form and
every config written before this still loads. PEM is what the operator has:
`ze show pki local-ca pem` prints it, a `.crt` file holds it, and a leaf that
refused it made the operator strip the armor and rejoin the lines by hand.

**Private key detection tries PKCS8, then SEC1, then PKCS1.** Real keys arrive
in all three encodings depending on the tool that produced them, so requiring
one format rejects valid input.

**The store is an atomic pointer swap, not a mutex.** Readers are the show
commands and the IPsec consumer; the writer is a config reload. Readers never
block the writer.

**The chain and the expiry are validated at load, not lazily.** Failing at
config load produces a better error and stops the daemon from running with a
broken certificate.

**PEM export goes to a temporary directory.** A consumer that expects PEM file
paths rather than inline data gets them from `ExportPEM`, and `CleanupPEM`
removes them.

**PKI is its own component, not part of ipsec.** It is shared infrastructure.
Web TLS, SSH host certificates and managed-device authentication are all
plausible consumers.

**The root lives in ZeFS, not in config.** The root is runtime state the daemon
generates, so it is not something an operator writes. A private key cannot live
in config either: the `$9$` encoding is obfuscation and always decodes back, so
a root in a `pki` block would sit recoverable in `show configuration` and in
every backup. `meta/ca/cert` and `meta/ca/key` hold it instead, and file
permissions are the only protection: the blob file is 0600 and the key is not
encrypted at rest.

**The root is generated once and read afterwards.** `LoadOrGenerateRoot` reads
before it writes, so a restart presents the root an operator already
distributed. Generation is serialized inside the process, which is what makes
two goroutines racing to start a listener agree on one root. It is not
serialized across processes: ZeFS takes no file lock, so two daemons sharing one
blob already replace arbitrary state rather than only the root.

**The store reaches the daemon's ZeFS handle through a three-method interface.**
`RootStore` names `ReadFile`, `WriteFile` and `Exists`, which `storage.Storage`
satisfies, so the daemon passes its own handle with no adapter. A certificate
authority has no business reaching a config path, a write lock, or a version
history, so it is given none.

**The export command answers from the root this process loaded, not from the
store.** `LoadOrGenerateRoot` publishes the root it returns, and
`show pki local-ca pem` reads it back through `loadedRoot`. A command context
carries no storage handle, so a command that reopened the blob would answer
about a file rather than about the authority the daemon is signing with. The
answer is the certificate, the subject and the expiry. There is no accessor for
the root private key and there will be none: it leaves this package only into a
signing operation.

**The doctor check reads the store, because `ze doctor` is a different
process.** It runs pre-config, since the root depends on no config and a daemon
whose config is broken still owes the operator the state of its authority. An
absent root and an expiring one take separate codes, because the operator answer
differs: `doctor-pki-ca-root-missing` says the next start generates a root that
every peer has to be given, and `doctor-pki-ca-root-expiry` says the root
serving today runs out. A stored pair that will not load takes
`doctor-tls-invalid`, the code every other Ze surface already reports for
certificate material that does not parse.

**The doctor warns 90 days ahead of the root's expiry, where a configured
certificate gets 30.** A configured certificate is replaced on the router that
serves it. The root is replaced on every peer that trusts it, by hand, because
Ze distributes it manually and holds no revocation.

**Issuance is INJECTED into its consumers, never imported.** This package
already reaches `internal/component/plugin/ipc` (`show.go` imports
`plugin/server`, which imports `plugin/ipc`), so a consumer importing this
package back would close a cycle. `plugin.LeafIssuer` is a plain func field, the
shape `ManagedServerConfig.TLSMaterialResolver` already uses for the same
reason, and `cmd/ze/hub/main.go` is where the two halves meet. A nil issuer is
an error at construction: there is no self-signed fallback, because a
certificate nothing issued is one no peer can validate and no operator can
rotate.

## Consequences worth knowing

- The IKE engine calls `Load` after the config parse, then uses `CAPool` and
  `GetCertificate` for X.509 authentication.
- The YANG module registers through a schema `init()` and a blank import. No
  wiring code is needed in the program entry point.
- The show commands are RPC handlers inside the pki package, not in the shared
  show command package. The component stays self-contained.
- The private key leaf is marked sensitive, so the config parser decodes the
  reversible encoding before the PKI parser sees it. The PKI parser then reads
  the plaintext as PEM or as base64 DER, the same two forms the certificate
  leaves take.
- The health check reports degraded when a certificate expires within 30 days
  and down when one has expired. Expiry warnings are raised on the report bus
  after each load and clear when the certificate is renewed.
- The root lives ten years and an issued leaf lives 24 hours. Those are the
  DEFAULTS, taken by every component that RENEWS its leaf, and such a component
  must not pick a lifetime of its own. A caller that mints once and keeps the
  result names its lifetime instead: `LoadOrGenerateRootFor` and `IssueLeafFor`
  each take a validity, and the appliance build host is that caller
  (`architecture/appliance/builder.md`). A validity of zero or less is refused,
  because a certificate that is expired when it is issued reads as a daemon that
  minted it wrong.
- A 24-hour leaf is short because it is RENEWED, not because a daemon restarts
  daily. `ServingLeaf` (`internal/component/plugin/leaf.go`) holds the leaf a
  listener presents and reissues it once two thirds of its life is spent, from
  `tls.Config.GetCertificate`, which crypto/tls calls at every handshake. The
  hub acceptor and the managed listener both serve through one. A listener that
  held a fixed certificate would present an expired one from the second day,
  with the operator's config unchanged and nothing naming the cause.
- A certificate the operator NAMED is never reissued. `plugin/hub/server/certificate`
  resolves through the store and is answered unchanged, because that material is
  the operator's and renewing it is a store operation.
- A validity given to `LoadOrGenerateRootFor` applies to a root it GENERATES. A
  root the store already holds keeps the lifetime it was created with, so
  changing the value never shortens a root peers already trust.
- Every certificate the authority issues backdates `NotBefore` by five minutes,
  because a peer whose clock is ahead refuses a freshly issued leaf as not yet
  valid, and a booting router is ahead until its first NTP correction.
- Every certificate draws a random 128-bit serial. There is no ledger:
  uniqueness per issuer is the whole requirement.
- The root signs leaves directly. An intermediate exists so a root can be kept
  offline, and Ze's root is on the machine that signs with it.

<!-- source: internal/component/pki/health.go -- the pki health check and expiry warnings -->

## Traps this code exists to avoid

**A YANG list is built with the list-entry API, not with nested containers.** A
test that builds list entries with the container accessor produces a tree whose
list getter returns nil. Always add list entries as list entries.

**Key size needs an explicit type switch.** RSA, ECDSA and Ed25519 public keys
have no common size method. ECDSA reads the curve parameter bit size.

**An Ed25519 private key in Go is a 64-byte seed.** The public key is the second
half of the slice. Do not take the public key through the interface method and
type-assert it for comparison; use the slice directly.
