# Handover: home-cpe closed, push pending (2026-11-09)

The home-cpe umbrella and every child spec are closed (umbrella `08788038a9`,
23b `f0072e4a2e`). The whole-CPE lab rerun passed 11 of 11 scenarios, and the
access-up pass-through break went red. Nothing is pushed: `main` is ahead of
`origin/main`.

## What a checking session must do

1. `git fetch` and `git rev-list --count origin/main..HEAD`: confirm the commits are
   still unpushed (or pushed, if Thomas pushed in between).
2. Re-run, in the main thread, the scoped evidence for the last changes:
   - `./le job run label dnssrc command go test -race -count=1 -tags "<all ze_ tags> zetest" ./internal/core/dnssource/ ./internal/component/iface/ ./internal/plugins/dhcpserver/ ./internal/plugins/dnsresolver/ ./internal/component/config/system/ ./internal/component/l2tp/plugins/pool/`
     (passed on 2026-11-09 after `e257821d8a`)
   - `./le test bgp parse dns-source-class-refused` and `dns-source-mode-required-consumers`
     (passed); `test/parse/dns-source-ra-rdnss-consumer.ci` needs Linux (guest:
     docker-lab harness, `tmp/qemu/linux-arm64/le test qemu all test <path>`).
3. Confirm the ze-test-judge verdict for the two test expectations `e257821d8a`
   changed (`TestRDNSSDNSSourceRefused`, `TestModeRequiredEachConsumer`):
   `tmp/commit-weakening-approved-*.md` or `tmp/commit-rfc-approved-*.md`.
4. Check what still blocks `./le commit push` (Thomas runs it from his own
   terminal; it refuses non-interactive use):
   - `c579480` owes an independent review of home-cpe-1 (718 code files pinned;
     571 have no recorded review at those blobs, about 50k lines). Lists:
     `tmp/session/2026-11-07-104251/scratch/hc1-uncovered`, `hc1-unc-nv`.
   - `f0072e4` owes an independent review of home-cpe-23b (149 code paths). The
     only finding of the review (`tmp/session/2026-11-07-104251/scratch/c23b-review-r6.md`,
     fix commands without instance keys) is fixed in `e257821d8a`; a clean
     re-review can now be recorded with `./le spec review record spec
     home-cpe-23b-dns-source-consumers verdict clean ...` (see that file, item 4).
   - Either debt also clears by Thomas's attestation:
     `./le commit create subject "attest owed reviews" attest <sha> "<owner words>"`.
5. Known reds not caused by home-cpe (name, do not fix): the dangling citation in
   `spec-oauth-login-0-umbrella.md` to the missing `spec-mgmt-oauth-bearer-and-web-sso.md`;
   stale BGP interop audit verdicts in `internal/le/interoplab/bgp/check_rfc.go`;
   the gh-pages and wiki command catalogs missing `show system dns`;
   `TestEveryDaemonStoppingFixtureDeclaresStopsDaemon` (another session's untracked
   `register_stops_daemon_test.go`); about 75 stale feature Doc reviews;
   `TestStreamingOneShotAnswersCarryData` fails on macOS only (passes on Linux).

Evidence logs: `tmp/session/2026-11-07-104251/scratch/rwinv/` (batches `out-gb*`,
`umbrella-lab/`, `out-main-rerun/`).
