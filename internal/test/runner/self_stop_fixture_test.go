package runner

import (
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/test/fixturedecl"
	"github.com/ze-software/ze/internal/test/tmpfs"
)

// selfStopFixtureName is a fixture name no compiled fixture uses, declared as
// stopping the daemon by this test alone.
const selfStopFixtureName = "runner-test/declares-self-stop"

var declareSelfStopFixture sync.Once

// observerConfig is a ze configuration whose only observer is the external
// plugin run.
func observerConfig(run string) []byte {
	return []byte("plugin {\n\texternal observer {\n\t\trun \"" + run +
		"\"\n\t\tencoder json\n\t}\n}\n")
}

// TestRecordStartsSelfStoppingFixture proves a .ci whose observer is a compiled
// fixture that declared StopsDaemon is given the self-stop grace although no text
// of the .ci holds `request shutdown`.
//
// VALIDATES: recordStartsSelfStoppingFixture finds the fixture where a record
//
//	starts one (a boot or reload configuration's external plugin, an
//	exec= step) and answers from the fixture's declaration.
//
// PREVENTS: a peerless .ci whose Go observer stops the daemon being signaled
//
//	about a second after start, as test/draft/reload/dns-source-reload.ci
//	was until it carried a `# request shutdown` comment.
//
// DISCRIMINATES: drop the recordStartsSelfStoppingFixture term from selfStop in
// runOrchestrated and the daemon is signaled; return false from
// startsSelfStoppingFixture and the declared cases below fail.
func TestRecordStartsSelfStoppingFixture(t *testing.T) {
	declareSelfStopFixture.Do(func() { fixturedecl.DeclareStopsDaemon(selfStopFixtureName) })
	declared := fixturedecl.Command + " " + selfStopFixtureName + " stages.script"
	undeclared := fixturedecl.Command + " runner-test/declares-nothing stages.script"

	cases := []struct {
		name string
		rec  *Record
		want bool
	}{
		{
			"boot configuration runs the declared fixture",
			&Record{StdinBlocks: map[string][]byte{"ze-bgp": observerConfig(declared)}},
			true,
		},
		{
			"reload configuration runs the declared fixture",
			&Record{TmpfsFiles: map[string]tmpfs.File{"config2.conf": {Content: observerConfig(declared)}}},
			true,
		},
		{
			"exec step runs the declared fixture",
			&Record{RunCommands: []RunCommand{{Exec: declared}}},
			true,
		},
		{
			"resolved launcher path still names the fixture",
			&Record{StdinBlocks: map[string][]byte{"ze-bgp": observerConfig("/repo/bin/" + declared)}},
			true,
		},
		{
			"fixture that declared nothing",
			&Record{StdinBlocks: map[string][]byte{"ze-bgp": observerConfig(undeclared)}},
			false,
		},
		{
			"declared fixture named only in a comment",
			&Record{StdinBlocks: map[string][]byte{"ze-bgp": []byte("# run \"" + declared + "\"\nsystem {\n}\n")}},
			false,
		},
		{
			"no fixture at all",
			&Record{RunCommands: []RunCommand{{Exec: "ze -"}}},
			false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// The record holds no shutdown text, so only the declaration can
			// make the answer true.
			if tmpfsRequestsDaemonShutdown(tc.rec.TmpfsFiles) {
				t.Fatalf("case carries %q in its text; it must not", requestShutdownMarker)
			}
			if got := recordStartsSelfStoppingFixture(tc.rec); got != tc.want {
				t.Errorf("recordStartsSelfStoppingFixture = %v, want %v", got, tc.want)
			}
		})
	}
}
