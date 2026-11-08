// Design: docs/functional-tests.md -- a compiled fixture that stops the daemon it runs inside
// Overview: runner_exec.go -- runOrchestrated decides selfStop from this and tmpfsRequestsDaemonShutdown
// Related: internal/test/fixturedecl/fixturedecl.go -- the declaration a fixture registers

package runner

import "github.com/ze-software/ze/internal/test/fixturedecl"

// recordStartsSelfStoppingFixture reports whether the record starts a compiled
// fixture that declared, at registration, that it stops the daemon it runs in
// (fixture.StopsDaemon, read through fixturedecl.StopsDaemon).
//
// tmpfsRequestsDaemonShutdown finds the command when a script embedded in the
// .ci carries it. A Go fixture carries it in its own compiled body, which no
// text of the .ci shows, so its registration is the only place that can say
// so. Without it, a peerless test whose observer is such a fixture was
// signaled about a second after start, before the fixture had run.
//
// A fixture is found where the record starts one: the `run` leaf of an
// `external` plugin block in any embedded text read as a ze configuration (the
// stdin blocks and the tmpfs files, which hold the daemon's boot and reload
// configurations), and the exec= of a cmd= step. A text the configuration
// reader refuses is one the daemon refuses as well, so it starts no plugin, and
// a script that is not a configuration at all starts none either. An exec= that
// does not split fails its test before teardown asks this question
// (runOrchestrated), so it starts nothing here.
func recordStartsSelfStoppingFixture(rec *Record) bool {
	for i := range rec.RunCommands {
		parts, err := SplitCommand(rec.RunCommands[i].Exec)
		if err != nil {
			continue
		}
		if startsSelfStoppingFixture(parts) {
			return true
		}
	}
	for _, content := range rec.StdinBlocks {
		if configStartsSelfStoppingFixture(content) {
			return true
		}
	}
	for _, file := range rec.TmpfsFiles {
		if configStartsSelfStoppingFixture(file.Content) {
			return true
		}
	}
	return false
}

// configStartsSelfStoppingFixture reports whether a ze configuration declares an
// external plugin whose run command starts a fixture that stops the daemon.
func configStartsSelfStoppingFixture(content []byte) bool {
	config, err := readConfig(string(content))
	if err != nil {
		// The daemon refuses a configuration this reader refuses, so it starts
		// no plugin from it; a non-configuration script lands here too.
		return false
	}
	for _, plugin := range config.blocks("external") {
		run, declared := plugin.body.leaf("run")
		if !declared {
			continue
		}
		parts, err := SplitCommand(run)
		if err != nil {
			continue
		}
		if startsSelfStoppingFixture(parts) {
			return true
		}
	}
	return false
}

// startsSelfStoppingFixture reports whether argv starts a compiled fixture that
// declared it stops the daemon.
func startsSelfStoppingFixture(argv []string) bool {
	name, isFixture := fixturedecl.FixtureOf(argv)
	if !isFixture {
		return false
	}
	return fixturedecl.StopsDaemon(name)
}
