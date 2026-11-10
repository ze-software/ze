// Design: docs/contributing/patching-the-standard-library.md -- the `go list std cmd` comparison this answers.
// Related: stub.go -- the go stub that answers through this listing.

package toolstub

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"strings"
)

// stubStdlibPackage is the one package the stub's GOROOT lists. The patched
// GOROOT le activates is built by internal/le/go/stdpatch/gen, which lists
// `std cmd` under the real GOROOT and under the staged tree and refuses an
// empty or differing listing. Both trees answer through this stub, so one
// package with no embedded files gives the builder a listing it can compare,
// and copies nothing.
const stubStdlibPackage = "errors"

// goListAnswer is the stub's reply to `go list <flags> std cmd` under goroot:
// a JSON stream in the shape `go list -json` prints. It refuses every other
// listing, and a goroot it was not given, so a caller that asks for more than
// the stand-in can answer is told so rather than handed an empty answer.
func goListAnswer(goroot string, args []string) ([]byte, error) {
	if goroot == "" {
		return nil, errors.New("toolstub: go list has no GOROOT to answer for: the caller sets none in GOROOT")
	}
	jsonFlag := false
	var patterns []string
	for _, arg := range args {
		if arg == "-json" || strings.HasPrefix(arg, "-json=") {
			jsonFlag = true
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		patterns = append(patterns, arg)
	}
	if !jsonFlag {
		return nil, errors.New("toolstub: go list without -json has no answer: the stub answers only the JSON listing")
	}
	if !slices.Equal(patterns, []string{"std", "cmd"}) {
		return nil, errors.New("toolstub: go list " + strings.Join(patterns, " ") + " has no answer: the stub answers only `go list -json std cmd`")
	}
	return json.Marshal(struct {
		ImportPath string
		Dir        string
	}{
		ImportPath: stubStdlibPackage,
		Dir:        filepath.Join(goroot, "src", stubStdlibPackage),
	})
}
