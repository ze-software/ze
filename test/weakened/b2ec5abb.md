# Test weakenings this commit accepts

The BFD two-link prestarts moved from `*-prestart.sh` files in the scenario
directory to Go constants indexed by link-local owner. The removed case tested a
refusal for a state the new type cannot hold, so nothing it guarded is left
unguarded.

| Test | Reason |
|------|--------|
| runner_test | The editor runner's tree-mode store moved into memory (owner decision 2026-10-17), so nothing is left on disk to reopen after the run. TestRunnerTreeStorageWritesTreeNotFile, renamed TestRunnerStoredEditorWritesStoreNotFixture, replaced its storage.OpenReadOnly reopen and ReadFile("test.conf") with `expect=key:path=file/active/test.conf:contains=5.6.7.8` in its .et: the same key and the same fact, checked while the run holds the store. Both loose-fixture assertions stay. Reopen durability is covered by the storage conformance table's tree row. |
| TestRunnerTreeStorageWritesTreeNotFile | Renamed, not lost: now TestRunnerStoredEditorWritesStoreNotFixture in the same file, because the tree-mode store is in memory. It keeps both loose-fixture assertions and checks the committed edit through the store with expect=key on file/active/test.conf, as the runner_test row says. |
| TestDefaultLoaderSurfacesUndeclaredExtension | The removed Module{} literals were counted as table cases; the same probe modules are now registered through RegisterModuleForTest, because DefaultLoader is cached per registry generation, and every assertion is kept. |
| TestDefaultLoaderReportsEveryRegisteredFailure | The removed Module{} literals were counted as table cases; the same failing probes are now registered through RegisterModuleForTest, and every assertion is kept. |
