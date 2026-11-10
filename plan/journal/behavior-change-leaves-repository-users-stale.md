# A behavior change leaves the repository's own users on the old contract

A default, an enforcement or an output format changes, and the configs, `.ci`
bodies, fixtures, benchmarks and pinned digests that use it are not converted
in the same work. Each one then goes red where nobody runs it, or stays green
because the new behavior removed what it checked.

Captured: 2026-11-02 rule testing/the-affected-population-is-not-the-edited-population/a-behavior-change-re-validates-every-repository-user

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-01 | bgp-rfc8212-ebgp-default-reject | `BenchmarkReactorForwardRSMixedRetry`, `.ci` daemon configs | After ff2aac12b0 refused an EBGP direction with no policy, the benchmark's route-server clients had no export policy and its first pass selected 0 destinations; several hundred `.ci` configs dropped every route, and some tests stayed green only because nothing was sent | fixed: be17e3283c and the per-suite `.ci` policy commits; `TestRepositoryConfigsValidate` now refuses an EBGP direction left closed outside an RFC 8212 test |
| 2026-11-01 | bgp-rfc8212-ebgp-default-reject | interop `ze.conf` files, `test/interop-l2tp` `TestNativeConfigBytesArePinned` | 747d82e911 added the RFC 8212 filter to 147 interop configs but did not repin `02-ppp-bgp-redistribute-frr/ze.conf`, and put a copy inside `control-plane-protection { bgp { } }`, where ze refused the config | fixed: 074da560f3 repins the digest; 3648c2c9d2 removes the misplaced copy |
| 2026-11-01 | - | `needs-linux` `.ci` under `test/plugin/` and `test/firewall/` | 1ae1c731bd (an expect reads only the stream it names) and 7278126c3f (ze's counter on every nft rule) left dns-stub, firewall and nft-output expectations on the old contract, unseen while the nightly ran no `needs-linux` test | fixed: 852a5abe33, all files pass in the QEMU guest on Ze's runtime kernel |
| 2026-11-09 | weekly-news-fragments | `internal/le/commit` journal-row tests | After 3475223c4b made specMechanismsRefusal require a metadata table on every edited carried spec, the journal-ownership fixtures in commit_test.go wrote specs without one, so three tests failed at commit A before reaching the ownership logic they target | fixed: fixtures render a metadata table (spec-weekly-news-fragments) |
| 2026-11-10 | - | `TestHomeWebPreviewUsesVerifiedMedia` (`internal/le/site/home_test.go`) | The test expects "poster artifact digest does not match its manifest", which the producer has not printed since `0baeaa10ab`: `terminaldemo/artifact.go` now refuses with `errAssetMismatch` ("artifact size or digest mismatch"), so the test is red for a correct refusal | open: assert the refusal the producer prints today, read from `errAssetMismatch` rather than a copied string |
