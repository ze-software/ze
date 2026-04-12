# 562 -- bfd-5-authentication

## Context

The BFD engine had carried an RFC 5880 §6.7 auth-header parser since
Stage 0 but rejected every authenticated packet with
`ErrAuthMismatch` because there was no verifier. Stage 5 implements
the four keyed variants (Keyed MD5, Meticulous Keyed MD5, Keyed
SHA1, Meticulous Keyed SHA1), explicitly refuses Simple Password,
persists the Meticulous TX sequence across process restart, and
wires the signer + verifier into the engine's receive and transmit
paths. The work closes the last "we have the parser, no
cryptographic integrity" gap in the BFD plugin.

## Decisions

- **One generic `digestSigner` / `digestVerifier` shared between
  SHA1 and MD5, keyed on a `digestFunc` alias.** Duplicating the
  wire-layout code would have been tolerable (60 lines) but the
  linter (`dupl`) caught the first attempt. The generic helper is
  slightly larger than the two split implementations combined
  would have been, but each algorithm is a two-function adapter
  (`newSHA1Signer`, `newMD5Signer`) and the shared path is easier
  to fuzz.

- **Signer/Verifier split over a single `Authenticator`
  interface.** Test code can drop in a pure-function signer
  without implementing verify, and vice versa. The real
  production types implement both sides via the same struct, so
  the split costs nothing at runtime.

- **`SeqState` tracks the receive floor as two atomics
  (`last` + `initialized`).** Before the first packet has been
  accepted, Check returns nil for any sequence so a freshly
  restarted peer with a high starting sequence is accepted. After
  the first packet, non-meticulous variants allow equal and
  meticulous requires strict increase. Check does NOT mutate the
  atomic, so a subsequent digest failure leaves the floor
  unchanged; Advance is called only on a successful Verify.

- **`SeqPersister` is a coalescing background writer, not a
  blocking call in Verify.** Writing the sequence on every TX
  would couple the express loop to disk I/O. The persister reads
  a file-backed value at construction (loaded into the public
  `Start()` accessor so the first outgoing TX picks up after the
  old floor), takes Store() nudges from the engine, and flushes
  at a fixed cadence (default 500 ms) or on Close. Write failures
  set a latch and do not block; RFC 5880 §6.7.3 tolerates a
  small sequence regression after a crash because the peer's
  replay window will slide forward eventually.

- **Simple Password rejected at parse time, not at runtime.**
  RFC 5880 §6.7.2 warns that Simple Password "is provided only
  for backwards compatibility" and gives no cryptographic
  protection. The YANG enum only lists the four keyed variants,
  and the parser double-checks with an explicit rejection that
  cites the RFC so an operator who hand-writes JSON config with
  `type: "simple-password"` sees a clear error.

- **Engine-owned AuthPair construction.** The `api.SessionRequest`
  carries only a value-typed `AuthSettings`. `engine.EnsureSession`
  imports `internal/plugins/bfd/auth`, builds the signer,
  verifier, and persister, and hands the bundle to
  `session.Machine.SetAuth`. The api package stays a leaf (no
  auth import) and BGP peers that opt into BFD can pass a nil
  Auth field without changes to their own code.

- **`Machine.Build` reports `c.Length` including the auth body.**
  The previous layout set `c.Length = MandatoryLen` because no
  sender could ever flip `c.Auth`. Stage 5 makes
  `c.Length = MandatoryLen + signer.BodyLen()` so the receiver
  sees the correct length, and `engine.sendLocked` appends the
  auth bytes after `c.WriteTo` via `machine.Sign`.

- **`bfd.XmitAuthSeq` advances on every TX, not only for
  meticulous variants.** RFC 5880 §6.7.3 allows non-meticulous
  sessions to keep the same sequence across back-to-back retries,
  but advancing unconditionally is equally spec-compliant and
  gives a slightly tighter replay window. Simpler too: the
  engine calls `AdvanceAuthSeq()` unconditionally after every
  Sign.

## Consequences

- **Authenticated BFD is usable.** Operators can add a single
  `auth { type keyed-sha1 key-id 7 secret "..." }` block to a
  profile and every session inheriting it signs and verifies.
  MD5 works the same way.

- **Sequence persistence is opt-in via `bfd { persist-dir ... }`.**
  Meticulous sessions without a persist-dir still work at
  runtime but lose their floor on restart; the peer will reject
  until the sequence catches up naturally. The leaf is
  top-level on `bfd { }` rather than per-session so a single
  directory holds the whole daemon's state.

- **`api.Service` signature unchanged.** `api.AuthSettings` is an
  additive field on `SessionRequest`; BGP peers that opt into
  BFD (Stage 3) continue to run unauthenticated without code
  changes. External plugin authors get the type via the api
  package without pulling in the engine.

- **Metrics gained `ze_bfd_auth_failures_total{mode}`.** Stage 4
  already published five metric families; Stage 5 adds the sixth.
  The counter increments on every Verify failure (digest
  mismatch, replay, short body, wrong key ID) and gives NOCs a
  clear signal that a key rotation or peer mis-config is dropping
  traffic.

