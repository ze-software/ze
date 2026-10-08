# Stress-repro builds its daemon from the tree

The run no longer falls back to the shared bin/ze, so the race-only build hook
became a build hook for both personalities. No assertion left the suite.

| Test | Reason |
|------|--------|
| stressrepro.buildRace | Renamed, not removed: the test doubles' buildRace became build(ctx, root, output, tags, race) because run now builds a plain ze too; the race test asserts the race flag, and TestDefaultDaemonIsBuiltFromTheTree asserts the plain build. |
| TestRaceBuildFailureAndMissingBinariesAreSetupErrors | The "missing" case removed bin/ze with an os.Remove guarded by t.Fatal; bin/ze is no longer read, so the case now pins ZE_BIN at an absent path and asserts the same setup error and zero invocations. |
