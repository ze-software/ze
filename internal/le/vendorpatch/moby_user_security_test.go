// Design: docs/contributing/patching-the-standard-library.md -- Patching a vendored module
// Keep GHSA-mjcv-p78q-w5fw's group-line allocation bound through dependency updates.

package vendorpatch

import (
	"bufio"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/moby/sys/user"
)

// TestMobyUserGroupParserPreservesValidGroups checks parsed account fields and
// members so a parser that refuses every input cannot satisfy the security guard.
func TestMobyUserGroupParserPreservesValidGroups(t *testing.T) {
	groups, err := user.ParseGroupFilter(strings.NewReader("root:x:0:root\nadm:x:4:root,adm,daemon\n"), nil)
	if err != nil {
		t.Fatalf("parse valid groups: %v", err)
	}
	want := []user.Group{
		{Name: "root", Pass: "x", Gid: 0, List: []string{"root"}},
		{Name: "adm", Pass: "x", Gid: 4, List: []string{"root", "adm", "daemon"}},
	}
	if !reflect.DeepEqual(groups, want) {
		t.Fatalf("parsed groups = %#v, want %#v", groups, want)
	}
}

// TestMobyUserGroupParserRejectsOversizedLine exercises the advisory's unbounded
// line path with only 2 MiB of input, without provoking an out-of-memory failure.
func TestMobyUserGroupParserRejectsOversizedLine(t *testing.T) {
	line := "oversized:x:42:" + strings.Repeat("a", 2<<20) + "\n"
	_, err := user.ParseGroupFilter(strings.NewReader(line), nil)
	if !errors.Is(err, bufio.ErrTooLong) {
		t.Fatalf("parse oversized group line: got %v, want %v", err, bufio.ErrTooLong)
	}
}
