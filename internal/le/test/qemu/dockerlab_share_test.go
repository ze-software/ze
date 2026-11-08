package testqemu

import (
	"os"
	"path"
	"path/filepath"
	"slices"
	"testing"
)

// VALIDATES: a checkout whose cache link names its target through a host
// symlink (cache -> <alias>/ze/, <alias> -> <resolved>) gets the resolved
// directory exported once at its own host path and once more, under its own
// tag, at the path the link's text names, so the link resolves in the
// docker-lab guest. A link with no symlink on the way gets the one share.
// METHOD: dockerLabShares over a temporary tree, with shareMountRoots set to
// the temporary root, compared share by share; then the link's own text,
// read back, must be one of the guest mount points.
// PREVENTS: `test qemu docker-lab ... harness` failing before the command with
// "mkdir: can't create directory '/workspace/cache/'" on a Mac whose ~/.cache
// is a symlink: the cache was mounted only at its resolved path, and the
// link's text named a path that did not exist in the guest.
func TestDockerLabMountsTheCacheWhereItsLinkPoints(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	resolved := filepath.Join(base, "real", "ze")
	if err := os.MkdirAll(resolved, 0o750); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(filepath.Join(base, "real"), alias); err != nil {
		t.Fatal(err)
	}
	tree := filepath.Join(base, "tree")
	// A checkout le has run in holds a real tmp, which tmplink.Shares reads
	// before the cache link: it carries no relocated child here, so it adds
	// no share and the cache shares are the only linked ones.
	if err := os.MkdirAll(filepath.Join(tree, "tmp"), 0o750); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(tree, "cache")
	if err := os.Symlink(filepath.Join(alias, "ze")+"/", link); err != nil {
		t.Fatal(err)
	}
	run := NewRun(tree, RunOptions{})
	run.shareMountRoots = []string{base}

	shares, err := run.dockerLabShares()
	if err != nil {
		t.Fatal(err)
	}
	named := filepath.Join(alias, "ze")
	want := []dockerLabShare{
		{tag: "zelab0", host: tree, point: tree},
		{tag: "zelab1", host: resolved, point: resolved},
		{tag: "zelab2", host: resolved, point: named},
	}
	if !slices.Equal(shares, want) {
		t.Fatalf("shares = %+v\nwant     %+v", shares, want)
	}
	text, err := os.Readlink(link)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(shares, func(share dockerLabShare) bool { return share.point == path.Clean(text) }) {
		t.Fatalf("no share is mounted at the link's target %q: %+v", text, shares)
	}

	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(resolved, link); err != nil {
		t.Fatal(err)
	}
	shares, err = run.dockerLabShares()
	if err != nil {
		t.Fatal(err)
	}
	want = want[:2]
	if !slices.Equal(shares, want) {
		t.Fatalf("direct link: shares = %+v\nwant     %+v", shares, want)
	}
}
