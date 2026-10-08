# Site: the 177 legacy URLs 404, owner decision 2026-10-08

The owner withdrew AC-12 of plan/pre-release/spec-site-renderers-in-go.md on
2026-10-08: no redirect stubs and no legacy-URL rewriting, and the paused code
is deleted with the legacy table rather than kept unregistered (no-layering).
The behavior these tests proved no longer exists.

| Test | Reason |
|------|--------|
| TestRedirectsApplyInTheRecordedOrder | Removed with rewriteLegacyPublicURLs: owner decision 2026-10-08 lets all 177 legacy URLs 404, so no legacy-URL replacement runs and there is no order to prove. |
| TestTheLegacyTableCarriesTheAddressesTheSourcesStillLink | Removed with legacyRoutes and the legacy table: owner decision 2026-10-08, no redirect table exists to carry a retired address. |
| TestTheLegacyRewriteReachesEveryPageAndMirror | Removed with rewriteArtifactLegacyURLs: owner decision 2026-10-08, the build rewrites no retired address inside a page or mirror. |
| TestTheSitemapListsEveryPublishedPageAndNoRetiredAddress | Renamed TestTheSitemapListsEveryPublishedPage and its redirect-stub fixture dropped: the sitemap's retired-address exclusion read the deleted legacy table, and under owner decision 2026-10-08 the site publishes no stub to exclude. Every other assertion (order, no repeat, unpublished directories, mirrors) stays. |
