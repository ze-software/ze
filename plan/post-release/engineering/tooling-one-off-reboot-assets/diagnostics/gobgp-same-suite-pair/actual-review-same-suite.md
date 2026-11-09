# Independent actual same-Suite pair review

ACCEPT the bounded secondary producer discrimination in the new same-Suite run.
This accepts the diagnostic evidence, not overlay promotion or whole-spec closure.
The older pair with changed Ze/BIRD images remains unaccepted.

1. NOTE — The actual log `job-gobgp-secondary-same-suite-private-inputs-20261106-ecdbab03.log`
   names `TestNativeGoBGPSecondarySameSuitePair` PASS, with the first native
   scenario's intended simultaneous stale-prefix failure and the second's pass.
   `selected_images_test.go` `requireReport` requires that exact one-fail/one-pass
   result, no scenario/image-retention cleanup errors, and the native Code 1.
2. NOTE — `pair-safe.json` and both safe proof projections have identical actual
   consumed Ze/BIRD/FRR/sink image IDs. Ze's executable digest also matches.
   GoBGP image and daemon digests differ; its CLI digest matches. Runtime patch
   bytes/SHA are bound to the reviewed cut `1b7b` and full `35eed` inputs by
   `requireGoBGPPairPatch` and the common `identifyPatch` algorithm.
3. NOTE — `requireCut` selects coherent complete attempts through
   `lastCompleteAttempt` and `snapshot`; the interrupted final probe is excluded.
   It reuses the original source-attribute, capability, session, generation and
   daemon comparisons. Every receiver route uses the original strict oracle,
   except that secondary prefix 6 must retain its exact prior useful path.
   The independent sink must have removed it; bookkeeping permits only that
   extra secondary survivor. A timeout alone cannot pass this outer root.
4. NOTE — The full proof has verdict `<nil>` and reaches all ten phase states,
   including repeat and restore. Both captures pass the existing wire checker
   again: cut 437 captured/439 filter/zero kernel drops; full 472/472/zero.
   `requirePair` refuses changed nonproducer inputs or a failed full verdict.
5. NOTE — Both plans call the same native assertion body through owned immutable
   expected-patch values. Their two-file GoBGP contexts contain the same canonical
   Dockerfile and selected patch only. Canonical source/patch remains read-only;
   `verifyControls` checks the frozen inputs before each plan and after the suite.
6. NOTE — Source-only review; no formatter, Go/jq, test, build or Docker execution.
   MAIN records matching formatted-source hashes after its run and retains raw
   proofs 668364761/2540349095 outside managed scratch in
   `gobgp-same-suite-private-native/`. Raw credential-bearing maps were not printed.
   Scope remains stable-session non-ADD-PATH IPv4-unicast secondary reload only.
   Historical four-peer evidence, fifteen checkers, wider phase debt and the
   original cleanup stall remain open.
