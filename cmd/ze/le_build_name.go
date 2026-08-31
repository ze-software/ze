// Design: docs/architecture/system-architecture.md -- build personalities
//
// Detail: the launcher builds bin/le-<name>/le and exports the name it asked
// for. This file is the receiving half. The process reads its own invoked name
// and refuses to answer when the two disagree, so a stale bin/le, a hardcoded
// path or a shadowed PATH entry produces an error rather than an answer from
// code that is no longer in the tree.
// Related: dispatch.go -- dispatchMain calls refuseWrongBuildName first

package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// envLeBuildName is set by the root launcher, never by an operator. The option
// `./le --name <name>` is the interface; this key is how the launcher carries
// that choice into the process it execs and into that process's children.
var envLeBuildName = env.MustRegister(env.EnvEntry{
	Key:         "ze.le.build.name",
	Type:        "string",
	Description: "build name the le launcher asked for, checked against the running binary",
	Private:     true,
})

// sharedBuildName is the invoked name of the shared bin/le binary, which the
// launcher builds once and then executes without a freshness check.
const sharedBuildName = "le"

// refuseWrongBuildName answers 0 when this process is the build the caller
// asked for, and a non-zero exit code when it is not. It governs the le
// personality alone: a product ze binary started by a test under the same
// environment carries no le root, so it is left alone.
func refuseWrongBuildName() int {
	want := env.Get(envLeBuildName.Key)
	if want == "" {
		return 0
	}
	if registry.LookupRoot(sharedBuildName) == nil {
		return 0
	}
	got := invokedBuildName(os.Args[0])
	if got == want {
		return 0
	}

	var tb textbuf.Buffer
	tb.Str("le: --name asked for '").Str(want).Str("' and this process is '").Str(got)
	tb.Str("' (").Str(os.Args[0]).Str(").\n")
	tb.Str("    Reach the named build through ./le --name ").Str(want)
	tb.Str(", never through a hardcoded binary path.\n")
	tb.StdErr() //nolint:errcheck // pre-exit diagnostic
	return 2
}

// invokedBuildName reports the build name of the running binary. `--name x`
// builds bin/le-x/le, so the name lives on the directory and the file keeps the
// one name that selects the le personality. Every other binary answers with its
// own file name, which is what makes the shared bin/le report "le".
func invokedBuildName(path string) string {
	if filepath.Base(path) != sharedBuildName {
		return filepath.Base(path)
	}
	parent := filepath.Base(filepath.Dir(path))
	if suffix, found := strings.CutPrefix(parent, sharedBuildName+"-"); found {
		return suffix
	}
	return sharedBuildName
}
