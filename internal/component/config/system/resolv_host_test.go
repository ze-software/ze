package system

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/dnssource"
)

// hostRegistry answers a registry that reads its host resolv.conf from
// hostPath, never the machine's /etc/resolv.conf, and writes no file.
func hostRegistry(hostPath string) *Resolvers {
	return &Resolvers{hostPath: hostPath}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// hostRecord answers the host origin's record in the registry's snapshot.
func hostRecord(r *Resolvers) (dnssource.Record, bool) {
	records := r.Snapshot().Records
	for i := range records {
		if records[i].Origin == hostResolverOrigin.name {
			return records[i], true
		}
	}
	return dnssource.Record{}, false
}

// TestHostRecordAbsentFileRegistersNothing: a host path that does not exist
// gives no record and no server, and no default server appears in its place.
//
// VALIDATES: spec-home-cpe-23 D-13 and D-14 condition 2.
// PREVENTS: a hardcoded fallback resolver when the host has none.
func TestHostRecordAbsentFileRegistersNothing(t *testing.T) {
	r := hostRegistry(filepath.Join(t.TempDir(), "missing-resolv.conf"))
	if err := r.Configure("", nil); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if record, ok := hostRecord(r); ok {
		t.Fatalf("host record %+v from a missing file", record)
	}
	if got := r.Servers(); len(got) != 0 {
		t.Fatalf("servers = %v, want none", got)
	}
}

// TestHostRecordNoNameserverRegistersNothing: a host file with no nameserver
// line gives no record.
//
// VALIDATES: D-14 condition 3.
// PREVENTS: a record holding a server the file never named.
func TestHostRecordNoNameserverRegistersNothing(t *testing.T) {
	hostPath := filepath.Join(t.TempDir(), "resolv.conf")
	writeFile(t, hostPath, "# managed elsewhere\nsearch example.net\noptions ndots:1\nnameserver\n")
	r := hostRegistry(hostPath)
	if err := r.Configure("", nil); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if record, ok := hostRecord(r); ok {
		t.Fatalf("host record %+v from a file with no nameserver", record)
	}
	if got := r.Servers(); len(got) != 0 {
		t.Fatalf("servers = %v, want none", got)
	}
}

// TestHostRecordServersInFileOrder: a host file naming several servers gives
// one record holding exactly those, in file order, and nothing added.
//
// VALIDATES: D-13, D-14 condition 4.
// PREVENTS: reordering, a dropped server, or an invented one.
func TestHostRecordServersInFileOrder(t *testing.T) {
	hostPath := filepath.Join(t.TempDir(), "resolv.conf")
	writeFile(t, hostPath, "nameserver 198.51.100.2\nsearch example.net\nnameserver 2001:db8::53\nnameserver 192.0.2.7\n")
	r := hostRegistry(hostPath)
	if err := r.Configure("", nil); err != nil {
		t.Fatalf("configure: %v", err)
	}
	record, ok := hostRecord(r)
	if !ok {
		t.Fatal("no host record")
	}
	want := []string{"198.51.100.2", "2001:db8::53", "192.0.2.7"}
	if got := serverTexts(record.Addresses); !slices.Equal(got, want) {
		t.Fatalf("host record = %v, want %v", got, want)
	}
	if got := r.Servers(); !slices.Equal(got, want) {
		t.Fatalf("servers = %v, want %v", got, want)
	}
}

// TestHostRecordSkipsZesOwnResolvConf: when the host file is Ze's own
// resolv-conf-path, named directly or through a symlink, it is not read.
//
// VALIDATES: D-13 ("read only when it is not Ze's own resolv-conf-path"),
// D-14 condition 5.
// PREVENTS: Ze reading back the servers it wrote and keeping them alive
// after their source is withdrawn.
func TestHostRecordSkipsZesOwnResolvConf(t *testing.T) {
	dir := t.TempDir()
	own := filepath.Join(dir, "ze-resolv.conf")
	writeFile(t, own, "nameserver 192.0.2.99\n")
	link := filepath.Join(dir, "etc-resolv.conf")
	if err := os.Symlink(own, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	for _, hostPath := range []string{own, link} {
		r := hostRegistry(hostPath)
		if err := r.Configure(own, nil); err != nil {
			t.Fatalf("configure: %v", err)
		}
		if record, ok := hostRecord(r); ok {
			t.Fatalf("host path %s is Ze's own file, yet read: %+v", hostPath, record)
		}
	}
}

// TestHostRecordAloneFeedsTheResolver: with no name-server and no lease, the
// host's servers are the router's selection, and a watcher (the hub hands
// that to Ze's internal resolver) receives them.
//
// VALIDATES: D-13 (a Ze on a distribution host keeps resolving), D-14
// condition 6.
// PREVENTS: Ze's own lookups failing on a host Ze does not manage.
func TestHostRecordAloneFeedsTheResolver(t *testing.T) {
	hostPath := filepath.Join(t.TempDir(), "resolv.conf")
	writeFile(t, hostPath, "nameserver 198.51.100.2\nnameserver 198.51.100.3\n")
	r := hostRegistry(hostPath)

	var seen []string
	stop := r.Watch(func(s *dnssource.Snapshot) { seen = serverTexts(s.Router.Servers) })
	defer stop()
	if len(seen) != 0 {
		t.Fatalf("before configure the watcher saw %v", seen)
	}
	if err := r.Configure("", nil); err != nil {
		t.Fatalf("configure: %v", err)
	}
	want := []string{"198.51.100.2", "198.51.100.3"}
	if !slices.Equal(seen, want) {
		t.Fatalf("watcher saw %v, want %v", seen, want)
	}

	// A configured name-server wins over the host record (the default list
	// puts configured first), and the watcher follows the change.
	if err := r.Configure("", []string{"192.0.2.53"}); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if want := []string{"192.0.2.53"}; !slices.Equal(seen, want) {
		t.Fatalf("watcher saw %v, want %v", seen, want)
	}
}