- **Follow-ups tracked.** `ze config show` secret redaction
  (AC-11) needs the core config pipeline and is deferred as its
  own row. A dedicated fuzz target for the auth body parser is
  also deferred -- the existing `packet/fuzz_test.go` covers
  `ParseControl`, and the digest helpers handle length checks
  before the hash computation so a malformed body cannot reach
  the crypto.

## Gotchas

- **`block-silent-ignore.sh` refuses `default:` in switch
  statements.** Same trap as every previous BFD stage. The auth
  type enum resolver uses an if-chain and a post-switch explicit
  check.

- **`check-existing-patterns.sh` refuses a top-level `Config`
  type.** Renamed to `Settings` so the existing `config` packages
  keep their Config type.

- **`gosec` flags any struct field named `Secret`.** Solved with
  an inline `//nolint:gosec` noting the field is a BFD auth key
  and the containing type is never serialized.

- **`dupl` flagged the first attempt at separate SHA1 and MD5
  files.** Extracted `digestSigner`/`digestVerifier` helpers that
  both hash algorithms parameterize via a shared layout and a
  `digestFunc` alias.

- **`errorlint` refuses `fmt.Errorf("%v; %v", a, b)` for multi-
  error paths.** Used `errors.Join` in `writeSeqFile` to
  aggregate the primary failure with any cleanup errors.

- **`persist_test.go` raced the writer goroutine when mutating
  `writeFn` post-construction.** Added an internal
  `newTestSeqPersister(dir, key, flush, writeFn)` helper so the
  test can set the field BEFORE starting the goroutine.

- **`proc.communicate` errors on closed stdin when used after a
  manual `proc.stdin.close()`.** The meticulous-persist .ci test
  switched to `proc.wait() + proc.stderr.read()` to sidestep the
  "I/O operation on closed file" failure.

- **Plugin parse errors do NOT exit ze.** The `bfd-auth-mismatch`
  test orchestrator has to SIGTERM the daemon after observing
  the rejection log line; ze continues running even when the
  BFD config is invalid because BFD is non-fatal by design.

- **`Machine.Build` needs to know the auth body length at Build
  time.** The engine calls `sendLocked` with the Control struct;
  if `c.Length` is wrong, the peer drops the packet for a length
  mismatch. Tried leaving `c.Length = MandatoryLen` first; the
  first unit test hit a length check in `ParseControl` and
  rejected the packet.

## Files

- `internal/plugins/bfd/auth/signer.go` (new) -- Signer/Verifier
  interfaces, Settings, `NewSigner`/`NewVerifier` dispatch.
- `internal/plugins/bfd/auth/sha1.go` (new) -- Generic
  `digestSigner`/`digestVerifier` + SHA1 adapters.
- `internal/plugins/bfd/auth/md5.go` (new) -- MD5 adapters.
- `internal/plugins/bfd/auth/meticulous.go` (new) -- `SeqState`.
- `internal/plugins/bfd/auth/persist.go` (new) -- `SeqPersister`
  coalescing writer + `newTestSeqPersister` helper.
- `internal/plugins/bfd/auth/sha1_test.go` (new).
- `internal/plugins/bfd/auth/persist_test.go` (new).
- `internal/plugins/bfd/session/auth.go` (new) -- `AuthPair` +
  Machine plumbing.
- `internal/plugins/bfd/session/session.go` -- `authPair` and
  `rcvAuthSeq` fields on Machine; `XmitAuthSeq` on Vars.
- `internal/plugins/bfd/session/fsm.go` -- `Build` reports
  MandatoryLen + signer.BodyLen() when auth is installed.
- `internal/plugins/bfd/engine/engine.go` -- `EnsureSession`
  builds the AuthPair; `ReleaseSession` calls `CloseAuth`;
  `MetricsHook` grows `OnAuthFailure`.
- `internal/plugins/bfd/engine/loop.go` -- `handleInbound`
  verifies before FSM, `sendLocked` signs after `WriteTo`.
- `internal/plugins/bfd/bfd.go` -- `applyPinned` plumbs
  `cfg.persistDir` into every `req.PersistDir`.
- `internal/plugins/bfd/config.go` -- `parseAuthConfig`,
  `authTypeFromEnum`, `authConfig` with Secret, persist-dir.
- `internal/plugins/bfd/api/events.go` -- `AuthSettings`,
  `SessionRequest.Auth`, `SessionRequest.PersistDir`.
- `internal/plugins/bfd/schema/ze-bfd-conf.yang` -- top-level
  `persist-dir` leaf, profile `auth { type key-id secret }`
  container with the four keyed enum variants.
- `internal/plugins/bfd/metrics.go` -- `authFailures` CounterVec,
  `metricsHook.OnAuthFailure`.
- `test/plugin/bfd-auth-sha1.ci` (new).
- `test/plugin/bfd-auth-mismatch.ci` (new).
- `test/plugin/bfd-auth-meticulous-persist.ci` (new).
- `plan/deferrals.md` -- Stage 5 row closed.
