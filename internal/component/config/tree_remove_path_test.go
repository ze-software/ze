// VALIDATES: Tree.RemoveList and Tree.RemovePath remove containers, nested
// containers, list entries, whole lists, and leaves addressed by slash-separated
// paths, and return false without mutating siblings when a path is absent.
// PREVENTS: LoadConfigForStart failing to prune a registered service block
// (or accidentally dropping sibling services) when excluding invalid config.

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildSampleTreeForRemovePath() *Tree {
	root := NewTree()

	env := root.GetOrCreateContainer("environment")
	web := env.GetOrCreateContainer("web")
	web.Set("enabled", "true")
	web.Set("certificate", "missing-cert")

	lg := env.GetOrCreateContainer("looking-glass")
	lg.Set("enabled", "true")

	r1 := NewTree()
	r1.Set("name", "ams-ix")
	lg.AddListEntry("next-hop-range", "192.0.2.0/24", r1)

	r2 := NewTree()
	r2.Set("name", "de-cix")
	lg.AddListEntry("next-hop-range", "198.51.100.0/24", r2)

	bus := root.GetOrCreateContainer("bus")
	hub := bus.GetOrCreateContainer("hub")

	mainSrv := NewTree()
	mainSrv.Set("ip", "127.0.0.1")
	mainSrv.Set("certificate", "main-cert")
	hub.AddListEntry("server", "main", mainSrv)

	backupSrv := NewTree()
	backupSrv.Set("ip", "127.0.0.2")
	backupSrv.Set("certificate", "backup-cert")
	hub.AddListEntry("server", "backup", backupSrv)

	root.Set("hostname", "router-1")
	return root
}

// TestTreeRemovePathContainerAndNestedContainer proves both top-level and nested
// container removal leave sibling containers intact and accept a leading slash.
func TestTreeRemovePathContainerAndNestedContainer(t *testing.T) {
	tree := buildSampleTreeForRemovePath()

	assert.True(t, tree.RemovePath("environment/web"))
	assert.Nil(t, tree.GetContainerPath("environment/web"))
	assert.NotNil(t, tree.GetContainerPath("environment/looking-glass"), "sibling container must survive")
	assert.False(t, tree.RemovePath("environment/web"), "second removal must report false")

	assert.True(t, tree.RemovePath("/bus"))
	assert.Nil(t, tree.GetContainer("bus"))
	assert.False(t, tree.RemovePath("bus"))
}

// TestTreeRemovePathListEntry proves removing a single list entry preserves
// sibling entries and supports list keys containing slashes (CIDR prefixes).
func TestTreeRemovePathListEntry(t *testing.T) {
	tree := buildSampleTreeForRemovePath()

	assert.True(t, tree.RemovePath("bus/hub/server[main]"))
	hub := tree.GetContainerPath("bus/hub")
	require.NotNil(t, hub)
	assert.Nil(t, hub.GetList("server")["main"])
	assert.NotNil(t, hub.GetList("server")["backup"], "sibling list entry must survive")
	assert.False(t, tree.RemovePath("bus/hub/server[main]"))

	// List key containing a slash (CIDR prefix) stays in one segment.
	assert.True(t, tree.RemovePath("environment/looking-glass/next-hop-range[192.0.2.0/24]"))
	lg := tree.GetContainerPath("environment/looking-glass")
	require.NotNil(t, lg)
	assert.Nil(t, lg.GetList("next-hop-range")["192.0.2.0/24"])
	assert.NotNil(t, lg.GetList("next-hop-range")["198.51.100.0/24"])
}

// TestTreeRemovePathWholeListAndRemoveList proves RemovePath removes an entire
// list and its entry order while preserving sibling leaves, and RemoveList works directly.
func TestTreeRemovePathWholeListAndRemoveList(t *testing.T) {
	tree := buildSampleTreeForRemovePath()

	const rangeListPath = "environment/looking-glass/next-hop-range"
	assert.True(t, tree.RemovePath(rangeListPath))

	lg := tree.GetContainerPath("environment/looking-glass")
	require.NotNil(t, lg)
	assert.Empty(t, lg.GetList("next-hop-range"))
	assert.Empty(t, lg.GetListOrdered("next-hop-range"))
	enabled, ok := lg.Get("enabled")
	assert.True(t, ok)
	assert.Equal(t, "true", enabled)
	assert.False(t, tree.RemovePath(rangeListPath), "second removal of removed list must report false")

	hub := tree.GetContainerPath("bus/hub")
	require.NotNil(t, hub)
	hub.RemoveList("server")
	assert.Empty(t, hub.GetList("server"))
	assert.Empty(t, hub.GetListOrdered("server"))
}

// TestTreeRemovePathLeaf proves scalar leaves at the root, inside containers,
// and inside list entries are removed cleanly.
func TestTreeRemovePathLeaf(t *testing.T) {
	tree := buildSampleTreeForRemovePath()

	assert.True(t, tree.RemovePath("hostname"))
	_, ok := tree.Get("hostname")
	assert.False(t, ok)
	assert.False(t, tree.RemovePath("hostname"))

	assert.True(t, tree.RemovePath("bus/hub/server[main]/certificate"))
	mainSrv := tree.GetContainerPath("bus/hub").GetList("server")["main"]
	require.NotNil(t, mainSrv)
	_, ok = mainSrv.Get("certificate")
	assert.False(t, ok)
	ip, hasIP := mainSrv.Get("ip")
	assert.True(t, hasIP)
	assert.Equal(t, "127.0.0.1", ip)
}

// TestTreeRemovePathAbsentAndInvalid proves absent, empty, malformed, and nil
// tree removals return false without panicking.
func TestTreeRemovePathAbsentAndInvalid(t *testing.T) {
	var nilTree *Tree
	assert.False(t, nilTree.RemovePath("environment/web"))

	tree := buildSampleTreeForRemovePath()
	for _, path := range []string{
		"",
		"/",
		"///",
		"nonexistent",
		"environment/nonexistent",
		"environment/web/nonexistent",
		"bus/hub/server[unknown]",
		"bus/hub/server[unknown]/certificate",
		"bus/hub/server[]",
		"bus/hub/server[unclosed",
		"bus/hub/server]bad",
		"environment//web",
		"environment/web/",
	} {
		assert.False(t, tree.RemovePath(path), "path %q should return false", path)
	}
}
