// Design: docs/architecture/plugin/feature-gates.md -- whole-value ownership survives field selection.
package repoinventory

import (
	"go/token"
	"strings"
	"testing"
)

// TestExternalReceivedStructFieldSolver exercises the ownership solver directly
// over production-selected retained source. The committed both-API regression
// separately proves Git capture and publication behavior; this test cannot.
func TestExternalReceivedStructFieldSolver(t *testing.T) {
	for _, receive := range []bool{false, true} {
		name := "known-initial-field"
		transfer := ""
		if receive {
			name = "received-whole-value"
			transfer = "s = <-ch;"
		}
		t.Run(name, func(t *testing.T) {
			const directory = "internal/channelproof"
			const filename = directory + "/proof.go"
			text := "package channelproof\ntype box struct { Cb func() }\nfunc Fire(ch <-chan box) { var s box = box{Cb:func(){}}; " + transfer + " s.Cb() }\n"
			snapshot := &Snapshot{
				blobs:       map[string][]byte{filename: []byte(text)},
				directories: map[string][]string{directory: {filename}},
				fset:        token.NewFileSet(), packages: make(map[string]*sourcePackage),
			}
			context := linuxContext(nil, snapshot.blobs)
			if err := snapshot.selectPackage(directory, &context); err != nil {
				t.Fatal(err)
			}
			source := &externalSource{snapshot: snapshot, roots: map[string]bool{directory: true}}
			effects := newExternalEffects(source)
			function, found := snapshot.packages[directory].functions["Fire"]
			if !found {
				t.Fatal("selected source lost Fire")
			}
			effects.body(function.decl, sourceExpr{file: function.file, pkg: function.pkg}, false)
			err := effects.solve()
			if !receive {
				if err != nil {
					t.Fatalf("known inert field refused: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("received whole struct borrowed the initial inert field's ownership")
			}
			if !strings.Contains(err.Error(), "channel") {
				t.Fatalf("received ownership refusal lost its channel cause: %v", err)
			}
		})
	}
}
