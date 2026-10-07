package fixture

// register_config_archive_prune.go names the scenario that proves
// `commit-revisions` caps the files a file:// archive location keeps.
//
// Related: config_archive_prune_fixture.go -- the scenario body
// Related: test/ui/config-archive-prune.ci -- the run

func init() {
	Register("ui/config-archive-prune", configArchivePrune)
}
